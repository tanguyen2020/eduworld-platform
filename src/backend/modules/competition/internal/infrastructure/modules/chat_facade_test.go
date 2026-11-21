package modules

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	chatapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Caller-side facade test of moderateDisplayText against the generated fake (TEST DOUBLE chatapi.Fake, F116): request
// mapping (party_name, tenant, author), caller identity, allowed/blocked, any failure → ErrModerationUnavailable (the
// command keeps the default name, F100-AC-05).
func TestChatModerationFacade(t *testing.T) {
	tenant, author := uuid.New(), uuid.New()
	sc, _ := authz.EventScope(tenant)
	ctx := tenancy.WithScope(context.Background(), sc)
	decision := chatapi.DisplayTextCheckResultDecisionAllowed
	var failure error
	fake := &chatapi.Fake{ModerateDisplayTextFunc: func(ctx context.Context, r chatapi.ModerateDisplayTextRequestObject) (chatapi.ModerateDisplayTextResponseObject, error) {
		if modcall.CallerFrom(ctx) != "competition" || r.Body.Context != chatapi.PartyName || r.Body.TenantId != tenant ||
			r.Body.AuthorId != author || r.Body.Text != "Nhóm Sao Băng" {
			t.Fatalf("request: %s %+v", modcall.CallerFrom(ctx), r.Body)
		}
		if failure != nil {
			return nil, failure
		}
		return chatapi.ModerateDisplayText200JSONResponse{Decision: decision}, nil
	}}
	m := Moderator{F: fake}
	if ok, err := m.Check(ctx, tenant, author, "Nhóm Sao Băng"); !ok || err != nil {
		t.Fatalf("allowed: %v %v", ok, err)
	}
	decision = chatapi.DisplayTextCheckResultDecisionBlocked
	if ok, err := m.Check(ctx, tenant, author, "Nhóm Sao Băng"); ok || err != nil {
		t.Fatalf("blocked: %v %v", ok, err)
	}
	failure = errors.New("TEST DOUBLE: chat unavailable")
	if _, err := m.Check(ctx, tenant, author, "Nhóm Sao Băng"); !errors.Is(err, port.ErrModerationUnavailable) {
		t.Fatalf("failure: %v", err)
	}
	var late LateModerator
	if _, err := late.Check(ctx, tenant, author, "Nhóm Sao Băng"); !errors.Is(err, port.ErrModerationUnavailable) {
		t.Fatalf("no facade: %v", err)
	}
	failure = nil
	late.Set(fake)
	if ok, err := late.Check(ctx, tenant, author, "Nhóm Sao Băng"); ok || err != nil {
		t.Fatalf("late wired: %v %v", ok, err)
	}
}
