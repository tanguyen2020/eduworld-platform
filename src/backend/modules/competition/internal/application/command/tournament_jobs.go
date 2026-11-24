package command

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/challenge"
	trn "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/tournament"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// tickBatch bounds the work of one pass of the job (the rest waits for the next pass, every 15 s).
const tickBatch = 200

// Tick is one pass of the job tournament-scheduler (F107-AC-07, AC-10, AC-15…AC-20, AC-27): the tournaments and
// challenges with work due are listed across tenants (SystemPlatformScope), then each one moves in a transaction of its
// own tenant: registration opens and closes (auto-confirm), check-in closes (no check-in → withdrawn, byes, too few →
// cancelled), match notices at −10 and −1 minutes, direct proposals at the match time, no-shows, complaint windows,
// completion and rewards; timed-quiz rooms open at starts_at and the challenge completes at ends_at.
func (h Tournaments) Tick(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.TournamentTick")
	defer func() { done(err) }()
	now := h.now()
	ps, err := authz.SystemPlatformScope(JobTournamentScheduler)
	if err != nil {
		return 0, err
	}
	var tournaments, challenges []write.DueRef
	if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		var err error
		if tournaments, err = h.Repo.DueTournaments(ctx, now, tickBatch); err != nil {
			return err
		}
		challenges, err = h.Repo.DueChallenges(ctx, now, tickBatch)
		return err
	}); err != nil {
		return 0, err
	}
	var errs []error
	for _, ref := range challenges {
		sc, err := authz.EventScope(ref.Tenant)
		if err != nil {
			return n, err
		}
		if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error { return h.tickChallenge(ctx, ref, now) }); err != nil {
			errs = append(errs, err)
			continue
		}
		n++
	}
	for _, ref := range tournaments {
		sc, err := authz.EventScope(ref.Tenant)
		if err != nil {
			return n, err
		}
		if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error { return h.tickTournament(ctx, ref, now) }); err != nil {
			errs = append(errs, err)
			continue
		}
		n++
	}
	return n, errors.Join(errs...)
}

// tickChallenge opens the timed-quiz rooms of a challenge at starts_at (the audience of that moment, rooms of at most 40
// students sorted by id; an empty audience cancels it) and completes it at ends_at (F107-AC-27).
func (h Tournaments) tickChallenge(ctx context.Context, ref write.DueRef, now time.Time) error {
	c, err := h.Repo.Challenge(ctx, ref.Tenant, ref.ID, true)
	if err != nil || c == nil || c.Format != challenge.FormatTimedQuiz {
		return err
	}
	switch {
	case c.Status == write.ChallengeScheduled && !now.Before(c.StartsAt):
		aud, err := h.audience(ctx, *c)
		if err != nil {
			return err
		}
		if len(aud) == 0 {
			r := "no_audience"
			c.Status, c.CancelReason = write.ChallengeCancelled, &r
			return h.Repo.UpdateChallenge(ctx, *c)
		}
		grade := 9
		info, err := h.Repo.Students(ctx, c.Tenant, aud, now)
		if err != nil {
			return err
		}
		for _, s := range info {
			grade = min(grade, max(s.Grade, 1))
		}
		var rooms []uuid.UUID
		for k := 0; k*TimedQuizRoomSize < len(aud); k++ {
			id := ChallengeRoomID(c.ID, k)
			end := min((k+1)*TimedQuizRoomSize, len(aud))
			if err := h.Matches.CreateChallengeMatch(ctx, ChallengeMatchInput{MatchID: id, Tenant: c.Tenant, School: c.School,
				SubjectID: c.SubjectID, Grade: grade, Students: aud[k*TimedQuizRoomSize : end]}); err != nil {
				return trnError(err)
			}
			rooms = append(rooms, id)
		}
		c.Status, c.MatchIDs = write.ChallengeRunning, rooms
		return h.Repo.UpdateChallenge(ctx, *c)
	case c.Status == write.ChallengeRunning && !now.Before(c.EndsAt):
		c.Status = write.ChallengeCompleted
		return h.Repo.UpdateChallenge(ctx, *c)
	}
	return nil
}

