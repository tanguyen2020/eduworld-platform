package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CourseView is Course (F040).
type CourseView struct {
	CourseID            uuid.UUID  `json:"course_id"`
	TenantID            uuid.UUID  `json:"tenant_id"`
	SchoolID            uuid.UUID  `json:"school_id"`
	ClassID             uuid.UUID  `json:"class_id"`
	ClassName           string     `json:"class_name,omitempty"`
	SubjectID           uuid.UUID  `json:"subject_id"`
	Grade               int        `json:"-"`
	AcademicYearID      uuid.UUID  `json:"academic_year_id"`
	CurriculumVersionID uuid.UUID  `json:"curriculum_version_id"`
	TextbookSeriesID    *uuid.UUID `json:"textbook_series_id,omitempty"`
	Status              string     `json:"status"`
	StudentCount        int        `json:"student_count"`
	Version             int64      `json:"version"` // If-Match of updateCourse from the list (F055 delta D1)
}

// ContentRefView is PathNode.content_ref.
type ContentRefView struct {
	ContentType      string    `json:"content_type"`
	ContentVersionID uuid.UUID `json:"content_version_id"`
}

// PathNodeView is PathNode.
type PathNodeView struct {
	NodeID       uuid.UUID       `json:"node_id"`
	Position     int             `json:"position"`
	ContentRef   ContentRefView  `json:"content_ref"`
	UnlockRule   json.RawMessage `json:"unlock_rule,omitempty"`
	ContentID    uuid.UUID       `json:"-"`
	Title        string          `json:"-"`
	ObjectiveIDs []uuid.UUID     `json:"-"`
}

// LearningPathView is LearningPath.
type LearningPathView struct {
	PathID           uuid.UUID      `json:"path_id"`
	CourseID         uuid.UUID      `json:"course_id"`
	Title            string         `json:"title"`
	UnlockMode       string         `json:"unlock_mode"`
	Nodes            []PathNodeView `json:"nodes"`
	IsTemplate       bool           `json:"is_template"`
	TextbookSeriesID *uuid.UUID     `json:"textbook_series_id,omitempty"`
	Version          int64          `json:"version"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// ScheduleItemView is ScheduleItem.
type ScheduleItemView struct {
	ScheduleItemID uuid.UUID  `json:"schedule_item_id"`
	CourseID       uuid.UUID  `json:"course_id"`
	SchoolID       uuid.UUID  `json:"-"`
	SubjectID      uuid.UUID  `json:"-"`
	Date           string     `json:"date"`
	Period         *int       `json:"period,omitempty"`
	Kind           string     `json:"kind"`
	RefType        *string    `json:"-"`
	RefID          *uuid.UUID `json:"ref_id,omitempty"`
	Note           *string    `json:"note,omitempty"`
	Version        int64      `json:"version"` // If-Match of updateScheduleItem from the list (F055 delta D1)
}

// StudyGroupView is StudyGroup.
type StudyGroupView struct {
	StudyGroupID uuid.UUID   `json:"study_group_id"`
	SchoolID     uuid.UUID   `json:"school_id"`
	Name         string      `json:"name"`
	ClassID      *uuid.UUID  `json:"class_id,omitempty"`
	GradeLevel   int         `json:"grade_level"`
	MemberIDs    []uuid.UUID `json:"member_ids"`
	MemberCount  int         `json:"member_count"`
	CreatedBy    uuid.UUID   `json:"created_by"`
	UpdatedAt    time.Time   `json:"updated_at"`
	Version      int64       `json:"-"`
}

// CurriculumSettingView is one item of SchoolCurriculumSettings.
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

// MyPathNodeView is one node of MyLearningPathList.
type MyPathNodeView struct {
	NodeID           uuid.UUID `json:"node_id"`
	Position         int       `json:"position"`
	ContentType      string    `json:"content_type"`
	ContentID        uuid.UUID `json:"content_id"`
	ContentVersionID uuid.UUID `json:"content_version_id"`
	Title            string    `json:"title,omitempty"`
	State            string    `json:"state"`
}

// MyLearningPathView is one item of MyLearningPathList.
type MyLearningPathView struct {
	CourseID   uuid.UUID        `json:"course_id"`
	SubjectID  uuid.UUID        `json:"subject_id"`
	PathID     uuid.UUID        `json:"path_id"`
	Title      string           `json:"title"`
	UnlockMode string           `json:"unlock_mode"`
	IsTemplate bool             `json:"is_template"`
	Nodes      []MyPathNodeView `json:"nodes"`
}

// MyLearningPathsView is MyLearningPathList.
type MyLearningPathsView struct {
	Items []MyLearningPathView `json:"items"`
}

// MyScheduleItemView is one item of MyScheduleList.
type MyScheduleItemView struct {
	ScheduleItemID uuid.UUID  `json:"schedule_item_id"`
	CourseID       uuid.UUID  `json:"course_id"`
	SubjectID      uuid.UUID  `json:"subject_id"`
	Date           string     `json:"date"`
	Period         *int       `json:"period,omitempty"`
	Kind           string     `json:"kind"`
	RefType        *string    `json:"ref_type,omitempty"`
	RefID          *uuid.UUID `json:"ref_id,omitempty"`
}

// MyScheduleView is MyScheduleList.
type MyScheduleView struct {
	Items []MyScheduleItemView `json:"items"`
}
