package authz

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
)

// RoleScopeKind is role_scope_kind of a role (ADMIN_PORTAL §2.3, ADR-0066).
type RoleScopeKind string

const (
	// RoleScopeTenant roles are assigned inside one tenant (scope tenant, school or subject).
	RoleScopeTenant RoleScopeKind = "tenant"
	// RoleScopePlatform roles have tenant_id = NULL (Admin Portal only; no MFA since ADR-0094).
	RoleScopePlatform RoleScopeKind = "platform"
)

// RoleDef is a system role definition (F002 T033).
type RoleDef struct {
	Name        string
	ScopeKind   RoleScopeKind
	Permissions []Permission
}

// Has reports whether the role grants p.
func (r RoleDef) Has(p Permission) bool { return slices.Contains(r.Permissions, p) }

// expand resolves patterns ("*", "<prefix>.*" or an exact code) against the catalog, minus excludes.
// A pattern that matches nothing is a programming error caught by the package tests.
func expand(patterns []string, exclude ...Permission) []Permission {
	var out []Permission
	for _, pat := range patterns {
		matched := false
		for _, p := range catalog {
			ok := pat == "*" || string(p) == pat ||
				(strings.HasSuffix(pat, ".*") && strings.HasPrefix(string(p), strings.TrimSuffix(pat, "*")))
			if ok {
				matched = true
				if !slices.Contains(out, p) && !slices.Contains(exclude, p) {
					out = append(out, p)
				}
			}
		}
		if !matched {
			panic("authz: role pattern matches no permission: " + pat)
		}
	}
	slices.Sort(out)
	return out
}

// SystemRoles returns the system roles of ADMIN_PORTAL §2.3. There is no teacher or tenant_moderator role
// (ADR-0072). Deviations recorded in the F002 clarifications [Proposed]: report.export goes to
// tenant_admin, school_admin and academic_manager; school_admin also holds social.club.approve and
// tenant.audit.read; tenant_admin does not hold learning_content.re_evaluate (§2.4 matrix);
// content_reviewer also holds learning_content.read.
func SystemRoles() []RoleDef {
	return []RoleDef{
		{Name: "tenant_admin", ScopeKind: RoleScopeTenant, Permissions: expand([]string{
			"tenant.*", "school.*", "lms.class.read", "lms.assignment.read", "lms.report.read", "lms.school_event.write",
			"lms.student.password_reset", "learning_content.*", "curriculum.read", "analytics.tenant.read",
			"moderation.school_notification.read", "moderation.school_notification.acknowledge", "chat.sensitive_terms.read",
			"report.export",
		}, PermLearningContentReEvaluate)},
		{Name: "school_admin", ScopeKind: RoleScopeTenant, Permissions: expand([]string{
			"school.*", "tenant.user.read", "tenant.user.lock", "tenant.user_role.write", "lms.*", "learning_content.read",
			"learning_content.write", "learning_content.review", "learning_content.publish", "analytics.school.read",
			"analytics.class.read", "moderation.school_notification.read", "moderation.school_notification.acknowledge",
			"tenant.sis.import", "social.club.approve", "tenant.audit.read", "report.export",
		})},
		{Name: "academic_manager", ScopeKind: RoleScopeTenant, Permissions: expand([]string{
			"curriculum.*", "learning_content.*", "lms.class.read",
			"lms.assignment.*", "lms.result.release", "lms.report.read",
			"lms.student_activity.read", "analytics.*", "report.export",
		})},
		{Name: "tenant_content_editor", ScopeKind: RoleScopeTenant, Permissions: expand([]string{
			"learning_content.read", "learning_content.write", "learning_content.import", "learning_content.activity.write",
			"media.upload",
		})},
		{Name: "tenant_content_reviewer", ScopeKind: RoleScopeTenant, Permissions: expand([]string{
			"learning_content.read", "learning_content.review", "learning_content.publish", "learning_content.activity.review",
		})},
		{Name: "system_admin", ScopeKind: RoleScopePlatform, Permissions: expand([]string{"*"})},
		{Name: "content_admin", ScopeKind: RoleScopePlatform, Permissions: expand([]string{"learning_content.*", "curriculum.*"})},
		{Name: "content_reviewer", ScopeKind: RoleScopePlatform, Permissions: expand([]string{
			"learning_content.read", "learning_content.review", "learning_content.publish", "game.content.review",
		})},
		{Name: "game_designer", ScopeKind: RoleScopePlatform, Permissions: expand([]string{"game.*"},
			PermGameContentPublish, PermGameEconomyAdjust)},
		{Name: "liveops", ScopeKind: RoleScopePlatform, Permissions: expand([]string{
			"game.event.write", "game.event.operate", "game.season_track.write", "game.content.publish",
			"content.release.rollback", "game.liveops.read", "game.analytics.read", "competition.config.season.write",
		})},
		{Name: "competition_admin", ScopeKind: RoleScopePlatform, Permissions: expand([]string{"competition.config.*"})},
		{Name: "safety_moderator", ScopeKind: RoleScopePlatform, Permissions: expand([]string{
			"moderation.case.read", "moderation.case.act", "moderation.case.messages.read", "moderation.sanction.read",
			"moderation.sanction.write", "moderation.school_notification.send", "chat.sensitive_terms.read",
			"chat.tenant_terms.write", "chat.tenant_category.toggle",
		})},
	}
}

