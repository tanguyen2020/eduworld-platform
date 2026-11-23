package command

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Audit actions of the persistent channels (ids only).
const (
	AuditDirectFrozen   = "chat.channel.frozen"
	AuditDirectReopened = "chat.channel.reopened"
)

// Membership ops of chat.channel.membership_changed.
const (
	MembershipAdded         = "added"
	MembershipRemoved       = "removed"
	MembershipStatusChanged = "status_changed"
)

// Persistent are the write use cases of the persistent channels (F120 US1, US3): openDirectChannel, the freeze and reopen
// of a direct channel on user.blocked / user.unblocked, and the lazy creation of World/Area channels on the first send.
// Every change runs in one UnitOfWork with its audit; the cached facts are dropped and the student is told after the
// commit.
type Persistent struct {
	UoW         write.UnitOfWork
	AfterCommit write.AfterCommit
	Repo        write.ChannelRepository
	Status      write.StatusRepository
	Social      write.SocialRepository
	Directory   write.Directory
	Audit       write.Auditor
	Facts       service.Facts
	Implicit    *service.Implicit   // nil: no World/Area channel
	Push        port.MembershipPush // nil: no push
	Now         func() time.Time
}

func (h Persistent) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h Persistent) record(ctx context.Context, action string, id uuid.UUID, meta map[string]any) error {
	if h.Audit == nil {
		return nil
	}
	return h.Audit.Record(ctx, audit.Event{Action: action, TargetType: "chat_channel", TargetID: id.String(), Result: audit.ResultSuccess,
		Metadata: meta})
}

// afterCommit drops the cached facts of the channel and pushes chat.channel.membership_changed to users.
func (h Persistent) afterCommit(ctx context.Context, tenant uuid.UUID, ch channel.Channel, op, status string, users ...uuid.UUID) error {
	if h.AfterCommit == nil {
		return nil
	}
	return h.AfterCommit(ctx, func(ctx context.Context) {
		_ = h.Facts.InvalidateChannel(ctx, tenant, ch.ID)
		if h.Push == nil {
			return
		}
		for _, u := range users {
			if err := h.Push.MembershipChanged(ctx, u, ch.ID, string(ch.Type), op, status); err != nil {
				slog.WarnContext(ctx, "chat: membership change not pushed", "error", err)
			}
		}
	})
}

// DirectResult is the outcome of openDirectChannel.
type DirectResult struct {
	Channel channel.Channel
	Created bool
}

// OpenDirect implements openDirectChannel (F120-AC-14): the direct channel of the caller and another student of the
// tenant, created once per normalised pair when they are friends, do not block each other and the effective policy allows
// direct (tenant common: only once System Admin re-enabled it, F120-AC-28). Not friends / blocked / not allowed → 403
// CHAT_NOT_ALLOWED; unknown or another tenant's student → 404. An existing channel is returned as is (frozen included).
func (h Persistent) OpenDirect(ctx context.Context, other uuid.UUID) (res DirectResult, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.OpenDirectChannel")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return DirectResult{}, err
	}
	if other == me {
		return DirectResult{}, self("user_id")
	}
	id := channel.DirectID(tenant, me, other)
	now := h.now()
	err = h.UoW.InTenantTx(ctx, mustScope(tenant), func(ctx context.Context) error {
		mine, err := h.Directory.Student(ctx, tenant, me)
		if err != nil {
			return err
		}
		if mine == nil || !mine.Active() {
			return problem.New(problem.GameAccessStudentOnly, "")
		}
		them, err := h.Directory.Student(ctx, tenant, other)
		if err != nil {
			return err
		}
		if them == nil || !them.Active() {
			return notFound()
		}
		if err := h.Social.LockUsers(ctx, me, other); err != nil {
			return err
		}
		existing, err := h.Repo.Channel(ctx, tenant, id)
		if err != nil {
			return err
		}
		friends, err := h.Social.Friends(ctx, tenant, social.NewPair(me, other))
		if err != nil {
			return err
		}
		blocked, err := h.Social.Blocked(ctx, tenant, me, other)
		if err != nil {
			return err
		}
		if existing != nil {
			res = DirectResult{Channel: *existing}
			if existing.Frozen() && friends && !blocked { // friends again after an unblock: the channel reopens (edge case)
				if ok, err := h.Status.SetStatus(ctx, tenant, id, []string{channel.StatusFrozen}, channel.StatusOpen); err != nil || !ok {
					return err
				}
				res.Channel.Status = channel.StatusOpen
				if err := h.record(ctx, AuditDirectReopened, id, nil); err != nil {
					return err
				}
				return h.afterCommit(ctx, tenant, res.Channel, MembershipStatusChanged, channel.StatusOpen, me, other)
			}
			return nil
		}
		if !friends || blocked {
			return problem.New(problem.ChatNotAllowed, "")
		}
		var school *uuid.UUID
		if mine.SchoolID == them.SchoolID {
			s := mine.SchoolID
			school = &s
		}
		ch, err := channel.New(id, tenant, school, channel.Direct, channel.DirectScope(me, other), now)
		if err != nil {
			return err
		}
		if _, err := h.Facts.Effective(ctx, tenant, &mine.SchoolID, ch); err != nil {
			if errors.Is(err, policy.ErrDenied) {
				return problem.New(problem.ChatNotAllowed, "")
			}
			return problem.New(problem.ChatTemporarilyUnavailable, "").Retry()
		}
		created, err := h.Repo.CreateChannel(ctx, ch)
		if err != nil {
			return err
		}
		for _, s := range []social.Student{*mine, *them} {
			sc := s.SchoolID
			if _, err := h.Repo.JoinMember(ctx, channel.Member{ChannelID: id, UserID: s.ID, TenantID: tenant, SchoolID: &sc,
				Role: channel.RoleMember, JoinedAt: now}); err != nil {
				return err
			}
		}
		if created {
			if err := h.record(ctx, AuditChannelOpened, id, map[string]any{"channel_type": string(channel.Direct)}); err != nil {
				return err
			}
		}
		res = DirectResult{Channel: ch, Created: created}
		return h.afterCommit(ctx, tenant, ch, MembershipAdded, ch.Status, me, other)
	})
	if err != nil {
		return DirectResult{}, err
	}
	return res, nil
}

