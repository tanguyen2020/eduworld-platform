package unit

import (
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/points"
)

// F106-AC-10: the point table of the Ranking Rules — completed +10, win +15, draw +7, +1 per correct answer (at most 10),
// win against a higher tier +5, abandon −5 never below 0 for the day; a loss is never deducted.
func TestPointTable(t *testing.T) {
	tb := seedRules(t).Points
	for _, c := range []struct {
		m      points.Match
		day    points.Day
		amount int
		reason string
	}{
		{points.Match{Outcome: "win", Correct: 5}, points.Day{}, 10 + 15 + 5, points.ReasonWin},
		{points.Match{Outcome: "win", Correct: 14, Upset: true}, points.Day{}, 10 + 15 + 10 + 5, points.ReasonWin},
		{points.Match{Outcome: "draw", Correct: 3}, points.Day{}, 10 + 7 + 3, points.ReasonDraw},
		{points.Match{Outcome: "loss", Correct: 2}, points.Day{}, 10 + 2, points.ReasonCompleted},
		{points.Match{Outcome: "loss", Correct: 0, Upset: true}, points.Day{}, 10, points.ReasonCompleted},
		{points.Match{Outcome: "abandoned", Correct: 4}, points.Day{Total: 50, Matches: 3}, -5, points.ReasonAbandon},
		{points.Match{Outcome: "abandoned"}, points.Day{Total: 3, Matches: 1}, -3, points.ReasonAbandon},
		{points.Match{Outcome: "forfeit"}, points.Day{}, 0, points.ReasonAbandon},
	} {
		a := points.ForMatch(tb, c.m, c.day)
		if a.Amount != c.amount || a.Reason != c.reason || a.Capped {
			t.Fatalf("%+v %+v → %+v", c.m, c.day, a)
		}
		if c.m.Outcome == "loss" && a.Amount < 0 {
			t.Fatal("a loss is deducted")
		}
	}
}

// F106-AC-11 (AC-RNK-006): past competition_points_daily_cap (300) the excess is 0 with reason capped; the 21st match
// of the day gives 0 (daily_matches = 20).
func TestPointDailyCaps(t *testing.T) {
	tb := seedRules(t).Points
	a := points.ForMatch(tb, points.Match{Outcome: "win", Correct: 10}, points.Day{Total: 290, Matches: 8})
	if a.Amount != 10 || a.Reason != points.ReasonCapped || !a.Capped {
		t.Fatalf("partial cap: %+v", a)
	}
	a = points.ForMatch(tb, points.Match{Outcome: "win"}, points.Day{Total: 300, Matches: 9})
	if a.Amount != 0 || a.Reason != points.ReasonCapped || !a.Capped {
		t.Fatalf("cap reached: %+v", a)
	}
	a = points.ForMatch(tb, points.Match{Outcome: "draw"}, points.Day{Total: 100, Matches: 20})
	if a.Amount != 0 || a.Reason != points.ReasonCapped {
		t.Fatalf("21st match: %+v", a)
	}
	// 21 matches in a day: the day total never exceeds the cap and the 21st gives 0
	day := points.Day{}
	for i := 1; i <= 21; i++ {
		a := points.ForMatch(tb, points.Match{Outcome: "win", Correct: 10}, day)
		if i == 21 && a.Amount != 0 {
			t.Fatalf("21st match: %+v", a)
		}
		day.Total += a.Amount
		day.Matches++
		if day.Total > tb.DailyCap {
			t.Fatalf("day total %d above the cap", day.Total)
		}
	}
}
