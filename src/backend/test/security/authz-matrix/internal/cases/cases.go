// Package cases holds the domain of the authz-matrix (F132-AC-15, TC-TEN-001): which request is sent for an
// operation × role × scope relation, and which answer is expected. Expectations are the negative security rules
// of the platform, independent of the implementation:
//
//   - no token on a non-public operation → 401;
//   - a tenant role reaching a resource of another tenant (id of tenant B with its own X-Tenant-Id, or X-Tenant-Id
//     of tenant B) → 404 (existence not leaked) or 403 (R8, ADR-0005, SECURITY_ARCHITECTURE §13.4);
//   - staff on a player route → 403 GAME_ACCESS_STUDENT_ONLY (R23, ADR-0031);
//   - a student on an Admin Portal route → 403 PORTAL_ACCESS_STAFF_ONLY; a tenant role on a platform route
//     (/game-admin/*, /admin/*) → 403 (R20, ADR-0027);
//   - never a 5xx.
//
// Positive access (a role allowed by its permissions) depends on permissions and fixtures; those cases are sent and
// reported as observations, not asserted. Platform roles are legitimately cross-tenant (R20: system_admin with
// tenant_ids, safety_moderator in moderation) and get no cross-tenant expectation.
package cases

import (
	"sort"
	"strings"

	"github.com/tanguyen2020/eduworld-platform/src/backend/test/security/authz-matrix/internal/spec"
)

// Role is a persona of the matrix (TEST_STRATEGY §5 demo accounts).
type Role string

// Roles of the matrix.
const (
	Anonymous       Role = "anonymous"
	Student         Role = "student"
	SchoolAdmin     Role = "school_admin"
	AcademicManager Role = "academic_manager"
	TenantAdmin     Role = "tenant_admin"
	SafetyModerator Role = "safety_moderator"
	SystemAdmin     Role = "system_admin"
)

// Roles are the authenticated roles of the matrix, in report order.
var Roles = []Role{Student, SchoolAdmin, AcademicManager, TenantAdmin, SafetyModerator, SystemAdmin}

// Platform reports a platform role (cross-tenant by design, R20).
func (r Role) Platform() bool { return r == SafetyModerator || r == SystemAdmin }

// Staff reports a staff role (Admin Portal token).
func (r Role) Staff() bool { return r != Student && r != Anonymous }

// Relation is the scope relation between the caller (tenant A, school A1) and the target resource.
type Relation string

// Relations of the matrix.
const (
	None                  Relation = "none" // anonymous: no caller
	SameSchool            Relation = "same_school"
	OtherSchoolSameTenant Relation = "other_school_same_tenant"
	OtherTenant           Relation = "other_tenant"
)

// Class is the route group of an operation (API_DESIGN §11, pkg/httpx effectiveGuard, AGENTS R23).
type Class string

// Route groups.
const (
	ClassPublic   Class = "public"
	ClassPlayer   Class = "player"   // game_player: students of the game client only
	ClassPortal   Class = "portal"   // portal_staff
	ClassPlatform Class = "platform" // portal_staff + platform role
	ClassOther    Class = "other"    // guarded by permissions only (learning, exams, competition, leaderboards…)
)

var (
	// Same prefixes as pkg/httpx (portalPrefixes, platformPrefixes): the route group implies the guard.
	portalPrefixes   = []string{"/cms/", "/lms/", "/tenant/", "/schools/", "/game-admin/", "/admin/"}
	platformPrefixes = []string{"/game-admin/", "/admin/"}
	// AGENTS R23: every player route — /game/* except client-config, /multiplayer/*, /social/* — plus the game chat and
	// the in-game notification inbox (ADR-0031: chat and notifications exist only in the game).
	playerPrefixes = []string{"/game/", "/multiplayer/", "/social/", "/chat/", "/notifications"}
	playerExempt   = []string{"/game/client-config"}
)

func hasPrefix(p string, prefixes []string) bool {
	for _, pre := range prefixes {
		if strings.HasPrefix(p, pre) || p+"/" == pre {
			return true
		}
	}
	return false
}

// Overrides adjust single operations (reviewed file expectations.yaml): e.g. a /competition/* route that is
// player-only, or an operation not testable on a shared environment.
type Overrides struct {
	Class map[string]Class `yaml:"class"`
	// Skip lists operations left out, with the reason.
	Skip map[string]string `yaml:"skip"`
}

// ClassOf returns the route group of op.
func ClassOf(op spec.Operation, ov Overrides) Class {
	if c, ok := ov.Class[op.ID]; ok {
		return c
	}
	switch {
	case op.Public:
		return ClassPublic
	case hasPrefix(op.Path, platformPrefixes):
		return ClassPlatform
	case hasPrefix(op.Path, portalPrefixes):
		return ClassPortal
	case hasPrefix(op.Path, playerPrefixes) && !exempt(op.Path):
		return ClassPlayer
	}
	return ClassOther
}