// tickTournament moves one tournament on by time.
func (h Tournaments) tickTournament(ctx context.Context, ref write.DueRef, now time.Time) error {
	t, err := h.Repo.Tournament(ctx, ref.Tenant, ref.ID, true)
	if err != nil || t == nil {
		return err
	}
	switch t.Status {
	case trn.StatusPublished:
		if !now.Before(t.RegOpenAt) {
			t.Status = trn.StatusRegistrationOpen
			return h.Repo.UpdateTournament(ctx, *t)
		}
	case trn.StatusRegistrationOpen:
		if !now.Before(t.RegCloseAt) {
			entries, err := h.Repo.Entries(ctx, t.Tenant, t.ID)
			if err != nil {
				return err
			}
			if activeCount(entries) < max(t.MinEntries, trn.MinEntries(t.Format)) {
				return h.cancelBySystem(ctx, t, "not_enough_entries", now)
			}
			return h.closeRegistration(ctx, t, now)
		}
	case trn.StatusScheduled:
		if !now.Before(t.StartsAt) {
			return h.closeCheckIn(ctx, t, now)
		}
		return h.earlyNotices(ctx, t, now)
	case trn.StatusInProgress:
		return h.tickBracket(ctx, t, now)
	case trn.StatusCompleted:
		return h.AwardRewards(ctx, t.Tenant, t.ID)
	}
	return nil
}

// cancelBySystem cancels a tournament the job cannot run (too few entries) and tells the organizer through the audit
// trail (tournament.cancelled, reason) — F107-AC-10.
func (h Tournaments) cancelBySystem(ctx context.Context, t *write.TournamentRow, reason string, now time.Time) error {
	if !trn.Cancellable(t.Status) {
		return trn.ErrInvalidTransition
	}
	r := reason
	t.Status, t.CancelReason = trn.StatusCancelled, &r
	if err := h.Repo.UpdateTournament(ctx, *t); err != nil {
		return err
	}
	return h.Audit.Record(ctx, audit.Event{Action: AuditTournamentCancelled, TargetType: "tournament", TargetID: t.ID.String(),
		Result: audit.ResultSuccess, SchoolID: schoolOf(t), Reason: reason, Metadata: map[string]any{"by": "system", "at": now}})
}

// closeCheckIn closes the check-in at starts_at (F107-AC-10): the confirmed entries not checked in are withdrawn and their
// bracket slots become byes; below the minimum of the format the tournament is cancelled, else it is in progress.
func (h Tournaments) closeCheckIn(ctx context.Context, t *write.TournamentRow, now time.Time) error {
	s, err := h.loadBracket(ctx, t)
	if err != nil {
		return err
	}
	gone := map[uuid.UUID]bool{}
	left := 0
	for id, e := range s.entries {
		switch e.Status {
		case trn.EntryConfirmed:
			gone[id] = true
			s.setEntry(id, trn.EntryWithdrawn)
		case trn.EntryCheckedIn:
			left++
		}
	}
	if left < max(t.MinEntries, trn.MinEntries(t.Format)) {
		if err := h.save(ctx, s); err != nil {
			return err
		}
		return h.cancelBySystem(ctx, t, "not_enough_entries", now)
	}
	s.g.Withdraw(gone)
	for _, n := range s.g.Nodes {
		if n.Status == trn.NodeBye || n.Status == trn.NodeVoid {
			s.eliminate(n)
		}
	}
	if err := trn.Move(t.Status, trn.StatusInProgress); err != nil {
		return err
	}
	t.Status = trn.StatusInProgress
	if err := h.Repo.UpdateTournament(ctx, *t); err != nil {
		return err
	}
	t.Version++
	if err := h.settle(ctx, s); err != nil {
		return err
	}
	if t.Status != trn.StatusInProgress {
		return nil
	}
	return h.tickBracket(ctx, t, now) // the first matches start at once
}

