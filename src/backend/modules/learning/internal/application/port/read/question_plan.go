package read

import (
	"context"

	"github.com/google/uuid"
)

// QuestionPlanItem is one pinned question of a match plan as the read side needs it (F105 revealPlanItem).
type QuestionPlanItem struct {
	Index             int       `json:"index"`
	QuestionVersionID uuid.UUID `json:"question_version_id"`
	QuestionType      string    `json:"question_type"`
}

// QuestionPlan is a live plan of the tenant of the session (assessment.question_plans).
type QuestionPlan struct {
	PlanID uuid.UUID
	Seed   int64
	Items  []QuestionPlanItem
}

// QuestionPlanReadModel reads question plans in the read session (tenant scope of ctx).
type QuestionPlanReadModel interface {
	// Plan returns a live plan of the tenant (nil when invisible).
	Plan(ctx context.Context, planID uuid.UUID) (*QuestionPlan, error)
}
