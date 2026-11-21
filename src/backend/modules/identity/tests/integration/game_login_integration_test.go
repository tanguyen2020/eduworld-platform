//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	portwrite "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/gamelogin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/gameauth"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/keycloak"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	identityhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

const (
	gamePw       = "Mat-khau-hs-2026" // test password: must never appear in logs (F013-AC-16 is checked in tests/security)
	commonSchool = "0193a000-0000-7000-8000-000000000002"
)

// extraMembership is a TEST DOUBLE around the real LoginMembershipReader: it adds one membership. A student
// cannot hold active enrollments in two tenants in the data model (org.students PK, enrollments_one_active_uq,
// OQ F013-2), so the multi-tenant path (US2) is exercised by offering a second tenant here.
type extraMembership struct {
	real  portwrite.LoginMembershipReader
	extra *gamelogin.Membership
}

func (x *extraMembership) LoginMemberships(ctx context.Context, id uuid.UUID) ([]gamelogin.Membership, error) {
	ms, err := x.real.LoginMemberships(ctx, id)
	if err == nil && x.extra != nil && len(ms) > 0 {
		ms = append(ms, *x.extra)
	}
	return ms, err
}

type gameIntegration struct {
	*env
	kc      *fakeKeycloak
	vk      valkey.Client
	srv     *httptest.Server
	auth    *command.GameAuth
	metrics *gameauth.Metrics
	members *extraMembership
	deps    httpx.Deps // to build more servers over the same use cases (security test)
	handler http.Handler
}

type gameOpts struct {
	limits command.GameRateLimits
	curfew gamelogin.Curfew
}

