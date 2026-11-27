package reconcile

import (
	"errors"
	"fmt"
	"time"
)

// Window of one run: [From, To) on the UUIDv7 time of the event ids, restore point T, --settle.
type Window struct {
	From, To     time.Time
	RestorePoint time.Time
	Settle       time.Duration
}

// ErrWindow marks an invalid window (exit code 1, no module is called — F134-AC-04).
var ErrWindow = errors.New("reconcile: invalid window")

// Validate checks the window against outbox_retention (the longest the source outboxes and processed_events are
// kept, DATABASE_DESIGN §3.9) before any module is called.
func (w Window) Validate(outboxRetention time.Duration, now time.Time) error {
	switch {
	case w.From.IsZero() || w.To.IsZero() || w.RestorePoint.IsZero():
		return fmt.Errorf("%w: --from, --to and --restore-point are required (RFC 3339)", ErrWindow)
	case !w.From.Before(w.To):
		return fmt.Errorf("%w: --from (%s) must be before --to (%s)", ErrWindow, w.From.Format(time.RFC3339), w.To.Format(time.RFC3339))
	case w.To.Sub(w.From) > outboxRetention:
		return fmt.Errorf("%w: window %s is longer than outbox_retention %s", ErrWindow, w.To.Sub(w.From), outboxRetention)
	case now.Sub(w.From) > outboxRetention:
		return fmt.Errorf("%w: --from is older than outbox_retention %s (rows already dropped)", ErrWindow, outboxRetention)
	case w.RestorePoint.Before(w.From) || w.RestorePoint.After(w.To):
		return fmt.Errorf("%w: --restore-point must be within [--from, --to]", ErrWindow)
	case w.Settle < 0:
		return fmt.Errorf("%w: --settle must not be negative", ErrWindow)
	}
	return nil
}
