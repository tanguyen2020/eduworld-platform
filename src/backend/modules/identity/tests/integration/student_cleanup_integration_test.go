//go:build integration

package integration

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/saga"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/settingscfg"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
)

// fakeParticipant is a TEST DOUBLE of a module facade in the cleanup saga (the game and learning facades
// have their own tests): it blocks the students of `block` and records cleanups and restores.
type fakeParticipant struct {
	name              string
	block             map[uuid.UUID]bool
	cleaned, restored map[uuid.UUID]int
}

func newFakeParticipant(name string) *fakeParticipant {
	return &fakeParticipant{name: name, block: map[uuid.UUID]bool{}, cleaned: map[uuid.UUID]int{}, restored: map[uuid.UUID]int{}}
}

func (f *fakeParticipant) Name() string { return f.name }
func (f *fakeParticipant) Cleanup(_ context.Context, s, _ uuid.UUID, _ *time.Time) (port.StudentDataResult, error) {
	if f.block[s] {
		return port.StudentDataResult{Status: saga.Blocked, BlockedReason: "open_exam_attempt"}, nil
	}
	f.cleaned[s]++
	return port.StudentDataResult{Status: saga.Done}, nil
}
func (f *fakeParticipant) Restore(_ context.Context, s uuid.UUID, _ bool) (port.StudentDataResult, error) {
	f.restored[s]++
	return port.StudentDataResult{Status: saga.Done}, nil
}

