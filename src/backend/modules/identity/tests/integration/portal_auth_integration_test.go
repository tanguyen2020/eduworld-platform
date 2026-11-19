//go:build integration

package integration

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/keycloak"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/portalmetrics"
	identityhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

const kcIssuer = "http://kc/realms/eduworld"

// fakeKeycloak is a TEST FAKE of the Keycloak 26 token and logout endpoints of realm eduworld for the
// confidential client eduworld-portal (direct grant flow eduworld-portal-direct-grant, F004 research R5):
// wrong credentials → 401 invalid_grant, student → 401 text/html, rotating single-use refresh tokens.
type fakeKeycloak struct {
	mu      sync.Mutex
	users   map[string]fakeKCUser // username → account
	refresh map[string]string     // live refresh token → sid
	sidUser map[string]string     // sid → username
	sidAMR  map[string][]string
	seq     int
	logouts int
	actions map[string][]string // username → Keycloak required actions (setup pending when not empty)
	emails  int                 // execute-actions-email CONFIGURE_TOTP
	kicked  int                 // Admin logout of every session of a user
	// F013: the game client eduworld-game-backend shares the fake; sidClient is the azp of each session,
	// deleted counts Admin DELETE /sessions/{sid}.
	sidClient map[string]string
	deleted   int
	down      bool // every token call answers 503 (F013-AC-21)
}

// fakeClients are the confidential clients of the fake realm and their secrets.
var fakeClients = map[string]string{"eduworld-portal": "portal-secret", "eduworld-game-backend": "game-secret"}

type fakeKCUser struct {
	password, otp string
	student       bool
}

