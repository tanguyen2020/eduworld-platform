package cache

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
)

// UnreadTTL is the TTL of ew:chat:ntf:unread:<user_id> (data-model F123: 7 days; lost → recounted from PostgreSQL).
const UnreadTTL = 7 * 24 * time.Hour

func unreadKey(user uuid.UUID) string { return "ew:chat:ntf:unread:" + user.String() }

// unreadAdd moves a cached count by ARGV[1] when it exists; a negative result drops the key (recounted at the next read).
var unreadAdd = valkey.NewLuaScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then return -1 end
local n = redis.call('INCRBY', KEYS[1], ARGV[1])
if n < 0 then redis.call('DEL', KEYS[1]) return -1 end
return n`)

// Unread implements port.UnreadCounter: STRING ew:chat:ntf:unread:<user_id> with TTL (VALKEY_DESIGN §3.1). Only a cache:
// PostgreSQL keeps the read state (R9, F123-AC-12).
type Unread struct {
	C   valkey.Client
	TTL time.Duration
}

var _ port.UnreadCounter = Unread{}

func (u Unread) ttl() time.Duration {
	if u.TTL <= 0 {
		return UnreadTTL
	}
	return u.TTL
}

// Get implements port.UnreadCounter.
func (u Unread) Get(ctx context.Context, user uuid.UUID) (int, bool, error) {
	n, err := u.C.Do(ctx, u.C.B().Get().Key(unreadKey(user)).Build()).AsInt64()
	if valkey.IsValkeyNil(err) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if n < 0 {
		return 0, false, nil
	}
	return int(n), true, nil
}

// Fill implements port.UnreadCounter (SET NX EX).
func (u Unread) Fill(ctx context.Context, user uuid.UUID, n int) error {
	err := u.C.Do(ctx, u.C.B().Set().Key(unreadKey(user)).Value(strconv.Itoa(max(n, 0))).Nx().Ex(u.ttl()).Build()).Error()
	if valkey.IsValkeyNil(err) {
		return nil
	}
	return err
}

// Add implements port.UnreadCounter.
func (u Unread) Add(ctx context.Context, user uuid.UUID, delta int) error {
	if delta == 0 {
		return nil
	}
	err := unreadAdd.Exec(ctx, u.C, []string{unreadKey(user)}, []string{strconv.Itoa(delta)}).Error()
	if err != nil && !errors.Is(err, valkey.Nil) {
		return err
	}
	return nil
}

// Set implements port.UnreadCounter (SET EX).
func (u Unread) Set(ctx context.Context, user uuid.UUID, n int) error {
	return u.C.Do(ctx, u.C.B().Set().Key(unreadKey(user)).Value(strconv.Itoa(max(n, 0))).Ex(u.ttl()).Build()).Error()
}
