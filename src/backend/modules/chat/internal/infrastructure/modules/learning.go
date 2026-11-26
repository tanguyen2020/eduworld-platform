// Package modules adapts the facades of other modules for module chat (ADR-0074, AGENTS R29): in-process calls through
// pkg/modcall (span, deadline, TenantScope and Principal of the caller, problem mapping), never internal HTTP, never the
// database of another module (R13).
package modules

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// LearningRecipients is listAssignmentRecipients of the learning facade (x-allowed-callers: chat, F123).
type LearningRecipients interface {
	ListAssignmentRecipients(ctx context.Context, req learningapi.ListAssignmentRecipientsRequestObject) (learningapi.ListAssignmentRecipientsResponseObject, error)
}

// ErrLearningNotWired: the binary runs the notifier without the learning facade (retryable: the consumer does not ack).
var ErrLearningNotWired = errors.New("chat: the learning facade is not wired")

// Learning implements port.AssignmentRecipients on the learning facade (deadline modcall.Read; no retry and no circuit
// breaker here: a failure is returned and the consumer retries, F123-AC-07).
type Learning struct{ F LearningRecipients }

var _ port.AssignmentRecipients = Learning{}

// Page implements port.AssignmentRecipients: 404 → port.ErrAssignmentNotFound; any other answer or failure is an error.
func (l Learning) Page(ctx context.Context, assignment uuid.UUID, cursor string) (port.AssignmentRecipientPage, error) {
	if l.F == nil {
		return port.AssignmentRecipientPage{}, ErrLearningNotWired
	}
	op := modcall.Op{Caller: "chat", Callee: "learning", Operation: "listAssignmentRecipients", Kind: modcall.Read,
		AllowedCallers: learningapi.AllowedCallers["listAssignmentRecipients"], ContextRequired: learningapi.ContextRequired["listAssignmentRecipients"]}
	req := learningapi.ListAssignmentRecipientsRequestObject{AssignmentId: assignment}
	if cursor != "" {
		c := cursor
		req.Params.Cursor = &c
	}
	resp, err := modcall.Call(ctx, op, func(ctx context.Context) (learningapi.ListAssignmentRecipientsResponseObject, error) {
		return l.F.ListAssignmentRecipients(ctx, req)
	})
	if err != nil {
		var pe *problem.Error
		if errors.As(err, &pe) && pe.Code == problem.NotFound {
			return port.AssignmentRecipientPage{}, port.ErrAssignmentNotFound
		}
		return port.AssignmentRecipientPage{}, fmt.Errorf("chat: listAssignmentRecipients: %w", err)
	}
	switch v := resp.(type) {
	case learningapi.ListAssignmentRecipients200JSONResponse:
		out := port.AssignmentRecipientPage{Title: v.Title, StudentIDs: v.StudentIds}
		if v.Page.HasMore && v.Page.NextCursor != nil {
			out.Next = *v.Page.NextCursor
		}
		return out, nil
	case learningapi.ListAssignmentRecipients404ApplicationProblemPlusJSONResponse:
		return port.AssignmentRecipientPage{}, port.ErrAssignmentNotFound
	}
	return port.AssignmentRecipientPage{}, fmt.Errorf("chat: listAssignmentRecipients: unexpected response %T", resp)
}

// LateLearning is the port.AssignmentRecipients of the consumers while module learning is wired after Register
// (Module.Wire): until a facade is set every call fails with ErrLearningNotWired (retried by the consumer).
type LateLearning struct {
	mu sync.RWMutex
	f  LearningRecipients
}

var _ port.AssignmentRecipients = (*LateLearning)(nil)

// Set wires the learning facade (nil: none).
func (l *LateLearning) Set(f LearningRecipients) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.f = f
}

// Page implements port.AssignmentRecipients.
func (l *LateLearning) Page(ctx context.Context, assignment uuid.UUID, cursor string) (port.AssignmentRecipientPage, error) {
	l.mu.RLock()
	f := l.f
	l.mu.RUnlock()
	return Learning{F: f}.Page(ctx, assignment, cursor)
}