func jwtOf(claims map[string]any) string {
	b, _ := json.Marshal(claims)
	return base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`)) + "." + base64.RawURLEncoding.EncodeToString(b) + ".sig"
}

func (k *fakeKeycloak) issue(w http.ResponseWriter, sid string) {
	k.seq++
	user := k.sidUser[sid]
	rt := jwtOf(map[string]any{"sid": sid, "jti": fmt.Sprint(k.seq), "typ": "Refresh"})
	k.refresh[rt] = sid
	at := jwtOf(map[string]any{"iss": kcIssuer, "sub": "kc-" + user, "sid": sid, "amr": k.sidAMR[sid], "iat": time.Now().Unix(),
		"azp": k.azp(sid), "typ": "Bearer"})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"access_token": at, "refresh_token": rt, "expires_in": 300, "refresh_expires_in": 1800,
		"token_type": "Bearer", "session_state": sid})
}

func (k *fakeKeycloak) azp(sid string) string {
	if c := k.sidClient[sid]; c != "" {
		return c
	}
	return "eduworld-portal"
}

func oauthErr(w http.ResponseWriter, status int, code, desc string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "error_description": desc})
}

func (k *fakeKeycloak) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	f, _ := url.ParseQuery(string(raw))
	k.mu.Lock()
	defer k.mu.Unlock()
	if f.Get("grant_type") == "client_credentials" && f.Get("client_id") == "svc-identity" && f.Get("client_secret") == "svc-secret" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "admin-tok", "expires_in": 300})
		return
	}
	if strings.HasPrefix(r.URL.Path, "/admin/realms/eduworld/") {
		k.adminAPI(w, r, raw)
		return
	}
	if k.down {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	client := f.Get("client_id")
	if sec, ok := fakeClients[client]; !ok || f.Get("client_secret") != sec {
		oauthErr(w, 401, "invalid_client", "Invalid client or Invalid client credentials")
		return
	}
	if strings.HasSuffix(r.URL.Path, "/logout") {
		k.logouts++
		sid, ok := k.refresh[f.Get("refresh_token")]
		if !ok {
			oauthErr(w, 400, "invalid_grant", "Session not active")
			return
		}
		for rt, s := range k.refresh {
			if s == sid {
				delete(k.refresh, rt)
			}
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	switch f.Get("grant_type") {
	case "password":
		u, ok := k.users[f.Get("username")]
		if !ok || u.password != f.Get("password") {
			oauthErr(w, 401, "invalid_grant", "Invalid user credentials")
			return
		}
		if u.student == (client == "eduworld-portal") { // portal denies students, the game denies staff
			w.Header().Set("Content-Type", "text/html;charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, "<html>portalStudentDenied</html>")
			return
		}
		if u.otp != "" && f.Get("otp") != u.otp {
			oauthErr(w, 401, "invalid_grant", "Invalid user credentials")
			return
		}
		if len(k.actions[f.Get("username")]) > 0 {
			oauthErr(w, 400, "invalid_grant", "Account is not fully set up")
			return
		}
		sid := fmt.Sprintf("sid-%d", k.seq+1)
		k.sidUser[sid] = f.Get("username")
		if k.sidClient != nil {
			k.sidClient[sid] = client
		}
		k.sidAMR[sid] = []string{"pwd"}
		if f.Get("otp") != "" {
			k.sidAMR[sid] = []string{"pwd", "otp"}
		}
		k.issue(w, sid)
	case "refresh_token":
		sid, ok := k.refresh[f.Get("refresh_token")]
		if !ok {
			oauthErr(w, 400, "invalid_grant", "Token is not active")
			return
		}
		delete(k.refresh, f.Get("refresh_token"))
		k.issue(w, sid)
	default:
		oauthErr(w, 400, "unsupported_grant_type", "")
	}
}

// adminAPI is the TEST FAKE Admin API of svc-identity (Keycloak 26 paths): users by username/email, required
// actions, reset-password with a minimum-length realm policy, execute-actions-email, logout of a user.
// Keycloak user ids are "kc-<username>"; staff emails are "<username>@test.vn" (staffUser).
func (k *fakeKeycloak) adminAPI(w http.ResponseWriter, r *http.Request, body []byte) {
	if r.Header.Get("Authorization") != "Bearer admin-tok" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	p := strings.TrimPrefix(r.URL.Path, "/admin/realms/eduworld")
	if sid, ok := strings.CutPrefix(p, "/sessions/"); ok && r.Method == http.MethodDelete {
		k.deleted++
		for rt, s := range k.refresh {
			if s == sid {
				delete(k.refresh, rt)
			}
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	rep := func(u string) map[string]any {
		return map[string]any{"id": "kc-" + u, "username": u, "email": u + "@test.vn", "enabled": true, "requiredActions": k.actions[u]}
	}
	switch {
	case r.Method == http.MethodGet && p == "/users":
		out := []map[string]any{}
		q := r.URL.Query()
		for u := range k.users {
			if q.Get("username") == u || q.Get("email") == u+"@test.vn" {
				out = append(out, rep(u))
			}
		}
		_ = json.NewEncoder(w).Encode(out)
		return
	case !strings.HasPrefix(p, "/users/kc-"):
		w.WriteHeader(http.StatusNotFound)
		return
	}
	rest := strings.TrimPrefix(p, "/users/kc-")
	user, action, _ := strings.Cut(rest, "/")
	if _, ok := k.users[user]; !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch {
	case action == "" && r.Method == http.MethodGet:
		_ = json.NewEncoder(w).Encode(rep(user))
	case action == "" && r.Method == http.MethodPut:
		var u struct {
			RequiredActions []string `json:"requiredActions"`
		}
		_ = json.Unmarshal(body, &u)
		k.actions[user] = u.RequiredActions
		w.WriteHeader(http.StatusNoContent)
	case action == "reset-password":
		var c struct {
			Value     string `json:"value"`
			Temporary bool   `json:"temporary"`
		}
		_ = json.Unmarshal(body, &c)
		if len(c.Value) < 10 || c.Temporary {
			oauthErr(w, 400, "invalidPasswordMinLengthMessage", "Invalid password: minimum length 10.")
			return
		}
		u := k.users[user]
		u.password = c.Value
		k.users[user] = u
		w.WriteHeader(http.StatusNoContent)
	case action == "execute-actions-email":
		k.emails++
		w.WriteHeader(http.StatusNoContent)
	case action == "logout":
		k.kicked++
		for rt, sid := range k.refresh {
			if k.sidUser[sid] == user {
				delete(k.refresh, rt)
			}
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// captureMailer is a TEST FAKE of port.Mailer (the SMTP adapter is tested against a fake SMTP server).
type captureMailer struct {
	mu   sync.Mutex
	sent []port.PasswordResetMail
}

func (m *captureMailer) SendPasswordReset(_ context.Context, mail port.PasswordResetMail) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, mail)
	return nil
}

type portalIntegration struct {
	*env
	kc      *fakeKeycloak
	kcSrv   *httptest.Server
	vk      valkey.Client
	srv     *httptest.Server
	metrics *portalmetrics.Metrics
	auth    *command.PortalAuth
	mailer  *captureMailer
}

func newPortalIntegration(ctx context.Context, t *testing.T) *portalIntegration {
	t.Helper()
	e := setup(ctx, t)
	_, addr := testkit.StartValkey(ctx, t)
	vk, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}, DisableCache: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vk.Close)
	kc := &fakeKeycloak{users: map[string]fakeKCUser{}, refresh: map[string]string{}, sidUser: map[string]string{},
		sidAMR: map[string][]string{}, actions: map[string][]string{}}
	kcSrv := httptest.NewServer(kc)
	t.Cleanup(kcSrv.Close)
	tokens, err := keycloak.NewPortalTokens(keycloak.PortalConfig{TokenURL: kcSrv.URL + "/realms/eduworld/protocol/openid-connect/token",
		LogoutURL: kcSrv.URL + "/realms/eduworld/protocol/openid-connect/logout", ClientID: "eduworld-portal", ClientSecret: "portal-secret",
		Timeout: 2 * time.Second, MaxRetries: 1}, kcSrv.Client())
	if err != nil {
		t.Fatal(err)
	}
	m, _ := portalmetrics.New(nil)
	auth := &command.PortalAuth{UoW: e.uow, Accounts: write.PortalAccountRepo{}, Audit: e.deps.Audit, IdP: tokens,
		Tickets: cache.SetupTickets{C: vk}, Sessions: cache.PortalSessions{C: vk}, Limiter: cache.RateLimiter{C: vk}, Metrics: m,
		Cfg: command.PortalAuthConfig{Issuers: []string{kcIssuer}, PortalBaseURL: "https://portal.eduworld.test"}}
	admin, err := keycloak.New(keycloak.Config{BaseURL: kcSrv.URL + "/admin/realms/eduworld", ClientID: "svc-identity", ClientSecret: "svc-secret",
		Timeout: 2 * time.Second, MaxRetries: 1, FailureRatio: 0.9}, kcSrv.Client())
	if err != nil {
		t.Fatal(err)
	}
	mailer := &captureMailer{}
	auth.KC, auth.Resets, auth.Mailer, auth.Background = admin, write.PortalResetTokenRepo{}, mailer, func(f func()) { f() }
	spec, err := httpx.LoadSpec(ctx, filepath.Join(testkit.RepoRoot(t), "specs", "openapi", "eduworld-api.v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	idp := httpxtest.NewIDP(t)
	s, err := httpx.NewServer(httpx.Deps{Auth: idp.AuthConfig(), Keys: idp.JWKS(t), Resolver: &httpxtest.Resolver{}, Spec: spec})
	if err != nil {
		t.Fatal(err)
	}
	p := &portalIntegration{env: e, kc: kc, kcSrv: kcSrv, vk: vk, metrics: m, auth: auth, mailer: mailer}
	if err := identityhttp.RegisterPortalAuth(s, p.handlers()); err != nil {
		t.Fatal(err)
	}
	p.srv = httptest.NewServer(s)
	t.Cleanup(p.srv.Close)
	return p
}

func (p *portalIntegration) handlers() identityhttp.PortalAuthHandlers {
	return identityhttp.PortalAuthHandlers{Session: p.auth, Setup: p.auth, Metrics: p.metrics}
}

type portalResp struct {
	code    int
	header  http.Header
	body    map[string]any
	cookies map[string]*http.Cookie
}

func (p *portalIntegration) post(t *testing.T, path string, body any, cookies map[string]string, hdr map[string]string) portalResp {
	t.Helper()
	var rd io.Reader = http.NoBody
	if body != nil {
		b, _ := json.Marshal(body)
		rd = strings.NewReader(string(b))
	}
	req, _ := http.NewRequest(http.MethodPost, p.srv.URL+"/api/v1"+path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	out := portalResp{code: resp.StatusCode, header: resp.Header, cookies: map[string]*http.Cookie{}}
	raw, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(raw, &out.body)
	for _, c := range resp.Cookies() {
		out.cookies[c.Name] = c
	}
	return out
}

func portalLogin(user, password, otp string) map[string]any {
	m := map[string]any{"username": user, "password": password, "device_id": uuid.NewString(), "locale": "vi-VN"}
	if otp != "" {
		m["otp"] = otp
	}
	return m
}

func cookiesOf(r portalResp) map[string]string {
	return map[string]string{identityhttp.PortalRefreshCookie: r.cookies[identityhttp.PortalRefreshCookie].Value,
		identityhttp.PortalCSRFCookie: r.cookies[identityhttp.PortalCSRFCookie].Value}
}

// F012-AC-27..AC-32 through the real chain: merged spec, PortalAccountRepo on PostgreSQL (role check across
// tenants and platform), Valkey session store and rate limiter, the Keycloak token client against a fake
// Keycloak, audit through the outbox.
func TestPortalLoginIntegration(t *testing.T) {
	ctx := context.Background()
	p := newPortalIntegration(ctx, t)
	a := seedTenant(ctx, t, p.platform, "pgd-portal-a")
	b := seedTenant(ctx, t, p.platform, "pgd-portal-b")
	if _, err := p.platform.ExecContext(ctx, `UPDATE platform.tenants SET status = 'disabled' WHERE tenant_id = $1`, b.tenant); err != nil {
		t.Fatal(err)
	}
	const pw = "Integration-Pw-1!"
	ad := staffUser(ctx, t, p.env, "ad.portal.a", &a.tenant)
	grantRole(ctx, t, p.env, ad, "school_admin", &a.tenant, "tenant", nil, "NULL")
	sa := staffUser(ctx, t, p.env, "sa.portal", nil)
	grantRole(ctx, t, p.env, sa, "system_admin", nil, "platform", nil, "NULL")
	_ = staffUser(ctx, t, p.env, "nv.portal.norole", &a.tenant)
	exp := staffUser(ctx, t, p.env, "nv.portal.expired", &a.tenant)
	grantRole(ctx, t, p.env, exp, "school_admin", &a.tenant, "tenant", nil, "now() - interval '1 hour'")
	dis := staffUser(ctx, t, p.env, "nv.portal.disabled", &b.tenant)
	grantRole(ctx, t, p.env, dis, "school_admin", &b.tenant, "tenant", nil, "NULL")
	off := staffUser(ctx, t, p.env, "nv.portal.off", &a.tenant)
	grantRole(ctx, t, p.env, off, "school_admin", &a.tenant, "tenant", nil, "NULL")
	if _, err := p.platform.ExecContext(ctx, `UPDATE identity.users SET status = 'deactivated' WHERE user_id = $1`, off); err != nil {
		t.Fatal(err)
	}
	for _, u := range []string{"ad.portal.a", "nv.portal.norole", "nv.portal.expired", "nv.portal.disabled", "nv.portal.off"} {
		p.kc.users[u] = fakeKCUser{password: pw}
	}
	p.kc.users["sa.portal"] = fakeKCUser{password: pw, otp: "482913"}
	p.kc.users["hs.portal"] = fakeKCUser{password: pw, student: true}

	// AC-27: tenant staff without OTP, platform staff with OTP.
	r := p.post(t, "/auth/portal/login", portalLogin("AD.Portal.A", pw, ""), nil, nil)
	if r.code != 200 || r.body["mfa"] != false || r.header.Get("Cache-Control") != "no-store" || !r.cookies[identityhttp.PortalRefreshCookie].HttpOnly {
		t.Fatalf("tenant staff login = %d %v", r.code, r.body)
	}
	session := cookiesOf(r)
	if r = p.post(t, "/auth/portal/login", portalLogin("sa.portal", pw, "482913"), nil, nil); r.code != 200 || r.body["mfa"] != true {
		t.Fatalf("platform login = %d %v", r.code, r.body)
	}
	// AC-28: missing OTP for platform staff = wrong password = unknown user.
	for _, in := range []map[string]any{portalLogin("sa.portal", pw, ""), portalLogin("sa.portal", pw, "000000"),
		portalLogin("ad.portal.a", "nope", ""), portalLogin("khong.co", pw, "")} {
		if r = p.post(t, "/auth/portal/login", in, nil, nil); r.code != 401 || r.body["code"] != "INVALID_CREDENTIALS" {
			t.Fatalf("%v → %d %v", in["username"], r.code, r.body)
		}
	}
	// AC-29: student (flow), no role, expired role, role only in a disabled tenant → staff only; deactivated.
	revokedBefore := p.kc.logouts
	for user, code := range map[string]string{"hs.portal": "PORTAL_ACCESS_STAFF_ONLY", "nv.portal.norole": "PORTAL_ACCESS_STAFF_ONLY",
		"nv.portal.expired": "PORTAL_ACCESS_STAFF_ONLY", "nv.portal.disabled": "PORTAL_ACCESS_STAFF_ONLY", "nv.portal.off": "ACCOUNT_DEACTIVATED"} {
		if r = p.post(t, "/auth/portal/login", portalLogin(user, pw, ""), nil, nil); r.code != 403 || r.body["code"] != code {
			t.Fatalf("%s → %d %v", user, r.code, r.body)
		}
	}
	if p.kc.logouts-revokedBefore != 4 {
		t.Fatalf("Keycloak sessions revoked = %d, want 4", p.kc.logouts-revokedBefore)
	}
	// AC-30: refresh through Valkey keeps auth_time; reuse of the rotated token → 401 and cookies cleared.
	csrf := map[string]string{identityhttp.PortalCSRFHeader: session[identityhttp.PortalCSRFCookie]}
	if r = p.post(t, "/auth/portal/token:refresh", nil, session, nil); r.code != 403 || r.body["code"] != "CSRF_TOKEN_INVALID" {
		t.Fatalf("refresh without CSRF = %d %v", r.code, r.body)
	}
	r = p.post(t, "/auth/portal/token:refresh", nil, session, csrf)
	if r.code != 200 {
		t.Fatalf("refresh = %d %v", r.code, r.body)
	}
	next := cookiesOf(r)
	if r = p.post(t, "/auth/portal/token:refresh", nil, session, csrf); r.code != 401 || r.body["code"] != "REFRESH_TOKEN_INVALID" ||
		r.cookies[identityhttp.PortalRefreshCookie].MaxAge >= 0 {
		t.Fatalf("reuse = %d %v", r.code, r.body)
	}
	if r = p.post(t, "/auth/portal/logout", nil, next, map[string]string{identityhttp.PortalCSRFHeader: next[identityhttp.PortalCSRFCookie]}); r.code != 204 {
		t.Fatalf("logout = %d", r.code)
	}
	// AC-28, AC-31: audit through the outbox, no username, password or token.
	var payloads []string
	rows, err := p.relay.QueryContext(ctx, `SELECT envelope::text FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' IN ('auth.portal_login', 'auth.portal_logout')`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		payloads = append(payloads, s)
	}
	_ = rows.Close()
	if len(payloads) < 12 {
		t.Fatalf("portal audits = %d", len(payloads))
	}
	all := strings.Join(payloads, "\n")
	for _, secret := range []string{pw, "482913", "ad.portal.a", "sa.portal", session[identityhttp.PortalRefreshCookie], next[identityhttp.PortalRefreshCookie]} {
		if strings.Contains(strings.ToLower(all), strings.ToLower(secret)) {
			t.Fatalf("audit contains %q", secret)
		}
	}
	// AC-32: per username in Valkey (hashed key), Retry-After.
	var last portalResp
	for range 11 {
		last = p.post(t, "/auth/portal/login", portalLogin("nv.portal.norole", "wrong", ""), nil, nil)
	}
	if last.code != 429 || last.header.Get("Retry-After") == "" || last.body["code"] != "RATE_LIMITED" {
		t.Fatalf("rate limit = %d %v", last.code, last.body)
	}
	keys, err := p.vk.Do(ctx, p.vk.B().Keys().Pattern("ew:identity:*").Build()).AsStrSlice()
	if err != nil || len(keys) == 0 {
		t.Fatalf("valkey keys = %v %v", keys, err)
	}
	for _, k := range keys {
		if strings.Contains(k, "portal.") || strings.Contains(k, "sid-") {
			t.Fatalf("raw identifier in Valkey key %q", k)
		}
	}
}

