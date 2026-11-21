package dto

import (
	"time"

	"github.com/google/uuid"
)

// CurriculumSettingView is one item of SchoolCurriculumSettings (F021 FR-CUR-008).
type CurriculumSettingView struct {
	AcademicYearID      uuid.UUID `json:"academic_year_id"`
	GradeLevel          *int      `json:"grade_level,omitempty"`
	CurriculumVersionID uuid.UUID `json:"curriculum_version_id"`
	SetBy               uuid.UUID `json:"set_by"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// CurriculumSettingsView is SchoolCurriculumSettings.
type CurriculumSettingsView struct {
	SchoolID uuid.UUID               `json:"school_id"`
	Items    []CurriculumSettingView `json:"items"`
	Version  int64                   `json:"-"`
}
