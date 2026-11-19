package command

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/appscope"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	domainauthz "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// UserRoleRoute is the API route of a grant/revoke.
type UserRoleRoute int

// Routes: /tenant/user-roles, /schools/{school_id}/user-roles, /admin/user-roles.
const (
	RouteTenant UserRoleRoute = iota
	RouteSchool
	RoutePlatform
)

// GrantUserRoleInput is grantTenantUserRole / grantSchoolUserRole / grantPlatformUserRole.
type GrantUserRoleInput struct {
	Route     UserRoleRoute
	SchoolID  *uuid.UUID // path of grantSchoolUserRole
	UserID    uuid.UUID
	RoleID    uuid.UUID
	ScopeType string
	ScopeID   *uuid.UUID
	ValidFrom *time.Time
	ValidTo   *time.Time
	Reason    string
	Training  *time.Time
}

// RevokeUserRoleInput is revokeTenantUserRole / revokePlatformUserRole.
type RevokeUserRoleInput struct {
	Route      UserRoleRoute
	UserRoleID uuid.UUID
	Reason     string
}

// UserRoles grants and revokes user roles (F012 US5: AC-20 … AC-24).
type UserRoles struct {
	UoW    write.UnitOfWork
	Repo   write.UserRoleRepository
	Events write.EventPublisher
	Audit  write.Auditor
	Clock  func() time.Time
}

func (h UserRoles) now() time.Time {
	if h.Clock != nil {
		return h.Clock().UTC()
	}
	return time.Now().UTC()
}

