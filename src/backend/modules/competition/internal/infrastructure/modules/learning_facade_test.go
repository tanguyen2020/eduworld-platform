package modules

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// T011 (caller side) — facade test of createQuestionPlan, scorePlanAnswer and revealPlanItem against the generated fake
// (TEST DOUBLE learningapi.Fake, F105): request mapping (match, grade, subject, count, time, participants; plan, index,
// student, response, server time), caller identity and Idempotency-Key, render payload kept, correct answer and
// explanation read back; a 4xx is ErrPlanRefused, anything else ErrPlanUnavailable; no facade → unavailable.
func TestLearningPlansFacade(t *testing.T) {
	tenant, match, plan, an := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	sc, _ := authz.EventScope(tenant)
	ctx := tenancy.WithScope(context.Background(), sc)
	at := time.Date(2026, 11, 21, 8, 0, 5, 0, time.UTC)
	var failure error
	fake := &learningapi.Fake{
		CreateQuestionPlanFunc: func(ctx context.Context, r learningapi.CreateQuestionPlanRequestObject) (learningapi.CreateQuestionPlanResponseObject, error) {
			key, _ := modcall.IdempotencyKeyFrom(ctx)
			b := r.Body
			if modcall.CallerFrom(ctx) != "competition" || key != match.String() || b.MatchId != match || b.GradeLevel != 3 ||
				b.SubjectId == nil || b.QuestionCount != 7 || b.TimePerQuestionSeconds == nil || *b.TimePerQuestionSeconds != 30 ||
				len(b.Participants) != 2 || string(b.Mode) != "battle_1v1" {
				t.Fatalf("createQuestionPlan request: %s %+v", modcall.CallerFrom(ctx), b)
			}
			if failure != nil {
				return nil, failure
			}
			v := "qp1-abc"
			return learningapi.CreateQuestionPlan201JSONResponse{PlanId: plan, MatchId: match, PlanVersion: &v, ExpiresAt: at.Add(time.Hour),
				Items: []learningapi.PlanItem{{Index: 0, QuestionVersionId: uuid.New(), QuestionType: "single_choice", TimeLimitSeconds: 30,
					RenderPayload: map[string]any{"stem": map[string]any{"text": "7 x 8"}, "options": []any{map[string]any{"option_id": "o_a"}}}}}}, nil
		},
		ScorePlanAnswerFunc: func(ctx context.Context, r learningapi.ScorePlanAnswerRequestObject) (learningapi.ScorePlanAnswerResponseObject, error) {
			key, _ := modcall.IdempotencyKeyFrom(ctx)
			raw, _ := r.Body.Response.MarshalJSON()
			if r.PlanId != plan || r.Body.Index != 2 || r.Body.StudentId != an || !r.Body.ReceivedAt.Equal(at) || string(raw) != `{"option_id":"o_a"}` ||
				key == "" {
				t.Fatalf("scorePlanAnswer request: %+v %s", r.Body, raw)
			}
			if failure != nil {
				return nil, failure
			}
			late := false
			return learningapi.ScorePlanAnswer200JSONResponse{PlanId: plan, Index: 2, StudentId: an, IsCorrect: true, ScoreFraction: 1, Late: &late}, nil
		},
		RevealPlanItemFunc: func(ctx context.Context, r learningapi.RevealPlanItemRequestObject) (learningapi.RevealPlanItemResponseObject, error) {
			if r.PlanId != plan || r.Index != 2 || !r.Params.ClosedAt.Equal(at) {
				t.Fatalf("revealPlanItem request: %+v", r)
			}
			var cr learningapi.QuestionResponse
			_ = cr.UnmarshalJSON([]byte(`{"option_id":"o_a"}`))
			text := "Vì 7 x 8 = 56"
			return learningapi.RevealPlanItem200JSONResponse{PlanId: plan, Index: 2, CorrectResponse: &cr,
				Explanation: &struct {
					Text *string `json:"text,omitempty"`
				}{Text: &text}}, nil
		},
	}
	l := Learning{F: fake}
	p, err := l.Create(ctx, port.PlanRequest{MatchID: match, Mode: "battle_1v1", Grade: 3, SubjectID: uuid.New(), Count: 7, TimeS: 30,
		Participants: []uuid.UUID{an, uuid.New()}})
	if err != nil || p.ID != plan || p.Version != "qp1-abc" || len(p.Items) != 1 || p.Items[0].Type != "single_choice" {
		t.Fatalf("plan: %+v %v", p, err)
	}
	var render map[string]any
	if json.Unmarshal(p.Items[0].Render, &render) != nil || render["stem"] == nil {
		t.Fatalf("render payload: %s", p.Items[0].Render)
	}
	if ok, err := l.Score(ctx, plan, 2, an, json.RawMessage(`{"option_id":"o_a"}`), at); !ok || err != nil {
		t.Fatalf("score: %v %v", ok, err)
	}
	rv, err := l.Reveal(ctx, plan, 2, at)
	if err != nil || string(rv.CorrectResponse) != `{"option_id":"o_a"}` || rv.Explanation != "Vì 7 x 8 = 56" {
		t.Fatalf("reveal: %+v %v", rv, err)
	}
	failure = problem.New(problem.QuestionPoolInsufficient, "")
	if _, err := l.Create(ctx, port.PlanRequest{MatchID: match, Mode: "battle_1v1", Grade: 3, SubjectID: uuid.New(), Count: 7, TimeS: 30,
		Participants: []uuid.UUID{an, uuid.New()}}); !errors.Is(err, port.ErrPlanRefused) {
		t.Fatalf("4xx: %v", err)
	}
	failure = errors.New("TEST DOUBLE: learning down")
	if _, err := l.Score(ctx, plan, 2, an, json.RawMessage(`{"option_id":"o_a"}`), at); !errors.Is(err, port.ErrPlanUnavailable) {
		t.Fatalf("5xx: %v", err)
	}
	var late LateLearning
	if _, err := late.Reveal(ctx, plan, 2, at); !errors.Is(err, port.ErrPlanUnavailable) {
		t.Fatalf("no facade: %v", err)
	}
}
