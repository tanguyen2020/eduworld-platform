package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/safety"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// SanctionInput is SanctionCreate.
type SanctionInput struct {
	UserID          uuid.UUID
	CaseID          *uuid.UUID
	Type            string
	DurationMinutes *int
	Reason          string
	ChannelID       *uuid.UUID
}

// CreateSanction implements createSanction (F121-AC-13, MODERATION §7): a sanction of a student of the target tenant
// (X-Tenant-Id), optionally tied to a case of that tenant; default durations of §7; account_lock and
// restrict_multiplayer longer than 7 days need the team lead or a System Admin (403 otherwise, audited); sanction.applied
// and chat.sanction.applied (reason_code, until) to the student only; the pipeline cache is cleared.
func (h Safety) CreateSanction(ctx context.Context, in SanctionInput) (out read.SanctionDetail, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.CreateSanction")
	defer func() { done(err) }()
	p, tenant, err := moderatorWrite(ctx, authz.PermModerationSanctionWrite)
	if err != nil {
		return out, err
	}
	if n := len([]rune(in.Reason)); n < 3 || n > 1000 {
		return out, problem.Invalid("reason", problem.SubInvalid, "3 to 1000 characters")
	}
	now := h.now()
	var s write.SafetySanction
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		var c *read.CaseRow
		if in.CaseID != nil {
			var err error
			if c, err = h.Store.LockCase(ctx, tenant, *in.CaseID); err != nil {
				return err
			}
			if c == nil {
				return notFound()
			}
			if c.Status == safety.StatusResolved {
				return safety.ErrClosed
			}
		}
		var err error
		if s, err = h.newSanction(ctx, p, tenant, in.UserID, in.CaseID, in.Type, in.DurationMinutes, in.ChannelID, in.Reason, now); err != nil {
			return err
		}
		if err := h.later(ctx, s.UserID, &s); err != nil {
			return err
		}
		if c != nil {
			act := safety.ActionRestrict
			switch in.Type {
			case safety.SanctionWarning:
				act = safety.ActionWarn
			case safety.SanctionMute:
				act = safety.ActionMute
			}
			sid := s.ID
			if err := h.action(ctx, c, act, &p.UserID, "moderator", nil, &sid, nil, now); err != nil {
				return err
			}
		}
		return idempotency.SetResult(ctx, service.SanctionBody(detailOf(s)))
	})
	if err != nil {
		return out, caseProblem(ctx, p, err)
	}
	return detailOf(s), nil
}

func detailOf(s write.SafetySanction) read.SanctionDetail {
	return read.SanctionDetail{ID: s.ID, TenantID: s.TenantID, SchoolID: s.SchoolID, UserID: s.UserID, CaseID: s.CaseID, Type: s.Type,
		Scope: s.Scope, ChannelID: s.ChannelID, Reason: s.Reason, ReasonCode: s.ReasonCode, IssuedByKind: s.IssuedByKind, IssuedBy: s.IssuedBy,
		StartsAt: s.StartsAt, EndsAt: s.EndsAt, CreatedAt: s.StartsAt, Version: 1}
}

// RevokeSanction implements revokeSanction (F121-AC-15): ends an active sanction of the target tenant before its end
// (reason required, audit), sanction.lifted, the pipeline cache cleared at once (the next send reloads it). A sanction
// that needs the team lead is revoked by the team lead too; an ended sanction is 409.
func (h Safety) RevokeSanction(ctx context.Context, id uuid.UUID, reason string) (out read.SanctionDetail, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.RevokeSanction")
	defer func() { done(err) }()
	p, tenant, err := moderatorWrite(ctx, authz.PermModerationSanctionWrite)
	if err != nil {
		return out, err
	}
	if n := len([]rune(reason)); n < 1 || n > 1000 {
		return out, problem.Invalid("reason", problem.SubInvalid, "1 to 1000 characters")
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		s, err := h.Store.LockSanction(ctx, tenant, id)
		if err != nil {
			return err
		}
		if s == nil {
			return notFound()
		}
		if s.RevokedAt != nil || s.LiftedAt != nil || (s.EndsAt != nil && !now.Before(*s.EndsAt)) {
			return problem.New(problem.Conflict, "the sanction already ended")
		}
		d := time.Duration(0)
		if s.EndsAt != nil {
			d = s.EndsAt.Sub(s.StartsAt)
		}
		if s.IssuedByKind == "moderator" && safety.RequiresLead(s.Type, d, s.EndsAt != nil) && !isLead(p) {
			return safety.ErrNeedsLead
		}
		if err := h.liftSanction(ctx, *s, &p.UserID, reason, liftRevoked, now); err != nil {
			return err
		}
		if err := h.later(ctx, s.UserID, nil); err != nil {
			return err
		}
		out = *s
		t, by, r := now, p.UserID, reason
		out.RevokedAt, out.RevokedBy, out.RevokeReason, out.LiftedAt = &t, &by, &r, &t
		return idempotency.SetResult(ctx, service.SanctionBody(out))
	})
	if err != nil {
		return out, caseProblem(ctx, p, err)
	}
	return out, nil
}

// ExpireSanctions is one run of the job sanction-expirer (F121-AC-15): every sanction past ends_at without
// sanction.lifted gets it (lift_reason expired) and its cache is cleared; holds of resolved cases left active (crash
// between the two steps) are released. The work list is read under tenancy.Bypass(platform_job); each item is written
// in the scope of its tenant.
func (h Safety) ExpireSanctions(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.ExpireSanctions")
	defer func() { done(err) }()
	if ok, err := h.lease(ctx, "sanction-expirer"); err != nil || !ok {
		return 0, err
	}
	now := h.now()
	var expired, stale []read.SanctionDetail
	if err := h.platformRead(ctx, func(ctx context.Context) error {
		var e error
		if expired, e = h.Store.ExpiredSanctions(ctx, now, 500); e != nil {
			return e
		}
		stale, e = h.Store.StaleHolds(ctx, 500)
		return e
	}); err != nil {
		return 0, err
	}
	for _, s := range append(expired, stale...) {
		sc, err := authz.EventScope(s.TenantID)
		if err != nil {
			continue
		}
		done := false
		err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
			cur, err := h.Store.LockSanction(ctx, s.TenantID, s.ID)
			if err != nil || cur == nil || cur.RevokedAt != nil || cur.LiftedAt != nil {
				return err
			}
			reason := liftExpired
			if cur.EndsAt == nil || now.Before(*cur.EndsAt) {
				reason = liftCaseResolved
			}
			done = true
			if err := h.liftSanction(ctx, *cur, nil, reasonResolvedReleaseHold, reason, now); err != nil {
				return err
			}
			return h.later(ctx, cur.UserID, nil)
		})
		if err != nil {
			return n, err
		}
		if done {
			n++
		}
	}
	return n, nil
}

// platformRead runs fn under tenancy.Bypass(platform_job) (audited, counted): the cross-tenant work lists of the jobs.
func (h Safety) platformRead(ctx context.Context, fn func(ctx context.Context) error) error {
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, "chat"), tenancy.ReasonPlatformJob)
	if err != nil {
		return err
	}
	return h.UoW.Do(bctx, fn)
}

// lease takes the job lease of one run (one replica per cycle); without a lock (tests) every run proceeds.
func (h Safety) lease(ctx context.Context, job string) (bool, error) {
	if h.Lock == nil {
		return true, nil
	}
	ok, err := h.Lock.Acquire(ctx, job, SafetyTick-5*time.Second)
	if err != nil {
		return false, errors.Join(errors.New("chat: job lease"), err)
	}
	return ok, nil
}
