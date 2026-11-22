package unit

import (
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/ranking"
)

// F106-AC-05: provisional while games_played < 10 (rating_provisional) or RD > 110 — no tier, out of rating boards.
func TestProvisionalPolicy(t *testing.T) {
	r := seedRules(t)
	for _, c := range []struct {
		games int
		rd    float64
		want  bool
	}{{0, 350, true}, {3, 100, true}, {9, 60, true}, {10, 111, true}, {10, 110, false}, {12, 80, false}} {
		if got := r.IsProvisional(c.games, c.rd); got != c.want {
			t.Fatalf("IsProvisional(%d, %v) = %v", c.games, c.rd, got)
		}
	}
}

// F106-AC-06: the weekly "no games" step raises RD (√(φ² + σ²)), never above RD max, never changes the rating nor the
// volatility (no rank decay, COMPETITION_SYSTEM §12).
func TestNoGamesRaisesRDOnly(t *testing.T) {
	p := seedRules(t).Glicko2
	r := ranking.Rating{R: 1720, RD: 80, Sigma: 0.06}
	prev := r
	for week := 0; week < 400; week++ {
		next := ranking.NoGames(p, prev)
		if next.R != r.R || next.Sigma != r.Sigma {
			t.Fatalf("week %d changed rating/volatility: %+v", week, next)
		}
		if next.RD < prev.RD || next.RD > p.RDMax {
			t.Fatalf("week %d RD %v → %v (max %v)", week, prev.RD, next.RD, p.RDMax)
		}
		prev = next
	}
	if prev.RD <= r.RD {
		t.Fatalf("RD did not grow over inactive weeks: %v", prev.RD)
	}
	if near := ranking.NoGames(p, ranking.Rating{R: 1500, RD: 349.9, Sigma: 0.06}); near.RD != p.RDMax {
		t.Fatalf("RD above the max: %v", near.RD)
	}
	if got := ranking.Update(p, r, nil); got.R != r.R || got.RD <= r.RD {
		t.Fatalf("Update without opponent is the no-games step: %+v", got)
	}
}
