package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// RefTable names a tenant projection whose rows the consumer checks before writing (F020-AC-04).
type RefTable string

// Tenant projections with an owning tenant.
const (
	RefSchools  RefTable = "schools"
	RefStudents RefTable = "students"
)

// RefTenant is a row of ref.tenants. Nil fields keep the stored value on update.
type RefTenant struct {
	TenantID       uuid.UUID
	Code           *string
	TenantType     string
	Status         string
	IsQA           *bool
	SettingsSubset []byte // JSON object
	SourceVersion  int64
}

// RefProvince is a row of ref.provinces.
type RefProvince struct {
	Code          string
	CountryCode   string
	NameI18n      []byte // JSON object
	Status        string
	SourceVersion int64
}

// RefSchool is a row of ref.schools. Nil fields keep the stored value on update.
type RefSchool struct {
	SchoolID       uuid.UUID
	TenantID       uuid.UUID
	Name           *string
	NameI18n       []byte
	ProvinceCode   *string
	SchoolLevel    *string
	IsVirtual      *bool
	Status         *string
	SettingsSubset []byte
	SourceVersion  int64
}

// RefStudent is a row of ref.students. Nil fields keep the stored value on update.
type RefStudent struct {
	StudentID  uuid.UUID
	TenantID   uuid.UUID
	SchoolID   uuid.UUID
	GradeLevel *int
	Status     *string
	// AcademicYearID is the current academic year of the school of the student (student.placement_changed, ADR-0095);
	// nil keeps the stored value.
	AcademicYearID *uuid.UUID
	SourceVersion  int64
}

// ProjectionWriter writes the ref.* projections of module identity (DATABASE_DESIGN §2.17). Only the command
// UpsertProjection, called by the consumer learning.projection_identity, uses it (F020-AC-05). Every upsert
// applies the row only when its source_version is newer than the stored one (applied = false otherwise).
type ProjectionWriter interface {
	// OwnerTenant returns the tenant owning the projected row of table, whatever the session tenant (nil when
	// unknown).
	OwnerTenant(ctx context.Context, table RefTable, id uuid.UUID) (*uuid.UUID, error)
	UpsertTenant(ctx context.Context, r RefTenant) (bool, error)
	UpsertProvince(ctx context.Context, r RefProvince) (bool, error)
	UpsertSchool(ctx context.Context, r RefSchool) (bool, error)
	UpsertStudent(ctx context.Context, r RefStudent) (bool, error)
	// MarkStudentDeleted sets (at != nil) or clears (at == nil) deleted_at of a projected student, ordered by
	// the event time.
	MarkStudentDeleted(ctx context.Context, student, tenant uuid.UUID, deleted bool, at time.Time) (bool, error)
}
