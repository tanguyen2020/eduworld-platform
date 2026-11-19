package http_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	identityhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
)

const (
	pwTenant   = "Mk-T3nant-Sample!" // sample secrets: never seen in logs/audit (F012-AC-31)
	pwPlatform = "Mk-Pl4tform-Sample!"
	otpSample  = "731905"
)

func loginBody(username, password, otp string, device uuid.UUID) string {
	m := map[string]any{"username": username, "password": password, "device_id": device.String(), "locale": "vi-VN"}
	if otp != "" {
		m["otp"] = otp
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func decodeBody(t *testing.T, r result) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(r.body), &m); err != nil {
		t.Fatalf("body %q: %v", r.body, err)
	}
	return m
}

// seedPortal: ad.truong.a (tenant staff, no OTP), sa.admin (platform staff, OTP), a student account, a staff
// without role and a deactivated staff.
func seedPortal(e *portalEnv) {
	e.idp.accounts["ad.truong.a"] = fakeKCAccount{password: pwTenant, sub: "kc-ad"}
	e.idp.accounts["sa.admin"] = fakeKCAccount{password: pwPlatform, otp: otpSample, sub: "kc-sa"}
	e.idp.accounts["hs.an.3a"] = fakeKCAccount{password: pwTenant, sub: "kc-hs", outcome: port.ErrPortalAccessDenied}
	e.idp.accounts["nv.norole"] = fakeKCAccount{password: pwTenant, sub: "kc-norole"}
	e.idp.accounts["nv.off"] = fakeKCAccount{password: pwTenant, sub: "kc-off"}
	e.accounts.add("kc-ad", "ad.truong.a", write.PortalAccount{UserType: "staff", Status: "active", HasPortalRole: true})
	e.accounts.add("kc-sa", "sa.admin", write.PortalAccount{UserType: "staff", Status: "active", HasPortalRole: true})
	e.accounts.add("kc-norole", "nv.norole", write.PortalAccount{UserType: "staff", Status: "active"})
	e.accounts.add("kc-off", "nv.off", write.PortalAccount{UserType: "staff", Status: "deactivated", HasPortalRole: true})
}

// F012-AC-27 (TC-AUTH-093): 200 PortalTokenResponse, no-store, refresh cookie HttpOnly/Secure/Strict on the
// portal auth path, CSRF cookie readable on /, audit success; X-Tenant-Id is ignored.
func TestPortalLoginSuccess(t *testing.T) {
	e := newPortalEnv(t)
	seedPortal(e)
	r := e.post("/auth/portal/login", loginBody("  AD.Truong.A ", pwTenant, "", uuid.New()), nil, map[string]string{"X-Tenant-Id": uuid.NewString()})
	if r.code != http.StatusOK {
		t.Fatalf("login = %d %s", r.code, r.body)
	}
	b := decodeBody(t, r)
	if b["access_token"] == "" || b["token_type"] != "Bearer" || b["expires_in"] != float64(300) || b["refresh_expires_in"] != float64(1800) ||
		b["mfa"] != false || b["auth_time"] != "2026-11-19T08:00:00Z" {
		t.Fatalf("body = %v", b)
	}
	if _, leaked := b["refresh_token"]; leaked {
		t.Fatal("refresh token must never be in the body")
	}
	if r.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", r.header.Get("Cache-Control"))
	}
	rt, csrf := r.cookies[identityhttp.PortalRefreshCookie], r.cookies[identityhttp.PortalCSRFCookie]
	if rt == nil || !rt.HttpOnly || !rt.Secure || rt.SameSite != http.SameSiteStrictMode || rt.Path != "/api/v1/auth/portal" || rt.MaxAge != 1800 || rt.Domain != "" {
		t.Fatalf("refresh cookie = %+v", rt)
	}
	if csrf == nil || csrf.HttpOnly || !csrf.Secure || csrf.SameSite != http.SameSiteStrictMode || csrf.Path != "/" || len(csrf.Value) != 43 || csrf.Domain != "" {
		t.Fatalf("csrf cookie = %+v", csrf)
	}
	ok := e.audit.find(command.AuditPortalLogin, audit.ResultSuccess)
	if len(ok) != 1 || ok[0].TargetType != "user" || ok[0].Metadata["mfa"] != false {
		t.Fatalf("audit = %+v", ok)
	}
	// Platform staff with OTP: amr otp → mfa = true.
	r = e.post("/auth/portal/login", loginBody("sa.admin", pwPlatform, otpSample, uuid.New()), nil, nil)
	if r.code != http.StatusOK || decodeBody(t, r)["mfa"] != true {
		t.Fatalf("platform login = %d %s", r.code, r.body)
	}
	if testutil.ToFloat64(e.metrics.LoginCount(port.LoginSuccess)) != 2 {
		t.Fatal("eduworld_portal_login_total{result=success}")
	}
	// The password slices handed to Keycloak are zeroed after the call (best effort, F012-AC-31).
	for _, p := range e.idp.passwords {
		if strings.Trim(string(p), "\x00") != "" {
			t.Fatalf("password bytes not cleared: %q", p)
		}
	}
}

