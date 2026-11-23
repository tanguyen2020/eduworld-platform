package port

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// RetentionMessage is one persistent message past the retention of its channel.
type RetentionMessage struct {
	MessageID uuid.UUID
	ChannelID uuid.UUID
	CreatedAt time.Time
}

// RetentionRun is one row of chat.persistent_retention_runs (counts only, no content).
type RetentionRun struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	StartedAt      time.Time
	FinishedAt     time.Time
	Channels       int
	Anonymized     int
	EvidenceCopied int
	Status         string
	ErrorCode      string
}

// RetentionStore is the write side of the job persistent-chat-retention (F120 US6): anonymisation of the persistent
// channels (club team, direct) — never DELETE, never a partition drop (R21, ADR-0070) — on the pool of chat_app, one
// tenant at a time (TenantScope).
type RetentionStore interface {
	// Tenants lists the projected tenants (ref.tenants, not tenant-scoped).
	Tenants(ctx context.Context) ([]uuid.UUID, error)
	// PlatformDays returns chat_retention.persistent_days projected from identity (nil: not projected).
	PlatformDays(ctx context.Context) (*int, error)
	// Expired returns at most limit messages of persistent channels of tenant older than the retention of their channel
	// (channels.retention_days, else days) not anonymised yet, oldest first.
	Expired(ctx context.Context, tenant uuid.UUID, days, limit int, now time.Time) ([]RetentionMessage, error)
	// CopyEvidence copies the messages referenced by a report, case or case action (with ±contextN messages) into
	// chat.moderation_evidence before anonymisation (F120-AC-25); returns how many copies were new.
	CopyEvidence(ctx context.Context, tenant uuid.UUID, messages []uuid.UUID, contextN int, at time.Time) (int, error)
	// Anonymize overwrites body, body_display, quote and the revisions of the messages and sets anonymized_at.
	Anonymize(ctx context.Context, tenant uuid.UUID, messages []uuid.UUID, at time.Time) (int, error)
	// RecordRun appends a run row.
	RecordRun(ctx context.Context, r RetentionRun) error
}
