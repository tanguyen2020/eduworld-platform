package modules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/battle"
	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// LearningPlans is the part of the learning facade for the question plans of competition matches (x-allowed-callers:
// competition; implemented by F105): createQuestionPlan, scorePlanAnswer, revealPlanItem.
type LearningPlans interface {
	CreateQuestionPlan(ctx context.Context, req learningapi.CreateQuestionPlanRequestObject) (learningapi.CreateQuestionPlanResponseObject, error)
	ScorePlanAnswer(ctx context.Context, req learningapi.ScorePlanAnswerRequestObject) (learningapi.ScorePlanAnswerResponseObject, error)
	RevealPlanItem(ctx context.Context, req learningapi.RevealPlanItemRequestObject) (learningapi.RevealPlanItemResponseObject, error)
}

// LearningDeadlines are the deadlines of the calls (EW_BATTLE_PLAN_DEADLINE, EW_BATTLE_SCORE_DEADLINE): 0 = the
// module_call_deadline of the kind (2 s write, 800 ms read).
type LearningDeadlines struct {
	Plan, Score, Reveal time.Duration
}

// Learning implements port.QuestionPlans on the learning facade through pkg/modcall (span, deadline, TenantScope and
// Principal of the call; Idempotency-Key = match id or <plan>:<index>:<student>). No retry here: the room host decides
// (a failed grading replays the question). F nil → port.ErrPlanUnavailable.
type Learning struct {
	F         LearningPlans
	Deadlines LearningDeadlines
}

var _ port.QuestionPlans = Learning{}

func learningOp(name string, kind modcall.Kind, d time.Duration) modcall.Op {
	return modcall.Op{Caller: "competition", Callee: "learning", Operation: name, Kind: kind, Deadline: d,
		AllowedCallers: learningapi.AllowedCallers[name], ContextRequired: learningapi.ContextRequired[name]}
}

func planErr(err error) error {
	var pe *problem.Error
	if errors.As(err, &pe) && problem.Status(pe.Code) < 500 {
		return fmt.Errorf("%w: %w", port.ErrPlanRefused, err)
	}
	return fmt.Errorf("%w: %w", port.ErrPlanUnavailable, err)
}

// Create implements port.QuestionPlans.
func (l Learning) Create(ctx context.Context, req port.PlanRequest) (port.Plan, error) {
	if l.F == nil {
		return port.Plan{}, port.ErrPlanUnavailable
	}
	subject, count, t := req.SubjectID, req.Count, req.TimeS
	body := learningapi.CreateQuestionPlanJSONRequestBody{MatchId: req.MatchID, Mode: learningapi.QuestionPlanRequestMode(req.Mode),
		GradeLevel: req.Grade, SubjectId: &subject, QuestionCount: count, TimePerQuestionSeconds: &t, Participants: req.Participants}
	if req.TopicID != nil {
		topics := []uuid.UUID{*req.TopicID}
		body.TopicIds = &topics
	}
	ctx = modcall.WithIdempotencyKey(ctx, req.MatchID.String())
	resp, err := modcall.Call(ctx, learningOp("createQuestionPlan", modcall.Write, l.Deadlines.Plan),
		func(ctx context.Context) (learningapi.CreateQuestionPlanResponseObject, error) {
			return l.F.CreateQuestionPlan(ctx, learningapi.CreateQuestionPlanRequestObject{Body: &body})
		})
	if err != nil {
		return port.Plan{}, planErr(err)
	}
	v, ok := resp.(learningapi.CreateQuestionPlan201JSONResponse)
	if !ok {
		return port.Plan{}, fmt.Errorf("%w: unexpected response %T", port.ErrPlanUnavailable, resp)
	}
	out := port.Plan{ID: v.PlanId}
	if v.PlanVersion != nil {
		out.Version = *v.PlanVersion
	}
	for _, it := range v.Items {
		render, err := json.Marshal(it.RenderPayload)
		if err != nil {
			return port.Plan{}, err
		}
		out.Items = append(out.Items, port.PlanItem{Index: it.Index, QuestionVersionID: it.QuestionVersionId, Type: string(it.QuestionType),
			TimeS: it.TimeLimitSeconds, Render: render})
	}
	return out, nil
}

