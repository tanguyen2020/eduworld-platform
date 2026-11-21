package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// CurriculumSettingRow is one row of lms.school_curriculum_settings (F021 FR-CUR-008; moved from F040 by ADR-0095).
type CurriculumSettingRow struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
	SchoolID            uuid.UUID
	AcademicYearID      uuid.UUID
	Grade               *int
	CurriculumVersionID uuid.UUID
	SetBy               uuid.UUID
	UpdatedAt           time.Time
}

// VersionAccess is a published-or-not content (or curriculum) version as the tenant of the session sees it: Visible
// when the tenant owns it or it is platform content assigned to the tenant; Platform when it is platform content.
type VersionAccess struct {
	ItemID       uuid.UUID
	VersionID    uuid.UUID
	Status       string
	Platform     bool
	Visible      bool
	Title        string
	SubjectID    *uuid.UUID
	ObjectiveIDs []uuid.UUID
	TopicIDs     []uuid.UUID
}

// CurriculumSettingsStore persists the curriculum version a school uses per academic year and grade in the TenantScope of
// ctx (RLS T, soft-delete only) and reads the curriculum versions visible to the tenant.
type CurriculumSettingsStore interface {
	// CurriculumSettings returns the live settings of a school and the version of the settings (optimistic lock).
	CurriculumSettings(ctx context.Context, schoolID uuid.UUID) ([]CurriculumSettingRow, int64, error)
	// ReplaceCurriculumSettings soft-deletes the live settings of the years and inserts rows.
	ReplaceCurriculumSettings(ctx context.Context, schoolID uuid.UUID, years []uuid.UUID, rows []CurriculumSettingRow, by uuid.UUID) error
	// CurriculumVersion is a curriculum version visible in the scope of ctx (nil when absent).
	CurriculumVersion(ctx context.Context, versionID uuid.UUID) (*VersionAccess, error)
}
