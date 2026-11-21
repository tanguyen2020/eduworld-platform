package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// day is the window of the per-day limits (party_invite_rate 50/day, party_invite_repeat_rules 3 rejections/day):
// a sliding 24 h window [Proposed F100].
const day = 24 * time.Hour

// InviteToParty implements inviteToParty (F100-AC-07…10): invitation_policy, blocks, school/tenant/class/grade band
// rules, sanctions, the kick block, then the repeat rules and party_invite_rate; a pending invitation expiring after
// party_invite_ttl, party.invitation_sent and party.invitation.received to the invitee.
func (h Parties) InviteToParty(ctx context.Context, partyID, invitee uuid.UUID) (res view.Invitation, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.InviteToParty")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return res, err
	}
	if invitee == me {
		return res, h.finish(ctx, CmdInvite, party.ValidationError{Field: "invitee_id", Reason: problem.SubInvalid, Message: "must be another student"})
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		now := h.now()
		fx := &effects{}
		inviter, err := h.caller(ctx, tenant, me, now)
		if err != nil {
			return err
		}
		sc, err := h.scope(ctx, tenant)
		if err != nil {
			return err
		}
		if err := h.Repo.LockStudents(ctx, me); err != nil { // one inviter at a time: the rate counts stay exact
			return err
		}
		p, err := h.load(ctx, tenant, partyID)
		if err != nil {
			return err
		}
		if err := p.RequireMember(me); err != nil {
			return err
		}
		target, err := h.subject(ctx, tenant, invitee, now)
		if err != nil {
			return err
		}
		if _, ok := p.Member(invitee); ok {
			return problem.New(problem.Conflict, "already a member")
		}
		check := party.InviteCheck{Party: p, Inviter: inviter, Invitee: target, Scope: sc}
		if target.Active {
			if check.Friends, err = h.Directory.Friends(ctx, tenant, me, invitee); err != nil {
				return err
			}
			if check.Blocked, err = h.Directory.Blocked(ctx, tenant, invitee, p.MemberIDs()); err != nil {
				return err
			}
			if check.Kicked, err = h.Repo.KickedSince(ctx, tenant, p.ID, invitee, now.Add(-h.Config.KickReinviteBlock)); err != nil {
				return err
			}
		}
		if check.Rules, err = h.rules(ctx, tenant, p.GradeBand, p.SchoolID, inviter, target); err != nil {
			return err
		}
		if err := check.Evaluate(); err != nil {
			return h.policyDenied(err, p, invitee, CmdInvite)
		}
		// A pending invitation past its expiry no longer counts (the job may not have run yet).
		if err := h.endPending(ctx, fx, tenant, write.PendingFilter{PartyID: &p.ID, InviteeID: &invitee, Expired: &now},
			party.InvitationExpired, party.EndTTL, nil, now); err != nil {
			return err
		}
		if err := h.inviteLimits(ctx, p, me, invitee, now); err != nil {
			return err
		}
		inv := &party.Invitation{ID: newID(), TenantID: tenant, SchoolID: p.SchoolID, PartyID: p.ID, InviterID: me, InviteeID: invitee,
			Status: party.InvitationPending, CreatedAt: now, ExpiresAt: now.Add(h.Config.InviteTTL)}
		if err := h.Repo.CreateInvitation(ctx, inv); err != nil {
			return err
		}
		if err := h.invitationEvent(ctx, inv, nil, now); err != nil {
			return err
		}
		if err := h.overflow(ctx, fx, tenant, invitee, now); err != nil {
			return err
		}
		cards, err := h.Directory.Cards(ctx, tenant, []uuid.UUID{me})
		if err != nil {
			return err
		}
		res = view.Invitation{Invitation: *inv, PartyName: p.Name, Inviter: cardOf(cards, me)}
		if err := idempotency.SetResult(ctx, res.Body()); err != nil {
			return err
		}
		fx.send([]uuid.UUID{invitee}, rtInvitationReceived, view.InvitationReceived(*inv, *p, res.Inviter))
		if h.Metrics != nil {
			h.Metrics.Invitation(party.InvitationPending)
		}
		return h.after(ctx, fx)
	})
	return res, h.finish(ctx, CmdInvite, err)
}

