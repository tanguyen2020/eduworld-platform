package http_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/portalauth"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
)

func jsonBody(m map[string]any) string {
	b, _ := json.Marshal(m)
	return string(b)
}

func strs(v any) []string {
	var out []string
	list, _ := v.([]any)
	for _, x := range list {
		s, _ := x.(string)
		out = append(out, s)
	}
	return out
}

func fieldCodes(b map[string]any) []string {
	var out []string
	list, _ := b["errors"].([]any)
	for _, e := range list {
		m, _ := e.(map[string]any)
		c, _ := m["code"].(string)
		out = append(out, c)
	}
	return out
}

// seedSetup: nv.moi has a temporary password (UPDATE_PASSWORD) and, from before ADR-0094, a pending OTP
// enrolment (CONFIGURE_TOTP). The fake direct grant answers "not fully set up" while Keycloak has required
// actions for the user.
func seedSetup(e *portalEnv) *write.PortalAccount {
	e.idp.accounts["nv.moi"] = fakeKCAccount{password: pwTenant, sub: "kc-moi"}
	e.kc.logins = map[string]string{"nv.moi": "kc-moi"}
	e.kc.actions["kc-moi"] = []string{port.KCActionUpdatePassword, portalauth.ActionLegacyConfigureTOTP}
	e.idp.pending = func(username string) bool {
		e.kc.mu.Lock()
		defer e.kc.mu.Unlock()
		id, ok := e.kc.logins[username]
		return ok && len(e.kc.actions[id]) > 0
	}
	return e.accounts.add("kc-moi", "nv.moi", write.PortalAccount{UserType: "staff", Status: "active", HasPortalRole: true})
}