// SetDirectFrozen freezes (on, user.blocked) or reopens (off, user.unblocked — only when both are still friends and no
// block remains either way) the direct channel of a pair (F120-AC-15). Idempotent: a second delivery changes nothing.
func (h Persistent) SetDirectFrozen(ctx context.Context, tenant, a, b, source uuid.UUID, on bool) (changed bool, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.FreezeDirectChannel")
	defer func() { done(err) }()
	id := channel.DirectID(tenant, a, b)
	err = h.UoW.InTenantTx(ctx, mustScope(tenant), func(ctx context.Context) error {
		ch, err := h.Repo.Channel(ctx, tenant, id)
		if err != nil || ch == nil {
			return err
		}
		from, to, action := []string{channel.StatusOpen}, channel.StatusFrozen, AuditDirectFrozen
		if !on {
			blocked, err := h.Social.Blocked(ctx, tenant, a, b)
			if err != nil || blocked {
				return err
			}
			friends, err := h.Social.Friends(ctx, tenant, social.NewPair(a, b))
			if err != nil || !friends {
				return err
			}
			from, to, action = []string{channel.StatusFrozen}, channel.StatusOpen, AuditDirectReopened
		}
		changed, err = h.Status.SetStatus(ctx, tenant, id, from, to)
		if err != nil || !changed {
			return err
		}
		for _, u := range []uuid.UUID{a, b} {
			if err := h.Status.SetSource(ctx, tenant, id, u, source); err != nil {
				return err
			}
		}
		if err := h.record(ctx, action, id, map[string]any{"source_event_id": source.String()}); err != nil {
			return err
		}
		ch.Status = to
		return h.afterCommit(ctx, tenant, *ch, MembershipStatusChanged, to, a, b)
	})
	return changed, err
}

// EnsureImplicit creates the World or Area channel id of the sender when it is its own (lazy creation at the first send
// or join, F120-AC-12, AC-13) and returns its facts; nil when id is no implicit channel of the sender.
func (h Persistent) EnsureImplicit(ctx context.Context, tenant, user, id uuid.UUID) (*read.ChannelFacts, error) {
	if h.Implicit == nil {
		return nil, nil
	}
	var ch channel.Channel
	now := h.now()
	if world, school, ok, err := h.Implicit.World(ctx, tenant, user); err != nil {
		return nil, err
	} else if ok && world == id {
		c, err := channel.NewImplicit(id, tenant, school, channel.World, channel.WorldScope(tenant, school), now)
		if err != nil {
			return nil, err
		}
		ch = c
	} else if area, room, ok, err := h.Implicit.Area(ctx, tenant, user); err != nil {
		return nil, err
	} else if ok && area == id {
		c, err := channel.NewImplicit(id, tenant, nil, channel.Area, channel.AreaScope(room), now)
		if err != nil {
			return nil, err
		}
		ch = c
	} else {
		return nil, nil
	}
	err := h.UoW.InTenantTx(ctx, mustScope(tenant), func(ctx context.Context) error {
		created, err := h.Repo.CreateChannel(ctx, ch)
		if err != nil || !created {
			return err
		}
		return h.record(ctx, AuditChannelOpened, ch.ID, map[string]any{"channel_type": string(ch.Type)})
	})
	if err != nil {
		return nil, err
	}
	_ = h.Facts.InvalidateChannel(ctx, tenant, id)
	return h.Facts.Channel(ctx, tenant, id)
}