// inviteLimits applies the duplicate rule (409), party_invite_repeat_rules and party_invite_rate (429 with audit —
// F100-AC-10).
func (h Parties) inviteLimits(ctx context.Context, p *party.Party, me, invitee uuid.UUID, now time.Time) error {
	pending, err := h.Repo.Pending(ctx, p.TenantID, write.PendingFilter{PartyID: &p.ID, InviteeID: &invitee})
	if err != nil {
		return err
	}
	if len(pending) > 0 {
		return write.ErrPendingExists
	}
	limited := func(reason string, after time.Duration) error {
		return denied{err: RetryAfter{Err: problem.New(problem.PartyInviteRateLimited, "").Retry(), After: after},
			action: AuditInviteRateLimited, party: p, target: invitee, reason: reason}
	}
	rejections, err := h.Repo.RejectionsSince(ctx, p.TenantID, me, invitee, now.Add(-day))
	if err != nil {
		return err
	}
	if n := h.Config.RejectsPerDay; n > 0 && len(rejections) >= n {
		return limited("repeat_rejections", retryAt(rejections[len(rejections)-n], day, now))
	}
	if len(rejections) > 0 && now.Sub(rejections[len(rejections)-1]) < h.Config.RepeatWait {
		return limited("repeat_wait", retryAt(rejections[len(rejections)-1], h.Config.RepeatWait, now))
	}
	sent, err := h.Repo.InvitesSince(ctx, p.TenantID, me, now.Add(-day))
	if err != nil {
		return err
	}
	if n := h.Config.InvitePerDay; n > 0 && len(sent) >= n {
		return limited("party_invite_rate_day", retryAt(sent[len(sent)-n], day, now))
	}
	var lastMinute []time.Time
	for _, t := range sent {
		if now.Sub(t) < time.Minute {
			lastMinute = append(lastMinute, t)
		}
	}
	if n := h.Config.InvitePerMinute; n > 0 && len(lastMinute) >= n {
		return limited("party_invite_rate_minute", retryAt(lastMinute[len(lastMinute)-n], time.Minute, now))
	}
	return nil
}

// overflow expires the oldest pending invitations of invitee above party_pending_invites_max (F100-AC-14).
func (h Parties) overflow(ctx context.Context, fx *effects, tenant, invitee uuid.UUID, now time.Time) error {
	if h.Config.PendingMax <= 0 {
		return nil
	}
	pending, err := h.Repo.Pending(ctx, tenant, write.PendingFilter{InviteeID: &invitee})
	if err != nil {
		return err
	}
	for i := 0; i < len(pending)-h.Config.PendingMax; i++ {
		inv := pending[i]
		inv.Expire(party.EndPendingOverflow, now)
		if err := h.Repo.SaveInvitation(ctx, &inv); err != nil {
			return err
		}
		if err := h.invitationEvent(ctx, &inv, nil, now); err != nil {
			return err
		}
		fx.send([]uuid.UUID{inv.InviteeID, inv.InviterID}, rtInvitationUpdated, view.InvitationUpdated(inv))
	}
	return nil
}

func cardOf(cards map[uuid.UUID]portCard, id uuid.UUID) portCard {
	if c, ok := cards[id]; ok {
		return c
	}
	return portCard{UserID: id, Level: 1}
}

// invitation returns an invitation of the tenant; 404 when unknown.
func (h Parties) invitation(ctx context.Context, tenant, id uuid.UUID, lock bool) (*party.Invitation, error) {
	inv, err := h.Repo.Invitation(ctx, tenant, id, lock)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, problem.New(problem.NotFound, "")
	}
	return inv, nil
}

