package command

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
)

// playedRepo is a TEST DOUBLE of write.SessionRepository (R17): only PlayedSince is used here.
type playedRepo struct {
	write.SessionRepository
	played      time.Duration
	since, upto time.Time
}

func (r *playedRepo) PlayedSince(_ context.Context, _, _ uuid.UUID, since, now time.Time) (time.Duration, error) {
	r.since, r.upto = since, now
	return r.played, nil
}

// F014-AC-27 (OQ-17): the allowance counts the ICT day of now, reopens at 00:00 ICT the next day; no limit (off,
// common tenant) is never reached; the warning goes out once, when the remaining time crosses curfew_warning_minutes.
func TestDailyAllowance(t *testing.T) {
	ctx := context.Background()
	limit := 120
	d := &directory{limit: &limit}
	r := &playedRepo{played: 110 * time.Minute}
	h := Sessions{Directory: d, Sessions: r, Config: SessionConfig{CurfewLocation: ict, WarningBefore: 10 * time.Minute}}
	now := time.Date(2026, 11, 20, 17, 30, 0, 0, time.UTC) // 00:30 ICT on 21/11
	a, err := h.dailyAllowance(ctx, uuid.New(), uuid.New(), uuid.New(), now)
	if err != nil || !a.Limited || a.Remaining != 10*time.Minute || a.Reached() {
		t.Fatalf("allowance: %+v %v", a, err)
	}
	if !r.since.Equal(time.Date(2026, 11, 21, 0, 0, 0, 0, ict)) || !a.Reopen.Equal(time.Date(2026, 11, 22, 0, 0, 0, 0, ict)) {
		t.Fatalf("day bounds: since %v reopen %v", r.since, a.Reopen)
	}
	// Warning: remaining 10 min just crossed (was 11 min one tick ago) → warn; 9 min of an old session → already warned.
	if !h.warnNow(a, now.Add(-time.Hour), now) {
		t.Fatal("warning at the crossing")
	}
	if h.warnNow(dailyAllowance{Limited: true, Remaining: 9 * time.Minute}, now.Add(-time.Hour), now) {
		t.Fatal("warned twice")
	}
	if !h.warnNow(dailyAllowance{Limited: true, Remaining: 5 * time.Minute}, now.Add(-30*time.Second), now) {
		t.Fatal("a session started inside the warning period is warned once")
	}
	if h.warnNow(dailyAllowance{Limited: true, Remaining: 20 * time.Minute}, now, now) || h.warnNow(dailyAllowance{}, now, now) {
		t.Fatal("no warning far from the limit or without limit")
	}
	r.played = 2 * time.Hour
	if a, _ = h.dailyAllowance(ctx, uuid.New(), uuid.New(), uuid.New(), now); !a.Reached() {
		t.Fatal("limit reached")
	}
	d.limit = nil
	if a, _ = h.dailyAllowance(ctx, uuid.New(), uuid.New(), uuid.New(), now); a.Limited || a.Reached() {
		t.Fatal("no limit")
	}
	if a, _ = h.dailyAllowance(ctx, uuid.New(), uuid.New(), uuid.Nil, now); a.Limited {
		t.Fatal("no school")
	}
}
