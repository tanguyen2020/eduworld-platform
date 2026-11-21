package keycloak

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// fakeTokenEndpoint is a TEST FAKE of the Keycloak token/logout endpoints of realm eduworld, answering
// like Keycloak 26 does for the direct grant flow eduworld-portal-direct-grant (F004 research R5).
type fakeTokenEndpoint struct {
	mu     sync.Mutex
	calls  []url.Values
	paths  []string
	status int    // forced status (0 = normal behaviour)
	ctype  string // forced content type
	body   string // forced body
}

func jwtWith(claims map[string]any) string {
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	b, _ := json.Marshal(claims)
	return h + "." + base64.RawURLEncoding.EncodeToString(b) + ".c2ln"
}

func (f *fakeTokenEndpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	form, _ := url.ParseQuery(string(raw))
	f.mu.Lock()
	f.calls = append(f.calls, form)
	f.paths = append(f.paths, r.URL.Path)
	status, ctype, body := f.status, f.ctype, f.body
	f.mu.Unlock()
	if status != 0 {
		if ctype == "" {
			ctype = "application/json"
		}
		w.Header().Set("Content-Type", ctype)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
		return
	}
	if form.Get("client_id") != "eduworld-portal" || form.Get("client_secret") != "portal-s3cret" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"invalid_client","error_description":"Invalid client or Invalid client credentials"}`)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/logout") {
		if form.Get("refresh_token") == "rt-dead" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":"invalid_grant","error_description":"Session not active"}`)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	roles := []string{"default-roles-eduworld", "tenant_staff"}
	if form.Get("username") == "sa.admin" {
		roles = []string{"default-roles-eduworld", "platform_staff"}
	}
	if form.Get("grant_type") == "refresh_token" && form.Get("refresh_token") != "rt-1" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"invalid_grant","error_description":"Token is not active"}`)
		return
	}
	at := jwtWith(map[string]any{"iss": "http://kc/realms/eduworld", "sub": "kc-user-1", "sid": "sid-9", "roles": roles, "amr": []string{"pwd"},
		"iat": 1_800_000_000, "azp": "eduworld-portal"})
	rt := jwtWith(map[string]any{"sid": "sid-9", "typ": "Refresh"})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"access_token": at, "refresh_token": rt, "expires_in": 300,
		"refresh_expires_in": 1800, "token_type": "Bearer", "session_state": "sid-9"})
}

func newPortalTokens(t *testing.T, f *fakeTokenEndpoint) *PortalTokens {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	p, err := NewPortalTokens(PortalConfig{TokenURL: srv.URL + "/realms/eduworld/protocol/openid-connect/token",
		LogoutURL: srv.URL + "/realms/eduworld/protocol/openid-connect/logout", ClientID: "eduworld-portal",
		ClientSecret: "portal-s3cret", Timeout: time.Second, MaxRetries: 2, FailureRatio: 0.9}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPortalConfigFrom(t *testing.T) {
	t.Setenv("EW_KC_ADMIN_BASE_URL", "http://kc:8180/admin/realms/eduworld/")
	t.Setenv("EW_KC_PORTAL_CLIENT_ID", "eduworld-portal")
	t.Setenv("EW_KC_PORTAL_CLIENT_SECRET", "x")
	c, err := PortalConfigFrom(config.Load())
	if err != nil {
		t.Fatal(err)
	}
	if c.TokenURL != "http://kc:8180/realms/eduworld/protocol/openid-connect/token" ||
		c.LogoutURL != "http://kc:8180/realms/eduworld/protocol/openid-connect/logout" || c.ClientID != "eduworld-portal" {
		t.Fatalf("config = %+v", c)
	}
	t.Setenv("EW_KC_PORTAL_CLIENT_SECRET", "")
	_, err = PortalConfigFrom(config.Load())
	if err == nil || !strings.Contains(err.Error(), "EW_KC_PORTAL_CLIENT_SECRET") || strings.Contains(err.Error(), "eduworld-portal") {
		t.Fatalf("missing secret: %v", err)
	}
}

// F012-AC-27: direct grant of the confidential client with username and password only (ADR-0094), claims read
// from the token (realm roles for the platform session bound).
func TestPortalPasswordGrant(t *testing.T) {
	f := &fakeTokenEndpoint{}
	p := newPortalTokens(t, f)
	pw := []byte(`p@ss word&="x`)
	tok, err := p.PasswordGrant(context.Background(), "ad.truong.a", pw)
	if err != nil {
		t.Fatal(err)
	}
	if tok.Subject != "kc-user-1" || tok.SessionID != "sid-9" || tok.Issuer != "http://kc/realms/eduworld" || tok.ExpiresIn != 300 ||
		tok.RefreshExpiresIn != 1800 || tok.RefreshToken == "" || !slices.Contains(tok.RealmRoles, "tenant_staff") || tok.IssuedAt.Unix() != 1_800_000_000 {
		t.Fatalf("tokens = %+v", tok)
	}
	c := f.calls[0]
	if c.Get("grant_type") != "password" || c.Get("username") != "ad.truong.a" || c.Get("password") != string(pw) || c.Has("otp") {
		t.Fatalf("form = %v", c)
	}
	tok, err = p.PasswordGrant(context.Background(), "sa.admin", []byte("x"))
	if err != nil || f.calls[1].Has("otp") || !slices.Contains(tok.RealmRoles, "platform_staff") {
		t.Fatalf("platform grant = %+v %v (form %v)", tok, err, f.calls[1])
	}
}

