package cache

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
)

// Keys of the F118 live state (VALKEY_DESIGN §3.1, owner chat). Valkey is never the source of truth (ADR-0004): every
// key is rebuilt from PostgreSQL when missing, except the typing set which exists only here (CHAT_SYSTEM §7.10).
func msgIndexKey(id uuid.UUID) string  { return "ew:chat:msgidx:" + id.String() }
func msgRevKey(id uuid.UUID) string    { return "ew:chat:msgrev:" + id.String() }
func reactionsKey(id uuid.UUID) string { return "ew:chat:rx:" + id.String() }
func pinsKey(ch uuid.UUID) string      { return "ew:chat:pins:" + ch.String() }
func typingKey(ch uuid.UUID) string    { return "ew:chat:typing:" + ch.String() }
func assetRateKey(user, ch uuid.UUID) string {
	return "ew:chat:rl:asset:" + user.String() + ":" + ch.String()
}
func featureStateTTL() time.Duration                { return 48 * time.Hour }
func reactionField(user uuid.UUID, e string) string { return user.String() + "|" + e }

// FeatureState implements port.MessageIndex, port.RevisionGate, port.ReactionState, port.PinState, port.TypingState and
// port.ExtraRate on Valkey.
type FeatureState struct {
	C valkey.Client
	// IndexTTL is the life of a message index entry (chat_recall_window + margin; default 25 h).
	IndexTTL time.Duration
}

var (
	_ port.MessageIndex  = FeatureState{}
	_ port.RevisionGate  = FeatureState{}
	_ port.ReactionState = FeatureState{}
	_ port.PinState      = FeatureState{}
	_ port.TypingState   = FeatureState{}
	_ port.ExtraRate     = FeatureState{}
)

// Put implements port.MessageIndex.
func (s FeatureState) Put(ctx context.Context, msg, tenant, channel uuid.UUID) error {
	ttl := s.IndexTTL
	if ttl <= 0 {
		ttl = 25 * time.Hour
	}
	if err := s.C.Do(ctx, s.C.B().Set().Key(msgIndexKey(msg)).Value(tenant.String()+":"+channel.String()).Px(ttl).Build()).Error(); err != nil {
		return unavailable(err)
	}
	return nil
}

// Get implements port.MessageIndex.
func (s FeatureState) Get(ctx context.Context, msg uuid.UUID) (uuid.UUID, uuid.UUID, bool, error) {
	v, err := s.C.Do(ctx, s.C.B().Get().Key(msgIndexKey(msg)).Build()).ToString()
	if valkey.IsValkeyNil(err) {
		return uuid.Nil, uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, uuid.Nil, false, unavailable(err)
	}
	a, b, ok := strings.Cut(v, ":")
	t, okA := parseID(a)
	c, okB := parseID(b)
	if !ok || !okA || !okB { // a malformed entry is a missing one
		return uuid.Nil, uuid.Nil, false, nil
	}
	return t, c, true, nil
}

// revAdvance: KEYS[1] revision; ARGV expected, floor, ttl ms → {ok, current}.
var revAdvance = valkey.NewLuaScript(`
local cur = tonumber(redis.call('GET', KEYS[1]) or '0')
local floor = tonumber(ARGV[2])
if floor > cur then cur = floor end
if cur ~= tonumber(ARGV[1]) then return {0, cur} end
redis.call('SET', KEYS[1], cur + 1, 'PX', ARGV[3])
return {1, cur + 1}`)

// revRestore: KEYS[1]; ARGV from, to — back to `to` only while the value is still `from`.
var revRestore = valkey.NewLuaScript(`
if tonumber(redis.call('GET', KEYS[1]) or '0') == tonumber(ARGV[1]) then
  redis.call('SET', KEYS[1], ARGV[2], 'KEEPTTL')
  return 1
end
return 0`)

// Advance implements port.RevisionGate.
func (s FeatureState) Advance(ctx context.Context, msg uuid.UUID, expected, floor int) (bool, int, error) {
	res, err := revAdvance.Exec(ctx, s.C, []string{msgRevKey(msg)}, []string{strconv.Itoa(expected), strconv.Itoa(floor),
		strconv.FormatInt(featureStateTTL().Milliseconds(), 10)}).ToArray()
	if err != nil || len(res) != 2 {
		return false, 0, unavailable(errors.Join(err, errors.New("revision script")))
	}
	ok, _ := res[0].AsInt64()
	cur, _ := res[1].AsInt64()
	return ok == 1, int(cur), nil
}

