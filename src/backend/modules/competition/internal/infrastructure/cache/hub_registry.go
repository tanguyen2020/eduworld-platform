// Package cache holds the Valkey adapters of module competition (VALKEY_DESIGN §3.1, prefix ew:competition:, owner
// competition · room). The hub registry is ephemeral (ADR-0004): it can be lost; the summary records stay in
// PostgreSQL and the room host rebuilds the live state from the next inputs.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/hub"
)

// Keys (real names, VALKEY_DESIGN §1 rule 4).
func roomKey(room string) string    { return "ew:competition:rt:room:" + room }
func leaseKey(room string) string   { return roomKey(room) + ":lease" }
func membersKey(room string) string { return roomKey(room) + ":members" }

// PosKey is the last valid position of a member (TTL 5 s, read by the facade of F071 for distance checks).
func PosKey(room string, user uuid.UUID) string { return roomKey(room) + ":pos:" + user.String() }
func userKey(user uuid.UUID) string             { return "ew:competition:rt:user:" + user.String() }

func areaKey(area string, p hub.Partition) string {
	return "ew:competition:rt:area:" + area + ":" + p.TenantID.String() + ":" + p.SchoolID.String() + ":" + p.GradeBand + ":instances"
}

// roomsKey indexes the live rooms by last lease renewal (failover watcher, REALTIME_ARCHITECTURE §5.1).
const roomsKey = "ew:competition:rt:rooms"

// roomTTL is the TTL of the registry keys of a room, extended by the owner (2 h, VALKEY_DESIGN §3.1).
const roomTTL = 2 * time.Hour

// HubRegistry implements port.Registry and the writes of the room host.
type HubRegistry struct {
	C valkey.Client
}

var _ port.Registry = HubRegistry{}

// MemberRecord is the seat of a member in the registry (rebuilt on failover, F091-AC-20). Public data only.
type MemberRecord struct {
	EID           int              `json:"eid"`
	Player        hub.PublicPlayer `json:"player"`
	SchoolID      uuid.UUID        `json:"school_id"`
	GradeBand     string           `json:"grade_band"`
	GameSessionID uuid.UUID        `json:"game_session_id"`
	Node          string           `json:"node"`
}

func (r HubRegistry) do(ctx context.Context, cmds ...valkey.Completed) error {
	for _, res := range r.C.DoMulti(ctx, cmds...) {
		if err := res.Error(); err != nil && !valkey.IsValkeyNil(err) {
			return err
		}
	}
	return nil
}

// Register records a new room (meta HASH, area ZSET with 0 seats, rooms index).
func (r HubRegistry) Register(ctx context.Context, m port.RoomMeta, owner string, now time.Time) error {
	p := m.Partition
	return r.do(ctx,
		r.C.B().Hset().Key(roomKey(m.RoomID)).FieldValue().FieldValue("owner", owner).FieldValue("type", hub.RoomTypeHub).
			FieldValue("tenant_id", p.TenantID.String()).FieldValue("school_id", p.SchoolID.String()).FieldValue("grade_band", p.GradeBand).
			FieldValue("area_id", m.AreaID).FieldValue("capacity", strconv.Itoa(m.Capacity)).FieldValue("members", "0").
			FieldValue("state_version", "1").Build(),
		r.C.B().Expire().Key(roomKey(m.RoomID)).Seconds(int64(roomTTL/time.Second)).Build(),
		r.C.B().Zadd().Key(areaKey(m.AreaID, p)).ScoreMember().ScoreMember(0, m.RoomID).Build(),
		r.C.B().Expire().Key(areaKey(m.AreaID, p)).Seconds(int64(roomTTL/time.Second)).Build(),
		r.C.B().Zadd().Key(roomsKey).ScoreMember().ScoreMember(float64(now.UnixMilli()), m.RoomID).Build())
}

// Meta implements port.Registry.
func (r HubRegistry) Meta(ctx context.Context, room string) (*port.RoomMeta, error) {
	h, err := r.C.Do(ctx, r.C.B().Hgetall().Key(roomKey(room)).Build()).AsStrMap()
	if err != nil || len(h) == 0 {
		return nil, err
	}
	tenant, err1 := uuid.Parse(h["tenant_id"])
	school, err2 := uuid.Parse(h["school_id"])
	capacity, err3 := strconv.Atoi(h["capacity"])
	if err := errors.Join(err1, err2, err3); err != nil {
		return nil, fmt.Errorf("hub registry: meta of %s: %w", room, err)
	}
	return &port.RoomMeta{RoomID: room, AreaID: h["area_id"], Capacity: capacity,
		Partition: hub.Partition{TenantID: tenant, SchoolID: school, GradeBand: h["grade_band"]}}, nil
}

