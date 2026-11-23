package modules

import (
	"context"
	"errors"
	"sync"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// LearningFacade is createAttempt and getAttemptResult of the learning facade (x-allowed-callers: game, F030 US5).
type LearningFacade interface {
	CreateAttempt(ctx context.Context, req learningapi.CreateAttemptRequestObject) (learningapi.CreateAttemptResponseObject, error)
	GetAttemptResult(ctx context.Context, req learningapi.GetAttemptResultRequestObject) (learningapi.GetAttemptResultResponseObject, error)
	ValidateMinigameQuestionSource(ctx context.Context, req learningapi.ValidateMinigameQuestionSourceRequestObject) (learningapi.ValidateMinigameQuestionSourceResponseObject, error)
}

// LearningAttempts implements port.LearningAttempts on the learning facade (F072 T001): pkg/modcall gives the span, the
// deadline (modcall.Write), the TenantScope and the Principal of the student; the Idempotency-Key of the player
// request is forwarded so that a retried startQuestActivity reuses the same attempt. No retry and no circuit breaker
// in process (ADR-0074): a deadline or a 5xx of module learning is port.ErrLearningUnavailable (503 + Retry-After).
type LearningAttempts struct{ F LearningFacade }

var _ port.LearningAttempts = LearningAttempts{}

// CreateAttempt implements port.LearningAttempts.
func (l LearningAttempts) CreateAttempt(ctx context.Context, in port.LearningAttempt) (port.LearningAttemptCreated, error) {
	var out port.LearningAttemptCreated
	if l.F == nil {
		return out, port.ErrLearningUnavailable
	}
	if p, ok := idempotency.PendingFrom(ctx); ok {
		ctx = modcall.WithIdempotencyKey(ctx, p.Key.Key.String())
	}
	step := in.StepID
	qv := in.QuestVersionID
	content := learningapi.ActivityRefContentTypeActivity
	if in.ContentType == port.ContentMiniGameConfig {
		content = learningapi.ActivityRefContentTypeMinigameConfig
	}
	body := learningapi.AttemptCreateRequest{StudentId: in.StudentID, Mode: learningapi.AttemptModeQuest,
		Activity: learningapi.ActivityRef{ContentType: content, ContentId: in.ContentID, ContentVersionId: in.ContentVersionID}}
	if in.MiniGamePlayID != nil {
		// F073: a free mini-game play — mode minigame, context minigame_session of the play.
		body.Mode = learningapi.AttemptModeMinigame
		body.Context.ContextType = learningapi.MinigameSession
		body.Context.ContextId = *in.MiniGamePlayID
		body.Context.GameSessionId = in.GameSessionID
	} else {
		body.Context.ContextType = learningapi.QuestStep
		body.Context.ContextId = in.QuestProgressID
		body.Context.QuestVersionId, body.Context.StepId, body.Context.GameSessionId = &qv, &step, in.GameSessionID
	}
	op := modcall.Op{Caller: "game", Callee: "learning", Operation: "createAttempt", Kind: modcall.Write,
		AllowedCallers: learningapi.AllowedCallers["createAttempt"], ContextRequired: learningapi.ContextRequired["createAttempt"]}
	resp, err := modcall.Call(ctx, op, func(ctx context.Context) (learningapi.CreateAttemptResponseObject, error) {
		return l.F.CreateAttempt(ctx, learningapi.CreateAttemptRequestObject{Body: &body})
	})
	var pe *problem.Error
	switch {
	case errors.As(err, &pe) && pe.Code == problem.Conflict && in.ContentType == port.ContentMiniGameConfig:
		// F073 edge case: the projection of module learning has not received the pinned version yet → 503 Retry-After.
		return out, errors.Join(port.ErrLearningUnavailable, err)
	case errors.As(err, &pe) && (pe.Code == problem.NotFound || pe.Code == problem.ContentNotPublished):
		return out, problem.New(problem.ItemRefInvalid, "the learning content of the step is not published").Wrap(err)
	case errors.As(err, &pe) && problem.Status(pe.Code) < 500:
		return out, err
	case err != nil:
		return out, errors.Join(port.ErrLearningUnavailable, err)
	}
	v, ok := resp.(learningapi.CreateAttempt201JSONResponse)
	if !ok {
		return out, port.ErrLearningUnavailable
	}
	return port.LearningAttemptCreated{AttemptID: v.AttemptId, Status: string(v.Status), QuestionCount: v.QuestionCount, StartedAt: v.StartedAt.UTC(),
		DeadlineAt: v.DeadlineAt, ContentVersionID: v.ContentVersionId}, nil
}

// LateLearning is the learning facade of the quest use cases while module learning is wired after Register
// (Module.Wire): until a facade is set every call is port.ErrLearningUnavailable.
type LateLearning struct {
	mu sync.RWMutex
	f  LearningFacade
}

var _ port.LearningAttempts = (*LateLearning)(nil)

// Set wires the learning facade (nil: none).
func (l *LateLearning) Set(f LearningFacade) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.f = f
}

// CreateAttempt implements port.LearningAttempts.
func (l *LateLearning) CreateAttempt(ctx context.Context, in port.LearningAttempt) (port.LearningAttemptCreated, error) {
	l.mu.RLock()
	f := l.f
	l.mu.RUnlock()
	return LearningAttempts{F: f}.CreateAttempt(ctx, in)
}
