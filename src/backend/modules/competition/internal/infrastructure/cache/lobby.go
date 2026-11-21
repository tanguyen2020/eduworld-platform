package cache

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
)

// Keys of the Party Lobby (VALKEY_DESIGN §3.2, owner competition · party, F101 research R3).

// PartyLobbyKey is ew:party:<party_id>:lobby (HASH): r:<user> Ready, s:<user> member status, g:<user> disconnected
// since (ms), leader_gone, last_online, below_min (ms). Separate from the F100 hashes rewritten by every party change.
func PartyLobbyKey(party uuid.UUID) string { return PartyKey(party) + ":lobby" }

// PartyActiveKey is ew:party:active (ZSET "<tenant_id>:<party_id>" → tracked since ms): the parties the presence
// sweeper evaluates; the sweeper removes disbanded ones and reconciles with PostgreSQL.
const PartyActiveKey = "ew:party:active"

// LobbyRateKey is ew:rl:party_lobby:<user_id> (STRING counter, 1 s window): party_lobby_cmd_rate.
func LobbyRateKey(user uuid.UUID) string { return "ew:rl:party_lobby:" + user.String() }

// JobLockKey is ew:lock:job:<name> (STRING NX PX): one replica runs a cycle of a job.
func JobLockKey(name string) string { return "ew:lock:job:" + name }

func activeMember(tenant, party uuid.UUID) string { return tenant.String() + ":" + party.String() }

// LobbyStore implements port.LobbyStore and port.JobLock on Valkey (ephemeral, ADR-0004).
type LobbyStore struct{ C valkey.Client }

var (
	_ port.LobbyStore = LobbyStore{}
	_ port.JobLock    = LobbyStore{}
)

func ttlSeconds() int64 { return int64(PartyStateTTL / time.Second) }

// Load implements port.LobbyStore.
func (s LobbyStore) Load(ctx context.Context, partyID uuid.UUID) (party.Lobby, error) {
	h, err := s.C.Do(ctx, s.C.B().Hgetall().Key(PartyLobbyKey(partyID)).Build()).AsStrMap()
	if err != nil && !valkey.IsValkeyNil(err) {
		return party.Lobby{}, fmt.Errorf("party lobby: load: %w", err)
	}
	l := party.Lobby{Members: map[uuid.UUID]party.MemberState{}}
	at := func(v string) *time.Time {
		ms, err := strconv.ParseInt(v, 10, 64)
		if err != nil || ms <= 0 {
			return nil
		}
		t := time.UnixMilli(ms).UTC()
		return &t
	}
	member := func(id string, fn func(m *party.MemberState)) {
		u, err := uuid.Parse(id)
		if err != nil {
			return
		}
		m := l.Members[u]
		fn(&m)
		l.Members[u] = m
	}
	for k, v := range h {
		switch {
		case strings.HasPrefix(k, "r:"):
			member(k[2:], func(m *party.MemberState) { m.Ready = v == "1" })
		case strings.HasPrefix(k, "s:"):
			member(k[2:], func(m *party.MemberState) { m.Status = v })
		case strings.HasPrefix(k, "g:"):
			member(k[2:], func(m *party.MemberState) { m.GoneAt = at(v) })
		case k == "leader_gone":
			l.LeaderGoneAt = at(v)
		case k == "below_min":
			l.BelowMinSince = at(v)
		case k == "last_online":
			if t := at(v); t != nil {
				l.LastOnline = *t
			}
		}
	}
	return l, nil
}

// SetReady implements port.LobbyStore.
func (s LobbyStore) SetReady(ctx context.Context, partyID, student uuid.UUID, ready bool) error {
	key := PartyLobbyKey(partyID)
	var cmd valkey.Completed
	if ready {
		cmd = s.C.B().Hset().Key(key).FieldValue().FieldValue("r:"+student.String(), "1").Build()
	} else {
		cmd = s.C.B().Hdel().Key(key).Field("r:" + student.String()).Build()
	}
	for _, r := range s.C.DoMulti(ctx, cmd, s.C.B().Expire().Key(key).Seconds(ttlSeconds()).Build()) {
		if err := r.Error(); err != nil && !valkey.IsValkeyNil(err) {
			return fmt.Errorf("party lobby: ready: %w", err)
		}
	}
	return nil
}

// resetReady deletes every r:* field of the lobby hash.
var resetReady = valkey.NewLuaScript(`
local h = redis.call('HKEYS', KEYS[1])
for _, f in ipairs(h) do
  if string.sub(f, 1, 2) == 'r:' then redis.call('HDEL', KEYS[1], f) end
end
return 1`)

// ResetReady implements port.LobbyStore.
func (s LobbyStore) ResetReady(ctx context.Context, partyID uuid.UUID) error {
	if err := resetReady.Exec(ctx, s.C, []string{PartyLobbyKey(partyID)}, nil).Error(); err != nil && !valkey.IsValkeyNil(err) {
		return fmt.Errorf("party lobby: reset ready: %w", err)
	}
	return nil
}

