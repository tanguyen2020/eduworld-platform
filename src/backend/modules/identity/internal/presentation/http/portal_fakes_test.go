package http_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/portalauth"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/portalmetrics"
	identityhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// TEST FAKES (AGENTS R17) of the ports of the portal login: they model the documented behaviour of Keycloak
// (direct grant, rotating refresh tokens) and of the stores, in memory. Production uses the adapters in
// infrastructure/keycloak and infrastructure/cache.

const testIssuer = "http://kc/realms/eduworld"

type fakeKCAccount struct {
	password string
	sub      string
	platform bool  // realm role platform_staff in the token (claim roles)
	outcome  error // forced outcome of the direct grant (access denied, setup required)
}

type fakeIdP struct {
	mu        sync.Mutex
	clock     func() time.Time
	accounts  map[string]fakeKCAccount
	refresh   map[string]string   // live refresh token → sid
	sids      map[string]string   // sid → subject
	roles     map[string][]string // sid → realm roles
	seq       int
	grants    int
	refreshes int
	logouts   []string
	passwords [][]byte // the slices received (zeroing check)
	fail      error
	// pending, when set, reports Keycloak required actions of a username ("Account is not fully set up").
	pending func(username string) bool
}

func newFakeIdP(clock func() time.Time) *fakeIdP {
	return &fakeIdP{clock: clock, accounts: map[string]fakeKCAccount{}, refresh: map[string]string{}, sids: map[string]string{},
		roles: map[string][]string{}}
}

func (f *fakeIdP) issue(sid string) port.PortalTokens {
	f.seq++
	rt := fmt.Sprintf("rt-%d-%s", f.seq, sid)
	f.refresh[rt] = sid
	return port.PortalTokens{AccessToken: fmt.Sprintf("at-%d", f.seq), RefreshToken: rt, ExpiresIn: 300, RefreshExpiresIn: 1800,
		Issuer: testIssuer, Subject: f.sids[sid], SessionID: sid, RealmRoles: f.roles[sid], IssuedAt: f.clock()}
}

func (f *fakeIdP) PasswordGrant(_ context.Context, username string, password []byte) (port.PortalTokens, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.grants++
	f.passwords = append(f.passwords, password)
	if f.fail != nil {
		return port.PortalTokens{}, f.fail
	}
	a, ok := f.accounts[username]
	switch {
	case !ok, string(password) != a.password:
		return port.PortalTokens{}, port.ErrPortalInvalidCredentials
	case a.outcome != nil:
		return port.PortalTokens{}, a.outcome
	case f.pending != nil && f.pending(username):
		return port.PortalTokens{}, port.ErrPortalSetupRequired
	}
	sid := fmt.Sprintf("sid-%s-%d", a.sub, f.seq+1)
	f.sids[sid] = a.sub
	f.roles[sid] = []string{"tenant_staff"}
	if a.platform {
		f.roles[sid] = []string{"platform_staff"}
	}
	return f.issue(sid), nil
}

func (f *fakeIdP) Refresh(_ context.Context, rt string) (port.PortalTokens, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refreshes++
	if f.fail != nil {
		return port.PortalTokens{}, f.fail
	}
	sid, ok := f.refresh[rt]
	if !ok {
		return port.PortalTokens{}, port.ErrPortalRefreshInvalid
	}
	delete(f.refresh, rt) // rotation: single use
	return f.issue(sid), nil
}

func (f *fakeIdP) Logout(_ context.Context, rt string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logouts = append(f.logouts, rt)
	sid := f.refresh[rt]
	for k, v := range f.refresh {
		if v == sid {
			delete(f.refresh, k)
		}
	}
	return nil
}

func (f *fakeIdP) SessionKey(rt string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if i := strings.Index(rt, "-sid-"); i > 0 {
		return portalauth.SessionKey(rt[i+1:])
	}
	return portalauth.Key("rt", rt)
}

