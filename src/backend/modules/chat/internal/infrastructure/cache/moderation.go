package cache

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// Keys of the moderation pipeline (VALKEY_DESIGN, owner chat · F116). Fingerprints are hashes: no content in Valkey.
func spamRepeatKey(user, channel uuid.UUID, fp string) string {
	return "ew:chat:spam:rep:" + user.String() + ":" + channel.String() + ":" + fp
}
func spamChannelsKey(user uuid.UUID, fp string) string {
	return "ew:chat:spam:chn:" + user.String() + ":" + fp
}
func spamBlocksKey(user uuid.UUID) string { return "ew:chat:spam:blk:" + user.String() }
func sanctionKey(user uuid.UUID) string   { return "ew:chat:sanction:" + user.String() }

// Pub/Sub channels of the configuration changes (MODERATION §11.6, §12.2).
const (
	SensitiveVersionChannel = "ew:chat:sensitive:version"
	SwitchesChangedChannel  = "ew:chat:switches:changed"
)

// spamRecord: KEYS rep, chn; ARGV now_ms, member, channel, rep_window_ms, chn_window_ms → {repeats, channels}.
var spamRecord = valkey.NewLuaScript(`
local now = tonumber(ARGV[1])
redis.call('ZADD', KEYS[1], now, ARGV[2])
redis.call('ZREMRANGEBYSCORE', KEYS[1], 0, now - tonumber(ARGV[4]))
redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[4]) + 1000)
redis.call('ZADD', KEYS[2], now, ARGV[3])
redis.call('ZREMRANGEBYSCORE', KEYS[2], 0, now - tonumber(ARGV[5]))
redis.call('PEXPIRE', KEYS[2], tonumber(ARGV[5]) + 1000)
return {redis.call('ZCARD', KEYS[1]), redis.call('ZCARD', KEYS[2])}`)

// spamBlock: KEYS blk; ARGV now_ms, member, window_ms, threshold → count (the counter restarts once it reaches the
// threshold: one mute per series).
var spamBlock = valkey.NewLuaScript(`
local now = tonumber(ARGV[1])
redis.call('ZADD', KEYS[1], now, ARGV[2])
redis.call('ZREMRANGEBYSCORE', KEYS[1], 0, now - tonumber(ARGV[3]))
redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[3]) + 1000)
local n = redis.call('ZCARD', KEYS[1])
if n >= tonumber(ARGV[4]) then redis.call('DEL', KEYS[1]) end
return n`)

// SpamWindow implements port.SpamWindow.
type SpamWindow struct{ C valkey.Client }

var _ port.SpamWindow = SpamWindow{}

// Record implements port.SpamWindow.
func (s SpamWindow) Record(ctx context.Context, user, channel uuid.UUID, fp string, now time.Time) (moderation.SpamCounts, error) {
	res, err := spamRecord.Exec(ctx, s.C, []string{spamRepeatKey(user, channel, fp), spamChannelsKey(user, fp)},
		[]string{strconv.FormatInt(now.UnixMilli(), 10), ids.New().String(), channel.String(),
			strconv.FormatInt(moderation.SpamRepeatWindow.Milliseconds(), 10),
			strconv.FormatInt(moderation.SpamChannelWindow.Milliseconds(), 10)}).ToArray()
	if err != nil || len(res) != 2 {
		return moderation.SpamCounts{}, unavailable(err)
	}
	r, _ := res[0].AsInt64()
	c, _ := res[1].AsInt64()
	return moderation.SpamCounts{Repeats: int(r), Channels: int(c)}, nil
}

// Block implements port.SpamWindow.
func (s SpamWindow) Block(ctx context.Context, user uuid.UUID, now time.Time) (int, error) {
	n, err := spamBlock.Exec(ctx, s.C, []string{spamBlocksKey(user)}, []string{strconv.FormatInt(now.UnixMilli(), 10), ids.New().String(),
		strconv.FormatInt(moderation.SpamViolationWindow.Milliseconds(), 10), strconv.Itoa(moderation.SpamViolationCount)}).AsInt64()
	if err != nil {
		return 0, unavailable(err)
	}
	return int(n), nil
}