func newGameIntegration(ctx context.Context, t *testing.T, o gameOpts) *gameIntegration {
	t.Helper()
	e := setup(ctx, t)
	_, addr := testkit.StartValkey(ctx, t)
	vk, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}, DisableCache: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vk.Close)
	kc := &fakeKeycloak{users: map[string]fakeKCUser{}, refresh: map[string]string{}, sidUser: map[string]string{},
		sidRoles: map[string][]string{}, actions: map[string][]string{}, sidClient: map[string]string{}}
	kcSrv := httptest.NewServer(kc)
	t.Cleanup(kcSrv.Close)
	tokens, err := keycloak.NewPortalTokens(keycloak.PortalConfig{TokenURL: kcSrv.URL + "/realms/eduworld/protocol/openid-connect/token",
		LogoutURL: kcSrv.URL + "/realms/eduworld/protocol/openid-connect/logout", ClientID: "eduworld-game-backend", ClientSecret: "game-secret",
		Timeout: 2 * time.Second, MaxRetries: 1, FailureRatio: 1}, kcSrv.Client())
	if err != nil {
		t.Fatal(err)
	}
	admin, err := keycloak.New(keycloak.Config{BaseURL: kcSrv.URL + "/admin/realms/eduworld", ClientID: "svc-identity", ClientSecret: "svc-secret",
		Timeout: 2 * time.Second, MaxRetries: 1, FailureRatio: 0.9}, kcSrv.Client())
	if err != nil {
		t.Fatal(err)
	}
	tickets, err := cache.NewLoginTickets(vk, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	m, _ := gameauth.NewMetrics(nil)
	members := &extraMembership{real: read.LoginMembershipReader{Session: e.read}}
	limits := o.limits
	if limits == (command.GameRateLimits{}) {
		limits = command.GameRateLimits{Global: command.Limit{N: 1000, Window: time.Second}, PerDevice: command.Limit{N: 100, Window: time.Minute},
			PerUsername: command.Limit{N: 100, Window: time.Minute}, RefreshPerSession: command.Limit{N: 100, Window: time.Minute}}
	}
	auth := &command.GameAuth{UoW: e.uow, Accounts: write.GameLoginRepo{}, Memberships: members, Sessions: write.GameAuthSessionRepo{},
		Bootstrap: read.BootstrapReader{Session: e.read}, Events: e.deps.Events, Audit: e.deps.Audit, IdP: tokens, KC: admin,
		Tickets: tickets, Limiter: cache.NewFallbackLimiter(cache.RateLimiter{C: vk}), Curfew: gameauth.PlatformCurfew{Curfew: o.curfew},
		Metrics: m, Cfg: command.GameAuthConfig{MinClientVersion: "1.2.0", Limits: limits}}
	spec, err := httpx.LoadSpec(ctx, filepath.Join(testkit.RepoRoot(t), "specs", "openapi", "eduworld-api.v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	idp := httpxtest.NewIDP(t)
	s, err := httpx.NewServer(httpx.Deps{Auth: idp.AuthConfig(), Keys: idp.JWKS(t), Resolver: &httpxtest.Resolver{}, Spec: spec})
	if err != nil {
		t.Fatal(err)
	}
	g := &gameIntegration{env: e, kc: kc, vk: vk, auth: auth, metrics: m, members: members}
	if err := identityhttp.RegisterGameAuth(s, identityhttp.GameAuthHandlers{Session: auth, IPHashKey: []byte("ip-hash-key")}); err != nil {
		t.Fatal(err)
	}
	g.deps, g.handler = httpx.Deps{Auth: idp.AuthConfig(), Keys: idp.JWKS(t), Resolver: &httpxtest.Resolver{}, Spec: spec}, s
	g.srv = httptest.NewServer(s)
	t.Cleanup(g.srv.Close)
	return g
}

// student creates an enrolled student of f (status pending, external identity at the fake realm) and its
// Keycloak account.
func (g *gameIntegration) student(ctx context.Context, t *testing.T, f fixture, username string) uuid.UUID {
	t.Helper()
	id := enrolled(ctx, t, g.platform, f, username)
	g.identity(ctx, t, id, username)
	return id
}

func (g *gameIntegration) identity(ctx context.Context, t *testing.T, id uuid.UUID, username string) {
	t.Helper()
	if _, err := g.platform.ExecContext(ctx, `INSERT INTO identity.external_identities (external_identity_id, created_at, updated_at, issuer, subject, user_id)
		VALUES ($1, now(), now(), $2, $3, $4)`, uuid.New(), kcIssuer, "kc-"+username, id); err != nil {
		t.Fatal(err)
	}
	g.kc.mu.Lock()
	g.kc.users[username] = fakeKCUser{password: gamePw, student: true}
	g.kc.mu.Unlock()
}

type gameResp struct {
	code   int
	header http.Header
	body   map[string]any
}

func (g *gameIntegration) post(t *testing.T, path string, body any, hdr map[string]string) gameResp {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, g.srv.URL+"/api/v1"+path, strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	r := gameResp{code: resp.StatusCode, header: resp.Header}
	_ = json.Unmarshal(raw, &r.body)
	return r
}

func gameLogin(user string, device uuid.UUID) map[string]any {
	return map[string]any{"username": user, "password": gamePw, "device_id": device.String(), "client_version": "1.4.0", "platform": "windows"}
}

func (g *gameIntegration) login(t *testing.T, user string, device uuid.UUID) gameResp {
	t.Helper()
	return g.post(t, "/auth/game/login", gameLogin(user, device), nil)
}

func (g *gameIntegration) refresh(t *testing.T, rt string, device uuid.UUID, tenant, mode, version string) gameResp {
	t.Helper()
	body := map[string]any{"refresh_token": rt, "device_id": device.String(), "mode": mode}
	if version != "" {
		body["client_version"] = version
	}
	return g.post(t, "/auth/game/token:refresh", body, map[string]string{"X-Tenant-Id": tenant})
}

func (g *gameIntegration) tokenCalls() int {
	g.kc.mu.Lock()
	defer g.kc.mu.Unlock()
	return g.kc.seq
}

func codeOf(r gameResp) string {
	c, _ := r.body["code"].(string)
	return c
}

func tokensOf(t *testing.T, r gameResp) map[string]any {
	t.Helper()
	tok, ok := r.body["tokens"].(map[string]any)
	if !ok {
		t.Fatalf("no tokens: %d %v", r.code, r.body)
	}
	return tok
}

func bypassCount() float64 {
	return testutil.ToFloat64(tenancy.BypassCounter().WithLabelValues("identity", string(tenancy.ReasonAuthLogin)))
}

// F013 US1: F013-AC-01..AC-08, AC-30, AC-31 (with the real repositories, Valkey and a fake Keycloak).
func TestGameLogin(t *testing.T) {
	ctx := context.Background()
	g := newGameIntegration(ctx, t, gameOpts{})
	a := seedTenant(ctx, t, g.platform, "pgd-game-a")
	an := g.student(ctx, t, a, "hs.an.3a")
	dev := uuid.New()

	// AC-06: unknown fields (tenant_id, takeover), username with @, too long → 422 without calling Keycloak.
	calls := g.tokenCalls()
	for _, body := range []map[string]any{
		func() map[string]any { b := gameLogin("hs.an.3a", dev); b["tenant_id"] = a.tenant; return b }(),
		func() map[string]any { b := gameLogin("hs.an.3a", dev); b["takeover"] = true; return b }(),
		gameLogin("hs.an@school.vn", dev),
		gameLogin(strings.Repeat("x", 129), dev),
	} {
		if r := g.post(t, "/auth/game/login", body, nil); r.code != 422 || codeOf(r) != string(problem.ValidationFailed) {
			t.Fatalf("validation: %d %v", r.code, r.body)
		}
	}
	if g.tokenCalls() != calls {
		t.Fatal("Keycloak called for an invalid body")
	}

	// AC-17, AC-31: wrong password and unknown user answer alike; no membership read; journal row without
	// tenant written by identity_platform.
	before := bypassCount()
	wrong := gameLogin("hs.an.3a", dev)
	wrong["password"] = "sai-mat-khau"
	r1 := g.post(t, "/auth/game/login", wrong, nil)
	r2 := g.post(t, "/auth/game/login", gameLogin("hs.khong.co", dev), nil)
	if r1.code != 401 || codeOf(r1) != string(problem.InvalidCredentials) || r2.code != 401 || codeOf(r2) != codeOf(r1) || r1.body["detail"] != r2.body["detail"] {
		t.Fatalf("uniform credentials error: %v / %v", r1.body, r2.body)
	}
	if bypassCount() != before {
		t.Fatal("memberships read before Keycloak accepted the password")
	}
	if n := count(ctx, t, g.platform, `SELECT count(*) FROM identity.login_events WHERE result = 'invalid_credentials' AND tenant_id IS NULL
		AND client = 'eduworld-game-backend' AND ip_hash IS NOT NULL`); n != 2 {
		t.Fatalf("invalid_credentials journal rows = %d", n)
	}

	// AC-01, AC-02, AC-07, AC-30: success → tokens + bootstrap only; pending → active + user.activated; journal.
	r := g.login(t, "hs.an.3a", dev)
	if r.code != 200 {
		t.Fatalf("login: %d %v", r.code, r.body)
	}
	if _, has := r.body["game_session_id"]; has || len(r.body) != 2 {
		t.Fatalf("response keys: %v", r.body)
	}
	tok := tokensOf(t, r)
	if tok["token_type"] != "Bearer" || tok["refresh_token"] == "" {
		t.Fatalf("tokens: %v", tok)
	}
	b := r.body["bootstrap"].(map[string]any)
	if ff, _ := b["feature_flags"].(map[string]any); ff["social.friends_enabled"] != true {
		t.Fatalf("bootstrap feature flags: %v", b["feature_flags"])
	}
	school, _ := b["school"].(map[string]any)
	student, _ := b["student"].(map[string]any)
	if b["tenant_id"] != a.tenant || b["tenant_type"] != "unit" || b["is_common_tenant"] != false || b["grade_band"] != "g1_3" ||
		school["school_id"] != a.school || school["province"] == nil || student["grade_level"] != float64(3) {
		t.Fatalf("bootstrap: %v", b)
	}
	// ADR-0095: no class in the bootstrap (no class, class_id or class_name anywhere).
	if raw, _ := json.Marshal(b); b["class"] != nil || strings.Contains(string(raw), "class_id") || strings.Contains(string(raw), "class_name") {
		t.Fatalf("bootstrap still has a class: %s", raw)
	}
	if u := b["user"].(map[string]any); u["user_id"] != an.String() || u["status"] != "active" || u["user_type"] != "student" {
		t.Fatalf("bootstrap user: %v", u)
	}
	if bypassCount() != before+1 {
		t.Fatalf("auth_login bypass count = %v", bypassCount()-before)
	}
	if n := count(ctx, t, g.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope::text LIKE '%cross_tenant.accessed%' AND envelope::text LIKE '%auth_login%'`); n < 1 {
		t.Fatal("cross_tenant.accessed audit for auth_login missing")
	}
	if n := count(ctx, t, g.platform, `SELECT count(*) FROM identity.users WHERE user_id = $1 AND status = 'active'`, an); n != 1 {
		t.Fatal("user not activated")
	}
	if n := count(ctx, t, g.platform, `SELECT count(*) FROM identity.external_identities WHERE user_id = $1 AND last_login_at IS NOT NULL`, an); n != 1 {
		t.Fatal("last_login_at not set")
	}
	if n := count(ctx, t, g.platform, `SELECT count(*) FROM identity.login_events WHERE user_id = $1 AND result = 'success'
		AND tenant_id = $2 AND tenant_resolution = 'single_membership' AND device_id = $3`, an, a.tenant, dev); n != 1 {
		t.Fatal("success journal row")
	}
	// AC-15: only the SHA-256 of the refresh token is stored.
	rt := tok["refresh_token"].(string)
	if n := count(ctx, t, g.platform, `SELECT count(*) FROM identity.game_auth_sessions WHERE user_id = $1 AND refresh_token_hash = $2
		AND device_id = $3 AND platform = 'windows' AND revoked_at IS NULL`, an, gamelogin.HashToken(rt), dev); n != 1 {
		t.Fatal("game auth session")
	}
	if n := count(ctx, t, g.platform, `SELECT count(*) FROM identity.game_auth_sessions WHERE previous_token_hashes::text LIKE '%' || $1 || '%'
		OR refresh_token_hash = $1`, rt); n != 0 {
		t.Fatal("raw refresh token stored")
	}
	// Second login: no second user.activated.
	if r := g.login(t, "hs.an.3a", uuid.New()); r.code != 200 {
		t.Fatalf("second login: %d %v", r.code, r.body)
	}
	if n := count(ctx, t, g.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'user.activated' AND envelope::text LIKE '%' || $1 || '%'`, an.String()); n != 1 {
		t.Fatalf("user.activated events = %d", n)
	}

	// AC-03: Keycloak account without EduWorld identity → ACCOUNT_NOT_PROVISIONED, Keycloak session ended, audit.
	g.kc.mu.Lock()
	g.kc.users["hs.nosis"] = fakeKCUser{password: gamePw, student: true}
	logouts := g.kc.logouts
	g.kc.mu.Unlock()
	if r := g.login(t, "hs.nosis", dev); r.code != 403 || codeOf(r) != string(problem.AccountNotProvisioned) {
		t.Fatalf("nosis: %d %v", r.code, r.body)
	}
	g.kc.mu.Lock()
	if g.kc.logouts != logouts+1 {
		t.Fatal("Keycloak session of nosis not revoked")
	}
	g.kc.mu.Unlock()
	if n := count(ctx, t, g.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope::text LIKE '%auth.account_not_provisioned%'`); n != 1 {
		t.Fatal("account_not_provisioned audit")
	}

	// AC-04: staff → GAME_ACCESS_STUDENT_ONLY, whether Keycloak refuses (flow) or EduWorld does (user_type).
	staff := staffUser(ctx, t, g.env, "gv.an", &a.tenant)
	_ = staff
	g.kc.mu.Lock()
	g.kc.users["gv.an"] = fakeKCUser{password: gamePw}                  // Keycloak flow denies non-students
	g.kc.users["gv.binh"] = fakeKCUser{password: gamePw, student: true} // flow passes, EduWorld refuses
	g.kc.mu.Unlock()
	staffUser(ctx, t, g.env, "gv.binh", &a.tenant)
	for _, u := range []string{"gv.an", "gv.binh"} {
		if r := g.login(t, u, dev); r.code != 403 || codeOf(r) != string(problem.GameAccessStudentOnly) {
			t.Fatalf("%s: %d %v", u, r.code, r.body)
		}
	}
	if n := count(ctx, t, g.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope::text LIKE '%auth.game_access_denied_non_student%'`); n != 2 {
		t.Fatalf("non-student audits = %d", n)
	}

	// AC-05: locked, deactivated, no effective school/grade (student inactive — ADR-0095: NO_ACTIVE_ENROLLMENT keeps its code).
	locked := g.student(ctx, t, a, "hs.khoa")
	deact := g.student(ctx, t, a, "hs.nghi")
	_, _ = g.platform.ExecContext(ctx, `UPDATE identity.users SET status = 'locked' WHERE user_id = $1`, locked)
	_, _ = g.platform.ExecContext(ctx, `UPDATE identity.users SET status = 'deactivated' WHERE user_id = $1`, deact)
	tx, _ := g.platform.BeginTx(ctx, nil)
	noClass, err := insertStudent(ctx, tx, a, "hs.chualop")
	if err == nil {
		_, err = tx.ExecContext(ctx, `UPDATE org.students SET status = 'inactive' WHERE student_id = $1`, noClass)
	}
	if err != nil {
		t.Fatal(err)
	}
	_ = tx.Commit()
	g.identity(ctx, t, uuid.MustParse(noClass), "hs.chualop")
	for u, want := range map[string]problem.Code{"hs.khoa": problem.AccountLocked, "hs.nghi": problem.AccountDeactivated, "hs.chualop": problem.NoActiveEnrollment} {
		if r := g.login(t, u, dev); r.code != 403 || codeOf(r) != string(want) {
			t.Fatalf("%s: %d %v", u, r.code, r.body)
		}
	}

	// AC-08: common tenant → is_common_tenant, virtual school with the grade of the student (no virtual class — ADR-0095).
	common := uuid.New()
	if _, err := g.platform.ExecContext(ctx, `INSERT INTO identity.users (user_id, created_at, updated_at, user_type, status, username, display_name, primary_tenant_id, provisioned_at)
		VALUES ($1, now(), now(), 'student', 'active', 'hs.chung', 'hs.chung', $2, now())`, common, commonTenant); err != nil {
		t.Fatal(err)
	}
	if _, err := g.platform.ExecContext(ctx, `INSERT INTO org.students (student_id, tenant_id, created_at, updated_at, school_id, grade_level, full_name)
		VALUES ($1, $2, now(), now(), $3, 4, 'Hoc Sinh Chung')`, common, commonTenant, commonSchool); err != nil {
		t.Fatal(err)
	}
	g.identity(ctx, t, common, "hs.chung")
	r = g.login(t, "hs.chung", dev)
	if r.code != 200 {
		t.Fatalf("common: %d %v", r.code, r.body)
	}
	cb := r.body["bootstrap"].(map[string]any)
	// F095-AC-23: friends are off by default in the common tenant (the client hides the buttons).
	if ff, _ := cb["feature_flags"].(map[string]any); ff["social.friends_enabled"] != false {
		t.Fatalf("common bootstrap feature flags: %v", cb["feature_flags"])
	}
	if cb["is_common_tenant"] != true || cb["tenant_type"] != "common" || cb["grade_band"] != "g4_5" ||
		cb["school"].(map[string]any)["is_virtual"] != true || cb["student"].(map[string]any)["grade_level"] != float64(4) || cb["class"] != nil {
		t.Fatalf("common bootstrap: %v", cb)
	}
	if n := count(ctx, t, g.platform, `SELECT count(*) FROM identity.login_events WHERE user_id = $1 AND tenant_resolution = 'common'`, common); n != 1 {
		t.Fatal("common resolution")
	}
	if testutil.ToFloat64(g.metrics.LoginCount("success")) != 3 {
		t.Fatalf("success metric = %v", testutil.ToFloat64(g.metrics.LoginCount("success")))
	}
}

// F013 US2: F013-AC-09..AC-11 (the second tenant is offered by the extraMembership TEST DOUBLE).
func TestGameLoginTenantSelect(t *testing.T) {
	ctx := context.Background()
	g := newGameIntegration(ctx, t, gameOpts{})
	a := seedTenant(ctx, t, g.platform, "pgd-sel-a")
	b := seedTenant(ctx, t, g.platform, "pgd-sel-b")
	an := g.student(ctx, t, a, "hs.hai.truong")
	g.members.extra = &gamelogin.Membership{TenantID: uuid.MustParse(b.tenant), TenantType: "unit", TenantName: "pgd-sel-b", TenantStatus: "active", MemberKind: "student"}
	dev := uuid.New()
	start := func() (string, gameResp) {
		r := g.login(t, "hs.hai.truong", dev)
		if r.code != 403 || codeOf(r) != string(problem.TenantNotSelected) {
			t.Fatalf("login: %d %v", r.code, r.body)
		}
		choices, _ := r.body["choices"].([]any)
		ticket, _ := r.body["login_ticket"].(string)
		if len(choices) != 2 || len(ticket) < 16 || r.body["login_ticket_expires_at"] == nil {
			t.Fatalf("choices/ticket: %v", r.body)
		}
		return ticket, r
	}
	ticket, _ := start()
	if n := count(ctx, t, g.platform, `SELECT count(*) FROM identity.login_events WHERE user_id = $1 AND result = 'tenant_not_selected' AND tenant_id IS NULL`, an); n != 1 {
		t.Fatal("tenant_not_selected journal")
	}
	sel := func(ticket, tenant string, device uuid.UUID) gameResp {
		return g.post(t, "/auth/game/tenant:select", map[string]any{"login_ticket": ticket, "tenant_id": tenant, "device_id": device.String()}, nil)
	}
	// AC-11: another device → LOGIN_TICKET_INVALID (the ticket is used up).
	if r := sel(ticket, a.tenant, uuid.New()); r.code != 401 || codeOf(r) != string(problem.LoginTicketInvalid) {
		t.Fatalf("other device: %d %v", r.code, r.body)
	}
	if r := sel(ticket, a.tenant, dev); r.code != 401 || codeOf(r) != string(problem.LoginTicketInvalid) {
		t.Fatalf("used ticket: %d %v", r.code, r.body)
	}
	// AC-11: tenant outside choices → TENANT_MISMATCH, no token.
	ticket, _ = start()
	if r := sel(ticket, uuid.NewString(), dev); r.code != 403 || codeOf(r) != string(problem.TenantMismatch) || r.body["tokens"] != nil {
		t.Fatalf("outside choices: %d %v", r.code, r.body)
	}
	// AC-10: valid ticket → tokens + bootstrap of the tenant, no password.
	ticket, _ = start()
	r := sel(ticket, a.tenant, dev)
	if r.code != 200 || r.body["bootstrap"].(map[string]any)["tenant_id"] != a.tenant {
		t.Fatalf("select: %d %v", r.code, r.body)
	}
	if n := count(ctx, t, g.platform, `SELECT count(*) FROM identity.login_events WHERE user_id = $1 AND result = 'success' AND tenant_resolution = 'selected'`, an); n != 1 {
		t.Fatal("selected journal")
	}
	// Expired ticket.
	ticket, _ = start()
	if err := g.vk.Do(ctx, g.vk.B().Del().Key("ew:identity:login_ticket:"+gamelogin.HashToken(ticket)).Build()).Error(); err != nil {
		t.Fatal(err)
	}
	if r := sel(ticket, a.tenant, dev); r.code != 401 || codeOf(r) != string(problem.LoginTicketInvalid) {
		t.Fatalf("expired: %d %v", r.code, r.body)
	}
}

// F013 US3: F013-AC-12..AC-15, AC-27..AC-29.
func TestGameRefreshAndLogout(t *testing.T) {
	ctx := context.Background()
	g := newGameIntegration(ctx, t, gameOpts{})
	a := seedTenant(ctx, t, g.platform, "pgd-rt-a")
	b := seedTenant(ctx, t, g.platform, "pgd-rt-b")
	an := g.student(ctx, t, a, "hs.refresh")
	dev := uuid.New()
	r := g.login(t, "hs.refresh", dev)
	if r.code != 200 {
		t.Fatalf("login: %d %v", r.code, r.body)
	}
	rt0 := tokensOf(t, r)["refresh_token"].(string)

	// AC-12: renew rotates; the new token differs and the stored hash follows.
	r = g.refresh(t, rt0, dev, a.tenant, "renew", "")
	if r.code != 200 || r.body["refresh_token"] == rt0 || r.body["bootstrap"] != nil {
		t.Fatalf("renew: %d %v", r.code, r.body)
	}
	rt1 := r.body["refresh_token"].(string)
	if n := count(ctx, t, g.platform, `SELECT count(*) FROM identity.game_auth_sessions WHERE refresh_token_hash = $1
		AND previous_token_hashes @> jsonb_build_array($2::text)`, gamelogin.HashToken(rt1), gamelogin.HashToken(rt0)); n != 1 {
		t.Fatal("rotation not recorded")
	}
	// Header of another tenant → TENANT_MISMATCH; unknown token → REFRESH_TOKEN_INVALID.
	if r := g.refresh(t, rt1, dev, b.tenant, "renew", ""); r.code != 403 || codeOf(r) != string(problem.TenantMismatch) {
		t.Fatalf("other tenant: %d %v", r.code, r.body)
	}
	if r := g.refresh(t, "khong-phai-token", dev, a.tenant, "renew", ""); r.code != 401 || codeOf(r) != string(problem.RefreshTokenInvalid) {
		t.Fatalf("unknown: %d %v", r.code, r.body)
	}
	// AC-27: another device → DEVICE_MISMATCH without calling Keycloak; the session still refreshes.
	calls := g.tokenCalls()
	if r := g.refresh(t, rt1, uuid.New(), a.tenant, "renew", ""); r.code != 401 || codeOf(r) != string(problem.RefreshTokenDeviceMismatch) {
		t.Fatalf("device: %d %v", r.code, r.body)
	}
	if g.tokenCalls() != calls {
		t.Fatal("Keycloak called on device mismatch")
	}
	// AC-28: resume returns the bootstrap; an old client version → 426 and the session is kept.
	if r := g.refresh(t, rt1, dev, a.tenant, "resume", "1.0.0"); r.code != 426 || codeOf(r) != string(problem.ClientVersionUnsupported) {
		t.Fatalf("426: %d %v", r.code, r.body)
	}
	r = g.refresh(t, rt1, dev, a.tenant, "resume", "1.4.0")
	if r.code != 200 || r.body["bootstrap"].(map[string]any)["tenant_id"] != a.tenant {
		t.Fatalf("resume: %d %v", r.code, r.body)
	}
	rt2 := r.body["refresh_token"].(string)

	// AC-13: a rotated token is reuse → the whole session is revoked (the newest token too) + audit.
	deleted := g.kc.deleted
	if r := g.refresh(t, rt1, dev, a.tenant, "renew", ""); r.code != 401 || codeOf(r) != string(problem.RefreshTokenReused) {
		t.Fatalf("reuse: %d %v", r.code, r.body)
	}
	if r := g.refresh(t, rt2, dev, a.tenant, "renew", ""); r.code != 401 || codeOf(r) != string(problem.RefreshTokenInvalid) {
		t.Fatalf("after reuse: %d %v", r.code, r.body)
	}
	if g.kc.deleted != deleted+1 || count(ctx, t, g.platform, `SELECT count(*) FROM identity.game_auth_sessions WHERE user_id = $1 AND revoke_reason = 'refresh_reuse'`, an) != 1 {
		t.Fatal("reuse revocation")
	}
	if n := count(ctx, t, g.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded' AND envelope::text LIKE '%auth.refresh_token_reused%'`); n != 1 {
		t.Fatal("reuse audit")
	}

	// Two concurrent refreshes of one token: exactly one succeeds (FOR UPDATE), the other is reuse.
	r = g.login(t, "hs.refresh", dev)
	rt := tokensOf(t, r)["refresh_token"].(string)
	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = g.refresh(t, rt, dev, a.tenant, "renew", "").code
		}()
	}
	wg.Wait()
	if (codes[0] == 200) == (codes[1] == 200) {
		t.Fatalf("concurrent refresh codes = %v", codes)
	}

	// AC-14: logout revokes (reason logout), ends the Keycloak session, 204 twice (idempotent).
	r = g.login(t, "hs.refresh", dev)
	rt = tokensOf(t, r)["refresh_token"].(string)
	var sid string
	if err := g.platform.QueryRowContext(ctx, `SELECT keycloak_sid FROM identity.game_auth_sessions WHERE refresh_token_hash = $1`, gamelogin.HashToken(rt)).Scan(&sid); err != nil {
		t.Fatal(err)
	}
	tenant := uuid.MustParse(a.tenant)
	pctx := authz.WithPrincipal(ctx, authz.NewPrincipal(authz.PrincipalData{UserID: an, UserType: authz.UserTypeStudent, TenantID: &tenant,
		ClientID: command.GameClientID, SessionID: sid}))
	for range 2 {
		if err := g.auth.Logout(pctx, rt); err != nil {
			t.Fatal(err)
		}
	}
	if n := count(ctx, t, g.platform, `SELECT count(*) FROM identity.game_auth_sessions WHERE keycloak_sid = $1 AND revoke_reason = 'logout'`, sid); n != 1 {
		t.Fatal("logout revocation")
	}
	if r := g.refresh(t, rt, dev, a.tenant, "renew", ""); r.code != 401 || codeOf(r) != string(problem.RefreshTokenInvalid) {
		t.Fatalf("after logout: %d %v", r.code, r.body)
	}

	// AC-29 (port): GameAuthSessionRevoker revokes the live sessions of the user once.
	r = g.login(t, "hs.refresh", dev)
	rt = tokensOf(t, r)["refresh_token"].(string)
	if n, err := g.auth.RevokeSessions(ctx, portwrite.SessionFilter{UserID: an}, gamelogin.RevokePasswordChanged); err != nil || n != 1 {
		t.Fatalf("revoke = %d %v", n, err)
	}
	if n, err := g.auth.RevokeSessions(ctx, portwrite.SessionFilter{UserID: an}, gamelogin.RevokePasswordChanged); err != nil || n != 0 {
		t.Fatalf("revoke again = %d %v", n, err)
	}
	if r := g.refresh(t, rt, dev, a.tenant, "renew", ""); r.code != 401 || codeOf(r) != string(problem.RefreshTokenInvalid) {
		t.Fatalf("after revoke: %d %v", r.code, r.body)
	}

	// AC-28: resume after the account is locked → 403 ACCOUNT_LOCKED and the session is revoked.
	r = g.login(t, "hs.refresh", dev)
	rt = tokensOf(t, r)["refresh_token"].(string)
	_, _ = g.platform.ExecContext(ctx, `UPDATE identity.users SET status = 'locked' WHERE user_id = $1`, an)
	if r := g.refresh(t, rt, dev, a.tenant, "resume", "1.4.0"); r.code != 403 || codeOf(r) != string(problem.AccountLocked) {
		t.Fatalf("locked resume: %d %v", r.code, r.body)
	}
	if n := count(ctx, t, g.platform, `SELECT count(*) FROM identity.game_auth_sessions WHERE refresh_token_hash = $1 AND revoke_reason = 'account_locked'`, gamelogin.HashToken(rt)); n != 1 {
		t.Fatal("locked revocation")
	}
	if testutil.ToFloat64(g.metrics.RefreshCount("renew", "ok")) < 1 || testutil.ToFloat64(g.metrics.RevokedCount("logout")) != 1 {
		t.Fatal("refresh/revoke metrics")
	}
}

