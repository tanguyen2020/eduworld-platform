package roomhost

import (
	"context"
	"strconv"
	"time"

	"github.com/valkey-io/valkey-go"
)

// Keys of the room registry (VALKEY_DESIGN §3.1): ew:rt:room:<room_id> (HASH type, tenant_id, due_at_ms, owner),
// ew:rt:room:<room_id>:lease (owner, TTL) and the orphan index of the hosting module (ZSET room_id → last renewal ms).
func roomKey(id string) string  { return "ew:rt:room:" + id }
func leaseKey(id string) string { return "ew:rt:room:" + id + ":lease" }

// ValkeyRegistry implements Registry on Valkey. Index is the orphan index of the hosting module (ew:rt:rooms:<module>):
// every module scans only its own rooms. Valkey holds the registry and the lease only — never the room state (ADR-0004).
type ValkeyRegistry struct {
	C     valkey.Client
	Index string
	// Keep is how long a room record outlives its due time (or its announce when it has no timer).
	Keep time.Duration
}

var _ Registry = ValkeyRegistry{}

func (r ValkeyRegistry) keep() time.Duration {
	if r.Keep > 0 {
		return r.Keep
	}
	return 2 * time.Hour
}

var (
	// KEYS lease, index, room · ARGV owner, ttl_ms, now_ms, room_id → 1 renewed, 0 lost (other owner or room forgotten).
	renewScript = valkey.NewLuaScript(`
if redis.call('EXISTS', KEYS[3]) == 0 then return 0 end
if redis.call('GET', KEYS[1]) ~= ARGV[1] then return 0 end
redis.call('PEXPIRE', KEYS[1], ARGV[2])
redis.call('ZADD', KEYS[2], ARGV[3], ARGV[4])
return 1`)
	// KEYS lease, index · ARGV owner, room_id → the lease is deleted and the room scored 0 (orphan at once).
	releaseScript = valkey.NewLuaScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
  redis.call('DEL', KEYS[1])
  redis.call('ZADD', KEYS[2], 'XX', 0, ARGV[2])
end
return 1`)
)

// Announce implements Registry: the record (expiring Keep after its due time) and an index entry scored 0.
func (r ValkeyRegistry) Announce(ctx context.Context, room Room) error {
	due, ttl := "", r.keep()
	if room.DueAt != nil {
		due = strconv.FormatInt(room.DueAt.UnixMilli(), 10)
		if d := time.Until(*room.DueAt) + r.keep(); d > ttl {
			ttl = d
		}
	}
	return r.do(ctx,
		r.C.B().Hset().Key(roomKey(room.ID)).FieldValue().FieldValue("type", room.Type).FieldValue("tenant_id", room.TenantID).
			FieldValue("due_at_ms", due).Build(),
		r.C.B().Expire().Key(roomKey(room.ID)).Seconds(int64(ttl/time.Second)+1).Build(),
		r.C.B().Zadd().Key(r.Index).Nx().ScoreMember().ScoreMember(0, room.ID).Build())
}

// Acquire implements Registry (SET NX PX; the owner is written in the record for the takeover metric).
func (r ValkeyRegistry) Acquire(ctx context.Context, room, owner string, ttl time.Duration) (bool, error) {
	err := r.C.Do(ctx, r.C.B().Set().Key(leaseKey(room)).Value(owner).Nx().PxMilliseconds(ttl.Milliseconds()).Build()).Error()
	if valkey.IsValkeyNil(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, r.C.Do(ctx, r.C.B().Hset().Key(roomKey(room)).FieldValue().FieldValue("owner", owner).Build()).Error()
}

// Renew implements Registry.
func (r ValkeyRegistry) Renew(ctx context.Context, room, owner string, ttl time.Duration, now time.Time) (bool, error) {
	n, err := renewScript.Exec(ctx, r.C, []string{leaseKey(room), r.Index, roomKey(room)},
		[]string{owner, strconv.FormatInt(ttl.Milliseconds(), 10), strconv.FormatInt(now.UnixMilli(), 10), room}).AsInt64()
	return n == 1, err
}

// Release implements Registry.
func (r ValkeyRegistry) Release(ctx context.Context, room, owner string) error {
	return releaseScript.Exec(ctx, r.C, []string{leaseKey(room), r.Index}, []string{owner, room}).Error()
}

// Room implements Registry.
func (r ValkeyRegistry) Room(ctx context.Context, id string) (*Room, error) {
	m, err := r.C.Do(ctx, r.C.B().Hgetall().Key(roomKey(id)).Build()).AsStrMap()
	if err != nil {
		return nil, err
	}
	if len(m) == 0 || m["type"] == "" {
		return nil, nil
	}
	out := &Room{ID: id, Type: m["type"], TenantID: m["tenant_id"], Owner: m["owner"]}
	if ms, err := strconv.ParseInt(m["due_at_ms"], 10, 64); err == nil && ms > 0 {
		t := time.UnixMilli(ms).UTC()
		out.DueAt = &t
	}
	return out, nil
}

// Forget implements Registry.
func (r ValkeyRegistry) Forget(ctx context.Context, id string) error {
	return r.do(ctx, r.C.B().Del().Key(roomKey(id)).Build(), r.C.B().Del().Key(leaseKey(id)).Build(), r.C.B().Zrem().Key(r.Index).Member(id).Build())
}

// Orphans implements Registry.
func (r ValkeyRegistry) Orphans(ctx context.Context, before time.Time, limit int) ([]string, error) {
	return r.C.Do(ctx, r.C.B().Zrangebyscore().Key(r.Index).Min("-inf").Max(strconv.FormatInt(before.UnixMilli(), 10)).
		Limit(0, int64(limit)).Build()).AsStrSlice()
}

func (r ValkeyRegistry) do(ctx context.Context, cmds ...valkey.Completed) error {
	for _, res := range r.C.DoMulti(ctx, cmds...) {
		if err := res.Error(); err != nil {
			return err
		}
	}
	return nil
}