// earlyNotices sends the notices of the first matches before the tournament starts (F107-AC-15: 10 and 1 minutes before
// the first round, while the check-in is open).
func (h Tournaments) earlyNotices(ctx context.Context, t *write.TournamentRow, now time.Time) error {
	s, err := h.loadBracket(ctx, t)
	if err != nil {
		return err
	}
	for _, n := range s.g.Nodes {
		if n.Status == trn.NodeScheduled && n.A != nil && n.B != nil && n.ScheduledAt != nil {
			if err := h.notices(ctx, s, n, now); err != nil {
				return err
			}
		}
	}
	return h.save(ctx, s)
}

// tickBracket handles the nodes of a tournament in progress.
func (h Tournaments) tickBracket(ctx context.Context, t *write.TournamentRow, now time.Time) error {
	s, err := h.loadBracket(ctx, t)
	if err != nil {
		return err
	}
	for _, n := range s.g.Nodes {
		switch {
		case n.Status == trn.NodeScheduled && n.A != nil && n.B != nil && n.ScheduledAt != nil:
			if err := h.notices(ctx, s, n, now); err != nil {
				return err
			}
			if !now.Before(*n.ScheduledAt) && n.MatchID == nil {
				if err := h.startNode(ctx, s, n, now); err != nil {
					return err
				}
			}
		case n.Status == trn.NodeInMatch && n.MatchID != nil && n.StartedAt != nil && !now.Before(n.StartedAt.Add(s.timers.NoShow)):
			if err := h.noShow(ctx, s, n, now); err != nil {
				return err
			}
		case n.Status == trn.NodeAwaitingConfirm && !n.DecisionRequired && n.ConfirmAfter != nil && !now.Before(*n.ConfirmAfter):
			if err := n.Confirm(); err != nil {
				return err
			}
		}
	}
	return h.settle(ctx, s)
}

// students of an entry that are still active (a cleaned-up or deactivated student forfeits its next match).
func (s *bracketState) activeMembers(e *write.EntryRow, info map[uuid.UUID]write.StudentInfo) []uuid.UUID {
	var out []uuid.UUID
	for _, m := range e.Members {
		if st, ok := info[m]; ok && st.Active {
			out = append(out, m)
		}
	}
	return out
}

// need is the number of members an entry needs to play: 1, or team_size − 1 for a team (COMPETITION_SYSTEM §10.7).
func (s *bracketState) need(e *write.EntryRow) int {
	if s.t.TeamSize == nil {
		return 1
	}
	return min(*s.t.TeamSize-1, len(e.Members))
}

// notices publishes tournament.match_scheduled (and pushes tournament.match.scheduled) 10 and 1 minutes before the match
// of a node with two entries, once each (F107-AC-15).
func (h Tournaments) notices(ctx context.Context, s *bracketState, n *trn.Node, now time.Time) error {
	for _, nt := range trn.Notices {
		if n.Notices&nt.Bit != 0 || now.Before(n.ScheduledAt.Add(-time.Duration(nt.Minutes)*time.Minute)) {
			continue
		}
		n.Notices |= nt.Bit
		n.Changed = true
		a, b := s.entries[*n.A], s.entries[*n.B]
		students := append(append([]uuid.UUID(nil), a.Members...), b.Members...)
		tenant := s.t.Tenant
		if err := h.Events.Publish(ctx, write.Event{Type: EventTournamentScheduled, SubjectType: "bracket_node", SubjectID: n.ID.String(),
			PartitionKey: s.t.ID.String(), TenantID: &tenant, OccurredAt: now, Payload: map[string]any{"tournament_id": s.t.ID,
				"node_id": n.ID, "school_id": s.t.School, "scheduled_at": n.ScheduledAt.UTC(), "notice_minutes": nt.Minutes,
				"entry_ids": []uuid.UUID{a.ID, b.ID}, "student_ids": students}}); err != nil {
			return err
		}
		minutes, at := nt.Minutes, n.ScheduledAt.UnixMilli()
		cards, err := h.cardsOf(ctx, tenant, students)
		if err != nil {
			return err
		}
		opponents := func(e *write.EntryRow) []map[string]any {
			out := []map[string]any{}
			for _, m := range e.Members {
				nick := any(nil)
				if c, ok := cards[m]; ok && c.Nickname != nil {
					nick = *c.Nickname
				}
				out = append(out, map[string]any{"user_id": m, "nickname": nick})
			}
			return out
		}
		pa, pb := opponents(b), opponents(a)
		if err := h.later(ctx, func(ctx context.Context) {
			h.send(ctx, a.Members, rtTournamentMatchScheduled, map[string]any{"tournament_id": s.t.ID, "node_id": n.ID, "scheduled_at": at,
				"notice_minutes": minutes, "opponents": pa})
			h.send(ctx, b.Members, rtTournamentMatchScheduled, map[string]any{"tournament_id": s.t.ID, "node_id": n.ID, "scheduled_at": at,
				"notice_minutes": minutes, "opponents": pb})
		}); err != nil {
			return err
		}
	}
	return nil
}

