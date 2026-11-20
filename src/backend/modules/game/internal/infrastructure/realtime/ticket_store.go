package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// TicketStore implements port.TicketStore with Valkey (F090 US1, VALKEY_DESIGN §1): tickets under
// ew:game:rt:ticket:<sha256> (SET NX EX, read once by the gateway with GETDEL), the ws_ticket_rate counter under
// ew:game:rl:ws_ticket:<user>:<window> (INCR + EXPIRE, fixed window).
type TicketStore struct {
	C       valkey.Client
	Timeout time.Duration // per call (default 1 s)
	Now     func() time.Time
}

var _ port.TicketStore = TicketStore{}

func (s TicketStore) ctx(ctx context.Context) (context.Context, context.CancelFunc) {
	t := s.Timeout
	if t <= 0 {
		t = time.Second
	}
	return context.WithTimeout(ctx, t)
}

func (s TicketStore) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// RateKey is the counter key of user in the window starting at start.
func RateKey(user uuid.UUID, start time.Time) string {
	return "ew:game:rl:ws_ticket:" + user.String() + ":" + strconv.FormatInt(start.Unix(), 10)
}

// Allow implements port.TicketStore.
func (s TicketStore) Allow(ctx context.Context, user uuid.UUID, limit int, window time.Duration) (bool, time.Duration, error) {
	if window <= 0 {
		window = time.Minute
	}
	now := s.now()
	start := now.Truncate(window)
	key := RateKey(user, start)
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	res := s.C.DoMulti(ctx, s.C.B().Incr().Key(key).Build(), s.C.B().Expire().Key(key).Seconds(int64(window/time.Second)+1).Build())
	n, err := res[0].AsInt64()
	if err != nil {
		return false, 0, fmt.Errorf("ticket rate: %w", err)
	}
	if err := res[1].Error(); err != nil {
		return false, 0, fmt.Errorf("ticket rate: %w", err)
	}
	if n > int64(limit) {
		after := start.Add(window).Sub(now)
		if after < time.Second {
			after = time.Second
		}
		return false, after, nil
	}
	return true, 0, nil
}

// ErrTicketExists means a ticket with the same hash is already stored (never expected with 256 random bits).
var ErrTicketExists = errors.New("realtime ticket already exists")

// Save implements port.TicketStore.
func (s TicketStore) Save(ctx context.Context, hash string, claims rt.TicketClaims, ttl time.Duration) error {
	b, err := json.Marshal(claims)
	if err != nil {
		return err
	}
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	err = s.C.Do(ctx, s.C.B().Set().Key(rt.TicketKey(hash)).Value(string(b)).Nx().ExSeconds(int64(ttl/time.Second)).Build()).Error()
	if valkey.IsValkeyNil(err) {
		return ErrTicketExists
	}
	if err != nil {
		return fmt.Errorf("ticket save: %w", err)
	}
	return nil
}