// F012-AC-33, AC-34, AC-35 (TC-AUTH-096) as changed by ADR-0094: ACCOUNT_SETUP_REQUIRED + one-time ticket +
// update_password only (the legacy CONFIGURE_TOTP is dropped, never a step, no OTP email); policy violation
// keeps the ticket; success removes UPDATE_PASSWORD and the next login succeeds with the password alone;
// used, expired or orphaned tickets → 401 SETUP_TICKET_INVALID.
func TestPortalSetupPassword(t *testing.T) {
	e := newPortalEnv(t)
	acct := seedSetup(e)
	login := func() map[string]any {
		t.Helper()
		r := e.post("/auth/portal/login", loginBody("nv.moi", pwTenant, uuid.New()), nil, nil)
		if r.code != http.StatusForbidden || len(r.cookies) != 0 {
			t.Fatalf("login = %d %s", r.code, r.body)
		}
		b := decodeBody(t, r)
		if b["code"] != "ACCOUNT_SETUP_REQUIRED" || len(b["setup_ticket"].(string)) < 32 {
			t.Fatalf("setup required = %v", b)
		}
		return b
	}
	b := login()
	if !slices.Equal(strs(b["required_steps"]), []string{"update_password"}) {
		t.Fatalf("steps = %v", b)
	}
	if _, has := b["otp_setup_email_sent"]; has {
		t.Fatalf("no OTP field since ADR-0094: %v", b)
	}
	if slices.Contains(e.kc.actions["kc-moi"], portalauth.ActionLegacyConfigureTOTP) || len(e.kc.emails) != 0 {
		t.Fatalf("legacy CONFIGURE_TOTP must be dropped without email: %+v", e.kc)
	}
	if _, stored := e.tickets.m[b["setup_ticket"].(string)]; stored || len(e.tickets.m) != 1 {
		t.Fatal("the ticket is stored by its hash only")
	}
	if ev := e.audit.find(command.AuditPortalLogin, audit.ResultDenied); len(ev) != 1 || ev[0].Reason != "ACCOUNT_SETUP_REQUIRED" || ev[0].TargetID != acct.UserID.String() {
		t.Fatalf("audit = %+v", ev)
	}
	ticket := b["setup_ticket"].(string)
	setup := func(ticket, pw string) result {
		return e.post("/auth/portal/setup:password", jsonBody(map[string]any{"setup_ticket": ticket, "new_password": pw}), nil, nil)
	}
	// Realm policy (fake: ≥ 8 characters) → 422 PASSWORD_POLICY_VIOLATION, ticket still valid.
	r := setup(ticket, "short")
	if r.code != http.StatusUnprocessableEntity || !slices.Contains(fieldCodes(decodeBody(t, r)), "PASSWORD_POLICY_VIOLATION") {
		t.Fatalf("weak = %d %s", r.code, r.body)
	}
	r = setup(ticket, "Mk-New-Passw0rd")
	if r.code != http.StatusOK || r.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("setup = %d %s", r.code, r.body)
	}
	b = decodeBody(t, r)
	if steps, ok := b["remaining_steps"].([]any); !ok || len(steps) != 0 || len(b) != 1 {
		t.Fatalf("result = %v", b)
	}
	if e.kc.passwords["kc-moi"] != "Mk-New-Passw0rd" || len(e.kc.actions["kc-moi"]) != 0 || len(e.kc.emails) != 0 {
		t.Fatalf("keycloak = %+v", e.kc)
	}
	if len(e.audit.find(command.AuditPortalSetupPassword, audit.ResultSuccess)) != 1 {
		t.Fatal("setup audits")
	}
	// One time.
	if r = setup(ticket, "Mk-New-Passw0rd"); r.code != http.StatusUnauthorized || decodeBody(t, r)["code"] != "SETUP_TICKET_INVALID" {
		t.Fatalf("reuse = %d %s", r.code, r.body)
	}
	// Nothing left: the password alone signs in.
	if r = e.post("/auth/portal/login", loginBody("nv.moi", pwTenant, uuid.New()), nil, nil); r.code != http.StatusOK {
		t.Fatalf("login after setup = %d %s", r.code, r.body)
	}
	// Expired ticket.
	e.kc.actions["kc-moi"] = []string{port.KCActionUpdatePassword}
	b = login()
	e.clock.add(command.DefaultPortalSetupTicketTTL + time.Second)
	if r = setup(b["setup_ticket"].(string), "Mk-New-Passw0rd"); r.code != http.StatusUnauthorized {
		t.Fatalf("expired = %d", r.code)
	}
	// Account deactivated after the ticket was issued (edge case: the ticket dies with the account).
	b = login()
	e.accounts.bySub["kc-moi"].Status = "deactivated"
	if r = setup(b["setup_ticket"].(string), "Mk-New-Passw0rd"); r.code != http.StatusUnauthorized {
		t.Fatalf("deactivated = %d", r.code)
	}
	// Unknown ticket.
	if r = setup(strings.Repeat("x", 43), "Mk-New-Passw0rd"); r.code != http.StatusUnauthorized {
		t.Fatalf("unknown = %d", r.code)
	}
}

// F012-AC-36, AC-37 (TC-AUTH-097): forgot always 202 without doing the work in the request; only active
// staff get an email with the link only; DB holds the hash only; one live token per user; reset is single
// use, revokes every session; policy violation and Keycloak failures keep the token.
// ADR-0094: a platform account still carrying the CONFIGURE_TOTP required action of ADR-0091 (enrolled before
// OTP was removed, realm not re-applied yet) is not locked out: module identity drops the action through the
// Admin API (audited), retries the password grant once and the user is signed in with the password alone.
func TestPortalLoginDropsLegacyOTPAction(t *testing.T) {
	e := newPortalEnv(t)
	seedPortal(e)
	e.idp.accounts["sm.antoan"] = fakeKCAccount{password: pwPlatform, sub: "kc-sm", platform: true}
	e.accounts.add("kc-sm", "sm.antoan", write.PortalAccount{UserType: "staff", Status: "active", HasPortalRole: true})
	e.kc.logins = map[string]string{"sm.antoan": "kc-sm"}
	e.kc.actions["kc-sm"] = []string{portalauth.ActionLegacyConfigureTOTP}
	e.idp.pending = func(username string) bool {
		e.kc.mu.Lock()
		defer e.kc.mu.Unlock()
		return len(e.kc.actions[e.kc.logins[username]]) > 0
	}
	r := e.post("/auth/portal/login", loginBody("sm.antoan", pwPlatform, uuid.New()), nil, nil)
	if r.code != http.StatusOK || decodeBody(t, r)["access_token"] == "" {
		t.Fatalf("login = %d %s", r.code, r.body)
	}
	if len(e.kc.actions["kc-sm"]) != 0 || len(e.kc.emails) != 0 || e.idp.grants != 2 {
		t.Fatalf("keycloak = %+v grants=%d", e.kc, e.idp.grants)
	}
	if ev := e.audit.find(command.AuditPortalLegacyOTPActionRemoved, audit.ResultSuccess); len(ev) != 1 || ev[0].TargetID != "kc-sm" {
		t.Fatalf("audit = %+v", ev)
	}
	for _, p := range e.idp.passwords {
		if strings.Trim(string(p), "\x00") != "" {
			t.Fatalf("password bytes not cleared: %q", p)
		}
	}
	// A wrong password never reaches the Admin API.
	e.kc.actions["kc-sm"] = []string{portalauth.ActionLegacyConfigureTOTP}
	if r = e.post("/auth/portal/login", loginBody("sm.antoan", "wrong", uuid.New()), nil, nil); r.code != http.StatusUnauthorized ||
		len(e.kc.actions["kc-sm"]) != 1 {
		t.Fatalf("wrong password = %d %s %+v", r.code, r.body, e.kc.actions)
	}
}

