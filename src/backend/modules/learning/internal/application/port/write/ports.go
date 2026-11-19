// Package write holds the write-context ports of module learning (ADR-0087): repositories load and save inside
// entx.UnitOfWork; events and audit go to the outbox of the same transaction.
package write

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/resourceassignment"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
)

// UnitOfWork runs fn in one write transaction (entx.UnitOfWork).
type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
	// Savepoint isolates one step: its failure rolls back only that step (one element of a batch).
	Savepoint(ctx context.Context, fn func(ctx context.Context) error) error
}

// Event is a domain event written to the outbox. TenantID nil with Platform false takes the session tenant
// of the scope.
type Event struct {
	Type        string
	SubjectType string
	SubjectID   string
	TenantID    *uuid.UUID
	Platform    bool
	Payload     any
}

// EventPublisher writes events into the outbox of the transaction.
type EventPublisher interface {
	Publish(ctx context.Context, e Event) error
}

// Auditor records audit events in the transaction (audit.recorded via the outbox, DATABASE_DESIGN §1.3).
type Auditor interface {
	Record(ctx context.Context, e audit.Event) error
}

// AssignmentRepository persists assignments (infra.tenant_resource_assignments) under a scope covering the
// tenants (CrossTenantScope of System Admin).
type AssignmentRepository interface {
	// Active returns the active assignments of the resource for tenants.
	Active(ctx context.Context, rt contentmodel.ResourceType, id uuid.UUID, tenants []uuid.UUID) ([]resourceassignment.Assignment, error)
	// Create inserts an assignment (a concurrent duplicate is 409 CONFLICT).
	Create(ctx context.Context, a resourceassignment.Assignment) error
	// Revoke soft-deletes an active assignment and returns it with RevokedAt set.
	Revoke(ctx context.Context, a resourceassignment.Assignment, reason string) (resourceassignment.Assignment, error)
}

// CacheInvalidator drops the cached read models of a tenant after a commit (best effort, never business
// state: Valkey is only a cache, ADR-0004).
type CacheInvalidator interface {
	InvalidateTenant(ctx context.Context, tenant uuid.UUID) error
}
