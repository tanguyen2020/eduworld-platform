//go:build integration

package integration

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/seed"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// ExecuteActionsEmail records the action email (TEST DOUBLE, R17).
func (k *kcUsers) ExecuteActionsEmail(_ context.Context, id string, actions ...string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	for _, a := range actions {
		k.roles[id] = append(k.roles[id], "email:"+a)
	}
	return nil
}

// F012-AC-25, AC-26 (TC-AUTH-081): staff created by hand in a tenant.
func TestTenantStaff(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	h, idp := realAuthHTTP(ctx, t, e)
	a := seedTenant(ctx, t, e.platform, "pgd-staff-a")
	b := seedTenant(ctx, t, e.platform, "pgd-staff-b")
	now := time.Now()
	admin := func(name, tenant string) string {
		t.Helper()
		u := staffUser(ctx, t, e, name, &tenant)
		if _, err := e.platform.ExecContext(ctx, `INSERT INTO identity.external_identities (external_identity_id, created_at, updated_at, issuer, subject, user_id)
			VALUES ($1, now(), now(), $2, $3, $4)`, uuid.New(), idp.Issuer(), "kc-"+name, u); err != nil {
			t.Fatal(err)
		}
		grantRole(ctx, t, e, u, "tenant_admin", &tenant, "tenant", nil, "NULL")
		return idp.Token("kc-"+name, httpxtest.PortalClient, now)
	}
	tok, tokB := admin("ta.staff", a.tenant), admin("tb.staff", b.tenant)
	newKey := func() map[string]string {
		return map[string]string{"X-Tenant-Id": a.tenant, "Idempotency-Key": ids.New().String()}
	}
	roles := []map[string]any{{"role_id": seed.RoleID("academic_manager").String(), "scope_type": "tenant"}}
	staff := func(username, email string, r []map[string]any) map[string]any {
		m := map[string]any{"username": username, "display_name": "Cô Lan", "user_type": "staff", "initial_user_roles": r}
		if email != "" {
			m["email"] = email
		}
		return m
	}

	// AC-25: Keycloak user + staff with the session tenant as home tenant + initial role, audited.
	createKey := newKey()
	r := h.do(t, "POST", "/tenant/users", tok, createKey, staff("GV.Lan", "Lan.GV@School.vn", roles))
	if r.status != 201 || r.body["username"] != "gv.lan" || r.body["email"] != "lan.gv@school.vn" || r.body["primary_tenant_id"] != a.tenant {
		t.Fatalf("create: %d %s", r.status, r.raw)
	}
	// R10: the same Idempotency-Key replays the stored response instead of running the use case again (no 409).
	if rr := h.do(t, "POST", "/tenant/users", tok, createKey, staff("GV.Lan", "Lan.GV@School.vn", roles)); rr.status != 201 ||
		rr.body["user_id"] != r.body["user_id"] || rr.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d %s", rr.status, rr.raw)
	}
	id, version := r.body["user_id"].(string), int(r.body["version"].(float64))
	kcID := h.kc.users["gv.lan"]
	if kcID == "" || !strings.Contains(strings.Join(h.kc.roles[kcID], ","), "email:UPDATE_PASSWORD") {
		t.Fatalf("keycloak user %q actions %v", kcID, h.kc.roles)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.user_roles ur JOIN identity.roles r ON r.role_id = ur.role_id
		WHERE ur.user_id = $1 AND r.code = 'academic_manager' AND ur.tenant_id = $2 AND ur.deleted_at IS NULL`, id, a.tenant); n != 1 {
		t.Fatalf("initial role = %d", n)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'user.created'`); n != 1 {
		t.Fatalf("audit = %d", n)
	}
	// AC-26: same username (any case) or email of another account → 409; missing email → 422/400.
	if r := h.do(t, "POST", "/tenant/users", tok, newKey(), staff("gv.LAN", "x@school.vn", roles)); r.status != 409 || !strings.Contains(r.raw, "USERNAME_TAKEN") {
		t.Fatalf("username taken: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "POST", "/tenant/users", tok, newKey(), staff("gv.khac", "LAN.gv@school.vn", roles)); r.status != 409 || !strings.Contains(r.raw, "EMAIL_TAKEN") {
		t.Fatalf("email taken: %d %s", r.status, r.raw)
	}
	if r := h.do(t, "POST", "/tenant/users", tok, newKey(), staff("gv.noemail", "", roles)); r.status != 422 && r.status != 400 {
		t.Fatalf("no email: %d %s", r.status, r.raw)
	}
	// A failing initial role rolls everything back and disables the Keycloak user.
	bad := []map[string]any{{"role_id": seed.RoleID("system_admin").String(), "scope_type": "tenant"}}
	if r := h.do(t, "POST", "/tenant/users", tok, newKey(), staff("gv.bad", "bad@school.vn", bad)); r.status != 422 {
		t.Fatalf("bad role: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE username = 'gv.bad'`); n != 0 || !h.kc.disabled[h.kc.users["gv.bad"]] {
		t.Fatalf("rollback: users=%d disabled=%v", n, h.kc.disabled)
	}

	// OQ F012-1: the username never changes (422); an email of another account → 409 EMAIL_TAKEN; a new email is
	// stored with user.email_changed (no address in the event) and the Keycloak worker sets it with emailVerified=false.
	ifMatch := func(v int) map[string]string {
		return map[string]string{"X-Tenant-Id": a.tenant, "If-Match": `W/"` + strconv.Itoa(v) + `"`}
	}
	if r := h.do(t, "PATCH", "/tenant/users/"+id, tok, ifMatch(version), map[string]any{"username": "gv.lan2"}); r.status != 422 {
		t.Fatalf("username change: %d %s", r.status, r.raw)
	}
	if other := h.do(t, "POST", "/tenant/users", tok, newKey(), staff("gv.hoa", "hoa@school.vn", roles)); other.status != 201 {
		t.Fatalf("second staff: %d %s", other.status, other.raw)
	}
	if r := h.do(t, "PATCH", "/tenant/users/"+id, tok, ifMatch(version), map[string]any{"email": "HOA@school.vn"}); r.status != 409 ||
		!strings.Contains(r.raw, "EMAIL_TAKEN") {
		t.Fatalf("email of another account: %d %s", r.status, r.raw)
	}
	r = h.do(t, "PATCH", "/tenant/users/"+id, tok, ifMatch(version), map[string]any{"email": "Lan.Moi@School.vn"})
	if r.status != 200 || r.body["email"] != "lan.moi@school.vn" {
		t.Fatalf("email change: %d %s", r.status, r.raw)
	}
	version = int(r.body["version"].(float64))
	var payload []byte
	if err := e.relay.QueryRowContext(ctx, `SELECT envelope->'payload' FROM infra.outbox_events WHERE event_type = 'user.email_changed'
		AND envelope->'payload'->>'user_id' = $1`, id).Scan(&payload); err != nil {
		t.Fatal("user.email_changed: ", err)
	}
	if strings.Contains(string(payload), "@") {
		t.Fatalf("the email reached the event: %s", payload)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'user.updated' AND envelope->'payload'->'metadata'->>'email_changed' = 'true'`); n != 1 {
		t.Fatalf("audit email_changed = %d", n)
	}
	kc := &kcAccess{disabled: map[string]int{}, enabled: map[string]int{}, logouts: map[string]int{}}
	c := consumer.KeycloakAccess{Access: command.KeycloakAccess{UoW: e.uow, Repo: write.LifecycleRepo{}, KC: kc, Email: kc,
		Issuers: []string{"http://kc/realms/eduworld"}}}
	ev := outbox.Envelope{EventID: uuid.New(), EventType: "user.email_changed", Payload: payload}
	if err := c.Handle(ctx, ev); err != nil {
		t.Fatal(err)
	}
	if kc.emails[kcID] != "lan.moi@school.vn" {
		t.Fatalf("keycloak email: %v (sub %s)", kc.emails, kcID)
	}
	kc.emailConflict = true
	if err := c.Handle(ctx, ev); !errors.Is(err, outbox.ErrPermanent) {
		t.Fatalf("a Keycloak conflict must go to the DLQ: %v", err)
	}
	// The same email again changes nothing on Keycloak.
	r = h.do(t, "PATCH", "/tenant/users/"+id, tok, ifMatch(version), map[string]any{"email": "lan.moi@school.vn"})
	if r.status != 200 {
		t.Fatalf("same email: %d %s", r.status, r.raw)
	}
	version = int(r.body["version"].(float64))
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'user.email_changed'`); n != 1 {
		t.Fatalf("user.email_changed = %d", n)
	}

	// List and update (If-Match); another tenant does not see the account.
	r = h.do(t, "GET", "/tenant/users?user_type=staff&role_code=academic_manager", tok, map[string]string{"X-Tenant-Id": a.tenant}, nil)
	if r.status != 200 || len(r.body["items"].([]any)) != 2 || !strings.Contains(r.raw, `"academic_manager"`) { // gv.lan, gv.hoa
		t.Fatalf("list: %d %s", r.status, r.raw)
	}
	r = h.do(t, "PATCH", "/tenant/users/"+id, tok, map[string]string{"X-Tenant-Id": a.tenant, "If-Match": `W/"` + strconv.Itoa(version) + `"`},
		map[string]any{"display_name": "Cô Lan Anh", "status": "deactivated"})
	if r.status != 200 || r.body["status"] != "deactivated" || r.body["display_name"] != "Cô Lan Anh" {
		t.Fatalf("update: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type IN ('user.status_changed', 'user.access_revoked')
		AND envelope->'payload'->>'user_id' = $1`, id); n != 2 {
		t.Fatalf("status events = %d", n)
	}
	if r := h.do(t, "PATCH", "/tenant/users/"+id, tokB, map[string]string{"X-Tenant-Id": b.tenant, "If-Match": `W/"2"`},
		map[string]any{"display_name": "X"}); r.status != 404 {
		t.Fatalf("other tenant update: %d %s", r.status, r.raw)
	}
}
