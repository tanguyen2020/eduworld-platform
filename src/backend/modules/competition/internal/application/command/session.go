package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// activityNamespace derives the party_activity_id of an idempotent start: a retry with the same Idempotency-Key after
// a failed commit asks module game for the same session (research R2).
var activityNamespace = uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-f10100000001")

func activityID(ctx context.Context, partyID uuid.UUID) uuid.UUID {
	if p, ok := idempotency.PendingFrom(ctx); ok {
		return uuid.NewSHA1(activityNamespace, []byte(partyID.String()+"/"+p.Key.Key.String()))
	}
	return ids.New()
}

// StartInput is startPartySession / party.session.start.
type StartInput struct {
	Expected    int64
	GameSession *uuid.UUID // game session of the caller (realtime command header)
}

// StartPartySession implements startPartySession and party.session.start (PARTY_SYSTEM §6.1, research R2,
// F101-AC-07…AC-10): checks of the leader, version, Ready and eligibility, the session of module game (quest/boss)
// outside the transaction and idempotent on party_activity_id, then one transaction with the activity, its room, the
// session memberships, the party in_activity and party.activity_started; party.session.started follows the commit.
func (h Parties) StartPartySession(ctx context.Context, partyID uuid.UUID, in StartInput) (res view.SessionStart, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.StartPartySession")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return res, err
	}
	activityType := "unknown"
	err = func() error {
		if h.Activities == nil {
			return problem.New(problem.Unavailable, "").Retry()
		}
		p, err := h.peek(ctx, tenant, me, partyID)
		if err != nil {
			return err
		}
		online := h.online(ctx, p.MemberIDs())
		online[me] = true
		ready := h.lobby(ctx, p.ID).Ready()
		parts, err := p.CanStart(me, in.Expected, online, ready, 1, party.MaxMembers)
		if err != nil {
			if errors.Is(err, party.ErrVersion) {
				return versionMismatch(p)
			}
			return err
		}
		sel := *p.Selected
		activityType = sel.Type
		elig, err := h.eligibility(ctx, p, sel, parts.Members)
		if err != nil {
			return err
		}
		if bad := elig.Ineligible(); len(bad) > 0 {
			reasons := make([]string, 0, len(bad))
			for _, c := range bad {
				reasons = append(reasons, c.Reason)
			}
			return denied{err: problem.New(problem.PartyActivityNotEligible, "").With("ineligible_members", view.Ineligible(bad, "student_id")),
				action: AuditSessionStartDenied, party: p, target: me, reason: "not_eligible",
				meta: map[string]any{"activity_type": sel.Type, "reason_codes": reasons}}
		}
		if n := len(parts.Members); n < elig.MinMembers || n > elig.MaxMembers {
			return party.ErrNotReady
		}
		a := party.Activity{ID: activityID(ctx, p.ID), TenantID: p.TenantID, SchoolID: p.SchoolID, PartyID: p.ID, Type: sel.Type,
			Ref: sel.Ref, GameMode: sel.GameMode, RoomType: party.RoomPrivateInstance, Status: party.ActivityRunning,
			MinMembers: elig.MinMembers, MaxMembers: elig.MaxMembers, AllowContinue: elig.AllowContinue}
		area := elig.AreaID
		if area == "" {
			area = sel.Ref
		}
		sessions := map[uuid.UUID]uuid.UUID{}
		if in.GameSession != nil {
			sessions[me] = *in.GameSession
		}
		scene := ""
		if party.FromGame(sel.Type) {
			gctx, err := withScope(ctx, p.TenantID)
			if err != nil {
				return err
			}
			started, err := h.Game.Start(gctx, port.GameStart{ActivityID: a.ID, PartyID: p.ID, LeaderID: me, Type: sel.Type, Ref: sel.Ref,
				GameMode: sel.GameMode, Members: parts.Members, Sessions: sessions})
			if err != nil {
				return err
			}
			ext := started.ExternalRef
			a.ExternalRef = &ext
			scene = started.SceneRef
			if started.RoomType == party.RoomBoss {
				a.RoomType, a.RoomID = party.RoomBoss, ext
			}
			if started.AreaID != "" {
				area = started.AreaID
			}
		}
		if a.RoomID == uuid.Nil {
			a.RoomID = uuid.NewSHA1(activityNamespace, []byte("room/"+a.ID.String()))
		}
		return h.UoW.Do(ctx, func(ctx context.Context) error {
			now := h.now()
			fx := &effects{}
			p, err := h.load(ctx, tenant, partyID)
			if err != nil {
				return err
			}
			if _, err := p.CanStart(me, in.Expected, online, ready, 1, party.MaxMembers); err != nil {
				if errors.Is(err, party.ErrVersion) {
					return versionMismatch(p) // the party changed meanwhile: the session of module game closes unused
				}
				return err
			}
			a.StartedAt = now
			capacity := a.MaxMembers
			if a.RoomType == party.RoomPrivateInstance {
				capacity = max(h.LobbyConfig.PrivateCapacity, a.MaxMembers)
			}
			if err := h.Activities.CreateRoom(ctx, write.SessionRoom{RoomID: a.RoomID, TenantID: p.TenantID, SchoolID: p.SchoolID,
				GradeBand: p.GradeBand, RoomType: a.RoomType, AreaID: area, Capacity: capacity, CreatedAt: now}); err != nil {
				return err
			}
			if err := h.Activities.Create(ctx, &a); err != nil {
				return err
			}
			if err := h.Activities.OpenMemberships(ctx, &a, parts.Members, sessions, now); err != nil {
				return err
			}
			p.StartActivity(now)
			if err := h.Repo.Save(ctx, p); err != nil {
				return err
			}
			payload := map[string]any{"party_activity_id": a.ID, "party_id": p.ID, "tenant_id": p.TenantID, "activity_type": a.Type,
				"activity_ref": a.Ref, "room_id": a.RoomID, "room_type": a.RoomType, "member_ids": parts.Members, "game_session_ref": a.ID,
				"started_at": now, "state_version": p.StateVersion}
			if a.GameMode != "" {
				payload["game_mode"] = a.GameMode
			}
			if a.ExternalRef != nil {
				payload["external_ref"] = *a.ExternalRef
			}
			if err := h.publish(ctx, "party.activity_started", p, now, payload); err != nil {
				return err
			}
			res = view.SessionStart{Activity: a, Members: parts.Members, Offline: parts.Offline, SceneRef: scene}
			if err := idempotency.SetResult(ctx, res.Body()); err != nil {
				return err
			}
			fx.send(parts.Members, rtSessionStarted, res.Started())
			fx.resetReady = append(fx.resetReady, p.ID)
			fx.state(p)
			return h.after(ctx, fx)
		})
	}()
	if h.LobbyMetrics != nil {
		result := "ok"
		var pe *problem.Error
		if err != nil {
			result = "error"
			if errors.As(mapError(err), &pe) {
				result = string(pe.Code)
			}
		}
		h.LobbyMetrics.SessionStart(activityType, result)
	}
	return res, h.finish(ctx, CmdStart, err)
}

