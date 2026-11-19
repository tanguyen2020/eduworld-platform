//go:build integration

package integration

import (
	"context"
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
	setup   map[string]bool // username → required actions pending
}

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
		"azp": "eduworld-portal", "typ": "Bearer"})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"access_token": at, "refresh_token": rt, "expires_in": 300, "refresh_expires_in": 1800,
		"token_type": "Bearer", "session_state": sid})
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
	if f.Get("client_id") != "eduworld-portal" || f.Get("client_secret") != "portal-secret" {
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
		if u.student {
			w.Header().Set("Content-Type", "text/html;charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, "<html>portalStudentDenied</html>")
			return
		}
		if u.otp != "" && f.Get("otp") != u.otp {
			oauthErr(w, 401, "invalid_grant", "Invalid user credentials")
			return
		}
		if k.setup[f.Get("username")] {
			oauthErr(w, 400, "invalid_grant", "Account is not fully set up")
			return
		}
		sid := fmt.Sprintf("sid-%d", k.seq+1)
		k.sidUser[sid] = f.Get("username")
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

type portalIntegration struct {
	*env
	kc      *fakeKeycloak
	kcSrv   *httptest.Server
	vk      valkey.Client
	srv     *httptest.Server
	metrics *portalmetrics.Metrics
	auth    *command.PortalAuth
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
		sidAMR: map[string][]string{}, setup: map[string]bool{}}
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
		Cfg: command.PortalAuthConfig{Issuers: []string{kcIssuer}}}
	spec, err := httpx.LoadSpec(ctx, filepath.Join(testkit.RepoRoot(t), "specs", "openapi", "eduworld-api.v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	idp := httpxtest.NewIDP(t)
	s, err := httpx.NewServer(httpx.Deps{Auth: idp.AuthConfig(), Keys: idp.JWKS(t), Resolver: &httpxtest.Resolver{}, Spec: spec})
	if err != nil {
		t.Fatal(err)
	}
	p := &portalIntegration{env: e, kc: kc, kcSrv: kcSrv, vk: vk, metrics: m, auth: auth}
	if err := identityhttp.RegisterPortalAuth(s, p.handlers()); err != nil {
		t.Fatal(err)
	}
	p.srv = httptest.NewServer(s)
	t.Cleanup(p.srv.Close)
	return p
}

func (p *portalIntegration) handlers() identityhttp.PortalAuthHandlers {
	return identityhttp.PortalAuthHandlers{Session: p.auth, Metrics: p.metrics}
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
