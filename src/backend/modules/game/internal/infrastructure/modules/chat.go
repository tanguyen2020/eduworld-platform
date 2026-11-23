package modules

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	chatapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
)

// ScreenDeadline bounds the text screening of a school event (F077 plan: 800 ms from the request context).
const ScreenDeadline = 800 * time.Millisecond

// ChatScreening is screenText of the chat facade (x-allowed-callers: game; F077).
type ChatScreening interface {
	ScreenText(ctx context.Context, req chatapi.ScreenTextRequestObject) (chatapi.ScreenTextResponseObject, error)
}

// TextScreener implements port.TextScreener on the chat facade (pkg/modcall: span, deadline, TenantScope and Principal
// of the caller). F nil (module chat not loaded) or any failure → port.ErrScreeningUnavailable: the school event is not
// stored (503, never fail-open — F077-AC-23). The response carries the positions of the flagged texts only (R15).
type TextScreener struct{ F ChatScreening }

var _ port.TextScreener = TextScreener{}

// Screen implements port.TextScreener.
func (s TextScreener) Screen(ctx context.Context, tenant uuid.UUID, locale string, texts []string) ([]int, error) {
	if s.F == nil {
		return nil, port.ErrScreeningUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, ScreenDeadline)
	defer cancel()
	op := modcall.Op{Caller: "game", Callee: "chat", Operation: "screenText", Kind: modcall.Read,
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

// LateChat is the text screener of the school event use cases while module chat is wired after Register (Module.Wire):
// until a facade is set every check is port.ErrScreeningUnavailable.
type LateChat struct {
	mu sync.RWMutex
	f  ChatScreening
}

var _ port.TextScreener = (*LateChat)(nil)

// Set wires the chat facade (nil: none).
func (l *LateChat) Set(f ChatScreening) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.f = f
}

// Screen implements port.TextScreener.
func (l *LateChat) Screen(ctx context.Context, tenant uuid.UUID, locale string, texts []string) ([]int, error) {
	l.mu.RLock()
	f := l.f
	l.mu.RUnlock()
	return TextScreener{F: f}.Screen(ctx, tenant, locale, texts)
}
