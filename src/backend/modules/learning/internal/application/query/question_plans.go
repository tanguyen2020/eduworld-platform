package query

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/attemptview"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// QuestionPlans serves revealPlanItem (F105-AC-05, AC-23): the correct answer and the explanation of a question of a
// match plan once the question closed — module competition never stores an answer key (ADR-0016).
type QuestionPlans struct {
	Session  read.Session
	Model    read.QuestionPlanReadModel
	Delivery Delivery
	Clock    func() time.Time
	// Tolerance is the clock difference accepted between the room host and this process (closed_at a little ahead).
	Tolerance time.Duration
}

// PlanReveal is PlanItemReveal.
type PlanReveal struct {
	PlanID          uuid.UUID
	Index           int
	QuestionType    string
	CorrectResponse json.RawMessage
	Explanation     string
	// Question is the render payload of the question (stem and render data, no answer) for the match review.
	Question map[string]any
}

// Reveal implements revealPlanItem: 404 for a plan outside the tenant of the call or an unknown index, 409 when
// closedAt is still ahead of the server clock (the question is not closed).
func (q QuestionPlans) Reveal(ctx context.Context, planID uuid.UUID, index int, closedAt time.Time) (out PlanReveal, err error) {
	ctx, done := observability.UseCase(ctx, "learning.facade.RevealPlanItem")
	defer func() { done(err) }()
	now := time.Now().UTC()
	if q.Clock != nil {
		now = q.Clock().UTC()
	}
	tol := q.Tolerance
	if tol <= 0 {
		tol = 2 * time.Second
	}
	if closedAt.After(now.Add(tol)) {
		return out, problem.New(problem.Conflict, "the question is not closed")
	}
	var item *read.QuestionPlanItem
	var seed int64
	if err := q.Session.Read(ctx, func(ctx context.Context) error {
		p, err := q.Model.Plan(ctx, planID)
		if err != nil {
			return err
		}
		if p == nil || index < 0 || index >= len(p.Items) {
			return problem.New(problem.NotFound, "no such question in the plan")
		}
		item, seed = &p.Items[index], p.Seed
		return nil
	}); err != nil {
		return out, err
	}
	var c question.Content
	if err := q.Delivery.Session.Read(ctx, func(ctx context.Context) error {
		var err error
		c, err = q.Delivery.Reader.Grading(ctx, item.QuestionVersionID)
		return err
	}); err != nil {
		return out, err
	}
	exp, err := q.Delivery.Explanation(ctx, item.QuestionVersionID)
	if err != nil {
		return out, err
	}
	var render map[string]any
	if rendered, _, err := q.Delivery.Render(ctx, []uuid.UUID{item.QuestionVersionID}, uint64(seed)); err == nil && len(rendered) == 1 {
		render = map[string]any{"stem": rendered[0].Stem}
		for k, v := range rendered[0].Payload {
			render[k] = v
		}
	}
	return PlanReveal{PlanID: planID, Index: index, QuestionType: string(c.Type), CorrectResponse: question.CorrectResponse(c.Type, c.AnswerKey),
		Explanation: attemptview.Truncate(attemptview.RichText(exp, c.Language), 2000), Question: render}, nil
}
