package write

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	entqp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/questionplan"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
)

// QuestionPlanRepo implements write.QuestionPlanStore with ent (entx.Base: tenant + deleted_at IS NULL filter of the
// scope of ctx; RLS T on assessment.question_plans).
type QuestionPlanRepo struct{}

var _ write.QuestionPlanStore = QuestionPlanRepo{}

// InsertPlan implements write.QuestionPlanStore.
func (QuestionPlanRepo) InsertPlan(ctx context.Context, p write.QuestionPlanRow) error {
	items, err := json.Marshal(p.Items)
	if err != nil {
		return err
	}
	_, err = client(ctx).QuestionPlan.Create().SetID(p.PlanID).SetTenantID(p.TenantID).SetMatchID(p.MatchID).SetMode(p.Mode).
		SetGradeLevel(int16(p.GradeLevel)).SetSubjectID(p.SubjectID).SetPlanVersion(p.PlanVersion).SetSeed(p.Seed).SetItems(items).
		SetParticipants(p.Participants).SetExpiresAt(p.ExpiresAt).SetCreatedAt(p.CreatedAt).SetUpdatedAt(p.CreatedAt).Save(ctx)
	if err != nil {
		return fmt.Errorf("insert question plan: %w", MapCurriculumErr(err))
	}
	return nil
}

func planOf(r *ent.QuestionPlan) (*write.QuestionPlanRow, error) {
	p := &write.QuestionPlanRow{PlanID: r.ID, TenantID: r.TenantID, MatchID: r.MatchID, Mode: r.Mode, GradeLevel: int(r.GradeLevel),
		SubjectID: r.SubjectID, PlanVersion: r.PlanVersion, Seed: r.Seed, Participants: r.Participants, ExpiresAt: r.ExpiresAt.UTC(),
		CreatedAt: r.CreatedAt.UTC()}
	if err := json.Unmarshal(r.Items, &p.Items); err != nil {
		return nil, fmt.Errorf("question plan items: %w", err)
	}
	return p, nil
}

func onePlan(ctx context.Context, q *ent.QuestionPlanQuery) (*write.QuestionPlanRow, error) {
	r, err := q.Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return planOf(r)
}

// PlanByMatch implements write.QuestionPlanStore.
func (QuestionPlanRepo) PlanByMatch(ctx context.Context, matchID uuid.UUID) (*write.QuestionPlanRow, error) {
	return onePlan(ctx, client(ctx).QuestionPlan.Query().Where(entqp.MatchID(matchID)))
}

// Plan implements write.QuestionPlanStore.
func (QuestionPlanRepo) Plan(ctx context.Context, planID uuid.UUID) (*write.QuestionPlanRow, error) {
	return onePlan(ctx, client(ctx).QuestionPlan.Query().Where(entqp.ID(planID)))
}
