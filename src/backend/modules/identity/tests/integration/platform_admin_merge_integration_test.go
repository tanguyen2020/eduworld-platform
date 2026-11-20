//go:build integration

package integration

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// mergeOwners is a TEST DOUBLE (R17) of mergeUserData in learning, game, competition and chat (no module
// implements it yet, OQ F045-3): it answers the configured status and records the Idempotency-Key and scope.
type mergeOwners struct {
	port.OwnerServiceGateway
	mu     sync.Mutex
	status map[string]string
	calls  map[string]int
	keys   map[string][]string
	scoped map[string]bool
}

func (m *mergeOwners) MergeUserData(ctx context.Context, module string, from uuid.UUID, req port.MergeUserDataRequest) (port.MergeUserDataResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k, _ := modcall.IdempotencyKeyFrom(ctx)
	_, scoped := tenancy.ScopeFrom(ctx)
	m.calls[module]++
	m.keys[module] = append(m.keys[module], k)
	m.scoped[module] = scoped
	st := m.status[module]
	if st == "" {
		st = "done"
	}
	return port.MergeUserDataResult{Status: st}, nil
}

func mergeIssue(ctx context.Context, t *testing.T, e *env, f fixture, resolution *string) uuid.UUID {
	t.Helper()
	id := ids.New()
	if _, err := e.platform.ExecContext(ctx, `INSERT INTO sis.reconciliation_issues (issue_id, tenant_id, created_at, updated_at, school_id, issue_type,
		entity_refs, status, resolution) VALUES ($1, $2, now(), now(), $3, 'duplicate_identifier', '{}', 'open', $4)`, id, f.tenant, f.school, resolution); err != nil {
		t.Fatal(err)
	}
	return id
}

func extIdentity(ctx context.Context, t *testing.T, e *env, user uuid.UUID, issuer, subject string) {
	t.Helper()
	if _, err := e.platform.ExecContext(ctx, `INSERT INTO identity.external_identities (external_identity_id, created_at, updated_at, issuer, subject, user_id)
		VALUES ($1, now(), now(), $2, $3, $4)`, uuid.New(), issuer, subject, user); err != nil {
		t.Fatal(err)
	}
}

