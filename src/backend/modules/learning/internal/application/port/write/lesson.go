package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// LessonSession is a row of learning.lesson_sessions (F031 US6).
type LessonSession struct {
	ID                   uuid.UUID
	TenantID             uuid.UUID
	SchoolID             uuid.UUID
	StudentID            uuid.UUID
	LessonID             uuid.UUID
	LessonVersionID      uuid.UUID
	LessonVersion        int
	Status               string
	ContextMode          string
	AssignmentID         *uuid.UUID
	LearningPathID       *uuid.UUID
	RecommendationID     *uuid.UUID
	CompletedActivityIDs []uuid.UUID
	CompletedAttemptIDs  []uuid.UUID
	ActiveMS             int64
	StartedAt            time.Time
	EndedAt              *time.Time
	Version              int64
}

// LessonAttempt are the facts of a graded attempt the lesson completion needs.
type LessonAttempt struct {
	AttemptID   uuid.UUID
	StudentID   uuid.UUID
	ActivityID  *uuid.UUID
	ContextType string
	ContextID   uuid.UUID
	Status      string
	DurationMS  int64
}

// LessonStore persists lesson sessions in the TenantScope of ctx (RLS T).
type LessonStore interface {
	// OpenSession returns the in-progress session of a student on a lesson (nil when none).
	OpenSession(ctx context.Context, studentID, lessonID uuid.UUID) (*LessonSession, error)
	// LatestSession returns the open session, else the newest completed one (nil when none).
	LatestSession(ctx context.Context, studentID, lessonID uuid.UUID) (*LessonSession, error)
	// CompletedVersion reports whether the student completed this lesson version.
	CompletedVersion(ctx context.Context, studentID, lessonVersionID uuid.UUID) (bool, error)
	// Session returns a session by id (nil when invisible).
	Session(ctx context.Context, id uuid.UUID) (*LessonSession, error)
	// InsertSession inserts an open session; false when one appeared meanwhile.
	InsertSession(ctx context.Context, s LessonSession) (bool, error)
	// UpdateSession writes the mutable columns when the row version still is s.Version (bumped); false otherwise.
	UpdateSession(ctx context.Context, s LessonSession) (bool, error)
	// Attempt returns the facts of an attempt (nil when invisible).
	Attempt(ctx context.Context, attemptID uuid.UUID) (*LessonAttempt, error)
}
