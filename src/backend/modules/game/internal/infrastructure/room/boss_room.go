package room

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/roomhost"
)

// Timed is the use case behind the timer and the checkpoint of a room type (command.Bosses for boss rooms,
// command.PartyQuests for party quest sessions).
type Timed interface {
	OnTimer(ctx context.Context, tenant, id uuid.UUID) (bool, error)
	RoomCheckpoint(ctx context.Context, tenant, id uuid.UUID) (time.Time, bool, error)
}

// Rooms implements port.GameRoom (announce/close in the registry), roomhost.Handler (timer) and
// roomhost.CheckpointStore (takeover) for one room type of F103: boss (an encounter) or party_quest (a shared quest
// session). The checkpoint is the row in PostgreSQL (ADR-0004); the due time is reloaded from it after every timer that
// did not end the room (pkg/roomhost).
type Rooms struct {
	Registry roomhost.Registry
	Type     string
	Use      Timed
	Timeout  time.Duration
}

var (
	_ port.GameRoom            = Rooms{}
	_ roomhost.Handler         = Rooms{}
	_ roomhost.CheckpointStore = Rooms{}
)

func (r Rooms) bounded(ctx context.Context) (context.Context, context.CancelFunc) {
	t := r.Timeout
	if t <= 0 {
		t = 500 * time.Millisecond
	}
	return context.WithTimeout(context.WithoutCancel(ctx), t)
}

// Open implements port.GameRoom: the room record (type, tenant, due time) in the orphan index; an owner node takes it at
// its next scan.
func (r Rooms) Open(ctx context.Context, tenant, id uuid.UUID, due time.Time) error {
	if r.Registry == nil {
		return nil
	}
	ctx, cancel := r.bounded(ctx)
	defer cancel()
	return r.Registry.Announce(ctx, roomhost.Room{ID: id.String(), Type: r.Type, TenantID: tenant.String(), DueAt: &due})
}

// Close implements port.GameRoom.
func (r Rooms) Close(ctx context.Context, id uuid.UUID) error {
	if r.Registry == nil {
		return nil
	}
	ctx, cancel := r.bounded(ctx)
	defer cancel()
	return r.Registry.Forget(ctx, id.String())
}

// OnTimer implements roomhost.Handler.
func (r Rooms) OnTimer(ctx context.Context, rm roomhost.Room, _ time.Time) (bool, error) {
	tenant, id, ok := ids(rm)
	if !ok {
		return true, nil
	}
	return r.Use.OnTimer(ctx, tenant, id)
}

// Load implements roomhost.CheckpointStore.
func (r Rooms) Load(ctx context.Context, rm roomhost.Room) (roomhost.Checkpoint, error) {
	tenant, id, ok := ids(rm)
	if !ok {
		return roomhost.Checkpoint{Room: rm, Closed: true}, nil
	}
	due, live, err := r.Use.RoomCheckpoint(ctx, tenant, id)
	if err != nil {
		return roomhost.Checkpoint{}, err
	}
	if !live {
		return roomhost.Checkpoint{Room: rm, Closed: true}, nil
	}
	rm.DueAt = &due
	return roomhost.Checkpoint{Room: rm}, nil
}

// Checkpoints dispatches the checkpoint of a room to the store of its type (one host serves the challenge, boss and
// party quest rooms of module game).
type Checkpoints map[string]roomhost.CheckpointStore

var _ roomhost.CheckpointStore = Checkpoints{}

// Load implements roomhost.CheckpointStore: a room of an unknown type is closed.
func (c Checkpoints) Load(ctx context.Context, rm roomhost.Room) (roomhost.Checkpoint, error) {
	s, ok := c[rm.Type]
	if !ok {
		return roomhost.Checkpoint{Room: rm, Closed: true}, nil
	}
	return s.Load(ctx, rm)
}