// leaveReasons is the session membership leave reason of the remaining participants when an activity ends.
var leaveReasons = map[string]string{party.EndCompleted: party.SessionRoomClosed, party.EndTimeout: party.SessionTimeout,
	party.EndAllLeft: party.SessionLeft, party.EndDisbanded: party.SessionRoomClosed, party.EndServerFailure: party.SessionRoomClosed}

// endActivity ends the running activity a of p in the transaction of ctx (F101-AC-12, AC-13): memberships and room
// closed, the party back to its lobby (unless disbanded) with Ready reset, party.activity_ended (and
// party.activity_completed) and party.session.ended to the members. Idempotent: an ended activity changes nothing.
func (h Parties) endActivity(ctx context.Context, fx *effects, p *party.Party, a *party.Activity, reason string, summary map[string]any,
	now time.Time) error {
	if err := a.End(reason, summary, now); err != nil {
		if errors.Is(err, party.ErrActivityEnded) {
			return nil
		}
		return err
	}
	participants, err := h.Activities.Participants(ctx, a.TenantID, a.ID)
	if err != nil {
		return err
	}
	if err := h.Activities.CloseMemberships(ctx, a.TenantID, a.ID, leaveReasons[reason], now); err != nil {
		return err
	}
	if err := h.Activities.End(ctx, a); err != nil {
		return err
	}
	if err := h.Activities.CloseRoom(ctx, a.TenantID, a.RoomID, now); err != nil {
		return err
	}
	if p.Status == party.StatusInActivity {
		p.BackToLobby(now)
		if err := h.Repo.Save(ctx, p); err != nil {
			return err
		}
	}
	base := map[string]any{"party_activity_id": a.ID, "party_id": a.PartyID, "tenant_id": a.TenantID, "activity_type": a.Type,
		"activity_ref": a.Ref}
	ended := map[string]any{"end_reason": reason, "outcome": party.Outcome(reason), "ended_at": now, "state_version": p.StateVersion}
	for k, v := range base {
		ended[k] = v
	}
	if err := h.publish(ctx, "party.activity_ended", p, now, ended); err != nil {
		return err
	}
	if reason == party.EndCompleted && len(participants) > 0 {
		completed := map[string]any{"member_ids": participants, "completed_at": now}
		for k, v := range base {
			completed[k] = v
		}
		if err := h.publish(ctx, "party.activity_completed", p, now, completed); err != nil {
			return err
		}
	}
	to := p.MemberIDs()
	if !p.Active() {
		to = participants
	}
	fx.send(to, rtSessionEnded, view.Ended(*a))
	fx.resetReady = append(fx.resetReady, p.ID)
	if h.LobbyMetrics != nil {
		h.LobbyMetrics.ActivityEnd(reason)
	}
	return nil
}

