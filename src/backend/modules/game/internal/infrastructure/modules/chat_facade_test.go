package modules

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	chatapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Caller-side facade test of screenText against the generated fake (TEST DOUBLE chatapi.Fake, F077-AC-23, T004): request
// mapping (tenant, locale, texts), caller identity game, flagged positions returned, any failure or no facade →
// ErrScreeningUnavailable (503, nothing stored — never fail-open).
func TestChatScreenTextFacade(t *testing.T) {
	tenant := uuid.New()
	sc, _ := authz.EventScope(tenant)
	ctx := tenancy.WithScope(context.Background(), sc)
	var failure error
	violations := []int{}
	fake := &chatapi.Fake{ScreenTextFunc: func(ctx context.Context, r chatapi.ScreenTextRequestObject) (chatapi.ScreenTextResponseObject, error) {
		if modcall.CallerFrom(ctx) != "game" || r.Body.TenantId != tenant || r.Body.Locale != chatapi.Vi || len(r.Body.Texts) != 2 {
			t.Fatalf("request: %s %+v", modcall.CallerFrom(ctx), r.Body)
		}
		if failure != nil {
			return nil, failure
		}
		return chatapi.ScreenText200JSONResponse{Allowed: len(violations) == 0, Violations: violations}, nil
	}}
	s := TextScreener{F: fake}
	if got, err := s.Screen(ctx, tenant, "vi", []string{"Tuần lễ Toán học", "Mô tả"}); err != nil || len(got) != 0 {
		t.Fatalf("clean: %v %v", got, err)
	}
	violations = []int{1}
	if got, err := s.Screen(ctx, tenant, "vi", []string{"Tuần lễ Toán học", "placeholder-flagged"}); err != nil || len(got) != 1 || got[0] != 1 {
		t.Fatalf("flagged: %v %v", got, err)
	}
	failure = errors.New("TEST DOUBLE: chat unavailable")
	if _, err := s.Screen(ctx, tenant, "vi", []string{"a", "b"}); !errors.Is(err, port.ErrScreeningUnavailable) {
		t.Fatalf("failure: %v", err)
	}
	var late LateChat
	if _, err := late.Screen(ctx, tenant, "vi", []string{"a", "b"}); !errors.Is(err, port.ErrScreeningUnavailable) {
		t.Fatalf("no facade: %v", err)
	}
}
