package leaderboard

import "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/ranking"

// Sources of effort_points contributions (leaderboard_contributions.source_type).
const (
	SourceActivity  = "activity_completed"
	SourceReview    = "review_activity"
	SourceActiveDay = "active_day"
)

// EffortDay is what the student already earned today (Asia/Ho_Chi_Minh): counted activities, total effort points and
// whether the active-day bonus was given.
type EffortDay struct {
	Activities int
	Total      int
	Active     bool
}

// Activity is one graded learning activity of the student.
type Activity struct {
	// Review: a review activity suggested by the system or assigned by the school (+2 instead of +1).
	Review bool
	// RapidGuess: the attempt is flagged rapid_guess (LEARNING_ANALYTICS) — it earns nothing (F106-AC-16, AC-LDB-007).
	RapidGuess bool
}

// EffortAward is the effort of one activity after the caps: the activity points (source activity_completed or
// review_activity) and the active-day bonus.
type EffortAward struct {
	Activity       int
	ActivitySource string
	ActiveDay      int
}

// Effort computes effort_points (COMPETITION_SYSTEM §9.2): +1 per activity up to activity_daily_max a day, +2 for a
// review activity, +3 the first active day, at most daily_cap a day. Right or wrong never matters; rapid guesses never
// count.
func Effort(t ranking.EffortTable, a Activity, d EffortDay) EffortAward {
	out := EffortAward{ActivitySource: SourceActivity}
	if a.RapidGuess {
		return out
	}
	if a.Review {
		out.ActivitySource = SourceReview
	}
	room := max(t.DailyCap-d.Total, 0)
	if d.Activities < t.ActivityDailyMax {
		pts := t.Activity
		if a.Review {
			pts = t.Review
		}
		out.Activity = min(pts, room)
		room -= out.Activity
	}
	if !d.Active {
		out.ActiveDay = min(t.ActiveDay, room)
	}
	return out
}

// SourceXPAwarded is the source of xp contributions: one xp.awarded posting of module game (F074 T042).
const SourceXPAwarded = "xp_awarded"
