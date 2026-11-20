package cache

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
)

// RatePrefix is the key prefix of the answer_rate counters: ew:learning:rate:<kind>:<student_id>:<minute> and
// ew:learning:rate:gap:<student_id>:<attempt_id> (server time of the last accepted answer, unix ms).
const RatePrefix = "ew:learning:rate:"

// rateKeyTTL keeps a counter past its window; the windows themselves are computed from the server time.
const rateKeyTTL = 2 * time.Minute

// RateMetrics counts the refused calls and the counter calls that failed (the call was let through, R9); nil = not
// counted.
type RateMetrics interface {
	RateLimited(kind string)
	RateCounterFailed(kind string)
}

// Rate implements command.RateLimiter on Valkey: a fixed one-minute window per student and kind, and for answers the
// minimum gap per attempt, both on the server time given by the use case. Every call is bounded by PoolTimeout.
type Rate struct {
	C       valkey.Client
	Metrics RateMetrics
}

var _ command.RateLimiter = Rate{}

func (r Rate) failed(ctx context.Context, kind string, err error) (time.Duration, error) {
	slog.WarnContext(ctx, "answer_rate counter failed: the call is let through", "kind", kind, "error", err)
	if r.Metrics != nil {
		r.Metrics.RateCounterFailed(kind)
	}
	return 0, err
}

func (r Rate) limited(kind string, wait time.Duration) time.Duration {
	if r.Metrics != nil {
		r.Metrics.RateLimited(kind)
	}
	return wait
}

// Allow implements command.RateLimiter.
func (r Rate) Allow(ctx context.Context, kind string, studentID, attemptID uuid.UUID, now time.Time, cfg command.AnswerRate) (time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, PoolTimeout)
	defer cancel()
	limit, gapKey := cfg.AnswersPerMinute, ""
	if kind == command.RateSubmit {
		limit = cfg.SubmitsPerMinute
	}
	if kind == command.RateAnswer && cfg.MinGap > 0 {
		gapKey = RatePrefix + "gap:" + studentID.String() + ":" + attemptID.String()
		last, err := r.C.Do(ctx, r.C.B().Get().Key(gapKey).Build()).AsInt64()
		switch {
		case err != nil && !valkey.IsValkeyNil(err):
			return r.failed(ctx, kind, err)
		case err == nil:
			if since := now.Sub(time.UnixMilli(last)); since >= 0 && since < cfg.MinGap {
				return r.limited(kind, cfg.MinGap-since), nil
			}
		}
	}
	minute := now.Unix() / 60
	key := RatePrefix + kind + ":" + studentID.String() + ":" + strconv.FormatInt(minute, 10)
	n, err := r.C.Do(ctx, r.C.B().Incr().Key(key).Build()).AsInt64()
	if err != nil {
		return r.failed(ctx, kind, err)
	}
	if n == 1 {
		if err := r.C.Do(ctx, r.C.B().Expire().Key(key).Seconds(int64(rateKeyTTL/time.Second)).Build()).Error(); err != nil {
			slog.WarnContext(ctx, "answer_rate counter expiry failed", "kind", kind, "error", err)
		}
	}
	if limit > 0 && n > int64(limit) {
		return r.limited(kind, time.Duration(60-now.Unix()%60)*time.Second), nil
	}
	if gapKey != "" {
		cmd := r.C.B().Set().Key(gapKey).Value(strconv.FormatInt(now.UnixMilli(), 10)).Px(rateKeyTTL).Build()
		if err := r.C.Do(ctx, cmd).Error(); err != nil {
			return r.failed(ctx, kind, err)
		}
	}
	return 0, nil
}