// Instances implements port.Registry: the open instances of a partition with their seated students.
func (r HubRegistry) Instances(ctx context.Context, area string, p hub.Partition) ([]port.InstanceInfo, error) {
	rooms, err := r.C.Do(ctx, r.C.B().Zrange().Key(areaKey(area, p)).Min("0").Max("-1").Build()).AsStrSlice()
	if err != nil {
		return nil, err
	}
	out := make([]port.InstanceInfo, 0, len(rooms))
	for _, room := range rooms {
		m, err := r.Meta(ctx, room)
		if err != nil {
			return nil, err
		}
		if m == nil { // closed meanwhile or expired
			continue
		}
		users, err := r.C.Do(ctx, r.C.B().Hkeys().Key(membersKey(room)).Build()).AsStrSlice()
		if err != nil {
			return nil, err
		}
		info := port.InstanceInfo{RoomID: room, Capacity: m.Capacity}
		for _, u := range users {
			if id, err := uuid.Parse(u); err == nil {
				info.Members = append(info.Members, id)
			}
		}
		out = append(out, info)
	}
	return out, nil
}

// UserRoom implements port.Registry.
func (r HubRegistry) UserRoom(ctx context.Context, user uuid.UUID) (*port.UserRoom, error) {
	h, err := r.C.Do(ctx, r.C.B().Hgetall().Key(userKey(user)).Build()).AsStrMap()
	if err != nil || len(h) == 0 || h["room_id"] == "" {
		return nil, err
	}
	return &port.UserRoom{RoomID: h["room_id"], AreaID: h["area_id"]}, nil
}

// Live implements port.Registry.
func (r HubRegistry) Live(ctx context.Context, room string) (bool, error) {
	n, err := r.C.Do(ctx, r.C.B().Exists().Key(leaseKey(room), membersKey(room)).Build()).AsInt64()
	return n > 0, err
}

var (
	renewScript = valkey.NewLuaScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
  redis.call('PEXPIRE', KEYS[1], ARGV[2])
  redis.call('ZADD', KEYS[2], ARGV[3], ARGV[4])
  redis.call('EXPIRE', KEYS[3], 7200)
  redis.call('EXPIRE', KEYS[4], 7200)
  return 1
end
return 0`)
	releaseScript = valkey.NewLuaScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then return redis.call('DEL', KEYS[1]) end
return 0`)
	clearUserScript = valkey.NewLuaScript(`
if redis.call('HGET', KEYS[1], 'room_id') == ARGV[1] then return redis.call('DEL', KEYS[1]) end
return 0`)
)

// Acquire takes the lease of a room (SET NX PX): one owner at a time (REALTIME_ARCHITECTURE §5.1).
func (r HubRegistry) Acquire(ctx context.Context, room, owner string, ttl time.Duration) (bool, error) {
	err := r.C.Do(ctx, r.C.B().Set().Key(leaseKey(room)).Value(owner).Nx().PxMilliseconds(ttl.Milliseconds()).Build()).Error()
	if valkey.IsValkeyNil(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, r.do(ctx, r.C.B().Hset().Key(roomKey(room)).FieldValue().FieldValue("owner", owner).Build())
}

// Renew extends the lease while owner still holds it (false: lost, stop the room here).
func (r HubRegistry) Renew(ctx context.Context, room, owner string, ttl time.Duration, now time.Time) (bool, error) {
	n, err := renewScript.Exec(ctx, r.C, []string{leaseKey(room), roomsKey, roomKey(room), membersKey(room)},
		[]string{owner, strconv.FormatInt(ttl.Milliseconds(), 10), strconv.FormatInt(now.UnixMilli(), 10), room}).AsInt64()
	return n == 1, err
}

// Release gives the lease back (graceful stop).
func (r HubRegistry) Release(ctx context.Context, room, owner string) error {
	return releaseScript.Exec(ctx, r.C, []string{leaseKey(room)}, []string{owner}).Error()
}

// Orphans returns the rooms whose lease was not renewed since before (failover candidates).
func (r HubRegistry) Orphans(ctx context.Context, before time.Time, limit int) ([]string, error) {
	return r.C.Do(ctx, r.C.B().Zrangebyscore().Key(roomsKey).Min("-inf").Max(strconv.FormatInt(before.UnixMilli(), 10)).
		Limit(0, int64(limit)).Build()).AsStrSlice()
}

// Seat records a member and the seat count of its room.
func (r HubRegistry) Seat(ctx context.Context, m port.RoomMeta, user uuid.UUID, rec MemberRecord, seats int) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return r.do(ctx,
		r.C.B().Hset().Key(membersKey(m.RoomID)).FieldValue().FieldValue(user.String(), string(b)).Build(),
		r.C.B().Expire().Key(membersKey(m.RoomID)).Seconds(int64(roomTTL/time.Second)).Build(),
		r.C.B().Hset().Key(roomKey(m.RoomID)).FieldValue().FieldValue("members", strconv.Itoa(seats)).Build(),
		r.C.B().Zadd().Key(areaKey(m.AreaID, m.Partition)).Xx().Ch().ScoreMember().ScoreMember(float64(seats), m.RoomID).Build(),
		r.C.B().Hset().Key(userKey(user)).FieldValue().FieldValue("room_id", m.RoomID).FieldValue("area_id", m.AreaID).Build(),
		r.C.B().Expire().Key(userKey(user)).Seconds(int64(roomTTL/time.Second)).Build())
}

