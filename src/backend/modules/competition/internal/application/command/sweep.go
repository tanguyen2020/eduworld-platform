package command

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// SweeperJob is the lock name of the presence sweeper (ew:lock:job:party-presence-sweeper, research R3).
const SweeperJob = "party-presence-sweeper"

// reconcileEvery is how many sweeps pass between two reconciliations of ew:party:active with PostgreSQL (after a Valkey
// loss the tracked set is rebuilt within one minute at the 5 s cycle).
const reconcileEvery = 12

// Sweeper runs the job party-presence-sweeper (PARTY_SYSTEM §9, research R3): every cycle it evaluates the tracked
// parties with the presence of their members, independently of the gateway node holding the connections.
type Sweeper struct {
	Parties Parties
	Every   time.Duration // cycle (lease of the job lock)
	runs    *int
}

// NewSweeper builds the sweeper of the job.
func NewSweeper(p Parties, every time.Duration) Sweeper {
	return Sweeper{Parties: p, Every: every, runs: new(int)}
}

// Run is one cycle: the job lock (one replica per cycle), the reconciliation every reconcileEvery cycles, then each
// tracked party. Returns the number of parties evaluated.
func (s Sweeper) Run(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.SweepPartyPresence")
	defer func() { done(err) }()
	h := s.Parties
	if h.Lobby == nil {
		return 0, nil
	}
	if h.Locks != nil {
		ok, err := h.Locks.Acquire(ctx, SweeperJob, s.Every)
		if err != nil || !ok {
			return 0, err
		}
	}
	if s.runs != nil {
		*s.runs++
		if *s.runs%reconcileEvery == 1 {
			if err := h.reconcile(ctx); err != nil {
				slog.WarnContext(ctx, "competition: active parties not reconciled", "error", err)
			}
		}
	}
	batch := h.LobbyConfig.SweepBatch
	if batch <= 0 {
		batch = 1000
	}
	refs, err := h.Lobby.Tracked(ctx, batch)
	if err != nil {
		return 0, err
	}
	for _, r := range refs {
		if err := h.SweepParty(ctx, r); err != nil {
			slog.WarnContext(ctx, "competition: party not swept", "error", err) // next cycle retries
			continue
		}
		n++
	}
	return n, nil
}

// reconcile tracks the active parties of PostgreSQL (SystemPlatformScope, ids only) missing from ew:party:active.
func (h Parties) reconcile(ctx context.Context) error {
	if h.Sweeps == nil {
		return nil
	}
	ps, err := authz.SystemPlatformScope(SweeperJob)
	if err != nil {
		return err
	}
	after := uuid.Nil
	for {
		var refs []port.PartyRef
		if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
			var err error
			refs, err = h.Sweeps.ActiveParties(ctx, after, 500)
			return err
		}); err != nil {
			return err
		}
		for _, r := range refs {
			if err := h.Lobby.Track(ctx, r.TenantID, r.PartyID, time.Time{}); err != nil {
				return err
			}
			after = r.PartyID
		}
		if len(refs) < 500 {
			return nil
		}
	}
}

// SweepParty evaluates one party (F101-AC-14…AC-18) and applies the outcome: member statuses and Ready reset in the
// lobby state with their pushes, then in one transaction the automatic leader transfer, the dormant disband, the
// session memberships past their grace and the end of the activity.
func (h Parties) SweepParty(ctx context.Context, r port.PartyRef) error {
	sc, err := authz.EventScope(r.TenantID)
	if err != nil {
		return err
	}
	var p *party.Party
	var a *party.Activity
	var open map[uuid.UUID]bool
	if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		var err error
		if p, err = h.Repo.Load(ctx, r.TenantID, r.PartyID, false); err != nil || p == nil || !p.Active() || h.Activities == nil {
			return err
		}
		if a, err = h.Activities.Running(ctx, r.TenantID, r.PartyID); err != nil || a == nil {
			return err
		}
		open, err = h.Activities.OpenMembers(ctx, r.TenantID, a.ID)
		return err
	}); err != nil {
		return err
	}
	if p == nil || !p.Active() {
		return h.Lobby.Drop(ctx, r.TenantID, r.PartyID)
	}
	now := h.now()
	lobby := h.lobby(ctx, p.ID)
	statuses, err := h.statuses(ctx, p.MemberIDs())
	if err != nil {
		return err
	}
	out := party.Sweep(party.SweepInput{Party: p, Lobby: lobby, Presence: statuses, Activity: a, Open: open, Now: now}, h.LobbyConfig.sweep())
	if out.Dirty || len(out.ReadyReset) > 0 {
		if err := h.Lobby.Save(ctx, p.ID, out.Lobby); err != nil {
			return err
		}
	}
	if h.Realtime != nil {
		h.snapshotBack(ctx, sc, p, a, lobby, out)
		for _, c := range out.Changes {
			if err := h.Realtime.Send(ctx, p.MemberIDs(), rtStatusChanged, map[string]any{"party_id": p.ID, "user_id": c.StudentID,
				"status": c.Status}); err != nil {
				slog.WarnContext(ctx, "competition: status push failed", "error", err)
			}
		}
		if len(out.ReadyReset) > 0 {
			online := map[uuid.UUID]bool{}
			for id, st := range statuses {
				online[id] = party.Connected(st)
			}
			all := party.AllReady(p, online, out.Lobby.Ready())
			for _, id := range out.ReadyReset {
				_ = h.Realtime.Send(ctx, p.MemberIDs(), rtReadyChanged, map[string]any{"party_id": p.ID, "user_id": id, "ready": false,
					"all_ready": all})
			}
		}
	}
	if out.TransferTo == uuid.Nil && !out.Disband && len(out.CloseMember) == 0 && out.EndActivity == "" {
		return nil
	}
	return h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error { return h.applySweep(ctx, r, out, statuses, now) })
}