// Restore implements port.RevisionGate.
func (s FeatureState) Restore(ctx context.Context, msg uuid.UUID, from, to int) error {
	if err := revRestore.Exec(ctx, s.C, []string{msgRevKey(msg)}, []string{strconv.Itoa(from), strconv.Itoa(to)}).Error(); err != nil {
		return unavailable(err)
	}
	return nil
}

// rxToggle: KEYS[1] hash user|emoji → 1 (field "_" marks a loaded key); ARGV user, emoji, limit, ttl ms, then the
// initial fields when the key is missing. Returns {state (1 set, 0 unset, -1 limited), field...}.
var rxToggle = valkey.NewLuaScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then
  redis.call('HSET', KEYS[1], '_', 1)
  for i = 5, #ARGV do redis.call('HSET', KEYS[1], ARGV[i], 1) end
end
local f = ARGV[1] .. '|' .. ARGV[2]
local state
if redis.call('HEXISTS', KEYS[1], f) == 1 then
  redis.call('HDEL', KEYS[1], f)
  state = 0
else
  local mine = 0
  for _, k in ipairs(redis.call('HKEYS', KEYS[1])) do
    if string.sub(k, 1, #ARGV[1] + 1) == ARGV[1] .. '|' then mine = mine + 1 end
  end
  if mine >= tonumber(ARGV[3]) then
    state = -1
  else
    redis.call('HSET', KEYS[1], f, 1)
    state = 1
  end
end
redis.call('PEXPIRE', KEYS[1], ARGV[4])
local out = {state}
for _, k in ipairs(redis.call('HKEYS', KEYS[1])) do
  if k ~= '_' then table.insert(out, k) end
end
return out`)

// Toggle implements port.ReactionState.
func (s FeatureState) Toggle(ctx context.Context, msg, user uuid.UUID, emoji string, limit int,
	load func(ctx context.Context) ([]port.Reaction, error)) (port.ReactionResult, error) {
	args := []string{user.String(), emoji, strconv.Itoa(limit), strconv.FormatInt(featureStateTTL().Milliseconds(), 10)}
	n, err := s.C.Do(ctx, s.C.B().Exists().Key(reactionsKey(msg)).Build()).AsInt64()
	if err != nil {
		return port.ReactionResult{}, unavailable(err)
	}
	if n == 0 {
		rs, err := load(ctx)
		if err != nil {
			return port.ReactionResult{}, err
		}
		for _, r := range rs {
			args = append(args, reactionField(r.UserID, r.Emoji))
		}
	}
	res, err := rxToggle.Exec(ctx, s.C, []string{reactionsKey(msg)}, args).ToArray()
	if err != nil || len(res) == 0 {
		return port.ReactionResult{}, unavailable(errors.Join(err, errors.New("reaction script")))
	}
	state, _ := res[0].AsInt64()
	out := port.ReactionResult{Set: state == 1, Limited: state == -1, Counts: map[string]int{}}
	for _, v := range res[1:] {
		f, _ := v.ToString()
		u, e, ok := strings.Cut(f, "|")
		if !ok {
			continue
		}
		out.Counts[e]++
		if u == user.String() {
			out.Mine = append(out.Mine, e)
		}
	}
	sort.Strings(out.Mine)
	return out, nil
}

// Undo implements port.ReactionState.
func (s FeatureState) Undo(ctx context.Context, msg, user uuid.UUID, emoji string, set bool) error {
	f := reactionField(user, emoji)
	var cmd valkey.Completed
	if set {
		cmd = s.C.B().Hdel().Key(reactionsKey(msg)).Field(f).Build()
	} else {
		cmd = s.C.B().Hset().Key(reactionsKey(msg)).FieldValue().FieldValue(f, "1").Build()
	}
	if err := s.C.Do(ctx, cmd).Error(); err != nil {
		return unavailable(err)
	}
	return nil
}

// pinAdd: KEYS[1] set of message ids ("_" marks a loaded key); ARGV msg, limit, ttl ms, initial ids.
// Returns {state (1 added, 0 already, -1 full), id...}.
var pinAdd = valkey.NewLuaScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then
  redis.call('SADD', KEYS[1], '_')
  for i = 4, #ARGV do redis.call('SADD', KEYS[1], ARGV[i]) end
end
local state
if redis.call('SISMEMBER', KEYS[1], ARGV[1]) == 1 then
  state = 0
elseif redis.call('SCARD', KEYS[1]) - 1 >= tonumber(ARGV[2]) then
  state = -1
else
  redis.call('SADD', KEYS[1], ARGV[1])
  state = 1
end
redis.call('PEXPIRE', KEYS[1], ARGV[3])
local out = {state}
for _, k in ipairs(redis.call('SMEMBERS', KEYS[1])) do
  if k ~= '_' then table.insert(out, k) end
end
return out`)