// Score implements port.QuestionPlans.
func (l Learning) Score(ctx context.Context, plan uuid.UUID, index int, student uuid.UUID, response json.RawMessage, at time.Time) (bool, error) {
	if l.F == nil {
		return false, port.ErrPlanUnavailable
	}
	var r learningapi.QuestionResponse
	if err := r.UnmarshalJSON(response); err != nil {
		return false, fmt.Errorf("%w: %w", port.ErrPlanRefused, err)
	}
	body := learningapi.ScorePlanAnswerJSONRequestBody{Index: index, StudentId: student, Response: r, ReceivedAt: at}
	ctx = modcall.WithIdempotencyKey(ctx, fmt.Sprintf("%s:%d:%s", plan, index, student))
	resp, err := modcall.Call(ctx, learningOp("scorePlanAnswer", modcall.Write, l.Deadlines.Score),
		func(ctx context.Context) (learningapi.ScorePlanAnswerResponseObject, error) {
			return l.F.ScorePlanAnswer(ctx, learningapi.ScorePlanAnswerRequestObject{PlanId: plan, Body: &body})
		})
	if err != nil {
		return false, planErr(err)
	}
	v, ok := resp.(learningapi.ScorePlanAnswer200JSONResponse)
	if !ok {
		return false, fmt.Errorf("%w: unexpected response %T", port.ErrPlanUnavailable, resp)
	}
	return v.IsCorrect && (v.Late == nil || !*v.Late), nil
}

// Reveal implements port.QuestionPlans.
func (l Learning) Reveal(ctx context.Context, plan uuid.UUID, index int, closedAt time.Time) (battle.Reveal, error) {
	if l.F == nil {
		return battle.Reveal{}, port.ErrPlanUnavailable
	}
	resp, err := modcall.Call(ctx, learningOp("revealPlanItem", modcall.Read, l.Deadlines.Reveal),
		func(ctx context.Context) (learningapi.RevealPlanItemResponseObject, error) {
			return l.F.RevealPlanItem(ctx, learningapi.RevealPlanItemRequestObject{PlanId: plan, Index: index,
				Params: learningapi.RevealPlanItemParams{ClosedAt: closedAt}})
		})
	if err != nil {
		return battle.Reveal{}, planErr(err)
	}
	v, ok := resp.(learningapi.RevealPlanItem200JSONResponse)
	if !ok {
		return battle.Reveal{}, fmt.Errorf("%w: unexpected response %T", port.ErrPlanUnavailable, resp)
	}
	out := battle.Reveal{}
	if v.CorrectResponse != nil {
		if b, err := v.CorrectResponse.MarshalJSON(); err == nil {
			out.CorrectResponse = b
		}
	}
	if v.Question != nil {
		if b, err := json.Marshal(*v.Question); err == nil {
			out.Question = b
		}
	}
	if v.Explanation != nil && v.Explanation.Text != nil {
		out.Explanation = *v.Explanation.Text
	}
	return out, nil
}

// LateLearning is the QuestionPlans of the battle engine while module learning is wired after Register (Module.Wire).
type LateLearning struct {
	mu sync.RWMutex
	f  LearningPlans
	d  LearningDeadlines
}

var _ port.QuestionPlans = (*LateLearning)(nil)

// Set wires the learning facade (nil: none).
func (l *LateLearning) Set(f LearningPlans, d LearningDeadlines) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.f, l.d = f, d
}

func (l *LateLearning) get() Learning {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return Learning{F: l.f, Deadlines: l.d}
}

// Create implements port.QuestionPlans.
func (l *LateLearning) Create(ctx context.Context, req port.PlanRequest) (port.Plan, error) {
	return l.get().Create(ctx, req)
}

// Score implements port.QuestionPlans.
func (l *LateLearning) Score(ctx context.Context, plan uuid.UUID, index int, student uuid.UUID, response json.RawMessage, at time.Time) (bool, error) {
	return l.get().Score(ctx, plan, index, student, response, at)
}

// Reveal implements port.QuestionPlans.
func (l *LateLearning) Reveal(ctx context.Context, plan uuid.UUID, index int, closedAt time.Time) (battle.Reveal, error) {
	return l.get().Reveal(ctx, plan, index, closedAt)
}
