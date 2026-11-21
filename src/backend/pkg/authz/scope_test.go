package authz_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// F002-AC-11 (TC-TEN-004, TC-TEN-018).
func TestScopeFor(t *testing.T) {
	ta := uuid.New()
	p := authz.NewPrincipal(authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, TenantID: ptr(ta)})
	s, err := authz.ScopeFor(p)
	if err != nil {
		t.Fatal(err)
	}
	if ids := s.TenantIDs(); len(ids) != 1 || ids[0] != ta {
		t.Fatalf("ids = %v", ids)
	}
	platform := authz.NewPrincipal(authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff})
	if _, err := authz.ScopeFor(platform); err == nil {
		t.Fatal("ScopeFor without tenant")
	}
}

func TestCrossTenantScopeRequirements(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	full := authz.PrincipalData{
		UserID: uuid.New(), UserType: authz.UserTypeStaff, IsSystemAdmin: true, Channel: authz.ChannelPortal,
		Grants: []authz.Grant{{Role: "system_admin", Permissions: []authz.Permission{authz.PermPlatformCrossTenantRead}}},
	}
	if s, err := authz.CrossTenantScope(authz.NewPrincipal(full), ids, tenancy.ReasonSystemAdminCrossTenant); err != nil || len(s.TenantIDs()) != 2 {
		t.Fatalf("valid cross-tenant scope refused: %v", err)
	}
	mutations := map[string]func(d *authz.PrincipalData){
		"no permission": func(d *authz.PrincipalData) { d.Grants = nil },
		"game channel":  func(d *authz.PrincipalData) { d.Channel = authz.ChannelGame },
		"no client":     func(d *authz.PrincipalData) { d.Channel = authz.ChannelUnknown },
		"tenant user":   func(d *authz.PrincipalData) { d.TenantID = ptr(uuid.New()) },
	}
	for name, m := range mutations {
		d := full
		m(&d)
		if _, err := authz.CrossTenantScope(authz.NewPrincipal(d), ids, tenancy.ReasonSystemAdminCrossTenant); err == nil {
			t.Fatalf("%s: cross-tenant scope granted", name)
		}
	}
	p := authz.NewPrincipal(full)
	if _, err := authz.CrossTenantScope(p, ids, ""); err == nil {
		t.Fatal("empty reason accepted")
	}
	if _, err := authz.CrossTenantScope(p, ids, "debug"); err == nil {
		t.Fatal("unknown reason accepted")
	}
	if _, err := authz.CrossTenantScope(p, nil, tenancy.ReasonSystemAdminCrossTenant); err == nil {
		t.Fatal("empty tenant list accepted")
	}
}

func TestPlatformScopes(t *testing.T) {
	staff := authz.NewPrincipal(authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff,
		Grants: []authz.Grant{{Role: "content_admin", Permissions: []authz.Permission{authz.PermCurriculumWrite}}}})
	s, err := authz.PlatformScope(staff)
	if err != nil || !s.PlatformOnly() || len(s.TenantIDs()) != 0 || s.Actor() != "user:"+staff.UserID.String() {
		t.Fatalf("platform scope: %+v %v", s, err)
	}
	tenantUser := authz.NewPrincipal(authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, TenantID: ptr(uuid.New())})
	if _, err := authz.PlatformScope(tenantUser); err == nil {
		t.Fatal("tenant user got platform scope")
	}
	student := authz.NewPrincipal(authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStudent})
	if _, err := authz.PlatformScope(student); err == nil {
		t.Fatal("student got platform scope")
	}
	js, err := authz.SystemPlatformScope("leaderboard-rollup")
	if err != nil || !js.PlatformOnly() || js.Actor() != "job:leaderboard-rollup" {
		t.Fatalf("job scope: %+v %v", js, err)
	}
	if _, err := authz.SystemPlatformScope("Bad Name"); err == nil {
		t.Fatal("invalid job name accepted")
	}
}

func TestEventScope(t *testing.T) {
	id := uuid.New()
	s, err := authz.EventScope(id)
	if err != nil || s.TenantIDs()[0] != id {
		t.Fatalf("event scope: %v %v", s, err)
	}
	if _, err := authz.EventScope(uuid.Nil); err == nil {
		t.Fatal("nil tenant accepted")
	}
}
