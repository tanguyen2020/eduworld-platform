package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AssignmentContentRefView is AssignmentCreate.content_ref.
type AssignmentContentRefView struct {
	ContentType     string     `json:"content_type"`
	ContentID       uuid.UUID  `json:"content_id"`
	PinnedVersionID *uuid.UUID `json:"pinned_version_id,omitempty"`
}

// AssignmentTargetView is AssignmentTarget (+ student_count of the snapshot).
type AssignmentTargetView struct {
	TargetType        string      `json:"target_type"`
	GradeLevel        *int        `json:"grade_level,omitempty"`
	IncludeStudentIDs []uuid.UUID `json:"include_student_ids,omitempty"`
	ExcludeStudentIDs []uuid.UUID `json:"exclude_student_ids,omitempty"`
	StudentIDs        []uuid.UUID `json:"student_ids,omitempty"`
	StudentCount      *int        `json:"student_count,omitempty"`
}

// LatePolicyView is AssignmentCreate.late_policy.
type LatePolicyView struct {
	AllowLate       bool `json:"allow_late"`
	LateWindowHours int  `json:"late_window_hours"`
}

// AssignmentCompletionView is Assignment.completion.
type AssignmentCompletionView struct {
	Assigned           int      `json:"assigned"`
	Submitted          int      `json:"submitted"`
	Missed             int      `json:"missed"`
	Excused            int      `json:"excused"`
	AverageScore       *float64 `json:"average_score"`
	RapidGuessStudents int      `json:"rapid_guess_students"`
}

// AssignmentView is the schema Assignment.
type AssignmentView struct {
	AssignmentID      uuid.UUID                 `json:"assignment_id"`
	TenantID          uuid.UUID                 `json:"tenant_id"`
	SchoolID          uuid.UUID                 `json:"school_id"`
	SubjectID         uuid.UUID                 `json:"subject_id"`
	Title             string                    `json:"title"`
	Instructions      *string                   `json:"instructions,omitempty"`
	ContentRef        AssignmentContentRefView  `json:"content_ref"`
	FollowLatest      bool                      `json:"follow_latest"`
	Mode              string                    `json:"mode"`
	Targets           []AssignmentTargetView    `json:"targets"`
	OpenAt            time.Time                 `json:"open_at"`
	DueAt             time.Time                 `json:"due_at"`
	LatePolicy        LatePolicyView            `json:"late_policy"`
	MaxAttempts       int                       `json:"max_attempts"`
	ScorePolicy       string                    `json:"score_policy"`
	FeedbackTiming    string                    `json:"feedback_timing"`
	ResultRelease     string                    `json:"result_release"`
	Status            string                    `json:"status"`
	CreatedBy         uuid.UUID                 `json:"created_by"`
	CreatedByRole     string                    `json:"created_by_role"`
	ExamSessionID     *uuid.UUID                `json:"exam_session_id,omitempty"`
	ResultsReleasedAt *time.Time                `json:"results_released_at,omitempty"`
	Completion        *AssignmentCompletionView `json:"completion,omitempty"`
	RowVersion        int64                     `json:"-"`
}

// AssignmentSubmissionView is the schema AssignmentSubmission.
type AssignmentSubmissionView struct {
	AssignmentID    uuid.UUID  `json:"assignment_id"`
	StudentID       uuid.UUID  `json:"student_id"`
	Status          string     `json:"status"`
	AttemptsUsed    int        `json:"attempts_used"`
	BestAttemptID   *uuid.UUID `json:"best_attempt_id,omitempty"`
	Score           *float64   `json:"score"`
	MaxScore        *float64   `json:"max_score"`
	RapidGuessCount int        `json:"rapid_guess_count"`
	DueAtOverride   *time.Time `json:"due_at_override,omitempty"`
	SubmittedAt     *time.Time `json:"submitted_at,omitempty"`
}

// MyAssignmentView is the schema MyAssignment.
type MyAssignmentView struct {
	AssignmentID  uuid.UUID       `json:"assignment_id"`
	Title         string          `json:"title"`
	SubjectID     uuid.UUID       `json:"subject_id"`
	SubjectName   json.RawMessage `json:"subject_name,omitempty"`
	ContentKind   string          `json:"content_kind"`
	ContentRef    uuid.UUID       `json:"content_ref"`
	OpenAt        time.Time       `json:"open_at"`
	DueAt         time.Time       `json:"due_at"`
	Status        string          `json:"status"`
	AttemptID     *uuid.UUID      `json:"attempt_id,omitempty"`
	AttemptsLeft  *int            `json:"attempts_left"`
	ExamSessionID *uuid.UUID      `json:"exam_session_id,omitempty"`
	ResultRelease string          `json:"result_release"`
	LatePolicy    LatePolicyView  `json:"late_policy"`
}

// GameActivityView is the schema GameActivityCatalogItem.
type GameActivityView struct {
	ContentType      string          `json:"content_type"`
	ContentID        uuid.UUID       `json:"content_id"`
	ContentVersionID uuid.UUID       `json:"content_version_id"`
	Title            string          `json:"title"`
	GradeLevels      []int           `json:"grade_levels"`
	ObjectiveIDs     []uuid.UUID     `json:"objective_ids"`
	TitleI18n        json.RawMessage `json:"-"`
}

// RemindView is the 202 response of remindAssignment.
type RemindView struct {
	Recipients int `json:"recipients"`
}