// F012-AC-28 (TC-AUTH-094): every credential failure is the same 401 INVALID_CREDENTIALS (status, code,
// message, fields), nothing says that OTP is needed, timing does not depend on the variant, the audit has
// no raw username.
func TestPortalLoginGenericFailure(t *testing.T) {
	e := newPortalEnv(t)
	seedPortal(e)
	variants := []struct{ name, user, pw, otp string }{
		{"unknown user", "khong.ton.tai", pwTenant, ""},
		{"wrong password", "ad.truong.a", "wrong", ""},
		{"wrong otp", "sa.admin", pwPlatform, "000000"},
		{"missing otp (platform_staff)", "sa.admin", pwPlatform, ""},
		{"disabled or locked (Keycloak)", "nv.locked", pwTenant, ""},
	}
	var first map[string]any
	durations := map[string][]time.Duration{}
	for round := range 15 {
		for _, v := range variants {
			start := time.Now()
			r := e.post("/auth/portal/login", loginBody(v.user, v.pw, v.otp, uuid.New()), nil, nil)
			durations[v.name] = append(durations[v.name], time.Since(start))
			if r.code != http.StatusUnauthorized {
				t.Fatalf("%s: %d %s", v.name, r.code, r.body)
			}
			b := decodeBody(t, r)
			delete(b, "instance")
			delete(b, "trace_id")
			if b["code"] != "INVALID_CREDENTIALS" {
				t.Fatalf("%s: %v", v.name, b)
			}
			if first == nil {
				first = b
			} else if fmt.Sprint(b) != fmt.Sprint(first) {
				t.Fatalf("%s: body %v differs from %v", v.name, b, first)
			}
			for k := range b {
				if strings.Contains(strings.ToLower(k), "otp") || strings.Contains(strings.ToLower(k), "mfa") {
					t.Fatalf("%s: field %s hints at OTP", v.name, k)
				}
			}
			if round == 0 {
				e.clock.add(time.Minute) // stay under portal_login_rate
			}
		}
		e.clock.add(time.Minute)
	}
	median := func(d []time.Duration) time.Duration {
		s := slices.Clone(d)
		sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
		return s[len(s)/2]
	}
	lo, hi := time.Hour, time.Duration(0)
	for _, d := range durations {
		m := median(d)
		lo, hi = min(lo, m), max(hi, m)
	}
	if hi-lo >= 50*time.Millisecond {
		t.Fatalf("p50 spread %v ≥ 50ms", hi-lo)
	}
	for _, ev := range e.audit.find(command.AuditPortalLogin, audit.ResultDenied) {
		if ev.Metadata["username_hash"] == nil || strings.Contains(fmt.Sprint(ev), "ad.truong.a") || strings.Contains(fmt.Sprint(ev), "sa.admin") {
			t.Fatalf("audit must carry a hash, never the username: %+v", ev)
		}
	}
}

// F012-AC-29 (TC-AUTH-095): student blocked by the flow → 403 PORTAL_ACCESS_STAFF_ONLY; staff without a
// portal role → same; EduWorld user deactivated → 403 ACCOUNT_DEACTIVATED and the new Keycloak session is
// revoked.
func TestPortalLoginRefusals(t *testing.T) {
	e := newPortalEnv(t)
	seedPortal(e)
	cases := []struct{ user, code string }{
		{"hs.an.3a", "PORTAL_ACCESS_STAFF_ONLY"},
		{"nv.norole", "PORTAL_ACCESS_STAFF_ONLY"},
		{"nv.off", "ACCOUNT_DEACTIVATED"},
	}
	for _, c := range cases {
		r := e.post("/auth/portal/login", loginBody(c.user, pwTenant, "", uuid.New()), nil, nil)
		if r.code != http.StatusForbidden || decodeBody(t, r)["code"] != c.code || len(r.cookies) != 0 {
			t.Fatalf("%s: %d %s", c.user, r.code, r.body)
		}
	}
	if len(e.idp.logouts) != 2 { // the sessions of nv.norole and nv.off were created then revoked
		t.Fatalf("revoked sessions = %v", e.idp.logouts)
	}
	if n := len(e.audit.find(command.AuditPortalLogin, audit.ResultDenied)); n != 3 {
		t.Fatalf("denied audits = %d", n)
	}
}