func (f *fakeIdP) count() (grants, refreshes int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.grants, f.refreshes
}

type fakeAccounts struct {
	mu       sync.Mutex
	bySub    map[string]*write.PortalAccount
	byLogin  map[string]*write.PortalAccount
	kcByUser map[uuid.UUID]string
}

func newFakeAccounts() *fakeAccounts {
	return &fakeAccounts{bySub: map[string]*write.PortalAccount{}, byLogin: map[string]*write.PortalAccount{}, kcByUser: map[uuid.UUID]string{}}
}

func (f *fakeAccounts) add(sub, login string, a write.PortalAccount) *write.PortalAccount {
	f.mu.Lock()
	defer f.mu.Unlock()
	a.UserID = uuid.New()
	f.bySub[sub] = &a
	f.byLogin[login] = &a
	f.kcByUser[a.UserID] = sub
	return &a
}

func (f *fakeAccounts) get(m map[string]*write.PortalAccount, k string) *write.PortalAccount {
	f.mu.Lock()
	defer f.mu.Unlock()
	if a, ok := m[k]; ok {
		c := *a
		return &c
	}
	return nil
}

func (f *fakeAccounts) BySubject(_ context.Context, issuer, subject string) (*write.PortalAccount, error) {
	if issuer != testIssuer {
		return nil, nil
	}
	return f.get(f.bySub, subject), nil
}
func (f *fakeAccounts) ByKeycloakID(_ context.Context, _ []string, subject string) (*write.PortalAccount, error) {
	return f.get(f.bySub, subject), nil
}
func (f *fakeAccounts) ByID(_ context.Context, id uuid.UUID) (*write.PortalAccount, error) {
	return f.get(f.bySub, f.kcByUser[id]), nil
}
func (f *fakeAccounts) ActiveStaffByLogin(_ context.Context, login string) (*write.PortalAccount, error) {
	a := f.get(f.byLogin, login)
	if a == nil || a.UserType != "staff" || a.Status != "active" {
		return nil, nil
	}
	return a, nil
}
func (f *fakeAccounts) KeycloakSubject(_ context.Context, id uuid.UUID, _ []string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.kcByUser[id], nil
}

type fakeUoW struct{}

func (fakeUoW) Do(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

type fakeAudit struct {
	mu     sync.Mutex
	events []audit.Event
}

func (f *fakeAudit) Record(_ context.Context, e audit.Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, e)
	return nil
}

func (f *fakeAudit) find(action string, result audit.Result) []audit.Event {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []audit.Event
	for _, e := range f.events {
		if e.Action == action && e.Result == result {
			out = append(out, e)
		}
	}
	return out
}

type memSessions struct {
	mu sync.Mutex
	m  map[string]port.PortalSession
}

func (s *memSessions) Save(_ context.Context, k string, v port.PortalSession, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[k] = v
	return nil
}
func (s *memSessions) Get(_ context.Context, k string) (port.PortalSession, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[k]
	return v, ok, nil
}
func (s *memSessions) Delete(_ context.Context, k string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, k)
	return nil
}

type memLimiter struct {
	mu    sync.Mutex
	clock func() time.Time
	win   map[string]time.Time
	n     map[string]int
}

func (l *memLimiter) Allow(_ context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.clock()
	if start, ok := l.win[key]; !ok || now.Sub(start) >= window {
		l.win[key], l.n[key] = now, 0
	}
	l.n[key]++
	if l.n[key] > limit {
		return false, window - now.Sub(l.win[key]), nil
	}
	return true, 0, nil
}

type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}
func (c *testClock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// portalEnv is the real pkg/httpx chain (merged spec, validation, observability) over the real use cases
// with the fakes above.
type portalEnv struct {
	srv      http.Handler
	idp      *fakeIdP
	accounts *fakeAccounts
	audit    *fakeAudit
	sessions *memSessions
	clock    *testClock
	metrics  *portalmetrics.Metrics
	logs     *bytes.Buffer
	auth     *command.PortalAuth
	kc       *fakeKCAdmin
	tickets  *memTickets
	resets   *fakeResets
	mailer   *fakeMailer
}

func specPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "..", "..", "specs", "openapi", "eduworld-api.v1.yaml")
}

