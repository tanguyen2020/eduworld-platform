package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
)

// Delete reasons of the channel memberships ended by the identity sagas (chat.channel_members.delete_reason).
const (
	MemberDeletedCleanup    = "inactive_cleanup"
	MemberDeletedAnonymized = "anonymized"
	MemberDeletedMerge      = "user_merge"
)

// Relation is one live block or mute involving a user (social.blocks / social.mutes).
type Relation struct {
	Kind string // "block" or "mute"
	From uuid.UUID
	To   uuid.UUID
}

// LifecycleStore is the write side of the chat part of the identity sagas (F115 T042, OQ F016-2, F045-3): student
// cleanup and restore (ADR-0032), user anonymisation and user merge (ADR-0028). Soft-delete and UPDATE only — never a
// DELETE (R21); every statement has explicit tenant_id conditions in the TenantScope of the step (R8, R27).
type LifecycleStore interface {
	// OpenCaseOf reports a live moderation case not resolved yet (open, in_review, escalated, reopened) whose subject or
	// user target is user.
	OpenCaseOf(ctx context.Context, tenant, user uuid.UUID) (bool, error)
	// EndMemberships soft-deletes the active memberships of user (deleted_at, delete_reason = reason); returns the channels.
	EndMemberships(ctx context.Context, tenant, user uuid.UUID, reason string, at time.Time) ([]uuid.UUID, error)
	// RestoreMemberships brings back the memberships of user soft-deleted with reason (newest per channel) in live,
	// non-gameplay channels not closed, when the pair has no live row; returns the channels.
	RestoreMemberships(ctx context.Context, tenant, user uuid.UUID, reason string, at time.Time) ([]uuid.UUID, error)
	// MoveMemberships gives into the active memberships of from in live, non-gameplay channels (role, joined_at, read
	// cursors and notification level of from) unless into is already an active member; every active membership of from
	// is then soft-deleted (user_merge). It returns the channels touched.
	MoveMemberships(ctx context.Context, tenant, from, into uuid.UUID, at time.Time) ([]uuid.UUID, error)
	// LiveChannels returns the channels where user is an active member.
	LiveChannels(ctx context.Context, tenant, user uuid.UUID) ([]uuid.UUID, error)
	// AuthoredMessages returns at most limit messages of chat.messages sent by user and not anonymised yet, oldest first.
	AuthoredMessages(ctx context.Context, tenant, user uuid.UUID, limit int) ([]uuid.UUID, error)
	// CopyEvidence copies the messages referenced by a report, case or case action (±contextN) into
	// chat.moderation_evidence before the anonymisation (evidence is never lost); returns how many copies were new.
	CopyEvidence(ctx context.Context, tenant uuid.UUID, messages []uuid.UUID, contextN int, at time.Time) (int, error)
	// AnonymizeMessages overwrites body, body_display, quote and the revisions of the messages, sets anonymized_at and
	// clears the quote of the replies to them; returns how many messages were anonymised.
	AnonymizeMessages(ctx context.Context, tenant uuid.UUID, messages []uuid.UUID, at time.Time) (int, error)
	// AnonymizeCard clears nickname, avatar and title of the projected public card of user (ref.player_profiles,
	// source_version kept so a newer game_profile.updated still applies); false when nothing was left.
	AnonymizeCard(ctx context.Context, tenant, user uuid.UUID) (bool, error)
	// Relations returns the live blocks and mutes where user is either side.
	Relations(ctx context.Context, tenant, user uuid.UUID) ([]Relation, error)
	// ActiveSanctions returns the live sanctions of user neither revoked, lifted nor ended at at.
	ActiveSanctions(ctx context.Context, tenant, user uuid.UUID, at time.Time) ([]read.SanctionDetail, error)
	// SanctionExists reports a sanction row with this id in tenant (idempotency of the copies of a merge).
	SanctionExists(ctx context.Context, tenant, id uuid.UUID) (bool, error)
}
