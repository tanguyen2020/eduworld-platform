package channel

import (
	"errors"
	"time"
)

// MinGameplayRetentionDays is the floor of gameplay_chat_retention_days (ADR-0033, CONFIG_DEFAULTS: "tối thiểu 3"): a
// lower effective value is raised to it (F117-AC-15). It is a rule of the ADR, not a tunable.
const MinGameplayRetentionDays = 3

// ErrNotGameplay: a gameplay lifecycle operation on a channel that is not match, party or team bound to a match.
var ErrNotGameplay = errors.New("channel: not a gameplay channel")

// Retention is the gameplay chat retention resolved for one tenant at closing time: Days as configured (tenant over
// platform, else the configured default) and Effective after the floor; Floored reports a value raised to the floor
// (the caller warns, F117-AC-15).
type Retention struct {
	Days      int
	Effective int
	Floored   bool
}

// ResolveRetention applies the floor of ADR-0033 to the configured number of days.
func ResolveRetention(days int) Retention {
	if days < MinGameplayRetentionDays {
		return Retention{Days: days, Effective: MinGameplayRetentionDays, Floored: true}
	}
	return Retention{Days: days, Effective: days}
}

// PurgeAfter is closed_at + the effective retention (CHAT_SYSTEM §6.1, DATABASE_DESIGN §3.8b step 2); it is fixed when
// the channel closes: a later settings change is never applied retroactively (research R4).
func (r Retention) PurgeAfter(closedAt time.Time) time.Time {
	return closedAt.UTC().AddDate(0, 0, r.Effective)
}

// Close makes an open gameplay channel read-only (match.completed / match.aborted / party.disbanded, F117-AC-05):
// closed_at and purge_after are set once; closing a closed channel is a no-op (changed false).
func (c *Channel) Close(at time.Time, r Retention) (changed bool, err error) {
	if !c.IsGameplay {
		return false, ErrNotGameplay
	}
	if !c.Open() {
		return false, nil
	}
	closed := at.UTC()
	purge := r.PurgeAfter(closed)
	c.Status, c.ClosedAt, c.PurgeAfter = StatusClosed, &closed, &purge
	return true, nil
}
