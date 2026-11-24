// Package modules adapts the facades of other modules for module competition (ADR-0074, AGENTS R29): in-process calls
// through pkg/modcall (span, deadline, TenantScope and Principal of the caller), never internal HTTP.
package modules

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

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

// ScreenDeadline bounds the text screening of a challenge name (same budget as the school events of F077).
const ScreenDeadline = 800 * time.Millisecond

// ChatScreening is screenText of the chat facade (x-allowed-callers: game, competition; F077, F107).
type ChatScreening interface {
	ScreenText(ctx context.Context, req chatapi.ScreenTextRequestObject) (chatapi.ScreenTextResponseObject, error)
}

// TextScreener implements port.TextScreener on the chat facade: the positions of the flagged texts only (R15). F nil or
// any failure → port.ErrScreeningUnavailable: the challenge is not stored (503, never fail-open).
type TextScreener struct{ F ChatScreening }

var _ port.TextScreener = TextScreener{}

// Screen implements port.TextScreener.
func (s TextScreener) Screen(ctx context.Context, tenant uuid.UUID, locale string, texts []string) ([]int, error) {
	if s.F == nil {
		return nil, port.ErrScreeningUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, ScreenDeadline)
	defer cancel()
	op := modcall.Op{Caller: "competition", Callee: "chat", Operation: "screenText", Kind: modcall.Read,
		AllowedCallers: chatapi.AllowedCallers["screenText"], ContextRequired: chatapi.ContextRequired["screenText"]}
	loc := chatapi.Vi
	if locale == "en" {
		loc = chatapi.En
	}
	body := chatapi.ScreenTextJSONRequestBody{TenantId: tenant, Locale: loc, Texts: texts}
	resp, err := modcall.Call(ctx, op, func(ctx context.Context) (chatapi.ScreenTextResponseObject, error) {
		return s.F.ScreenText(ctx, chatapi.ScreenTextRequestObject{Body: &body})
	})
	if err != nil {
		return nil, errors.Join(port.ErrScreeningUnavailable, err)
	}
	v, ok := resp.(chatapi.ScreenText200JSONResponse)
	if !ok {
		return nil, fmt.Errorf("%w: unexpected response %T", port.ErrScreeningUnavailable, resp)
	}
	if !v.Allowed && len(v.Violations) == 0 {
		return []int{0}, nil
	}
	return v.Violations, nil
}

// LateScreener is the text screener of the challenges while module chat is wired after Register (Module.Wire): until a
// facade is set every check is port.ErrScreeningUnavailable.
type LateScreener struct {
	mu sync.RWMutex
	f  ChatScreening
}

var _ port.TextScreener = (*LateScreener)(nil)

// Set wires the chat facade (nil: none).
func (l *LateScreener) Set(f ChatScreening) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.f = f
}

// Screen implements port.TextScreener.
func (l *LateScreener) Screen(ctx context.Context, tenant uuid.UUID, locale string, texts []string) ([]int, error) {
	l.mu.RLock()
	f := l.f
	l.mu.RUnlock()
	return TextScreener{F: f}.Screen(ctx, tenant, locale, texts)
}
