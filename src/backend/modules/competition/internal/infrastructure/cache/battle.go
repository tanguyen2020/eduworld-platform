package cache

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"
)

// Battle rooms (F105, REALTIME_ARCHITECTURE §5.1, VALKEY_DESIGN §3.1): the room HASH (owner, type, tenant), the lease
// (SET NX PX, renewed every 3 s), the checkpoint written after every state transition (TTL 1 h) and the index of the
// live battle rooms by last renewal for the failover watcher. Never the source of truth (ADR-0004): results, scores and
// memberships are in PostgreSQL; a lost checkpoint aborts the match (F105-AC-15).
func ckptKey(room string) string { return roomKey(room) + ":ckpt" }

// battlesKey indexes the live battle rooms by last lease renewal (separate from the hub rooms of F091).
const battlesKey = "ew:competition:rt:battles"

// BattleStore is the Valkey store of the battle room host.
type BattleStore struct {
	C valkey.Client
}

var battleRenew = valkey.NewLuaScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
  redis.call('PEXPIRE', KEYS[1], ARGV[2])
  redis.call('ZADD', KEYS[2], ARGV[3], ARGV[4])
  redis.call('EXPIRE', KEYS[3], 7200)
  return 1
end
return 0`)

func (s BattleStore) do(ctx context.Context, cmds ...valkey.Completed) error {
	for _, res := range s.C.DoMulti(ctx, cmds...) {
		if err := res.Error(); err != nil && !valkey.IsValkeyNil(err) {
			return err
		}
	}
	return nil
}

// Register records a battle room hosted by owner.
func (s BattleStore) Register(ctx context.Context, room string, tenant uuid.UUID, roomType, owner string, now time.Time) error {
	return s.do(ctx,
		s.C.B().Hset().Key(roomKey(room)).FieldValue().FieldValue("owner", owner).FieldValue("type", roomType).
			FieldValue("tenant_id", tenant.String()).Build(),
		s.C.B().Expire().Key(roomKey(room)).Seconds(int64(roomTTL/time.Second)).Build(),
		s.C.B().Zadd().Key(battlesKey).ScoreMember().ScoreMember(float64(now.UnixMilli()), room).Build())
}

// Tenant returns the tenant of a registered battle room (nil when unknown).
func (s BattleStore) Tenant(ctx context.Context, room string) (*uuid.UUID, error) {
	v, err := s.C.Do(ctx, s.C.B().Hget().Key(roomKey(room)).Field("tenant_id").Build()).ToString()
	if valkey.IsValkeyNil(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	id, perr := uuid.Parse(v)
	if perr != nil {
		return nil, nil //nolint:nilerr // a corrupt registry entry is an unknown room
	}
	return &id, nil
}

// Acquire takes the lease of a room (one owner at a time).
func (s BattleStore) Acquire(ctx context.Context, room, owner string, ttl time.Duration) (bool, error) {
	err := s.C.Do(ctx, s.C.B().Set().Key(leaseKey(room)).Value(owner).Nx().PxMilliseconds(ttl.Milliseconds()).Build()).Error()
	if valkey.IsValkeyNil(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, s.do(ctx, s.C.B().Hset().Key(roomKey(room)).FieldValue().FieldValue("owner", owner).Build())
}

// Renew extends the lease while owner holds it (false: lost).
func (s BattleStore) Renew(ctx context.Context, room, owner string, ttl time.Duration, now time.Time) (bool, error) {
	n, err := battleRenew.Exec(ctx, s.C, []string{leaseKey(room), battlesKey, roomKey(room)},
		[]string{owner, strconv.FormatInt(ttl.Milliseconds(), 10), strconv.FormatInt(now.UnixMilli(), 10), room}).AsInt64()
	return n == 1, err
}

// Release gives the lease back.
func (s BattleStore) Release(ctx context.Context, room, owner string) error {
	return releaseScript.Exec(ctx, s.C, []string{leaseKey(room)}, []string{owner}).Error()
}

// Orphans returns the battle rooms whose lease was not renewed since before.
func (s BattleStore) Orphans(ctx context.Context, before time.Time, limit int) ([]string, error) {
	return s.C.Do(ctx, s.C.B().Zrangebyscore().Key(battlesKey).Min("-inf").Max(strconv.FormatInt(before.UnixMilli(), 10)).
		Limit(0, int64(limit)).Build()).AsStrSlice()
}

// SaveCheckpoint writes the checkpoint of a room (TTL ttl).
func (s BattleStore) SaveCheckpoint(ctx context.Context, room string, data []byte, ttl time.Duration) error {
	return s.C.Do(ctx, s.C.B().Set().Key(ckptKey(room)).Value(valkey.BinaryString(data)).ExSeconds(int64(ttl/time.Second)).Build()).Error()
}

// Checkpoint reads the checkpoint of a room (nil when none).
func (s BattleStore) Checkpoint(ctx context.Context, room string) ([]byte, error) {
	b, err := s.C.Do(ctx, s.C.B().Get().Key(ckptKey(room)).Build()).AsBytes()
	if valkey.IsValkeyNil(err) {
		return nil, nil
	}
	return b, err
}

// Remove drops a room that ended: index, meta, checkpoint and lease.
func (s BattleStore) Remove(ctx context.Context, room string) error {
	return s.do(ctx,
		s.C.B().Zrem().Key(battlesKey).Member(room).Build(),
		s.C.B().Del().Key(roomKey(room), ckptKey(room), leaseKey(room)).Build())
}
