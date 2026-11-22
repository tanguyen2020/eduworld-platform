package cache

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
)

// Keys of the read receipts (VALKEY_DESIGN §3.1, owner chat): the window gate and the buffered newest seq of a member.
func cursorGateKey(channel, user uuid.UUID) string {
	return "ew:chat:cursor_gate:" + channel.String() + ":" + user.String()
}
func cursorBufKey(channel, user uuid.UUID) string {
	return "ew:chat:cursor_buf:" + channel.String() + ":" + user.String()
}

// cursorHold keeps the highest seq (sorted set, ZADD GT) and answers 1 for the first held seq of the window
// (KEYS[1] buffer, ARGV[1] seq, ARGV[2] window ms).
var cursorHold = valkey.NewLuaScript(`
local first = redis.call('EXISTS', KEYS[1]) == 0
redis.call('ZADD', KEYS[1], 'GT', ARGV[1], 'seq')
redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[2]) * 5)
if first then return 1 end
return 0`)

// Cursor implements port.CursorBuffer (ew:chat:cursor_gate, ew:chat:cursor_buf). Valkey only coalesces: the cursor of
// record is chat.channel_members.last_read_seq (ADR-0004).
type Cursor struct{ C valkey.Client }

var _ port.CursorBuffer = Cursor{}

// Gate implements port.CursorBuffer.
func (c Cursor) Gate(ctx context.Context, channel, user uuid.UUID, window time.Duration) (bool, error) {
	err := c.C.Do(ctx, c.C.B().Set().Key(cursorGateKey(channel, user)).Value("1").Nx().Px(window).Build()).Error()
	if valkey.IsValkeyNil(err) {
		return false, nil
	}
	if err != nil {
		return false, unavailable(err)
	}
	return true, nil
}

// Hold implements port.CursorBuffer.
func (c Cursor) Hold(ctx context.Context, channel, user uuid.UUID, seq int64, window time.Duration) (bool, error) {
	n, err := cursorHold.Exec(ctx, c.C, []string{cursorBufKey(channel, user)},
		[]string{strconv.FormatInt(seq, 10), strconv.FormatInt(window.Milliseconds(), 10)}).AsInt64()
	if err != nil {
		return false, unavailable(err)
	}
	return n == 1, nil
}

// Take implements port.CursorBuffer.
func (c Cursor) Take(ctx context.Context, channel, user uuid.UUID) (int64, bool, error) {
	zs, err := c.C.Do(ctx, c.C.B().Zpopmax().Key(cursorBufKey(channel, user)).Build()).AsZScores()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return 0, false, nil
		}
		return 0, false, unavailable(err)
	}
	if len(zs) == 0 {
		return 0, false, nil
	}
	return int64(zs[0].Score), true, nil
}

// Expire sets a TTL on the seq counter of a closed channel (no message is accepted any more: CHAT_CHANNEL_CLOSED).
func (s Seq) Expire(ctx context.Context, channel uuid.UUID, ttl time.Duration) error {
	return s.C.Do(ctx, s.C.B().Pexpire().Key(seqKey(channel)).Milliseconds(ttl.Milliseconds()).Build()).Error()
}
