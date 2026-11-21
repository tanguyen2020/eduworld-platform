package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/hub"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
)

// joinPartyRoom seats a participant of a running party session in the private_instance of its party (F101-AC-11,
// AC-17): hub.instance.join of the area of the session leads there — first entry, reconnection within
// disconnect_grace, or another device after a takeover — always the same room_id. The room host opens the room on
// first use with the participants as the only allowed seats. ok false: the student has no party room in that area.
func (h Hub) joinPartyRoom(ctx context.Context, j *joinCtx, seat hub.Seat, current *port.UserRoom, start time.Time) (*Assigned, bool, error) {
	if h.PartyRooms == nil {
		return nil, false, nil
	}
	sc, err := authz.EventScope(j.c.TenantID)
	if err != nil {
		return nil, false, fail("UNAUTHENTICATED", "tenant")
	}
	var pr *write.PartyRoom
	if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		var err error
		pr, err = h.PartyRooms.RoomOf(ctx, j.c.TenantID, j.c.UserID, j.area.ID)
		return err
	}); err != nil {
		return nil, true, fmt.Errorf("hub: party room: %w", err)
	}
	if pr == nil {
		return nil, false, nil
	}
	room := pr.RoomID.String()
	req := port.JoinRequest{Seat: seat, Spawn: j.in.SpawnPoint}
	live, err := h.Registry.Meta(ctx, room)
	if err != nil {
		return nil, true, fmt.Errorf("hub: party room meta: %w", err)
	}
	if live == nil { // first entry (or the registry lost it): the room host opens it here
		meta := port.RoomMeta{RoomID: room, AreaID: pr.AreaID, Capacity: pr.Capacity, Private: true, Allowed: pr.Members,
			Partition: hub.Partition{TenantID: j.c.TenantID, SchoolID: pr.SchoolID, GradeBand: pr.GradeBand}}
		if err := h.Rooms.Open(ctx, meta); err != nil && !errors.Is(err, ErrRoomGone) {
			return nil, true, fmt.Errorf("hub: open party room: %w", err)
		}
	}
	res, err := h.Rooms.Join(ctx, room, req)
	switch {
	case errors.Is(err, hub.ErrNotMember), errors.Is(err, ErrRoomGone):
		return nil, true, fail(CodeRoomNotMember, "")
	case errors.Is(err, hub.ErrFull):
		return nil, true, fail(CodeRoomFull, "")
	case err != nil:
		return nil, true, err
	}
	previous := ""
	if current != nil && current.RoomID != room {
		if err := h.Rooms.Leave(ctx, current.RoomID, j.c.UserID, "left"); err != nil && !errors.Is(err, hub.ErrNotMember) &&
			!errors.Is(err, ErrRoomGone) {
			slog.WarnContext(ctx, "hub: previous seat not released", "error", err)
		}
		previous = current.RoomID
	}
	if j.c.GameSessionID != uuid.Nil {
		if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
			return h.PartyRooms.SetGameSession(ctx, j.c.TenantID, pr.RoomID, j.c.UserID, j.c.GameSessionID)
		}); err != nil {
			return nil, true, fmt.Errorf("hub: party room session: %w", err)
		}
	}
	j.audit = false // the session membership of the party room already exists
	a, err := h.assigned(ctx, j, room, res, previous, start)
	return a, true, err
}
