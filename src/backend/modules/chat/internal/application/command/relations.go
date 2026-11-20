package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// RemoveFriend implements removeFriend (F095-AC-12): soft-delete of the friendship, friend.removed, no notification
// to the other student; not a friend → 404.
func (h Social) RemoveFriend(ctx context.Context, other uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.RemoveFriend")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return err
	}
	if other == me {
		return notFound()
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		now := h.now()
		if err := h.Repo.LockUsers(ctx, me, other); err != nil {
			return err
		}
		removed, err := h.Repo.RemoveFriendship(ctx, tenant, social.NewPair(me, other), social.RemovedByUser)
		if err != nil {
			return err
		}
		if !removed {
			return notFound()
		}
		if err := h.friendRemoved(ctx, tenant, me, other, social.RemovedByUser, now); err != nil {
			return err
		}
		return h.invalidate(ctx, me, other)
	})
}

func (h Social) friendRemoved(ctx context.Context, tenant, me, other uuid.UUID, reason string, now time.Time) error {
	p := social.NewPair(me, other)
	if err := h.publish(ctx, "friend.removed", me, tenant, now, map[string]any{"tenant_id": tenant, "user_low_id": p.Low,
		"user_high_id": p.High, "reason": reason, "source_version": version(now)}); err != nil {
		return err
	}
	return h.record(ctx, AuditFriendRemoved, other, map[string]any{"reason": reason})
}

// relationTarget validates the target of a block or mute (F095-AC-18): self → 422; not a student of the session
// tenant → 404.
func (h Social) relationTarget(ctx context.Context, tenant, me, other uuid.UUID) error {
	if other == me {
		return self("user_id")
	}
	if _, err := h.caller(ctx, tenant, me); err != nil {
		return err
	}
	_, err := h.target(ctx, tenant, other, false)
	return err
}

// BlockUser implements blockUser (F095-AC-14): idempotent (a live block → 204, nothing written); ends the friendship
// (friend.removed reason blocked), cancels the pending requests of both directions, user.blocked + audit, all in one
// transaction. The blocked student is never notified.
func (h Social) BlockUser(ctx context.Context, other uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.BlockUser")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return err
	}
	if other == me {
		return self("user_id")
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		now := h.now()
		if err := h.relationTarget(ctx, tenant, me, other); err != nil {
			return err
		}
		if err := h.Repo.LockUsers(ctx, me, other); err != nil {
			return err
		}
		created, err := h.Repo.SetBlock(ctx, tenant, me, other, true)
		if err != nil || !created {
			return err
		}
		removed, err := h.Repo.RemoveFriendship(ctx, tenant, social.NewPair(me, other), social.RemovedByBlock)
		if err != nil {
			return err
		}
		if removed {
			if err := h.friendRemoved(ctx, tenant, me, other, social.RemovedByBlock, now); err != nil {
				return err
			}
		}
		if _, err := h.Repo.CancelPending(ctx, tenant, me, other, now); err != nil {
			return err
		}
		if err := h.publish(ctx, "user.blocked", me, tenant, now, map[string]any{"tenant_id": tenant, "blocker_id": me,
			"blocked_id": other, "source_version": version(now)}); err != nil {
			return err
		}
		if err := h.record(ctx, AuditUserBlocked, other, nil); err != nil {
			return err
		}
		if h.Metrics != nil {
			h.Metrics.Block()
		}
		return h.invalidate(ctx, me, other)
	})
}

// UnblockUser implements unblockUser (F095-AC-16): soft-delete of the block, user.unblocked; the friendship is not
// restored; no block → 404.
func (h Social) UnblockUser(ctx context.Context, other uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.UnblockUser")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return err
	}
	if other == me {
		return self("user_id")
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		now := h.now()
		if err := h.relationTarget(ctx, tenant, me, other); err != nil {
			return err
		}
		if err := h.Repo.LockUsers(ctx, me, other); err != nil {
			return err
		}
		removed, err := h.Repo.SetBlock(ctx, tenant, me, other, false)
		if err != nil {
			return err
		}
		if !removed {
			return notFound()
		}
		if err := h.publish(ctx, "user.unblocked", me, tenant, now, map[string]any{"tenant_id": tenant, "blocker_id": me,
			"blocked_id": other, "source_version": version(now)}); err != nil {
			return err
		}
		if err := h.record(ctx, AuditUserUnblocked, other, nil); err != nil {
			return err
		}
		return h.invalidate(ctx, me, other)
	})
}

// MuteUser implements muteUser and UnmuteUser unmuteUser (F095-AC-17): idempotent, user.muted / user.unmuted + audit
// when the relation changed; only the muter is affected and the muted student is never told.
func (h Social) MuteUser(ctx context.Context, other uuid.UUID) error {
	return h.setMute(ctx, other, true)
}

// UnmuteUser implements unmuteUser (see MuteUser).
func (h Social) UnmuteUser(ctx context.Context, other uuid.UUID) error {
	return h.setMute(ctx, other, false)
}

func (h Social) setMute(ctx context.Context, other uuid.UUID, on bool) (err error) {
	name, typ, action := "chat.command.UnmuteUser", "user.unmuted", AuditUserUnmuted
	if on {
		name, typ, action = "chat.command.MuteUser", "user.muted", AuditUserMuted
	}
	ctx, done := observability.UseCase(ctx, name)
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return err
	}
	if other == me {
		return self("user_id")
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		now := h.now()
		if err := h.relationTarget(ctx, tenant, me, other); err != nil {
			return err
		}
		if err := h.Repo.LockUsers(ctx, me, other); err != nil {
			return err
		}
		changed, err := h.Repo.SetMute(ctx, tenant, me, other, on)
		if err != nil || !changed {
			return err
		}
		if err := h.publish(ctx, typ, me, tenant, now, map[string]any{"tenant_id": tenant, "muter_id": me, "muted_id": other,
			"source_version": version(now)}); err != nil {
			return err
		}
		return h.record(ctx, action, other, nil)
	})
}
