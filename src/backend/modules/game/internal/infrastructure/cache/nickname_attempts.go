package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// NicknameAttempts implements port.NicknameAttemptCounter (VALKEY_DESIGN, F136 data-model §1): a sliding window per
// student and tenant, the sorted set ew:game:nick:attempts:<tenant_id>:<student_id> of submission times (scores, ms)
// with the TTL of the window. Numbers only — never the typed text (R14). Temporary data (R9): a lost key only loosens
// the limit for one window.
type NicknameAttempts struct {
	C       valkey.Client
	Limit   int           // nickname_attempt_rate: submissions per window (default 10)
	Window  time.Duration // nickname_attempt_rate: window (default 10 minutes)
	Timeout time.Duration // per call (default 1 s)
}

var _ port.NicknameAttemptCounter = NicknameAttempts{}

// AttemptsKey is the window of student in tenant.
func AttemptsKey(tenant, student uuid.UUID) string {
	return "ew:game:nick:attempts:" + tenant.String() + ":" + student.String()
}

// Hit implements port.NicknameAttemptCounter: drop the submissions older than the window, add this one, count; beyond
// Limit the wait is the time until the oldest counted submission leaves the window.
func (a NicknameAttempts) Hit(ctx context.Context, tenant, student uuid.UUID, now time.Time) (bool, time.Duration, error) {
	t := a.Timeout
	if t <= 0 {
		t = time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()
	key, ms, win := AttemptsKey(tenant, student), now.UnixMilli(), a.Window.Milliseconds()
	res := a.C.DoMulti(ctx,
		a.C.B().Zremrangebyscore().Key(key).Min("-inf").Max(strconv.FormatInt(ms-win, 10)).Build(),
		a.C.B().Zadd().Key(key).ScoreMember().ScoreMember(float64(ms), ids.New().String()).Build(),
		a.C.B().Pexpire().Key(key).Milliseconds(win).Build(),
		a.C.B().Zcard().Key(key).Build(),
		a.C.B().Zrange().Key(key).Min("0").Max("0").Withscores().Build(),
	)
	for _, r := range res[:3] {
		if err := r.Error(); err != nil {
			return false, 0, fmt.Errorf("nickname attempts: %w", err)
		}
	}
	n, err := res[3].AsInt64()
	if err != nil {
		return false, 0, fmt.Errorf("nickname attempts: count: %w", err)
	}
	if int(n) <= a.Limit {
		return true, 0, nil
	}
	oldest, err := res[4].AsZScores()
	if err != nil || len(oldest) == 0 {
		return false, a.Window, nil
	}
	wait := time.Duration(int64(oldest[0].Score)+win-ms) * time.Millisecond
	if wait <= 0 {
		wait = time.Second
	}
	return false, wait, nil
}