func newPortalEnv(t *testing.T) *portalEnv {
	t.Helper()
	clk := &testClock{t: time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC)}
	e := &portalEnv{idp: newFakeIdP(clk.now), accounts: newFakeAccounts(), audit: &fakeAudit{},
		sessions: &memSessions{m: map[string]port.PortalSession{}}, clock: clk, logs: &bytes.Buffer{}}
	logger := observability.NewLogger(e.logs, observability.LoggerOptions{Binary: "test", Level: slog.LevelDebug})
	prev := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(prev) })
	var err error
	if e.metrics, err = portalmetrics.New(nil); err != nil {
		t.Fatal(err)
	}
	e.auth = &command.PortalAuth{UoW: fakeUoW{}, Accounts: e.accounts, Audit: e.audit, IdP: e.idp, Sessions: e.sessions,
		Limiter: &memLimiter{clock: clk.now, win: map[string]time.Time{}, n: map[string]int{}}, Metrics: e.metrics,
		Cfg: command.PortalAuthConfig{Issuers: []string{testIssuer}, PortalBaseURL: "https://portal.eduworld.test/"}, Clock: clk.now,
		Background: func(f func()) { f() }}
	e.kc = &fakeKCAdmin{actions: map[string][]string{}, passwords: map[string]string{}, policyMin: 8}
	e.tickets = &memTickets{clock: clk.now, m: map[string]port.SetupTicket{}}
	e.resets = &fakeResets{}
	e.mailer = &fakeMailer{}
	e.auth.KC, e.auth.Tickets, e.auth.Resets, e.auth.Mailer = e.kc, e.tickets, e.resets, e.mailer
	e.auth.UoW = txUoW{r: e.resets}
	spec, err := httpx.LoadSpec(context.Background(), specPath())
	if err != nil {
		t.Fatal(err)
	}
	idp := httpxtest.NewIDP(t)
	obs := observability.NewForTest(logger)
	s, err := httpx.NewServer(httpx.Deps{Auth: idp.AuthConfig(), Keys: idp.JWKS(t), Resolver: &httpxtest.Resolver{}, Spec: spec,
		Logger: logger, Middlewares: []func(http.Handler) http.Handler{obs.HTTP}})
	if err != nil {
		t.Fatal(err)
	}
	if err := identityhttp.RegisterPortalAuth(s, e.handlers()); err != nil {
		t.Fatal(err)
	}
	e.srv = s
	return e
}

func (e *portalEnv) handlers() identityhttp.PortalAuthHandlers {
	return identityhttp.PortalAuthHandlers{Session: e.auth, Setup: e.auth, Metrics: e.metrics}
}

type result struct {
	code    int
	header  http.Header
	body    string
	cookies map[string]*http.Cookie
}

