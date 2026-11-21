package authz

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// UserType is derived from realm roles (ADR-0081): only students play, only staff use the portal.
type UserType string

const (
	UserTypeStudent UserType = "student"
	UserTypeStaff   UserType = "staff"
)

// TenantType is the kind of tenant (ADR-0005).
type TenantType string

const (
	TenantTypeSchool TenantType = "school"
	TenantTypeUnit   TenantType = "unit"
	TenantTypeCommon TenantType = "common"
)

// Channel is the client a token was issued to, resolved by httpx from EW_KC_PORTAL_CLIENT_ID and
// EW_KC_GAME_CLIENT_ID (client ids are configuration, AGENTS R3).
type Channel string

const (
	ChannelUnknown Channel = ""
	ChannelPortal  Channel = "portal"
	ChannelGame    Channel = "game"
)

// PermissionSet is a set of permissions.
type PermissionSet map[Permission]struct{}

// Has reports whether p is in the set.
func (s PermissionSet) Has(p Permission) bool { _, ok := s[p]; return ok }

// List returns the permissions sorted.
func (s PermissionSet) List() []Permission {
	out := make([]Permission, 0, len(s))
	for p := range s {
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}

// Grant is one effective UserRole: its permissions and the school/subject it is limited to (nil = the
// whole tenant, or the platform for platform roles). There is no class scope (ADR-0072).
type Grant struct {
	Role        string        `json:"role"`
	ScopeKind   RoleScopeKind `json:"scope_kind"` // role_scope_kind of the role
	Permissions []Permission  `json:"permissions"`
	SchoolID    *uuid.UUID    `json:"school_id,omitempty"`
	SubjectID   *uuid.UUID    `json:"subject_id,omitempty"`
}

// PrincipalData is what the identity module resolves plus what httpx reads from the token.
type PrincipalData struct {
	UserID        uuid.UUID  `json:"user_id"`
	UserType      UserType   `json:"user_type"`
	TenantID      *uuid.UUID `json:"tenant_id,omitempty"` // nil = platform staff (or System Admin reading cross-tenant)
	TenantType    TenantType `json:"tenant_type,omitempty"`
	Grants        []Grant    `json:"grants,omitempty"`
	IsSystemAdmin bool       `json:"is_system_admin,omitempty"`
	ClientID      string     `json:"-"` // azp (per request, not cached)
	SessionID     string     `json:"-"` // sid (per request, not cached)
	// Channel comes from the verified token on each request, never from the cache.
	Channel Channel `json:"-"`
}

// Principal is the authenticated actor of a request (BACKEND_DESIGN §6). Build it with NewPrincipal so
// the permission union matches the grants.
type Principal struct {
	PrincipalData
	perms PermissionSet
}

// NewPrincipal computes the permission union of the grants.
func NewPrincipal(d PrincipalData) Principal {
	ps := PermissionSet{}
	for _, g := range d.Grants {
		for _, p := range g.Permissions {
			ps[p] = struct{}{}
		}
	}
	return Principal{PrincipalData: d, perms: ps}
}

// Valid reports a principal built for a user.
func (p Principal) Valid() bool { return p.UserID != uuid.Nil && p.perms != nil }

// Permissions returns the union of permissions of every effective UserRole.
func (p Principal) Permissions() PermissionSet { return p.perms }

// Has reports whether any grant holds perm (ignoring school/subject limits; use Require for resources).
func (p Principal) Has(perm Permission) bool { return p.perms.Has(perm) }

// IsPlatform reports platform staff (no session tenant, staff).
func (p Principal) IsPlatform() bool { return p.TenantID == nil && p.UserType == UserTypeStaff }

// SubjectRef identifies the token subject; tokens carry no EduWorld user_id, so resolution starts from
// (issuer, subject).
type SubjectRef struct {
	Issuer  string
	Subject string
}

// ResolveRequest is the input of operation resolvePrincipal of the identity facade.
type ResolveRequest struct {
	Subject  SubjectRef
	TenantID *uuid.UUID // the X-Tenant-Id header; nil for platform staff and exempt routes
	ClientID string     // azp
}

// ErrNotMember means the user has no membership in the requested tenant (403 TENANT_MISMATCH).
var ErrNotMember = errors.New("authz: user is not a member of the tenant")

// PrincipalResolver resolves a Principal; implemented by an adapter over the identity facade and wrapped
// by the two-level cache. Errors other than ErrNotMember mean "unavailable" (fail closed, F002-AC-09).
type PrincipalResolver interface {
	Resolve(ctx context.Context, req ResolveRequest) (Principal, error)
}

type principalKey struct{}

// WithPrincipal stores p in ctx (middleware principal).
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom returns the principal of ctx.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok && p.Valid()
}

