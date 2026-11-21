package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/hub"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// PositionReader reads the last valid position the room host stored for a member of a room (Valkey, cache.HubRegistry).
type PositionReader interface {
	Position(ctx context.Context, room string, user uuid.UUID) (*hub.Vec3, error)
}

// HubPositionResult is where the room host last saw a student.
type HubPositionResult struct {
	AreaID   string
	RoomID   string
	Position hub.Vec3
}

// HubPosition answers getHubPosition of the competition facade (F071 T049, x-allowed-callers: game): the area, room and
// last position of a student seated in a hub instance of the tenant of the scope; nil when the student is in no room of
// that tenant or the position expired. Read only (registry and positions of the room host, never the database).
type HubPosition struct {
	Rooms     port.Registry
	Positions PositionReader
}

// Get implements getHubPosition.
func (q HubPosition) Get(ctx context.Context, student uuid.UUID) (out *HubPositionResult, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.GetHubPosition")
	defer func() { done(err) }()
	s, ok := tenancy.ScopeFrom(ctx)
	tenant, single := uuid.Nil, false
	if ok {
		tenant, single = s.SessionTenantID()
	}
	if !single {
		return nil, problem.New(problem.TenantNotSelected, "getHubPosition needs the tenant scope of the caller")
	}
	ur, err := q.Rooms.UserRoom(ctx, student)
	if err != nil || ur == nil {
		return nil, err
	}
	meta, err := q.Rooms.Meta(ctx, ur.RoomID)
	if err != nil || meta == nil || meta.Partition.TenantID != tenant {
		return nil, err // another tenant's room is never revealed
	}
	pos, err := q.Positions.Position(ctx, ur.RoomID, student)
	if err != nil || pos == nil {
		return nil, err
	}
	return &HubPositionResult{AreaID: meta.AreaID, RoomID: ur.RoomID, Position: *pos}, nil
}