// Save implements port.LobbyStore: rewrites the lobby hash (statuses, timers, Ready) in one script.
func (s LobbyStore) Save(ctx context.Context, partyID uuid.UUID, l party.Lobby) error {
	ms := func(t *time.Time) string {
		if t == nil {
			return ""
		}
		return strconv.FormatInt(t.UnixMilli(), 10)
	}
	args := []string{strconv.FormatInt(ttlSeconds(), 10)}
	add := func(k, v string) {
		if v != "" {
			args = append(args, k, v)
		}
	}
	for id, m := range l.Members {
		if m.Ready {
			add("r:"+id.String(), "1")
		}
		add("s:"+id.String(), m.Status)
		add("g:"+id.String(), ms(m.GoneAt))
	}
	add("leader_gone", ms(l.LeaderGoneAt))
	add("below_min", ms(l.BelowMinSince))
	if !l.LastOnline.IsZero() {
		add("last_online", strconv.FormatInt(l.LastOnline.UnixMilli(), 10))
	}
	if err := saveLobby.Exec(ctx, s.C, []string{PartyLobbyKey(partyID)}, args).Error(); err != nil && !valkey.IsValkeyNil(err) {
		return fmt.Errorf("party lobby: save: %w", err)
	}
	return nil
}

// saveLobby rewrites the hash: KEYS[1] lobby hash; ARGV[1] ttl s, ARGV[2..] field/value pairs.
var saveLobby = valkey.NewLuaScript(`
redis.call('DEL', KEYS[1])
if #ARGV > 1 then
  for i = 2, #ARGV, 2 do redis.call('HSET', KEYS[1], ARGV[i], ARGV[i + 1]) end
  redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return 1`)

// Drop implements port.LobbyStore.
func (s LobbyStore) Drop(ctx context.Context, tenant, partyID uuid.UUID) error {
	for _, r := range s.C.DoMulti(ctx, s.C.B().Del().Key(PartyLobbyKey(partyID)).Build(),
		s.C.B().Zrem().Key(PartyActiveKey).Member(activeMember(tenant, partyID)).Build()) {
		if err := r.Error(); err != nil && !valkey.IsValkeyNil(err) {
			return fmt.Errorf("party lobby: drop: %w", err)
		}
	}
	return nil
}

// Track implements port.LobbyStore (ZADD NX: the first tracking time stays).
func (s LobbyStore) Track(ctx context.Context, tenant, partyID uuid.UUID, at time.Time) error {
	if at.IsZero() {
		at = time.Now()
	}
	if err := s.C.Do(ctx, s.C.B().Zadd().Key(PartyActiveKey).Nx().ScoreMember().ScoreMember(float64(at.UnixMilli()),
		activeMember(tenant, partyID)).Build()).Error(); err != nil && !valkey.IsValkeyNil(err) {
		return fmt.Errorf("party lobby: track: %w", err)
	}
	return nil
}

// Tracked implements port.LobbyStore.
func (s LobbyStore) Tracked(ctx context.Context, limit int) ([]port.PartyRef, error) {
	ms, err := s.C.Do(ctx, s.C.B().Zrange().Key(PartyActiveKey).Min("0").Max(strconv.Itoa(limit-1)).Build()).AsStrSlice()
	if err != nil && !valkey.IsValkeyNil(err) {
		return nil, fmt.Errorf("party lobby: tracked: %w", err)
	}
	out := make([]port.PartyRef, 0, len(ms))
	for _, m := range ms {
		t, p, ok := strings.Cut(m, ":")
		if !ok {
			continue
		}
		tid, err1 := uuid.Parse(t)
		pid, err2 := uuid.Parse(p)
		if err1 != nil || err2 != nil {
			continue
		}
		out = append(out, port.PartyRef{TenantID: tid, PartyID: pid})
	}
	return out, nil
}

// allow counts one command in the window of the current second.
var allow = valkey.NewLuaScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then redis.call('PEXPIRE', KEYS[1], 1000) end
if n > tonumber(ARGV[1]) then return redis.call('PTTL', KEYS[1]) end
return -1`)

// Allow implements port.LobbyStore (party_lobby_cmd_rate per student and second).
func (s LobbyStore) Allow(ctx context.Context, student uuid.UUID, perSecond int) (bool, time.Duration, error) {
	n, err := allow.Exec(ctx, s.C, []string{LobbyRateKey(student)}, []string{strconv.Itoa(perSecond)}).AsInt64()
	if err != nil {
		return true, 0, fmt.Errorf("party lobby: rate: %w", err)
	}
	if n < 0 {
		return true, 0, nil
	}
	return false, max(time.Duration(n)*time.Millisecond, time.Second), nil
}

// Acquire implements port.JobLock (SET NX PX: the lease expires by itself).
func (s LobbyStore) Acquire(ctx context.Context, name string, ttl time.Duration) (bool, error) {
	err := s.C.Do(ctx, s.C.B().Set().Key(JobLockKey(name)).Value(strconv.FormatInt(time.Now().UnixMilli(), 10)).Nx().
		Px(max(ttl, 100*time.Millisecond)).Build()).Error()
	if valkey.IsValkeyNil(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("job lock %s: %w", name, err)
	}
	return true, nil
}

// Statuses implements port.PresenceStatuses on ew:party:presence:<user_id> ("" when no signal or expired).
func (p PartyPresence) Statuses(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := make(map[uuid.UUID]string, len(ids))
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
		out[ids[i]] = s
	}
	return out, nil
}

var _ port.PresenceStatuses = PartyPresence{}
