package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Reasons of party.member.left (AsyncAPI): the realtime reason of a removal by policy depends on its cause.
const (
	RealtimeLeft              = "left"
	RealtimeKicked            = "kicked"
	RealtimeTransferredSchool = "transferred_school"
	RealtimeAccountDisabled   = "account_disabled"
)

// JoinParty implements joinParty (F100-AC-16): class (same class as the leader) or friends (friend of the leader)
// parties within the policies; a private party needs an accepted invitation that has not expired.
func (h Parties) JoinParty(ctx context.Context, partyID uuid.UUID) (res view.Party, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.JoinParty")
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
		p, err := h.load(ctx, tenant, partyID)
		if err != nil {
			return err
		}
		active, err := h.Repo.ActivePartyOf(ctx, tenant, me)
		if err != nil {
			return err
		}
		if active != uuid.Nil && active != p.ID {
			return write.ErrAlreadyInParty
		}
		if _, ok := p.Member(me); !ok {
			leader, err := h.subject(ctx, tenant, p.LeaderID, now)
			if err != nil {
				return err
			}
			check := party.JoinCheck{Party: p, Leader: leader, Joiner: sub, Scope: sc}
			if check.FriendOfLeader, err = h.Directory.Friends(ctx, tenant, p.LeaderID, me); err != nil {
				return err
			}
			if check.Blocked, err = h.Directory.Blocked(ctx, tenant, me, p.MemberIDs()); err != nil {
				return err
			}
			if check.Kicked, err = h.Repo.KickedSince(ctx, tenant, p.ID, me, now.Add(-h.Config.KickReinviteBlock)); err != nil {
				return err
			}
			if check.Invited, err = h.Repo.AcceptedInvitation(ctx, tenant, p.ID, me, now); err != nil {
				return err
			}
			if check.Rules, err = h.rules(ctx, tenant, p.GradeBand, p.SchoolID, leader, sub); err != nil {
				return err
			}
			if err := check.Evaluate(); err != nil {
				return h.policyDenied(err, p, me, CmdJoin)
			}
			m := newMember(sub, nil, now)
			if err := p.Add(m, now); err != nil {
				return err
			}
			if err := h.joined(ctx, fx, p, m, "join", now); err != nil {
				return err
			}
		}
		if res, err = h.view(ctx, p); err != nil {
			return err
		}
		if err := idempotency.SetResult(ctx, res.Body()); err != nil {
			return err
		}
		return h.after(ctx, fx)
	})
	return res, h.finish(ctx, CmdJoin, err)
}

// removal is a member leaving (leave, kick, policy) with its events.
type removal struct {
	student   uuid.UUID
	reason    string     // party_members.left_reason
	rtReason  string     // party.member.left reason
	actor     *uuid.UUID // nil: the system
	inviteEnd string     // reason ending the invitations the student sent in this party
}

// remove ends a membership in the transaction of ctx: the leader passes to the online member who joined first, the
// invitations sent by the leaving student end, the last member out disbands the party (F100-AC-17, AC-18, AC-25).
func (h Parties) remove(ctx context.Context, fx *effects, p *party.Party, r removal, now time.Time) error {
	online := h.online(ctx, p.MemberIDs())
	oldLeader := p.LeaderID
	out, err := p.Remove(r.student, online, now)
	if err != nil {
		return err
	}
	if err := h.Repo.EndMember(ctx, p.TenantID, p.ID, r.student, r.reason, now); err != nil {
		return err
	}
	if out.NewLeader != uuid.Nil {
		if err := h.Repo.SetRoles(ctx, p); err != nil {
			return err
		}
	}
	if err := h.endPending(ctx, fx, p.TenantID, write.PendingFilter{PartyID: &p.ID, InviterID: &r.student}, party.InvitationCancelled,
		r.inviteEnd, nil, now); err != nil {
		return err
	}
	if err := h.Repo.Save(ctx, p); err != nil {
		return err
	}
	// party.member_kicked replaces party.member_left for a kick (EVENT_DESIGN §4: one event per change).
	if r.reason == party.LeftKicked {
		if err := h.history(ctx, p, histKicked, r.actor, now, map[string]any{"student_id": r.student.String()}); err != nil {
			return err
		}
		if err := h.publish(ctx, "party.member_kicked", p, now, map[string]any{"party_id": p.ID, "tenant_id": p.TenantID,
			"student_id": r.student, "kicked_by": *r.actor, "state_version": p.StateVersion}); err != nil {
			return err
		}
	} else {
		if err := h.history(ctx, p, histLeft, r.actor, now, map[string]any{"student_id": r.student.String(), "reason": r.reason}); err != nil {
			return err
		}
		if err := h.publish(ctx, "party.member_left", p, now, map[string]any{"party_id": p.ID, "tenant_id": p.TenantID,
			"student_id": r.student, "reason": r.reason, "state_version": p.StateVersion}); err != nil {
			return err
		}
	}
	fx.send(append(p.MemberIDs(), r.student), rtMemberLeft, map[string]any{"party_id": p.ID, "state_version": p.StateVersion,
		"user_id": r.student, "reason": r.rtReason})
	if out.NewLeader != uuid.Nil {
		if err := h.transferred(ctx, fx, p, oldLeader, out.NewLeader, party.TransferLeaderLeft, now); err != nil {
			return err
		}
	}
	if out.Disbanded {
		return h.disbanded(ctx, fx, p, party.DisbandAllLeft, nil, []uuid.UUID{r.student}, now)
	}
	fx.state(p, r.student)
	return nil
}

