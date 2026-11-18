// Package write holds the write-context ports of module identity (ADR-0087): repositories load and save
// aggregates inside entx.UnitOfWork; events and audit go to the outbox of the same transaction.
package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/enrollment"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/school"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/tenant"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
)

// UnitOfWork runs fn in one write transaction (entx.UnitOfWork).
type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

// Savepoints isolates one step of a transaction: its failure rolls back only that step (batch rows).
type Savepoints interface {
	Savepoint(ctx context.Context, fn func(ctx context.Context) error) error
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

// EnrollmentRepository persists enrollments (org.enrollments, T): history kept, at most one active.
type EnrollmentRepository interface {
	// LockActive returns the active enrollment of the student locked FOR UPDATE (404 when none), so
	// concurrent transfers of one student serialize (F010-AC-11).
	LockActive(ctx context.Context, studentID uuid.UUID) (*enrollment.Enrollment, error)
	// Close saves the closed enrollment (status, end_date) where version matches.
	Close(ctx context.Context, e *enrollment.Enrollment) error
	// Open inserts an active enrollment; a second active one violates enrollments_one_active_uq → 409.
	Open(ctx context.Context, e *enrollment.Enrollment) error
	// Placement returns the tenant and school of a live class (404 when unknown in the scope).
	Placement(ctx context.Context, classID uuid.UUID) (enrollment.Placement, error)
}

// StudentRepository moves students between schools/tenants (org.students and identity.users).
type StudentRepository interface {
	// Move sets the school (and tenant) of the student and the user's primary tenant/school; user_id is kept.
	Move(ctx context.Context, studentID uuid.UUID, to enrollment.Placement) error
}

// NewStudentAccount is a platform-created student of the common tenant (ACCOUNT_PROVISIONING §3.4).
type NewStudentAccount struct {
	UserID       uuid.UUID
	Username     string // normalized
	DisplayName  string
	TenantID     uuid.UUID
	SchoolID     uuid.UUID
	ClassID      uuid.UUID
	Grade        int
	Issuer       string // OIDC issuer of the local Keycloak realm
	Subject      string // Keycloak user id
	ExternalRef  string // optional, stored as issuer platform_common
	EnrollmentID uuid.UUID
}

// AccountRepository creates platform-provisioned accounts (identity.users, org.students,
// identity.external_identities, org.enrollments) in the current transaction.
type AccountRepository interface {
	// UsernameTaken reports whether lower(username) exists on any row, soft-deleted included (ADR-0069).
	UsernameTaken(ctx context.Context, username string) (bool, error)
	// ExternalRefTaken reports whether a live (issuer, subject) exists.
	ExternalRefTaken(ctx context.Context, issuer, subject string) (bool, error)
	// CreateStudent inserts the account; a duplicate username is problem USERNAME_TAKEN, a duplicate
	// identity problem IDENTITY_CONFLICT.
	CreateStudent(ctx context.Context, a NewStudentAccount) error
}

// CatalogWriter persists the platform catalog (catalog.*, G tables; System Admin only).
type CatalogWriter interface {
	CreateCountry(ctx context.Context, c *catalog.Country) error           // duplicate code → 409
	GetCountry(ctx context.Context, code string) (*catalog.Country, error) // 404
	SaveCountry(ctx context.Context, c *catalog.Country) error             // version check → 409
	CreateProvince(ctx context.Context, p *catalog.ProvinceRecord) error   // duplicate code → 409
	GetProvince(ctx context.Context, id uuid.UUID) (*catalog.ProvinceRecord, error)
	SaveProvince(ctx context.Context, p *catalog.ProvinceRecord) error
	CreateSuccession(ctx context.Context, s catalog.Succession) error // one successor per former unit → 409
}

// SisConnection is a SIS connection of a tenant (F011: provider csv_import only).
type SisConnection struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	ProviderKind string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// SisConnectionSchool is one school served by a connection.
type SisConnectionSchool struct {
	ConnectionID uuid.UUID
	SchoolID     uuid.UUID
	TenantID     uuid.UUID
	Active       bool
	AssignedAt   time.Time
}

// SisConnectionRepository persists SIS connections and their schools (sis.*, T).
type SisConnectionRepository interface {
	Create(ctx context.Context, c *SisConnection) error
	// TenantOf returns the tenant of a connection under a System Admin bypass (the route has no tenant).
	TenantOf(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) (*SisConnection, error)
	// SetSchools makes schoolIDs exactly the active schools of the connection; a school active on another
	// connection is problem SIS_SCHOOL_ALREADY_CONNECTED, a school outside the tenant 422.
	SetSchools(ctx context.Context, c *SisConnection, schoolIDs []uuid.UUID, by uuid.UUID) ([]SisConnectionSchool, error)
	Schools(ctx context.Context, connectionID uuid.UUID) ([]SisConnectionSchool, error)
}
