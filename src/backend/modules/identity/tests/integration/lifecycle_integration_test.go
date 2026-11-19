//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	portwrite "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/settingscfg"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/consumer"
	identityhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
)

// revokeRecorder is a TEST DOUBLE of command.GameAuthSessionRevoker (the real one is covered by F013 tests).
type revokeRecorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *revokeRecorder) RevokeSessions(_ context.Context, f portwrite.SessionFilter, reason string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, f.UserID.String()+":"+reason)
	return 1, nil
}

// kcAccess is a TEST DOUBLE of port.KeycloakAccessAdmin.
type kcAccess struct{ disabled, enabled, logouts map[string]int }

func (k *kcAccess) DisableUser(_ context.Context, id string) error { k.disabled[id]++; return nil }
func (k *kcAccess) EnableUser(_ context.Context, id string) error  { k.enabled[id]++; return nil }
func (k *kcAccess) LogoutUser(_ context.Context, id string) error  { k.logouts[id]++; return nil }

const localIssuer = "http://kc/realms/eduworld"

func lifecycleHandlers(e *env, kc *kcUsers, rec *revokeRecorder, codec *page.Codec) identityhttp.LifecycleHandlers {
	defaults, err := settingscfg.DefaultsFrom(config.Load())
	if err != nil {
		panic(err)
	}
	return identityhttp.LifecycleHandlers{
		Lifecycle: command.UserLifecycle{UoW: e.uow, Repo: write.LifecycleRepo{}, Events: e.deps.Events, Audit: e.deps.Audit},
		ResetPassword: command.ResetStudentPassword{UoW: e.uow, Repo: write.LifecycleRepo{}, KC: kc, Sessions: rec, Events: e.deps.Events,
			Audit: e.deps.Audit, Issuers: []string{localIssuer}},
		// The HTTP side of cleanup (preview, exclusions, runs, approve); the run itself is driven by TestStudentCleanup.
		Cleanup: command.StudentCleanup{UoW: e.uow, Repo: write.CleanupRepo{}, Settings: write.SettingsRepo{}, Defaults: defaults,
			Lifecycle: write.LifecycleRepo{}, Events: e.deps.Events, Audit: e.deps.Audit, Cursors: codec},
	}
}

func linkLocal(ctx context.Context, t *testing.T, e *env, user uuid.UUID, sub string) {
	t.Helper()
	if _, err := e.platform.ExecContext(ctx, `INSERT INTO identity.external_identities (external_identity_id, created_at, updated_at, issuer, subject, user_id)
		VALUES ($1, now(), now(), $2, $3, $4)`, uuid.New(), localIssuer, sub, user); err != nil {
		t.Fatal(err)
	}
}