// F012-AC-33..AC-37 through the real chain: setup ticket in Valkey (GETDEL), Keycloak Admin adapter against
// the fake Admin API, reset tokens in PostgreSQL (hash only, superseded by soft-delete, atomic used_at).
func TestPortalAccountSetupIntegration(t *testing.T) {
	ctx := context.Background()
	p := newPortalIntegration(ctx, t)
	a := seedTenant(ctx, t, p.platform, "pgd-portal-setup")
	const tmp, fresh = "Temp-Passw0rd-1", "Fresh-Passw0rd-2"
	moi := staffUser(ctx, t, p.env, "nv.portal.moi", &a.tenant)
	grantRole(ctx, t, p.env, moi, "school_admin", &a.tenant, "tenant", nil, "NULL")
	p.kc.users["nv.portal.moi"] = fakeKCUser{password: tmp}
	p.kc.actions["nv.portal.moi"] = []string{"UPDATE_PASSWORD", "CONFIGURE_TOTP"}

	// AC-33: password right, required actions left → 403 + ticket + steps, no token, no cookie.
	r := p.post(t, "/auth/portal/login", portalLogin("nv.portal.moi", tmp, ""), nil, nil)
	if r.code != 403 || r.body["code"] != "ACCOUNT_SETUP_REQUIRED" || len(r.cookies) != 0 || r.body["otp_setup_email_sent"] != false {
		t.Fatalf("login = %d %v", r.code, r.body)
	}
	ticket, _ := r.body["setup_ticket"].(string)
	keys, _ := p.vk.Do(ctx, p.vk.B().Keys().Pattern("ew:identity:portal_setup:*").Build()).AsStrSlice()
	if len(ticket) < 32 || len(keys) != 1 || strings.Contains(keys[0], ticket) {
		t.Fatalf("ticket %q keys %v", ticket, keys)
	}
	// AC-34: policy violation keeps the ticket; success removes UPDATE_PASSWORD; AC-35: CONFIGURE_TOTP email.
	setupBody := func(pw string) map[string]any { return map[string]any{"setup_ticket": ticket, "new_password": pw} }
	if r = p.post(t, "/auth/portal/setup:password", setupBody("short"), nil, nil); r.code != 422 {
		t.Fatalf("weak = %d %v", r.code, r.body)
	}
	if r = p.post(t, "/auth/portal/setup:password", setupBody(fresh), nil, nil); r.code != 200 || r.body["otp_setup_email_sent"] != true {
		t.Fatalf("setup = %d %v", r.code, r.body)
	}
	if steps, _ := r.body["remaining_steps"].([]any); len(steps) != 1 || steps[0] != "configure_otp" || p.kc.emails != 1 {
		t.Fatalf("remaining = %v emails = %d", r.body, p.kc.emails)
	}
	if r = p.post(t, "/auth/portal/setup:password", setupBody(fresh), nil, nil); r.code != 401 || r.body["code"] != "SETUP_TICKET_INVALID" {
		t.Fatalf("reuse = %d %v", r.code, r.body)
	}
	if r = p.post(t, "/auth/portal/login", portalLogin("nv.portal.moi", fresh, ""), nil, nil); r.code != 403 || p.kc.emails != 1 {
		t.Fatalf("otp-only login = %d %v emails=%d (one email per window)", r.code, r.body, p.kc.emails)
	}
	// The user scans the QR on the Keycloak action-token page; then the new password works.
	p.kc.actions["nv.portal.moi"] = nil
	if r = p.post(t, "/auth/portal/login", portalLogin("nv.portal.moi", fresh, ""), nil, nil); r.code != 200 {
		t.Fatalf("login after setup = %d %v", r.code, r.body)
	}

	// AC-36: forgot → 202 for staff (by email, any case), student and unknown; only staff gets the link.
	ad := staffUser(ctx, t, p.env, "ad.portal.reset", &a.tenant)
	grantRole(ctx, t, p.env, ad, "school_admin", &a.tenant, "tenant", nil, "NULL")
	p.kc.users["ad.portal.reset"] = fakeKCUser{password: tmp}
	var student string
	if err := inTenant(ctx, t, p.app, a.tenant, true, func(tx *sql.Tx) error {
		var err error
		student, err = insertStudent(ctx, tx, a, "hs.portal.reset")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.platform.ExecContext(ctx, `UPDATE identity.users SET email = 'hs.portal.reset@test.vn' WHERE user_id = $1`, student); err != nil {
		t.Fatal(err)
	}
	forgot := func(id string) {
		t.Helper()
		if r := p.post(t, "/auth/portal/password:forgot", map[string]any{"identifier": id, "device_id": uuid.NewString()}, nil, nil); r.code != 202 {
			t.Fatalf("forgot %s = %d %v", id, r.code, r.body)
		}
	}
	forgot("AD.Portal.Reset@Test.VN")
	forgot("hs.portal.reset@test.vn")
	forgot("khong.co.ai")
	if len(p.mailer.sent) != 1 || p.mailer.sent[0].To != "ad.portal.reset@test.vn" {
		t.Fatalf("mails = %+v", p.mailer.sent)
	}
	tokenOf := func(i int) string {
		tok, ok := strings.CutPrefix(p.mailer.sent[i].Link, "https://portal.eduworld.test/reset-password?token=")
		if !ok {
			t.Fatalf("link %q", p.mailer.sent[i].Link)
		}
		return tok
	}
	first := tokenOf(0)
	if n := count(ctx, t, p.platform, `SELECT count(*) FROM identity.portal_password_reset_tokens WHERE user_id = $1 AND token_hash = encode(sha256($2::bytea), 'hex')`, ad, first); n != 1 {
		t.Fatalf("hashed rows = %d", n)
	}
	if n := count(ctx, t, p.platform, `SELECT count(*) FROM identity.portal_password_reset_tokens WHERE token_hash = $1`, first); n != 0 {
		t.Fatal("the raw token must never be stored")
	}
	forgot("ad.portal.reset")
	second := tokenOf(1)
	if n := count(ctx, t, p.platform, `SELECT count(*) FROM identity.portal_password_reset_tokens WHERE user_id = $1 AND deleted_at IS NOT NULL AND delete_reason = 'superseded'`, ad); n != 1 {
		t.Fatalf("superseded = %d", n)
	}
	// AC-37: superseded → 401; weak → 422 and the token survives; two concurrent uses → exactly one 204.
	reset := func(tok, pw string) portalResp {
		return p.post(t, "/auth/portal/password:reset", map[string]any{"token": tok, "new_password": pw, "device_id": uuid.NewString()}, nil, nil)
	}
	if r = reset(first, fresh); r.code != 401 || r.body["code"] != "PASSWORD_RESET_TOKEN_INVALID" {
		t.Fatalf("superseded = %d %v", r.code, r.body)
	}
	if r = reset(second, "short"); r.code != 422 {
		t.Fatalf("weak = %d %v", r.code, r.body)
	}
	session := p.post(t, "/auth/portal/login", portalLogin("ad.portal.reset", tmp, ""), nil, nil)
	if session.code != 200 {
		t.Fatalf("login before reset = %d", session.code)
	}
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- reset(second, fresh).code
		}()
	}
	wg.Wait()
	close(codes)
	got := map[int]int{}
	for c := range codes {
		got[c]++
	}
	if got[204] != 1 || got[401] != 1 {
		t.Fatalf("concurrent uses = %v", got)
	}
	if n := count(ctx, t, p.platform, `SELECT count(*) FROM identity.portal_password_reset_tokens WHERE user_id = $1 AND used_at IS NOT NULL`, ad); n != 1 {
		t.Fatalf("used rows = %d", n)
	}
	// Every portal session of the user is revoked; the new password works.
	ck := cookiesOf(session)
	if r = p.post(t, "/auth/portal/token:refresh", nil, ck, map[string]string{identityhttp.PortalCSRFHeader: ck[identityhttp.PortalCSRFCookie]}); r.code != 401 || p.kc.kicked != 1 {
		t.Fatalf("refresh after reset = %d kicked=%d", r.code, p.kc.kicked)
	}
	if r = p.post(t, "/auth/portal/login", portalLogin("ad.portal.reset", fresh, ""), nil, nil); r.code != 200 {
		t.Fatalf("login with the new password = %d", r.code)
	}
	if n := count(ctx, t, p.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' IN ('auth.portal_setup_password', 'auth.portal_password_reset_requested', 'auth.portal_password_reset', 'auth.portal_otp_setup_email_sent')`); n != 5 {
		t.Fatalf("setup/reset audits = %d", n)
	}
}