// F045 US6: F045-AC-27..AC-33, SC-006 — mergeUsers through HTTP, the saga user_merge run by the job
// platform-saga, the identity step on the real tables, the module steps on the mergeOwners TEST DOUBLE.
func TestPlatformAdminMerge(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	owners := &mergeOwners{status: map[string]string{}, calls: map[string]int{}, keys: map[string][]string{}, scoped: map[string]bool{}}
	p := platformHTTP(ctx, t, e, func(g port.OwnerServiceGateway) port.OwnerServiceGateway {
		owners.OwnerServiceGateway = g
		return owners
	})
	a := seedTenant(ctx, t, e.platform, "th-mg-a")
	b := seedTenant(ctx, t, e.platform, "th-mg-b")
	platformUsers(ctx, t, p, a, "mg")
	sa, ca, sch := p.token("sa.mg", true), p.token("ca.mg", true), p.token("ad.truong.mg", false)
	inA := map[string]string{"X-Tenant-Id": a.tenant}

	a1 := enrolled(ctx, t, e.platform, a, "hs.trung.a1")
	a2 := enrolled(ctx, t, e.platform, a, "hs.trung.a2")
	b1 := enrolled(ctx, t, e.platform, b, "hs.trung.b1")
	const local, schoolIdP = "http://kc/realms/eduworld", "https://idp.truong.example"
	extIdentity(ctx, t, e, a1, local, "kc-a1")
	extIdentity(ctx, t, e, a2, local, "kc-a2")
	extIdentity(ctx, t, e, a2, schoolIdP, "school-a2")
	conn := ids.New()
	if _, err := e.platform.ExecContext(ctx, `INSERT INTO sis.sis_connections (sis_connection_id, tenant_id, created_at, updated_at, provider_kind, status)
		VALUES ($1, $2, now(), now(), 'csv_import', 'active')`, conn, a.tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := e.platform.ExecContext(ctx, `INSERT INTO identity.sis_identity_links (link_id, tenant_id, created_at, updated_at, user_id, school_id,
		sis_connection_id, student_external_id, linked_at) VALUES ($1, $2, now(), now(), $3, $4, $5, 'HS-A2', now())`, ids.New(), a.tenant, a2, a.school, conn); err != nil {
		t.Fatal(err)
	}
	requested := "merge_requested"
	issue := mergeIssue(ctx, t, e, a, &requested)
	notRequested := mergeIssue(ctx, t, e, a, nil)

	merge := func(key string, body map[string]any, tok string, hdr map[string]string) httpResp {
		h := map[string]string{"Idempotency-Key": key}
		for k, v := range hdr {
			h[k] = v
		}
		return p.do(t, "POST", "/admin/users:merge", tok, h, body)
	}
	body := map[string]any{"from_user_id": a2, "into_user_id": a1, "reconciliation_issue_id": issue, "reason": "cùng một học sinh, hai tài khoản"}

	// AC-30: School Admin (direct call), content_admin, missing MFA → 403; a user outside the tenant → 404.
	if r := merge(newKey(), body, sch, inA); r.status != 403 {
		t.Fatalf("school admin: %d %s", r.status, r.raw)
	}
	if r := merge(newKey(), body, ca, inA); r.status != 403 {
		t.Fatalf("content admin: %d %s", r.status, r.raw)
	}
	if r := merge(newKey(), body, p.token("sa.mg", false), inA); r.status != 403 || r.body["code"] != "MFA_REQUIRED" {
		t.Fatalf("mfa: %d %s", r.status, r.raw)
	}
	if r := merge(newKey(), map[string]any{"from_user_id": b1, "into_user_id": a1, "reconciliation_issue_id": issue, "reason": "khác tenant"}, sa, inA); r.status != 404 {
		t.Fatalf("other tenant: %d %s", r.status, r.raw)
	}
	authztest.Cover(t, "mergeUsers", authztest.CrossTenant, authztest.RoleDenied)
	// AC-31: same user, issue not proposed → 409; reason too short/long → 422.
	if r := merge(newKey(), map[string]any{"from_user_id": a1, "into_user_id": a1, "reconciliation_issue_id": issue, "reason": "trùng"}, sa, inA); r.status != 409 {
		t.Fatalf("same user: %d %s", r.status, r.raw)
	}
	if r := merge(newKey(), map[string]any{"from_user_id": a2, "into_user_id": a1, "reconciliation_issue_id": notRequested, "reason": "chưa đề nghị"}, sa, inA); r.status != 409 {
		t.Fatalf("issue not proposed: %d %s", r.status, r.raw)
	}
	for _, reason := range []string{"ab", strings.Repeat("x", 1001)} {
		if r := merge(newKey(), map[string]any{"from_user_id": a2, "into_user_id": a1, "reconciliation_issue_id": issue, "reason": reason}, sa, inA); r.status != 422 {
			t.Fatalf("reason %d chars: %d %s", len(reason), r.status, r.raw)
		}
	}

	// AC-27, AC-32: 202 + merge_id; replay → same merge_id; same key other payload → 409 IDEMPOTENCY_KEY_REUSED.
	key := newKey()
	r := merge(key, body, sa, inA)
	if r.status != 202 || r.body["merge_id"] == nil || r.body["status"] != "running" {
		t.Fatalf("merge: %d %s", r.status, r.raw)
	}
	mergeID := r.body["merge_id"].(string)
	if r = merge(key, body, sa, inA); r.status != 202 || r.body["merge_id"] != mergeID {
		t.Fatalf("replay: %d %s", r.status, r.raw)
	}
	other := map[string]any{"from_user_id": a2, "into_user_id": a1, "reconciliation_issue_id": issue, "reason": "lý do khác hẳn"}
	if r = merge(key, other, sa, inA); r.status != 409 || r.body["code"] != "IDEMPOTENCY_KEY_REUSED" {
		t.Fatalf("key reuse: %d %s", r.status, r.raw)
	}
	p.runSagas(ctx, t)
	r = p.do(t, "GET", "/admin/user-merges/"+mergeID, sa, nil, nil)
	if r.status != 200 || r.body["status"] != "succeeded" || len(r.body["steps"].([]any)) != 5 {
		t.Fatalf("getUserMerge: %d %s", r.status, r.raw)
	}
	authztest.Cover(t, "getUserMerge", authztest.RoleDenied)
	// AC-27: identities and SIS link moved, one user_merges row, source soft-deleted (merged), issue closed.
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.user_merges WHERE from_user_id = $1 AND into_user_id = $2`, a2, a1) != 1 ||
		count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE user_id = $1 AND deleted_at IS NOT NULL AND delete_reason = 'merged'`, a2) != 1 ||
		count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE user_id = $1`, a2) != 1 ||
		count(ctx, t, e.platform, `SELECT count(*) FROM identity.external_identities WHERE subject = 'school-a2' AND user_id = $1 AND status = 'active'`, a1) != 1 ||
		count(ctx, t, e.platform, `SELECT count(*) FROM identity.external_identities WHERE subject = 'kc-a2' AND user_id = $1 AND status = 'superseded'`, a1) != 1 ||
		count(ctx, t, e.platform, `SELECT count(*) FROM identity.sis_identity_links WHERE user_id = $1 AND status = 'active'`, a1) != 1 ||
		count(ctx, t, e.platform, `SELECT count(*) FROM sis.reconciliation_issues WHERE issue_id = $1 AND status = 'resolved' AND resolution = 'merged'`, issue) != 1 {
		t.Fatal("identity step of the merge")
	}
	// OQ F045-6: the target keeps its active enrollment, the one of the source ends; the student record of the
	// source is soft-deleted (merged) and its membership is removed (tenant_membership.changed).
	if count(ctx, t, e.platform, `SELECT count(*) FROM org.enrollments WHERE student_id = $1 AND status = 'ended' AND end_date IS NOT NULL`, a2) != 1 ||
		count(ctx, t, e.platform, `SELECT count(*) FROM org.enrollments WHERE student_id = $1 AND status = 'active' AND deleted_at IS NULL`, a1) != 1 ||
		count(ctx, t, e.platform, `SELECT count(*) FROM org.students WHERE student_id = $1 AND deleted_at IS NOT NULL AND delete_reason = 'merged'`, a2) != 1 ||
		outboxCount(ctx, t, p, "tenant_membership.changed", `"removed"`) != 1 || outboxCount(ctx, t, p, "enrollment.changed", `"ended"`) != 1 {
		t.Fatal("enrollment/student/membership of the merged source")
	}
	// AC-28: every module called once with Idempotency-Key = merge_id and the target tenant scope.
	for _, m := range []string{"learning", "game", "competition", "chat"} {
		if owners.calls[m] != 1 || owners.keys[m][0] != mergeID || !owners.scoped[m] {
			t.Fatalf("%s: calls %d keys %v scoped %v", m, owners.calls[m], owners.keys[m], owners.scoped[m])
		}
	}
	// AC-32: audit user.merged in the tenant, no PII (usernames).
	if outboxCount(ctx, t, p, "audit.recorded", `"user.merged"`) != 1 || outboxCount(ctx, t, p, "audit.recorded", "hs.trung.a2") != 0 {
		t.Fatal("audit user.merged without PII")
	}
	// AC-33: the external identity of the source enters the target; the source is in the trash, not restorable.
	u, err := read.PrincipalModel{Session: e.read}.UserBySubject(ctx, schoolIdP, "school-a2")
	if err != nil || u == nil || u.UserID != a1 {
		t.Fatalf("login with the moved identity: %+v %v", u, err)
	}
	if u, _ := (read.PrincipalModel{Session: e.read}).UserByID(ctx, a2); u != nil {
		t.Fatal("merged account still readable")
	}
	r = p.do(t, "GET", "/admin/trash?resource_type=user&tenant_ids="+a.tenant, sa, nil, nil)
	if r.status != 200 || len(r.body["data"].([]any)) != 1 || r.body["data"].([]any)[0].(map[string]any)["restorable"] != false {
		t.Fatalf("trash of merged accounts: %d %s", r.status, r.raw)
	}
	if r = p.do(t, "POST", "/admin/trash/user/"+a2.String()+":restore", sa, map[string]string{"Idempotency-Key": newKey(), "X-Tenant-Id": a.tenant},
		map[string]any{"reason": "thử khôi phục"}); r.status != 409 {
		t.Fatalf("restore merged account: %d %s", r.status, r.raw)
	}
	// AC-31: the source was merged already → 409.
	if r = merge(newKey(), body, sa, inA); r.status != 409 {
		t.Fatalf("merged twice: %d %s", r.status, r.raw)
	}

	// AC-29: a module answers blocked → saga failed with the step error; re-run after the fix does not repeat done steps.
	c1 := enrolled(ctx, t, e.platform, a, "hs.trung.c1")
	c2 := enrolled(ctx, t, e.platform, a, "hs.trung.c2")
	issueC := mergeIssue(ctx, t, e, a, &requested)
	owners.status["game"] = "blocked"
	bodyC := map[string]any{"from_user_id": c2, "into_user_id": c1, "reconciliation_issue_id": issueC, "reason": "trùng tài khoản C"}
	if r = merge(newKey(), bodyC, sa, inA); r.status != 202 {
		t.Fatalf("merge C: %d %s", r.status, r.raw)
	}
	mergeC := r.body["merge_id"].(string)
	p.runSagas(ctx, t)
	r = p.do(t, "GET", "/admin/user-merges/"+mergeC, sa, nil, nil)
	steps := r.body["steps"].([]any)
	if r.body["status"] != "failed" || steps[2].(map[string]any)["service"] != "game" || steps[2].(map[string]any)["status"] != "blocked" {
		t.Fatalf("blocked merge: %s", r.raw)
	}
	learningCalls := owners.calls["learning"]
	owners.status["game"] = "done"
	if r = merge(newKey(), bodyC, sa, inA); r.status != 202 || r.body["merge_id"] != mergeC {
		t.Fatalf("re-run: %d %s", r.status, r.raw)
	}
	p.runSagas(ctx, t)
	if r = p.do(t, "GET", "/admin/user-merges/"+mergeC, sa, nil, nil); r.body["status"] != "succeeded" {
		t.Fatalf("re-run result: %s", r.raw)
	}
	if owners.calls["learning"] != learningCalls || owners.calls["game"] != 3 {
		t.Fatalf("done steps re-run: learning %d→%d game %d", learningCalls, owners.calls["learning"], owners.calls["game"])
	}
	// OQ F045-6: a target whose enrollment ended before the saga ran receives the active enrollment of the source.
	d1 := enrolled(ctx, t, e.platform, a, "hs.trung.d1")
	d2 := enrolled(ctx, t, e.platform, a, "hs.trung.d2")
	issueD := mergeIssue(ctx, t, e, a, &requested)
	if r = merge(newKey(), map[string]any{"from_user_id": d2, "into_user_id": d1, "reconciliation_issue_id": issueD, "reason": "trùng tài khoản D"}, sa, inA); r.status != 202 {
		t.Fatalf("merge D: %d %s", r.status, r.raw)
	}
	if _, err := e.platform.ExecContext(ctx, `UPDATE org.enrollments SET status = 'ended', end_date = current_date WHERE student_id = $1`, d1); err != nil {
		t.Fatal(err)
	}
	p.runSagas(ctx, t)
	if count(ctx, t, e.platform, `SELECT count(*) FROM org.enrollments WHERE student_id = $1 AND status = 'active' AND deleted_at IS NULL`, d1) != 1 ||
		count(ctx, t, e.platform, `SELECT count(*) FROM org.enrollments WHERE student_id = $1`, d2) != 0 ||
		count(ctx, t, e.platform, `SELECT count(*) FROM org.students WHERE student_id = $1 AND delete_reason = 'merged'`, d2) != 1 ||
		outboxCount(ctx, t, p, "enrollment.changed", d1.String()) != 1 {
		t.Fatal("the enrollment of the source moves to a target without one")
	}
	// SC-006: nothing physically removed.
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE user_id IN ($1, $2)`, a2, c2) != 2 {
		t.Fatal("a merged account was removed")
	}
}
