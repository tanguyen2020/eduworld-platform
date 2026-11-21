// Package modules adapts the facades of other modules for module game (ADR-0074, AGENTS R29): in-process calls through
// pkg/modcall (span, deadline, TenantScope and Principal of the caller), never internal HTTP.
package modules

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"

	competitionapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/world"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// CompetitionHub is getHubPosition of the competition facade (x-allowed-callers: game, F071 T049).
type CompetitionHub interface {
	GetHubPosition(ctx context.Context, req competitionapi.GetHubPositionRequestObject) (competitionapi.GetHubPositionResponseObject, error)
}

// ErrHubUnavailable is a facade that cannot answer (not wired, deadline, error): the caller keeps the presence check only.
var ErrHubUnavailable = errors.New("game: hub position unavailable")

// HubPositions implements port.HubPositions on the competition facade (deadline modcall.Read: the request waits).
type HubPositions struct{ F CompetitionHub }

var _ port.HubPositions = HubPositions{}

// Position implements port.HubPositions: nil when the student is in no hub instance of the tenant (404).
func (h HubPositions) Position(ctx context.Context, _, student uuid.UUID) (*port.HubPosition, error) {
	if h.F == nil {
		return nil, ErrHubUnavailable
	}
	op := modcall.Op{Caller: "game", Callee: "competition", Operation: "getHubPosition", Kind: modcall.Read,
		AllowedCallers: competitionapi.AllowedCallers["getHubPosition"], ContextRequired: competitionapi.ContextRequired["getHubPosition"]}
	resp, err := modcall.Call(ctx, op, func(ctx context.Context) (competitionapi.GetHubPositionResponseObject, error) {
		return h.F.GetHubPosition(ctx, competitionapi.GetHubPositionRequestObject{StudentId: student})
	})
	var pe *problem.Error
	if errors.As(err, &pe) && pe.Code == problem.NotFound {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Join(ErrHubUnavailable, err)
	}
	v, ok := resp.(competitionapi.GetHubPosition200JSONResponse)
	if !ok || len(v.Position) != 3 {
		return nil, ErrHubUnavailable
	}
	return &port.HubPosition{AreaID: v.AreaId, RoomID: v.RoomId, Position: world.Vec3{v.Position[0], v.Position[1], v.Position[2]}}, nil
}

// LateHub is the HubPositions of the world use cases while module competition is wired after Register (Module.Wire):
// until a facade is set every lookup is ErrHubUnavailable (presence only).
type LateHub struct {
	mu sync.RWMutex
	f  CompetitionHub
}

var _ port.HubPositions = (*LateHub)(nil)

// Set wires the competition facade (nil: none).
func (l *LateHub) Set(f CompetitionHub) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.f = f
}

// Position implements port.HubPositions.
func (l *LateHub) Position(ctx context.Context, tenant, student uuid.UUID) (*port.HubPosition, error) {
	l.mu.RLock()
	f := l.f
	l.mu.RUnlock()
	return HubPositions{F: f}.Position(ctx, tenant, student)
}
