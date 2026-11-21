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

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
)

// Keys of the chat core (VALKEY_DESIGN §3.1, owner chat · chat). Valkey is never the source of truth (ADR-0004): seq is
// rebuilt from PostgreSQL, the recent buffer and records are ephemeral, cached facts are rebuildable.
func seqKey(channel uuid.UUID) string    { return "ew:chat:seq:" + channel.String() }
func recentKey(channel uuid.UUID) string { return "ew:chat:recent:" + channel.String() }
func cmidKey(sender, cmid uuid.UUID) string {
	return "ew:chat:cmid:" + sender.String() + ":" + cmid.String()
}
func rateUserKey(user uuid.UUID) string { return "ew:chat:rl:" + user.String() + ":60s" }
func rateChannelKey(user, channel uuid.UUID) string {
	return "ew:chat:rl:" + user.String() + ":" + channel.String() + ":60s"
}
func factsKey(key string) string { return "ew:chat:facts:" + key }

func unavailable(err error) error { return fmt.Errorf("%w: %w", port.ErrUnavailable, err) }

// seqNext is seq_next.lua (VALKEY_DESIGN §4.9): initialise a missing counter with the floor read from PostgreSQL, then
// INCR. With ARGV[1] = "" the script only answers -1 when the key is missing (the floor is read only then).
var seqNext = valkey.NewLuaScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then
  if ARGV[1] == '' then return -1 end
  redis.call('SET', KEYS[1], ARGV[1], 'NX')
end
return redis.call('INCR', KEYS[1])`)

// Seq implements port.SeqAllocator on ew:chat:seq:<channel_id> (no TTL; F117 expires it after the channel closes).
type Seq struct{ C valkey.Client }

var _ port.SeqAllocator = Seq{}

// Next implements port.SeqAllocator: the floor is read only when the counter is missing (F115-AC-20).
func (s Seq) Next(ctx context.Context, channel uuid.UUID, floor func(ctx context.Context) (int64, error)) (int64, error) {
	key := seqKey(channel)
	n, err := seqNext.Exec(ctx, s.C, []string{key}, []string{""}).AsInt64()
	if err != nil {
		return 0, unavailable(err)
	}
	if n >= 0 {
		return n, nil
	}
	f, err := floor(ctx)
	if err != nil {
		return 0, err
	}
	n, err = seqNext.Exec(ctx, s.C, []string{key}, []string{strconv.FormatInt(f, 10)}).AsInt64()
	if err != nil {
		return 0, unavailable(err)
	}
	return n, nil
}

// Recent implements port.RecentBuffer: LIST ew:chat:recent:<channel_id>, newest first, LTRIM to chat_recent_buffer,
// TTL chat_recent_buffer renewed on every write.
type Recent struct {
	C    valkey.Client
	Size int
	TTL  time.Duration
}

var _ port.RecentBuffer = Recent{}

// Push implements port.RecentBuffer.
func (r Recent) Push(ctx context.Context, e message.Envelope) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	key := recentKey(e.ChannelID)
	cmds := valkey.Commands{
		r.C.B().Lpush().Key(key).Element(string(b)).Build(),
		r.C.B().Ltrim().Key(key).Start(0).Stop(int64(r.Size - 1)).Build(),
		r.C.B().Expire().Key(key).Seconds(int64(max(r.TTL/time.Second, 1))).Build(),
	}
	for _, res := range r.C.DoMulti(ctx, cmds...) {
		if err := res.Error(); err != nil {
			return unavailable(err)
		}
	}
	return nil
}

// Drop deletes the recent buffer of a channel (a message removed by the Safety Team is never served from Valkey;
// the next history read falls back to PostgreSQL).
func (r Recent) Drop(ctx context.Context, channel uuid.UUID) error {
	return r.C.Do(ctx, r.C.B().Del().Key(recentKey(channel)).Build()).Error()
}

// List implements port.RecentBuffer (invalid entries are skipped).
func (r Recent) List(ctx context.Context, channel uuid.UUID) ([]message.Envelope, error) {
	vals, err := r.C.Do(ctx, r.C.B().Lrange().Key(recentKey(channel)).Start(0).Stop(int64(r.Size-1)).Build()).AsStrSlice()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return nil, nil
		}
		return nil, unavailable(err)
	}
	out := make([]message.Envelope, 0, len(vals))
	for _, v := range vals {
		var e message.Envelope
		if json.Unmarshal([]byte(v), &e) == nil && e.Check() == nil && e.ChannelID == channel {
			out = append(out, e)
		}
	}
	return out, nil
}

// rateCheck is the sliding window of VALKEY_DESIGN §4.5 over two ZSETs (channel and user) plus, on the channel ZSET,
// the slow mode (time since the last message of the sender) and the burst (messages within the burst window; F116).
// Every limit is checked before the message is counted, so a refused send consumes nothing. Returns {ok, retry_ms}.
// ARGV: now_ms, msg, per_channel, per_user, burst, burst_window_ms, slow_ms.
var rateCheck = valkey.NewLuaScript(`
local now = tonumber(ARGV[1])
local slow = tonumber(ARGV[7])
local keep = math.max(60000, slow)
redis.call('ZREMRANGEBYSCORE', KEYS[1], 0, now - keep)
redis.call('ZREMRANGEBYSCORE', KEYS[2], 0, now - 60000)
if slow > 0 then
  local last = redis.call('ZREVRANGE', KEYS[1], 0, 0, 'WITHSCORES')
  if #last == 2 and now - tonumber(last[2]) < slow then
    return {0, tonumber(last[2]) + slow - now}
  end