// session resolves the data scope and the tenant of the operation.
//   - tenant/school routes: the session tenant (tenant.user_role.write is checked per resource later);
//   - platform route: platform.user_role.write; with X-Tenant-Id a grant into that tenant, else platform.
func (h UserRoles) session(ctx context.Context, route UserRoleRoute) (context.Context, authz.Principal, *uuid.UUID, error) {
	p, err := principal(ctx)
	if err != nil {
		return ctx, p, nil, err
	}
	if route == RoutePlatform {
		if err := authz.Require(ctx, p, authz.PermPlatformUserRoleWrite, authz.Resource{}); err != nil {
			return ctx, p, nil, err
		}
		if p.TenantID != nil {
			sc, err := appscope.TargetTenant(p, *p.TenantID)
			if err != nil {
				return ctx, p, nil, err
			}
			t := *p.TenantID
			return tenancy.WithScope(ctx, sc), p, &t, nil
		}
		sc, err := authz.PlatformScope(p)
		if err != nil {
			return ctx, p, nil, problem.New(problem.PermissionDenied, "").Wrap(err)
		}
		return tenancy.WithScope(ctx, sc), p, nil, nil
	}
	if p.TenantID == nil {
		return ctx, p, nil, problem.New(problem.TenantHeaderRequired, "")
	}
	sc, err := authz.ScopeFor(p)
	if err != nil {
		return ctx, p, nil, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	t := *p.TenantID
	return tenancy.WithScope(ctx, sc), p, &t, nil
}

// requireTenantGrant checks tenant.user_role.write on the school of the scope (tenant-wide otherwise).
func requireTenantGrant(ctx context.Context, p authz.Principal, tenant uuid.UUID, school *uuid.UUID) error {
	return authz.Require(ctx, p, authz.PermTenantUserRoleWrite, authz.Resource{TenantID: &tenant, SchoolID: school})
}

var grantErrors = map[error]struct{ field, msg string }{
	domainauthz.ErrScope:           {"scope_type", "scope not allowed for this role (no class scope; platform roles at platform scope only)"},
	domainauthz.ErrScopeID:         {"scope_id", "required for school/subject scopes, forbidden for tenant/platform"},
	domainauthz.ErrStudentRole:     {"user_id", "a student cannot hold a portal role"},
	domainauthz.ErrPlatformStaff:   {"role_id", "platform staff hold platform roles only (ADR-0081)"},
	domainauthz.ErrTenantStaff:     {"role_id", "tenant staff hold tenant roles only (ADR-0081)"},
	domainauthz.ErrTrainingMissing: {"training_completed_at", "required for safety_moderator"},
	domainauthz.ErrValidity:        {"valid_to", "must be after valid_from"},
}

// Grant assigns a role.
func (h UserRoles) Grant(ctx context.Context, in GrantUserRoleInput) (out write.UserRoleRow, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.GrantUserRole")
	defer func() { done(err) }()
	if utf8.RuneCountInString(in.Reason) > 1000 {
		return out, problem.Invalid("reason", problem.SubMaxLength, "at most 1000 characters")
	}
	ctx, p, tenant, err := h.session(ctx, in.Route)
	if err != nil {
		return out, err
	}
	if in.Route == RouteSchool {
		if in.SchoolID == nil || (in.ScopeType != domainauthz.ScopeSchool) || (in.ScopeID != nil && *in.ScopeID != *in.SchoolID) {
			return out, problem.Invalid("scope_type", problem.SubInvalid, "a school route grants at the scope of its school")
		}
		in.ScopeID = in.SchoolID
	}
	var school *uuid.UUID
	if in.ScopeType == domainauthz.ScopeSchool {
		school = in.ScopeID
	}
	if in.Route != RoutePlatform {
		if err := requireTenantGrant(ctx, p, *tenant, school); err != nil {
			return out, err
		}
	}
	validFrom := h.now()
	if in.ValidFrom != nil {
		validFrom = in.ValidFrom.UTC()
	}
	row := write.UserRoleRow{ID: ids.New(), UserID: in.UserID, RoleID: in.RoleID, TenantID: tenant, ScopeType: in.ScopeType, ScopeID: in.ScopeID,
		SchoolID: school, ValidFrom: validFrom, ValidTo: in.ValidTo, Source: "manual", GrantedBy: p.UserID, Training: in.Training}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		role, err := h.Repo.Role(ctx, in.RoleID, tenant)
		if err != nil {
			return err
		}
		if role == nil {
			return problem.New(problem.RoleNotFound, "")
		}
		if in.Route != RoutePlatform && role.ScopeKind != domainauthz.KindTenant {
			return problem.Invalid("role_id", problem.SubInvalid, "a tenant route grants tenant roles only")
		}
		// F012-AC-22: a School Admin (tenant.user_role.write limited to schools) never hands out a permission it does
		// not hold itself; a tenant-wide grant (tenant_admin) assigns every tenant role of ADMIN_PORTAL §2.3.
		if in.Route != RoutePlatform && !p.IsSystemAdmin && !tenantWide(p, authz.PermTenantUserRoleWrite) {
			held := map[string]bool{}
			for perm := range p.Permissions() {
				held[string(perm)] = true
			}
			if extra := domainauthz.Exceeds(role.Permissions, held); len(extra) > 0 {
				return problem.New(problem.PermissionDenied, "the role grants permissions you do not hold")
			}
		}
		target, err := h.Repo.Target(ctx, in.UserID)
		if err != nil {
			return err
		}
		if target == nil {
			return problem.Invalid("user_id", problem.SubInvalid, "unknown user")
		}
		if err := domainauthz.CheckGrant(domainauthz.Grant{RoleCode: role.Code, RoleKind: role.ScopeKind, ScopeType: in.ScopeType,
			HasScopeID: in.ScopeID != nil, UserType: target.UserType, UserHasHome: target.PrimaryTenantID != nil, TrainingDone: in.Training != nil,
			ValidityOrder: in.ValidTo == nil || in.ValidTo.After(validFrom)}); err != nil {
			for e, f := range grantErrors {
				if errors.Is(err, e) {
					return problem.Invalid(f.field, problem.SubInvalid, f.msg)
				}
			}
			return err
		}
		if school != nil {
			ok, err := h.Repo.SchoolInTenant(ctx, *school, *tenant)
			if err != nil {
				return err
			}
			if !ok {
				return problem.Invalid("scope_id", problem.SubInvalid, "unknown school of the tenant")
			}
		}
		row.RoleCode = role.Code
		if err := h.Repo.Insert(ctx, &row); err != nil {
			return err
		}
		if err := h.Repo.BumpUserVersion(ctx, in.UserID); err != nil {
			return err
		}
		return h.changed(ctx, row, "granted", p.UserID, in.Reason)
	})
	return row, err
}

