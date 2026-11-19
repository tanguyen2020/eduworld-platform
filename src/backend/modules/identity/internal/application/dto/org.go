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

// Class is the Class schema.
type Class struct {
	ClassID        uuid.UUID  `json:"class_id"`
	SchoolID       uuid.UUID  `json:"school_id"`
	AcademicYearID *uuid.UUID `json:"academic_year_id,omitempty"`
	GradeLevel     int        `json:"grade_level"`
	Name           string     `json:"name"`
	StudentCount   *int       `json:"student_count,omitempty"`
}

// UnassignedStudent is the UnassignedStudent schema (F010: students without an active enrollment).
type UnassignedStudent struct {
	StudentID       uuid.UUID  `json:"student_id"`
	DisplayName     string     `json:"display_name"`
	Username        string     `json:"username,omitempty"`
	SchoolID        uuid.UUID  `json:"school_id"`
	GradeLevel      int        `json:"grade_level,omitempty"`
	Reason          string     `json:"reason"`
	SISConnectionID *uuid.UUID `json:"sis_connection_id"`
	DetectedAt      *time.Time `json:"detected_at,omitempty"`
}

// StudentSummary is the StudentSummary schema (getMyStudentProfile, getStudentProfile).
type StudentSummary struct {
	StudentID   uuid.UUID  `json:"student_id"`
	DisplayName string     `json:"display_name,omitempty"`
	SchoolID    uuid.UUID  `json:"school_id"`
	SchoolName  string     `json:"school_name,omitempty"`
	GradeLevel  int        `json:"grade_level"`
	ClassID     *uuid.UUID `json:"class_id,omitempty"`
	ClassName   string     `json:"class_name,omitempty"`
}
