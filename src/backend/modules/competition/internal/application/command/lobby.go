package command

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// LobbyConfig is the configuration of the Party Lobby and the party sessions (CONFIG_DEFAULTS party_lobby_cmd_rate,
// private_instance_capacity, party_leader_grace, party_dormant_timeout, disconnect_grace private_instance/boss,
// party_activity_min_members_timeout); read by infrastructure/config, never hard-coded.
type LobbyConfig struct {
	CmdPerSecond      int
	PrivateCapacity   int
	LeaderGrace       time.Duration
	DormantTimeout    time.Duration
	MemberGrace       time.Duration
	MinMembersTimeout time.Duration
	SweepBatch        int
}

func (c LobbyConfig) sweep() party.SweepConfig {
	return party.SweepConfig{LeaderGrace: c.LeaderGrace, DormantTimeout: c.DormantTimeout, MemberGrace: c.MemberGrace,
		MinMembersTimeout: c.MinMembersTimeout}
}

// Commands of eduworld_party_command_total (F101).
const (
	CmdReady  = "set_ready"
	CmdSelect = "select_activity"
	CmdStart  = "start_session"
)

// Audit action of a start refused because a member is not eligible (plan F101: ids and reason codes only).
const AuditSessionStartDenied = "party.session_start_denied"

// Realtime message types of the lobby (AsyncAPI).
const (
	rtReadyChanged     = "party.member.ready_changed"
	rtStatusChanged    = "party.member.status_changed"
	rtActivitySelected = "party.activity.selected"
	rtSessionStarted   = "party.session.started"
	rtSessionEnded     = "party.session.ended"
)

// mapLobbyError maps the lobby and session errors of the domain (after mapError).
func mapLobbyError(err error) error {
	switch {
	case errors.Is(err, party.ErrNotOpen), errors.Is(err, party.ErrNoSelection):
		return problem.New(problem.InvalidStateTransition, "")
	case errors.Is(err, party.ErrVersion):
		return problem.New(problem.VersionMismatch, "")
	case errors.Is(err, party.ErrNotReady):
		return problem.New(problem.PartyNotReady, "")
	case errors.Is(err, party.ErrActivityDisabled):
		return problem.Invalid("activity_type", problem.SubInvalid, "activity type not available")
	case errors.Is(err, write.ErrActivityRunning):
		return problem.New(problem.InvalidStateTransition, "")
	case errors.Is(err, port.ErrGameUnavailable):
		return RetryAfter{Err: problem.New(problem.Unavailable, "").Retry(), After: 2 * time.Second}
	case errors.Is(err, port.ErrGameRefused):
		return problem.Invalid("activity_ref", problem.SubInvalid, "activity not available")
	}
	return err
}

// versionMismatch is 412 VERSION_MISMATCH with the current state_version (F101-AC-04).
func versionMismatch(p *party.Party) error {
	return problem.New(problem.VersionMismatch, "").With("state_version", p.StateVersion)
}

// lobbyMetric observes the time from the command to the pushes of the lobby (eduworld_party_lobby_update_seconds).
func (h Parties) lobbyMetric(start time.Time) {
	if h.LobbyMetrics != nil {
		h.LobbyMetrics.LobbyUpdate(time.Since(start))
	}
}

// allow counts a lobby command against party_lobby_cmd_rate (Valkey; a lost counter lets the command through).
func (h Parties) allow(ctx context.Context, me uuid.UUID) error {
	if h.Lobby == nil || h.LobbyConfig.CmdPerSecond <= 0 {
		return nil
	}
	ok, wait, err := h.Lobby.Allow(ctx, me, h.LobbyConfig.CmdPerSecond)
	if err != nil {
		slog.WarnContext(ctx, "competition: lobby rate limit unavailable", "error", err)
		return nil
	}
	if !ok {
		return RetryAfter{Err: problem.New(problem.RateLimited, "").Retry(), After: wait}
	}
	return nil
}

// lobby returns the lobby state of a party (empty when Valkey lost it: Ready false).
func (h Parties) lobby(ctx context.Context, partyID uuid.UUID) party.Lobby {
	if h.Lobby == nil {
		return party.Lobby{}
	}
	l, err := h.Lobby.Load(ctx, partyID)
	if err != nil {
		slog.WarnContext(ctx, "competition: lobby state unavailable", "error", err) // ephemeral: Ready false
		return party.Lobby{}
	}
	return l
}

