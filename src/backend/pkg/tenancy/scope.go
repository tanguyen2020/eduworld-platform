// Package tenancy carries the data scope of a request or job (F002 T022, ADR-0005, ADR-0089 §3).
//
// Three scope kinds exist: TenantScope (one tenant: tenant users, event consumers, realtime commands),
// CrossTenantScope (explicit tenant list with a reason: System Admin) and PlatformScope (only rows with
// tenant_id IS NULL: platform staff and platform jobs). Scopes are created ONLY by package authz
// (authz.ScopeFor, authz.CrossTenantScope, authz.PlatformScope, authz.SystemPlatformScope); the
// *ForAuthz constructors below are reserved for it and the scopeguard analyzer rejects any other caller
// and any composite literal of these types. Zero values are invalid and must be rejected by data access.
//
// The public API of this package is frozen (ADR-0089 §3): changing it needs an ADR.
package tenancy

import (
	"context"

	"github.com/google/uuid"
)

// Scope is implemented by TenantScope, CrossTenantScope and PlatformScope only.
type Scope interface {
	// Valid reports whether the scope was built by authz (zero values are invalid).
	Valid() bool
	// TenantIDs returns the tenants rows may belong to (empty for PlatformScope).
	TenantIDs() []uuid.UUID
	// SessionTenantID is the tenant to put in app.tenant_id for RLS (only TenantScope).
	SessionTenantID() (uuid.UUID, bool)
	// PlatformOnly reports a scope limited to tenant_id IS NULL rows.
	PlatformOnly() bool
	sealed()
}

// TenantScope is exactly one tenant.
type TenantScope struct{ id uuid.UUID }

// Valid implements Scope.
func (s TenantScope) Valid() bool { return s.id != uuid.Nil }

// TenantIDs implements Scope.
func (s TenantScope) TenantIDs() []uuid.UUID {
	if !s.Valid() {
		return nil
	}
	return []uuid.UUID{s.id}
}

// SessionTenantID implements Scope.
func (s TenantScope) SessionTenantID() (uuid.UUID, bool) { return s.id, s.Valid() }

// PlatformOnly implements Scope.
func (TenantScope) PlatformOnly() bool { return false }
func (TenantScope) sealed()            {}

// CrossTenantScope is an explicit tenant list used by System Admin, always with a reason.
type CrossTenantScope struct {
	ids    []uuid.UUID
	reason BypassReason
}

// Valid implements Scope.
func (s CrossTenantScope) Valid() bool { return len(s.ids) > 0 && s.reason != "" }

// TenantIDs implements Scope.
func (s CrossTenantScope) TenantIDs() []uuid.UUID { return append([]uuid.UUID(nil), s.ids...) }

// SessionTenantID implements Scope (no single session tenant; queries filter tenant_id = ANY(ids)).
func (CrossTenantScope) SessionTenantID() (uuid.UUID, bool) { return uuid.Nil, false }

// PlatformOnly implements Scope.
func (CrossTenantScope) PlatformOnly() bool { return false }

// Reason returns the bypass reason that justified the scope.
func (s CrossTenantScope) Reason() BypassReason { return s.reason }
func (CrossTenantScope) sealed()                {}

// PlatformScope sees only rows with tenant_id IS NULL (platform content, platform roles).
type PlatformScope struct{ actor string }

// Valid implements Scope.
func (s PlatformScope) Valid() bool { return s.actor != "" }

// TenantIDs implements Scope.
func (PlatformScope) TenantIDs() []uuid.UUID { return nil }

// SessionTenantID implements Scope.
func (PlatformScope) SessionTenantID() (uuid.UUID, bool) { return uuid.Nil, false }

// PlatformOnly implements Scope.
func (s PlatformScope) PlatformOnly() bool { return s.Valid() }

// Actor is "user:<id>" for platform staff or "job:<name>"/"component:<name>" for system components.
func (s PlatformScope) Actor() string { return s.actor }
func (PlatformScope) sealed()         {}

// NewTenantScopeForAuthz is reserved for package authz (scopeguard analyzer).
func NewTenantScopeForAuthz(id uuid.UUID) TenantScope { return TenantScope{id: id} }

// NewCrossTenantScopeForAuthz is reserved for package authz (scopeguard analyzer).
func NewCrossTenantScopeForAuthz(ids []uuid.UUID, reason BypassReason) CrossTenantScope {
	return CrossTenantScope{ids: append([]uuid.UUID(nil), ids...), reason: reason}
}

// NewPlatformScopeForAuthz is reserved for package authz (scopeguard analyzer).
func NewPlatformScopeForAuthz(actor string) PlatformScope { return PlatformScope{actor: actor} }

type scopeKey struct{}

// WithScope stores the scope in ctx.
func WithScope(ctx context.Context, s Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, s)
}

// ScopeFrom returns the scope of ctx; ok is false when absent or invalid.
func ScopeFrom(ctx context.Context) (Scope, bool) {
	s, ok := ctx.Value(scopeKey{}).(Scope)
	if !ok || s == nil || !s.Valid() {
		return nil, false
	}
	return s, true
}

type moduleKey struct{}

// WithModule records which module is executing (metric label `module` of bypass and data access).
func WithModule(ctx context.Context, module string) context.Context {
	return context.WithValue(ctx, moduleKey{}, module)
}

// ModuleFrom returns the module of ctx or "unknown".
func ModuleFrom(ctx context.Context) string {
	if m, ok := ctx.Value(moduleKey{}).(string); ok && m != "" {
		return m
	}
	return "unknown"
}
