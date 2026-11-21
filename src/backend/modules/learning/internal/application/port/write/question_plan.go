package write

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// QuestionPlanItem is one pinned question of a match plan (assessment.question_plans.items): the version, its type, the
// presentation shown in the room (seed and shuffled orders), the time limit and the objectives — never an answer key.
type QuestionPlanItem struct {
	Index             int             `json:"index"`
	QuestionVersionID uuid.UUID       `json:"question_version_id"`
	QuestionType      string          `json:"question_type"`
	TimeLimitSeconds  int             `json:"time_limit_seconds"`
	MaxScore          float64         `json:"max_score"`
	Presentation      json.RawMessage `json:"presentation"`
	ObjectiveIDs      []uuid.UUID     `json:"objective_ids,omitempty"`
}

// QuestionPlanRow is one row of assessment.question_plans (T, F105).
type QuestionPlanRow struct {
	PlanID       uuid.UUID
	TenantID     uuid.UUID
	MatchID      uuid.UUID
	Mode         string
	GradeLevel   int
	SubjectID    uuid.UUID
	PlanVersion  string
	Seed         int64
	Items        []QuestionPlanItem
	Participants []uuid.UUID
	ExpiresAt    time.Time
	CreatedAt    time.Time
}

// QuestionPlanStore persists question plans in the tenant scope of ctx (RLS T, soft-delete filter of entx.Base).
type QuestionPlanStore interface {
	InsertPlan(ctx context.Context, p QuestionPlanRow) error
	// PlanByMatch returns the live plan of a match (nil when none).
	PlanByMatch(ctx context.Context, matchID uuid.UUID) (*QuestionPlanRow, error)
	// Plan returns a live plan (nil when invisible in the tenant of ctx).
	Plan(ctx context.Context, planID uuid.UUID) (*QuestionPlanRow, error)
}
