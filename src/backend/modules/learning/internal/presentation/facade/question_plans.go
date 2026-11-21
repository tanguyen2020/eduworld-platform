package facade

import (
	"context"
	"time"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// PlanCommands are the write use cases of the question plans (command.QuestionPlans, F105).
type PlanCommands interface {
	CreatePlan(ctx context.Context, in command.CreatePlanInput) (command.PlanView, error)
	ScoreAnswer(ctx context.Context, in command.ScorePlanInput) (command.PlanScore, error)
}

// PlanReveals is the query of revealPlanItem (query.QuestionPlans, F105).
type PlanReveals interface {
	Reveal(ctx context.Context, planID uuid.UUID, index int, closedAt time.Time) (query.PlanReveal, error)
}

// needsKey checks the Idempotency-Key that x-context-required asks for; the operations are idempotent by their natural
// keys (match_id, (plan_id, index, student_id)), whatever key the caller sends.
func needsKey(ctx context.Context) error {
	if _, ok := modcall.IdempotencyKeyFrom(ctx); !ok {
		return problem.New(problem.IdempotencyKeyRequired, "")
	}
	return nil
}

// CreateQuestionPlan implements createQuestionPlan (x-allowed-callers: competition, F105-AC-01, AC-02): render payloads
// only; a second call for the same match returns the same plan.
func (f Facade) CreateQuestionPlan(ctx context.Context, req learningapi.CreateQuestionPlanRequestObject) (learningapi.CreateQuestionPlanResponseObject, error) {
	if req.Body == nil {
		return nil, problem.New(problem.BadRequest, "body is required")
	}
	if f.Plans == nil {
		return nil, problem.New(problem.Internal, "question plans are not wired")
	}
	if err := needsKey(ctx); err != nil {
		return nil, err
	}
	b := *req.Body
	in := command.CreatePlanInput{MatchID: b.MatchId, Mode: string(b.Mode), GradeLevel: b.GradeLevel, QuestionCount: b.QuestionCount,
		Participants: b.Participants, TimePerQuestionS: 20}
	if b.SubjectId == nil {
		return nil, problem.Invalid("/subject_id", problem.SubRequired, "module competition sends the subject id of the match")
	}
	in.SubjectID = *b.SubjectId
	if b.TopicIds != nil {
		in.TopicIDs = *b.TopicIds
	}
	if b.DifficultyMin != nil {
		in.DifficultyMin = *b.DifficultyMin
	}
	if b.DifficultyMax != nil {
		in.DifficultyMax = *b.DifficultyMax
	}
	if b.TimePerQuestionSeconds != nil {
		in.TimePerQuestionS = *b.TimePerQuestionSeconds
	}
	v, err := f.Plans.CreatePlan(ctx, in)
	if err != nil {
		return nil, err
	}
	out := learningapi.CreateQuestionPlan201JSONResponse{PlanId: v.PlanID, MatchId: v.MatchID, ExpiresAt: v.ExpiresAt, Items: []learningapi.PlanItem{}}
	if v.PlanVersion != "" {
		pv := v.PlanVersion
		out.PlanVersion = &pv
	}
	for _, it := range v.Items {
		out.Items = append(out.Items, learningapi.PlanItem{Index: it.Index, QuestionVersionId: it.QuestionVersionID,
			QuestionType: learningapi.QuestionType(it.QuestionType), RenderPayload: it.RenderPayload, TimeLimitSeconds: it.TimeLimitSeconds})
	}
	return out, nil
}

// ScorePlanAnswer implements scorePlanAnswer (x-allowed-callers: competition, F105-AC-04, AC-09).
func (f Facade) ScorePlanAnswer(ctx context.Context, req learningapi.ScorePlanAnswerRequestObject) (learningapi.ScorePlanAnswerResponseObject, error) {
	if req.Body == nil {
		return nil, problem.New(problem.BadRequest, "body is required")
	}
	if f.Plans == nil {
		return nil, problem.New(problem.Internal, "question plans are not wired")
	}
	if err := needsKey(ctx); err != nil {
		return nil, err
	}
	b := *req.Body
	resp, err := b.Response.MarshalJSON()
	if err != nil {
		return nil, problem.New(problem.InvalidResponse, "the response is not valid JSON")
	}
	in := command.ScorePlanInput{PlanID: req.PlanId, Index: b.Index, StudentID: b.StudentId, Response: resp, ReceivedAt: b.ReceivedAt}
	if b.ClientElapsedMs != nil {
		ms := int64(*b.ClientElapsedMs)
		in.ClientElapsedMS = &ms
	}
	s, err := f.Plans.ScoreAnswer(ctx, in)
	if err != nil {
		return nil, err
	}
	late := s.Late
	out := learningapi.ScorePlanAnswer200JSONResponse{PlanId: s.PlanID, Index: s.Index, StudentId: s.StudentID, IsCorrect: s.IsCorrect,
		ScoreFraction: float32(s.ScoreFraction), Late: &late}
	if s.AttemptAnswerID != uuid.Nil {
		id := s.AttemptAnswerID
		out.AttemptAnswerId = &id
	}
	return out, nil
}

// RevealPlanItem implements revealPlanItem (x-allowed-callers: competition, F105-AC-05): after the question closed only.
func (f Facade) RevealPlanItem(ctx context.Context, req learningapi.RevealPlanItemRequestObject) (learningapi.RevealPlanItemResponseObject, error) {
	if f.Reveals == nil {
		return nil, problem.New(problem.Internal, "question plans are not wired")
	}
	r, err := f.Reveals.Reveal(ctx, req.PlanId, req.Index, req.Params.ClosedAt)
	if err != nil {
		return nil, err
	}
	out := learningapi.RevealPlanItem200JSONResponse{PlanId: r.PlanID, Index: r.Index}
	if r.QuestionType != "" {
		qt := learningapi.QuestionType(r.QuestionType)
		out.QuestionType = &qt
	}
	if r.Question != nil {
		q := r.Question
		out.Question = &q
	}
	if len(r.CorrectResponse) > 0 {
		var cr learningapi.QuestionResponse
		if err := cr.UnmarshalJSON(r.CorrectResponse); err == nil {
			out.CorrectResponse = &cr
		}
	}
	if r.Explanation != "" {
		text := r.Explanation
		out.Explanation = &struct {
			Text *string `json:"text,omitempty"`
		}{Text: &text}
	}
	return out, nil
}
