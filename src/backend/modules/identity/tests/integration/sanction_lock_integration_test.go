//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// sanctionEnv drives the consumer identity.sanction_enforcer as pkg/outbox does: one transaction in the scope of the
// envelope tenant (processed_events itself is covered by F003).
type sanctionEnv struct {
	e *env
	c consumer.SanctionEnforcer
}

func newSanctionEnv(e *env) sanctionEnv {
	return sanctionEnv{e: e, c: consumer.SanctionEnforcer{Locks: command.SanctionLocks{Repo: write.LifecycleRepo{}, Locks: write.AccountLockRepo{},
		Events: e.deps.Events, Audit: e.deps.Audit}, Audit: e.deps.Audit}}
}

// deliver hands one sanction event of envelope tenant to the consumer (eventID uuid.Nil: a new one).
func (s sanctionEnv) deliver(ctx context.Context, t *testing.T, typ, tenant string, eventID, sanction, user uuid.UUID, sanctionType string,
	caseID *uuid.UUID) error {
	t.Helper()
	if eventID == uuid.Nil {
		eventID = uuid.New()
	}
	tid := uuid.MustParse(tenant)
	now := time.Now().UTC()
	p := map[string]any{"tenant_id": tid, "sanction_id": sanction, "user_id": user, "case_id": caseID, "sanction_type": sanctionType}
	if typ == "sanction.applied" {
		p["starts_at"], p["scope"], p["reason_code"], p["issued_by_kind"], p["source_version"] = now, "all", "moderation", "moderator", now.UnixMicro()
	} else {
		p["lift_reason"], p["lifted_at"], p["source_version"] = "revoked", now, now.UnixMicro()
	}
	raw, _ := json.Marshal(p)
	env := outbox.Envelope{EventID: eventID, EventType: typ, OccurredAt: now, TenantID: &tid, Payload: raw}
	sc, err := authz.EventScope(tid)
	if err != nil {
		t.Fatal(err)
	}
	return s.e.uow.InTenantTx(ctx, sc, func(ctx context.Context) error { return s.c.Handle(ctx, env) })
}

