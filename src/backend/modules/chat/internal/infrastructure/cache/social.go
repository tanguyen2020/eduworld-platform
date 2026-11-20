// Package cache implements the ephemeral Valkey state of module chat (VALKEY_DESIGN, prefix ew:chat:): the last
// presence received per student and the friend ids cache. Losing it only means "offline" until the next signal and a
// read of the database (ADR-0004, AGENTS R9).
package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
)

func presenceKey(user uuid.UUID) string { return "ew:chat:social:presence:" + user.String() }
func friendsKey(user uuid.UUID) string  { return "ew:chat:social:friends:" + user.String() }

// friendsEmpty marks a cached empty friend list (a SET cannot be empty).
const friendsEmpty = "-"

// putPresence stores the presence unless the stored one is newer, refreshes the TTL and returns the previous fields.
var putPresence = valkey.NewLuaScript(`
local cur = redis.call('HGETALL', KEYS[1])
local ts = 0
for i = 1, #cur, 2 do if cur[i] == 'ts' then ts = tonumber(cur[i+1]) end end
if #cur > 0 and ts > tonumber(ARGV[5]) then return {0, cur} end
redis.call('HSET', KEYS[1], 'status', ARGV[1], 'activity', ARGV[2], 'area_id', ARGV[3], 'hidden', ARGV[4], 'ts', ARGV[5])
redis.call('EXPIRE', KEYS[1], ARGV[6])
return {1, cur}`)

// Presence implements port.PresenceStore: HASH ew:chat:social:presence:<user_id> {status, activity, area_id, hidden,
// ts} with TTL presence_ttl.
type Presence struct {
	C   valkey.Client
	TTL time.Duration
}

var _ port.PresenceStore = Presence{}

func fields(m map[string]string) social.Presence {
	ts, _ := strconv.ParseInt(m["ts"], 10, 64)
	return social.Presence{Status: m["status"], Activity: m["activity"], AreaID: m["area_id"], Hidden: m["hidden"] == "1", Ts: ts}
}

// Put implements port.PresenceStore.
func (p Presence) Put(ctx context.Context, user uuid.UUID, s social.Presence) (*social.Presence, bool, error) {
	hidden := "0"
	if s.Hidden {
		hidden = "1"
	}
	ttl := int64(p.TTL / time.Second)
	if ttl < 1 {
		ttl = 1
	}
	res, err := putPresence.Exec(ctx, p.C, []string{presenceKey(user)},
		[]string{s.Status, s.Activity, s.AreaID, hidden, strconv.FormatInt(s.Ts, 10), strconv.FormatInt(ttl, 10)}).ToArray()
	if err != nil {
		return nil, false, fmt.Errorf("chat presence: put: %w", err)
	}
	if len(res) != 2 {
		return nil, false, fmt.Errorf("chat presence: put: unexpected reply")
	}
	stored, _ := res[0].AsInt64()
	cur, err := res[1].ToArray()
	if err != nil {
		return nil, false, fmt.Errorf("chat presence: put: %w", err)
	}
	var prev *social.Presence
	if len(cur) > 0 {
		m := map[string]string{}
		for i := 0; i+1 < len(cur); i += 2 {
			k, _ := cur[i].ToString()
			v, _ := cur[i+1].ToString()
			m[k] = v
		}
		f := fields(m)
		prev = &f
	}
	return prev, stored == 1, nil
}

// Get implements port.PresenceStore.
func (p Presence) Get(ctx context.Context, users []uuid.UUID) (map[uuid.UUID]social.Presence, error) {
	out := make(map[uuid.UUID]social.Presence, len(users))
	if len(users) == 0 {
		return out, nil
	}
	cmds := make(valkey.Commands, len(users))
	for i, u := range users {
		cmds[i] = p.C.B().Hgetall().Key(presenceKey(u)).Build()
	}
	for i, r := range p.C.DoMulti(ctx, cmds...) {
		m, err := r.AsStrMap()
		if err != nil {
			return nil, fmt.Errorf("chat presence: get: %w", err)
		}
		if len(m) > 0 {
			out[users[i]] = fields(m)
		}
	}
	return out, nil
}

// Friends implements port.FriendCache: SET ew:chat:social:friends:<user_id> of friend ids, TTL 10 min, dropped when a
// relation of the student changes.
type Friends struct {
	C   valkey.Client
	TTL time.Duration
}

var _ port.FriendCache = Friends{}

// FriendsTTL is the TTL of the friend ids cache (VALKEY_DESIGN §3.1).
const FriendsTTL = 10 * time.Minute

// Get implements port.FriendCache.
func (f Friends) Get(ctx context.Context, user uuid.UUID) ([]uuid.UUID, bool, error) {
	members, err := f.C.Do(ctx, f.C.B().Smembers().Key(friendsKey(user)).Build()).AsStrSlice()
	if err != nil || len(members) == 0 {
		return nil, false, err
	}
	out := make([]uuid.UUID, 0, len(members))
	for _, m := range members {
		if m == friendsEmpty {
			continue
		}
		id, err := uuid.Parse(m)
		if err != nil {
			return nil, false, fmt.Errorf("chat friends cache: member: %w", err)
		}
		out = append(out, id)
	}
	return out, true, nil
}

// Set implements port.FriendCache.
func (f Friends) Set(ctx context.Context, user uuid.UUID, ids []uuid.UUID) error {
	ttl := f.TTL
	if ttl <= 0 {
		ttl = FriendsTTL
	}
	members := []string{friendsEmpty}
	for _, id := range ids {
		members = append(members, id.String())
	}
	key := friendsKey(user)
	for _, r := range f.C.DoMulti(ctx, f.C.B().Del().Key(key).Build(), f.C.B().Sadd().Key(key).Member(members...).Build(),
		f.C.B().Expire().Key(key).Seconds(int64(ttl/time.Second)).Build()) {
		if err := r.Error(); err != nil {
			return fmt.Errorf("chat friends cache: %w", err)
		}
	}
	return nil
}

// Invalidate implements port.FriendCache.
func (f Friends) Invalidate(ctx context.Context, users ...uuid.UUID) error {
	if len(users) == 0 {
		return nil
	}
	cmds := make(valkey.Commands, len(users)) // one key per command: no cross-slot DEL in a cluster
	for i, u := range users {
		cmds[i] = f.C.B().Del().Key(friendsKey(u)).Build()
	}
	for _, r := range f.C.DoMulti(ctx, cmds...) {
		if err := r.Error(); err != nil {
			return fmt.Errorf("chat friends cache: %w", err)
		}
	}
	return nil
}