// transferred records a leader change: history, party.leader_transferred, audit, party.leader.transferred.
func (h Parties) transferred(ctx context.Context, fx *effects, p *party.Party, from, to uuid.UUID, reason string, now time.Time) error {
	if err := h.history(ctx, p, histTransferred, &from, now, map[string]any{"from": from.String(), "to": to.String(), "reason": reason}); err != nil {
		return err
	}
	if err := h.publish(ctx, "party.leader_transferred", p, now, map[string]any{"party_id": p.ID, "tenant_id": p.TenantID,
		"from": from, "to": to, "reason": reason, "state_version": p.StateVersion}); err != nil {
		return err
	}
	if err := h.record(ctx, AuditLeaderTransferred, p, to, reason, map[string]any{"from": from.String()}); err != nil {
		return err
	}
	fx.send(p.MemberIDs(), rtLeaderTransferred, map[string]any{"party_id": p.ID, "from_user_id": from, "to_user_id": to,
		"reason": reason})
	return nil
}

// disbanded records a disband: pending invitations expire, history, party.disbanded, audit, party.party.disbanded to
// every former member (left: those who already left in this command).
func (h Parties) disbanded(ctx context.Context, fx *effects, p *party.Party, reason string, actor *uuid.UUID, left []uuid.UUID,
	now time.Time) error {
	if err := h.Repo.Save(ctx, p); err != nil {
		return err
	}
	if err := h.endPending(ctx, fx, p.TenantID, write.PendingFilter{PartyID: &p.ID}, party.InvitationExpired, party.EndPartyDisbanded,
		nil, now); err != nil {
		return err
	}
	if err := h.history(ctx, p, histDisbanded, actor, now, map[string]any{"reason": reason}); err != nil {
		return err
	}
	payload := map[string]any{"party_id": p.ID, "tenant_id": p.TenantID, "reason": reason, "disbanded_at": now,
		"state_version": p.StateVersion}
	target := uuid.Nil
	if actor != nil {
		payload["actor_id"] = *actor
		target = *actor
	}
	if err := h.publish(ctx, "party.disbanded", p, now, payload); err != nil {
		return err
	}
	if err := h.record(ctx, AuditPartyDisbanded, p, target, reason, nil); err != nil {
		return err
	}
	fx.send(left, rtDisbanded, map[string]any{"party_id": p.ID, "reason": reason})
	fx.state(p, left...)
	return nil
}

// LeaveParty implements leaveParty (F100-AC-17).
func (h Parties) LeaveParty(ctx context.Context, partyID uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.LeaveParty")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		now := h.now()
		fx := &effects{}
		p, err := h.load(ctx, tenant, partyID)
		if err != nil {
			return err
		}
		if err := p.RequireMember(me); err != nil {
			return err
		}
		if err := h.remove(ctx, fx, p, removal{student: me, reason: party.LeftLeft, rtReason: RealtimeLeft, actor: &me,
			inviteEnd: party.EndInviterLeft}, now); err != nil {
			return err
		}
		return h.after(ctx, fx)
	})
	return h.finish(ctx, CmdLeave, err)
}

// KickPartyMember implements kickPartyMember (F100-AC-18): leader only; the invitations sent by the kicked student end,
// the student may not be invited into this party again for party_kick_reinvite_block; party_kick_rate per leader.
func (h Parties) KickPartyMember(ctx context.Context, partyID, target uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.KickPartyMember")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		now := h.now()
		fx := &effects{}
		p, err := h.load(ctx, tenant, partyID)
		if err != nil {
			return err
		}
		if err := p.RequireLeader(me); err != nil {
			return err
		}
		if target == me {
			return party.ValidationError{Field: "student_id", Reason: problem.SubInvalid, Message: "the leader cannot kick itself"}
		}
		if _, ok := p.Member(target); !ok {
			return problem.New(problem.NotFound, "")
		}
		if h.Config.KickPerHour > 0 {
			times, err := h.Repo.HistorySince(ctx, tenant, me, []string{histKicked}, now.Add(-time.Hour))
			if err != nil {
				return err
			}
			if len(times) >= h.Config.KickPerHour {
				return denied{err: RetryAfter{Err: problem.New(problem.RateLimited, "").Retry(),
					After: retryAt(times[len(times)-h.Config.KickPerHour], time.Hour, now)}, action: AuditRateLimited, party: p, target: target,
					reason: "party_kick_rate"}
			}
		}
		if err := h.remove(ctx, fx, p, removal{student: target, reason: party.LeftKicked, rtReason: RealtimeKicked, actor: &me,
			inviteEnd: party.EndInviterKicked}, now); err != nil {
			return err
		}
		if err := h.record(ctx, AuditMemberKicked, p, target, "", nil); err != nil {
			return err
		}
		return h.after(ctx, fx)
	})
	return h.finish(ctx, CmdKick, err)
}

