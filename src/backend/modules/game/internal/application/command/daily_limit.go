package command

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// warningTick is the period of the job curfew-session-closer (job.CurfewCloserInterval): a session whose remaining
// play time crosses curfew_warning_minutes during one tick gets one session.session.ending_soon.
const warningTick = time.Minute

// dailyAllowance is the daily play limit of a student at now (OQ-17, CHILD_SAFETY §4.2, F014-AC-27).
type dailyAllowance struct {
	Limited   bool
	Remaining time.Duration // limit − time played today (ICT) including active sessions; ≤ 0 = reached
	Reopen    time.Time     // 00:00 ICT the next day
}

// Reached reports a limited student who already played the limit today.
func (a dailyAllowance) Reached() bool { return a.Limited && a.Remaining <= 0 }

func (h Sessions) location() *time.Location {
	if h.Config.CurfewLocation != nil {
		return h.Config.CurfewLocation
	}
	return time.FixedZone("ICT", 7*3600)
}

// dailyAllowance reads daily_play_limit_minutes of the school (projection of school.updated; none for the common
// tenant) and the time the student played today (ICT) in the tenant, inside the transaction of ctx.
func (h Sessions) dailyAllowance(ctx context.Context, tenant, student, school uuid.UUID, now time.Time) (dailyAllowance, error) {
	if school == uuid.Nil {
		return dailyAllowance{}, nil
	}
	limit, err := h.Directory.DailyPlayLimit(ctx, tenant, school)
	if err != nil || limit == nil {
		return dailyAllowance{}, err
	}
	loc := h.location()
	y, m, d := now.In(loc).Date()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, loc)
	played, err := h.Sessions.PlayedSince(ctx, tenant, student, midnight, now)
	if err != nil {
		return dailyAllowance{}, err
	}
	return dailyAllowance{Limited: true, Remaining: time.Duration(*limit)*time.Minute - played,
		Reopen: time.Date(y, m, d+1, 0, 0, 0, 0, loc)}, nil
}

// warnNow reports whether the job sends the daily-limit warning in this tick: the remaining time just crossed
// curfew_warning_minutes, or the session started inside the warning period (it would never cross it).
func (h Sessions) warnNow(a dailyAllowance, startedAt, now time.Time) bool {
	w := h.Config.WarningBefore
	if !a.Limited || a.Remaining <= 0 || w <= 0 || a.Remaining > w {
		return false
	}
	return a.Remaining+warningTick > w || now.Sub(startedAt) < warningTick
}
