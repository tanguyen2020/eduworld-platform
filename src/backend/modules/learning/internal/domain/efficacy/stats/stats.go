// Package stats computes the statistics of the efficacy report (F035 US6, research R2, LEARNING_EFFICACY_PLAN §4–§5): gain
// and normalized gain, Hedges' g with a 95 % CI corrected by the design effect of the class clusters (one-way ANOVA
// ICC), Spearman ρ with average ranks, the AUC (normalized Mann–Whitney statistic) and the calibration rates. Pure Go,
// float64, deterministic: the same input gives the same output. Multilevel models run outside the system.
package stats

import (
	"math"
	"sort"
)

// Mean returns the mean (0 for no value).
func Mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

// Variance returns the sample variance (n − 1; 0 under two values).
func Variance(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := Mean(xs)
	var ss float64
	for _, x := range xs {
		ss += (x - m) * (x - m)
	}
	return ss / float64(len(xs)-1)
}

// Gain is post − pre on proportions correct.
func Gain(pre, post float64) float64 { return post - pre }

// NormalizedGain is (post − pre) / (1 − pre); ok is false when pre = 1 (left out, research R2).
func NormalizedGain(pre, post float64) (float64, bool) {
	if pre >= 1 {
		return 0, false
	}
	return (post - pre) / (1 - pre), true
}

// Effect is a Hedges' g with its standard error and 95 % confidence interval.
type Effect struct {
	G, SE, SEAdj, Deff, ICC float64
	Low, High               float64
	NI, NC                  int
	OK                      bool // false when a group has fewer than 2 values or no variance
}

// HedgesG computes g = J·(Mᵢ − M_c)/S_p with J = 1 − 3/(4(nᵢ+n_c)−9), S_p the pooled SD,
// SE = √((nᵢ+n_c)/(nᵢn_c) + g²/(2(nᵢ+n_c))); the CI uses SE_adj = SE·√deff (deff ≥ 1, 1 for no clustering).
func HedgesG(intervention, comparison []float64, deff float64) Effect {
	e := Effect{NI: len(intervention), NC: len(comparison), Deff: math.Max(1, deff)}
	ni, nc := float64(e.NI), float64(e.NC)
	if e.NI < 2 || e.NC < 2 {
		return e
	}
	sp := math.Sqrt(((ni-1)*Variance(intervention) + (nc-1)*Variance(comparison)) / (ni + nc - 2))
	if sp == 0 {
		return e
	}
	j := 1 - 3/(4*(ni+nc)-9)
	e.G = j * (Mean(intervention) - Mean(comparison)) / sp
	e.SE = math.Sqrt((ni+nc)/(ni*nc) + e.G*e.G/(2*(ni+nc)))
	e.SEAdj = e.SE * math.Sqrt(e.Deff)
	e.Low, e.High = e.G-1.96*e.SEAdj, e.G+1.96*e.SEAdj
	e.OK = true
	return e
}

// ICC is the one-way ANOVA intraclass correlation of values grouped by cluster:
// ICC = (MSB − MSW) / (MSB + (n₀ − 1)·MSW), n₀ = (N − Σnⱼ²/N)/(k − 1); clipped at 0; 0 with fewer than 2 clusters.
// It also returns the mean cluster size m̄.
func ICC(clusters [][]float64) (icc, mbar float64) {
	var all []float64
	k := 0
	for _, c := range clusters {
		if len(c) > 0 {
			k++
			all = append(all, c...)
		}
	}
	n := float64(len(all))
	if k == 0 {
		return 0, 0
	}
	mbar = n / float64(k)
	if k < 2 || len(all) <= k {
		return 0, mbar
	}
	grand := Mean(all)
	var ssb, ssw, sumSq float64
	for _, c := range clusters {
		if len(c) == 0 {
			continue
		}
		m := Mean(c)
		ssb += float64(len(c)) * (m - grand) * (m - grand)
		for _, x := range c {
			ssw += (x - m) * (x - m)
		}
		sumSq += float64(len(c) * len(c))
	}
	msb := ssb / float64(k-1)
	msw := ssw / (n - float64(k))
	n0 := (n - sumSq/n) / float64(k-1)
	den := msb + (n0-1)*msw
	if den <= 0 {
		return 0, mbar
	}
	return math.Max(0, (msb-msw)/den), mbar
}

// DesignEffect is 1 + (m̄ − 1)·ICC.
func DesignEffect(icc, mbar float64) float64 { return 1 + math.Max(0, mbar-1)*icc }

// Ranks returns the ranks (1-based) of xs, ties sharing their average rank.
func Ranks(xs []float64) []float64 {
	idx := make([]int, len(xs))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return xs[idx[a]] < xs[idx[b]] })
	out := make([]float64, len(xs))
	for i := 0; i < len(idx); {
		j := i
		for j+1 < len(idx) && xs[idx[j+1]] == xs[idx[i]] {
			j++
		}
		r := float64(i+j)/2 + 1
		for k := i; k <= j; k++ {
			out[idx[k]] = r
		}
		i = j + 1
	}
	return out
}

// Pearson is the correlation of two series (ok false without variance or under 3 pairs).
func Pearson(x, y []float64) (float64, bool) {
	if len(x) != len(y) || len(x) < 3 {
		return 0, false
	}
	mx, my := Mean(x), Mean(y)
	var sxy, sxx, syy float64
	for i := range x {
		sxy += (x[i] - mx) * (y[i] - my)
		sxx += (x[i] - mx) * (x[i] - mx)
		syy += (y[i] - my) * (y[i] - my)
	}
	if sxx == 0 || syy == 0 {
		return 0, false
	}
	return sxy / math.Sqrt(sxx*syy), true
}

// Spearman is the Pearson correlation of the average ranks.
func Spearman(x, y []float64) (float64, bool) { return Pearson(Ranks(x), Ranks(y)) }

// AUC is the probability that a positive scores above a negative (ties count one half): the Mann–Whitney U divided by
// n₊·n₋; ok false without both classes.
func AUC(scores []float64, positive []bool) (float64, bool) {
	var np, nn float64
	for _, p := range positive {
		if p {
			np++
		} else {
			nn++
		}
	}
	if np == 0 || nn == 0 || len(scores) != len(positive) {
		return 0, false
	}
	r := Ranks(scores)
	var sumPos float64
	for i, p := range positive {
		if p {
			sumPos += r[i]
		}
	}
	u := sumPos - np*(np+1)/2
	return u / (np * nn), true
}

// Calibration is the share of students whose level agrees with the independent test (LEARNING_EFFICACY_PLAN §5):
// mastered students reaching ≥ 80 %, beginning students under 50 %; ok false without such students.
func Calibration(levels []string, post []float64) (float64, bool) {
	var n, hit float64
	for i, l := range levels {
		switch l {
		case "mastered":
			n++
			if post[i] >= 0.8 {
				hit++
			}
		case "beginning":
			n++
			if post[i] < 0.5 {
				hit++
			}
		}
	}
	if n == 0 {
		return 0, false
	}
	return hit / n, true
}

// Round rounds to d decimals (report values).
func Round(v float64, d int) float64 {
	p := math.Pow(10, float64(d))
	return math.Round(v*p) / p
}
