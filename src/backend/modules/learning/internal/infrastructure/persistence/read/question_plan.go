package read

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// QuestionPlanModel implements read.QuestionPlanReadModel with explicit SQL on assessment.question_plans (RLS T).
type QuestionPlanModel struct{}

var _ read.QuestionPlanReadModel = QuestionPlanModel{}

// Plan implements read.QuestionPlanReadModel.
func (QuestionPlanModel) Plan(ctx context.Context, planID uuid.UUID) (*read.QuestionPlan, error) {
	tx, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	var items []byte
	var seed int64
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL in the condition
	err := tx.QueryRowContext(ctx, `SELECT items, seed FROM assessment.question_plans
		WHERE plan_id = $1 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, planID).Scan(&items, &seed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("question plan: %w", err)
	}
	p := &read.QuestionPlan{PlanID: planID, Seed: seed}
	if err := json.Unmarshal(items, &p.Items); err != nil {
		return nil, fmt.Errorf("question plan items: %w", err)
	}
	return p, nil
}
