package devseed

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const seedDir = "../../../seed" // modules/identity/seed

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "deploy", "keycloak", "realm-eduworld.dev.json")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root not found")
		}
		dir = parent
	}
}

type realm struct {
	Users []struct {
		Username   string   `json:"username"`
		Email      string   `json:"email"`
		RealmRoles []string `json:"realmRoles"`
	} `json:"users"`
	Organizations []struct {
		Alias   string `json:"alias"`
		Members []struct {
			Username string `json:"username"`
		} `json:"members"`
	} `json:"organizations"`
}

func loadRealm(t *testing.T) (roles map[string][]string, emails map[string]string, members map[string][]string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "deploy", "keycloak", "realm-eduworld.dev.json"))
	if err != nil {
		t.Fatal(err)
	}
	var r realm
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	roles, emails, members = map[string][]string{}, map[string]string{}, map[string][]string{}
	for _, u := range r.Users {
		roles[u.Username], emails[u.Username] = u.RealmRoles, u.Email
	}
	for _, o := range r.Organizations {
		for _, m := range o.Members {
			members[m.Username] = append(members[m.Username], o.Alias)
		}
	}
	return roles, emails, members
}

// studentsOf returns login_username → school code of every students.csv of the plan.
func studentsOf(t *testing.T, p Plan) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, tp := range p.Tenants {
		for _, s := range tp.Schools {
			if s.Sis == "" {
				continue
			}
			f, err := os.Open(filepath.Join(p.SisDir(s), "students.csv"))
			if err != nil {
				t.Fatal(err)
			}
			rows, err := csv.NewReader(f).ReadAll()
			_ = f.Close()
			if err != nil {
				t.Fatal(err)
			}
			col := slices.Index(rows[0], "login_username")
			if col < 0 {
				t.Fatalf("%s: students.csv has no login_username", s.Code)
			}
			for _, r := range rows[1:] {
				out[r[col]] = s.Code
			}
		}
	}
	return out
}

// The committed plan loads, and every account it links exists in the realm import with the right realm role and
// Organization (LOCAL_DEVELOPMENT §5): one consistent set of demo accounts.
func TestRepositoryPlanMatchesRealmImport(t *testing.T) {
	p, err := Load(seedDir)
	if err != nil {
		t.Fatal(err)
	}
	roles, emails, members := loadRealm(t)
	tenantOfSchool := map[string]string{}
	for _, tp := range p.Tenants {
		for _, s := range tp.Schools {
			tenantOfSchool[s.Code] = tp.Alias
			for _, kind := range SisFiles {
				if _, err := os.Stat(filepath.Join(p.SisDir(s), kind+".csv")); err != nil {
					t.Errorf("school %s: %v", s.Code, err)
				}
			}
		}
	}
	alias := map[string]string{}
	for _, tp := range p.Tenants {
		alias[tp.Code] = tp.Alias
	}
	for _, s := range p.PlatformStaff {
		if !slices.Contains(roles[s.Username], "platform_staff") || emails[s.Username] != s.Email || len(members[s.Username]) != 0 {
			t.Errorf("platform staff %s: realm roles %v, email %q, organizations %v", s.Username, roles[s.Username], emails[s.Username], members[s.Username])
		}
	}
	for _, s := range p.TenantStaff {
		if !slices.Contains(roles[s.Username], "tenant_staff") || emails[s.Username] != s.Email {
			t.Errorf("tenant staff %s: realm roles %v, email %q", s.Username, roles[s.Username], emails[s.Username])
		}
		for _, r := range s.Roles {
			tenant := r.Tenant
			if tenant == "" {
				tenant = s.Tenant
			}
			if !slices.Contains(members[s.Username], alias[tenant]) {
				t.Errorf("tenant staff %s: not a member of Organization %s", s.Username, alias[tenant])
			}
		}
	}
	students := studentsOf(t, p)
	if len(students) == 0 {
		t.Fatal("no student in the SIS files")
	}
	for u, school := range students {
		if !slices.Contains(roles[u], "student") || !slices.Equal(members[u], []string{tenantOfSchool[school]}) {
			t.Errorf("student %s: realm roles %v, organizations %v, want member of %s", u, roles[u], members[u], tenantOfSchool[school])
		}
	}
	if _, ok := students["hs.nosis"]; ok {
		t.Error("hs.nosis must stay without account (ACCOUNT_NOT_PROVISIONED, ADR-0063)")
	}
	for _, l := range p.LockedStudents {
		if students[l.Username] == "" || tenantOfSchool[students[l.Username]] != alias[l.Tenant] {
			t.Errorf("locked student %s is not a student of %s", l.Username, l.Tenant)
		}
	}
	// The documented demo accounts of the quickstarts are all seeded.
	for _, u := range []string{"sa.admin", "ad.donvi", "ad.truong.a", "ad.truong.b", "hs.an.3a", "hs.binh.3a", "hs.chi.3b"} {
		seeded := students[u] != "" || slices.ContainsFunc(append(append([]StaffPlan{}, p.PlatformStaff...), p.TenantStaff...),
			func(s StaffPlan) bool { return s.Username == u })
		if !seeded {
			t.Errorf("%s is not seeded", u)
		}
	}
}

