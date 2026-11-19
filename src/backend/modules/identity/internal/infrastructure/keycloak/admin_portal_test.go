package keycloak

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
)

// fakePortalAdmin is a TEST FAKE of the Keycloak Admin API calls of the portal setup/reset (Keycloak 26).
type fakePortalAdmin struct {
	mu        sync.Mutex
	actions   []string
	passwords []map[string]any
	emails    [][]string
	failEmail bool
	policy    bool
	calls     []string
}

func (f *fakePortalAdmin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/protocol/openid-connect/token") {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "expires_in": 300})
		return
	}
	b, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	p := strings.TrimPrefix(r.URL.Path, "/admin/realms/eduworld")
	f.calls = append(f.calls, r.Method+" "+p)
	switch {
	case r.Method == http.MethodGet && p == "/users" && r.URL.Query().Get("email") != "":
		_ = json.NewEncoder(w).Encode([]loginUserRep{{ID: "kc-a", Username: "ad.truong.a", Email: "Ad.Truong.A@Example.test", Enabled: true}})
	case r.Method == http.MethodGet && p == "/users":
		_ = json.NewEncoder(w).Encode([]userRep{{ID: "kc-a", Username: r.URL.Query().Get("username"), Enabled: true}})
	case r.Method == http.MethodGet && p == "/users/kc-a":
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "kc-a", "username": "ad.truong.a", "requiredActions": f.actions})
	case r.Method == http.MethodPut && p == "/users/kc-a":
		var u struct {
			RequiredActions []string `json:"requiredActions"`
		}
		_ = json.Unmarshal(b, &u)
		f.actions = u.RequiredActions
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPut && p == "/users/kc-a/reset-password":
		if f.policy {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":"invalidPasswordMinLengthMessage","error_description":"Invalid password: minimum length 8."}`)
			return
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		f.passwords = append(f.passwords, m)
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPut && p == "/users/kc-a/execute-actions-email":
		if f.failEmail {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		var a []string
		_ = json.Unmarshal(b, &a)
		f.emails = append(f.emails, a)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func newPortalAdmin(t *testing.T, f *fakePortalAdmin) *Admin {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	a, err := New(Config{BaseURL: srv.URL + "/admin/realms/eduworld", ClientID: "svc-identity", ClientSecret: "s3cret",
		Timeout: time.Second, MaxRetries: 2, FailureRatio: 0.9}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// F012-AC-33: the Keycloak user behind a username or an email (case-insensitive) and its required actions.
func TestFindUserByLoginAndRequiredActions(t *testing.T) {
	f := &fakePortalAdmin{actions: []string{"UPDATE_PASSWORD", "CONFIGURE_TOTP"}}
	a := newPortalAdmin(t, f)
	ctx := context.Background()
	u, err := a.FindUserByLogin(ctx, "ad.truong.a@example.test")
	if err != nil || u.ID != "kc-a" {
		t.Fatalf("by email = %+v %v", u, err)
	}
	if u, err = a.FindUserByLogin(ctx, "ad.truong.a"); err != nil || u.ID != "kc-a" {
		t.Fatalf("by username = %+v %v", u, err)
	}
	acts, err := a.RequiredActions(ctx, "kc-a")
	if err != nil || len(acts) != 2 {
		t.Fatalf("actions = %v %v", acts, err)
	}
}

// F012-AC-34, AC-37: permanent password with JSON escaping; the realm policy is a distinct error.
func TestResetPasswordAndPolicy(t *testing.T) {
	f := &fakePortalAdmin{actions: []string{"UPDATE_PASSWORD", "CONFIGURE_TOTP"}}
	a := newPortalAdmin(t, f)
	ctx := context.Background()
	pw := []byte("Pa\"ss\\w\x01ord ✓")
	if err := a.ResetPassword(ctx, "kc-a", pw); err != nil {
		t.Fatal(err)
	}
	if got := f.passwords[0]; got["value"] != string(pw) || got["temporary"] != false || got["type"] != "password" {
		t.Fatalf("payload = %v", got)
	}
	f.policy = true
	if err := a.ResetPassword(ctx, "kc-a", []byte("x")); !errors.Is(err, port.ErrKCPasswordPolicy) {
		t.Fatalf("policy = %v", err)
	}
	if err := a.RemoveRequiredAction(ctx, "kc-a", port.KCActionUpdatePassword); err != nil {
		t.Fatal(err)
	}
	if len(f.actions) != 1 || f.actions[0] != "CONFIGURE_TOTP" {
		t.Fatalf("actions = %v", f.actions)
	}
	n := len(f.calls)
	if err := a.RemoveRequiredAction(ctx, "kc-a", port.KCActionUpdatePassword); err != nil || len(f.calls) != n+1 {
		t.Fatalf("absent action must not PUT: %v %v", err, f.calls[n:])
	}
}

// F012-AC-35: execute-actions-email CONFIGURE_TOTP; it sends an email so it is never retried.
func TestExecuteActionsEmail(t *testing.T) {
	f := &fakePortalAdmin{}
	a := newPortalAdmin(t, f)
	ctx := context.Background()
	if err := a.ExecuteActionsEmail(ctx, "kc-a", port.KCActionConfigureTOTP); err != nil || len(f.emails) != 1 || f.emails[0][0] != "CONFIGURE_TOTP" {
		t.Fatalf("email = %v %v", f.emails, err)
	}
	f.failEmail = true
	n := len(f.calls)
	if err := a.ExecuteActionsEmail(ctx, "kc-a", port.KCActionConfigureTOTP); err == nil || len(f.calls) != n+1 {
		t.Fatalf("no retry: %v calls=%v", err, f.calls[n:])
	}
}
