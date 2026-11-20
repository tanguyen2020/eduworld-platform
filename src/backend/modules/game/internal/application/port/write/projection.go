package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// RefTable names a tenant projection whose rows the consumer checks before writing.
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

// RefSchool is a row of ref.schools. Nil fields keep the stored value on update.
type RefSchool struct {
	SchoolID       uuid.UUID
	TenantID       uuid.UUID
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
	// SISConnectionID is the SIS connection of student.synced (nil keeps the stored value, OQ F014-1).
	SISConnectionID *uuid.UUID
	SourceVersion   int64
}

// ProjectionWriter writes the ref.* projections of module identity (DATABASE_DESIGN §2.17). Only the command
// UpsertProjection, called by the consumer game.projection_identity, uses it (rule 4). Every upsert applies the
// row only when its source_version is newer than the stored one (applied = false otherwise).
type ProjectionWriter interface {
	// OwnerTenant returns the tenant owning the projected row of table, whatever the session tenant (nil when unknown).
	OwnerTenant(ctx context.Context, table RefTable, id uuid.UUID) (*uuid.UUID, error)
	UpsertTenant(ctx context.Context, r RefTenant) (bool, error)
	UpsertSchool(ctx context.Context, r RefSchool) (bool, error)
	UpsertStudent(ctx context.Context, r RefStudent) (bool, error)
	// SetStudentStatus applies user.status_changed to a projected student of the transaction scope (tenant nil:
	// every tenant). It keeps source_version (the version of student.synced), so a later student.synced still applies.
	SetStudentStatus(ctx context.Context, student uuid.UUID, tenant *uuid.UUID, status string) (bool, error)
	// MarkStudentDeleted sets (deleted) or clears deleted_at of a projected student, ordered by the event time.
	MarkStudentDeleted(ctx context.Context, student, tenant uuid.UUID, deleted bool, at time.Time) (bool, error)
	// SetStudentSIS sets the SIS connection of a projected student of tenant (student.sis_link_created /
	// student.sis_link_superseded, OQ F014-1); false when the student is not projected yet (student.synced carries it).
	SetStudentSIS(ctx context.Context, student, tenant, connection uuid.UUID) (bool, error)
}

// KeyPlatformCurfew is the platform setting platform_curfew (CONFIG_DEFAULTS, ADR-0078; owner module identity, F015).
const KeyPlatformCurfew = "platform_curfew"

// RefPlatformSetting is a row of ref.platform_settings: the latest value of a platform setting of module identity
// (platform_settings.changed, scope platform). Value is a JSON object (platform_curfew: CurfewWindow).
type RefPlatformSetting struct {
	Key           string
	Value         []byte
	SourceVersion int64
}

// PlatformSettingWriter writes ref.platform_settings. Only the command PlatformSettings, called by the consumer
// game.platform_settings, uses it (rule 4); a row is applied only when its source_version is newer.
type PlatformSettingWriter interface {
	UpsertPlatformSetting(ctx context.Context, r RefPlatformSetting) (bool, error)
}
