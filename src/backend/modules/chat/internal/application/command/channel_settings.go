package command

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// AuditChannelMembershipUpdated is the audit action of updateChannelMembership (ids and levels only, no message).
const AuditChannelMembershipUpdated = "chat.channel_membership.updated"

// Notification levels of a channel member (CHAT_SYSTEM §10; chat.channel_members.notification_level).
const (
	NotificationAll      = "all"
	NotificationMentions = "mentions"
	NotificationNone     = "none"
)

// NotificationLevels lists the levels of the contract (ChannelNotificationLevel).
var NotificationLevels = []string{NotificationAll, NotificationMentions, NotificationNone}

// ChannelMembership is the answer of updateChannelMembership.
type ChannelMembership struct {
	ChannelID         uuid.UUID
	NotificationLevel string
}

// Muted reports notification_level = none.
func (m ChannelMembership) Muted() bool { return m.NotificationLevel == NotificationNone }

// Body renders ChannelMembership of the contract.
func (m ChannelMembership) Body() map[string]any {
	return map[string]any{"channel_id": m.ChannelID, "notification_level": m.NotificationLevel, "muted": m.Muted()}
}

// ChannelSettings is updateChannelMembership (delta F112-9, CHAT_SYSTEM §9 "Mute channel"): the notification level of
// the caller in a channel it is an active member of. A notification setting only: the caller still receives and reads
// every message, unread still counts, and nothing of the moderation pipeline, reports, blocks or mutes changes (R15).
type ChannelSettings struct {
	UoW   write.UnitOfWork
	Facts service.Facts // membership of the caller (cached facts of the send path)
	Repo  write.MembershipSettings
	Audit write.Auditor
	Now   func() time.Time
}

// SetNotificationLevel implements updateChannelMembership: a student of the session tenant only (game_player); an
// unknown channel, another tenant's channel or a channel the caller is not an active member of → 404; World/Area (no
// membership) → 422; the same level again changes nothing and writes no audit; a change is audited in the transaction.
func (h ChannelSettings) SetNotificationLevel(ctx context.Context, ch uuid.UUID, level string) (out ChannelMembership, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.UpdateChannelMembership")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return out, err
	}
	if !slices.Contains(NotificationLevels, level) {
		return out, problem.Invalid("notification_level", problem.SubInvalid, "all, mentions or none")
	}
	facts, err := h.Facts.Channel(ctx, tenant, ch)
	if err != nil {
		return out, problem.New(problem.ChatTemporarilyUnavailable, "").Retry()
	}
	if facts == nil {
		return out, notFound()
	}
	if facts.Channel.Type.Implicit() {
		return out, problem.Invalid("channel_id", problem.SubInvalid, "world and area channels have no membership")
	}
	if _, ok := facts.Members.Active(me); !ok {
		return out, notFound()
	}
	now := time.Now().UTC()
	if h.Now != nil {
		now = h.Now().UTC()
	}
	err = h.UoW.InTenantTx(ctx, mustScope(tenant), func(ctx context.Context) error {
		before, ok, err := h.Repo.SetNotificationLevel(ctx, tenant, ch, me, level, now)
		if err != nil {
			return err
		}
		if !ok { // left between the cached facts and the write
			return notFound()
		}
		out = ChannelMembership{ChannelID: ch, NotificationLevel: level}
		if before != level && h.Audit != nil {
			if err := h.Audit.Record(ctx, audit.Event{Action: AuditChannelMembershipUpdated, TargetType: "chat_channel",
				TargetID: ch.String(), Result: audit.ResultSuccess,
				Metadata: map[string]any{"channel_type": string(facts.Channel.Type), "before": before, "after": level}}); err != nil {
				return err
			}
		}
		return idempotency.SetResult(ctx, out.Body())
	})
	if err != nil {
		return ChannelMembership{}, err
	}
	return out, nil
}
