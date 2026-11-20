//go:build integration

package integration

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// rateMetrics records the answer_rate metrics of the test.
type rateMetrics struct {
	mu      sync.Mutex
	limited map[string]int
	failed  map[string]int
}

func (m *rateMetrics) RateLimited(kind string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.limited[kind]++
}

func (m *rateMetrics) RateCounterFailed(kind string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failed[kind]++
}

func (m *rateMetrics) get(what map[string]int, kind string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return what[kind]
}

// F030 T053 (OQ owner 2026-11-20): answer_rate counted by module learning on Valkey per student, on the server clock:
// the minimum gap between two answers of an attempt, a one-minute window for answers and for submits; over → 429
// RATE_LIMITED with Retry-After; Valkey down → the call goes through and the failure metric grows (R9).
func TestAnswerRate(t *testing.T) {
	ctx := context.Background()
	container, addr := testkit.StartValkey(ctx, t)
	vk, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}, DisableCache: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vk.Close)
	m := &rateMetrics{limited: map[string]int{}, failed: map[string]int{}}
	cfg := command.AnswerRate{AnswersPerMinute: 3, SubmitsPerMinute: 1, MinGap: time.Second}
	c := newCurEnv(ctx, t, withAnswerRate(cache.Rate{C: vk, Metrics: m}, cfg))
	s := c.attemptSetup(ctx, t, nil)
	// Every step below stays in one minute window of the server clock.
	c.clk.mu.Lock()
	c.clk.t = c.clk.t.Truncate(time.Minute).Add(time.Second)
	c.clk.mu.Unlock()
	right, wrong := correctOption()
	id := must(t, c.start(t, hsAn, s.activityID, nil, nil), http.StatusCreated, "start").str("attempt_id")

	c.clk.Advance(answerTime)
	must(t, c.answer(t, hsAn, id, 1, map[string]any{"option_id": right}, nil), http.StatusOK, "answer 1")
	// Minimum gap: a second answer at the same server time is refused with the rest of the gap.
	r := c.answer(t, hsAn, id, 2, map[string]any{"option_id": right}, nil)
	if r.status != http.StatusTooManyRequests || r.str("code") != "RATE_LIMITED" || r.header.Get("Retry-After") != "1" {
		t.Fatalf("gap: %d %s Retry-After=%q", r.status, r.raw, r.header.Get("Retry-After"))
	}
	c.clk.Advance(answerTime)
	must(t, c.answer(t, hsAn, id, 2, map[string]any{"option_id": right}, nil), http.StatusOK, "answer 2 after the gap")
	c.clk.Advance(answerTime)
	must(t, c.answer(t, hsAn, id, 3, map[string]any{"option_id": wrong}, nil), http.StatusOK, "answer 3")
	// Window: the fourth answer of the minute is refused until the minute ends.
	c.clk.Advance(answerTime)
	r = c.answer(t, hsAn, id, 3, map[string]any{"option_id": right}, nil)
	after, _ := strconv.Atoi(r.header.Get("Retry-After"))
	if r.status != http.StatusTooManyRequests || r.str("code") != "RATE_LIMITED" || after < 2 || after > 60 {
		t.Fatalf("window: %d %s Retry-After=%q", r.status, r.raw, r.header.Get("Retry-After"))
	}
	if got := m.get(m.limited, command.RateAnswer); got != 2 {
		t.Fatalf("limited answers = %d", got)
	}
	// Submits: one per minute in this configuration; the second is refused before the attempt is read.
	must(t, c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":submit", hsAn, idem(), nil), http.StatusOK, "submit")
	r = c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":submit", hsAn, idem(), nil)
	if r.status != http.StatusTooManyRequests || r.str("code") != "RATE_LIMITED" || r.header.Get("Retry-After") == "" {
		t.Fatalf("submit window: %d %s", r.status, r.raw)
	}
	// The counters are per student: another student of the tenant is not limited.
	other := must(t, c.start(t, hsBinh, s.activityID, nil, nil), http.StatusCreated, "start other").str("attempt_id")
	must(t, c.answer(t, hsBinh, other, 1, map[string]any{"option_id": right}, nil), http.StatusOK, "answer other")

	// Valkey down: answers and submits go through, the failure is counted and bounded by PoolTimeout.
	if err := container.Stop(ctx, nil); err != nil {
		t.Fatal(err)
	}
	c.clk.Advance(time.Minute)
	id = must(t, c.start(t, hsAn, s.activityID, nil, nil), http.StatusCreated, "start with Valkey down").str("attempt_id")
	for pos := 1; pos <= 3; pos++ {
		began := time.Now()
		must(t, c.answer(t, hsAn, id, pos, map[string]any{"option_id": right}, nil), http.StatusOK, "answer with Valkey down")
		if d := time.Since(began); d > 5*time.Second {
			t.Fatalf("answer with Valkey down took %s", d)
		}
	}
	must(t, c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":submit", hsAn, idem(), nil), http.StatusOK, "submit with Valkey down")
	if m.get(m.failed, command.RateAnswer) != 3 || m.get(m.failed, command.RateSubmit) != 1 {
		t.Fatalf("failure metric: %+v", m.failed)
	}
}
