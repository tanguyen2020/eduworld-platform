// Package resourceassignment holds the Assignment aggregate (ADR-0029, MULTITENANCY §10): a platform resource
// of module learning made visible, read-only, to one tenant. Revoking soft-deletes the assignment.
package resourceassignment

import (
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// MaxTenants is maxItems of tenant_ids in TenantAssignmentApplyRequest.
const MaxTenants = 500

// ErrNotPlatform: only platform resources (owner_scope = platform) can be assigned.
var ErrNotPlatform = errors.New("resourceassignment: only platform resources can be assigned")

// Assignment is one (resource, tenant) pair.
type Assignment struct {
	ID           uuid.UUID
	ResourceType contentmodel.ResourceType
	ResourceID   uuid.UUID
	TenantID     uuid.UUID
	AssignedBy   uuid.UUID
	AssignedAt   time.Time
	RevokedAt    *time.Time
}

// Active reports an assignment that is not revoked.
func (a Assignment) Active() bool { return a.RevokedAt == nil }

// New assigns resource to tenant; the owner of the resource must be the platform.
func New(id uuid.UUID, rt contentmodel.ResourceType, resource uuid.UUID, owner contentmodel.Owner, tenant, by uuid.UUID, at time.Time) (Assignment, error) {
	if !owner.Platform() {
		return Assignment{}, ErrNotPlatform
	}
	if id == uuid.Nil || resource == uuid.Nil || tenant == uuid.Nil || by == uuid.Nil {
		return Assignment{}, contentmodel.ErrInvalid{Field: "tenant_ids", Reason: "ids must not be empty"}
	}
	return Assignment{ID: id, ResourceType: rt, ResourceID: resource, TenantID: tenant, AssignedBy: by, AssignedAt: at.UTC()}, nil
}

// Tenants validates the requested tenants: 1–500 distinct non-empty ids, kept in request order.
func Tenants(ids []uuid.UUID) ([]uuid.UUID, error) {
	if len(ids) == 0 {
		return nil, contentmodel.ErrInvalid{Field: "tenant_ids", Reason: "is required"}
	}
	if len(ids) > MaxTenants {
		return nil, contentmodel.ErrInvalid{Field: "tenant_ids", Reason: "has more than 500 tenants"}
	}
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			return nil, contentmodel.ErrInvalid{Field: "tenant_ids", Reason: "ids must not be empty"}
		}
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out, nil
}

// Missing returns the tenants of requested without an active assignment in existing (re-assigning an
// already assigned tenant creates nothing, F020-AC-15).
func Missing(requested []uuid.UUID, existing []Assignment) []uuid.UUID {
	var out []uuid.UUID
	for _, t := range requested {
		if !slices.ContainsFunc(existing, func(a Assignment) bool { return a.TenantID == t && a.Active() }) {
			out = append(out, t)
		}
	}
	return out
}
