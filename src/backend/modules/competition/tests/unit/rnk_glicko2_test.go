package unit

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/ranking"
)

// seedRules returns the seeded Ranking Rules (seed/ranking_rules.json = CONFIG_DEFAULTS glicko2_params, tiers…).
func seedRules(t *testing.T) ranking.Rules {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "seed", "ranking_rules.json"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := ranking.ParseRules(b)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// F106-AC-01 (TC-CMP-008, SC-001): the reference example of Glickman (r = 1500, RD = 200, σ = 0.06, τ = 0.5 against
// 1400/30 won, 1550/100 lost, 1700/300 lost) → r' ≈ 1464.06, RD' ≈ 151.52, σ' ≈ 0.05999, error < 0.01.
func TestGlicko2GlickmanVector(t *testing.T) {
	p := seedRules(t).Glicko2
	got := ranking.Update(p, ranking.Rating{R: 1500, RD: 200, Sigma: 0.06}, []ranking.Opponent{
		{R: 1400, RD: 30, Score: 1}, {R: 1550, RD: 100, Score: 0}, {R: 1700, RD: 300, Score: 0}})
	if math.Abs(got.R-1464.06) >= 0.01 || math.Abs(got.RD-151.52) >= 0.01 || math.Abs(got.Sigma-0.05999) >= 0.0001 {
		t.Fatalf("Glickman vector: r=%.4f RD=%.4f σ=%.6f", got.R, got.RD, got.Sigma)
	}
}

// F106-AC-01: the Illinois iteration converges (volatility stays finite and positive) even for extreme upsets, RD stays
// within [RD min, RD max] and a win never lowers the rating.
func TestGlicko2IllinoisConverges(t *testing.T) {
	p := seedRules(t).Glicko2
	for _, c := range []struct{ r, rd, or, ord, s float64 }{
		{1500, 350, 1500, 350, 1}, {800, 50, 2600, 50, 1}, {2600, 50, 800, 50, 0}, {1500, 60, 1500, 60, 0.5}, {1500, 350, 3000, 350, 1},
	} {
		got := ranking.Update(p, ranking.Rating{R: c.r, RD: c.rd, Sigma: p.Sigma0}, []ranking.Opponent{{R: c.or, RD: c.ord, Score: c.s}})
		if math.IsNaN(got.R) || math.IsNaN(got.Sigma) || got.Sigma <= 0 || got.RD < p.RDMin || got.RD > p.RDMax {
			t.Fatalf("%+v → %+v", c, got)
		}
		if c.s == 1 && got.R < c.r {
			t.Fatalf("a win lowered the rating: %+v → %+v", c, got)
		}
	}
}

// F106-AC-04: a team battle member plays a "virtual opponent" with μ, φ averaged over the other team; abandon/forfeit
// score 0, the others score the team result.
func TestGlicko2TeamVirtualOpponent(t *testing.T) {
	p := seedRules(t).Glicko2
	other := []ranking.Rating{{R: 1400, RD: 100, Sigma: 0.06}, {R: 1600, RD: 200, Sigma: 0.06}, {R: 1500, RD: 300, Sigma: 0.06}}
	v := ranking.Virtual(other)
	if v.R != 1500 || v.RD != 200 {
		t.Fatalf("virtual opponent %+v", v)
	}
	me := ranking.Rating{R: 1500, RD: 200, Sigma: 0.06}
	win := ranking.Update(p, me, []ranking.Opponent{{R: v.R, RD: v.RD, Score: 1}})
	direct := ranking.Update(p, me, []ranking.Opponent{{R: 1500, RD: 200, Score: 1}})
	if win != direct || win.R <= 1500 {
		t.Fatalf("virtual win %+v, direct %+v", win, direct)
	}
	for outcome, want := range map[string]float64{"win": 1, "draw": 0.5, "loss": 0, "abandoned": 0, "forfeit": 0} {
		if s, ok := ranking.Score(outcome); !ok || s != want {
			t.Fatalf("score(%s) = %v %v", outcome, s, ok)
		}
	}
	if _, ok := ranking.Score("no_contest"); ok {
		t.Fatal("no_contest must never rate")
	}
	abandon := ranking.Update(p, me, []ranking.Opponent{{R: v.R, RD: v.RD, Score: 0}})
	if abandon.R >= 1500 {
		t.Fatalf("an abandon must lose: %+v", abandon)
	}
}
