// Package room hosts the rooms of module game on the shared room host (pkg/roomhost, F076 T033): a solo challenge
// attempt in progress is a room of type challenge_solo whose timer ends it at its due time on the server clock; the
// checkpoint is the attempt row in PostgreSQL.
package room

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/challenge"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/roomhost"
)

// Expirer is the use case behind the timer and the checkpoint (command.Challenges).
type Expirer interface {
	Expire(ctx context.Context, tenant, attemptID uuid.UUID) (bool, error)
	RoomCheckpoint(ctx context.Context, tenant, attemptID uuid.UUID) (time.Time, bool, error)
}

// Challenges implements port.ChallengeRoom (announce/close in the registry), roomhost.Handler (timer) and
// roomhost.CheckpointStore (takeover) for the room type challenge_solo.
type Challenges struct {
	Registry roomhost.Registry
	Type     string
	Expirer  Expirer
	Timeout  time.Duration // Valkey calls of announce/close (best effort)
}

var (
	_ port.ChallengeRoom       = Challenges{}
	_ roomhost.Handler         = Challenges{}
	_ roomhost.CheckpointStore = Challenges{}
)

func (c Challenges) bounded(ctx context.Context) (context.Context, context.CancelFunc) {
	t := c.Timeout
	if t <= 0 {
		t = 500 * time.Millisecond
	}
	return context.WithTimeout(context.WithoutCancel(ctx), t)
}

// Open implements port.ChallengeRoom: the room record (type, tenant, due time) in the orphan index; an owner node
// takes it at its next scan.
func (c Challenges) Open(ctx context.Context, a *challenge.Attempt, due time.Time) error {
	if c.Registry == nil {
		return nil
	}
	ctx, cancel := c.bounded(ctx)
	defer cancel()
	return c.Registry.Announce(ctx, roomhost.Room{ID: a.ID.String(), Type: c.Type, TenantID: a.TenantID.String(), DueAt: &due})
}

// Close implements port.ChallengeRoom.
func (c Challenges) Close(ctx context.Context, attemptID uuid.UUID) error {
	if c.Registry == nil {
		return nil
	}
	ctx, cancel := c.bounded(ctx)
	defer cancel()
	return c.Registry.Forget(ctx, attemptID.String())
}

// ids reads the tenant and the attempt of a room record; false for a record that is not a challenge room.
func ids(r roomhost.Room) (uuid.UUID, uuid.UUID, bool) {
	tenant, err1 := uuid.Parse(r.TenantID)
	id, err2 := uuid.Parse(r.ID)
	return tenant, id, err1 == nil && err2 == nil
}

// OnTimer implements roomhost.Handler: the attempt ends (failed past deadline_at, abandoned past the resume window).
func (c Challenges) OnTimer(ctx context.Context, r roomhost.Room, _ time.Time) (bool, error) {
	tenant, id, ok := ids(r)
	if !ok {
		return true, nil // not a challenge room: forget it
	}
	return c.Expirer.Expire(ctx, tenant, id)
}

// Load implements roomhost.CheckpointStore: the due time of the attempt row wins over the registry.
func (c Challenges) Load(ctx context.Context, r roomhost.Room) (roomhost.Checkpoint, error) {
	tenant, id, ok := ids(r)
	if !ok {
		return roomhost.Checkpoint{Room: r, Closed: true}, nil
	}
	due, active, err := c.Expirer.RoomCheckpoint(ctx, tenant, id)
	if err != nil {
		return roomhost.Checkpoint{}, err
	}
	if !active {
		return roomhost.Checkpoint{Room: r, Closed: true}, nil
	}
	r.DueAt = &due
	return roomhost.Checkpoint{Room: r}, nil
}