// AcceptPartyInvitation implements acceptPartyInvitation (F100-AC-11, AC-12): every rule is checked again, the member
// is added under the party lock (PARTY_FULL beyond max_members), the invitation accepted; a repeated accept of the
// same invitation returns the party without a second membership.
func (h Parties) AcceptPartyInvitation(ctx context.Context, id uuid.UUID) (res view.Party, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.AcceptPartyInvitation")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return res, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		now := h.now()
		fx := &effects{}
		sub, err := h.caller(ctx, tenant, me, now)
		if err != nil {
			return err
		}
		sc, err := h.scope(ctx, tenant)
		if err != nil {
			return err
		}
		if err := h.Repo.LockStudents(ctx, me); err != nil {
			return err
		}
		inv, err := h.invitation(ctx, tenant, id, false)
		if err != nil {
			return err
		}
		active, err := h.Repo.ActivePartyOf(ctx, tenant, me)
		if err != nil {
			return err
		}
		if inv.InviteeID == me && inv.Status == party.InvitationAccepted && active == inv.PartyID { // repeated accept
			p, err := h.load(ctx, tenant, inv.PartyID)
			if err != nil {
				return err
			}
			res, err = h.view(ctx, p)
			if err != nil {
				return err
			}
			return idempotency.SetResult(ctx, res.Body())
		}
		if inv.InviteeID != me {
			return party.ErrNotInvitee
		}
		if active != uuid.Nil { // before the status: entering another party expired the invitation (edge case)
			return write.ErrAlreadyInParty
		}
		if err := inv.Respond(me, now); err != nil {
			return err
		}
		p, err := h.load(ctx, tenant, inv.PartyID)
		if err != nil {
			return err
		}
		if !p.Active() { // the disband expires its invitations; a leftover is expired
			return party.ErrExpired
		}
		if inv, err = h.invitation(ctx, tenant, id, true); err != nil { // re-read under the party lock
			return err
		}
		if err := inv.Respond(me, now); err != nil {
			return err
		}
		ref := inv.InviterID
		if _, ok := p.Member(ref); !ok {
			ref = p.LeaderID
		}
		refSub, err := h.subject(ctx, tenant, ref, now)
		if err != nil {
			return err
		}
		check := party.InviteCheck{Party: p, Inviter: refSub, Invitee: sub, Scope: sc}
		if check.Friends, err = h.Directory.Friends(ctx, tenant, ref, me); err != nil {
			return err
		}
		if check.Blocked, err = h.Directory.Blocked(ctx, tenant, me, p.MemberIDs()); err != nil {
			return err
		}
		if check.Kicked, err = h.Repo.KickedSince(ctx, tenant, p.ID, me, now.Add(-h.Config.KickReinviteBlock)); err != nil {
			return err
		}
		if check.Rules, err = h.rules(ctx, tenant, p.GradeBand, p.SchoolID, refSub, sub); err != nil {
			return err
		}
		if err := check.AcceptCheck(); err != nil {
			return h.policyDenied(err, p, me, CmdAccept)
		}
		m := newMember(sub, &inv.ID, now)
		if err := p.Add(m, now); err != nil {
			return err
		}
		inv.Accept(now)
		if err := h.Repo.SaveInvitation(ctx, inv); err != nil {
			return err
		}
		if err := h.invitationEvent(ctx, inv, nil, now); err != nil {
			return err
		}
		if err := h.joined(ctx, fx, p, m, "invitation", now); err != nil {
			return err
		}
		fx.send([]uuid.UUID{inv.InviterID}, rtInvitationUpdated, view.InvitationUpdated(*inv))
		if h.Metrics != nil {
			h.Metrics.Invitation(party.InvitationAccepted)
		}
		if res, err = h.view(ctx, p); err != nil {
			return err
		}
		if err := idempotency.SetResult(ctx, res.Body()); err != nil {
			return err
		}
		return h.after(ctx, fx)
	})
	return res, h.finish(ctx, CmdAccept, err)
}

// joined persists a new member (accept, join): membership, state_version, the end of the other invitations of the
// student, history, party.member_joined and party.member.joined to every member.
func (h Parties) joined(ctx context.Context, fx *effects, p *party.Party, m party.Member, via string, now time.Time) error {
	if err := h.Repo.AddMember(ctx, p, m); err != nil {
		return err
	}
	if err := h.Repo.Save(ctx, p); err != nil {
		return err
	}
	if err := h.endPending(ctx, fx, p.TenantID, write.PendingFilter{InviteeID: &m.StudentID}, party.InvitationExpired,
		party.EndJoinedOtherParty, nil, now); err != nil {
		return err
	}
	if err := h.history(ctx, p, histJoined, &m.StudentID, now, map[string]any{"student_id": m.StudentID.String(), "via": via}); err != nil {
		return err
	}
	payload := map[string]any{"party_id": p.ID, "tenant_id": p.TenantID, "student_id": m.StudentID, "role": m.Role, "via": via,
		"state_version": p.StateVersion, "joined_at": m.JoinedAt}
	if m.InvitationID != nil {
		payload["invitation_id"] = *m.InvitationID
	}
	if err := h.publish(ctx, "party.member_joined", p, now, payload); err != nil {
		return err
	}
	cards, err := h.Directory.Cards(ctx, p.TenantID, []uuid.UUID{m.StudentID})
	if err != nil {
		return err
	}
	online := h.online(ctx, []uuid.UUID{m.StudentID})
	fx.send(p.MemberIDs(), rtMemberJoined, map[string]any{"party_id": p.ID, "state_version": p.StateVersion,
		"member": view.MemberState(m, cardOf(cards, m.StudentID), online[m.StudentID])})
	fx.state(p)
	return nil
}