func csrfHeaders(v string) map[string]string {
	return map[string]string{identityhttp.PortalCSRFHeader: v}
}

func sessionCookies(r result) map[string]string {
	return map[string]string{identityhttp.PortalRefreshCookie: r.cookies[identityhttp.PortalRefreshCookie].Value,
		identityhttp.PortalCSRFCookie: r.cookies[identityhttp.PortalCSRFCookie].Value}
}

// F012-AC-30 (TC-AUTH-098): CSRF double-submit before Keycloak; rotation of refresh token and CSRF;
// reuse → 401 REFRESH_TOKEN_INVALID with both cookies cleared; logout revokes, clears, is idempotent.
func TestPortalRefreshAndLogout(t *testing.T) {
	e := newPortalEnv(t)
	seedPortal(e)
	login := e.post("/auth/portal/login", loginBody("ad.truong.a", pwTenant, "", uuid.New()), nil, nil)
	ck := sessionCookies(login)
	csrf := ck[identityhttp.PortalCSRFCookie]
	other := strings.Repeat("A", 43)

	for name, req := range map[string]struct {
		cookies map[string]string
		hdr     map[string]string
	}{
		"no header":      {ck, nil},
		"header differs": {ck, csrfHeaders(other)},
		"no csrf cookie": {map[string]string{identityhttp.PortalRefreshCookie: ck[identityhttp.PortalRefreshCookie]}, csrfHeaders(csrf)},
	} {
		for _, path := range []string{"/auth/portal/token:refresh", "/auth/portal/logout"} {
			r := e.post(path, "", req.cookies, req.hdr)
			if r.code != http.StatusForbidden || decodeBody(t, r)["code"] != "CSRF_TOKEN_INVALID" {
				t.Fatalf("%s %s: %d %s", name, path, r.code, r.body)
			}
		}
	}
	if _, refreshes := e.idp.count(); refreshes != 0 || len(e.idp.logouts) != 0 {
		t.Fatal("Keycloak must not be called before the CSRF check")
	}
	if testutil.ToFloat64(e.metrics.CSRFCount("refreshPortalToken")) != 3 || testutil.ToFloat64(e.metrics.CSRFCount("portalLogout")) != 3 {
		t.Fatal("eduworld_portal_csrf_rejected_total")
	}
	// Missing refresh cookie → 401 and cookies cleared.
	r := e.post("/auth/portal/token:refresh", "", map[string]string{identityhttp.PortalCSRFCookie: csrf}, csrfHeaders(csrf))
	if r.code != http.StatusUnauthorized || decodeBody(t, r)["code"] != "REFRESH_TOKEN_INVALID" || r.cookies[identityhttp.PortalRefreshCookie].MaxAge >= 0 {
		t.Fatalf("no refresh cookie: %d %s", r.code, r.body)
	}
	// Rotation.
	e.clock.add(4 * time.Minute)
	r = e.post("/auth/portal/token:refresh", "", ck, csrfHeaders(csrf))
	if r.code != http.StatusOK || r.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("refresh = %d %s", r.code, r.body)
	}
	b := decodeBody(t, r)
	if b["auth_time"] != "2026-11-19T08:00:00Z" {
		t.Fatalf("auth_time must stay the password time: %v", b)
	}
	next := sessionCookies(r)
	if next[identityhttp.PortalRefreshCookie] == ck[identityhttp.PortalRefreshCookie] || next[identityhttp.PortalCSRFCookie] == csrf {
		t.Fatal("refresh token and CSRF must rotate")
	}
	// Reuse of the rotated token (second tab) → 401 and both cookies cleared.
	r = e.post("/auth/portal/token:refresh", "", ck, csrfHeaders(csrf))
	if r.code != http.StatusUnauthorized || decodeBody(t, r)["code"] != "REFRESH_TOKEN_INVALID" ||
		r.cookies[identityhttp.PortalRefreshCookie].MaxAge >= 0 || r.cookies[identityhttp.PortalCSRFCookie].MaxAge >= 0 {
		t.Fatalf("reuse = %d %s %v", r.code, r.body, r.cookies)
	}
	// Logout revokes the session (sid), clears cookies, 204; again without cookie → 204.
	r = e.post("/auth/portal/logout", "", next, csrfHeaders(next[identityhttp.PortalCSRFCookie]))
	if r.code != http.StatusNoContent || r.cookies[identityhttp.PortalRefreshCookie].MaxAge >= 0 || r.cookies[identityhttp.PortalCSRFCookie].MaxAge >= 0 {
		t.Fatalf("logout = %d %v", r.code, r.cookies)
	}
	if len(e.idp.logouts) != 1 || e.idp.logouts[0] != next[identityhttp.PortalRefreshCookie] {
		t.Fatalf("logouts = %v", e.idp.logouts)
	}
	r = e.post("/auth/portal/token:refresh", "", next, csrfHeaders(next[identityhttp.PortalCSRFCookie]))
	if r.code != http.StatusUnauthorized {
		t.Fatalf("refresh after logout = %d", r.code)
	}
	r = e.post("/auth/portal/logout", "", map[string]string{identityhttp.PortalCSRFCookie: csrf}, csrfHeaders(csrf))
	if r.code != http.StatusNoContent {
		t.Fatalf("second logout = %d %s", r.code, r.body)
	}
	if n := len(e.audit.find(command.AuditPortalLogout, audit.ResultSuccess)); n != 1 {
		t.Fatalf("logout audits = %d", n)
	}
}

