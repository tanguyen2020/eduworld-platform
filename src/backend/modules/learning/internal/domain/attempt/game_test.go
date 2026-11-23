package attempt_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
)

// F073-AC-08, AC-11: a mini-game attempt starts with a game score of 0; the served interval runs from the previous answer
// (or the start) on the server clock, whatever the pauses; a late answer of a timed play is a passed deadline.
func TestMinigameAttempt(t *testing.T) {
	now := time.Date(2026, 11, 23, 8, 0, 0, 0, time.UTC)
	limit := 10
	pinned := uuid.New()
	a := attempt.New(attempt.Start{ID: uuid.New(), Mode: attempt.ModeMinigame, ActivityType: "minigame", Now: now, TimeLimitSeconds: &limit,
		ActivityFeedback: "immediate", MinigameConfigVersionID: &pinned, Items: []attempt.Item{{Position: 1, MaxScore: 1}}})
	if !a.GameScored() || *a.GameScore != 0 || a.GameStreak != 0 || *a.MinigameConfigVersionID != pinned {
		t.Fatalf("start: %+v", a)
	}
	if plain := attempt.New(attempt.Start{ID: uuid.New(), Mode: attempt.ModePractice, Now: now}); plain.GameScored() {
		t.Fatal("a practice attempt has a game score")
	}
	if ms := attempt.ServedInterval(now, nil, now.Add(4*time.Second)); ms != 4000 {
		t.Fatalf("from the start: %d", ms)
	}
	last := now.Add(3 * time.Second)
	if ms := attempt.ServedInterval(now, &last, now.Add(5*time.Second)); ms != 2000 {
		t.Fatalf("from the previous answer: %d", ms)
	}
	if ms := attempt.ServedInterval(now, nil, now.Add(-time.Second)); ms != 0 {
		t.Fatalf("never negative: %d", ms)
	}
	a.ApplyGame(activity.GamePoints{Points: 150, Score: 150, Streak: 1})
	if *a.GameScore != 150 || a.GameStreak != 1 {
		t.Fatalf("apply: %+v", a)
	}
	pol := attempt.DefaultPolicy()
	if a.DeadlinePassed(now.Add(11*time.Second), pol) {
		t.Fatal("within the grace")
	}
	if !a.DeadlinePassed(now.Add(13*time.Second), pol) {
		t.Fatal("past deadline + grace")
	}
	if _, err := a.Respond(1, now.Add(13*time.Second), pol, 2, true, nil); err == nil {
		t.Fatal("late response admitted")
	}
	a.Expire(now.Add(13*time.Second), pol)
	if !a.DeadlinePassed(now.Add(13*time.Second), pol) {
		t.Fatal("expired play")
	}
}
