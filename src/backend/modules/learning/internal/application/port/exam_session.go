package port

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrExamSessionUnavailable is returned by an ExamSessionCreator that cannot create exam sessions in this binary.
var ErrExamSessionUnavailable = errors.New("exam sessions are not available")

// ExamSessionRequest is the exam session of a graded assignment on an assessment blueprint (SCHOOL_LMS §4.1 step 5).
type ExamSessionRequest struct {
	AssignmentID    uuid.UUID
	TenantID        uuid.UUID
	SchoolID        uuid.UUID
	SubjectID       uuid.UUID // subject of the assignment (the blueprint must be of this subject)
	BlueprintID     uuid.UUID
	BlueprintVerID  uuid.UUID
	OpenAt, CloseAt time.Time
	StudentIDs      []uuid.UUID
	FeedbackTiming  string
	ResultRelease   string
	Title           string
	MaxAttempts     int
	ScorePolicy     string
}

// ExamSessionCreator is owned by F041 (CODE_PLAN §4.1: F040 → F041 → F042): the assignment use cases create and cancel
// the exam session of a graded assignment on a blueprint through it. F041 wires the "unavailable" adapter (graded +
// blueprint → 422 VALIDATION_FAILED, F041-AC-09); F042 replaces it with the adapter of CreateExamSession/CancelExamSession
// (F042-AC-31).
type ExamSessionCreator interface {
	// Available reports whether exam sessions can be created.
	Available() bool
	Create(ctx context.Context, r ExamSessionRequest) (uuid.UUID, error)
	Cancel(ctx context.Context, examSessionID uuid.UUID, reason string) error
}