// F013 US4: F013-AC-18..AC-21.
func TestGameLoginProtection(t *testing.T) {
	ctx := context.Background()
	g := newGameIntegration(ctx, t, gameOpts{limits: command.GameRateLimits{Global: command.Limit{N: 1000, Window: time.Minute},
		PerDevice: command.Limit{N: 3, Window: time.Minute}, PerUsername: command.Limit{N: 5, Window: time.Minute},
		RefreshPerSession: command.Limit{N: 100, Window: time.Minute}}})
	a := seedTenant(ctx, t, g.platform, "pgd-prot-a")
	g.student(ctx, t, a, "hs.dem")
	g.student(ctx, t, a, "hs.khac")

	// AC-18: per device (same IP for every request).
	dev := uuid.New()
	for i := range 3 {
		if r := g.login(t, "hs.dem", dev); r.code != 200 {
			t.Fatalf("login %d: %d %v", i, r.code, r.body)
		}
	}
	r := g.login(t, "hs.dem", dev)
	if r.code != 429 || codeOf(r) != string(problem.RateLimited) || r.header.Get("Retry-After") == "" {
		t.Fatalf("device limit: %d %v", r.code, r.body)
	}
	if r := g.login(t, "hs.khac", uuid.New()); r.code != 200 {
		t.Fatalf("another student behind the same NAT: %d %v", r.code, r.body)
	}
	// Per username across devices.
	for range 2 {
		_ = g.login(t, "hs.dem", uuid.New())
	}
	if r := g.login(t, "hs.dem", uuid.New()); r.code != 429 || codeOf(r) != string(problem.RateLimited) {
		t.Fatalf("username limit: %d %v", r.code, r.body)
	}
	// AC-20: old client → 426 without Keycloak.
	calls := g.tokenCalls()
	old := gameLogin("hs.khac", uuid.New())
	old["client_version"] = "1.1.9"
	if r := g.post(t, "/auth/game/login", old, nil); r.code != 426 || codeOf(r) != string(problem.ClientVersionUnsupported) || g.tokenCalls() != calls {
		t.Fatalf("426: %d %v", r.code, r.body)
	}
	// AC-21: Keycloak down → 503 + Retry-After, no detail.
	g.kc.mu.Lock()
	g.kc.down = true
	g.kc.mu.Unlock()
	r = g.login(t, "hs.khac", uuid.New())
	if r.code != 503 || codeOf(r) != string(problem.Unavailable) || r.header.Get("Retry-After") == "" || strings.Contains(fmt.Sprint(r.body), "keycloak") {
		t.Fatalf("503: %d %v", r.code, r.body)
	}

	// AC-19: global login_rate → LOGIN_QUEUE_BUSY.
	q := newGameIntegration(ctx, t, gameOpts{limits: command.GameRateLimits{Global: command.Limit{N: 2, Window: time.Minute},
		PerDevice: command.Limit{N: 100, Window: time.Minute}, PerUsername: command.Limit{N: 100, Window: time.Minute}}})
	qa := seedTenant(ctx, t, q.platform, "pgd-queue")
	q.student(ctx, t, qa, "hs.hang")
	for range 2 {
		_ = q.login(t, "hs.hang", uuid.New())
	}
	r = q.login(t, "hs.hang", uuid.New())
	if r.code != 429 || codeOf(r) != string(problem.LoginQueueBusy) || r.header.Get("Retry-After") == "" || testutil.ToFloat64(q.metrics.QueueCount()) != 1 {
		t.Fatalf("queue: %d %v", r.code, r.body)
	}
}

