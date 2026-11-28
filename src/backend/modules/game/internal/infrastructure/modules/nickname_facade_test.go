package modules

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	chatapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	identityapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Caller-side facade test of moderateDisplayText with context nickname (TEST DOUBLE chatapi.Fake, F136-AC-11, AC-14,
// AC-29): request mapping (normalised text, context nickname, tenant of the scope, author = student), caller identity
// game, the decision mapped; a failure, an unexpected answer or no facade → ErrNicknameScreenUnavailable (fail-closed).
func TestChatNicknameModerationFacade(t *testing.T) {
	tenant, student := uuid.New(), uuid.New()
	sc, _ := authz.EventScope(tenant)
	ctx := tenancy.WithScope(context.Background(), sc)
	var failure error
	decision := chatapi.DisplayTextCheckResultDecisionAllowed
	fake := &chatapi.Fake{ModerateDisplayTextFunc: func(ctx context.Context, r chatapi.ModerateDisplayTextRequestObject) (chatapi.ModerateDisplayTextResponseObject, error) {
		if modcall.CallerFrom(ctx) != "game" || r.Body.Context != chatapi.Nickname || r.Body.TenantId != tenant || r.Body.AuthorId != student ||
			r.Body.Text != "Siêu Nhân Sao Băng" {
			t.Fatalf("request: %s %+v", modcall.CallerFrom(ctx), r.Body)
		}
		if failure != nil {
			return nil, failure
		}
		return chatapi.ModerateDisplayText200JSONResponse{Decision: decision}, nil
	}}
	m := NicknameModeration{F: fake}
	if ok, err := m.CheckNickname(ctx, tenant, student, "Siêu Nhân Sao Băng"); err != nil || !ok {
		t.Fatalf("allowed: %v %v", ok, err)
	}
	decision = chatapi.DisplayTextCheckResultDecisionBlocked
	if ok, err := m.CheckNickname(ctx, tenant, student, "Siêu Nhân Sao Băng"); err != nil || ok {
		t.Fatalf("blocked: %v %v", ok, err)
	}
	decision = "maybe"
	if _, err := m.CheckNickname(ctx, tenant, student, "Siêu Nhân Sao Băng"); !errors.Is(err, port.ErrNicknameScreenUnavailable) {
		t.Fatalf("unknown decision: %v", err)
	}
	failure = errors.New("TEST DOUBLE: chat unavailable")
	if _, err := m.CheckNickname(ctx, tenant, student, "Siêu Nhân Sao Băng"); !errors.Is(err, port.ErrNicknameScreenUnavailable) {
		t.Fatalf("failure: %v", err)
	}
	late := &LateNicknameScreens{}
	if _, err := late.CheckNickname(ctx, tenant, student, "Siêu Nhân Sao Băng"); !errors.Is(err, port.ErrNicknameScreenUnavailable) {
		t.Fatalf("no facade: %v", err)
	}
	failure, decision = nil, chatapi.DisplayTextCheckResultDecisionAllowed
	late.SetChat(fake)
	if ok, err := late.CheckNickname(ctx, tenant, student, "Siêu Nhân Sao Băng"); err != nil || !ok {
		t.Fatalf("late wired: %v %v", ok, err)
	}
}

// Caller-side facade test of screenNicknamePersonalData (TEST DOUBLE identityapi.Fake, F136-AC-12, AC-14, AC-29):
// request mapping (student path, text), caller game, decision mapped; 404, a failure or no facade → unavailable.
func TestIdentityNicknameScreenFacade(t *testing.T) {
	tenant, student := uuid.New(), uuid.New()
	sc, _ := authz.EventScope(tenant)
	ctx := tenancy.WithScope(context.Background(), sc)
	var resp identityapi.ScreenNicknamePersonalDataResponseObject = identityapi.ScreenNicknamePersonalData200JSONResponse{Decision: identityapi.Allowed}
	var failure error
	fake := &identityapi.Fake{ScreenNicknamePersonalDataFunc: func(ctx context.Context, r identityapi.ScreenNicknamePersonalDataRequestObject) (identityapi.ScreenNicknamePersonalDataResponseObject, error) {
		if modcall.CallerFrom(ctx) != "game" || r.StudentId != student || r.Body == nil || r.Body.Text != "Siêu Nhân" {
			t.Fatalf("request: %s %+v", modcall.CallerFrom(ctx), r)
		}
		return resp, failure
	}}
	s := NicknamePersonalScreen{F: fake}
	if ok, err := s.ScreenNickname(ctx, tenant, student, "Siêu Nhân"); err != nil || !ok {
		t.Fatalf("allowed: %v %v", ok, err)
	}
	resp = identityapi.ScreenNicknamePersonalData200JSONResponse{Decision: identityapi.Blocked}
	if ok, err := s.ScreenNickname(ctx, tenant, student, "Siêu Nhân"); err != nil || ok {
		t.Fatalf("blocked: %v %v", ok, err)
	}
	resp = identityapi.ScreenNicknamePersonalData404ApplicationProblemPlusJSONResponse{}
	if _, err := s.ScreenNickname(ctx, tenant, student, "Siêu Nhân"); !errors.Is(err, port.ErrNicknameScreenUnavailable) {
		t.Fatalf("404: %v", err)
	}
	resp, failure = nil, errors.New("TEST DOUBLE: identity unavailable")
	if _, err := s.ScreenNickname(ctx, tenant, student, "Siêu Nhân"); !errors.Is(err, port.ErrNicknameScreenUnavailable) {
		t.Fatalf("failure: %v", err)
	}
	if _, err := (&LateNicknameScreens{}).ScreenNickname(ctx, tenant, student, "Siêu Nhân"); !errors.Is(err, port.ErrNicknameScreenUnavailable) {
		t.Fatalf("no facade: %v", err)
	}
}
