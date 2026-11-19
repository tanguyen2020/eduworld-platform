package read

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
)

// QuestionFilter selects questions of the CMS search (searchQuestions, F022-AC-13).
type QuestionFilter struct {
	SubjectID    *uuid.UUID
	GradeLevel   *int
	TopicID      *uuid.UUID
	ObjectiveID  *uuid.UUID
	Difficulties []int
	Status       string
	Type         string
	Source       string // own, assigned, platform ("" = every visible)
	Efficacy     *bool
}

// QuestionHit is a question with its latest version.
type QuestionHit struct {
	Question dto.Question
	Version  dto.QuestionVersion
}

// QuestionReadModel reads questions for the CMS with the scope of ctx.
type QuestionReadModel interface {
	Question(ctx context.Context, id uuid.UUID) (dto.Question, error)
	Version(ctx context.Context, questionID uuid.UUID, versionNo int) (dto.QuestionVersion, error)
	// Latest returns the newest version of a question.
	Latest(ctx context.Context, questionID uuid.UUID) (dto.QuestionVersion, error)
	Search(ctx context.Context, f QuestionFilter) ([]QuestionHit, error)
}

// PoolFilter narrows the pool read from the database (the rule is applied by selection.Select).
type PoolFilter struct {
	SubjectID *uuid.UUID
}

// DeliveryReader reads what a student may receive (F022-AC-24): the pool and the render data come from the view
// content.question_delivery_v (no answer column); only Grading and RenderKey read the answer key, server side.
type DeliveryReader interface {
	// Pool lists the current release of every visible question (efficacy instruments excluded).
	Pool(ctx context.Context, f PoolFilter) ([]selection.Candidate, error)
	// Deliverable returns the render data of published versions (no answer key, no feedback, no hints).
	Deliverable(ctx context.Context, versionIDs []uuid.UUID) ([]question.Content, error)
	// Grading returns a published version with its answer key and scoring rule (grader only).
	Grading(ctx context.Context, versionID uuid.UUID) (question.Content, error)
	// Explanation returns the explanation of a published version (sent after grading, FeedbackTiming of F030).
	Explanation(ctx context.Context, versionID uuid.UUID) (json.RawMessage, error)
}

// PoolCache caches the pool of a tenant (Valkey ew:learning:qb:pool:<tenant_id>:<filter_hash>, 5 minutes); a miss
// or an error falls back to the database (cache only, ADR-0004).
type PoolCache interface {
	Get(ctx context.Context, tenant uuid.UUID, key string) ([]selection.Candidate, bool)
	Put(ctx context.Context, tenant uuid.UUID, key string, pool []selection.Candidate)
}
