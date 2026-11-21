package query_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

type fakePlatformUsers struct {
	calls int
	got   read.PlatformUserFilter
	rows  []read.UserView
}

func (m *fakePlatformUsers) PlatformUsers(_ context.Context, f read.PlatformUserFilter) ([]read.UserView, error) {
	m.calls++
	m.got = f
	return m.rows, nil
}

type auditSpy struct {
	mu     sync.Mutex
	events []audit.Event
}

func (a *auditSpy) Record(_ context.Context, e audit.Event) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, e)
	return nil
}

// F059-1 (listPlatformUsers): platform.user.manage, never a tenant session; without tenant_ids the platform staff
// only; tenant_ids needs the CrossTenantScope (audited cross_tenant.accessed), never a fallback listing.
func TestListPlatformUsers(t *testing.T) {
	spy := &auditSpy{}
	tenancy.Configure(tenancy.Hooks{Recorder: spy})
	t.Cleanup(func() { tenancy.Configure(tenancy.Hooks{}) })
	m := &fakePlatformUsers{}
	h := query.PlatformUsers{Model: m, Cursors: codec(t)}
	bg := context.Background()

	for _, p := range []authz.Principal{authztest.Staff(authztest.TenantA, "tenant_admin"), authztest.Platform("content_admin")} {
		_, err := h.List(authztest.Ctx(bg, p), query.PlatformUsersInput{})
		authztest.ExpectDenied(t, err)
	}
	sa := authztest.Ctx(bg, authztest.Platform("system_admin"))
	for name, in := range map[string]query.PlatformUsersInput{
		"short q":   {Q: "a"},
		"long q":    {Q: strings.Repeat("x", 101)},
		"user_type": {UserType: "admin"},
		"status":    {Status: "deleted"},
		"role_code": {RoleCode: strings.Repeat("r", 65)},
	} {
		if _, err := h.List(sa, in); codeOf(err) != problem.ValidationFailed {
			t.Errorf("%s: %v", name, err)
		}
	}
	// tenant_ids without a CrossTenantScope (no platform.cross_tenant.read): 403, nothing read.
	if _, err := h.List(sa, query.PlatformUsersInput{TenantFilter: true}); codeOf(err) != problem.PermissionDenied {
		t.Fatalf("tenant filter without scope: %v", err)
	}
	if m.calls != 0 {
		t.Fatal("read model reached for a rejected request")
	}

	out, err := h.List(sa, query.PlatformUsersInput{Q: "ops", Status: "active"})
	if err != nil || out.Items == nil || m.got.TenantIDs != nil || m.got.Q != "ops" || m.got.Status != "active" || m.got.Limit != 20 {
		t.Fatalf("platform list: %+v %+v %v", out, m.got, err)
	}

	sc, err := authz.CrossTenantScope(authztest.Platform("system_admin"), []uuid.UUID{authztest.TenantA, authztest.TenantB},
		tenancy.ReasonSystemAdminCrossTenant)
	if err != nil {
		t.Fatal(err)
	}
	cross := tenancy.WithScope(sa, sc)
	m.rows = []read.UserView{{UserID: uuid.New(), UserType: "student", TenantIDs: []uuid.UUID{authztest.TenantA}, UserRoles: []read.UserRoleView{}}}
	out, err = h.List(cross, query.PlatformUsersInput{TenantFilter: true, UserType: "student"})
	if err != nil || len(out.Items) != 1 || len(m.got.TenantIDs) != 2 {
		t.Fatalf("cross-tenant list: %+v %+v %v", out, m.got, err)
	}
	found := false
	for _, e := range spy.events {
		found = found || e.Action == audit.ActionCrossTenantAccessed
	}
	if !found {
		t.Fatalf("cross_tenant.accessed not recorded: %+v", spy.events)
	}
	raw, _ := json.Marshal(out)
	for _, k := range []string{"password", "token", "secret", `"ip`} {
		if strings.Contains(strings.ToLower(string(raw)), k) {
			t.Fatalf("response carries %q: %s", k, raw)
		}
	}
}

type fakeSisStats struct{ counts map[string]int }

func (f fakeSisStats) ConnectionStatusCounts(context.Context) (map[string]int, error) {
	return f.counts, nil
}

// F059-2 (getIntegrationOverview): any-of platform.sis_connection.manage / platform.tenant.manage, platform staff
// only; configuration and counts only, gateway URLs null when not configured.
func TestIntegrationOverview(t *testing.T) {
	now := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	h := query.Integrations{Config: query.IntegrationConfig{IssuerURL: "https://sso.example/realms/eduworld", Realm: "eduworld",
		Clients:            []query.IntegrationClient{{ClientID: "eduworld-portal", Purpose: "portal"}},
		PlatformMaxAuthAge: 8 * time.Hour, InternalBaseURL: "https://admin-api.example"},
		Stats: fakeSisStats{counts: map[string]int{"active": 3, "paused": 1}}, Clock: func() time.Time { return now }}
	bg := context.Background()
	for _, p := range []authz.Principal{authztest.Staff(authztest.TenantA, "tenant_admin"), authztest.Platform("content_admin")} {
		_, err := h.Overview(authztest.Ctx(bg, p))
		authztest.ExpectDenied(t, err)
	}
	out, err := h.Overview(authztest.Ctx(bg, authztest.Platform("system_admin")))
	if err != nil {
		t.Fatal(err)
	}
	if out.Sis.ConnectionsTotal != 4 || out.Sis.ConnectionsByStatus["active"] != 3 ||
		out.Keycloak.PlatformAuthMaxAgeSeconds != 28800 || out.Apisix.PublicBaseURL != nil || out.Apisix.InternalBaseURL == nil ||
		!out.GeneratedAt.Equal(now) {
		t.Fatalf("overview = %+v", out)
	}
	raw, _ := json.Marshal(out)
	for _, k := range []string{"secret", "password", "token", "credential"} {
		if strings.Contains(strings.ToLower(string(raw)), k) {
			t.Fatalf("overview carries %q: %s", k, raw)
		}
	}
	if !strings.Contains(string(raw), `"public_base_url":null`) {
		t.Fatalf("unset gateway URL must be null: %s", raw)
	}
}