// F016 US3: F016-AC-09..AC-14, AC-16 (participants are test doubles; facades are tested in their modules).
func TestStudentCleanup(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	h, idp := realAuthHTTP(ctx, t, e)
	a := seedTenant(ctx, t, e.platform, "pgd-cl-a")
	b := seedTenant(ctx, t, e.platform, "pgd-cl-b")
	ta := uuid.MustParse(a.tenant)
	mk := func(name string, lastLoginDaysAgo, provisionedDaysAgo int) uuid.UUID {
		id := enrolled(ctx, t, e.platform, a, name)
		if _, err := e.platform.ExecContext(ctx, `UPDATE identity.users SET status = 'active', provisioned_at = now() - make_interval(days => $2),
			last_login_at = CASE WHEN $3 > 0 THEN now() - make_interval(days => $3) END WHERE user_id = $1`, id, provisionedDaysAgo, lastLoginDaysAgo); err != nil {
			t.Fatal(err)
		}
		return id
	}
	s1 := mk("hs.cl.1", 100, 400) // last login 100 days ago → due
	s2 := mk("hs.cl.2", 0, 95)    // never logged in, provisioned 95 days ago → due
	s3 := mk("hs.cl.3", 120, 400) // excluded
	s4 := mk("hs.cl.4", 130, 400) // blocked by a participant
	mk("hs.cl.5", 5, 400)         // recent
	s6 := mk("hs.cl.6", 80, 400)  // due in 10 days → notice only
	enrolled(ctx, t, e.platform, b, "hs.cl.b")
	// Tenant cleanup days 90 (the platform default allows overrides).
	if _, err := e.platform.ExecContext(ctx, `UPDATE platform.tenants SET settings = '{"inactive_student_cleanup_days": 90}' WHERE tenant_id = $1`, a.tenant); err != nil {
		t.Fatal(err)
	}
	iss := idp.Issuer()
	portalUser(ctx, t, e, iss, "ad.donvi.cl", "tenant_admin", &a.tenant, "tenant", nil)
	portalUser(ctx, t, e, iss, "ad.truong.cl", "school_admin", &a.tenant, "school", &a.school)
	portalUser(ctx, t, e, iss, "sa.cl", "system_admin", nil, "platform", nil)
	now := time.Now()
	tok := func(u string) string { return idp.Token("sub-"+u, httpxtest.PortalClient, now) }
	inA := func() map[string]string {
		return map[string]string{"X-Tenant-Id": a.tenant, "Idempotency-Key": uuid.Must(uuid.NewV7()).String()}
	}

	// AC-11: exclusion with reason (tenant admin), audited; School Admin without the permission → 403.
	if r := h.do(t, "POST", "/tenant/student-cleanup/exclusions", tok("ad.donvi.cl"), inA(), map[string]any{"student_ids": []uuid.UUID{s3},
		"reason": "Học sinh ốm dài ngày"}); r.status != 204 {
		t.Fatalf("exclude: %d %v", r.status, r.body)
	}
	if r := h.do(t, "POST", "/tenant/student-cleanup/exclusions", tok("ad.truong.cl"), inA(), map[string]any{"student_ids": []uuid.UUID{s4},
		"reason": "Thử"}); r.status != 403 {
		t.Fatalf("school admin exclude: %d", r.status)
	}
	// AC-10, AC-16: preview of the tenant (due + notice window), never another tenant's students.
	r := h.do(t, "GET", "/tenant/student-cleanup/preview?limit=50", tok("ad.donvi.cl"), map[string]string{"X-Tenant-Id": a.tenant}, nil)
	if r.status != 200 || len(r.body["data"].([]any)) != 4 { // s1, s2, s4, s6 (s3 excluded, s5 recent)
		t.Fatalf("preview: %d %v", r.status, r.body)
	}
	// API_DESIGN §5: keyset pages of the preview (limit 3 → 3 + 1, same students, no overlap).
	p1 := h.do(t, "GET", "/tenant/student-cleanup/preview?limit=3", tok("ad.donvi.cl"), map[string]string{"X-Tenant-Id": a.tenant}, nil)
	pg1, _ := p1.body["page"].(map[string]any)
	if p1.status != 200 || len(p1.body["data"].([]any)) != 3 || pg1["has_more"] != true || pg1["next_cursor"] == nil {
		t.Fatalf("preview page 1: %d %v", p1.status, p1.body)
	}
	p2 := h.do(t, "GET", "/tenant/student-cleanup/preview?limit=3&cursor="+url.QueryEscape(pg1["next_cursor"].(string)), tok("ad.donvi.cl"),
		map[string]string{"X-Tenant-Id": a.tenant}, nil)
	pg2, _ := p2.body["page"].(map[string]any)
	if p2.status != 200 || len(p2.body["data"].([]any)) != 1 || pg2["has_more"] != false {
		t.Fatalf("preview page 2: %d %v", p2.status, p2.body)
	}
	seen := map[any]bool{}
	for _, x := range append(p1.body["data"].([]any), p2.body["data"].([]any)...) {
		seen[x.(map[string]any)["student_id"]] = true
	}
	if len(seen) != 4 {
		t.Fatalf("preview pages overlap: %v", seen)
	}

	defaults, err := settingscfg.DefaultsFrom(config.Load())
	if err != nil {
		t.Fatal(err)
	}
	game, learning := newFakeParticipant("game"), newFakeParticipant("learning")
	learning.block[s4] = true
	eng := saga.Engine{UoW: e.uow, Repo: write.SagaRepo{}}
	cleanup := command.StudentCleanup{UoW: e.uow, Repo: write.CleanupRepo{}, Settings: write.SettingsRepo{}, Defaults: defaults, Saga: eng,
		Participants: []port.StudentDataParticipant{game, learning}, Lifecycle: write.LifecycleRepo{}, Events: e.deps.Events, Audit: e.deps.Audit}

	// AC-12: 3 candidates out of 7 students is above 10 % → awaiting_approval, nothing removed.
	run, err := cleanup.RunTenant(ctx, ta)
	if err != nil || run == nil || run.Status != "awaiting_approval" || run.Candidates != 3 {
		t.Fatalf("large batch: %+v %v", run, err)
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE delete_reason = 'inactive_cleanup'`) != 0 {
		t.Fatal("removed before approval")
	}
	// AC-10: notice for s6.
	if count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'student.cleanup_scheduled' AND envelope::text LIKE '%' || $1 || '%'`, s6.String()) != 1 {
		t.Fatal("cleanup_scheduled")
	}
	// OQ F057-2: cancelStudentCleanupRun — only platform.student_cleanup.approve (tenant admin → 403), reason 3..500,
	// awaiting_approval → cancelled, audited, nothing removed; a cancelled run cannot be approved or cancelled again
	// and the next job tick starts a fresh run.
	idem := func() map[string]string {
		return map[string]string{"Idempotency-Key": uuid.Must(uuid.NewV7()).String()}
	}
	cancelPath := "/admin/student-cleanup/runs/" + run.ID.String() + ":cancel"
	if r := h.do(t, "POST", cancelPath, tok("ad.donvi.cl"), idem(), map[string]any{"reason": "Không dọn đợt này"}); r.status != 403 {
		t.Fatalf("tenant admin cancel: %d", r.status)
	}
	if r := h.do(t, "POST", cancelPath, tok("sa.cl"), idem(), map[string]any{"reason": "ab"}); r.status != 422 {
		t.Fatalf("short reason: %d %v", r.status, r.body)
	}
	if r := h.do(t, "POST", "/admin/student-cleanup/runs/"+uuid.Must(uuid.NewV7()).String()+":cancel", tok("sa.cl"), idem(),
		map[string]any{"reason": "Không dọn đợt này"}); r.status != 404 {
		t.Fatalf("unknown run: %d", r.status)
	}
	if r := h.do(t, "POST", cancelPath, tok("sa.cl"), idem(), map[string]any{"reason": "Không dọn đợt này"}); r.status != 204 {
		t.Fatalf("cancel: %d %v", r.status, r.body)
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM platform.student_cleanup_runs WHERE run_id = $1 AND status = 'cancelled'
		AND finished_at IS NOT NULL`, run.ID) != 1 {
		t.Fatal("run not cancelled")
	}
	if count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'student_cleanup.cancelled' AND envelope::text LIKE '%' || $1 || '%'`, run.ID.String()) != 1 {
		t.Fatal("cancel not audited")
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE delete_reason = 'inactive_cleanup'`) != 0 {
		t.Fatal("removed by a cancelled run")
	}
	if r := h.do(t, "POST", cancelPath, tok("sa.cl"), idem(), map[string]any{"reason": "Không dọn đợt này"}); r.status != 409 {
		t.Fatalf("cancel twice: %d", r.status)
	}
	if r := h.do(t, "POST", "/admin/student-cleanup/runs/"+run.ID.String()+":approve", tok("sa.cl"), idem(), nil); r.status != 409 {
		t.Fatalf("approve a cancelled run: %d", r.status)
	}
	cancelled := run.ID
	run, err = cleanup.RunTenant(ctx, ta)
	if err != nil || run == nil || run.Status != "awaiting_approval" || run.ID == cancelled {
		t.Fatalf("fresh run after cancel: %+v %v", run, err)
	}

	// Only platform.student_cleanup.approve approves (tenant admin → 403).
	path := "/admin/student-cleanup/runs/" + run.ID.String() + ":approve"
	if r := h.do(t, "POST", path, tok("ad.donvi.cl"), map[string]string{"Idempotency-Key": uuid.Must(uuid.NewV7()).String()}, nil); r.status != 403 {
		t.Fatalf("tenant admin approve: %d", r.status)
	}
	if r := h.do(t, "POST", path, tok("sa.cl"), map[string]string{"Idempotency-Key": uuid.Must(uuid.NewV7()).String()}, nil); r.status != 202 {
		t.Fatalf("approve: %d %v", r.status, r.body)
	}

	// AC-13, AC-14: the approved run is applied: s1, s2 cleaned (soft-delete, events), s4 blocked → game step compensated.
	run, err = cleanup.RunTenant(ctx, ta)
	if err != nil || run.Status != "completed" || run.Cleaned != 2 || run.SkippedActive != 1 {
		t.Fatalf("apply: %+v %v", run, err)
	}
	for _, s := range []uuid.UUID{s1, s2} {
		if count(ctx, t, e.platform, `SELECT count(*) FROM identity.users u JOIN org.students st ON st.student_id = u.user_id
			WHERE u.user_id = $1 AND u.delete_reason = 'inactive_cleanup' AND st.delete_reason = 'inactive_cleanup'`, s) != 1 {
			t.Fatalf("%s not cleaned", s)
		}
		if count(ctx, t, e.app, `SELECT count(*) FROM identity.memberships_for_user($1)`, s) != 0 {
			t.Fatal("membership kept")
		}
		if count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'student.cleaned_up' AND envelope::text LIKE '%' || $1 || '%'`, s.String()) != 1 {
			t.Fatal("student.cleaned_up")
		}
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE user_id IN ($1, $2) AND deleted_at IS NOT NULL`, s3, s4) != 0 {
		t.Fatal("excluded or blocked student removed")
	}
	if game.restored[s4] != 1 || game.cleaned[s4] != 1 {
		t.Fatalf("compensation of s4: cleaned=%d restored=%d", game.cleaned[s4], game.restored[s4])
	}
	// Runs list for the System Admin: the applied run and the cancelled one.
	r = h.do(t, "GET", "/admin/student-cleanup/runs", tok("sa.cl"), nil, nil)
	if r.status != 200 || len(r.body["data"].([]any)) != 2 {
		t.Fatalf("runs: %d %v", r.status, r.body)
	}
	statuses := map[string]bool{}
	for _, x := range r.body["data"].([]any) {
		statuses[x.(map[string]any)["status"].(string)] = true
	}
	if !statuses["completed"] || !statuses["cancelled"] {
		t.Fatalf("run statuses = %v", statuses)
	}
	// API_DESIGN §5: limit 1 → a cursor to the second run, then the last page.
	r1 := h.do(t, "GET", "/admin/student-cleanup/runs?limit=1", tok("sa.cl"), nil, nil)
	rp1, _ := r1.body["page"].(map[string]any)
	if r1.status != 200 || len(r1.body["data"].([]any)) != 1 || rp1["has_more"] != true || rp1["next_cursor"] == nil {
		t.Fatalf("runs page 1: %d %v", r1.status, r1.body)
	}
	r2 := h.do(t, "GET", "/admin/student-cleanup/runs?limit=1&cursor="+url.QueryEscape(rp1["next_cursor"].(string)), tok("sa.cl"), nil, nil)
	rp2, _ := r2.body["page"].(map[string]any)
	if r2.status != 200 || len(r2.body["data"].([]any)) != 1 || rp2["has_more"] != false ||
		r2.body["data"].([]any)[0].(map[string]any)["run_id"] == r1.body["data"].([]any)[0].(map[string]any)["run_id"] {
		t.Fatalf("runs page 2: %d %v", r2.status, r2.body)
	}
	// No physical delete anywhere (R21): the rows are still there.
	if count(ctx, t, e.platform, `SELECT count(*) FROM org.students WHERE student_id IN ($1, $2)`, s1, s2) != 2 {
		t.Fatal("rows deleted")
	}
	_ = authz.PermPlatformStudentCleanupApprove
}