// F012-AC-28, AC-29, AC-33: Keycloak answers are mapped exactly (F004 research R5).
func TestPortalGrantErrors(t *testing.T) {
	cases := []struct {
		name          string
		status        int
		ctype, body   string
		want          error
		misconfigured bool
	}{
		{"wrong password", 401, "", `{"error":"invalid_grant","error_description":"Invalid user credentials"}`, port.ErrPortalInvalidCredentials, false},
		{"disabled", 400, "", `{"error":"invalid_grant","error_description":"Account disabled"}`, port.ErrPortalInvalidCredentials, false},
		{"locked", 400, "", `{"error":"invalid_grant","error_description":"Account temporarily disabled"}`, port.ErrPortalInvalidCredentials, false},
		{"setup", 400, "", `{"error":"invalid_grant","error_description":"Account is not fully set up"}`, port.ErrPortalSetupRequired, false},
		{"student", 401, "text/html;charset=utf-8", `<html>portalStudentDenied</html>`, port.ErrPortalAccessDenied, false},
		{"bad client", 401, "", `{"error":"invalid_client"}`, nil, true},
		{"unauthorized client", 400, "", `{"error":"unauthorized_client"}`, nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeTokenEndpoint{status: c.status, ctype: c.ctype, body: c.body}
			p := newPortalTokens(t, f)
			_, err := p.PasswordGrant(context.Background(), "u", []byte("p"))
			if c.want != nil && !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if c.misconfigured && (!errors.Is(err, errMisconfigured) || errors.Is(err, port.ErrPortalInvalidCredentials)) {
				t.Fatalf("misconfiguration must not look like a credential error: %v", err)
			}
		})
	}
}

// Password grants are never retried and 5xx is transient; credential errors never open the breaker.
func TestPortalGrantNoRetry(t *testing.T) {
	f := &fakeTokenEndpoint{status: 503}
	p := newPortalTokens(t, f)
	_, err := p.PasswordGrant(context.Background(), "u", []byte("p"))
	if err == nil || errors.Is(err, port.ErrPortalInvalidCredentials) || len(f.calls) != 1 {
		t.Fatalf("err = %v calls = %d", err, len(f.calls))
	}
	f2 := &fakeTokenEndpoint{status: 401, body: `{"error":"invalid_grant"}`}
	p2 := newPortalTokens(t, f2)
	for range 30 {
		if _, err := p2.PasswordGrant(context.Background(), "u", []byte("p")); !errors.Is(err, port.ErrPortalInvalidCredentials) {
			t.Fatalf("credential errors must not open the circuit: %v", err)
		}
	}
}

// F012-AC-30: refresh rotates; an inactive token is REFRESH_TOKEN_INVALID; logout is idempotent.
func TestPortalRefreshAndLogout(t *testing.T) {
	f := &fakeTokenEndpoint{}
	p := newPortalTokens(t, f)
	ctx := context.Background()
	tok, err := p.Refresh(ctx, "rt-1")
	if err != nil || tok.RefreshToken == "" || f.calls[0].Get("grant_type") != "refresh_token" {
		t.Fatalf("refresh = %+v %v", tok, err)
	}
	if _, err := p.Refresh(ctx, "rt-old"); !errors.Is(err, port.ErrPortalRefreshInvalid) {
		t.Fatalf("stale refresh = %v", err)
	}
	if _, err := p.Refresh(ctx, ""); !errors.Is(err, port.ErrPortalRefreshInvalid) {
		t.Fatalf("empty refresh = %v", err)
	}
	if err := p.Logout(ctx, "rt-1"); err != nil {
		t.Fatal(err)
	}
	if err := p.Logout(ctx, "rt-dead"); err != nil {
		t.Fatalf("ended session = %v", err)
	}
	last := f.calls[len(f.calls)-1]
	if !strings.HasSuffix(f.paths[len(f.paths)-1], "/logout") || last.Get("refresh_token") != "rt-dead" || last.Get("client_secret") != "portal-s3cret" {
		t.Fatalf("logout form = %v", last)
	}
	// Logout is idempotent: retried on 5xx.
	f5 := &fakeTokenEndpoint{status: 503}
	p5 := newPortalTokens(t, f5)
	if err := p5.Logout(ctx, "rt-1"); err == nil || len(f5.calls) != 3 {
		t.Fatalf("logout retries: %v calls=%d", err, len(f5.calls))
	}
}

func TestSessionKey(t *testing.T) {
	p := newPortalTokens(t, &fakeTokenEndpoint{})
	a := p.SessionKey(jwtWith(map[string]any{"sid": "s1", "jti": "a"}))
	b := p.SessionKey(jwtWith(map[string]any{"sid": "s1", "jti": "b"}))
	c := p.SessionKey(jwtWith(map[string]any{"sid": "s2"}))
	if a != b || a == c || strings.Contains(a, "s1") || p.SessionKey("opaque") == a {
		t.Fatalf("keys %s %s %s", a, b, c)
	}
}
