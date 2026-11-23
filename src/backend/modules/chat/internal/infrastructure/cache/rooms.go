package cache

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
)

// Rooms implements port.RoomTracker on ew:chat:room:<tenant_id>:<user_id> = room_id with TTL presence_ttl (F120-AC-13):
// the hub instance the gateway reported in the last presence signal of the student. Temporary state only (ADR-0004).
type Rooms struct {
	C   valkey.Client
	TTL time.Duration
}

var _ port.RoomTracker = Rooms{}

func roomKey(tenant, user uuid.UUID) string {
	return "ew:chat:room:" + tenant.String() + ":" + user.String()
}

// SetRoom implements port.RoomTracker ("" forgets the room).
func (r Rooms) SetRoom(ctx context.Context, tenant, user uuid.UUID, room string) error {
	if room == "" {
		return r.C.Do(ctx, r.C.B().Del().Key(roomKey(tenant, user)).Build()).Error()
	}
	ttl := r.TTL
	if ttl < time.Second {
		ttl = 90 * time.Second
	}
	return r.C.Do(ctx, r.C.B().Set().Key(roomKey(tenant, user)).Value(room).Ex(ttl).Build()).Error()
}

// Room implements port.RoomTracker.
func (r Rooms) Room(ctx context.Context, tenant, user uuid.UUID) (string, error) {
	v, err := r.C.Do(ctx, r.C.B().Get().Key(roomKey(tenant, user)).Build()).ToString()
	if valkey.IsValkeyNil(err) {
		return "", nil
	}
	return v, err
}
