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
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// fakeKC is a minimal Keycloak Admin API (Keycloak 26 paths) recording requests.
type fakeKC struct {
	mu       sync.Mutex
	orgs     []orgRep
	members  map[string]map[string]bool
	users    []userRep
	tokens   atomic.Int32
	failNext atomic.Int32 // respond 503 to the next n admin requests
	calls    []string
	bodies   []string
}

func newFake() *fakeKC { return &fakeKC{members: map[string]map[string]bool{}} }

func (f *fakeKC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/realms/eduworld/protocol/openid-connect/token" {
		_ = r.ParseForm()
		if r.Form.Get("client_id") != "svc-identity" || r.Form.Get("client_secret") != "s3cret" || r.Form.Get("grant_type") != "client_credentials" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f.tokens.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "expires_in": 300})
		return
	}
	if r.Header.Get("Authorization") != "Bearer tok" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	b, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, r.Method+" "+r.URL.Path)
	f.bodies = append(f.bodies, string(b))
	if f.failNext.Load() > 0 {
		f.failNext.Add(-1)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	p := strings.TrimPrefix(r.URL.Path, "/admin/realms/eduworld")
	switch {
	case r.Method == http.MethodGet && p == "/organizations":
		if r.URL.Query().Get("first") != "0" {
			_ = json.NewEncoder(w).Encode([]orgRep{})
			return
		}
		_ = json.NewEncoder(w).Encode(f.orgs)
	case r.Method == http.MethodPost && p == "/organizations":
		var o orgRep
		_ = json.Unmarshal(b, &o)
		for _, x := range f.orgs {
			if x.Alias == o.Alias {
				w.WriteHeader(http.StatusConflict)
				return
			}
		}
		o.ID = "org-" + o.Alias
		f.orgs = append(f.orgs, o)
		w.Header().Set("Location", "http://kc/admin/realms/eduworld/organizations/"+o.ID)
		w.WriteHeader(http.StatusCreated)
	case strings.HasPrefix(p, "/organizations/") && strings.HasSuffix(p, "/members") && r.Method == http.MethodPost:
		org := strings.Split(p, "/")[2]
		var uid string
		_ = json.Unmarshal(b, &uid)
		if f.members[org] == nil {
			f.members[org] = map[string]bool{}
		}
		if f.members[org][uid] {
			w.WriteHeader(http.StatusConflict)
			return
		}
		f.members[org][uid] = true
		w.WriteHeader(http.StatusCreated)
	case strings.HasPrefix(p, "/organizations/") && strings.Contains(p, "/members/") && r.Method == http.MethodDelete:
		parts := strings.Split(p, "/")
		if !f.members[parts[2]][parts[4]] {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		delete(f.members[parts[2]], parts[4])
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPost && p == "/users":
		var u userRep
		_ = json.Unmarshal(b, &u)
		for _, x := range f.users {
			if x.Username == u.Username {
				w.WriteHeader(http.StatusConflict)
				return
			}
		}
		u.ID = "kc-" + u.Username
		f.users = append(f.users, u)
		w.Header().Set("Location", "http://kc/admin/realms/eduworld/users/"+u.ID)
		w.WriteHeader(http.StatusCreated)
	case r.Method == http.MethodGet && p == "/users" && r.URL.Query().Get("q") != "":
		kv := strings.SplitN(r.URL.Query().Get("q"), ":", 2)
		out := []userRep{}
		for _, u := range f.users {
			for _, v := range u.Attributes[kv[0]] {
				if v == kv[1] {
					out = append(out, u)
				}
			}
		}
		_ = json.NewEncoder(w).Encode(out)
	case r.Method == http.MethodPut && strings.HasSuffix(p, "/reset-password"):
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodGet && strings.HasPrefix(p, "/roles/"):
		name := strings.TrimPrefix(p, "/roles/")
		if name == "missing" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(roleRep{ID: "r-" + name, Name: name})
	case strings.HasSuffix(p, "/role-mappings/realm"), strings.HasSuffix(p, "/logout"):
		w.WriteHeader(http.StatusNoContent)
	case p == "/bad":
		w.WriteHeader(http.StatusBadRequest)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func newAdmin(t *testing.T, f *fakeKC) *Admin {
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

func TestConfigFrom(t *testing.T) {
	t.Setenv("EW_KC_ADMIN_BASE_URL", "http://kc:8180/admin/realms/eduworld/")
	t.Setenv("EW_KC_ADMIN_CLIENT_ID", "svc-identity")
	t.Setenv("EW_KC_ADMIN_CLIENT_SECRET", "x")
	c, err := ConfigFrom(config.Load())
	if err != nil {
		t.Fatal(err)
	}
	if c.BaseURL != "http://kc:8180/admin/realms/eduworld" || c.Timeout != 2*time.Second || c.MaxRetries != 2 || c.FailureRatio != 0.5 {
		t.Fatalf("config = %+v", c)
	}
	a, err := New(c, nil)
	if err != nil || a.tokenURL != "http://kc:8180/realms/eduworld/protocol/openid-connect/token" {
		t.Fatalf("token url = %q, %v", a.tokenURL, err)
	}
	t.Setenv("EW_KC_ADMIN_CLIENT_SECRET", "")
	if _, err := ConfigFrom(config.Load()); err == nil {
		t.Fatal("missing secret must fail")
	}
	if _, err := New(Config{BaseURL: "http://kc/realms/x", ClientID: "a", ClientSecret: "b"}, nil); err == nil {
		t.Fatal("base url without /admin/realms/ must fail")
	}
}

// F010-AC-01 (adapter part): Organization ensured once by alias, idempotent and race-safe.
func TestEnsureOrganization(t *testing.T) {
	f := newFake()
	a := newAdmin(t, f)
	ctx := context.Background()
	org := port.KCOrganization{Alias: "th-demo-x", Name: "TH Demo X", Enabled: true,
		Attributes: map[string][]string{"tenant_id": {"0193"}, "tenant_type": {"school"}}}
	id, err := a.EnsureOrganization(ctx, org)
	if err != nil || id != "org-th-demo-x" {
		t.Fatalf("ensure = %q, %v", id, err)
	}
	id2, err := a.EnsureOrganization(ctx, org)
	if err != nil || id2 != id || len(f.orgs) != 1 {
		t.Fatalf("second ensure = %q, %v (orgs %d)", id2, err, len(f.orgs))
	}
	if f.orgs[0].Attributes["tenant_id"][0] != "0193" || len(f.orgs[0].Domains) != 1 {
		t.Fatalf("org = %+v", f.orgs[0])
	}
	if _, err := a.FindOrganizationByAlias(ctx, "nope"); !errors.Is(err, port.ErrKCNotFound) {
		t.Fatalf("find missing = %v", err)
	}
	if f.tokens.Load() != 1 {
		t.Fatalf("token fetched %d times, want cached once", f.tokens.Load())
	}
}

func TestMembersIdempotent(t *testing.T) {
	f := newFake()
	a := newAdmin(t, f)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := a.AddOrganizationMember(ctx, "org-a", "u1"); err != nil {
			t.Fatalf("add #%d: %v", i, err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := a.RemoveOrganizationMember(ctx, "org-a", "u1"); err != nil {
			t.Fatalf("remove #%d: %v", i, err)
		}
	}
}

// F010-AC-23 (adapter part): transient failures are retried for idempotent calls only; a 4xx is not.
func TestRetries(t *testing.T) {
	f := newFake()
	a := newAdmin(t, f)
	ctx := context.Background()
	f.failNext.Store(2)
	if err := a.LogoutUser(ctx, "u1"); err != nil {
		t.Fatalf("logout after 2× 503: %v", err)
	}
	if n := len(f.calls); n != 3 {
		t.Fatalf("calls = %d, want 3 (2 retries)", n)
	}
	f.failNext.Store(5)
	if err := a.LogoutUser(ctx, "u1"); err == nil || errors.Is(err, port.ErrKCNotFound) {
		t.Fatalf("exhausted retries must return a transient error, got %v", err)
	}
	f.failNext.Store(0)
	f.calls = nil
	f.failNext.Store(1)
	if _, err := a.CreateUser(ctx, port.KCNewUser{Username: "hs.once", Enabled: true}); err == nil {
		t.Fatal("create user is not retried")
	}
	if len(f.calls) != 1 {
		t.Fatalf("create user calls = %d, want 1", len(f.calls))
	}
	f.calls = nil
	if _, err := a.call(ctx, "bad", http.MethodGet, "/bad", nil, nil, true); !errors.Is(err, errClient) || len(f.calls) != 1 {
		t.Fatalf("400: %v after %d calls", err, len(f.calls))
	}
}

// F010-AC-28/29 (adapter part): username conflict and password never in errors.
func TestUserAndPassword(t *testing.T) {
	f := newFake()
	a := newAdmin(t, f)
	ctx := context.Background()
	id, err := a.CreateUser(ctx, port.KCNewUser{Username: "hs.tudo", Enabled: true, Attributes: map[string][]string{"ew_user_id": {"u"}}})
	if err != nil || id != "kc-hs.tudo" {
		t.Fatalf("create = %q, %v", id, err)
	}
	if _, err := a.CreateUser(ctx, port.KCNewUser{Username: "hs.tudo"}); !errors.Is(err, port.ErrKCConflict) {
		t.Fatalf("duplicate = %v", err)
	}
	// F011-AC-09 (adapter part): the user is found again by eduworld_user_id.
	if u, err := a.FindUserByAttribute(ctx, "ew_user_id", "u"); err != nil || u.ID != id {
		t.Fatalf("find by attribute = %+v %v", u, err)
	}
	if _, err := a.FindUserByAttribute(ctx, "ew_user_id", "other"); !errors.Is(err, port.ErrKCNotFound) {
		t.Fatalf("find unknown = %v", err)
	}
	if err := a.SetPassword(ctx, id, "Pw-unique-123", true); err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	_ = json.Unmarshal([]byte(f.bodies[len(f.bodies)-1]), &body)
	if body["temporary"] != true || body["type"] != "password" {
		t.Fatalf("reset-password body = %v", body)
	}
	f.failNext.Store(10)
	err = a.SetPassword(ctx, id, "Pw-unique-456", false)
	if err == nil || strings.Contains(err.Error(), "Pw-unique") {
		t.Fatalf("error leaks the password or is nil: %v", err)
	}
}

func TestRealmRolesAndTokenRefresh(t *testing.T) {
	f := newFake()
	a := newAdmin(t, f)
	ctx := context.Background()
	if err := a.AddRealmRoles(ctx, "u1", "tenant_staff"); err != nil {
		t.Fatal(err)
	}
	if err := a.RemoveRealmRoles(ctx, "u1", "missing"); !errors.Is(err, port.ErrKCNotFound) {
		t.Fatalf("unknown role = %v", err)
	}
	a.mu.Lock()
	a.token = "stale"
	a.expiry = time.Now().Add(time.Hour)
	a.mu.Unlock()
	if err := a.LogoutUser(ctx, "u1"); err != nil {
		t.Fatalf("401 must refresh the token and retry: %v", err)
	}
	if f.tokens.Load() < 2 {
		t.Fatalf("token not refreshed (%d fetches)", f.tokens.Load())
	}
}

// T040: every Admin API call is counted by operation and result.
func TestOrgSyncMetric(t *testing.T) {
	f := newFake()
	a := newAdmin(t, f)
	before := testutil.ToFloat64(orgSync.WithLabelValues("logout_user", "ok"))
	if err := a.LogoutUser(context.Background(), "u1"); err != nil {
		t.Fatal(err)
	}
	if testutil.ToFloat64(orgSync.WithLabelValues("logout_user", "ok")) != before+1 {
		t.Fatal("ok call not counted")
	}
	nf := testutil.ToFloat64(orgSync.WithLabelValues("get_role", "not_found"))
	_ = a.AddRealmRoles(context.Background(), "u1", "missing")
	if testutil.ToFloat64(orgSync.WithLabelValues("get_role", "not_found")) != nf+1 {
		t.Fatal("not_found call not counted")
	}
}
