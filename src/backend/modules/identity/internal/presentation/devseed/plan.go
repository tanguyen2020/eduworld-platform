// Package devseed is the dev seed of module identity (LOCAL_DEVELOPMENT §5): it reads the plan of
// modules/identity/seed/dev.yaml and the SIS CSV files next to it, then creates the demo tenants, schools, staff,
// roles and students through the use cases of the module (never SQL by hand). Every step looks first and acts only
// when something is missing, so running the seed again changes nothing.
package devseed

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"go.yaml.in/yaml/v3"

	domainauthz "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
)

// PlanFile is the name of the plan in the seed directory.
const PlanFile = "dev.yaml"

// SisFiles are the CSV files of a school directory: students carry their school and grade (SIS contract 1.3.0,
// ADR-0095 — no classes.csv, no enrollments.csv).
var SisFiles = []string{"students"}

// Plan is the dev seed of module identity.
type Plan struct {
	Tenants        []TenantPlan `yaml:"tenants"`
	PlatformStaff  []StaffPlan  `yaml:"platform_staff"`
	TenantStaff    []StaffPlan  `yaml:"tenant_staff"`
	LockedStudents []LockedPlan `yaml:"locked_students"`
	dir            string
}

// TenantPlan is one tenant and its schools.
type TenantPlan struct {
	Code    string       `yaml:"code"`
	Name    string       `yaml:"name"`
	Type    string       `yaml:"type"`
	Alias   string       `yaml:"alias"`
	Schools []SchoolPlan `yaml:"schools"`
}

// SchoolPlan is one school; Sis is the directory (relative to the plan) of its SIS CSV files.
type SchoolPlan struct {
	Code     string `yaml:"code"`
	NameVi   string `yaml:"name_vi"`
	NameEn   string `yaml:"name_en"`
	Province string `yaml:"province"`
	Level    string `yaml:"level"`
	GradeMin int    `yaml:"grade_min"`
	GradeMax int    `yaml:"grade_max"`
	Sis      string `yaml:"sis"`
	// AcademicYear is the current academic year of the school in its SIS (YYYY-YYYY), created by the import (ADR-0095).
	AcademicYear string `yaml:"academic_year"`
}

// StaffPlan is a staff account; Tenant is the home tenant (empty for platform staff).
type StaffPlan struct {
	Username    string     `yaml:"username"`
	Email       string     `yaml:"email"`
	DisplayName string     `yaml:"display_name"`
	Tenant      string     `yaml:"tenant"`
	Roles       []RolePlan `yaml:"roles"`
}

// RolePlan is one grant: Tenant defaults to the home tenant; School limits the grant to one school (by code).
type RolePlan struct {
	Role     string `yaml:"role"`
	Tenant   string `yaml:"tenant"`
	School   string `yaml:"school"`
	Training bool   `yaml:"training"` // training_completed_at = seed time (safety_moderator)
}

// LockedPlan is a student locked by the seed (lockTenantUser).
type LockedPlan struct {
	Username string `yaml:"username"`
	Tenant   string `yaml:"tenant"`
}

// Dir is the seed directory the plan was loaded from.
func (p Plan) Dir() string { return p.dir }

// SisDir returns the absolute directory of the SIS files of a school ("" when the school has none).
func (p Plan) SisDir(s SchoolPlan) string {
	if s.Sis == "" {
		return ""
	}
	return filepath.Join(p.dir, filepath.FromSlash(s.Sis))
}

// Load reads and validates dir/dev.yaml.
func Load(dir string) (Plan, error) {
	b, err := os.ReadFile(filepath.Join(dir, PlanFile)) //nolint:gosec // seed directory of the repository
	if err != nil {
		return Plan{}, err
	}
	var p Plan
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil {
		return Plan{}, fmt.Errorf("%s: %w", PlanFile, err)
	}
	p.dir = dir
	if err := p.Validate(); err != nil {
		return Plan{}, fmt.Errorf("%s: %w", PlanFile, err)
	}
	return p, nil
}

func roleKinds() map[string]authz.RoleScopeKind {
	out := map[string]authz.RoleScopeKind{}
	for _, r := range authz.SystemRoles() {
		out[r.Name] = r.ScopeKind
	}
	return out
}

