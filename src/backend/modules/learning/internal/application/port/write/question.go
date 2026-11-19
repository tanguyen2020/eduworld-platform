package write

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/trash"
)

// QuestionStore persists questions and their versions (F022) with the scope of ctx: invisible rows are problem
// NotFound, a stale row version VersionMismatch, a change of a version that is not draft (trigger)
// ContentNotEditable, the second open draft of a question (partial unique index) Conflict.
type QuestionStore interface {
	InsertQuestion(ctx context.Context, q dto.Question) error
	Question(ctx context.Context, id uuid.UUID) (dto.Question, error)
	// UpdateQuestion saves latest_version_no and current_version_id when the row version is ifRow.
	UpdateQuestion(ctx context.Context, q dto.Question, ifRow int64) (dto.Question, error)
	SoftDeleteQuestion(ctx context.Context, id uuid.UUID, reason string) error

	// InsertQuestionVersion inserts a version and its objectives.
	InsertQuestionVersion(ctx context.Context, v dto.QuestionVersion) error
	QuestionVersion(ctx context.Context, questionID uuid.UUID, versionNo int) (dto.QuestionVersion, error)
	QuestionVersions(ctx context.Context, questionID uuid.UUID) ([]dto.QuestionVersion, error)
	// QuestionVersionByID returns a version by its id (references of question sets and blueprints, F024).
	QuestionVersionByID(ctx context.Context, id uuid.UUID) (dto.QuestionVersion, error)
	// UpdateQuestionVersion saves the content of a draft and replaces its objectives.
	UpdateQuestionVersion(ctx context.Context, v dto.QuestionVersion, ifRow int64) (dto.QuestionVersion, error)

	// DeletedQuestions and RestoreQuestion serve the trash (ctx carries tenancy.Bypass(trash)).
	DeletedQuestions(ctx context.Context, f port.TrashFilter) ([]trash.Entry, error)
	RestoreQuestion(ctx context.Context, id uuid.UUID) (trash.Entry, error)
}