end
local minute = string.format('(%d', now - 60000)
local limits = {tonumber(ARGV[3]), tonumber(ARGV[4])}
for i = 1, 2 do
  if limits[i] > 0 and redis.call('ZCOUNT', KEYS[i], minute, '+inf') >= limits[i] then
    local oldest = redis.call('ZRANGEBYSCORE', KEYS[i], minute, '+inf', 'WITHSCORES', 'LIMIT', 0, 1)
    return {0, tonumber(oldest[2]) + 60000 - now}
  end
end
local burst, bw = tonumber(ARGV[5]), tonumber(ARGV[6])
if burst > 0 and bw > 0 then
  local from = string.format('(%d', now - bw)
  if redis.call('ZCOUNT', KEYS[1], from, '+inf') >= burst then
    local oldest = redis.call('ZRANGEBYSCORE', KEYS[1], from, '+inf', 'WITHSCORES', 'LIMIT', 0, 1)
    return {0, tonumber(oldest[2]) + bw - now}
  end
end
redis.call('ZADD', KEYS[1], now, ARGV[2])
redis.call('ZADD', KEYS[2], now, ARGV[2])
redis.call('EXPIRE', KEYS[1], math.ceil(keep / 1000) + 1)
redis.call('EXPIRE', KEYS[2], 61)
return {1, 0}`)

// Rate implements port.RateLimiter (ew:chat:rl:*, VALKEY_DESIGN §4.5).
type Rate struct{ C valkey.Client }

var _ port.RateLimiter = Rate{}

// Allow implements port.RateLimiter.
func (r Rate) Allow(ctx context.Context, user, channel, msg uuid.UUID, l port.RateLimits, now time.Time) (bool, time.Duration, error) {
	ms := now.UnixMilli()
	res, err := rateCheck.Exec(ctx, r.C, []string{rateChannelKey(user, channel), rateUserKey(user)},
		[]string{strconv.FormatInt(ms, 10), msg.String(), strconv.Itoa(l.PerChannel), strconv.Itoa(l.PerUser), strconv.Itoa(l.Burst),
			strconv.FormatInt(l.BurstWindow.Milliseconds(), 10), strconv.FormatInt(l.SlowMode.Milliseconds(), 10)}).ToArray()
	if err != nil || len(res) != 2 {
		return false, 0, unavailable(errors.Join(err, errors.New("rate script")))
	}
	ok, _ := res[0].AsInt64()
	if ok == 1 {
		return true, 0, nil
	}
	retry, _ := res[1].AsInt64()
	return false, max(time.Duration(retry)*time.Millisecond, time.Second), nil
}

// Dedupe implements port.SendDedupe on ew:chat:cmid:<sender>:<client_message_id> (SET NX, TTL 10 min, CHAT_SYSTEM §5.2).
type Dedupe struct {
	C   valkey.Client
	TTL time.Duration
}

var _ port.SendDedupe = Dedupe{}

// Reserve implements port.SendDedupe.
func (d Dedupe) Reserve(ctx context.Context, sender, cmid uuid.UUID) (port.SendRecord, bool, error) {
	key := cmidKey(sender, cmid)
	b, _ := json.Marshal(port.SendRecord{})
	err := d.C.Do(ctx, d.C.B().Set().Key(key).Value(string(b)).Nx().Ex(d.TTL).Build()).Error()
	if err == nil {
		return port.SendRecord{}, true, nil
	}
	if !valkey.IsValkeyNil(err) {
		return port.SendRecord{}, false, unavailable(err)
	}
	raw, err := d.C.Do(ctx, d.C.B().Get().Key(key).Build()).ToString()
	if err != nil {
		if valkey.IsValkeyNil(err) { // expired in between: reserve again
			return d.Reserve(ctx, sender, cmid)
		}
		return port.SendRecord{}, false, unavailable(err)
	}
	var r port.SendRecord
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return port.SendRecord{}, false, unavailable(err)
	}
	return r, false, nil
}

// Save implements port.SendDedupe (keeps the TTL of the reservation).
func (d Dedupe) Save(ctx context.Context, sender, cmid uuid.UUID, r port.SendRecord) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err := d.C.Do(ctx, d.C.B().Set().Key(cmidKey(sender, cmid)).Value(string(b)).Ex(d.TTL).Build()).Error(); err != nil {
		return unavailable(err)
	}
	return nil
}

// Release implements port.SendDedupe.
func (d Dedupe) Release(ctx context.Context, sender, cmid uuid.UUID) error {
	if err := d.C.Do(ctx, d.C.B().Del().Key(cmidKey(sender, cmid)).Build()).Error(); err != nil {
		return unavailable(err)
	}
	return nil
}

// Facts implements port.Cache: STRING ew:chat:facts:<key> = {"at": epoch ms, "v": value}, kept StaleTTL so the last
// known value survives a PostgreSQL outage (F115-AC-16); a value older than fresh is reloaded first.
type Facts struct {
	C        valkey.Client
	StaleTTL time.Duration
	Now      func() time.Time
	OnStale  func() // metric hook: a stale value was served
}

var _ port.Cache = Facts{}

type factsEntry struct {
	At int64           `json:"at"`
	V  json.RawMessage `json:"v"`
}

func (f Facts) now() time.Time {
	if f.Now != nil {
		return f.Now()
	}
	return time.Now()
}

// Get implements port.Cache. When Valkey itself fails the loader still answers (the facts are rebuildable).
func (f Facts) Get(ctx context.Context, key string, fresh time.Duration, load func(ctx context.Context) ([]byte, error)) ([]byte, error) {
	k := factsKey(key)
	var cached *factsEntry
	if raw, err := f.C.Do(ctx, f.C.B().Get().Key(k).Build()).ToString(); err == nil {
		var e factsEntry
		if json.Unmarshal([]byte(raw), &e) == nil && len(e.V) > 0 {
			cached = &e
		}
	}
	now := f.now()
	if cached != nil && now.Sub(time.UnixMilli(cached.At)) < fresh {
		return cached.V, nil
	}
	v, err := load(ctx)
	if err != nil {
		if cached != nil {
			if f.OnStale != nil {
				f.OnStale()
			}
			return cached.V, nil
		}
		return nil, err
	}
	b, _ := json.Marshal(factsEntry{At: now.UnixMilli(), V: v})
	_ = f.C.Do(ctx, f.C.B().Set().Key(k).Value(string(b)).Ex(f.StaleTTL).Build()).Error()
	return v, nil
}

// Invalidate implements port.Cache.
func (f Facts) Invalidate(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	ks := make([]string, len(keys))
	for i, k := range keys {
		ks[i] = factsKey(k)
	}
	return f.C.Do(ctx, f.C.B().Del().Key(ks...).Build()).Error()
}