// withScope makes sure ctx carries the TenantScope of the party for the facade calls (R29, F101-AC-20).
func withScope(ctx context.Context, tenant uuid.UUID) (context.Context, error) {
	if s, ok := tenancy.ScopeFrom(ctx); ok {
		if id, ok := s.SessionTenantID(); ok && id == tenant {
			return ctx, nil
		}
	}
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return ctx, err
	}
	return tenancy.WithScope(ctx, sc), nil
}

// peek loads the party of the caller without a lock (short read transaction) and checks the membership.
func (h Parties) peek(ctx context.Context, tenant, me, partyID uuid.UUID) (*party.Party, error) {
	var p *party.Party
	err := h.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		if p, err = h.Repo.Load(ctx, tenant, partyID, false); err != nil {
			return err
		}
		if p == nil {
			return problem.New(problem.PartyNotFound, "")
		}
		return p.RequireMember(me)
	})
	return p, err
}

// SetReady implements setPartyReady and party.member.ready_set (F101-AC-01, AC-02): a member of an open party; the
// Ready flag lives in the lobby state only (no domain event), every member receives party.member.ready_changed with
// all_ready.
func (h Parties) SetReady(ctx context.Context, partyID uuid.UUID, ready bool) (err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.SetPartyReady")
	defer func() { done(err) }()
	start := time.Now()
	tenant, me, err := student(ctx)
	if err != nil {
		return err
	}
	err = func() error {
		if h.Lobby == nil {
			return problem.New(problem.Unavailable, "").Retry()
		}
		if err := h.allow(ctx, me); err != nil {
			return err
		}
		p, err := h.peek(ctx, tenant, me, partyID)
		if err != nil {
			return err
		}
		if p.Status != party.StatusOpen {
			return party.ErrNotOpen
		}
		if err := h.Lobby.SetReady(ctx, p.ID, me, ready); err != nil {
			return err
		}
		online := h.online(ctx, p.MemberIDs())
		online[me] = true
		all := party.AllReady(p, online, h.lobby(ctx, p.ID).Ready())
		if h.Realtime != nil {
			if err := h.Realtime.Send(ctx, p.MemberIDs(), rtReadyChanged, map[string]any{"party_id": p.ID, "user_id": me, "ready": ready,
				"all_ready": all}); err != nil {
				slog.WarnContext(ctx, "competition: ready push failed", "error", err) // members resync with getParty
			}
		}
		h.lobbyMetric(start)
		return nil
	}()
	return h.finish(ctx, CmdReady, err)
}

// SelectInput is selectPartyActivity / party.activity.select.
type SelectInput struct {
	Selection party.Selection
	Expected  int64
}

// SelectActivity implements selectPartyActivity and party.activity.select (F101-AC-03, AC-04): leader only, open
// party, expected_version; the selection is durable, Ready of every member resets, party.activity_selected and
// party.activity.selected with the ineligible members (reason codes only).
func (h Parties) SelectActivity(ctx context.Context, partyID uuid.UUID, in SelectInput) (res view.Party, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.SelectPartyActivity")
	defer func() { done(err) }()
	start := time.Now()
	tenant, me, err := student(ctx)
	if err != nil {
		return res, err
	}
	err = func() error {
		if err := h.allow(ctx, me); err != nil {
			return err
		}
		if !party.ValidActivityType(in.Selection.Type) || !party.EnabledActivityType(in.Selection.Type) {
			return party.ErrActivityDisabled
		}
		p, err := h.peek(ctx, tenant, me, partyID)
		if err != nil {
			return err
		}
		if err := p.RequireLeader(me); err != nil {
			return err
		}
		if p.Status != party.StatusOpen {
			return party.ErrNotOpen
		}
		if in.Expected != p.StateVersion {
			return versionMismatch(p)
		}
		// Eligibility before the transaction: module game is called outside any transaction of module competition.
		elig, err := h.eligibility(ctx, p, in.Selection, p.MemberIDs())
		if err != nil {
			return err
		}
		return h.UoW.Do(ctx, func(ctx context.Context) error {
			now := h.now()
			fx := &effects{}
			p, err := h.load(ctx, tenant, partyID)
			if err != nil {
				return err
			}
			if err := p.Select(me, in.Selection, in.Expected, now); err != nil {
				if errors.Is(err, party.ErrVersion) {
					return versionMismatch(p)
				}
				return err
			}
			if err := h.Repo.Save(ctx, p); err != nil {
				return err
			}
			payload := map[string]any{"party_id": p.ID, "tenant_id": p.TenantID, "activity_type": in.Selection.Type,
				"activity_ref": in.Selection.Ref, "selected_by": me, "state_version": p.StateVersion}
			if in.Selection.GameMode != "" {
				payload["game_mode"] = in.Selection.GameMode
			}
			if err := h.publish(ctx, "party.activity_selected", p, now, payload); err != nil {
				return err
			}
			push := map[string]any{"party_id": p.ID, "activity": view.ActivityRef(in.Selection.Type, in.Selection.Ref),
				"state_version": p.StateVersion, "ineligible_members": view.Ineligible(elig.Ineligible(), "user_id")}
			if in.Selection.GameMode != "" {
				push["game_mode"] = in.Selection.GameMode
			}
			fx.send(p.MemberIDs(), rtActivitySelected, push)
			fx.resetReady = append(fx.resetReady, p.ID)
			fx.state(p)
			if res, err = h.view(ctx, p); err != nil {
				return err
			}
			l := h.lobby(ctx, p.ID)
			for id, m := range l.Members {
				m.Ready = false
				l.Members[id] = m
			}
			res.Lobby = &l
			return h.after(ctx, fx)
		})
	}()
	if err == nil {
		h.lobbyMetric(start)
	}
	return res, h.finish(ctx, CmdSelect, err)
}

