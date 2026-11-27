package cases

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/test/security/authz-matrix/internal/spec"
)

const (
	backendRoot = "../../../../.."
	repoRoot    = "../../../../../../.."
)

func fixtureOps(t *testing.T) []spec.Operation {
	t.Helper()
	ops, err := spec.Load(context.Background(), "../spec/testdata/mini.yaml", "../spec/testdata/mini_matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	return ops
}

func find(cs []Case, op string, r Role, rel Relation) (Case, bool) {
	for _, c := range cs {
		if c.OpID == op && c.Role == r && c.Relation == rel {
			return c, true
		}
	}
	return Case{}, false
}

// F132-AC-15 (TC-TEN-001, T022): cases from the fixture contract.
func TestGenerateExpectations(t *testing.T) {
	cs := Generate(fixtureOps(t), Overrides{})
	for _, c := range cs {
		if c.OpID == "gameLogin" || c.OpID == "getClientConfig" {
			t.Fatalf("public operation got a case: %+v", c)
		}
	}
	for _, tc := range []struct {
		op       string
		role     Role
		rel      Relation
		statuses []int
		codes    []string
		observe  bool
		foreign  bool
	}{
		{op: "getPlayerProfile", role: Anonymous, rel: None, statuses: []int{401}},
		// tenant-scoped route of another tenant → 404/403 for every tenant role
		{op: "getPlayerProfile", role: Student, rel: OtherTenant, statuses: []int{403, 404}},
		{op: "getLeaderboard", role: SchoolAdmin, rel: OtherTenant, statuses: []int{403, 404}},
		{op: "listCmsThings", role: TenantAdmin, rel: OtherTenant, statuses: []int{403, 404}, foreign: true},
		// player route with staff → 403 GAME_ACCESS_STUDENT_ONLY
		{op: "getPlayerProfile", role: SchoolAdmin, rel: SameSchool, statuses: []int{403}, codes: []string{"GAME_ACCESS_STUDENT_ONLY"}},
		{op: "getPlayerProfile", role: AcademicManager, rel: SameSchool, statuses: []int{403}, codes: []string{"GAME_ACCESS_STUDENT_ONLY"}},
		{op: "getPlayerProfile", role: SystemAdmin, rel: SameSchool, statuses: []int{403}, codes: []string{"GAME_ACCESS_STUDENT_ONLY", "TENANT_MISMATCH"}},
		{op: "getPlayerProfile", role: Student, rel: SameSchool, observe: true},
		// portal route with a student, platform route with a tenant role
		{op: "listCmsThings", role: Student, rel: SameSchool, statuses: []int{403}, codes: []string{"PORTAL_ACCESS_STAFF_ONLY"}},
		{op: "getGameAdminThing", role: TenantAdmin, rel: SameSchool, statuses: []int{403}, codes: []string{"PERMISSION_DENIED", "TENANT_MISMATCH"}},
		{op: "getGameAdminThing", role: Student, rel: SameSchool, statuses: []int{403}, codes: []string{"PORTAL_ACCESS_STAFF_ONLY"}},
		{op: "getGameAdminThing", role: SystemAdmin, rel: SameSchool, observe: true},
		// platform roles are cross-tenant by design (R20): observed only
		{op: "getLeaderboard", role: SystemAdmin, rel: OtherTenant, observe: true},
		{op: "getLeaderboard", role: SafetyModerator, rel: OtherTenant, observe: true},
		{op: "getLeaderboard", role: SchoolAdmin, rel: OtherSchoolSameTenant, observe: true},
	} {
		c, ok := find(cs, tc.op, tc.role, tc.rel)
		if !ok {
			t.Fatalf("no case %s %s %s", tc.op, tc.role, tc.rel)
		}
		if c.Expect.Observe != tc.observe || !slices.Equal(c.Expect.Statuses, tc.statuses) || !slices.Equal(c.Expect.Codes, tc.codes) || c.ForeignTenantHeader != tc.foreign {
			t.Fatalf("%s %s %s: got %+v foreign=%v", tc.op, tc.role, tc.rel, c.Expect, c.ForeignTenantHeader)
		}
	}
	// no cross-tenant case for an operation neither tenant-scoped nor taking an id
	if _, ok := find(cs, "listMyTenants", Student, OtherTenant); ok {
		t.Fatal("listMyTenants has a cross-tenant case")
	}
	// no other-school case without an id parameter
	if _, ok := find(cs, "listCmsThings", SchoolAdmin, OtherSchoolSameTenant); ok {
		t.Fatal("listCmsThings has an other-school case")
	}
}

func TestOverrides(t *testing.T) {
	ops := fixtureOps(t)
	ov := Overrides{Class: map[string]Class{"getLeaderboard": ClassPlayer}, Skip: map[string]string{"listMyTenants": "fixture"}}
	cs := Generate(ops, ov)
	if c, _ := find(cs, "getLeaderboard", TenantAdmin, SameSchool); c.Expect.Rule != "player_route_staff_403" {
		t.Fatalf("override ignored: %+v", c.Expect)
	}
	if _, ok := find(cs, "listMyTenants", Student, SameSchool); ok {
		t.Fatal("skipped operation got cases")
	}
}

func TestSatisfied(t *testing.T) {
	e := Expect{Statuses: []int{403}, Codes: []string{"GAME_ACCESS_STUDENT_ONLY"}}
	for _, tc := range []struct {
		status int
		code   string
		want   bool
	}{{403, "GAME_ACCESS_STUDENT_ONLY", true}, {403, "PERMISSION_DENIED", false}, {200, "", false}, {500, "INTERNAL", false}} {
		if got := e.Satisfied(tc.status, tc.code); got != tc.want {
			t.Fatalf("Satisfied(%d,%s) = %v", tc.status, tc.code, got)
		}
	}
	if (Expect{Observe: true}).Satisfied(503, "UNAVAILABLE") {
		t.Fatal("a 5xx satisfies an observation")
	}
	if !(Expect{Observe: true}).Satisfied(200, "") {
		t.Fatal("observation must accept 200")
	}
}

// The real contract: every non-public operation has a 401 case, every player route a staff case.
func TestGenerateRealContract(t *testing.T) {
	ops, err := spec.Load(context.Background(), filepath.Join(repoRoot, "specs/openapi/eduworld-api.v1.yaml"),
		filepath.Join(backendRoot, "test/authz/matrix_gen.json"))
	if err != nil {
		t.Fatal(err)
	}
	cs := Generate(ops, Overrides{})
	asserted, anon, player, cross := 0, 0, 0, 0
	for _, c := range cs {
		if !c.Expect.Observe {
			asserted++
		}
		switch c.Expect.Rule {
		case "no_token_401":
			anon++
		case "player_route_staff_403":
			player++
		case "cross_tenant_404_or_403":
			cross++
		}
	}
	nonPublic := 0
	for _, op := range ops {
		if !op.Public {
			nonPublic++
		}
	}
	if anon != nonPublic || player == 0 || cross == 0 {
		t.Fatalf("anon=%d (want %d) player=%d cross=%d", anon, nonPublic, player, cross)
	}
	t.Logf("%d operations → %d cases (%d asserted: %d no-token, %d player-staff, %d cross-tenant)", len(ops), len(cs), asserted, anon, player, cross)
}
