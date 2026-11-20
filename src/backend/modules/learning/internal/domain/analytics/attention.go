package analytics

// Reason is why a student is on the "needs attention" list (SCHOOL_LMS §8.1, enum of the contract).
type Reason string

// Reasons of rm_student_attention.
const (
	ReasonMissedAssignments   Reason = "missed_assignments"
	ReasonBeginningObjectives Reason = "beginning_objectives"
	ReasonInactiveGame        Reason = "inactive_game"
	ReasonRapidGuess          Reason = "rapid_guess" // shown as "làm quá nhanh", never as cheating
	ReasonScheduledCleanup    Reason = "scheduled_cleanup"
)

// Thresholds is dashboard_attention_thresholds (CONFIG_DEFAULTS, [Confirmed — Technical Architect 2026-11-15]).
type Thresholds struct {
	MissedAssignments   int     // at least n assignments missed …
	MissedWindowDays    int     // … within these days
	BeginningObjectives int     // at least n objectives at level beginning
	InactiveDays        int     // no game login for at least n days
	RapidGuessRatio     float64 // share of rapid_guess answers at least …
	RapidWindowDays     int     // … within these days
}

// DefaultThresholds: ≥ 2 missed in 14 days · ≥ 3 objectives beginning · no game for ≥ 7 days · rapid_guess ≥ 30 % in 7 days.
func DefaultThresholds() Thresholds {
	return Thresholds{MissedAssignments: 2, MissedWindowDays: 14, BeginningObjectives: 3, InactiveDays: 7,
		RapidGuessRatio: 0.30, RapidWindowDays: 7}
}

// Signals are the facts of one student over the windows of the thresholds. DaysSinceLogin is nil while the login
// projection does not exist (spec assumption: "chưa có dữ liệu", the reason is never raised).
type Signals struct {
	Missed           int
	Beginning        int
	DaysSinceLogin   *int
	RapidNum         int64
	RapidDen         int64
	ScheduledCleanup bool
}

// Evaluate returns the reasons of a student in a stable order (F043-AC-08).
func (t Thresholds) Evaluate(s Signals) []Reason {
	out := []Reason{}
	if t.MissedAssignments > 0 && s.Missed >= t.MissedAssignments {
		out = append(out, ReasonMissedAssignments)
	}
	if t.BeginningObjectives > 0 && s.Beginning >= t.BeginningObjectives {
		out = append(out, ReasonBeginningObjectives)
	}
	if s.DaysSinceLogin != nil && t.InactiveDays > 0 && *s.DaysSinceLogin >= t.InactiveDays {
		out = append(out, ReasonInactiveGame)
	}
	if s.RapidDen > 0 && float64(s.RapidNum) >= t.RapidGuessRatio*float64(s.RapidDen) {
		out = append(out, ReasonRapidGuess)
	}
	if s.ScheduledCleanup {
		out = append(out, ReasonScheduledCleanup)
	}
	return out
}
