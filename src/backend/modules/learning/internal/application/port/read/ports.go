// Package read holds the read-context ports of module learning (ADR-0087): read models only read, in a READ
// ONLY session (replica allowed), never write, never publish events.
package read

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/resourceassignment"
)

// Session runs fn in a READ ONLY transaction with the scope of ctx (entx.ReadSession).
type Session interface {
	Read(ctx context.Context, fn func(ctx context.Context) error) error
}

// AssignmentReadModel reads infra.tenant_resource_assignments.
type AssignmentReadModel interface {
	// ByResource returns up to limit+1 active assignments of one resource ordered by assignment id, after
	// the assignment id after (uuid.Nil = first page).
	ByResource(ctx context.Context, rt contentmodel.ResourceType, id uuid.UUID, after uuid.UUID, limit int) ([]resourceassignment.Assignment, error)
	// Assigned reports whether resource (rt, id) has an active assignment to tenant.
	Assigned(ctx context.Context, rt contentmodel.ResourceType, id, tenant uuid.UUID) (bool, error)
}
