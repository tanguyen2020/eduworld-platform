// Package read holds the read-context ports of module identity (ADR-0087): read models only read, in a
// READ ONLY session (replica allowed), never write, never publish events.
package read

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
)

// TenantFilter selects tenants for listTenants.
type TenantFilter struct {
	Q          string // code or name contains (case-insensitive)
	TenantType string
	After      *page.Cursor
	Limit      int
}

// TenantReadModel lists tenants (platform.tenants, G) ordered by code then id.
type TenantReadModel interface {
	// ListTenants returns up to f.Limit+1 rows so the caller knows whether more exist.
	ListTenants(ctx context.Context, f TenantFilter) ([]dto.Tenant, error)
}

// TenantOrg is a tenant the user belongs to and its Keycloak Organization (nil until created).
type TenantOrg struct {
	TenantID uuid.UUID
	OrgID    *uuid.UUID
}

// UserMembership is what Keycloak membership must mirror for one user (F010 US4).
type UserMembership struct {
	KeycloakUserID string // subject of the local-realm identity; "" when the user has no Keycloak account
	Tenants        []TenantOrg
}

// MembershipReader reads a user's memberships across tenants (platform read, bypass platform_job).
type MembershipReader interface {
	MembershipOf(ctx context.Context, userID uuid.UUID) (UserMembership, error)
}

// OrgMembershipReader lists, per tenant, the Organization and the Keycloak subjects that should be members
// (platform read for the reconciliation job, F010-AC-22).
type OrgMembershipReader interface {
	TenantOrgs(ctx context.Context) ([]TenantOrg, error)
	ExpectedMembers(ctx context.Context, tenantID uuid.UUID) (map[string]bool, error)
}

// SchoolFilter selects schools of the scope (the scope decides the visible tenants).
type SchoolFilter struct {
	TenantID     *uuid.UUID // restrict to one tenant (System Admin path); nil = every tenant of the scope
	Status       string
	ProvinceCode string
	Level        string
	Q            string
	After        *page.Cursor
	Limit        int
}

// ClassFilter selects classes of one school.
type ClassFilter struct {
	SchoolID       uuid.UUID
	AcademicYearID *uuid.UUID
	Grade          int
	After          *page.Cursor
	Limit          int // 0 = no paging (listClasses)
}

// OrgReadModel reads the catalog and the organization of the scope (keyset pages return Limit+1 rows).
type OrgReadModel interface {
	Countries(ctx context.Context) ([]dto.Country, error)
	Provinces(ctx context.Context, country, status string) ([]dto.Province, error)
	Schools(ctx context.Context, f SchoolFilter) ([]dto.School, error)
	School(ctx context.Context, id uuid.UUID) (dto.School, error)
	Classes(ctx context.Context, f ClassFilter) ([]dto.Class, error)
	UnassignedStudents(ctx context.Context, schoolID *uuid.UUID, after *page.Cursor, limit int) ([]dto.UnassignedStudent, error)
	StudentSummary(ctx context.Context, studentID uuid.UUID) (dto.StudentSummary, error)
}

// SnapshotEntityTypes are the entities of listProjectionSnapshot.
var SnapshotEntityTypes = map[string]bool{"tenant": true, "school": true, "class": true, "student": true, "enrollment": true}

// SnapshotItem is one ProjectionSnapshotItem: the fields of the projection event payload (no PII beyond
// them), the aggregate version and whether the source row is soft-deleted.
type SnapshotItem struct {
	EntityType    string
	EntityID      uuid.UUID
	TenantID      uuid.UUID
	SourceVersion int64
	Deleted       bool
	Data          map[string]any
}

// SnapshotReader pages the current state of one entity type ordered by id, soft-deleted rows included.
type SnapshotReader interface {
	Snapshot(ctx context.Context, entityType string, tenantID *uuid.UUID, afterID *uuid.UUID, limit int) ([]SnapshotItem, error)
}

// SisIssueFilter filters reconciliation issues of the scope, newest first (issue_id < After).
type SisIssueFilter struct {
	SchoolIDs    []uuid.UUID // nil = every school of the scope
	SchoolID     *uuid.UUID
	ConnectionID *uuid.UUID
	Type, Status string
	After        *page.Cursor
	Limit        int
}

// SisIssueReadModel reads sis.reconciliation_issues.
type SisIssueReadModel interface {
	Issues(ctx context.Context, f SisIssueFilter) ([]dto.SisIssue, error)
}

// SisImportReadModel reads sis.sis_imports and the record errors of their run.
type SisImportReadModel interface {
	// Import returns an import of the scope (problem NotFound otherwise).
	Import(ctx context.Context, id uuid.UUID) (dto.SisImport, error)
	// RunErrors returns up to limit record errors of a run, by row.
	RunErrors(ctx context.Context, runID uuid.UUID, limit int) ([]dto.SisRecordError, error)
}

// SisAdminReadModel reads connections, runs and run errors for platform staff (gateway nội bộ).
type SisAdminReadModel interface {
	Connections(ctx context.Context, providerKind, status string, after *page.Cursor, limit int) ([]dto.SisConnection, error)
	// ConnectionTenant / RunTenant resolve the tenant of a resource (System Admin bypass) to scope the read.
	ConnectionTenant(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	RunTenant(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	Runs(ctx context.Context, connectionID uuid.UUID, status string, after *page.Cursor, limit int) ([]dto.SyncRun, error)
	Run(ctx context.Context, id uuid.UUID) (dto.SyncRun, error)
	RunErrorPage(ctx context.Context, runID uuid.UUID, after *page.Cursor, limit int) ([]dto.SisRecordError, error)
}

// BootstrapReader builds the PlayerBootstrap of a student in the tenant of the scope (F013 T012): user,
// student, school (bilingual name, province), active class, tenant. Runs under the TenantScope of the
// chosen tenant; nothing from other modules (ADR-0074).
type BootstrapReader interface {
	Bootstrap(ctx context.Context, userID uuid.UUID) (dto.PlayerBootstrap, error)
}