// Refreshes of an MFA (platform) session stop once the password + OTP entry is older than
// EW_OIDC_PLATFORM_MAX_AUTH_AGE (direct grant tokens carry no auth_time — F004 research R5, ADR-0091 §4).
func TestPortalRefreshMaxAuthAge(t *testing.T) {
	e := newPortalEnv(t)
	seedPortal(e)
	r := e.post("/auth/portal/login", loginBody("sa.admin", pwPlatform, otpSample, uuid.New()), nil, nil)
	ck := sessionCookies(r)
	for range 3 {
		e.clock.add(2 * time.Hour)
		r = e.post("/auth/portal/token:refresh", "", ck, csrfHeaders(ck[identityhttp.PortalCSRFCookie]))
		if r.code != http.StatusOK || decodeBody(t, r)["mfa"] != true {
			t.Fatalf("refresh = %d %s", r.code, r.body)
		}
		ck = sessionCookies(r)
	}
	e.clock.add(2*time.Hour + time.Minute) // 8h01 after the password entry
	r = e.post("/auth/portal/token:refresh", "", ck, csrfHeaders(ck[identityhttp.PortalCSRFCookie]))
	if r.code != http.StatusUnauthorized || decodeBody(t, r)["code"] != "REFRESH_TOKEN_INVALID" {
		t.Fatalf("refresh after max auth age = %d %s", r.code, r.body)
	}
	// An MFA session whose record is lost cannot prove its auth time: refused (fail-safe).
	r = e.post("/auth/portal/login", loginBody("sa.admin", pwPlatform, otpSample, uuid.New()), nil, nil)
	ck = sessionCookies(r)
	e.sessions.mu.Lock()
	clear(e.sessions.m)
	e.sessions.mu.Unlock()
	r = e.post("/auth/portal/token:refresh", "", ck, csrfHeaders(ck[identityhttp.PortalCSRFCookie]))
	if r.code != http.StatusUnauthorized {
		t.Fatalf("refresh without session record = %d", r.code)
	}
	// A deactivated user loses the session at the next refresh.
	r = e.post("/auth/portal/login", loginBody("ad.truong.a", pwTenant, "", uuid.New()), nil, nil)
	ck = sessionCookies(r)
	e.accounts.bySub["kc-ad"].Status = "deactivated"
	r = e.post("/auth/portal/token:refresh", "", ck, csrfHeaders(ck[identityhttp.PortalCSRFCookie]))
	if r.code != http.StatusForbidden || decodeBody(t, r)["code"] != "ACCOUNT_DEACTIVATED" || r.cookies[identityhttp.PortalRefreshCookie].MaxAge >= 0 {
		t.Fatalf("deactivated refresh = %d %s", r.code, r.body)
	}
}

