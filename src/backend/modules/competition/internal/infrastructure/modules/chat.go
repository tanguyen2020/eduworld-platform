// Package modules adapts the facades of other modules for module competition (ADR-0074, AGENTS R29): in-process calls
// through pkg/modcall (span, deadline, TenantScope and Principal of the caller), never internal HTTP.
package modules

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"

	chatapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
)

// ChatModeration is moderateDisplayText of the chat facade (x-allowed-callers: competition; implemented by F116).
type ChatModeration interface {
	ModerateDisplayText(ctx context.Context, req chatapi.ModerateDisplayTextRequestObject) (chatapi.ModerateDisplayTextResponseObject, error)
}

// Moderator implements port.NameModerator on the chat facade. F nil (module chat offers no moderation yet) or any
// failure → port.ErrModerationUnavailable: the command stores the generated default name (F100-AC-05, research R2).
type Moderator struct{ F ChatModeration }

var _ port.NameModerator = Moderator{}

// Check implements port.NameModerator (deadline modcall.Read: the request is waiting).
func (m Moderator) Check(ctx context.Context, tenant, author uuid.UUID, text string) (bool, error) {
	if m.F == nil {
		return false, port.ErrModerationUnavailable
	}
	op := modcall.Op{Caller: "competition", Callee: "chat", Operation: "moderateDisplayText", Kind: modcall.Read,
		AllowedCallers: chatapi.AllowedCallers["moderateDisplayText"], ContextRequired: chatapi.ContextRequired["moderateDisplayText"]}
	body := chatapi.ModerateDisplayTextJSONRequestBody{Text: text, Context: chatapi.PartyName, TenantId: tenant, AuthorId: author}
	resp, err := modcall.Call(ctx, op, func(ctx context.Context) (chatapi.ModerateDisplayTextResponseObject, error) {
		return m.F.ModerateDisplayText(ctx, chatapi.ModerateDisplayTextRequestObject{Body: &body})
	})
	if err != nil {
		return false, fmt.Errorf("%w: %w", port.ErrModerationUnavailable, err)
	}
	v, ok := resp.(chatapi.ModerateDisplayText200JSONResponse)
	if !ok {
		return false, fmt.Errorf("%w: unexpected response %T", port.ErrModerationUnavailable, resp)
	}
	return v.Decision == chatapi.DisplayTextCheckResultDecisionAllowed, nil
}

// LateModerator is the NameModerator of the commands while module chat is wired after Register (Module.Wire): until a
// facade is set every name falls back to the default name.
type LateModerator struct {
	mu sync.RWMutex
	f  ChatModeration
}

var _ port.NameModerator = (*LateModerator)(nil)

// Set wires the chat facade (nil: none).
func (l *LateModerator) Set(f ChatModeration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.f = f
}

// Check implements port.NameModerator.
func (l *LateModerator) Check(ctx context.Context, tenant, author uuid.UUID, text string) (bool, error) {
	l.mu.RLock()
	f := l.f
	l.mu.RUnlock()
	return Moderator{F: f}.Check(ctx, tenant, author, text)
}
