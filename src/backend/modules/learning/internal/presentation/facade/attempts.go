package facade

import (
	"context"
	"time"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// AttemptOpener opens quest/mini-game attempts for module game (command.Attempts, F030 US5).
type AttemptOpener interface {
	CreateInternal(ctx context.Context, in command.CreateInternalInput) (command.InternalCreated, error)
}

// AttemptResults reads attempt results for module game (query.Attempts, F030 US5).
type AttemptResults interface {
	Result(ctx context.Context, id uuid.UUID) (query.AttemptResult, error)
}

// CreateAttempt implements createAttempt (x-allowed-callers: game, F030-AC-25): idempotent on the Idempotency-Key of
// the call; no route serves it (R29).
func (f Facade) CreateAttempt(ctx context.Context, req learningapi.CreateAttemptRequestObject) (learningapi.CreateAttemptResponseObject, error) {
	if req.Body == nil {
		return nil, problem.New(problem.BadRequest, "body is required")
	}
	if f.AttemptOpener == nil {
		return nil, problem.New(problem.Internal, "attempts are not wired")
	}
	b := *req.Body
	created, err := idempotent(ctx, f.Idempotency, "createAttempt", b, func(ctx context.Context) (command.InternalCreated, error) {
		return f.AttemptOpener.CreateInternal(ctx, command.CreateInternalInput{StudentID: b.StudentId, Mode: string(b.Mode),
			ContentType: string(b.Activity.ContentType), ContentID: b.Activity.ContentId, ContentVersionID: b.Activity.ContentVersionId,
			ContextType: string(b.Context.ContextType), ContextID: b.Context.ContextId})
	})
	if err != nil {
		return nil, err
	}
	started, err := time.Parse(time.RFC3339Nano, created.StartedAt)
	if err != nil {
		return nil, err
	}
	out := learningapi.CreateAttempt201JSONResponse{AttemptId: created.AttemptID, Status: learningapi.AttemptCreatedStatus(created.Status),
		QuestionCount: created.QuestionCount, StartedAt: started, ContentVersionId: created.ContentVersionID}
	if created.DeadlineAt != nil {
		d, err := time.Parse(time.RFC3339Nano, *created.DeadlineAt)
		if err != nil {
			return nil, err
		}
		out.DeadlineAt = &d
	}
	return out, nil
}

// GetAttemptResult implements getAttemptResult (x-allowed-callers: game, F030-AC-26): no answer data; an attempt of
// another tenant than the TenantScope of the call is 404.
func (f Facade) GetAttemptResult(ctx context.Context, req learningapi.GetAttemptResultRequestObject) (learningapi.GetAttemptResultResponseObject, error) {
	if f.AttemptResults == nil {
		return nil, problem.New(problem.Internal, "attempts are not wired")
	}
	r, err := f.AttemptResults.Result(ctx, req.AttemptId)
	if err != nil {
		return nil, err
	}
	out := learningapi.GetAttemptResult200JSONResponse{AttemptId: r.AttemptID, StudentId: r.StudentID, Status: learningapi.AttemptStatus(r.Status),
		QuestionCount: r.QuestionCount, CorrectCount: r.CorrectCount, SubmittedAt: r.SubmittedAt, GradedAt: r.GradedAt}
	cv := r.ContentVersionID
	out.ContentVersionId = &cv
	if r.Score != nil {
		s := float32(*r.Score)
		out.Score = &s
	}
	if r.MaxScore != nil {
		m := float32(*r.MaxScore)
		out.MaxScore = &m
	}
	objs := r.ObjectiveIDs
	if objs == nil {
		objs = []uuid.UUID{}
	}
	out.ObjectiveIds = &objs
	return out, nil
}
