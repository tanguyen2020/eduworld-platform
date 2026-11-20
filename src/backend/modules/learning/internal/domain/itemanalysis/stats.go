// Package itemanalysis computes the quality statistics of a question version (F044 US4, QUESTION_BANK §9) and the
// semi-automatic difficulty calibration (F044 US5, QUESTION_BANK §9.1). Pure Go: the inputs are the first scored answers
// of the version, already read by the caller; nothing here changes a published version (AGENTS R11).
package itemanalysis

import (
	"math"
	"slices"
)

// DefaultMinResponses is item_analysis_min_responses (CONFIG_DEFAULTS: ≥ 30 first scored answers once rapid guesses are
// removed, owner QUESTION_BANK §9).
const DefaultMinResponses = 30

// Flags of a version (content.question_stats.flags).
const (
	FlagNeedsCalibration      = "needs_calibration"
	FlagUnusedOption          = "unused_option"
	FlagSuggestOptionFeedback = "suggest_option_feedback"
	FlagLowDiscrimination     = "low_discrimination"
)

// Thresholds of the flags: a wrong option chosen by at least SuggestFeedbackRate of the valid answers suggests writing its
// option feedback [Proposed]; LowDiscrimination is the "problem item" threshold of EXAM_ENGINE §8.1.
const (
	SuggestFeedbackRate = 0.25
	LowDiscrimination   = 0.1
)

// Response is the first scored answer of one student to the version in one attempt.
type Response struct {
	Correct    bool
	Total      float64 // score ratio (0..1) of the whole attempt: the criterion of the point-biserial
	TimeMS     *int
	RapidGuess bool
	HintsUsed  int
	Options    []string // selected option ids (choice types)
}

// Options of the computation.
type Options struct {
	MinResponses     int      // item_analysis_min_responses (DefaultMinResponses when ≤ 0)
	OptionIDs        []string // every option of the version (choice types), for distractor rates and unused options
	CorrectOptionIDs []string
	Difficulty       int // declared difficulty 1..5
}

// Stats of one question version (and one grade level).
type Stats struct {
	FirstAttempts      int
	ExcludedRapidGuess int
	WithHint           int // valid (not rapid) answers with a hint: first − rapid − with_hint answers feed p_value
	Sufficient         bool
	PValue             *float64
	PValueWithHint     *float64
	Discrimination     *float64
	DistractorRates    map[string]float64
	AvgTimeMS          int
	RapidGuessRate     float64
	HintUsageRate      float64
	Flags              []string
}

// Compute applies QUESTION_BANK §9: rapid guesses are removed; answers with a hint are kept apart (p_value_with_hint);
// p_value, the point-biserial discrimination against the attempt total and the distractor rates use the valid answers
// without hint, and only once there are at least MinResponses of them.
func Compute(rs []Response, o Options) Stats {
	minN := o.MinResponses
	if minN <= 0 {
		minN = DefaultMinResponses
	}
	s := Stats{FirstAttempts: len(rs), DistractorRates: map[string]float64{}}
	var plain, hinted []Response
	var timeSum, timeN int
	for _, r := range rs {
		if r.RapidGuess {
			s.ExcludedRapidGuess++
			continue
		}
		if r.HintsUsed > 0 {
			s.WithHint++
		}
		if r.TimeMS != nil {
			timeSum += *r.TimeMS
			timeN++
		}
		if r.HintsUsed > 0 {
			hinted = append(hinted, r)
		} else {
			plain = append(plain, r)
		}
	}
	if len(rs) > 0 {
		s.RapidGuessRate = float64(s.ExcludedRapidGuess) / float64(len(rs))
		s.HintUsageRate = float64(s.WithHint) / float64(len(rs))
	}
	if timeN > 0 {
		s.AvgTimeMS = timeSum / timeN
	}
	if len(plain) < minN {
		return s
	}
	s.Sufficient = true
	p := correctRate(plain)
	s.PValue = &p
	if len(hinted) > 0 {
		ph := correctRate(hinted)
		s.PValueWithHint = &ph
	}
	s.Discrimination = pointBiserial(plain)
	for _, id := range o.OptionIDs {
		if slices.Contains(o.CorrectOptionIDs, id) {
			continue
		}
		n := 0
		for _, r := range plain {
			if slices.Contains(r.Options, id) {
				n++
			}
		}
		s.DistractorRates[id] = float64(n) / float64(len(plain))
	}
	s.Flags = flags(s, o.Difficulty)
	return s
}

func correctRate(rs []Response) float64 {
	n := 0
	for _, r := range rs {
		if r.Correct {
			n++
		}
	}
	return float64(n) / float64(len(rs))
}

// pointBiserial is r_pb = (M1 − M0) / S × √(p·q) with S the population standard deviation of the totals; nil when every
// answer is right, every answer is wrong or the totals do not vary.
func pointBiserial(rs []Response) *float64 {
	var sum, sum1 float64
	n1 := 0
	for _, r := range rs {
		sum += r.Total
		if r.Correct {
			sum1 += r.Total
			n1++
		}
	}
	n := len(rs)
	if n1 == 0 || n1 == n {
		return nil
	}
	mean := sum / float64(n)
	var ss float64
	for _, r := range rs {
		ss += (r.Total - mean) * (r.Total - mean)
	}
	sd := math.Sqrt(ss / float64(n))
	if sd == 0 {
		return nil
	}
	m1, m0 := sum1/float64(n1), (sum-sum1)/float64(n-n1)
	p := float64(n1) / float64(n)
	d := (m1 - m0) / sd * math.Sqrt(p*(1-p))
	return &d
}

func flags(s Stats, declared int) []string {
	var out []string
	if s.PValue != nil && declared >= 1 && abs(ProposedDifficulty(*s.PValue)-declared) >= 1 {
		out = append(out, FlagNeedsCalibration)
	}
	ids := make([]string, 0, len(s.DistractorRates))
	for id := range s.DistractorRates {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	unused, suggest := false, false
	for _, id := range ids {
		r := s.DistractorRates[id]
		unused = unused || r == 0
		suggest = suggest || r >= SuggestFeedbackRate
	}
	if unused {
		out = append(out, FlagUnusedOption)
	}
	if suggest {
		out = append(out, FlagSuggestOptionFeedback)
	}
	if s.Discrimination == nil || *s.Discrimination < LowDiscrimination {
		out = append(out, FlagLowDiscrimination)
	}
	return out
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
