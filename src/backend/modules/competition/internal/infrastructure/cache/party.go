package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// Keys of the party state (VALKEY_DESIGN §3.2, owner competition · party): every key is built here (no scattered
// concatenation, VALKEY_DESIGN §8); scripts receive them through KEYS[] only.

// PartyKey is ew:party:<party_id> (HASH: leader_id, party_type, max_members, status, state_version).
func PartyKey(party uuid.UUID) string { return "ew:party:" + party.String() }

// PartyMembersKey is ew:party:<party_id>:members (HASH student_id → role).
func PartyMembersKey(party uuid.UUID) string { return PartyKey(party) + ":members" }

// PartyUserKey is ew:party:user:<user_id> (STRING party_id).
func PartyUserKey(user uuid.UUID) string { return "ew:party:user:" + user.String() }

// PartyPresenceKey is ew:party:presence:<user_id> (HASH status, ts): the last presence signal of the realtime gateway
// for the party rules (transfer to an online member, next leader), TTL presence_ttl.
func PartyPresenceKey(user uuid.UUID) string { return "ew:party:presence:" + user.String() }

// PartyStateTTL is the lifetime of the realtime party state (VALKEY_DESIGN §3.2: 24 h, renewed on activity).
const PartyStateTTL = 24 * time.Hour

// syncParty rewrites the party hash and the members hash, and points the user keys of the members at the party.
// KEYS[1] party hash, KEYS[2] members hash, KEYS[3..] user keys; ARGV[1] ttl s, ARGV[2] party id, ARGV[3..7] fields,
// ARGV[8..] member/role pairs (same order as KEYS[3..]).
var syncParty = valkey.NewLuaScript(`
redis.call('DEL', KEYS[1], KEYS[2])
redis.call('HSET', KEYS[1], 'leader_id', ARGV[3], 'party_type', ARGV[4], 'max_members', ARGV[5], 'status', ARGV[6], 'state_version', ARGV[7])
redis.call('EXPIRE', KEYS[1], ARGV[1])
for i = 3, #KEYS do
  redis.call('HSET', KEYS[2], ARGV[8 + (i - 3) * 2], ARGV[9 + (i - 3) * 2])
  redis.call('SET', KEYS[i], ARGV[2], 'EX', ARGV[1])
end
if #KEYS > 2 then redis.call('EXPIRE', KEYS[2], ARGV[1]) end
return 1`)

// dropParty deletes the party hashes (KEYS[1], KEYS[2] when given) and the user keys still pointing at ARGV[1].
var dropParty = valkey.NewLuaScript(`
local first = 1
if ARGV[2] == '1' then redis.call('DEL', KEYS[1], KEYS[2]); first = 3 end
for i = first, #KEYS do
  if redis.call('GET', KEYS[i]) == ARGV[1] then redis.call('DEL', KEYS[i]) end
end
return 1`)

// PartyState implements port.PartyStateCache (rebuildable from PostgreSQL, ADR-0004).
type PartyState struct{ C valkey.Client }

var _ port.PartyStateCache = PartyState{}

// Sync implements port.PartyStateCache.
func (s PartyState) Sync(ctx context.Context, p port.PartySnapshot) error {
	keys := []string{PartyKey(p.PartyID), PartyMembersKey(p.PartyID)}
	args := []string{strconv.Itoa(int(PartyStateTTL / time.Second)), p.PartyID.String(), p.LeaderID.String(), p.Type,
		strconv.Itoa(p.MaxMembers), p.Status, strconv.FormatInt(p.StateVersion, 10)}
	for id, role := range p.Members {
		keys = append(keys, PartyUserKey(id))
		args = append(args, id.String(), role)
	}
	if err := syncParty.Exec(ctx, s.C, keys, args).Error(); err != nil {
		return fmt.Errorf("party state: sync: %w", err)
	}
	return nil
}

// Drop implements port.PartyStateCache: party uuid.Nil keeps the party hashes and only drops the user keys of students
// who left.
func (s PartyState) Drop(ctx context.Context, party uuid.UUID, students []uuid.UUID) error {
	var keys []string
	whole := "0"
	if party != uuid.Nil {
		keys = append(keys, PartyKey(party), PartyMembersKey(party))
		whole = "1"
	}
	for _, id := range students {
		keys = append(keys, PartyUserKey(id))
	}
	if len(keys) == 0 {
		return nil
	}
	if err := dropParty.Exec(ctx, s.C, keys, []string{party.String(), whole}).Error(); err != nil {
		return fmt.Errorf("party state: drop: %w", err)
	}
	return nil
}

// putPresence stores the last presence of a student unless the stored one is newer (by ts), with TTL.
var putPresence = valkey.NewLuaScript(`
local ts = tonumber(redis.call('HGET', KEYS[1], 'ts') or '0')
if ts > tonumber(ARGV[2]) then return 0 end
redis.call('HSET', KEYS[1], 'status', ARGV[1], 'ts', ARGV[2])
redis.call('EXPIRE', KEYS[1], ARGV[3])
return 1`)

// PartyPresence implements port.PartyPresence on ew:party:presence:<user_id>: a student is online while the key holds
// a status other than offline (expired key = offline).
type PartyPresence struct {
	C   valkey.Client
	TTL time.Duration
}

var _ port.PartyPresence = PartyPresence{}

// Put stores a presence signal (subscriber competition.party_presence); offline (or a lost connection) clears it.
func (p PartyPresence) Put(ctx context.Context, user uuid.UUID, status string, ts int64) error {
	ttl := int64(p.TTL / time.Second)
	if ttl < 1 {
		ttl = 1
	}
	if err := putPresence.Exec(ctx, p.C, []string{PartyPresenceKey(user)},
		[]string{status, strconv.FormatInt(ts, 10), strconv.FormatInt(ttl, 10)}).Error(); err != nil {
		return fmt.Errorf("party presence: put: %w", err)
	}
	return nil
}

// Online implements port.PartyPresence.
func (p PartyPresence) Online(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	out := make(map[uuid.UUID]bool, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	cmds := make(valkey.Commands, len(ids))
	for i, id := range ids {
		cmds[i] = p.C.B().Hget().Key(PartyPresenceKey(id)).Field("status").Build()
	}
	for i, r := range p.C.DoMulti(ctx, cmds...) {
		s, err := r.ToString()
		if valkey.IsValkeyNil(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("party presence: %w", err)
		}
		out[ids[i]] = s != "" && s != rt.PresenceOffline
	}
	return out, nil
}
