package attempt

import "math"

// RapidGuess is rapid_guess_min_response_ms (CONFIG_DEFAULTS, owner LEARNING_ANALYTICS §3.1): the default minimum
// response time per question type and the factor of grades 1–3 (ACTIVITY_ENGINE §11.1, F032-AC-16, AC-19).
type RapidGuess struct {
	// ByType is the default threshold in milliseconds per QuestionType; a type absent from the map (interactive) is
	// never flagged by default.
	ByType map[string]int
	// G13Factor multiplies the default threshold for grades 1–3 (reading and listening to the stem take longer).
	G13Factor float64
}

// DefaultRapidGuess are the values of LEARNING_ANALYTICS §3.1 [Confirmed — Technical Architect 2026-11-15].
func DefaultRapidGuess() RapidGuess {
	return RapidGuess{ByType: map[string]int{"true_false": 1500, "single_choice": 2000, "image_based": 2000,
		"multiple_choice": 3000, "fill_in_blank": 3000, "short_answer": 3000, "numeric": 3000,
		"matching": 4000, "ordering": 4000, "drag_drop": 4000}, G13Factor: 1.5}
}

// fastModes have a fast pace by design: only the threshold of the mini-game or match configuration applies.
var fastModes = []Mode{ModeMinigame, ModeCompetition}

// GuessInput is one scored answer to classify.
type GuessInput struct {
	Mode         Mode
	QuestionType string
	// ItemMinMS is min_response_ms of the pinned question version (nil = not declared).
	ItemMinMS *int
	// ModeMinMS is the threshold of the mini-game or match configuration (nil = none).
	ModeMinMS  *int
	Grade      *int
	ResponseMS int64
	Scored     bool
}

// Threshold returns the minimum response time of an answer in milliseconds (0 = no threshold): the threshold of the
// mini-game/match for minigame and competition, otherwise min_response_ms of the question, otherwise the default of
// its type multiplied by G13Factor for grades 1–3.
func (r RapidGuess) Threshold(in GuessInput) int {
	for _, m := range fastModes {
		if in.Mode == m {
			if in.ModeMinMS != nil && *in.ModeMinMS > 0 {
				return *in.ModeMinMS
			}
			return 0
		}
	}
	if in.ItemMinMS != nil {
		return max(*in.ItemMinMS, 0)
	}
	ms, ok := r.ByType[in.QuestionType]
	if !ok {
		return 0
	}
	if in.Grade != nil && *in.Grade >= 1 && *in.Grade <= 3 && r.G13Factor > 0 {
		return int(math.Round(float64(ms) * r.G13Factor))
	}
	return ms
}

// Flag reports rapid_guess for a scored answer whose response time is below its threshold; an unscored retry is never
// flagged (F032-AC-16).
func (r RapidGuess) Flag(in GuessInput) bool {
	if !in.Scored {
		return false
	}
	t := r.Threshold(in)
	return t > 0 && in.ResponseMS < int64(t)
}
