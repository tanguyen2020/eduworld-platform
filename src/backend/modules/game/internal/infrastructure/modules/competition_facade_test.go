package modules

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	competitionapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Caller-side facade test of getHubPosition (F071 T049) against the generated fake (TEST DOUBLE competitionapi.Fake):
// caller identity game, TenantScope passed, 200 mapped to a position, 404 = not in a hub instance (nil), any other
// failure or no facade = ErrHubUnavailable (the world use case keeps the presence check only).
func TestHubPositionsFacade(t *testing.T) {
	tenant, student := uuid.New(), uuid.New()
	sc, _ := authz.EventScope(tenant)
	ctx := tenancy.WithScope(context.Background(), sc)
	var failure error
	fake := &competitionapi.Fake{GetHubPositionFunc: func(ctx context.Context, r competitionapi.GetHubPositionRequestObject) (competitionapi.GetHubPositionResponseObject, error) {
		s, _ := tenancy.ScopeFrom(ctx)
		got, _ := s.SessionTenantID()
		if modcall.CallerFrom(ctx) != "game" || r.StudentId != student || got != tenant {
			t.Fatalf("request: %s %v %v", modcall.CallerFrom(ctx), r.StudentId, got)
		}
		if failure != nil {
			return nil, failure
		}
		return competitionapi.GetHubPosition200JSONResponse{StudentId: student, AreaId: "main_hub", RoomId: "r1", Position: []float64{1.5, 0, -4}}, nil
	}}
	h := HubPositions{F: fake}
	p, err := h.Position(ctx, tenant, student)
	if err != nil || p == nil || p.AreaID != "main_hub" || p.RoomID != "r1" || p.Position[2] != -4 {
		t.Fatalf("position: %+v %v", p, err)
	}
	failure = problem.New(problem.NotFound, "")
	if p, err := h.Position(ctx, tenant, student); p != nil || err != nil {
		t.Fatalf("not in a hub: %+v %v", p, err)
	}
	failure = errors.New("valkey down")
	if _, err := h.Position(ctx, tenant, student); !errors.Is(err, ErrHubUnavailable) {
		t.Fatalf("failure: %v", err)
	}
	if _, err := (&LateHub{}).Position(ctx, tenant, student); !errors.Is(err, ErrHubUnavailable) {
		t.Fatalf("not wired: %v", err)
	}
	late := &LateHub{}
	failure = nil
	late.Set(fake)
	if p, err := late.Position(ctx, tenant, student); err != nil || p == nil {
		t.Fatalf("wired: %+v %v", p, err)
	}
}