// F013-AC-32: inside the curfew, login and resume answer 403 CURFEW_ACTIVE + reopen_at after the SSO and
// account checks; resume does not revoke the session; renew of a running session is not blocked.
func TestGameLoginCurfew(t *testing.T) {
	ctx := context.Background()
	ict := time.FixedZone("ICT", 7*3600)
	now := time.Now().In(ict)
	mins := func(t time.Time) int { return t.Hour()*60 + t.Minute() }
	// A window that excludes now (login allowed, curfew shown in the bootstrap), then one that includes now.
	outside := gamelogin.Curfew{Start: (mins(now) + 60) % 1440, End: (mins(now) + 120) % 1440, Loc: ict}
	inside := gamelogin.Curfew{Start: (mins(now) + 1440 - 30) % 1440, End: (mins(now) + 30) % 1440, Loc: ict}
	g := newGameIntegration(ctx, t, gameOpts{curfew: outside})
	a := seedTenant(ctx, t, g.platform, "pgd-curfew")
	g.student(ctx, t, a, "hs.dem.khuya")
	dev := uuid.New()
	r := g.login(t, "hs.dem.khuya", dev)
	if r.code != 200 {
		t.Fatalf("login: %d %v", r.code, r.body)
	}
	pt := r.body["bootstrap"].(map[string]any)["policies"].(map[string]any)["play_time"].(map[string]any)["curfew"].(map[string]any)
	if pt["warning_minutes"] != float64(10) || pt["start"] == "" {
		t.Fatalf("curfew policy: %v", pt)
	}
	rt := tokensOf(t, r)["refresh_token"].(string)

	g.auth.Curfew = gameauth.PlatformCurfew{Curfew: inside}
	logouts := g.kc.logouts
	r = g.login(t, "hs.dem.khuya", dev)
	if r.code != 403 || codeOf(r) != string(problem.CurfewActive) || r.body["reopen_at"] == nil {
		t.Fatalf("curfew login: %d %v", r.code, r.body)
	}
	if g.kc.logouts != logouts+1 {
		t.Fatal("no Keycloak session may survive a refused login")
	}
	// The order: wrong password inside the curfew is still INVALID_CREDENTIALS (SSO first).
	wrong := gameLogin("hs.dem.khuya", dev)
	wrong["password"] = "sai"
	if r := g.post(t, "/auth/game/login", wrong, nil); codeOf(r) != string(problem.InvalidCredentials) {
		t.Fatalf("order: %v", r.body)
	}
	if r := g.refresh(t, rt, dev, a.tenant, "resume", "1.4.0"); r.code != 403 || codeOf(r) != string(problem.CurfewActive) {
		t.Fatalf("curfew resume: %d %v", r.code, r.body)
	}
	if n := count(ctx, t, g.platform, `SELECT count(*) FROM identity.game_auth_sessions WHERE refresh_token_hash = $1 AND revoked_at IS NULL`, gamelogin.HashToken(rt)); n != 1 {
		t.Fatal("curfew revoked the session")
	}
	r = g.refresh(t, rt, dev, a.tenant, "renew", "")
	if r.code != 200 {
		t.Fatalf("renew in curfew: %d %v", r.code, r.body)
	}
	// After the curfew, resume works without a password.
	g.auth.Curfew = gameauth.PlatformCurfew{Curfew: outside}
	rt = r.body["refresh_token"].(string)
	if r := g.refresh(t, rt, dev, a.tenant, "resume", "1.4.0"); r.code != 200 || r.body["bootstrap"] == nil {
		t.Fatalf("resume after curfew: %d %v", r.code, r.body)
	}
}
