package modules

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"

	chatapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	identityapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
)

// ChatNicknameModeration is moderateDisplayText of the chat facade (x-allowed-callers: competition, game; F136).
type ChatNicknameModeration interface {
	ModerateDisplayText(ctx context.Context, req chatapi.ModerateDisplayTextRequestObject) (chatapi.ModerateDisplayTextResponseObject, error)
}

// NicknameModeration implements port.NicknameModeration on moderateDisplayText with context nickname (pkg/modcall:
// span, deadline of the caller — nickname_moderation_deadline —, TenantScope and Principal). F nil (module chat not
// loaded) or any failure → port.ErrNicknameScreenUnavailable: the nickname is not stored (503, never fail-open —
// F136-AC-14). The response is the decision only (R15).
type NicknameModeration struct{ F ChatNicknameModeration }

var _ port.NicknameModeration = NicknameModeration{}

// CheckNickname implements port.NicknameModeration.
func (m NicknameModeration) CheckNickname(ctx context.Context, tenant, student uuid.UUID, text string) (bool, error) {
	if m.F == nil {
		return false, port.ErrNicknameScreenUnavailable
	}
	op := modcall.Op{Caller: "game", Callee: "chat", Operation: "moderateDisplayText", Kind: modcall.Read,
		AllowedCallers: chatapi.AllowedCallers["moderateDisplayText"], ContextRequired: chatapi.ContextRequired["moderateDisplayText"]}
	body := chatapi.DisplayTextCheckRequest{Text: text, Context: chatapi.Nickname, TenantId: tenant, AuthorId: student}
	resp, err := modcall.Call(ctx, op, func(ctx context.Context) (chatapi.ModerateDisplayTextResponseObject, error) {
		return m.F.ModerateDisplayText(ctx, chatapi.ModerateDisplayTextRequestObject{Body: &body})
	})
	if err != nil {
		return false, errors.Join(port.ErrNicknameScreenUnavailable, err)
	}
	v, ok := resp.(chatapi.ModerateDisplayText200JSONResponse)
	if !ok {
		return false, fmt.Errorf("%w: unexpected response %T", port.ErrNicknameScreenUnavailable, resp)
	}
	switch v.Decision {
	case chatapi.DisplayTextCheckResultDecisionAllowed:
		return true, nil
	case chatapi.DisplayTextCheckResultDecisionBlocked:
		return false, nil
	}
	return false, fmt.Errorf("%w: unknown decision %q", port.ErrNicknameScreenUnavailable, v.Decision)
}

// IdentityNicknameScreen is screenNicknamePersonalData of the identity facade (x-allowed-callers: game; F136).
type IdentityNicknameScreen interface {
	ScreenNicknamePersonalData(ctx context.Context, req identityapi.ScreenNicknamePersonalDataRequestObject) (identityapi.ScreenNicknamePersonalDataResponseObject, error)
}

// NicknamePersonalScreen implements port.NicknamePersonalDataScreen on screenNicknamePersonalData (pkg/modcall). F nil,
// a 404 (student not in the scope tenant) or any failure → port.ErrNicknameScreenUnavailable (fail-closed). Module game
// never receives the real name or the school name (R13, R16).
type NicknamePersonalScreen struct{ F IdentityNicknameScreen }

var _ port.NicknamePersonalDataScreen = NicknamePersonalScreen{}

// ScreenNickname implements port.NicknamePersonalDataScreen.
func (s NicknamePersonalScreen) ScreenNickname(ctx context.Context, _, student uuid.UUID, text string) (bool, error) {
	if s.F == nil {
		return false, port.ErrNicknameScreenUnavailable
	}
	op := modcall.Op{Caller: "game", Callee: "identity", Operation: "screenNicknamePersonalData", Kind: modcall.Read,
		AllowedCallers:  identityapi.AllowedCallers["screenNicknamePersonalData"],
		ContextRequired: identityapi.ContextRequired["screenNicknamePersonalData"]}
	body := identityapi.NicknameScreenRequest{Text: text}
	resp, err := modcall.Call(ctx, op, func(ctx context.Context) (identityapi.ScreenNicknamePersonalDataResponseObject, error) {
		return s.F.ScreenNicknamePersonalData(ctx, identityapi.ScreenNicknamePersonalDataRequestObject{StudentId: student, Body: &body})
	})
	if err != nil {
		return false, errors.Join(port.ErrNicknameScreenUnavailable, err)
	}
	v, ok := resp.(identityapi.ScreenNicknamePersonalData200JSONResponse)
	if !ok {
		return false, fmt.Errorf("%w: unexpected response %T", port.ErrNicknameScreenUnavailable, resp)
	}
	switch v.Decision {
	case identityapi.Allowed:
		return true, nil
	case identityapi.Blocked:
		return false, nil
	}
	return false, fmt.Errorf("%w: unknown decision %q", port.ErrNicknameScreenUnavailable, v.Decision)
}

// LateNicknameScreens are the nickname screens of the profile use cases while modules chat and identity are wired after
// Register (Module.Wire, Module.WireIdentity): until a facade is set every check is ErrNicknameScreenUnavailable.
type LateNicknameScreens struct {
	mu       sync.RWMutex
	chat     ChatNicknameModeration
	identity IdentityNicknameScreen
}

var (
	_ port.NicknameModeration         = (*LateNicknameScreens)(nil)
	_ port.NicknamePersonalDataScreen = (*LateNicknameScreens)(nil)
)

// SetChat wires the chat facade (nil: none).
func (l *LateNicknameScreens) SetChat(f ChatNicknameModeration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.chat = f
}

// SetIdentity wires the identity facade (nil: none).
func (l *LateNicknameScreens) SetIdentity(f IdentityNicknameScreen) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.identity = f
}

// CheckNickname implements port.NicknameModeration.
func (l *LateNicknameScreens) CheckNickname(ctx context.Context, tenant, student uuid.UUID, text string) (bool, error) {
	l.mu.RLock()
	f := l.chat
	l.mu.RUnlock()
	return NicknameModeration{F: f}.CheckNickname(ctx, tenant, student, text)
}

// ScreenNickname implements port.NicknamePersonalDataScreen.
func (l *LateNicknameScreens) ScreenNickname(ctx context.Context, tenant, student uuid.UUID, text string) (bool, error) {
	l.mu.RLock()
	f := l.identity
	l.mu.RUnlock()
	return NicknamePersonalScreen{F: f}.ScreenNickname(ctx, tenant, student, text)
}
