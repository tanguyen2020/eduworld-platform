package unit

import (
	"math"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/ranking"
)

// F106-AC-23 (AC-RNK-004): the new season rating is 1500 + 0.5·(r − 1500) (season_soft_reset), RD at least 150 (never
// above RD max), the volatility is kept.
func TestSeasonSoftReset(t *testing.T) {
	r := seedRules(t)
	for _, c := range []struct{ r, rd, wantR, wantRD float64 }{
		{1900, 60, 1700, 150}, {1300, 220, 1400, 220}, {1500, 350, 1500, 350}, {2100, 149.9, 1800, 150},
	} {
		got := r.SeasonReset(ranking.Rating{R: c.r, RD: c.rd, Sigma: 0.0612})
		if math.Abs(got.R-c.wantR) > 1e-9 || math.Abs(got.RD-c.wantRD) > 1e-9 || got.Sigma != 0.0612 {
			t.Fatalf("%v/%v → %+v", c.r, c.rd, got)
		}
	}
}