// RejectPartyInvitation implements rejectPartyInvitation (F100-AC-13): the invitee only; the inviter is told
// (party.invitation.updated) and the repeat rules start.
func (h Parties) RejectPartyInvitation(ctx context.Context, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.RejectPartyInvitation")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		now := h.now()
		fx := &effects{}
		inv, err := h.invitation(ctx, tenant, id, true)
		if err != nil {
			return err
		}
		if err := inv.Respond(me, now); err != nil {
			return err
		}
		inv.Reject(now)
		if err := h.Repo.SaveInvitation(ctx, inv); err != nil {
			return err
		}
		if err := h.invitationEvent(ctx, inv, nil, now); err != nil {
			return err
		}
		fx.send([]uuid.UUID{inv.InviterID}, rtInvitationUpdated, view.InvitationUpdated(*inv))
		if h.Metrics != nil {
			h.Metrics.Invitation(party.InvitationRejected)
		}
		return h.after(ctx, fx)
	})
	return h.finish(ctx, CmdReject, err)
}

// CancelPartyInvitation implements cancelPartyInvitation (F100-AC-13): the inviter or the current leader only.
func (h Parties) CancelPartyInvitation(ctx context.Context, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.CancelPartyInvitation")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		now := h.now()
		fx := &effects{}
		inv, err := h.invitation(ctx, tenant, id, false)
		if err != nil {
			return err
		}
		p, err := h.load(ctx, tenant, inv.PartyID)
		if err != nil {
			return err
		}
		if inv, err = h.invitation(ctx, tenant, id, true); err != nil {
			return err
		}
		if err := inv.CanCancel(me, p.LeaderID); err != nil {
			return err
		}
		inv.Cancel(party.EndCancelled, now)
		if err := h.Repo.SaveInvitation(ctx, inv); err != nil {
			return err
		}
		if err := h.invitationEvent(ctx, inv, &me, now); err != nil {
			return err
		}
		to := []uuid.UUID{inv.InviteeID}
		if inv.InviterID != me {
			to = append(to, inv.InviterID)
		}
		fx.send(to, rtInvitationUpdated, view.InvitationUpdated(*inv))
		if h.Metrics != nil {
			h.Metrics.Invitation(party.InvitationCancelled)
		}
		return h.after(ctx, fx)
	})
	return h.finish(ctx, CmdCancel, err)
}

// ExpireInvitations is one run of the job party-invitation-expiry (F100-AC-14): pending invitations past expires_at of
// every tenant become expired (ttl), each in the transaction of its tenant with its event and pushes.
func (h Parties) ExpireInvitations(ctx context.Context, expiry write.ExpiryRepository, batch int) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.ExpirePartyInvitations")
	defer func() { done(err) }()
	ps, err := authz.SystemPlatformScope("party-invitation-expiry")
	if err != nil {
		return 0, err
	}
	now := h.now()
	var refs []write.InvitationRef
	if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		var err error
		refs, err = expiry.ExpiredPending(ctx, now, batch)
		return err
	}); err != nil {
		return 0, err
	}
	for _, r := range refs {
		sc, err := authz.EventScope(r.TenantID)
		if err != nil {
			return n, err
		}
		err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
			fx := &effects{}
			inv, err := h.Repo.Invitation(ctx, r.TenantID, r.InvitationID, true)
			if err != nil || inv == nil || inv.Status != party.InvitationPending || inv.ExpiresAt.After(now) {
				return err
			}
			inv.Expire(party.EndTTL, now)
			if err := h.Repo.SaveInvitation(ctx, inv); err != nil {
				return err
			}
			if err := h.invitationEvent(ctx, inv, nil, now); err != nil {
				return err
			}
			fx.send([]uuid.UUID{inv.InviteeID, inv.InviterID}, rtInvitationUpdated, view.InvitationUpdated(*inv))
			n++
			return h.after(ctx, fx)
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			return n, err
		}
	}
	return n, nil
}
