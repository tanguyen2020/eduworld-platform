package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// Audit actions of the internal channel commands (ids only).
const (
	AuditChannelOpened = "chat.channel.opened"
	AuditMemberAdded   = "chat.channel.member_added"
	AuditMemberRemoved = "chat.channel.member_removed"
)

// ErrNotStudent: AddMember of someone who is not an active projected student of the tenant (staff is never a member,
// ADR-0031, ADR-0072).
var ErrNotStudent = errors.New("chat: only an active student of the tenant can be a channel member")

// ErrUnknownChannel: AddMember/RemoveMember of a channel unknown in the tenant.
var ErrUnknownChannel = errors.New("chat: unknown channel")

// FactsInvalidator drops the cached facts of a channel after the commit (service.Facts).
type FactsInvalidator interface {
	InvalidateChannel(ctx context.Context, tenant, id uuid.UUID) error
}

// Channels are the internal write commands of the channel aggregate (Clarifications 2026-11-16): no public API; the
// consumers of F117 (gameplay channels) and F120 (persistent channels) and the test fixtures call them in the event or
// job scope. Each runs in one UnitOfWork with its audit; the cached facts are dropped after the commit.
type Channels struct {
	UoW         write.UnitOfWork
	AfterCommit write.AfterCommit
	Repo        write.ChannelRepository
	Audit       write.Auditor
	Facts       FactsInvalidator // nil: no cache
	Now         func() time.Time
}

func (h Channels) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h Channels) invalidate(ctx context.Context, tenant, id uuid.UUID) error {
	if h.Facts == nil || h.AfterCommit == nil {
		return nil
	}
	return h.AfterCommit(ctx, func(ctx context.Context) { _ = h.Facts.InvalidateChannel(ctx, tenant, id) })
}

func (h Channels) record(ctx context.Context, action string, id uuid.UUID, meta map[string]any) error {
	if h.Audit == nil {
		return nil
	}
	return h.Audit.Record(ctx, audit.Event{Action: action, TargetType: "chat_channel", TargetID: id.String(), Result: audit.ResultSuccess,
		Metadata: meta})
}

// OpenChannelInput describes a channel to open (id deterministic for gameplay channels: UUIDv5, CHAT_SYSTEM §4.1).
type OpenChannelInput struct {
	ID       uuid.UUID
	TenantID uuid.UUID
	SchoolID *uuid.UUID
	Type     channel.Type
	ScopeRef map[string]string
	Members  []uuid.UUID
}

// OpenChannel creates the channel (idempotent on its id) and adds the members; it returns whether the channel was new.
func (h Channels) OpenChannel(ctx context.Context, in OpenChannelInput) (created bool, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.OpenChannel")
	defer func() { done(err) }()
	now := h.now()
	ch, err := channel.New(in.ID, in.TenantID, in.SchoolID, in.Type, in.ScopeRef, now)
	if err != nil {
		return false, err
	}
	err = h.UoW.InTenantTx(ctx, mustScope(in.TenantID), func(ctx context.Context) error {
		created, err = h.Repo.CreateChannel(ctx, ch)
		if err != nil {
			return err
		}
		if created {
			if err := h.record(ctx, AuditChannelOpened, ch.ID, map[string]any{"channel_type": string(ch.Type)}); err != nil {
				return err
			}
		}
		for _, u := range in.Members {
			if err := h.join(ctx, ch, u, now); err != nil {
				return err
			}
		}
		return h.invalidate(ctx, in.TenantID, ch.ID)
	})
	return created, err
}

func (h Channels) join(ctx context.Context, ch channel.Channel, user uuid.UUID, now time.Time) error {
	school, ok, err := h.Repo.StudentSchool(ctx, ch.TenantID, user)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotStudent
	}
	changed, err := h.Repo.JoinMember(ctx, channel.Member{ChannelID: ch.ID, UserID: user, TenantID: ch.TenantID, SchoolID: &school,
		Role: channel.RoleMember, JoinedAt: now})
	if err != nil || !changed {
		return err
	}
	return h.record(ctx, AuditMemberAdded, ch.ID, map[string]any{"user_id": user.String()})
}

// AddMember makes user an active member of the channel (no-op when already active).
func (h Channels) AddMember(ctx context.Context, tenant, id, user uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.AddMember")
	defer func() { done(err) }()
	return h.UoW.InTenantTx(ctx, mustScope(tenant), func(ctx context.Context) error {
		ch, err := h.Repo.Channel(ctx, tenant, id)
		if err != nil {
			return err
		}
		if ch == nil {
			return ErrUnknownChannel
		}
		if err := h.join(ctx, *ch, user, h.now()); err != nil {
			return err
		}
		return h.invalidate(ctx, tenant, id)
	})
}

// RemoveMember sets left_at of an active member: the next send answers NOT_FOUND and the fan-out no longer targets
// the user (edge case of spec.md).
func (h Channels) RemoveMember(ctx context.Context, tenant, id, user uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.RemoveMember")
	defer func() { done(err) }()
	return h.UoW.InTenantTx(ctx, mustScope(tenant), func(ctx context.Context) error {
		changed, err := h.Repo.LeaveMember(ctx, tenant, id, user, h.now())
		if err != nil || !changed {
			return err
		}
		if err := h.record(ctx, AuditMemberRemoved, id, map[string]any{"user_id": user.String()}); err != nil {
			return err
		}
		return h.invalidate(ctx, tenant, id)
	})
}
