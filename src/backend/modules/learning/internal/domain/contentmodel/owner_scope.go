package contentmodel

import "github.com/google/uuid"

// OwnerScope says who owns a content item (CONTENT_MODEL §4, ADR-0005, ADR-0029).
type OwnerScope string

// Owner scopes: platform content has no tenant and reaches a tenant only through an assignment.
const (
	OwnerPlatform OwnerScope = "platform"
	OwnerTenant   OwnerScope = "tenant"
)

// ParseOwnerScope accepts platform or tenant.
func ParseOwnerScope(s string) (OwnerScope, error) {
	switch OwnerScope(s) {
	case OwnerPlatform, OwnerTenant:
		return OwnerScope(s), nil
	}
	return "", ErrInvalid{Field: "owner_scope", Reason: "must be platform or tenant"}
}

// Owner is the owner of an item or version: (owner_scope = 'platform') = (tenant_id IS NULL) (F020-AC-08,
// also a CHECK of every H table).
type Owner struct {
	Scope    OwnerScope
	TenantID *uuid.UUID
}

// NewOwner validates the pair.
func NewOwner(scope OwnerScope, tenantID *uuid.UUID) (Owner, error) {
	if _, err := ParseOwnerScope(string(scope)); err != nil {
		return Owner{}, err
	}
	if tenantID != nil && *tenantID == uuid.Nil {
		tenantID = nil
	}
	if (scope == OwnerPlatform) != (tenantID == nil) {
		if scope == OwnerPlatform {
			return Owner{}, ErrInvalid{Field: "tenant_id", Reason: "must be empty for platform content"}
		}
		return Owner{}, ErrInvalid{Field: "tenant_id", Reason: "is required for tenant content"}
	}
	o := Owner{Scope: scope}
	if tenantID != nil {
		t := *tenantID
		o.TenantID = &t
	}
	return o, nil
}

// Platform reports platform content.
func (o Owner) Platform() bool { return o.Scope == OwnerPlatform }

// WritableBy reports whether a tenant session may change the content: never platform content (assigned
// content is read-only for tenants, F020-AC-18), and only content of its own tenant.
func (o Owner) WritableBy(sessionTenant uuid.UUID) bool {
	return !o.Platform() && o.TenantID != nil && *o.TenantID == sessionTenant
}