// ScopeFor returns the TenantScope of a tenant principal (exactly one tenant).
func ScopeFor(p Principal) (tenancy.TenantScope, error) {
	if !p.Valid() || p.TenantID == nil || *p.TenantID == uuid.Nil {
		return tenancy.TenantScope{}, errors.New("authz: principal has no session tenant")
	}
	return tenancy.NewTenantScopeForAuthz(*p.TenantID), nil
}

// CrossTenantScope returns an explicit multi-tenant scope for System Admin: it requires
// platform.cross_tenant.read, the portal client, no session tenant and a reason of the closed list
// (F002-AC-11, ADR-0027 §6). Data access records cross_tenant.accessed on use (tenancy.RecordCrossTenantScope).
func CrossTenantScope(p Principal, ids []uuid.UUID, reason tenancy.BypassReason) (tenancy.CrossTenantScope, error) {
	switch {
	case !p.Valid() || p.TenantID != nil:
		return tenancy.CrossTenantScope{}, errors.New("authz: cross-tenant scope needs a platform principal")
	case !p.Has(PermPlatformCrossTenantRead):
		return tenancy.CrossTenantScope{}, errors.New("authz: missing platform.cross_tenant.read")
	case p.Channel != ChannelPortal:
		return tenancy.CrossTenantScope{}, errors.New("authz: cross-tenant scope needs the portal client")
	case !reason.Valid():
		return tenancy.CrossTenantScope{}, fmt.Errorf("%w: %q", tenancy.ErrBypassReason, reason)
	case len(ids) == 0 || slices.Contains(ids, uuid.Nil):
		return tenancy.CrossTenantScope{}, errors.New("authz: cross-tenant scope needs tenant ids")
	}
	return tenancy.NewCrossTenantScopeForAuthz(ids, reason), nil
}

// PlatformScope returns the platform-only scope (tenant_id IS NULL rows) of platform staff.
func PlatformScope(p Principal) (tenancy.PlatformScope, error) {
	if !p.Valid() || !p.IsPlatform() {
		return tenancy.PlatformScope{}, errors.New("authz: platform scope needs platform staff")
	}
	return tenancy.NewPlatformScopeForAuthz("user:" + p.UserID.String()), nil
}

var jobNameRe = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}$`)

// SystemPlatformScope returns the platform-only scope of a platform job or system component.
func SystemPlatformScope(job string) (tenancy.PlatformScope, error) {
	if !jobNameRe.MatchString(job) {
		return tenancy.PlatformScope{}, fmt.Errorf("authz: invalid job name %q", job)
	}
	return tenancy.NewPlatformScopeForAuthz("job:" + job), nil
}

// EventScope returns the TenantScope of an event consumer or realtime command: the tenant comes from the
// event envelope (or command routing), never from user input (MULTITENANCY §4.1 item 7, F003-AC-20).
func EventScope(tenantID uuid.UUID) (tenancy.TenantScope, error) {
	if tenantID == uuid.Nil {
		return tenancy.TenantScope{}, errors.New("authz: event scope needs a tenant id")
	}
	return tenancy.NewTenantScopeForAuthz(tenantID), nil
}