func exempt(p string) bool {
	for _, e := range playerExempt {
		if p == e || strings.HasPrefix(p, e+"/") {
			return true
		}
	}
	return false
}

// Expect is the expected answer of a case. Observe = true means no assertion (reported only), except no 5xx.
type Expect struct {
	Statuses []int    `json:"statuses,omitempty"`
	Codes    []string `json:"codes,omitempty"` // Problem Details code; empty = any
	Observe  bool     `json:"observe,omitempty"`
	Rule     string   `json:"rule"`
}

// Case is one request of the matrix.
type Case struct {
	Op       spec.Operation `json:"-"`
	OpID     string         `json:"operation_id"`
	Class    Class          `json:"class"`
	Role     Role           `json:"role"`
	Relation Relation       `json:"relation"`
	// ForeignTenantHeader: X-Tenant-Id names tenant B (operation without id parameter); otherwise the caller's tenant.
	ForeignTenantHeader bool   `json:"foreign_tenant_header,omitempty"`
	Expect              Expect `json:"expect"`
}

var (
	denyCross = []int{403, 404}
	forbidden = []int{403}
)

// Generate builds every case of ops, sorted by operation (stable in role, relation order). Public and skipped
// operations produce no case.
func Generate(ops []spec.Operation, ov Overrides) []Case {
	var out []Case
	for _, op := range ops {
		if _, skip := ov.Skip[op.ID]; skip {
			continue
		}
		cl := ClassOf(op, ov)
		if cl == ClassPublic {
			continue
		}
		add := func(r Role, rel Relation, foreign bool, e Expect) {
			out = append(out, Case{Op: op, OpID: op.ID, Class: cl, Role: r, Relation: rel, ForeignTenantHeader: foreign, Expect: e})
		}
		add(Anonymous, None, false, Expect{Statuses: []int{401}, Rule: "no_token_401"})
		for _, r := range Roles {
			add(r, SameSchool, false, roleExpect(cl, r))
			if op.HasIDParam {
				add(r, OtherSchoolSameTenant, false, Expect{Observe: true, Rule: "other_school_observed"})
			}
			if op.HasIDParam || op.TenantScoped {
				add(r, OtherTenant, !op.HasIDParam, crossExpect(r))
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].OpID < out[j].OpID })
	return out
}

// roleExpect is the expectation of a role on its own tenant and school (route group guards).
func roleExpect(cl Class, r Role) Expect {
	switch cl {
	case ClassPlayer:
		if r.Platform() { // a platform principal has no tenant: the X-Tenant-Id check may answer first
			return Expect{Statuses: forbidden, Codes: []string{"GAME_ACCESS_STUDENT_ONLY", "TENANT_MISMATCH"}, Rule: "player_route_staff_403"}
		}
		if r.Staff() {
			return Expect{Statuses: forbidden, Codes: []string{"GAME_ACCESS_STUDENT_ONLY"}, Rule: "player_route_staff_403"}
		}
	case ClassPortal:
		if r == Student {
			return Expect{Statuses: forbidden, Codes: []string{"PORTAL_ACCESS_STAFF_ONLY"}, Rule: "portal_route_student_403"}
		}
	case ClassPlatform:
		if r == Student {
			return Expect{Statuses: forbidden, Codes: []string{"PORTAL_ACCESS_STAFF_ONLY"}, Rule: "portal_route_student_403"}
		}
		if !r.Platform() {
			return Expect{Statuses: forbidden, Codes: []string{"PERMISSION_DENIED", "TENANT_MISMATCH"}, Rule: "platform_route_tenant_role_403"}
		}
	}
	return Expect{Observe: true, Rule: "own_scope_observed"}
}

// crossExpect is the expectation of a role on a resource (or X-Tenant-Id) of another tenant.
func crossExpect(r Role) Expect {
	if r.Platform() {
		return Expect{Observe: true, Rule: "platform_cross_tenant_observed"} // R20: legitimately cross-tenant
	}
	return Expect{Statuses: denyCross, Rule: "cross_tenant_404_or_403"}
}

// Satisfied reports whether status/code meet e (observations are satisfied by any non-5xx answer).
func (e Expect) Satisfied(status int, code string) bool {
	if status >= 500 {
		return false
	}
	if e.Observe {
		return true
	}
	okStatus := false
	for _, s := range e.Statuses {
		if s == status {
			okStatus = true
		}
	}
	if !okStatus {
		return false
	}
	if len(e.Codes) == 0 {
		return true
	}
	for _, c := range e.Codes {
		if c == code {
			return true
		}
	}
	return false
}
