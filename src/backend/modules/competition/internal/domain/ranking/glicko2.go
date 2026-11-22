package ranking

import "math"

// scale converts between the Glicko scale (r, RD) and the Glicko-2 scale (μ, φ) (COMPETITION_SYSTEM §7.3).
const scale = 173.7178

// Rating is a Glicko-2 rating on the Glicko scale: r (1500 scale), RD and volatility σ.
type Rating struct {
	R     float64
	RD    float64
	Sigma float64
}

// Opponent is one opponent of a rating period with the score s ∈ {1, 0.5, 0} of the player against it.
type Opponent struct {
	R, RD float64
	Score float64
}

// Initial is the rating of a student without any game (glicko2_params r0, RD0, σ0).
func (p Glicko2Params) Initial() Rating { return Rating{R: p.R0, RD: p.RD0, Sigma: p.Sigma0} }

func g(phi float64) float64 { return 1 / math.Sqrt(1+3*phi*phi/(math.Pi*math.Pi)) }

func expected(mu, muj, phij float64) float64 { return 1 / (1 + math.Exp(-g(phij)*(mu-muj))) }

// Update is glicko2.Update: the rating of the player after one rating period against opponents (Glickman 2012, the
// volatility by the Illinois algorithm to Epsilon). Every match is one period (COMPETITION_SYSTEM §7.2). RD stays within
// [RDMin, RDMax]. Without an opponent it is the "no games" step.
func Update(p Glicko2Params, player Rating, opponents []Opponent) Rating {
	if len(opponents) == 0 {
		return NoGames(p, player)
	}
	mu := (player.R - p.R0) / scale
	phi := player.RD / scale
	sigma := player.Sigma
	var vinv, sum float64
	for _, o := range opponents {
		muj, phij := (o.R-p.R0)/scale, o.RD/scale
		e := expected(mu, muj, phij)
		gj := g(phij)
		vinv += gj * gj * e * (1 - e)
		sum += gj * (o.Score - e)
	}
	v := 1 / vinv
	delta := v * sum
	sigmaP := volatility(p, phi, v, delta, sigma)
	phiStar := math.Sqrt(phi*phi + sigmaP*sigmaP)
	phiP := 1 / math.Sqrt(1/(phiStar*phiStar)+1/v)
	muP := mu + phiP*phiP*sum
	return Rating{R: scale*muP + p.R0, RD: clamp(scale*phiP, p.RDMin, p.RDMax), Sigma: sigmaP}
}

// volatility solves f(x) = 0 by the Illinois algorithm (Glickman step 5).
func volatility(p Glicko2Params, phi, v, delta, sigma float64) float64 {
	a := math.Log(sigma * sigma)
	tau2 := p.Tau * p.Tau
	f := func(x float64) float64 {
		ex := math.Exp(x)
		d := phi*phi + v + ex
		return ex*(delta*delta-phi*phi-v-ex)/(2*d*d) - (x-a)/tau2
	}
	A := a
	var B float64
	if delta*delta > phi*phi+v {
		B = math.Log(delta*delta - phi*phi - v)
	} else {
		k := 1.0
		for f(a-k*p.Tau) < 0 && k < 1000 {
			k++
		}
		B = a - k*p.Tau
	}
	fA, fB := f(A), f(B)
	for i := 0; math.Abs(B-A) > p.Epsilon && i < 200; i++ {
		C := A + (A-B)*fA/(fB-fA)
		fC := f(C)
		if fC*fB <= 0 {
			A, fA = B, fB
		} else {
			fA /= 2
		}
		B, fB = C, fC
	}
	return math.Exp(A / 2)
}

// NoGames is the "no games" step of Glicko-2 for a period without a match (COMPETITION_SYSTEM §7.2: weekly batch):
// φ' = sqrt(φ² + σ²), never above RDMax; the rating and the volatility never change (no rank decay, §12).
func NoGames(p Glicko2Params, player Rating) Rating {
	phi := player.RD / scale
	rd := scale * math.Sqrt(phi*phi+player.Sigma*player.Sigma)
	return Rating{R: player.R, RD: clamp(math.Max(rd, player.RD), p.RDMin, p.RDMax), Sigma: player.Sigma}
}

// Virtual is the "virtual opponent" of a team battle member: μ and φ averaged over the other team (§7.3).
func Virtual(team []Rating) Rating {
	if len(team) == 0 {
		return Rating{}
	}
	var r, rd, s float64
	for _, x := range team {
		r += x.R
		rd += x.RD
		s += x.Sigma
	}
	n := float64(len(team))
	return Rating{R: r / n, RD: rd / n, Sigma: s / n}
}

// Score is the Glicko-2 score of a match outcome (§7.3): win 1, draw 0.5, loss/abandoned/forfeit 0; ok false for an
// outcome that never rates (no_contest, unknown).
func Score(outcome string) (float64, bool) {
	switch outcome {
	case "win":
		return 1, true
	case "draw":
		return 0.5, true
	case "loss", "abandoned", "forfeit":
		return 0, true
	}
	return 0, false
}

// SeasonReset is the season soft reset (season_soft_reset, §8.1): r0 + factor·(r − r0), RD at least the floor, σ kept.
func (r Rules) SeasonReset(old Rating) Rating {
	g := r.Glicko2
	return Rating{R: g.R0 + r.SoftReset.Factor*(old.R-g.R0), RD: clamp(math.Max(old.RD, r.SoftReset.RDFloor), g.RDMin, g.RDMax),
		Sigma: old.Sigma}
}

func clamp(x, lo, hi float64) float64 { return math.Min(math.Max(x, lo), hi) }
