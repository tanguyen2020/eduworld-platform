package itemanalysis

import "time"

// CalibrationConfig is difficulty_calibration (CONFIG_DEFAULTS [Confirmed — Technical Architect 2026-11-15], owner
// QUESTION_BANK §9.1): ≥ 100 first answers once rapid guesses are removed (same grade level), the p_value table below, and
// no new proposal for 90 days after two consecutive dismissals.
type CalibrationConfig struct {
	MinResponses    int
	SuppressAfter   int
	SuppressForDays int
}

// DefaultCalibrationConfig returns the defaults of QUESTION_BANK §9.1.
func DefaultCalibrationConfig() CalibrationConfig {
	return CalibrationConfig{MinResponses: 100, SuppressAfter: 2, SuppressForDays: 90}
}

// ProposedDifficulty maps a p_value to a difficulty (QUESTION_BANK §9.1): ≥ 0.85 → 1, ≥ 0.70 → 2, ≥ 0.50 → 3, ≥ 0.30 → 4,
// else 5.
func ProposedDifficulty(p float64) int {
	switch {
	case p >= 0.85:
		return 1
	case p >= 0.70:
		return 2
	case p >= 0.50:
		return 3
	case p >= 0.30:
		return 4
	}
	return 5
}

// PHat is the estimated p of a question without enough data: the middle of its band (1 → 0.90, 2 → 0.77, 3 → 0.60,
// 4 → 0.40, 5 → 0.20; QUESTION_BANK §9.1, EXAM_ENGINE §4.1). An unknown difficulty reads as 3.
func PHat(difficulty int) float64 {
	switch difficulty {
	case 1:
		return 0.90
	case 2:
		return 0.77
	case 4:
		return 0.40
	case 5:
		return 0.20
	}
	return 0.60
}

// PriorState is what is known of the earlier proposals of a question.
type PriorState struct {
	Pending               bool
	ConsecutiveDismissals int
	SuppressedUntil       *time.Time
}

// Propose returns the proposed difficulty when a weekly run must create a proposal (F044-AC-19): enough answers, at least
// one level away from the declared difficulty, no pending proposal, not suppressed.
func Propose(declared int, p float64, n int, st PriorState, cfg CalibrationConfig, now time.Time) (int, bool) {
	if n < cfg.MinResponses || st.Pending {
		return 0, false
	}
	if st.SuppressedUntil != nil && now.Before(*st.SuppressedUntil) {
		return 0, false
	}
	d := ProposedDifficulty(p)
	if abs(d-declared) < 1 {
		return 0, false
	}
	return d, true
}

// Dismiss is the state after a dismissal: the second consecutive one suppresses the question for SuppressForDays.
func Dismiss(st PriorState, cfg CalibrationConfig, now time.Time) PriorState {
	st.Pending = false
	st.ConsecutiveDismissals++
	if st.ConsecutiveDismissals >= cfg.SuppressAfter {
		until := now.AddDate(0, 0, cfg.SuppressForDays)
		st.SuppressedUntil = &until
	}
	return st
}

// Accept is the state after an accepted proposal: the dismissal streak restarts.
func Accept(PriorState) PriorState {
	return PriorState{}
}