// eligibility checks an activity for students (F101-AC-03, AC-05, AC-08): explore from the projections of module
// competition, quest/boss from the facade game (outside any transaction, TenantScope of the party).
func (h Parties) eligibility(ctx context.Context, p *party.Party, sel party.Selection, students []uuid.UUID) (party.Eligibility, error) {
	if party.FromGame(sel.Type) {
		if h.Game == nil {
			return party.Eligibility{}, port.ErrGameUnavailable
		}
		gctx, err := withScope(ctx, p.TenantID)
		if err != nil {
			return party.Eligibility{}, err
		}
		return h.Game.Check(gctx, p.ID, sel.Type, sel.Ref, students)
	}
	if h.Areas == nil {
		return party.Eligibility{}, port.ErrGameUnavailable
	}
	var e party.Eligibility
	err := h.UoW.Do(ctx, func(ctx context.Context) error {
		area, err := h.Areas.Area(ctx, sel.Ref)
		if err != nil {
			return err
		}
		if area == nil || !area.Active {
			return problem.Invalid("activity_ref", problem.SubInvalid, "unknown area")
		}
		subjects := make([]party.Subject, 0, len(students))
		now := h.now()
		for _, id := range students {
			s, err := h.subject(ctx, p.TenantID, id, now)
			if err != nil {
				return err
			}
			subjects = append(subjects, s)
		}
		unlocked, err := h.Areas.Unlocked(ctx, p.TenantID, students, area.ID)
		if err != nil {
			return err
		}
		e = party.ExploreEligibility(*area, subjects, unlocked, h.LobbyConfig.PrivateCapacity)
		return nil
	})
	return e, err
}

// afterLobby runs the lobby effects after the commit (best effort, ADR-0004): Ready resets, tracking of the active
// parties by the presence sweeper, lobby state of a disbanded party dropped.
func (h Parties) afterLobby(ctx context.Context, fx *effects) {
	if h.Lobby == nil {
		return
	}
	for _, id := range fx.resetReady {
		if err := h.Lobby.ResetReady(ctx, id); err != nil {
			slog.WarnContext(ctx, "competition: ready not reset", "error", err)
		}
	}
	if fx.sync != nil && fx.sync.TenantID != uuid.Nil {
		if err := h.Lobby.Track(ctx, fx.sync.TenantID, fx.sync.PartyID, h.now()); err != nil {
			slog.WarnContext(ctx, "competition: party not tracked", "error", err) // the sweeper reconciles from PostgreSQL
		}
	}
	if fx.drop != nil && fx.drop.party != uuid.Nil {
		if err := h.Lobby.Drop(ctx, fx.drop.tenant, fx.drop.party); err != nil {
			slog.WarnContext(ctx, "competition: lobby state not dropped", "error", err)
		}
	}
}