// Sanctions implements port.SanctionCache on ew:chat:sanction:<user_id> (JSON list, TTL until the first end).
type Sanctions struct{ C valkey.Client }

var _ port.SanctionCache = Sanctions{}

// Get implements port.SanctionCache.
func (s Sanctions) Get(ctx context.Context, user uuid.UUID) ([]port.Sanction, bool, error) {
	raw, err := s.C.Do(ctx, s.C.B().Get().Key(sanctionKey(user)).Build()).ToString()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return nil, false, nil
		}
		return nil, false, unavailable(err)
	}
	var list []port.Sanction
	ok := json.Unmarshal([]byte(raw), &list) == nil // a corrupt entry is a miss: PostgreSQL answers
	return list, ok, nil
}

// Set implements port.SanctionCache.
func (s Sanctions) Set(ctx context.Context, user uuid.UUID, list []port.Sanction, ttl time.Duration) error {
	if list == nil {
		list = []port.Sanction{}
	}
	b, err := json.Marshal(list)
	if err != nil {
		return err
	}
	return s.C.Do(ctx, s.C.B().Set().Key(sanctionKey(user)).Value(string(b)).Px(max(ttl, time.Second)).Build()).Error()
}

// Invalidate implements port.SanctionCache.
func (s Sanctions) Invalidate(ctx context.Context, user uuid.UUID) error {
	return s.C.Do(ctx, s.C.B().Del().Key(sanctionKey(user)).Build()).Error()
}

// Notifier publishes the configuration changes on Valkey Pub/Sub (best effort: the instances also reconcile every
// moderation_config_propagation_max / 2).
type Notifier struct{ C valkey.Client }

// CatalogPublished announces a new catalogue version (tenant nil: platform).
func (n Notifier) CatalogPublished(ctx context.Context, tenant *uuid.UUID, version int64) error {
	b, _ := json.Marshal(map[string]any{"tenant_id": tenant, "version": version})
	return n.C.Do(ctx, n.C.B().Publish().Channel(SensitiveVersionChannel).Message(string(b)).Build()).Error()
}

// SwitchesChanged announces a switch or category-setting change of tenant (nil: platform).
func (n Notifier) SwitchesChanged(ctx context.Context, tenant *uuid.UUID) error {
	b, _ := json.Marshal(map[string]any{"tenant_id": tenant})
	return n.C.Do(ctx, n.C.B().Publish().Channel(SwitchesChangedChannel).Message(string(b)).Build()).Error()
}

// Notification is a decoded change notification.
type Notification struct {
	Channel  string
	TenantID *uuid.UUID `json:"tenant_id"`
}

// Subscribe receives the change notifications until ctx ends (reconnecting); fn is called for each.
func Subscribe(ctx context.Context, c valkey.Client, fn func(Notification)) error {
	for ctx.Err() == nil {
		err := c.Receive(ctx, c.B().Subscribe().Channel(SensitiveVersionChannel, SwitchesChangedChannel).Build(),
			func(m valkey.PubSubMessage) {
				var n Notification
				_ = json.Unmarshal([]byte(m.Message), &n)
				n.Channel = m.Channel
				fn(n)
			})
		if ctx.Err() != nil {
			return nil //nolint:nilerr // shutdown: the subscription ends with the context
		}
		slog.WarnContext(ctx, "chat: moderation notifications lost, reconnecting (reconcile covers the gap)", "error", err)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Second):
		}
	}
	return nil
}

// JobLockKey is ew:lock:job:<name> (STRING NX PX): one replica runs a cycle of a job.
func JobLockKey(name string) string { return "ew:lock:job:" + name }

// JobLock implements port.JobLock on Valkey.
type JobLock struct{ C valkey.Client }

var _ port.JobLock = JobLock{}

// Acquire takes the lease of name for ttl (false: another replica holds it).
func (l JobLock) Acquire(ctx context.Context, name string, ttl time.Duration) (bool, error) {
	err := l.C.Do(ctx, l.C.B().Set().Key(JobLockKey(name)).Value(ids.New().String()).Nx().Px(ttl).Build()).Error()
	if valkey.IsValkeyNil(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
