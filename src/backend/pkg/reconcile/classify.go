// Package reconcile is the cross-module event reconciliation of `worker reconcile --module=<module>` (job
// reconcile-<module>, F134 US1, BACKUP_RECOVERY §3.5, §8): after one module database is restored to T, it compares the
// outbox of each source module with the processed_events / dead_letters of each consumer, in both directions, and
// classifies every difference. It only reads (never writes business tables, never compensates — compensation is a
// manual business API call with audit). Data of other modules is read only through their facades (listOutboxEventIds,
// listProcessedEventIds) in process (pkg/modcall, ADR-0074); the facades answer IDs and metadata only, never payloads
// (AGENTS R14).
package reconcile

import "time"

// Direction of a comparison relative to the reconciled module.
type Direction string

const (
	// Outbound: events of the reconciled module consumed by another module.
	Outbound Direction = "outbound"
	// Inbound: events of another module consumed by the reconciled module.
	Inbound Direction = "inbound"
)

// Class of a mismatch (spec F134 Clarifications 2026-11-16).
type Class string

const (
	LostAfterRestore Class = "lost_after_restore" // the consumer processed an event the restored source no longer has, event time > T
	PendingPublish   Class = "pending_publish"    // still in the source outbox, not yet published by the relay
	PendingDelivery  Class = "pending_delivery"   // published within --settle, the consumer has not caught up yet
	DeadLettered     Class = "dead_lettered"      // the consumer moved it to its DLQ
	Unexplained      Class = "unexplained"        // anything else: event time ≤ T, or published past --settle and not processed nor in the DLQ
)

// Classes in report order.
var Classes = []Class{LostAfterRestore, PendingPublish, PendingDelivery, DeadLettered, Unexplained}

// Observation is what both sides say about one (consumer, event).
type Observation struct {
	Direction    Direction
	InSource     bool       // the event is in the source outbox
	PublishedAt  *time.Time // source published_at (nil = not published)
	Processed    bool       // in the consumer's processed_events
	DeadLettered bool       // in the consumer's dead_letters
	EventTime    time.Time  // UUIDv7 time of the event id
}

// Classify returns the class of one observation, or "" when both sides agree (F134-AC-01, AC-02). restorePoint is T,
// settle the --settle duration (reconcile_settle_minutes) and now the time of the run.
func Classify(o Observation, restorePoint time.Time, settle time.Duration, now time.Time) Class {
	switch {
	case o.InSource && o.Processed:
		return ""
	case !o.InSource:
		if !o.Processed && !o.DeadLettered {
			return ""
		}
		// The consumer has an event the source does not: the restore of the source lost it. Explained only for the
		// restored module's own events after T (the receiver kept what the source lost); inbound, the source was not
		// restored, so a missing source event is never explained.
		if o.Direction == Outbound && o.EventTime.After(restorePoint) {
			return LostAfterRestore
		}
		return Unexplained
	case o.DeadLettered:
		return DeadLettered
	case o.PublishedAt == nil:
		return PendingPublish
	case now.Sub(*o.PublishedAt) < settle:
		return PendingDelivery
	default:
		return Unexplained
	}
}