// Unseat removes a member (and its user → room pointer when it still points to this room).
func (r HubRegistry) Unseat(ctx context.Context, m port.RoomMeta, user uuid.UUID, seats int) error {
	if err := r.do(ctx,
		r.C.B().Hdel().Key(membersKey(m.RoomID)).Field(user.String()).Build(),
		r.C.B().Del().Key(PosKey(m.RoomID, user)).Build(),
		r.C.B().Hset().Key(roomKey(m.RoomID)).FieldValue().FieldValue("members", strconv.Itoa(seats)).Build(),
		r.C.B().Zadd().Key(areaKey(m.AreaID, m.Partition)).Xx().Ch().ScoreMember().ScoreMember(float64(seats), m.RoomID).Build()); err != nil {
		return err
	}
	return clearUserScript.Exec(ctx, r.C, []string{userKey(user)}, []string{m.RoomID}).Error()
}

// Members returns the seats of a room (failover).
func (r HubRegistry) Members(ctx context.Context, room string) (map[uuid.UUID]MemberRecord, error) {
	h, err := r.C.Do(ctx, r.C.B().Hgetall().Key(membersKey(room)).Build()).AsStrMap()
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]MemberRecord, len(h))
	for k, v := range h {
		id, err := uuid.Parse(k)
		var rec MemberRecord
		if err != nil || json.Unmarshal([]byte(v), &rec) != nil {
			continue
		}
		out[id] = rec
	}
	return out, nil
}

// Positions writes the last valid positions (≈ 1 Hz, TTL 5 s) and reads them back for a failover.
func (r HubRegistry) Positions(ctx context.Context, room string, pos map[uuid.UUID]hub.Vec3) error {
	if len(pos) == 0 {
		return nil
	}
	cmds := make(valkey.Commands, 0, len(pos))
	for u, p := range pos {
		b, _ := json.Marshal(p)
		cmds = append(cmds, r.C.B().Set().Key(PosKey(room, u)).Value(string(b)).ExSeconds(5).Build())
	}
	return r.do(ctx, cmds...)
}

// Position returns the last valid position of a member (nil when expired).
func (r HubRegistry) Position(ctx context.Context, room string, user uuid.UUID) (*hub.Vec3, error) {
	s, err := r.C.Do(ctx, r.C.B().Get().Key(PosKey(room, user)).Build()).ToString()
	if valkey.IsValkeyNil(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p hub.Vec3
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return nil, fmt.Errorf("hub registry: position: %w", err)
	}
	return &p, nil
}

// Remove deletes a closed room from the registry.
func (r HubRegistry) Remove(ctx context.Context, m port.RoomMeta, owner string) error {
	if err := r.do(ctx,
		r.C.B().Zrem().Key(areaKey(m.AreaID, m.Partition)).Member(m.RoomID).Build(),
		r.C.B().Zrem().Key(roomsKey).Member(m.RoomID).Build(),
		r.C.B().Del().Key(roomKey(m.RoomID), membersKey(m.RoomID)).Build()); err != nil {
		return err
	}
	return r.Release(ctx, m.RoomID, owner)
}