// Revoke revokes a role (soft-delete); permissions disappear within principal_cache_ttl (event).
func (h UserRoles) Revoke(ctx context.Context, in RevokeUserRoleInput) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.RevokeUserRole")
	defer func() { done(err) }()
	reason := strings.TrimSpace(in.Reason)
	if n := utf8.RuneCountInString(reason); n < 3 || n > 1000 {
		return problem.Invalid("reason", problem.SubInvalid, "3 to 1000 characters")
	}
	ctx, p, tenant, err := h.session(ctx, in.Route)
	if err != nil {
		return err
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		row, err := h.Repo.Get(ctx, in.UserRoleID)
		if err != nil {
			return err
		}
		if (tenant == nil) != (row.TenantID == nil) || (tenant != nil && *row.TenantID != *tenant) {
			return problem.New(problem.NotFound, "")
		}
		if in.Route != RoutePlatform {
			if err := requireTenantGrant(ctx, p, *tenant, row.SchoolID); err != nil {
				return problem.New(problem.NotFound, "").Wrap(err) // outside the school scope: not visible
			}
		}
		if err := h.Repo.Revoke(ctx, row.ID, p.UserID, "revoked"); err != nil {
			return err
		}
		if err := h.Repo.BumpUserVersion(ctx, row.UserID); err != nil {
			return err
		}
		return h.changed(ctx, *row, "revoked", p.UserID, reason)
	})
}

// tenantWide reports a grant of perm not limited to a school or subject.
func tenantWide(p authz.Principal, perm authz.Permission) bool {
	for _, g := range p.Grants {
		if g.SchoolID == nil && g.SubjectID == nil && slices.Contains(g.Permissions, perm) {
			return true
		}
	}
	return false
}

// changed publishes user_role.changed and records the audit (F012-AC-23).
func (h UserRoles) changed(ctx context.Context, r write.UserRoleRow, op string, actor uuid.UUID, reason string) error {
	payload := map[string]any{"user_role_id": r.ID.String(), "user_id": r.UserID.String(), "role_id": r.RoleID.String(), "role_code": r.RoleCode,
		"tenant_id": nil, "scope_type": r.ScopeType, "scope_id": nil, "source": r.Source, "op": op, "valid_from": r.ValidFrom.Format(time.RFC3339),
		"valid_to": nil, "actor_id": actor.String()}
	if r.TenantID != nil {
		payload["tenant_id"] = r.TenantID.String()
	}
	if r.ScopeID != nil {
		payload["scope_id"] = r.ScopeID.String()
	}
	if r.ValidTo != nil {
		payload["valid_to"] = r.ValidTo.Format(time.RFC3339)
	}
	ev := write.Event{Type: "user_role.changed", SubjectType: "user_role", SubjectID: r.ID.String(), TenantID: r.TenantID,
		Platform: r.TenantID == nil, Payload: payload}
	if err := h.Events.Publish(ctx, ev); err != nil {
		return err
	}
	return h.Audit.Record(ctx, audit.Event{Action: "user_role." + op, TargetType: "user", TargetID: r.UserID.String(), Result: audit.ResultSuccess,
		Metadata: map[string]any{"role_code": r.RoleCode, "scope_type": r.ScopeType, "user_role_id": r.ID.String(), "reason": reason}})
}
