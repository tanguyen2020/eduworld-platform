// Package write holds the write-context ports of module identity (ADR-0087): repositories load and save
// aggregates inside entx.UnitOfWork; events and audit go to the outbox of the same transaction.
package write

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/tenant"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
)

// UnitOfWork runs fn in one write transaction (entx.UnitOfWork).
type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

// Event is a domain event written to the outbox. TenantID nil with Platform false takes the session tenant
// of the scope; every event gets its own idempotency key (several updates of one subject are distinct).
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

// TenantRepository persists the Tenant aggregate (platform.tenants, G).
type TenantRepository interface {
	// Create inserts the tenant; a duplicate code, alias or a second common tenant is 409 CONFLICT.
	Create(ctx context.Context, t *tenant.Tenant) error
	// Get loads a tenant (404 NOT_FOUND when absent or soft-deleted).
	Get(ctx context.Context, id uuid.UUID) (*tenant.Tenant, error)
	// Save updates name/status with optimistic concurrency on t.Version (412/409 on a stale version).
	Save(ctx context.Context, t *tenant.Tenant) error
	// SetOrgID records the Keycloak Organization id created by the worker.
	SetOrgID(ctx context.Context, id, orgID uuid.UUID) error
}