func TestPortalForgotAndReset(t *testing.T) {
	e := newPortalEnv(t)
	seedPortal(e)
	staff := e.accounts.bySub["kc-ad"]
	staff.Email = "ad.truong.a@school.test"
	e.accounts.byLogin["ad.truong.a@school.test"] = staff
	e.accounts.add("kc-hs", "hs.an.3a", write.PortalAccount{UserType: "student", Status: "active", Email: "hs@school.test"})
	forgot := func(id string) result {
		return e.post("/auth/portal/password:forgot", jsonBody(map[string]any{"identifier": id, "device_id": uuid.NewString(), "locale": "vi-VN"}), nil, nil)
	}
	// The request never does the lookup itself: the work is queued, the answer is the same for everyone.
	var queued []func()
	e.auth.Background = func(f func()) { queued = append(queued, f) }
	var bodies []string
	for _, id := range []string{"AD.Truong.A@School.test", "hs.an.3a", "khong.ton.tai"} {
		r := forgot(id)
		if r.code != http.StatusAccepted {
			t.Fatalf("%s: %d %s", id, r.code, r.body)
		}
		bodies = append(bodies, r.body)
	}
	if len(e.mailer.sent) != 0 || len(e.resets.rows) != 0 || bodies[0] != bodies[1] || bodies[1] != bodies[2] {
		t.Fatal("forgot must answer before any lookup, identically")
	}
	for _, f := range queued {
		f()
	}
	e.auth.Background = func(f func()) { f() }
	if len(e.mailer.sent) != 1 || e.mailer.sent[0].To != "ad.truong.a@school.test" {
		t.Fatalf("mails = %+v (students and unknown identifiers get nothing)", e.mailer.sent)
	}
	link := e.mailer.sent[0].Link
	token, ok := strings.CutPrefix(link, "https://portal.eduworld.test/reset-password?token=")
	if !ok || len(token) != 43 {
		t.Fatalf("link = %q", link)
	}
	row := e.resets.rows[0]
	if row.TokenHash != portalauth.HashToken(token) || row.TokenHash == token || !row.ExpiresAt.Equal(e.clock.now().Add(30*time.Minute)) {
		t.Fatalf("row = %+v", row)
	}
	// A second request supersedes the first token.
	_ = forgot("ad.truong.a")
	newToken, _ := strings.CutPrefix(e.mailer.sent[1].Link, "https://portal.eduworld.test/reset-password?token=")
	reset := func(tok, pw string) result {
		return e.post("/auth/portal/password:reset", jsonBody(map[string]any{"token": tok, "new_password": pw, "device_id": uuid.NewString()}), nil, nil)
	}
	if r := reset(token, "Mk-Reset-Passw0rd"); r.code != http.StatusUnauthorized || decodeBody(t, r)["code"] != "PASSWORD_RESET_TOKEN_INVALID" {
		t.Fatalf("superseded = %d %s", r.code, r.body)
	}
	// Policy violation and Keycloak failure keep the token.
	if r := reset(newToken, "short"); r.code != http.StatusUnprocessableEntity || !slices.Contains(fieldCodes(decodeBody(t, r)), "PASSWORD_POLICY_VIOLATION") {
		t.Fatalf("weak = %d %s", r.code, r.body)
	}
	e.kc.failLogout = true
	if r := reset(newToken, "Mk-Reset-Passw0rd"); r.code != http.StatusServiceUnavailable {
		t.Fatalf("keycloak down = %d %s", r.code, r.body)
	}
	e.kc.failLogout = false
	if r := reset(newToken, "Mk-Reset-Passw0rd"); r.code != http.StatusNoContent {
		t.Fatalf("reset = %d %s", r.code, r.body)
	}
	if e.kc.passwords["kc-ad"] != "Mk-Reset-Passw0rd" || !slices.Contains(e.kc.loggedOut, "kc-ad") {
		t.Fatalf("keycloak = %+v", e.kc)
	}
	if r := reset(newToken, "Mk-Reset-Passw0rd"); r.code != http.StatusUnauthorized {
		t.Fatalf("second use = %d", r.code)
	}
	if len(e.audit.find(command.AuditPortalPasswordReset, audit.ResultSuccess)) != 1 || len(e.audit.find(command.AuditPortalPasswordResetRequested, audit.ResultSuccess)) != 2 {
		t.Fatal("reset audits")
	}
	// Expired token.
	_ = forgot("ad.truong.a")
	last, _ := strings.CutPrefix(e.mailer.sent[len(e.mailer.sent)-1].Link, "https://portal.eduworld.test/reset-password?token=")
	e.clock.add(31 * time.Minute)
	if r := reset(last, "Mk-Reset-Passw0rd"); r.code != http.StatusUnauthorized {
		t.Fatalf("expired = %d", r.code)
	}
	// No secret in logs or audit.
	haystack := e.logs.String() + fmt.Sprintf("%+v", e.audit.events)
	for _, s := range []string{token, newToken, "Mk-Reset-Passw0rd", "ad.truong.a@school.test"} {
		if strings.Contains(haystack, s) {
			t.Fatalf("%q found in logs/audit", s)
		}
	}
}

