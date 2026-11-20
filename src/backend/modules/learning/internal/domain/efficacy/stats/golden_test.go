package stats

import (
	"math"
	"testing"
)

func near(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.001 {
		t.Errorf("%s = %.6f, want %.6f", name, got, want)
	}
}

// F035-AC-29, SC-003: gain, normalized gain, Hedges' g, SE and CI on values computed by hand (sai số ≤ 0,001).
func TestGoldenEffect(t *testing.T) {
	near(t, "gain", Gain(0.4, 0.75), 0.35)
	if g, ok := NormalizedGain(0.5, 0.75); !ok || math.Abs(g-0.5) > 1e-9 {
		t.Fatalf("normalized gain %v %v", g, ok)
	}
	if _, ok := NormalizedGain(1, 1); ok {
		t.Fatal("normalized gain with pre = 1 kept")
	}
	// I = [0.2, 0.4, 0.6] (mean 0.4, s² 0.04), C = [0.1, 0.2, 0.3] (mean 0.2, s² 0.01): S_p = √0.025 = 0.158114,
	// J = 1 − 3/15 = 0.8, g = 0.8 × 0.2 / 0.158114 = 1.011929, SE = √(6/9 + g²/12) = 0.867179, CI = g ± 1.96 SE.
	e := HedgesG([]float64{0.2, 0.4, 0.6}, []float64{0.1, 0.2, 0.3}, 1)
	if !e.OK {
		t.Fatal("effect not computed")
	}
	near(t, "g", e.G, 1.011929)
	near(t, "SE", e.SE, 0.867179)
	near(t, "low", e.Low, -0.687743)
	near(t, "high", e.High, 2.711600)
	// The design effect widens the interval: SE_adj = SE × √deff.
	e2 := HedgesG([]float64{0.2, 0.4, 0.6}, []float64{0.1, 0.2, 0.3}, 2)
	near(t, "SE_adj", e2.SEAdj, 0.867179*math.Sqrt2)
	near(t, "g unchanged", e2.G, e.G)
	if HedgesG([]float64{0.2}, []float64{0.1, 0.2}, 1).OK || HedgesG([]float64{0.2, 0.2}, []float64{0.2, 0.2}, 1).OK {
		t.Fatal("degenerate groups computed")
	}
	// ICC of [[1,2,3],[4,5,6]]: MSB 13.5, MSW 1, n₀ 3 → 12.5/15.5 = 0.806452; m̄ 3; deff = 1 + 2 × ICC = 2.612903.
	icc, mbar := ICC([][]float64{{1, 2, 3}, {4, 5, 6}})
	near(t, "ICC", icc, 0.806452)
	near(t, "m̄", mbar, 3)
	near(t, "deff", DesignEffect(icc, mbar), 2.612903)
	if icc1, _ := ICC([][]float64{{1, 2, 3}}); icc1 != 0 {
		t.Fatal("ICC of one cluster")
	}
	// Same input, same output (determinism of the report).
	if again := HedgesG([]float64{0.2, 0.4, 0.6}, []float64{0.1, 0.2, 0.3}, 1); again != e {
		t.Fatal("not deterministic")
	}
}

// F035-AC-30: Spearman ρ (average ranks), AUC (Mann–Whitney), calibration — computed by hand.
func TestGoldenValidity(t *testing.T) {
	rho, ok := Spearman([]float64{1, 2, 3, 4, 5}, []float64{2, 1, 4, 3, 5})
	if !ok {
		t.Fatal("rho")
	}
	near(t, "ρ", rho, 0.8) // 1 − 6·4/(5·24)
	rho, _ = Spearman([]float64{1, 2, 2, 3}, []float64{1, 2, 3, 4})
	near(t, "ρ ties", rho, 0.948683) // 4.5 / √22.5
	if _, ok := Spearman([]float64{1, 1, 1}, []float64{1, 2, 3}); ok {
		t.Fatal("ρ without variance")
	}
	auc, ok := AUC([]float64{0.9, 0.8, 0.7, 0.6, 0.5}, []bool{true, false, true, false, false})
	if !ok {
		t.Fatal("auc")
	}
	near(t, "AUC", auc, 5.0/6)
	tie, _ := AUC([]float64{0.5, 0.5}, []bool{true, false})
	near(t, "AUC tie", tie, 0.5)
	if _, ok := AUC([]float64{0.5}, []bool{true}); ok {
		t.Fatal("AUC with one class")
	}
	cal, ok := Calibration([]string{"mastered", "mastered", "beginning", "developing", "beginning"}, []float64{0.9, 0.7, 0.3, 0.5, 0.6})
	if !ok {
		t.Fatal("calibration")
	}
	near(t, "calibration", cal, 0.5)
	if _, ok := Calibration([]string{"developing"}, []float64{0.5}); ok {
		t.Fatal("calibration without mastered/beginning")
	}
	r := Ranks([]float64{3, 1, 3, 2})
	if r[0] != 3.5 || r[1] != 1 || r[2] != 3.5 || r[3] != 2 {
		t.Fatalf("ranks %v", r)
	}
}