// TransferPartyLeader implements transferPartyLeader (F100-AC-19): leader only, to an online member, in one
// transaction (exactly one active leader).
func (h Parties) TransferPartyLeader(ctx context.Context, partyID, to uuid.UUID) (res view.Party, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.TransferPartyLeader")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return res, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		now := h.now()
		fx := &effects{}
		p, err := h.load(ctx, tenant, partyID)
		if err != nil {
			return err
		}
		if err := p.RequireLeader(me); err != nil {
			return err
		}
		if to == me {
			return party.ValidationError{Field: "new_leader_id", Reason: problem.SubInvalid, Message: "already the leader"}
		}
		if _, ok := p.Member(to); !ok {
			return party.ValidationError{Field: "new_leader_id", Reason: problem.SubInvalid, Message: "not a member of the party"}
		}
		if !h.online(ctx, []uuid.UUID{to})[to] {
			return problem.New(problem.PartyMemberNotOnline, "")
		}
		if err := p.Transfer(to, now); err != nil {
			return err
		}
		if err := h.Repo.SetRoles(ctx, p); err != nil {
			return err
		}
		if err := h.Repo.Save(ctx, p); err != nil {
			return err
		}
		if err := h.transferred(ctx, fx, p, me, to, party.TransferManual, now); err != nil {
			return err
		}
		if res, err = h.view(ctx, p); err != nil {
			return err
		}
		if err := idempotency.SetResult(ctx, res.Body()); err != nil {
			return err
		}
		fx.state(p)
		return h.after(ctx, fx)
	})
	return res, h.finish(ctx, CmdTransfer, err)
}

// DisbandParty implements disbandParty (F100-AC-20): leader only; every member leaves (disbanded), pending
// invitations expire; a second call by the same leader answers 204 without a second event.
func (h Parties) DisbandParty(ctx context.Context, partyID uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.DisbandParty")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		now := h.now()
		fx := &effects{}
		p, err := h.load(ctx, tenant, partyID)
		if err != nil {
			return err
		}
		if !p.Active() && p.LeaderID == me {
			return nil // already disbanded by this leader: idempotent
		}
		if err := p.RequireLeader(me); err != nil {
			return err
		}
		members, ok := p.Disband(now)
		if !ok {
			return nil
		}
		left := make([]uuid.UUID, 0, len(members))
		for _, m := range members {
			if err := h.Repo.EndMember(ctx, tenant, p.ID, m.StudentID, party.LeftDisbanded, now); err != nil {
				return err
			}
			left = append(left, m.StudentID)
		}
		if err := h.disbanded(ctx, fx, p, party.DisbandLeader, &me, left, now); err != nil {
			return err
		}
		return h.after(ctx, fx)
	})
	return h.finish(ctx, CmdDisband, err)
}

// RemoveByPolicy takes student out of its party and ends its pending invitations, in the transaction of ctx (the
// consumer's): sanction restrict_multiplayer, school change, account disabled, cleanup, anonymization (F100-AC-25,
// edge cases). rtReason is the party.member.left reason. Returns whether something changed.
func (h Parties) RemoveByPolicy(ctx context.Context, tenant, student uuid.UUID, rtReason, why string) (bool, error) {
	now := h.now()
	fx := &effects{}
	changed := false
	parties, err := h.Repo.MembershipsOf(ctx, tenant, student)
	if err != nil {
		return false, err
	}
	for _, id := range parties {
		p, err := h.Repo.Load(ctx, tenant, id, true)
		if err != nil {
			return false, err
		}
		if p == nil || !p.Active() {
			continue
		}
		if err := h.remove(ctx, fx, p, removal{student: student, reason: party.LeftPolicy, rtReason: rtReason,
			inviteEnd: party.EndPolicy}, now); err != nil {
			return false, err
		}
		if err := h.record(ctx, AuditMemberRemovedPolicy, p, student, why, nil); err != nil {
			return false, err
		}
		changed = true
	}
	before := len(fx.pushes)
	if err := h.endPending(ctx, fx, tenant, write.PendingFilter{Student: &student}, party.InvitationCancelled, party.EndPolicy, nil,
		now); err != nil {
		return false, err
	}
	changed = changed || len(fx.pushes) > before
	return changed, h.after(ctx, fx)
}

// CancelBetween cancels the pending invitations between two students who blocked each other (F100-AC-25).
func (h Parties) CancelBetween(ctx context.Context, tenant, a, b uuid.UUID) (bool, error) {
	fx := &effects{}
	if err := h.endPending(ctx, fx, tenant, write.PendingFilter{Between: &[2]uuid.UUID{a, b}}, party.InvitationCancelled, party.EndBlocked,
		nil, h.now()); err != nil {
		return false, err
	}
	return len(fx.pushes) > 0, h.after(ctx, fx)
}