// leftSession closes the session membership of a student leaving the party during an activity; the last participant
// out ends the activity all_left (F101-AC-12).
func (h Parties) leftSession(ctx context.Context, fx *effects, p *party.Party, student uuid.UUID, now time.Time) error {
	if h.Activities == nil {
		return nil
	}
	a, err := h.Activities.Running(ctx, p.TenantID, p.ID)
	if err != nil || a == nil {
		return err
	}
	if _, err := h.Activities.CloseMembership(ctx, p.TenantID, a.ID, student, party.SessionLeft, now); err != nil {
		return err
	}
	open, err := h.Activities.OpenMembers(ctx, p.TenantID, a.ID)
	if err != nil {
		return err
	}
	if len(open) == 0 {
		return h.endActivity(ctx, fx, p, a, party.EndAllLeft, nil, now)
	}
	return nil
}

// disbandedSession ends the running activity of a disbanded party (F101-AC-12, reason disbanded).
func (h Parties) disbandedSession(ctx context.Context, fx *effects, p *party.Party, now time.Time) error {
	if h.Activities == nil {
		return nil
	}
	a, err := h.Activities.Running(ctx, p.TenantID, p.ID)
	if err != nil || a == nil {
		return err
	}
	return h.endActivity(ctx, fx, p, a, party.EndDisbanded, nil, now)
}

// EndPartyActivity ends a party activity on an event of module game (boss.encounter_completed, quest.party_completed;
// F101-AC-12) in the transaction of ctx (consumer): idempotent, unknown or ended activities change nothing. Returns
// whether something changed.
func (h Parties) EndPartyActivity(ctx context.Context, tenant, activityID uuid.UUID, reason string, summary map[string]any) (changed bool,
	err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.EndPartyActivity")
	defer func() { done(err) }()
	if h.Activities == nil {
		return false, nil
	}
	a, err := h.Activities.Get(ctx, tenant, activityID)
	if err != nil || a == nil || a.Status != party.ActivityRunning {
		return false, err
	}
	p, err := h.Repo.Load(ctx, tenant, a.PartyID, true)
	if err != nil || p == nil {
		return false, err
	}
	fx := &effects{}
	now := h.now()
	if err := h.endActivity(ctx, fx, p, a, reason, summary, now); err != nil {
		return false, err
	}
	fx.state(p)
	return true, h.after(ctx, fx)
}
