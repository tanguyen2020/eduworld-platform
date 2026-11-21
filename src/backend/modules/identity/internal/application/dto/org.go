package dto

import (
	"time"

	"github.com/google/uuid"
)

// Country is the Country schema.
type Country struct {
	CountryCode   string            `json:"country_code"`
	Name          map[string]string `json:"name"`
	DefaultLocale string            `json:"default_locale"`
	Status        string            `json:"status"`
	Version       int64             `json:"version,omitempty"` // If-Match of updateCountry (OQ F057-3)
}

// Province is the Province schema.
type Province struct {
	ProvinceID    uuid.UUID         `json:"province_id"`
	Code          string            `json:"code"`
	CountryCode   string            `json:"country_code"`
	OfficialCode  *string           `json:"official_code"`
	Name          map[string]string `json:"name"`
	ShortName     map[string]string `json:"short_name"`
	ProvinceType  string            `json:"province_type"`
	EffectiveFrom *string           `json:"effective_from"`
	EffectiveTo   *string           `json:"effective_to"`
	Status        string            `json:"status"`
	SuccessorCode *string           `json:"successor_code"`
	Version       int64             `json:"version,omitempty"` // If-Match of updateProvince (OQ F057-3)
}

// GradeStudent is the GradeStudent schema (listTenantGradeStudents, ADR-0095): the read-only row of a student of one
// grade of a school. Nothing beyond these fields leaves the module — no birth date, guardian contact, IP or device id
// (ADR-0031, R16).
type GradeStudent struct {
	StudentID          uuid.UUID  `json:"student_id"`
	DisplayName        string     `json:"display_name"`
	StudentCode        *string    `json:"student_code"`
	Username           string     `json:"username"`
	Status             string     `json:"status"`
	Source             string     `json:"source"`
	LastLoginAt        *time.Time `json:"last_login_at"`
	DaysSinceLastLogin *int       `json:"days_since_last_login"`
	GradeLevel         int        `json:"grade_level"`
}

// StudentSummary is the StudentSummary schema (getMyStudentProfile, getStudentProfile).
type StudentSummary struct {
	StudentID   uuid.UUID `json:"student_id"`
	DisplayName string    `json:"display_name,omitempty"`
	SchoolID    uuid.UUID `json:"school_id"`
	SchoolName  string    `json:"school_name,omitempty"`
	GradeLevel  int       `json:"grade_level"`
}

// SchoolAcademicYear is the SchoolAcademicYear schema (listSchoolAcademicYears, F010 T047).
type SchoolAcademicYear struct {
	AcademicYearID uuid.UUID      `json:"academic_year_id"`
	Name           string         `json:"name"`
	StartDate      string         `json:"start_date"`
	EndDate        string         `json:"end_date"`
	IsCurrent      bool           `json:"is_current"`
	Terms          []AcademicTerm `json:"terms"`
}

// AcademicTerm is the AcademicTerm schema (no term is stored yet: always empty, OQ F010-4).
type AcademicTerm struct {
	Name      string `json:"name"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}
