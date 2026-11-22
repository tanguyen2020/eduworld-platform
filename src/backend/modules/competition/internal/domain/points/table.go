// Package points is the pure domain of Competition Points (F106 US3, COMPETITION_SYSTEM §8.3): participation and effort
// are rewarded, a loss is never deducted, an abandon costs points but never takes the day below 0, and the daily cap
// (points and matches, competition_points_daily_cap) turns the excess into a 0 entry with reason capped.
package points

import "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/ranking"

// Reasons of a ledger entry (competition_point_ledger.reason).
const (
	ReasonCompleted = "completed"
	ReasonWin       = "win"
	ReasonDraw      = "draw"
	ReasonAbandon   = "abandon"
	ReasonCapped    = "capped"
)

// Match is the result of one ranked match for one student.
type Match struct {
	Outcome string // win, draw, loss, abandoned, forfeit
	Correct int
	// Upset: the student won against a higher tier (the opponent, or the virtual opponent of a team).
	Upset bool
}

// Day is what the student already has in the ledger today (Asia/Ho_Chi_Minh): the sum of the amounts and the number of
// match entries.
type Day struct {
	Total   int
	Matches int
}

// Award is the ledger entry of a match: the amount after the caps, its reason and the breakdown before the caps.
type Award struct {
	Amount    int
	Reason    string
	Capped    bool
	Breakdown map[string]int
}

// ForMatch computes the Competition Points of a ranked match (F106-AC-10, AC-11).
func ForMatch(t ranking.PointTable, m Match, day Day) Award {
	b := map[string]int{}
	reason := ReasonCompleted
	switch m.Outcome {
	case "abandoned", "forfeit":
		b["abandon"] = t.Abandon
		reason = ReasonAbandon
	default:
		b["completed"] = t.Completed
		switch m.Outcome {
		case "win":
			b["win"] = t.Win
			reason = ReasonWin
			if m.Upset {
				b["upset"] = t.Upset
			}
		case "draw":
			b["draw"] = t.Draw
			reason = ReasonDraw
		}
		if c := min(max(m.Correct, 0), t.CorrectMax); c > 0 {
			b["correct"] = c * t.Correct
		}
	}
	raw := 0
	for _, v := range b {
		raw += v
	}
	a := Award{Amount: raw, Reason: reason, Breakdown: b}
	switch {
	case t.DailyMatches > 0 && day.Matches >= t.DailyMatches:
		// after daily_matches matches the day gives 0, abandon included (no deduction past the cap either).
		a.Amount, a.Reason, a.Capped = 0, ReasonCapped, raw != 0
	case raw > 0:
		room := max(t.DailyCap-day.Total, 0)
		if raw > room {
			a.Amount, a.Reason, a.Capped = room, ReasonCapped, true
		}
	case raw < 0:
		// an abandon never takes the day below 0 (§8.3).
		a.Amount = max(raw, -max(day.Total, 0))
	}
	return a
}