func (e *portalEnv) post(path, body string, cookies map[string]string, hdr map[string]string) result {
	req := httptest.NewRequest(http.MethodPost, "/api/v1"+path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.srv.ServeHTTP(rec, req)
	out := result{code: rec.Code, header: rec.Header(), body: rec.Body.String(), cookies: map[string]*http.Cookie{}}
	for _, c := range rec.Result().Cookies() {
		out.cookies[c.Name] = c
	}
	return out
}

// fakeKCAdmin is a TEST FAKE of the Keycloak Admin API calls of the setup/reset (svc-identity).
type fakeKCAdmin struct {
	mu         sync.Mutex
	logins     map[string]string   // login → Keycloak id (set by tests)
	actions    map[string][]string // Keycloak id → required actions
	passwords  map[string]string   // Keycloak id → password set through the Admin API
	policyMin  int                 // realm password policy: minimum length
	emails     []string            // Keycloak ids that received an execute-actions-email
	loggedOut  []string
	failLogout bool
}

func (k *fakeKCAdmin) FindUserByLogin(_ context.Context, login string) (port.KCUser, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if id, ok := k.logins[login]; ok {
		return port.KCUser{ID: id, Username: login, Enabled: true}, nil
	}
	return port.KCUser{}, port.ErrKCNotFound
}
func (k *fakeKCAdmin) RequiredActions(_ context.Context, id string) ([]string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return slices.Clone(k.actions[id]), nil
}
func (k *fakeKCAdmin) ResetPassword(_ context.Context, id string, pw []byte) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(pw) < k.policyMin {
		return port.ErrKCPasswordPolicy
	}
	k.passwords[id] = string(pw)
	return nil
}
func (k *fakeKCAdmin) RemoveRequiredAction(_ context.Context, id, action string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.actions[id] = slices.DeleteFunc(k.actions[id], func(a string) bool { return a == action })
	return nil
}
func (k *fakeKCAdmin) ExecuteActionsEmail(_ context.Context, id string, actions ...string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.emails = append(k.emails, id)
	return nil
}
func (k *fakeKCAdmin) LogoutUser(_ context.Context, id string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.failLogout {
		return errors.New("keycloak down")
	}
	k.loggedOut = append(k.loggedOut, id)
	return nil
}

// memTickets is a TEST FAKE of the Valkey setup ticket store (GETDEL semantics, expiry by the test clock).
type memTickets struct {
	mu    sync.Mutex
	clock func() time.Time
	m     map[string]port.SetupTicket
}

func (s *memTickets) Save(_ context.Context, h string, t port.SetupTicket) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[h] = t
	return nil
}
func (s *memTickets) Take(_ context.Context, h string) (port.SetupTicket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.m[h]
	delete(s.m, h)
	if !ok || !t.ExpiresAt.After(s.clock()) {
		return port.SetupTicket{}, port.ErrSetupTicketInvalid
	}
	return t, nil
}

// fakeResets is a TEST FAKE of identity.portal_password_reset_tokens (hash only, one live token per user).
type fakeResets struct {
	mu   sync.Mutex
	rows []fakeResetRow
}

type fakeResetRow struct {
	write.PortalResetToken
	used, deleted bool
}

func (f *fakeResets) Issue(_ context.Context, t write.PortalResetToken) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for i := range f.rows {
		if f.rows[i].UserID == t.UserID && !f.rows[i].used && !f.rows[i].deleted {
			f.rows[i].deleted = true
			n++
		}
	}
	f.rows = append(f.rows, fakeResetRow{PortalResetToken: t})
	return n, nil
}
func (f *fakeResets) Use(_ context.Context, hash string, now time.Time) (uuid.UUID, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.rows {
		r := &f.rows[i]
		if r.TokenHash == hash && !r.used && !r.deleted && r.ExpiresAt.After(now) {
			r.used = true
			return r.UserID, true, nil
		}
	}
	return uuid.Nil, false, nil
}

// fakeMailer is a TEST FAKE of port.Mailer.
type fakeMailer struct {
	mu   sync.Mutex
	sent []port.PasswordResetMail
}

func (m *fakeMailer) SendPasswordReset(_ context.Context, mail port.PasswordResetMail) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, mail)
	return nil
}

// txUoW is a TEST FAKE unit of work whose rollback restores the reset tokens (the only fake with writes).
type txUoW struct{ r *fakeResets }

func (u txUoW) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	u.r.mu.Lock()
	snap := slices.Clone(u.r.rows)
	u.r.mu.Unlock()
	if err := fn(ctx); err != nil {
		u.r.mu.Lock()
		u.r.rows = snap
		u.r.mu.Unlock()
		return err
	}
	return nil
}