// statuses returns the member statuses of the lobby from the presence store (missing = disconnected).
func (h Parties) statuses(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := make(map[uuid.UUID]string, len(ids))
	if h.Statuses == nil {
		for id, on := range h.online(ctx, ids) {
			if on {
				out[id] = party.MemberOnline
			}
		}
		return out, nil
	}
	raw, err := h.Statuses.Statuses(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("party presence: %w", err)
	}
	for _, id := range ids {
		out[id] = party.MemberStatus(raw[id])
	}
	return out, nil
}

// applySweep applies the transactional part of a sweep on the party locked again (the state may have changed since
// the evaluation: every step re-checks it).
func (h Parties) applySweep(ctx context.Context, r port.PartyRef, out party.SweepOutcome, statuses map[uuid.UUID]string, now time.Time) error {
	fx := &effects{}
	p, err := h.Repo.Load(ctx, r.TenantID, r.PartyID, true)
	if err != nil || p == nil || !p.Active() {
		return err
	}
	if out.Disband {
		for _, id := range p.MemberIDs() {
			if party.Connected(statuses[id]) {
				return nil // somebody came back
			}
		}
		members, ok := p.Disband(now)
		if !ok {
			return nil
		}
		left := make([]uuid.UUID, 0, len(members))
		for _, m := range members {
			if err := h.Repo.EndMember(ctx, p.TenantID, p.ID, m.StudentID, party.LeftDisbanded, now); err != nil {
				return err
			}
			left = append(left, m.StudentID)
		}
		if err := h.disbanded(ctx, fx, p, party.DisbandInactivity, nil, left, now); err != nil {
			return err
		}
		return h.after(ctx, fx)
	}
	if to := out.TransferTo; to != uuid.Nil {
		if _, ok := p.Member(to); ok && to != p.LeaderID && !party.Connected(statuses[p.LeaderID]) {
			from := p.LeaderID
			if err := p.Transfer(to, now); err != nil {
				return err
			}
			if err := h.Repo.SetRoles(ctx, p); err != nil {
				return err
			}
			if err := h.Repo.Save(ctx, p); err != nil {
				return err
			}
			if err := h.transferred(ctx, fx, p, from, to, party.TransferLeaderDisconnected, now); err != nil {
				return err
			}
			if h.LobbyMetrics != nil {
				h.LobbyMetrics.LeaderAutoTransfer()
			}
		}
	}
	if h.Activities != nil && (len(out.CloseMember) > 0 || out.EndActivity != "") {
		a, err := h.Activities.Running(ctx, p.TenantID, p.ID)
		if err != nil {
			return err
		}
		if a != nil {
			for _, id := range out.CloseMember {
				if _, err := h.Activities.CloseMembership(ctx, p.TenantID, a.ID, id, party.SessionDisconnected, now); err != nil {
					return err
				}
			}
			if out.EndActivity != "" {
				if err := h.endActivity(ctx, fx, p, a, out.EndActivity, nil, now); err != nil {
					return err
				}
			}
		}
	}
	fx.state(p)
	return h.after(ctx, fx)
}

// snapshotBack pushes party.state.snapshot to the members who came back online (F101-AC-06: full state on reconnection,
// with the room of the running session to return to, F101-AC-17).
func (h Parties) snapshotBack(ctx context.Context, sc tenancy.Scope, p *party.Party, a *party.Activity, before party.Lobby, out party.SweepOutcome) {
	var back []uuid.UUID
	for _, c := range out.Changes {
		if party.Connected(c.Status) && !party.Connected(before.Member(c.StudentID).Status) {
			back = append(back, c.StudentID)
		}
	}
	if len(back) == 0 {
		return
	}
	var v view.Party
	if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		var err error
		v, err = h.view(ctx, p)
		return err
	}); err != nil {
		slog.WarnContext(ctx, "competition: snapshot not built", "error", err)
		return
	}
	l := out.Lobby
	v.Lobby = &l
	if a != nil {
		room := a.RoomID
		v.Room = &room
	}
	if err := h.Realtime.Send(ctx, back, rtSnapshot, v.Snapshot()); err != nil {
		slog.WarnContext(ctx, "competition: snapshot push failed", "error", err)
	}
}
