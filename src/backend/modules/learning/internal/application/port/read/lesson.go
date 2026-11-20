package read

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// LessonVersionView is a released lesson version visible to the session tenant (getLesson).
type LessonVersionView struct {
	LessonID     uuid.UUID
	VersionID    uuid.UUID
	VersionNo    int
	Status       string
	Title        json.RawMessage
	Summary      json.RawMessage
	SubjectID    *uuid.UUID
	GradeLevels  []int
	ObjectiveIDs []uuid.UUID
	Body         json.RawMessage
}

// LessonSessionView is the latest session of a student on a lesson.
type LessonSessionView struct {
	LessonVersionID      uuid.UUID
	Status               string
	CompletedActivityIDs []uuid.UUID
	EndedAt              *time.Time
}

// ActivityInfo is the type and title of the released version of an activity.
type ActivityInfo struct {
	ActivityType string
	Title        json.RawMessage
}

// LessonReadModel reads lessons and lesson sessions in the scope of ctx (RLS; H content visible to the tenant).
type LessonReadModel interface {
	// Version returns versionID of the lesson, or its released version when versionID is nil (nil when invisible or
	// not released).
	Version(ctx context.Context, lessonID uuid.UUID, versionID *uuid.UUID) (*LessonVersionView, error)
	// Session returns the open session of the student, else the newest completed one (nil when none).
	Session(ctx context.Context, studentID, lessonID uuid.UUID) (*LessonSessionView, error)
	// Activities returns the released type and title of activities visible to the tenant.
	Activities(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]ActivityInfo, error)
}
