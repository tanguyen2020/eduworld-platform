package command

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// AnswerRate is answer_rate (CONFIG_DEFAULTS, owner SECURITY_ARCHITECTURE): answers and submits per student per minute
// and the minimum gap between two answers of an attempt on the server clock (OQ F030 owner 2026-11-20). The values
// are the defaults until module learning reads platform settings (like attempt_start_rate) [Proposed].
type AnswerRate struct {
	AnswersPerMinute int
	SubmitsPerMinute int
	MinGap           time.Duration
}

// DefaultAnswerRate is 60 answers per minute, at least 1 s between answers, 10 submits per minute.
func DefaultAnswerRate() AnswerRate {
	return AnswerRate{AnswersPerMinute: 60, SubmitsPerMinute: 10, MinGap: time.Second}
}

// Rate limit kinds.
const (
	RateAnswer = "answer"
	RateSubmit = "submit"
)

// RateLimiter counts the answers and submits of a student (Valkey; counters are not business state, R9) at the server
// time now: it returns the delay before the next allowed call (0 = allowed). An error of the store lets the call
// through (the adapter counts it).
type RateLimiter interface {
	Allow(ctx context.Context, kind string, studentID, attemptID uuid.UUID, now time.Time, r AnswerRate) (time.Duration, error)
}

// RetryAfter is 429 RATE_LIMITED with the delay the presentation sends as Retry-After.
type RetryAfter struct {
	After time.Duration
	Err   *problem.Error
}

func (e *RetryAfter) Error() string { return e.Err.Error() }

// Unwrap exposes the problem error.
func (e *RetryAfter) Unwrap() error { return e.Err }

// Seconds is the Retry-After value (at least 1).
func (e *RetryAfter) Seconds() string {
	s := int((e.After + time.Second - 1) / time.Second)
	if s < 1 {
		s = 1
	}
	return strconv.Itoa(s)
}

// checkAnswerRate applies answer_rate before an answer or a submit (F030 T053): over the limit → 429 RATE_LIMITED with
// Retry-After; no limiter, or a limiter error, lets the call through.
func (h Attempts) checkAnswerRate(ctx context.Context, kind string, studentID, attemptID uuid.UUID) error {
	if h.Rate == nil {
		return nil
	}
	r := h.AnswerRate
	if r.AnswersPerMinute == 0 {
		r = DefaultAnswerRate()
	}
	wait, err := h.Rate.Allow(ctx, kind, studentID, attemptID, h.now(), r)
	if err != nil {
		return nil //nolint:nilerr // a counter failure lets the call through (R9); the adapter logs and counts it
	}
	if wait <= 0 {
		return nil
	}
	return &RetryAfter{After: wait, Err: problem.New(problem.RateLimited, "answer_rate reached").Retry()}
}