// Validate checks the references of the plan: unique codes and usernames, known roles of the right kind, schools of
// the grant tenant, a first platform staff that is the bootstrap System Admin.
func (p Plan) Validate() error {
	kinds := roleKinds()
	tenants, schools := map[string]TenantPlan{}, map[string]string{} // school code → tenant code
	for _, t := range p.Tenants {
		if t.Code == "" || t.Name == "" || t.Alias == "" {
			return fmt.Errorf("tenant %q: code, name and alias are required", t.Code)
		}
		if !slices.Contains([]string{"school", "unit"}, t.Type) {
			return fmt.Errorf("tenant %s: type must be school or unit (the common tenant comes with the migrations)", t.Code)
		}
		if _, dup := tenants[t.Code]; dup {
			return fmt.Errorf("tenant %s: duplicate", t.Code)
		}
		tenants[t.Code] = t
		if t.Type == "school" && len(t.Schools) != 1 {
			return fmt.Errorf("tenant %s: a school tenant has exactly one school", t.Code)
		}
		for _, s := range t.Schools {
			if s.Code == "" || s.NameVi == "" || s.Province == "" || s.Level == "" {
				return fmt.Errorf("school %q of %s: code, name_vi, province and level are required", s.Code, t.Code)
			}
			if _, dup := schools[s.Code]; dup {
				return fmt.Errorf("school %s: duplicate", s.Code)
			}
			if s.GradeMin < 1 || s.GradeMax > 9 || s.GradeMin > s.GradeMax {
				return fmt.Errorf("school %s: grades must be within 1–9", s.Code)
			}
			schools[s.Code] = t.Code
		}
	}
	users := map[string]bool{}
	unique := func(u string) error {
		if u == "" {
			return fmt.Errorf("a username is required")
		}
		if users[u] {
			return fmt.Errorf("username %s: duplicate", u)
		}
		users[u] = true
		return nil
	}
	if len(p.PlatformStaff) == 0 || len(p.PlatformStaff[0].Roles) != 1 || p.PlatformStaff[0].Roles[0].Role != "system_admin" {
		return fmt.Errorf("platform_staff: the first entry must be the bootstrap System Admin (role system_admin only)")
	}
	for _, s := range p.PlatformStaff {
		if err := unique(s.Username); err != nil {
			return err
		}
		if s.Tenant != "" || s.Email == "" || s.DisplayName == "" || len(s.Roles) == 0 {
			return fmt.Errorf("platform staff %s: email, display_name and roles are required, no tenant (ADR-0081)", s.Username)
		}
		for _, r := range s.Roles {
			if kinds[r.Role] != authz.RoleScopePlatform || r.Tenant != "" || r.School != "" {
				return fmt.Errorf("platform staff %s: %s is not a platform role at platform scope", s.Username, r.Role)
			}
		}
	}
	for _, s := range p.TenantStaff {
		if err := unique(s.Username); err != nil {
			return err
		}
		if _, ok := tenants[s.Tenant]; !ok || s.Email == "" || s.DisplayName == "" || len(s.Roles) == 0 {
			return fmt.Errorf("tenant staff %s: a known home tenant, email, display_name and roles are required", s.Username)
		}
		home := false
		for _, r := range s.Roles {
			t := r.Tenant
			if t == "" {
				t, home = s.Tenant, true
			}
			if _, ok := tenants[t]; !ok {
				return fmt.Errorf("tenant staff %s: unknown tenant %s", s.Username, t)
			}
			if kinds[r.Role] != authz.RoleScopeTenant || r.Training {
				return fmt.Errorf("tenant staff %s: %s is not a tenant role", s.Username, r.Role)
			}
			if r.School != "" && schools[r.School] != t {
				return fmt.Errorf("tenant staff %s: school %s is not a school of %s", s.Username, r.School, t)
			}
		}
		if !home {
			return fmt.Errorf("tenant staff %s: at least one role in the home tenant is required", s.Username)
		}
	}
	for _, l := range p.LockedStudents {
		if _, ok := tenants[l.Tenant]; !ok || l.Username == "" {
			return fmt.Errorf("locked student %q: a known tenant is required", l.Username)
		}
	}
	return nil
}

// ScopeOf returns the scope type of a tenant grant (school when limited to a school, else the whole tenant).
func (r RolePlan) ScopeOf() string {
	if r.School != "" {
		return domainauthz.ScopeSchool
	}
	return domainauthz.ScopeTenant
}
