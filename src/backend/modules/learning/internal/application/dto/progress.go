package dto

import (
	"time"

	"github.com/google/uuid"
)

// LearningProgressView is LearningProgress of the public API (getMyProgress, getStudentProgress; F031 US3).
type LearningProgressView struct {
	StudentID  uuid.UUID               `json:"student_id"`
	SchoolID   uuid.UUID               `json:"-"` // school of the student (audit of getStudentProgress), never serialized
	Totals     ProgressTotalsView      `json:"totals"`
	Objectives []ObjectiveProgressView `json:"objectives"`
}

// ProgressTotalsView are the totals of a student (accuracy on the first scored answers; absent before any answer).
type ProgressTotalsView struct {
	LessonsCompleted int      `json:"lessons_completed"`
	ItemsAnswered    int      `json:"items_answered"`
	Accuracy         *float64 `json:"accuracy,omitempty"`
	ActiveMinutes    int      `json:"active_minutes"`
}

// ObjectiveProgressView is the level of one objective after decay (the stored value is never returned).
type ObjectiveProgressView struct {
	ObjectiveID    uuid.UUID  `json:"objective_id"`
	Level          string     `json:"level"`
	IconCode       string     `json:"icon_code,omitempty"` // <grade_band>.<level> (F032-AC-34)
	EvidenceCount  int        `json:"evidence_count"`
	LastEvidenceAt *time.Time `json:"last_evidence_at,omitempty"`
}
