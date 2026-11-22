package port

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ExpiredChannel is a closed gameplay channel whose purge_after has passed (any tenant: the purger lists them across
// tenants, then works on each one in a transaction of its tenant).
type ExpiredChannel struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	ChannelType string
	PurgeAfter  time.Time
}

// Holds are the moderation reasons to keep a channel (F117-AC-18): reports not yet triaged and cases still being worked
// on (open, in_review, escalated, reopened) that point at the channel or one of its messages.
type Holds struct {
	OpenReports int
	OpenCases   int
}

// Held reports a channel that must not be purged yet.
func (h Holds) Held() bool { return h.OpenReports > 0 || h.OpenCases > 0 }

// DayStats is one row of chat.gameplay_chat_daily_stats added by a rollup (no content, no user).
type DayStats struct {
	Day         time.Time `json:"day"`
	ChannelType string    `json:"channel_type"`
	Channels    int       `json:"channels"`
	Messages    int       `json:"messages"`
	Masked      int       `json:"masked"`
	Blocked     int       `json:"blocked"`
	Reported    int       `json:"reported"`
}

// ChannelPurge is what the purge of one channel did.
type ChannelPurge struct {
	Messages int64
	Evidence int
	Stats    []DayStats
}

// PurgeRun is one row of chat.gameplay_purge_runs (aggregates only, no content).
type PurgeRun struct {
	RunID             uuid.UUID
	TenantID          *uuid.UUID // nil: the run-level row (blocked run, partitions)
	Kind              string     // gameplay | ephemeral
	StartedAt         time.Time
	FinishedAt        time.Time
	ChannelsPurged    int
	MessagesPurged    int64
	PartitionsDropped int
	DeferredOpenCase  int
	EvidenceCopied    int
	Status            string // succeeded | partial | failed | blocked
}

// Purge run statuses and kinds.
const (
	RunSucceeded  = "succeeded"
	RunPartial    = "partial"
	RunFailed     = "failed"
	RunBlocked    = "blocked"
	KindGameplay  = "gameplay"
	KindEphemeral = "ephemeral"
)

// PurgeStore is the purge of module chat (ADR-0033): the ONLY code that deletes rows, on the gameplay tables only, with
// the database role chat_purger (implemented in internal/infrastructure/purge; R21). Methods with a tenant run inside a
// transaction of that tenant (UnitOfWork of the purger pool); the others without a scope (purger RLS: closed gameplay
// channels only).
type PurgeStore interface {
	// ModerationReady reports that the sources of the holds and of the evidence exist (reports, cases, case actions,
	// sanctions, evidence) and are readable by the purger: false → nothing is deleted (fail-closed, F117-AC-19).
	ModerationReady(ctx context.Context) (bool, error)
	// Expired lists at most limit closed gameplay channels with purge_after < now, oldest first.
	Expired(ctx context.Context, now time.Time, limit int) ([]ExpiredChannel, error)
	// Holds returns the holds of a channel of tenant.
	Holds(ctx context.Context, tenant, channel uuid.UUID) (Holds, error)
	// PurgeChannel, in the transaction of tenant: rollup of the daily stats, copy of the referenced messages (and their
	// context) into chat.moderation_evidence, deletion in batches of the messages, the members and the channel, and
	// gameplay_partition_channels.purged_at. Any error rolls everything back (the channel stays, fail-closed).
	PurgeChannel(ctx context.Context, c ExpiredChannel, contextN, batch int, at time.Time) (ChannelPurge, error)
	// DropGameplayPartitions detaches and drops the daily partitions whose every channel was purged (returns the count).
	DropGameplayPartitions(ctx context.Context) (int, error)
	// EphemeralTenants lists the tenants having world/area messages created before cutoff.
	EphemeralTenants(ctx context.Context, cutoff time.Time) ([]uuid.UUID, error)
	// CopyEphemeralEvidence copies, in the transaction of tenant, the world/area messages created before cutoff that a
	// report, case or case action references (with their context) into chat.moderation_evidence.
	CopyEphemeralEvidence(ctx context.Context, tenant uuid.UUID, cutoff time.Time, contextN int, at time.Time) (int, error)
	// DropEphemeralPartitions drops the daily partitions entirely before cutoff (returns the count).
	DropEphemeralPartitions(ctx context.Context, cutoff time.Time) (int, error)
	// DeleteEphemeral deletes in batches, in the transaction of tenant, the remaining world/area messages created before
	// cutoff (default partition, partial day).
	DeleteEphemeral(ctx context.Context, tenant uuid.UUID, cutoff time.Time, batch int) (int64, error)
	// RecordRun appends a run row (transaction of its tenant, or no scope for the run-level row).
	RecordRun(ctx context.Context, r PurgeRun) error
	// EphemeralRetention returns chat_retention.ephemeral_retention_hours projected from identity (nil: not projected).
	EphemeralRetention(ctx context.Context) (*time.Duration, error)
	// EnsurePartitions creates the daily partitions of the message tables from day for days days; it returns the
	// partitions created and the consecutive days covered ahead per table.
	EnsurePartitions(ctx context.Context, day time.Time, days int) (created int, ahead map[string]int, err error)
}

// PurgeMetrics are the metrics of the purge (labels tenant/kind/reason only; no user, R14).
type PurgeMetrics interface {
	Purged(tenant, kind string, n int)
	Deferred(reason string)
	Blocked(reason string, on bool)
	Backlog(channels int, oldest time.Duration)
	Failed(stage string)
	PartitionsAhead(table string, days int)
}

// GameplayMetrics are the metrics of the gameplay channel lifecycle (no user label, R14).
type GameplayMetrics interface {
	Skipped(reason string)
	Opened(channelType string)
	Closed(channelType string)
	RetentionFloored()
}

// SeqExpirer sets a TTL on the Valkey seq counter of a closed channel (VALKEY_DESIGN §4.9: no message is accepted any more).
type SeqExpirer interface {
	Expire(ctx context.Context, channel uuid.UUID, ttl time.Duration) error
}