var pinRemove = valkey.NewLuaScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then
  redis.call('SADD', KEYS[1], '_')
  for i = 3, #ARGV do redis.call('SADD', KEYS[1], ARGV[i]) end
end
local state = redis.call('SREM', KEYS[1], ARGV[1])
redis.call('PEXPIRE', KEYS[1], ARGV[2])
local out = {state}
for _, k in ipairs(redis.call('SMEMBERS', KEYS[1])) do
  if k ~= '_' then table.insert(out, k) end
end
return out`)

func (s FeatureState) pinArgs(ctx context.Context, channel uuid.UUID, load func(ctx context.Context) ([]uuid.UUID, error), args []string) ([]string, error) {
	n, err := s.C.Do(ctx, s.C.B().Exists().Key(pinsKey(channel)).Build()).AsInt64()
	if err != nil {
		return nil, unavailable(err)
	}
	if n == 0 {
		ids, err := load(ctx)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			args = append(args, id.String())
		}
	}
	return args, nil
}

func pinIDs(res []valkey.ValkeyMessage) (int64, []uuid.UUID) {
	state, _ := res[0].AsInt64()
	var ids []uuid.UUID
	for _, v := range res[1:] {
		sv, _ := v.ToString()
		if id, err := uuid.Parse(sv); err == nil {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	return state, ids
}

// Add implements port.PinState.
func (s FeatureState) Add(ctx context.Context, channel, msg uuid.UUID, limit int, load func(ctx context.Context) ([]uuid.UUID, error)) (bool, bool, []uuid.UUID, error) {
	args, err := s.pinArgs(ctx, channel, load, []string{msg.String(), strconv.Itoa(limit), strconv.FormatInt(featureStateTTL().Milliseconds(), 10)})
	if err != nil {
		return false, false, nil, err
	}
	res, err := pinAdd.Exec(ctx, s.C, []string{pinsKey(channel)}, args).ToArray()
	if err != nil || len(res) == 0 {
		return false, false, nil, unavailable(errors.Join(err, errors.New("pin script")))
	}
	state, ids := pinIDs(res)
	return state == 1, state == -1, ids, nil
}

// Remove implements port.PinState.
func (s FeatureState) Remove(ctx context.Context, channel, msg uuid.UUID, load func(ctx context.Context) ([]uuid.UUID, error)) (bool, []uuid.UUID, error) {
	args, err := s.pinArgs(ctx, channel, load, []string{msg.String(), strconv.FormatInt(featureStateTTL().Milliseconds(), 10)})
	if err != nil {
		return false, nil, err
	}
	res, err := pinRemove.Exec(ctx, s.C, []string{pinsKey(channel)}, args).ToArray()
	if err != nil || len(res) == 0 {
		return false, nil, unavailable(errors.Join(err, errors.New("pin script")))
	}
	state, ids := pinIDs(res)
	return state == 1, ids, nil
}

// typingStart: KEYS[1] activity hash (user → ms), KEYS[2] broadcast hash (user → ms of the last broadcast);
// ARGV user, now ms, throttle ms, ttl ms. Returns 1 when a broadcast is due (throttle passed), 0 otherwise.
var typingStart = valkey.NewLuaScript(`
local now, throttle, ttl = tonumber(ARGV[2]), tonumber(ARGV[3]), tonumber(ARGV[4])
redis.call('HSET', KEYS[1], ARGV[1], ARGV[2])
redis.call('PEXPIRE', KEYS[1], ttl * 2)
local last = tonumber(redis.call('HGET', KEYS[2], ARGV[1]) or '0')
if now - last < throttle then return 0 end
redis.call('HSET', KEYS[2], ARGV[1], ARGV[2])
redis.call('PEXPIRE', KEYS[2], ttl * 2)
return 1`)

// Start implements port.TypingState: at most one broadcast per throttle per member (server clock of the caller).
func (s FeatureState) Start(ctx context.Context, channel, user uuid.UUID, now time.Time, throttle, ttl time.Duration) (bool, []uuid.UUID, error) {
	n, err := typingStart.Exec(ctx, s.C, []string{typingKey(channel), typingKey(channel) + ":b"}, []string{user.String(),
		strconv.FormatInt(now.UnixMilli(), 10), strconv.FormatInt(throttle.Milliseconds(), 10), strconv.FormatInt(ttl.Milliseconds(), 10)}).AsInt64()
	if err != nil {
		return false, nil, unavailable(err)
	}
	if n != 1 {
		return false, nil, nil
	}
	typing, err := s.typing(ctx, channel, now, ttl)
	return true, typing, err
}

// Stop implements port.TypingState.
func (s FeatureState) Stop(ctx context.Context, channel, user uuid.UUID, now time.Time, ttl time.Duration) (bool, []uuid.UUID, error) {
	res := s.C.DoMulti(ctx, s.C.B().Hdel().Key(typingKey(channel)).Field(user.String()).Build(),
		s.C.B().Hdel().Key(typingKey(channel)+":b").Field(user.String()).Build())
	n, err := res[0].AsInt64()
	if err != nil {
		return false, nil, unavailable(err)
	}
	if n == 0 {
		return false, nil, nil
	}
	typing, err := s.typing(ctx, channel, now, ttl)
	return true, typing, err
}

// typing lists the members typing within ttl (older entries are dropped).
func (s FeatureState) typing(ctx context.Context, channel uuid.UUID, now time.Time, ttl time.Duration) ([]uuid.UUID, error) {
	m, err := s.C.Do(ctx, s.C.B().Hgetall().Key(typingKey(channel)).Build()).AsStrMap()
	if err != nil && !valkey.IsValkeyNil(err) {
		return nil, unavailable(err)
	}
	type entry struct {
		id uuid.UUID
		at int64
	}
	var out []entry
	for k, v := range m {
		at, _ := strconv.ParseInt(v, 10, 64)
		id, err := uuid.Parse(k)
		if err != nil {
			continue
		}
		if now.UnixMilli()-at > ttl.Milliseconds() {
			_ = s.C.Do(ctx, s.C.B().Hdel().Key(typingKey(channel)).Field(k).Build()).Error()
			continue
		}
		out = append(out, entry{id, at})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].at < out[j].at })
	ids := make([]uuid.UUID, len(out))
	for i, e := range out {
		ids[i] = e.id
	}
	return ids, nil
}

// AllowAsset implements port.ExtraRate (sliding window of one minute per sender per channel).
func (s FeatureState) AllowAsset(ctx context.Context, user, channel, msg uuid.UUID, perMinute int, now time.Time) (bool, time.Duration, error) {
	if perMinute <= 0 {
		return true, 0, nil
	}
	res, err := rateCheck.Exec(ctx, s.C, []string{assetRateKey(user, channel), assetRateKey(user, channel) + ":u"},
		[]string{strconv.FormatInt(now.UnixMilli(), 10), msg.String(), strconv.Itoa(perMinute), "0", "0", "0", "0"}).ToArray()
	if err != nil || len(res) != 2 {
		return false, 0, unavailable(errors.Join(err, errors.New("asset rate script")))
	}
	ok, _ := res[0].AsInt64()
	if ok == 1 {
		return true, 0, nil
	}
	retry, _ := res[1].AsInt64()
	return false, max(time.Duration(retry)*time.Millisecond, time.Second), nil
}

func parseID(s string) (uuid.UUID, bool) {
	u, err := uuid.Parse(s)
	return u, err == nil
}
