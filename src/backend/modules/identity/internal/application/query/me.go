package query

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Me holds the session user's own reads (F012 US2): listMyTenants, getMyPermissions, getMe. They build the
// portal session after portalLogin (ADR-0091): listMyTenants → (choose a tenant) → getMyPermissions → getMe.
type Me struct {
	Model      read.MeReadModel
	Workspaces port.WorkspaceCatalog
	Toggles    port.FeatureToggles // nil until F015 provides featuregate.Source: no menu is disabled
}

// MyTenant is one item of MyTenantsResponse.
type MyTenant struct {
	TenantID   uuid.UUID `json:"tenant_id"`
	Code       string    `json:"code"`
	Name       string    `json:"name"`
	TenantType string    `json:"tenant_type"`
	Status     string    `json:"status"`
	RoleCodes  []string  `json:"role_codes"`
}

// MyTenants is MyTenantsResponse.
type MyTenants struct {
	Items           []MyTenant `json:"items"`
	HasPlatformRole bool       `json:"has_platform_role"`
}

// EffectiveRole is a role of the Principal with its scope.
type EffectiveRole struct {
	Role      string     `json:"role"`
	ScopeType string     `json:"scope_type"`
	TenantID  *uuid.UUID `json:"tenant_id"`
	ScopeID   *uuid.UUID `json:"scope_id,omitempty"`
}

// PermissionSet is the response of getMyPermissions.
type PermissionSet struct {
	TenantID        *uuid.UUID      `json:"tenant_id"`
	Roles           []EffectiveRole `json:"roles"`
	Permissions     []string        `json:"permissions"`
	Workspaces      []string        `json:"workspaces"`
	DisabledMenus   []string        `json:"disabled_menus"`
	DisabledActions []string        `json:"disabled_actions"`
}

// MeView is the Principal schema of getMe (no PII beyond the display name).
type MeView struct {
	UserID      uuid.UUID       `json:"user_id"`
	UserType    string          `json:"user_type"`
	Status      string          `json:"status"`
	DisplayName string          `json:"display_name,omitempty"`
	Roles       []EffectiveRole `json:"roles"`
}

// ListMyTenants is listMyTenants (F012-AC-04, AC-05): tenants with membership and valid portal roles, and
// has_platform_role. On the portal, staff with no tenant role and no platform role get
// PORTAL_ACCESS_STAFF_ONLY (recorded as an authorization denial).
func (q Me) ListMyTenants(ctx context.Context) (out MyTenants, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListMyTenants")
	defer func() { done(err) }()
	p, err := authenticated(ctx)
	if err != nil {
		return out, err
	}
	rows, platform, err := q.Model.MyTenants(ctx, p.UserID)
	if err != nil {
		return out, err
	}
	out = MyTenants{Items: []MyTenant{}, HasPlatformRole: platform}
	portal := p.Channel == authz.ChannelPortal
	for _, r := range rows {
		if portal && len(r.RoleCodes) == 0 {
			continue // portal: only tenants where the staff holds a valid portal role
		}
		roles := r.RoleCodes
		if roles == nil {
			roles = []string{}
		}
		sort.Strings(roles)
		out.Items = append(out.Items, MyTenant{TenantID: r.TenantID, Code: r.Code, Name: r.Name, TenantType: r.TenantType, Status: r.Status, RoleCodes: roles})
	}
	if portal && (p.UserType != authz.UserTypeStaff || (len(out.Items) == 0 && !platform)) {
		authz.RecordDenied(ctx, string(problem.PortalAccessStaffOnly), map[string]any{"operation": "listMyTenants"})
		return MyTenants{}, problem.New(problem.PortalAccessStaffOnly, "")
	}
	return out, nil
}

// GetMyPermissions is getMyPermissions (F012-AC-10, AC-39): roles, permissions and workspaces of the session
// scope (tenant of X-Tenant-Id; a System Admin without header = platform), plus the disabled menus/actions.
func (q Me) GetMyPermissions(ctx context.Context) (out PermissionSet, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.GetMyPermissions")
	defer func() { done(err) }()
	p, err := authenticated(ctx)
	if err != nil {
		return out, err
	}
	perms := make([]string, 0, len(p.Permissions()))
	for perm := range p.Permissions() {
		// A tenant session never carries platform-only permissions (ADR-0027, F012-AC-10).
		if p.TenantID != nil && !p.IsSystemAdmin && perm.PlatformOnly() {
			continue
		}
		perms = append(perms, string(perm))
	}
	sort.Strings(perms)
	out = PermissionSet{TenantID: p.TenantID, Roles: effectiveRoles(p), Permissions: perms, Workspaces: []string{},
		DisabledMenus: []string{}, DisabledActions: []string{}}
	if q.Workspaces != nil {
		out.Workspaces = q.Workspaces.Workspaces(perms)
	}
	if q.Toggles != nil {
		menus, actions, err := q.Toggles.Disabled(ctx, p.TenantID)
		if err != nil {
			return PermissionSet{}, problem.New(problem.Unavailable, "feature toggles unavailable").Retry().Wrap(err)
		}
		out.DisabledMenus, out.DisabledActions = nonNilStrings(menus), nonNilStrings(actions)
	}
	return out, nil
}

// GetMe is getMe: the session user with its effective roles.
func (q Me) GetMe(ctx context.Context) (out MeView, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.GetMe")
	defer func() { done(err) }()
	p, err := authenticated(ctx)
	if err != nil {
		return out, err
	}
	name, err := q.Model.DisplayName(ctx, p.UserID)
	if err != nil {
		return out, err
	}
	// The Principal middleware only lets active users through.
	return MeView{UserID: p.UserID, UserType: string(p.UserType), Status: "active", DisplayName: name, Roles: effectiveRoles(p)}, nil
}

func effectiveRoles(p authz.Principal) []EffectiveRole {
	out := make([]EffectiveRole, 0, len(p.Grants))
	for _, g := range p.Grants {
		r := EffectiveRole{Role: g.Role, ScopeType: "tenant", TenantID: p.TenantID}
		switch {
		case g.ScopeKind == authz.RoleScopePlatform:
			r.ScopeType, r.TenantID = "platform", nil
		case g.SubjectID != nil:
			r.ScopeType, r.ScopeID = "subject", g.SubjectID
		case g.SchoolID != nil:
			r.ScopeType, r.ScopeID = "school", g.SchoolID
		}
		out = append(out, r)
	}
	return out
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