func TestValidateRejectsInconsistentPlans(t *testing.T) {
	base := func() Plan {
		return Plan{
			Tenants: []TenantPlan{
				{Code: "t-a", Name: "A", Type: "school", Alias: "t-a", Schools: []SchoolPlan{{Code: "SA", NameVi: "A", Province: "vn.ha-noi",
					Level: "primary", GradeMin: 1, GradeMax: 5}}},
				{Code: "t-b", Name: "B", Type: "unit", Alias: "t-b", Schools: []SchoolPlan{{Code: "SB", NameVi: "B", Province: "vn.ha-noi",
					Level: "primary", GradeMin: 1, GradeMax: 5}}},
			},
			PlatformStaff: []StaffPlan{{Username: "sa.x", Email: "sa.x@example.invalid", DisplayName: "SA", Roles: []RolePlan{{Role: "system_admin"}}}},
			TenantStaff: []StaffPlan{{Username: "ad.x", Email: "ad.x@example.invalid", DisplayName: "AD", Tenant: "t-a",
				Roles: []RolePlan{{Role: "school_admin", School: "SA"}}}},
		}
	}
	if err := base().Validate(); err != nil {
		t.Fatalf("base plan: %v", err)
	}
	cases := map[string]struct {
		mutate func(*Plan)
		want   string
	}{
		"first platform staff is not the bootstrap": {func(p *Plan) { p.PlatformStaff[0].Roles[0].Role = "content_admin" }, "bootstrap"},
		"tenant role for platform staff": {func(p *Plan) {
			p.PlatformStaff = append(p.PlatformStaff, StaffPlan{Username: "cm.x", Email: "c@example.invalid", DisplayName: "C",
				Roles: []RolePlan{{Role: "tenant_admin"}}})
		}, "not a platform role"},
		"platform role for tenant staff": {func(p *Plan) { p.TenantStaff[0].Roles[0] = RolePlan{Role: "system_admin"} }, "not a tenant role"},
		"school of another tenant":       {func(p *Plan) { p.TenantStaff[0].Roles[0].School = "SB" }, "not a school of t-a"},
		"no role in the home tenant": {func(p *Plan) {
			p.TenantStaff[0].Roles = []RolePlan{{Role: "academic_manager", Tenant: "t-b"}}
		}, "home tenant"},
		"two schools in a school tenant": {func(p *Plan) {
			p.Tenants[0].Schools = append(p.Tenants[0].Schools, SchoolPlan{Code: "SC", NameVi: "C", Province: "vn.ha-noi", Level: "primary",
				GradeMin: 1, GradeMax: 5})
		}, "exactly one school"},
		"duplicate username":    {func(p *Plan) { p.TenantStaff[0].Username = "sa.x" }, "duplicate"},
		"common tenant in plan": {func(p *Plan) { p.Tenants[1].Type = "common" }, "school or unit"},
		"unknown locked tenant": {func(p *Plan) { p.LockedStudents = []LockedPlan{{Username: "hs.x", Tenant: "t-z"}} }, "known tenant"},
		"grades out of range":   {func(p *Plan) { p.Tenants[0].Schools[0].GradeMax = 12 }, "1–9"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := base()
			c.mutate(&p)
			err := p.Validate()
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("Validate = %v, want %q", err, c.want)
			}
		})
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, PlanFile), []byte("tenants: []\npassword: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "password") {
		t.Fatalf("Load = %v, want an unknown field error", err)
	}
}
