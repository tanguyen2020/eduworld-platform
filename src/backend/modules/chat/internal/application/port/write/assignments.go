package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Assignment is one row of infra.tenant_resource_assignments of eduworld_chat (a sticker pack a tenant sees, ADR-0029).
type Assignment struct {
	ID           uuid.UUID
	ResourceType string
	ResourceID   uuid.UUID
	TenantID     uuid.UUID
	AssignedBy   uuid.UUID
	AssignedAt   time.Time
	RevokedAt    *time.Time
}

// AssignmentStore writes the tenant assignments of the sticker packs (F082 T040; CrossTenantScope of the requested
// tenants, role chat_platform; soft delete only — R21).
type AssignmentStore interface {
	// PackPublished reports a live pack (exists) and whether it is published.
	PackPublished(ctx context.Context, id uuid.UUID) (exists, published bool, err error)
	ActiveTenants(ctx context.Context, tenants []uuid.UUID) ([]uuid.UUID, error)
	Active(ctx context.Context, resourceType string, id uuid.UUID, tenants []uuid.UUID) ([]Assignment, error)
	Create(ctx context.Context, a Assignment) error
	Revoke(ctx context.Context, a Assignment, by uuid.UUID, at time.Time) (Assignment, error)
	// Page lists the active assignments of a resource, keyset after tenant id.
	Page(ctx context.Context, resourceType string, id uuid.UUID, after *uuid.UUID, limit int) ([]Assignment, error)
}
