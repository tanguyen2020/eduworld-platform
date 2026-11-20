package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// LessonCompletionView is LessonCompletion of the API (completeLesson, F031 US6).
type LessonCompletionView struct {
	LessonID    uuid.UUID `json:"lesson_id"`
	CompletedAt time.Time `json:"completed_at"`
}

// LessonActivityView is one activity of a lesson for the student.
type LessonActivityView struct {
	ActivityID   uuid.UUID       `json:"activity_id"`
	ActivityType string          `json:"activity_type"`
	Title        json.RawMessage `json:"title,omitempty"`
	Required     bool            `json:"required"`
	Completed    bool            `json:"completed"`
}

// LessonView is Lesson of the API (getLesson): the pinned version, no answer data.
type LessonView struct {
	LessonID         uuid.UUID            `json:"lesson_id"`
	Version          int                  `json:"version"`
	Title            json.RawMessage      `json:"title"`
	Summary          json.RawMessage      `json:"summary,omitempty"`
	SubjectID        *uuid.UUID           `json:"subject_id,omitempty"`
	GradeLevel       *int                 `json:"grade_level,omitempty"`
	ObjectiveIDs     []uuid.UUID          `json:"objective_ids"`
	EstimatedMinutes *int                 `json:"estimated_minutes,omitempty"`
	Activities       []LessonActivityView `json:"activities"`
	CompletionRule   string               `json:"completion_rule"`
	CompletedAt      *time.Time           `json:"completed_at"`
}