// Violation is one problem found by ValidateCatalog.
type Violation struct {
	Role       string
	Permission Permission
	Reason     string
}

// CatalogError lists every violation; it names roles and permissions only.
type CatalogError struct{ Violations []Violation }

func (e *CatalogError) Error() string {
	parts := make([]string, 0, len(e.Violations))
	for _, v := range e.Violations {
		parts = append(parts, fmt.Sprintf("role %q permission %q: %s", v.Role, v.Permission, v.Reason))
	}
	return "authz: invalid role catalog: " + strings.Join(parts, "; ")
}

// ValidateCatalog checks role definitions against the permission catalog: unique names, a known scope
// kind, known permissions, and no platform-only permission in a tenant role (F002-AC-14, F002-AC-15).
func ValidateCatalog(roles []RoleDef) error {
	var vs []Violation
	seen := map[string]bool{}
	for _, r := range roles {
		if r.Name == "" || seen[r.Name] {
			vs = append(vs, Violation{Role: r.Name, Reason: "empty or duplicate role name"})
		}
		seen[r.Name] = true
		if r.ScopeKind != RoleScopeTenant && r.ScopeKind != RoleScopePlatform {
			vs = append(vs, Violation{Role: r.Name, Reason: "unknown role_scope_kind " + string(r.ScopeKind)})
		}
		for _, p := range r.Permissions {
			switch {
			case !p.Known():
				vs = append(vs, Violation{Role: r.Name, Permission: p, Reason: "not in the permission catalog"})
			case r.ScopeKind == RoleScopeTenant && p.PlatformOnly():
				vs = append(vs, Violation{Role: r.Name, Permission: p, Reason: "platform-only permission in a tenant role (AGENTS R20)"})
			}
		}
	}
	if len(vs) > 0 {
		return &CatalogError{Violations: vs}
	}
	return nil
}

var exit = os.Exit

// SetExitForTest replaces os.Exit in MustValidate and returns a restore function.
func SetExitForTest(f func(int)) (restore func()) {
	old := exit
	exit = f
	return func() { exit = old }
}

// MustValidate logs every violation (role and permission names, JSON) to w and exits with code 1.
func MustValidate(w io.Writer, roles []RoleDef) {
	err := ValidateCatalog(roles)
	if err == nil {
		return
	}
	log := slog.New(slog.NewJSONHandler(w, nil))
	var ce *CatalogError
	if errors.As(err, &ce) {
		for _, v := range ce.Violations {
			log.Error("invalid role catalog", "role", v.Role, "permission", string(v.Permission), "reason", v.Reason)
		}
	}
	exit(1)
}

// MustValidateAtStartup validates SystemRoles; every binary calls it before serving (F002-AC-14).
func MustValidateAtStartup() { MustValidate(os.Stderr, SystemRoles()) }