// F012-AC-32 for forgot (3/h per identifier, 10/h per device), reset (10/min per device) and setup (per
// ticket).
func TestPortalAccountRateLimits(t *testing.T) {
	e := newPortalEnv(t)
	seedPortal(e)
	last := result{}
	for range 4 {
		last = e.post("/auth/portal/password:forgot", jsonBody(map[string]any{"identifier": "ad.truong.a", "device_id": uuid.NewString()}), nil, nil)
	}
	if last.code != http.StatusTooManyRequests || last.header.Get("Retry-After") == "" {
		t.Fatalf("forgot per identifier = %d", last.code)
	}
	dev := uuid.NewString()
	for i := range 11 {
		last = e.post("/auth/portal/password:forgot", jsonBody(map[string]any{"identifier": "x" + string(rune('a'+i)) + ".user", "device_id": dev}), nil, nil)
	}
	if last.code != http.StatusTooManyRequests {
		t.Fatalf("forgot per device = %d", last.code)
	}
	for range 11 {
		last = e.post("/auth/portal/password:reset", jsonBody(map[string]any{"token": strings.Repeat("t", 43), "new_password": "Mk-Reset-Passw0rd", "device_id": dev}), nil, nil)
	}
	if last.code != http.StatusTooManyRequests {
		t.Fatalf("reset per device = %d", last.code)
	}
	for range 11 {
		last = e.post("/auth/portal/setup:password", jsonBody(map[string]any{"setup_ticket": strings.Repeat("s", 43), "new_password": "Mk-New-Passw0rd"}), nil, nil)
	}
	if last.code != http.StatusTooManyRequests {
		t.Fatalf("setup per ticket = %d", last.code)
	}
}