// F012-AC-31 (TC-AUTH-100): no password, OTP, refresh token, CSRF value or username in logs or audit;
// no Idempotency-Key is needed.
func TestPortalNoSecretsInLogsOrAudit(t *testing.T) {
	e := newPortalEnv(t)
	seedPortal(e)
	login := e.post("/auth/portal/login", loginBody("sa.admin", pwPlatform, otpSample, uuid.New()), nil, nil)
	ck := sessionCookies(login)
	_ = e.post("/auth/portal/login", loginBody("sa.admin", "Wr0ng-Sample-Pw", otpSample, uuid.New()), nil, nil)
	r := e.post("/auth/portal/token:refresh", "", ck, csrfHeaders(ck[identityhttp.PortalCSRFCookie]))
	ck2 := sessionCookies(r)
	_ = e.post("/auth/portal/token:refresh", "", ck, csrfHeaders("bad-"+ck[identityhttp.PortalCSRFCookie]))
	_ = e.post("/auth/portal/logout", "", ck2, csrfHeaders(ck2[identityhttp.PortalCSRFCookie]))
	if e.logs.Len() == 0 {
		t.Fatal("expected access/use-case logs")
	}
	haystack := e.logs.String() + fmt.Sprintf("%+v", e.audit.events)
	for _, secret := range []string{pwPlatform, "Wr0ng-Sample-Pw", otpSample, ck[identityhttp.PortalRefreshCookie], ck[identityhttp.PortalCSRFCookie],
		ck2[identityhttp.PortalRefreshCookie], "sa.admin", decodeBody(t, login)["access_token"].(string)} {
		if strings.Contains(haystack, secret) {
			t.Fatalf("secret %q found in logs/audit", secret)
		}
	}
	for id, op := range identityhttp.PortalAuthOperations {
		if op.Idempotency != 0 || !op.Public {
			t.Fatalf("%s: public, no Idempotency-Key", id)
		}
	}
}

// F012-AC-32 (TC-AUTH-099): portal_login_rate per hashed username (several devices), per device_id and
// globally → 429 + Retry-After; never per IP (50 usernames from one client pass when spread).
func TestPortalLoginRate(t *testing.T) {
	e := newPortalEnv(t)
	seedPortal(e)
	// Per username across devices.
	for i := range 11 {
		r := e.post("/auth/portal/login", loginBody("ad.truong.a", "wrong", "", uuid.New()), nil, nil)
		if i < 10 && r.code != http.StatusUnauthorized || i == 10 && (r.code != http.StatusTooManyRequests || r.header.Get("Retry-After") == "") {
			t.Fatalf("username attempt %d = %d %v", i, r.code, r.header)
		}
	}
	if testutil.ToFloat64(e.metrics.LoginCount(port.LoginRateLimited)) != 1 {
		t.Fatal("rate_limited metric")
	}
	e.clock.add(time.Minute)
	// Per device across usernames.
	dev := uuid.New()
	for i := range 11 {
		r := e.post("/auth/portal/login", loginBody(fmt.Sprintf("u%d.dev", i), "x", "", dev), nil, nil)
		if i == 10 && r.code != http.StatusTooManyRequests {
			t.Fatalf("device attempt %d = %d", i, r.code)
		}
	}
	e.clock.add(time.Minute)
	// Global: 20 per second.
	codes := map[int]int{}
	for i := range 21 {
		codes[e.post("/auth/portal/login", loginBody(fmt.Sprintf("g%d.user", i), "x", "", uuid.New()), nil, nil).code]++
	}
	if codes[http.StatusTooManyRequests] != 1 {
		t.Fatalf("global = %v", codes)
	}
	// 50 usernames from one client (same IP), new device each, spread over time: never limited by IP.
	for i := range 50 {
		if i%10 == 0 {
			e.clock.add(time.Second)
		}
		if r := e.post("/auth/portal/login", loginBody(fmt.Sprintf("ip%d.user", i), "x", "", uuid.New()), nil, nil); r.code != http.StatusUnauthorized {
			t.Fatalf("user %d from one IP = %d", i, r.code)
		}
	}
	// Refresh: per Keycloak session (no device_id in the request).
	e.clock.add(time.Minute)
	login := e.post("/auth/portal/login", loginBody("ad.truong.a", pwTenant, "", uuid.New()), nil, nil)
	ck := sessionCookies(login)
	limited := false
	for range 31 {
		r := e.post("/auth/portal/token:refresh", "", ck, csrfHeaders(ck[identityhttp.PortalCSRFCookie]))
		if r.code == http.StatusTooManyRequests {
			limited = r.header.Get("Retry-After") != ""
			break
		}
		ck = sessionCookies(r)
	}
	if !limited {
		t.Fatal("refresh limit per session not applied")
	}
}