// F016 US1: F016-AC-01, AC-03..AC-06, AC-25..AC-27.
func TestUserLifecycle(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	h, idp := realAuthHTTP(ctx, t, e)
	a := seedTenant(ctx, t, e.platform, "pgd-lc-a")
	b := seedTenant(ctx, t, e.platform, "pgd-lc-b")
	st := enrolled(ctx, t, e.platform, a, "hs.lc")
	linkLocal(ctx, t, e, st, "kc-hs.lc")
	h.kc.users["hs.lc"] = "kc-hs.lc"
	tx, _ := e.platform.BeginTx(ctx, nil)
	other, err := insertStudent(ctx, tx, fixture{tenant: a.tenant, school: a.otherSchool}, "hs.lc.khac")
	if err != nil {
		t.Fatal(err)
	}
	_ = tx.Commit()
	stB := enrolled(ctx, t, e.platform, b, "hs.lc.b")
	staff := staffUser(ctx, t, e, "gv.lc", &a.tenant)
	iss := idp.Issuer()
	portalUser(ctx, t, e, iss, "ad.truong.lc", "school_admin", &a.tenant, "school", &a.school)
	portalUser(ctx, t, e, iss, "ad.donvi.lc", "tenant_admin", &a.tenant, "tenant", nil)
	portalUser(ctx, t, e, iss, "sa.lc", "system_admin", nil, "platform", nil)
	now := time.Now()
	tok := func(u string, mfa bool) string { return idp.Token("sub-"+u, httpxtest.PortalClient, mfa, now) }
	hdr := func() map[string]string {
		return map[string]string{"X-Tenant-Id": a.tenant, "Idempotency-Key": uuid.Must(uuid.NewV7()).String()}
	}
	body := map[string]any{"reason": "Nghỉ học tạm thời"}
	events := func(typ string) int {
		return count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = $1 AND envelope::text LIKE '%' || $2 || '%'`, typ, st.String())
	}

	// AC-01: lock → locked, history, audit, status_changed + access_revoked; a second lock is a no-op.
	if r := h.do(t, "POST", "/tenant/users/"+st.String()+":lock", tok("ad.truong.lc", false), hdr(), body); r.status != 204 {
		t.Fatalf("lock: %d %v", r.status, r.body)
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE user_id = $1 AND status = 'locked'`, st) != 1 ||
		count(ctx, t, e.platform, `SELECT count(*) FROM identity.user_status_history WHERE user_id = $1 AND to_status = 'locked' AND reason IS NOT NULL`, st) != 1 ||
		events("user.status_changed") != 1 || events("user.access_revoked") != 1 {
		t.Fatal("lock effects")
	}
	if r := h.do(t, "POST", "/tenant/users/"+st.String()+":lock", tok("ad.truong.lc", false), hdr(), body); r.status != 204 || events("user.access_revoked") != 1 {
		t.Fatalf("second lock: %d", r.status)
	}
	// AC-05: other school → 403; other tenant → 404; staff → 403; no reason → 422.
	if r := h.do(t, "POST", "/tenant/users/"+other+":lock", tok("ad.truong.lc", false), hdr(), body); r.status != 403 {
		t.Fatalf("other school: %d", r.status)
	}
	if r := h.do(t, "POST", "/tenant/users/"+stB.String()+":lock", tok("ad.truong.lc", false), hdr(), body); r.status != 404 {
		t.Fatalf("other tenant: %d", r.status)
	}
	if r := h.do(t, "POST", "/tenant/users/"+staff.String()+":lock", tok("ad.donvi.lc", false), hdr(), body); r.status != 403 {
		t.Fatalf("staff: %d", r.status)
	}
	if r := h.do(t, "POST", "/tenant/users/"+st.String()+":unlock", tok("ad.donvi.lc", false), hdr(), map[string]any{}); r.status != 422 {
		t.Fatalf("no reason: %d", r.status)
	}
	// AC-04: unlock → active + access_restored.
	if r := h.do(t, "POST", "/tenant/users/"+st.String()+":unlock", tok("ad.donvi.lc", false), hdr(), body); r.status != 204 || events("user.access_restored") != 1 {
		t.Fatalf("unlock: %d", r.status)
	}
	// System Admin deactivates (target tenant in X-Tenant-Id).
	if r := h.do(t, "POST", "/admin/users/"+stB.String()+":deactivate", tok("sa.lc", true), map[string]string{"X-Tenant-Id": b.tenant,
		"Idempotency-Key": uuid.Must(uuid.NewV7()).String()}, body); r.status != 204 {
		t.Fatalf("deactivate: %d %v", r.status, r.body)
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE user_id = $1 AND status = 'deactivated' AND deactivated_at IS NOT NULL`, stB) != 1 {
		t.Fatal("deactivated")
	}
	// Tenant admin cannot use the platform route.
	if r := h.do(t, "POST", "/admin/users/"+st.String()+":lock", tok("ad.donvi.lc", false), hdr(), body); r.status != 403 {
		t.Fatalf("tenant admin on /admin: %d", r.status)
	}

	// AC-03, AC-04, AC-26: the Keycloak access consumer.
	kc := &kcAccess{disabled: map[string]int{}, enabled: map[string]int{}, logouts: map[string]int{}}
	c := consumer.KeycloakAccess{Access: command.KeycloakAccess{UoW: e.uow, Repo: write.LifecycleRepo{}, KC: kc, Issuers: []string{localIssuer}}}
	env := func(typ, reason string) outbox.Envelope {
		raw, _ := json.Marshal(map[string]any{"user_id": st, "reason": reason})
		return outbox.Envelope{EventID: uuid.New(), EventType: typ, Payload: raw}
	}
	if err := c.Handle(ctx, env("user.access_revoked", "locked")); err != nil || kc.disabled["kc-hs.lc"] != 1 || kc.logouts["kc-hs.lc"] != 1 {
		t.Fatalf("revoked: %v %v", err, kc)
	}
	if err := c.Handle(ctx, env("user.access_revoked", "password_changed")); err != nil || kc.disabled["kc-hs.lc"] != 1 {
		t.Fatal("password_changed must not disable the user")
	}
	if err := c.Handle(ctx, env("user.access_restored", "unlocked")); err != nil || kc.enabled["kc-hs.lc"] != 1 {
		t.Fatal("restored")
	}

	// AC-25..AC-27: reset password by the School Admin; replay without password; other school/tenant refused.
	key := uuid.Must(uuid.NewV7()).String()
	rh := map[string]string{"X-Tenant-Id": a.tenant, "Idempotency-Key": key}
	r := h.do(t, "POST", "/tenant/students/"+st.String()+"/password:reset", tok("ad.truong.lc", false), rh, map[string]any{"reason": "Quên mật khẩu"})
	if r.status != 200 || r.body["initial_password"] == nil || r.body["password_temporary"] != false || r.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("reset: %d %v", r.status, r.body)
	}
	if _, ok := h.kc.temporary["kc-hs.lc"]; !ok || len(h.revoker.calls) != 1 || h.revoker.calls[0] != st.String()+":password_changed" {
		t.Fatalf("reset effects: %v %v", h.kc.temporary, h.revoker.calls)
	}
	if count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'user.access_revoked' AND envelope::text LIKE '%password_changed%'`) != 1 {
		t.Fatal("access_revoked password_changed")
	}
	r = h.do(t, "POST", "/tenant/students/"+st.String()+"/password:reset", tok("ad.truong.lc", false), rh, map[string]any{"reason": "Quên mật khẩu"})
	if r.status != 200 || r.body["initial_password"] != nil || r.body["password_redacted"] != true || len(h.revoker.calls) != 1 {
		t.Fatalf("replay: %d %v", r.status, r.body)
	}
	if r := h.do(t, "POST", "/tenant/students/"+other+"/password:reset", tok("ad.truong.lc", false), hdr(), map[string]any{"reason": "Thử"}); r.status != 403 {
		t.Fatalf("reset other school: %d", r.status)
	}
	if r := h.do(t, "POST", "/tenant/students/"+stB.String()+"/password:reset", tok("ad.donvi.lc", false), hdr(), map[string]any{"reason": "Thử"}); r.status != 404 {
		t.Fatalf("reset other tenant: %d", r.status)
	}
}
