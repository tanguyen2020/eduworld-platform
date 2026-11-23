package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
)

// GoalKey is the Valkey counter of a community goal in one scope (EVENT_SEASON_SYSTEM §6): a cache only — the
// contributions and community_goal_progress in PostgreSQL are the truth (R9).
func GoalKey(goal, scope uuid.UUID) string {
	return "ew:game:cgoal:" + goal.String() + ":" + scope.String()
}

// goalFlushKey throttles the realtime push of a goal scope (community_goal_progress_flush_seconds).
func goalFlushKey(goal, scope uuid.UUID) string {
	return "ew:game:cgoal:flush:" + goal.String() + ":" + scope.String()
}

// goalTTL keeps a counter after its event (rebuilt from PostgreSQL when it expires or is lost).
const goalTTL = 60 * 24 * time.Hour

// raiseScript sets the counter to ARGV[1] when it is missing or lower (concurrent commits never lower it).
var raiseScript = valkey.NewLuaScript(`local c = redis.call('GET', KEYS[1])
if (not c) or (tonumber(c) < tonumber(ARGV[1])) then redis.call('SET', KEYS[1], ARGV[1], 'EX', ARGV[2]) return 1 end
return 0`)

// GoalCounter implements port.GoalCounter on Valkey.
type GoalCounter struct{ C valkey.Client }

var _ port.GoalCounter = GoalCounter{}

// Get implements port.GoalCounter.
func (g GoalCounter) Get(ctx context.Context, goal, scope uuid.UUID) (int64, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, worldTimeout)
	defer cancel()
	n, err := g.C.Do(ctx, g.C.B().Get().Key(GoalKey(goal, scope)).Build()).AsInt64()
	if valkey.IsValkeyNil(err) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("goal counter: get: %w", err)
	}
	return n, true, nil
}

// Raise implements port.GoalCounter.
func (g GoalCounter) Raise(ctx context.Context, goal, scope uuid.UUID, total int64) error {
	ctx, cancel := context.WithTimeout(ctx, worldTimeout)
	defer cancel()
	err := raiseScript.Exec(ctx, g.C, []string{GoalKey(goal, scope)}, []string{strconv.FormatInt(total, 10),
		strconv.FormatInt(int64(goalTTL/time.Second), 10)}).Error()
	if err != nil {
		return fmt.Errorf("goal counter: raise: %w", err)
	}
	return nil
}

// Set implements port.GoalCounter.
func (g GoalCounter) Set(ctx context.Context, goal, scope uuid.UUID, total int64) error {
	ctx, cancel := context.WithTimeout(ctx, worldTimeout)
	defer cancel()
	if err := g.C.Do(ctx, g.C.B().Set().Key(GoalKey(goal, scope)).Value(strconv.FormatInt(total, 10)).Ex(goalTTL).Build()).Error(); err != nil {
		return fmt.Errorf("goal counter: set: %w", err)
	}
	return nil
}

// Throttle implements port.GoalCounter (SET NX PX: the first caller of a window pushes).
func (g GoalCounter) Throttle(ctx context.Context, goal, scope uuid.UUID, window time.Duration) (bool, error) {
	return setNX(ctx, g.C, goalFlushKey(goal, scope), window)
}

func setNX(ctx context.Context, c valkey.Client, key string, ttl time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, worldTimeout)
	defer cancel()
	err := c.Do(ctx, c.B().Set().Key(key).Value("1").Nx().Px(ttl).Build()).Error()
	if valkey.IsValkeyNil(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("valkey: set nx %s: %w", key, err)
	}
	return true, nil
}

// JobLock implements port.JobLock on Valkey (ew:lock:job:<name>, SET NX PX): one instance runs a tick; the lock expires
// with its TTL (no unlock: the next tick after the TTL takes it again).
type JobLock struct{ C valkey.Client }

var _ port.JobLock = JobLock{}

// Acquire implements port.JobLock.
func (l JobLock) Acquire(ctx context.Context, name string, ttl time.Duration) (bool, error) {
	return setNX(ctx, l.C, "ew:lock:job:"+name, ttl)
}
