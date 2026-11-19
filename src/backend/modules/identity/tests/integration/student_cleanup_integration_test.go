//go:build integration

package integration

import (
	"context"
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
	tok := func(u string, mfa bool) string { return idp.Token("sub-"+u, httpxtest.PortalClient, mfa, now) }
	inA := func() map[string]string {
		return map[string]string{"X-Tenant-Id": a.tenant, "Idempotency-Key": uuid.Must(uuid.NewV7()).String()}
	}

	// AC-11: exclusion with reason (tenant admin), audited; School Admin without the permission → 403.
	if r := h.do(t, "POST", "/tenant/student-cleanup/exclusions", tok("ad.donvi.cl", false), inA(), map[string]any{"student_ids": []uuid.UUID{s3},
		"reason": "Học sinh ốm dài ngày"}); r.status != 204 {
		t.Fatalf("exclude: %d %v", r.status, r.body)
	}
	if r := h.do(t, "POST", "/tenant/student-cleanup/exclusions", tok("ad.truong.cl", false), inA(), map[string]any{"student_ids": []uuid.UUID{s4},
		"reason": "Thử"}); r.status != 403 {
		t.Fatalf("school admin exclude: %d", r.status)
	}
	// AC-10, AC-16: preview of the tenant (due + notice window), never another tenant's students.
	r := h.do(t, "GET", "/tenant/student-cleanup/preview?limit=50", tok("ad.donvi.cl", false), map[string]string{"X-Tenant-Id": a.tenant}, nil)
	if r.status != 200 || len(r.body["data"].([]any)) != 4 { // s1, s2, s4, s6 (s3 excluded, s5 recent)
		t.Fatalf("preview: %d %v", r.status, r.body)
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
	// Only platform.student_cleanup.approve approves (tenant admin → 403).
	path := "/admin/student-cleanup/runs/" + run.ID.String() + ":approve"
	if r := h.do(t, "POST", path, tok("ad.donvi.cl", false), map[string]string{"Idempotency-Key": uuid.Must(uuid.NewV7()).String()}, nil); r.status != 403 {
		t.Fatalf("tenant admin approve: %d", r.status)
	}
	if r := h.do(t, "POST", path, tok("sa.cl", true), map[string]string{"Idempotency-Key": uuid.Must(uuid.NewV7()).String()}, nil); r.status != 202 {
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
	// Runs list for the System Admin.
	if r := h.do(t, "GET", "/admin/student-cleanup/runs", tok("sa.cl", true), nil, nil); r.status != 200 || len(r.body["data"].([]any)) != 1 {
		t.Fatalf("runs: %d %v", r.status, r.body)
	}
	// No physical delete anywhere (R21): the rows are still there.
	if count(ctx, t, e.platform, `SELECT count(*) FROM org.students WHERE student_id IN ($1, $2)`, s1, s2) != 2 {
		t.Fatal("rows deleted")
	}
	_ = authz.PermPlatformStudentCleanupApprove
}