func statusOf(ctx context.Context, t *testing.T, e *env, user uuid.UUID) string {
	t.Helper()
	var s string
	if err := e.platform.QueryRowContext(ctx, `SELECT status FROM identity.users WHERE user_id = $1`, user).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func eventsOf(ctx context.Context, t *testing.T, e *env, typ string, user uuid.UUID) int {
	t.Helper()
	return count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = $1 AND envelope::text LIKE '%' || $2 || '%'`, typ, user.String())
}

func auditActions(ctx context.Context, t *testing.T, e *env, action string) []map[string]any {
	t.Helper()
	rows, err := e.relay.QueryContext(ctx, `SELECT envelope FROM infra.outbox_events WHERE event_type = 'audit.recorded' ORDER BY created_at`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []map[string]any
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var env struct {
			Payload outbox.AuditPayload `json:"payload"`
		}
		if json.Unmarshal(raw, &env) == nil && env.Payload.Action == action {
			m := map[string]any{"actor_type": env.Payload.ActorType, "target_id": env.Payload.TargetID}
			for k, v := range env.Payload.Metadata {
				m[k] = v
			}
			out = append(out, m)
		}
	}
	return out
}

// T011, T013, T014 / F124-AC-01, AC-03, AC-04 (TC-AUTH-006, TC-SVC-003, TC-TEN-001): sanction.applied account_lock adds a
// lock of source moderation_sanction, locks the student and publishes user.status_changed + user.access_revoked with the
// audit user.locked (system actor) in the consumer transaction; the Keycloak access consumer then disables the user and
// ends its sessions; the Principal resolves locked (REST 403 ACCOUNT_LOCKED within the principal cache TTL). Delivered
// twice → one lock; lifted before applied → never locked; another sanction type, a staff account → no change; a student
// of another tenant → no change, audit security.tenant_mismatch (outside the rolled back transaction) and DLQ (permanent).
func TestSanctionLockApplied(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	s := newSanctionEnv(e)
	a := seedTenant(ctx, t, e.platform, "pgd-sl-a")
	b := seedTenant(ctx, t, e.platform, "pgd-sl-b")
	st := enrolled(ctx, t, e.platform, a, "hs.sl.binh")
	linkLocal(ctx, t, e, st, "kc-hs.sl.binh")
	sanction, caseID := uuid.New(), uuid.New()

	// AC-01.
	if err := s.deliver(ctx, t, "sanction.applied", a.tenant, uuid.Nil, sanction, st, "account_lock", &caseID); err != nil {
		t.Fatal(err)
	}
	if statusOf(ctx, t, e, st) != "locked" || count(ctx, t, e.platform, `SELECT count(*) FROM identity.account_locks WHERE user_id = $1
		AND source = 'moderation_sanction' AND source_ref_id = $2 AND case_id = $3 AND released_at IS NULL AND tenant_id = $4`, st, sanction, caseID, a.tenant) != 1 {
		t.Fatal("moderation lock")
	}
	if eventsOf(ctx, t, e, "user.status_changed", st) != 1 || eventsOf(ctx, t, e, "user.access_revoked", st) != 1 {
		t.Fatal("revocation chain events")
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.user_status_history WHERE user_id = $1 AND to_status = 'locked' AND source = 'moderation'`, st) != 1 {
		t.Fatal("status history")
	}
	locked := auditActions(ctx, t, e, "user.locked")
	if len(locked) != 1 || locked[0]["actor_type"] != "system" || locked[0]["reason_code"] != "moderation_sanction" {
		t.Fatalf("audit user.locked: %v", locked)
	}
	kc := &kcAccess{disabled: map[string]int{}, enabled: map[string]int{}, logouts: map[string]int{}}
	access := consumer.KeycloakAccess{Access: command.KeycloakAccess{UoW: e.uow, Repo: write.LifecycleRepo{}, KC: kc, Issuers: []string{localIssuer}}}
	raw, _ := json.Marshal(map[string]any{"user_id": st, "reason": "locked"})
	if err := access.Handle(ctx, outbox.Envelope{EventID: uuid.New(), EventType: "user.access_revoked", Payload: raw}); err != nil ||
		kc.disabled["kc-hs.sl.binh"] != 1 || kc.logouts["kc-hs.sl.binh"] != 1 {
		t.Fatalf("keycloak disable + logout: %v %v", err, kc)
	}
	p, err := query.ResolvePrincipal{Model: read.PrincipalModel{Session: e.read}}.Handle(ctx, query.ResolvePrincipalInput{UserID: &st})
	if err != nil || p.Status != "locked" {
		t.Fatalf("principal: %v %v", p.Status, err)
	}

	// AC-03: same event again, another delivery of the same sanction → still one lock, no new event.
	if err := s.deliver(ctx, t, "sanction.applied", a.tenant, uuid.Nil, sanction, st, "account_lock", &caseID); err != nil {
		t.Fatal(err)
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.account_locks WHERE source_ref_id = $1`, sanction) != 1 ||
		eventsOf(ctx, t, e, "user.access_revoked", st) != 1 {
		t.Fatal("applied twice")
	}
	st2 := enrolled(ctx, t, e.platform, a, "hs.sl.an")
	if _, err := e.platform.ExecContext(ctx, `UPDATE identity.users SET status = 'active' WHERE user_id = $1`, st2); err != nil {
		t.Fatal(err)
	}
	early := uuid.New()
	if err := s.deliver(ctx, t, "sanction.lifted", a.tenant, uuid.Nil, early, st2, "account_lock", nil); err != nil {
		t.Fatal(err)
	}
	if err := s.deliver(ctx, t, "sanction.applied", a.tenant, uuid.Nil, early, st2, "account_lock", nil); err != nil {
		t.Fatal(err)
	}
	if statusOf(ctx, t, e, st2) != "active" || count(ctx, t, e.platform, `SELECT count(*) FROM identity.account_locks WHERE source_ref_id = $1
		AND released_at IS NOT NULL`, early) != 1 {
		t.Fatal("lifted before applied must never lock")
	}

	// AC-04: other sanction type; staff account; student of another tenant (DLQ + audit).
	if err := s.deliver(ctx, t, "sanction.applied", a.tenant, uuid.Nil, uuid.New(), st2, "mute", nil); err != nil || statusOf(ctx, t, e, st2) != "active" {
		t.Fatalf("mute: %v", err)
	}
	staff := staffUser(ctx, t, e, "gv.sl", &a.tenant)
	if err := s.deliver(ctx, t, "sanction.applied", a.tenant, uuid.Nil, uuid.New(), staff, "account_lock", nil); err != nil || statusOf(ctx, t, e, staff) != "active" {
		t.Fatalf("staff: %v", err)
	}
	err = s.deliver(ctx, t, "sanction.applied", b.tenant, uuid.Nil, uuid.New(), st2, "account_lock", nil)
	if !errors.Is(err, outbox.ErrPermanent) || !errors.Is(err, command.ErrSanctionTenantMismatch) || statusOf(ctx, t, e, st2) != "active" {
		t.Fatalf("tenant mismatch: %v", err)
	}
	if mm := auditActions(ctx, t, e, "security.tenant_mismatch"); len(mm) != 1 || mm[0]["target_id"] != st2.String() {
		t.Fatalf("audit security.tenant_mismatch: %v", mm)
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.account_locks WHERE user_id = $1 AND released_at IS NULL`, st2) != 0 {
		t.Fatal("no lock on mismatch")
	}
	// A payload tenant other than the envelope tenant is refused before any read.
	tb := uuid.MustParse(b.tenant)
	raw, _ = json.Marshal(map[string]any{"tenant_id": uuid.MustParse(a.tenant), "sanction_id": uuid.New(), "user_id": st2, "sanction_type": "account_lock"})
	if err := s.c.Handle(ctx, outbox.Envelope{EventID: uuid.New(), EventType: "sanction.applied", TenantID: &tb, Payload: raw}); !errors.Is(err, outbox.ErrPermanent) {
		t.Fatalf("payload tenant: %v", err)
	}
}

