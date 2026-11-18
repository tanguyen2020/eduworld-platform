// Package write holds the write-context ports of module identity (ADR-0087): repositories load and save
// aggregates inside entx.UnitOfWork; events and audit go to the outbox of the same transaction.
package write

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/school"
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

// SchoolRepository persists the School aggregate (org.schools, T). Writes need a scope covering the
// school's tenant (TenantScope, or CrossTenantScope of the target tenant for System Admin).
type SchoolRepository interface {
	// Create inserts the school with its tenant_id; a duplicate code or moet_code is 409 CONFLICT.
	Create(ctx context.Context, s *school.School) error
	// Get loads a live school of the scope with its province (404 outside the scope).
	Get(ctx context.Context, id uuid.UUID) (*school.School, error)
	// Save updates the mutable fields where version = s.Version (409 CONFLICT on a concurrent change).
	Save(ctx context.Context, s *school.School) error
	// CountInTenant counts the live schools of a tenant (tenant shape, DATABASE_DESIGN §2.2).
	CountInTenant(ctx context.Context, tenantID uuid.UUID) (int, error)
}

// CatalogRepository reads the platform catalog inside a write transaction.
type CatalogRepository interface {
	// ProvinceByCode returns the province with the code (problem NotFound when absent).
	ProvinceByCode(ctx context.Context, code string) (catalog.Province, error)
	// Successor returns the successor of a former province, nil when none.
	Successor(ctx context.Context, provinceID uuid.UUID) (*catalog.Province, error)
}