// startNode starts the match of a node at its time (F107-AC-15): an entry without enough active members forfeits
// (both: double forfeit); otherwise a direct proposal of matchmaking with a ready check of no_show (F104-AC-21,
// idempotent per node and attempt).
func (h Tournaments) startNode(ctx context.Context, s *bracketState, n *trn.Node, now time.Time) error {
	a, b := s.entries[*n.A], s.entries[*n.B]
	students := append(append([]uuid.UUID(nil), a.Members...), b.Members...)
	info, err := h.Repo.Students(ctx, s.t.Tenant, students, now)
	if err != nil {
		return err
	}
	am, bm := s.activeMembers(a, info), s.activeMembers(b, info)
	okA, okB := len(am) >= s.need(a), len(bm) >= s.need(b)
	if !okA || !okB {
		return h.forfeit(s, n, okA, okB, now, "inactive")
	}
	grade := 9
	for _, m := range append(append([]uuid.UUID(nil), am...), bm...) {
		grade = min(grade, max(info[m].Grade, 1))
	}
	entry := func(members []uuid.UUID, e *write.EntryRow) DirectEntry {
		d := DirectEntry{Students: members, PartyID: e.Party}
		for _, m := range members {
			d.Tenants = append(d.Tenants, s.t.Tenant)
			d.Schools = append(d.Schools, info[m].School)
		}
		return d
	}
	tid := s.t.ID
	id, err := h.Matches.CreateDirectProposal(ctx, DirectInput{TournamentNodeID: n.ID, TournamentID: &tid, Attempt: n.Attempts,
		ReadyCheck: s.timers.NoShow, Mode: s.t.Mode, SubjectID: s.t.SubjectID, Grade: grade, ScopeKind: s.t.OwnerScope,
		Entries: [2]DirectEntry{entry(am, a), entry(bm, b)}})
	if err != nil {
		return trnError(err)
	}
	at := now
	n.MatchID, n.StartedAt, n.Status, n.Changed = &id, &at, trn.NodeInMatch, true
	if h.Metrics != nil {
		h.Metrics.MatchScheduled()
		if n.ScheduledAt != nil {
			h.Metrics.SchedulerLag(now.Sub(*n.ScheduledAt).Seconds())
		}
	}
	return nil
}

