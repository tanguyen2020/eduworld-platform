package read

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
)

// ExamSessionFilter filters listExamSessions and listExamLibrary (F042): staff by the schools and subjects of their
// grants; a student by its participations (assigned sessions, or the open library).
type ExamSessionFilter struct {
	Scopes     []AssignmentScope
	StudentID  *uuid.UUID
	Library    bool
	OpenAt     *time.Time
	SubjectID  *uuid.UUID
	GradeLevel *int
	Type       string
	After      *uuid.UUID
	Limit      int
}

// ExamSessionRow, ExamPaperRow and ExamParticipationRow are the rows of the exam tables (the shapes of the write side).
type (
	ExamSessionRow       = write.ExamSessionRow
	ExamPaperRow         = write.ExamPaperRow
	ExamParticipationRow = write.ExamParticipationRow
)

// ExamReadModel reads the exam sessions (tenant of the session, deleted_at IS NULL; rows of the write side).
type ExamReadModel interface {
	Sessions(ctx context.Context, f ExamSessionFilter) ([]write.ExamSessionRow, error)
	Session(ctx context.Context, id uuid.UUID) (*write.ExamSessionRow, error)
	Papers(ctx context.Context, sessionID uuid.UUID) ([]write.ExamPaperRow, error)
	Participation(ctx context.Context, sessionID, studentID uuid.UUID) (*write.ExamParticipationRow, error)
	AttemptPaper(ctx context.Context, attemptID uuid.UUID) (*uuid.UUID, error)
	LatestAnswers(ctx context.Context, attemptID uuid.UUID) (map[int]json.RawMessage, error)
}