// T012, T015, T016 / F124-AC-02, AC-05, AC-06 (TC-AUTH-006, SC-002): the matrix of lock sources × release through the real
// HTTP chain: a school lock plus a moderation lock — sanction.lifted keeps the account locked (no access_restored); the
// School Admin's unlockTenantUser while the moderation lock remains releases the school lock only and answers 409
// ACCOUNT_LOCK_HELD_BY_SAFETY; System Admin unlockUser likewise; sanction.lifted without other lock → active +
// user.access_restored. Staff in scope see lock_sources (no case id, no reason); another tenant gets 404.
func TestSanctionLockSourcesAndUnlock(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	s := newSanctionEnv(e)
	h, idp := realAuthHTTP(ctx, t, e)
	a := seedTenant(ctx, t, e.platform, "pgd-su-a")
	b := seedTenant(ctx, t, e.platform, "pgd-su-b")
	st := enrolled(ctx, t, e.platform, a, "hs.su.binh")
	iss := idp.Issuer()
	portalUser(ctx, t, e, iss, "ad.truong.su", "school_admin", &a.tenant, "school", &a.school)
	portalUser(ctx, t, e, iss, "ad.donvi.su.b", "tenant_admin", &b.tenant, "tenant", nil)
	portalUser(ctx, t, e, iss, "sa.su", "system_admin", nil, "platform", nil)
	now := time.Now()
	tok := func(u string) string { return idp.Token("sub-"+u, httpxtest.PortalClient, now) }
	hdr := func(tenant string) map[string]string {
		return map[string]string{"X-Tenant-Id": tenant, "Idempotency-Key": uuid.Must(uuid.NewV7()).String()}
	}
	reason := map[string]any{"reason": "Nghỉ học tạm thời"}
	path := "/tenant/users/" + st.String()

	// School lock, then the Safety Team lock.
	if r := h.do(t, "POST", path+":lock", tok("ad.truong.su"), hdr(a.tenant), reason); r.status != 204 {
		t.Fatalf("school lock: %d %v", r.status, r.body)
	}
	sanction, caseID := uuid.New(), uuid.New()
	if err := s.deliver(ctx, t, "sanction.applied", a.tenant, uuid.Nil, sanction, st, "account_lock", &caseID); err != nil {
		t.Fatal(err)
	}
	// AC-06: staff see the sources, never the case id or the internal reason.
	r := h.do(t, "GET", "/tenant/users?user_type=student", tok("ad.truong.su"), map[string]string{"X-Tenant-Id": a.tenant}, nil)
	if r.status != 200 {
		t.Fatalf("list: %d %v", r.status, r.body)
	}
	raw, _ := json.Marshal(r.body)
	if !strings.Contains(string(raw), `"lock_sources":["moderation_sanction","school_admin"]`) || strings.Contains(string(raw), caseID.String()) ||
		strings.Contains(string(raw), "Nghỉ học") {
		t.Fatalf("lock_sources: %s", raw)
	}
	// AC-02: the sanction is lifted while the school lock remains → still locked, no access_restored.
	if err := s.deliver(ctx, t, "sanction.lifted", a.tenant, uuid.Nil, sanction, st, "account_lock", &caseID); err != nil {
		t.Fatal(err)
	}
	if statusOf(ctx, t, e, st) != "locked" || eventsOf(ctx, t, e, "user.access_restored", st) != 0 {
		t.Fatal("school lock must keep the account locked")
	}
	// A second sanction; AC-05: the school unlocks → its lock released, 409 ACCOUNT_LOCK_HELD_BY_SAFETY, still locked.
	sanction2 := uuid.New()
	if err := s.deliver(ctx, t, "sanction.applied", a.tenant, uuid.Nil, sanction2, st, "account_lock", nil); err != nil {
		t.Fatal(err)
	}
	if r := h.do(t, "POST", path+":unlock", tok("ad.truong.su"), hdr(a.tenant), reason); r.status != 409 || r.body["code"] != "ACCOUNT_LOCK_HELD_BY_SAFETY" {
		t.Fatalf("unlock while held by safety: %d %v", r.status, r.body)
	}
	if statusOf(ctx, t, e, st) != "locked" || count(ctx, t, e.platform, `SELECT count(*) FROM identity.account_locks WHERE user_id = $1
		AND source = 'school_admin' AND released_at IS NULL`, st) != 0 {
		t.Fatal("the school lock is released, the account stays locked")
	}
	if r := h.do(t, "POST", "/admin/users/"+st.String()+":unlock", tok("sa.su"), hdr(a.tenant), reason); r.status != 409 || r.body["code"] != "ACCOUNT_LOCK_HELD_BY_SAFETY" {
		t.Fatalf("system admin unlock while held by safety: %d %v", r.status, r.body)
	}
	if r := h.do(t, "POST", path+":unlock", tok("ad.donvi.su.b"), hdr(b.tenant), reason); r.status != 404 {
		t.Fatalf("other tenant: %d %v", r.status, r.body)
	}
	// AC-02: last lock lifted → active + user.access_restored + audit user.unlocked.
	if err := s.deliver(ctx, t, "sanction.lifted", a.tenant, uuid.Nil, sanction2, st, "account_lock", nil); err != nil {
		t.Fatal(err)
	}
	if statusOf(ctx, t, e, st) != "active" || eventsOf(ctx, t, e, "user.access_restored", st) != 1 || len(auditActions(ctx, t, e, "user.unlocked")) != 1 {
		t.Fatal("restored")
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.account_locks WHERE user_id = $1 AND released_at IS NULL`, st) != 0 {
		t.Fatal("no lock left")
	}
	// Moderation lock alone + school unlock (no school lock) → 409, nothing released.
	sanction3 := uuid.New()
	if err := s.deliver(ctx, t, "sanction.applied", a.tenant, uuid.Nil, sanction3, st, "account_lock", nil); err != nil {
		t.Fatal(err)
	}
	if r := h.do(t, "POST", path+":unlock", tok("ad.truong.su"), hdr(a.tenant), reason); r.status != 409 || r.body["code"] != "ACCOUNT_LOCK_HELD_BY_SAFETY" {
		t.Fatalf("unlock of a moderation lock only: %d %v", r.status, r.body)
	}
	if statusOf(ctx, t, e, st) != "locked" {
		t.Fatal("still locked")
	}
}