// forfeit decides a node where one or both entries did not show up (F107-AC-16): the present entry wins; nobody present
// → double forfeit (knockout: both out and the next node gets a bye; group: both lose).
func (h Tournaments) forfeit(s *bracketState, n *trn.Node, okA, okB bool, now time.Time, reason string) error {
	res := trn.Result{}
	o := trn.Outcome{Result: res}
	switch {
	case okA && !okB:
		o.Winner, o.Result.Forfeit = n.A, "b"
		o.Result.A.Outcome, o.Result.B.Outcome = "win", "forfeit"
	case okB && !okA:
		o.Winner, o.Result.Forfeit = n.B, "a"
		o.Result.A.Outcome, o.Result.B.Outcome = "forfeit", "win"
	default:
		o.DoubleForfeit, o.Result.DoubleForfeit = true, true
		o.Result.A.Outcome, o.Result.B.Outcome = "forfeit", "forfeit"
	}
	if n.Status == trn.NodeScheduled {
		n.Status = trn.NodeInMatch // Apply takes a node scheduled or in a match
	}
	if err := s.g.Apply(n, o, now.Add(s.timers.Complaint)); err != nil {
		return err
	}
	s.eliminate(n)
	if h.Metrics != nil {
		h.Metrics.Forfeit(reason)
	}
	return nil
}

// noShow handles a ready check past the no-show limit without a match (F107-AC-16): every member of an entry needed to
// play accepted → present (a team short of one member is re-proposed once with the members present); the proposal lost
// with Valkey is a technical failure (reschedule).
func (h Tournaments) noShow(ctx context.Context, s *bracketState, n *trn.Node, now time.Time) error {
	created, err := h.Repo.MatchCreated(ctx, *n.MatchID)
	if err != nil || created {
		return err // the match runs: its result comes with match.completed
	}
	accepted, open, found, err := h.Matches.DirectAcceptance(ctx, *n.MatchID)
	if err != nil {
		return err
	}
	if !found {
		h.reschedule(s, n, "proposal_lost")
		slog.WarnContext(ctx, "competition: tournament proposal lost, node rescheduled", "node_id", n.ID)
		return nil
	}
	if open && !now.After(n.StartedAt.Add(s.timers.NoShow+5*time.Second)) {
		return nil // the matchmaker closes it in a moment
	}
	a, b := s.entries[*n.A], s.entries[*n.B]
	count := func(e *write.EntryRow) int {
		c := 0
		for _, m := range e.Members {
			if accepted[m] {
				c++
			}
		}
		return c
	}
	ca, cb := count(a), count(b)
	okA, okB := ca >= s.need(a), cb >= s.need(b)
	if okA && okB {
		if s.t.ParticipantType == trn.ParticipantTeam && n.Attempts == n.Reschedules {
			// both teams showed up short of a member: propose again with the members present only.
			var am, bm []uuid.UUID
			for _, m := range a.Members {
				if accepted[m] {
					am = append(am, m)
				}
			}
			for _, m := range b.Members {
				if accepted[m] {
					bm = append(bm, m)
				}
			}
			info, err := h.Repo.Students(ctx, s.t.Tenant, append(append([]uuid.UUID(nil), am...), bm...), now)
			if err != nil {
				return err
			}
			grade := 9
			for _, st := range info {
				grade = min(grade, max(st.Grade, 1))
			}
			mk := func(members []uuid.UUID, e *write.EntryRow) DirectEntry {
				d := DirectEntry{Students: members, PartyID: e.Party}
				for _, m := range members {
					d.Tenants, d.Schools = append(d.Tenants, s.t.Tenant), append(d.Schools, info[m].School)
				}
				return d
			}
			n.Attempts++
			tid := s.t.ID
			id, err := h.Matches.CreateDirectProposal(ctx, DirectInput{TournamentNodeID: n.ID, TournamentID: &tid, Attempt: n.Attempts,
				ReadyCheck: s.timers.NoShow, Mode: s.t.Mode, SubjectID: s.t.SubjectID, Grade: grade, ScopeKind: s.t.OwnerScope,
				Entries: [2]DirectEntry{mk(am, a), mk(bm, b)}})
			if err != nil {
				return trnError(err)
			}
			at := now
			n.MatchID, n.StartedAt, n.Changed = &id, &at, true
			return nil
		}
		h.reschedule(s, n, "incomplete_ready_check")
		return nil
	}
	return h.forfeit(s, n, okA, okB, now, "no_show")
}
