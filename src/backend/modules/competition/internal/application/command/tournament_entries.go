package command

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/challenge"
	trn "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/tournament"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Skip reasons of a registration made by the organizer.
const (
	SkipNotEligible       = "not_eligible"
	SkipRestricted        = "restricted"
	SkipAlreadyRegistered = "already_registered"
	SkipFull              = "full"
)

// EntryInput is TournamentEntryCreate.
type EntryInput struct {
	StudentIDs []uuid.UUID
	PartyID    *uuid.UUID
}

// Skipped is a student the organizer asked for and who got no entry.
type Skipped struct {
	StudentID uuid.UUID `json:"student_id"`
	Reason    string    `json:"reason"`
}

// EntryResult is TournamentEntryResult.
type EntryResult struct {
	Entries []map[string]any `json:"entries"`
	Skipped []Skipped        `json:"skipped,omitempty"`
}

// visible reports whether a student (school, grade) is in the scope of a tournament (F107-AC-08, AC-25): same tenant
// (RLS), the school of a school tournament, a grade of the tournament, and the audience of its challenge.
func visible(t *write.TournamentRow, c *write.ChallengeRow, me uuid.UUID, s write.StudentInfo) bool {
	if !s.Active || (t.School != nil && s.School != *t.School) || !slices.Contains(t.Grades, s.Grade) {
		return false
	}
	if c != nil {
		return challenge.InAudience(me, s.School, s.Grade, c.School, c.Grade, c.Listed, c.Exclude)
	}
	return true
}

// studentVisible is a status a student may see (no draft).
func studentVisible(status string) bool { return status != trn.StatusDraft }

// tournamentScope loads the tournament (locked) and its challenge.
func (h Tournaments) tournamentScope(ctx context.Context, tenant, id uuid.UUID) (*write.TournamentRow, *write.ChallengeRow, error) {
	t, err := h.Repo.Tournament(ctx, tenant, id, true)
	if err != nil || t == nil {
		return nil, nil, err
	}
	var c *write.ChallengeRow
	if t.ChallengeID != nil {
		if c, err = h.Repo.Challenge(ctx, tenant, *t.ChallengeID, false); err != nil {
			return nil, nil, err
		}
	}
	return t, c, nil
}

// RegisterTournamentEntry implements registerTournamentEntry (F107-AC-08, AC-09, AC-24). A student registers itself (or
// its party for a team tournament: the leader, every member eligible and of the school, team_size−1 to team_size
// members): outside the scope → 404; restricted → 403 TOURNAMENT_NOT_ELIGIBLE; full → 409 TOURNAMENT_FULL; outside the
// registration window → 422 TOURNAMENT_REGISTRATION_CLOSED (same HTTP statuses as before the codes of 2026-11-27).
// The organizer registers students on their behalf (student_ids, or the whole audience of the challenge when empty):
// the students who are not eligible are skipped. Idempotent (Idempotency-Key).
func (h Tournaments) RegisterTournamentEntry(ctx context.Context, id uuid.UUID, in EntryInput) (out EntryResult, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.RegisterTournamentEntry")
	defer func() { done(err) }()
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return out, problem.New(problem.Unauthenticated, "")
	}
	switch {
	case p.UserType == authz.UserTypeStudent:
		out, err = h.registerSelf(ctx, id, in)
	case p.UserType == authz.UserTypeStaff && p.Channel == authz.ChannelGame:
		return out, problem.New(problem.GameAccessStudentOnly, "")
	default:
		out, err = h.registerOnBehalf(ctx, id, in)
	}
	return out, trnError(err)
}

func (h Tournaments) registerSelf(ctx context.Context, id uuid.UUID, in EntryInput) (out EntryResult, err error) {
	tenant, me, err := student(ctx)
	if err != nil {
		return out, err
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		t, c, err := h.tournamentScope(ctx, tenant, id)
		if err != nil {
			return err
		}
		if t == nil || !studentVisible(t.Status) {
			return problem.New(problem.NotFound, "")
		}
		members := []uuid.UUID{me}
		var party *uuid.UUID
		if t.ParticipantType == trn.ParticipantTeam {
			if in.PartyID == nil {
				return problem.Invalid("party_id", problem.SubRequired, "a team tournament registers a party")
			}
			leader, ms, err := h.Repo.PartyMembers(ctx, tenant, *in.PartyID)
			if err != nil {
				return err
			}
			if leader == nil || !slices.Contains(ms, me) {
				return problem.New(problem.NotFound, "")
			}
			if *leader != me {
				return problem.New(problem.PartyNotLeader, "")
			}
			size := 2
			if t.TeamSize != nil {
				size = *t.TeamSize
			}
			if len(ms) < size-1 || len(ms) > size {
				return problem.Invalid("party_id", problem.SubInvalid, "the party has team_size or team_size − 1 members")
			}
			members, party = ms, in.PartyID
		}
		info, err := h.Repo.Students(ctx, tenant, members, now)
		if err != nil {
			return err
		}
		self, ok := info[me]
		if !ok || !visible(t, c, me, self) {
			return problem.New(problem.NotFound, "")
		}
		for _, m := range members {
			s, ok := info[m]
			if !ok || !visible(t, c, m, s) {
				return problem.Invalid("party_id", problem.SubInvalid, "every member is eligible and of the school of the tournament")
			}
			if s.Restricted {
				return problem.New(problem.TournamentNotEligible, "")
			}
		}
		if t.Status == trn.StatusRegistrationClosed && now.Before(t.RegCloseAt) {
			return problem.New(problem.TournamentFull, "") // closed early by max_entries (F107-AC-08)
		}
		if t.Status != trn.StatusRegistrationOpen || now.Before(t.RegOpenAt) || !now.Before(t.RegCloseAt) {
			return problem.New(problem.TournamentRegistrationClosed, "")
		}
		var revive *write.EntryRow
		for _, m := range members {
			e, err := h.Repo.EntryOfMember(ctx, tenant, t.ID, m)
			if err != nil {
				return err
			}
			if e != nil && e.Status != trn.EntryWithdrawn {
				return problem.New(problem.Conflict, "already registered")
			}
			if e != nil && m == me && t.ParticipantType == trn.ParticipantIndividual {
				revive = e
			}
		}
		entries, err := h.Repo.Entries(ctx, tenant, t.ID)
		if err != nil {
			return err
		}
		active := activeCount(entries)
		if active >= t.MaxEntries {
			return problem.New(problem.TournamentFull, "")
		}
		var e write.EntryRow
		if revive != nil {
			e = *revive
			e.Status, e.RegisteredBy, e.CheckedInAt = trn.EntryRegistered, me, nil
			if err := h.Repo.UpdateEntry(ctx, e); err != nil {
				return err
			}
			e.Version++
		} else {
			e = write.EntryRow{ID: ids.New(), Tournament: t.ID, Tenant: tenant, School: self.School, Party: party, Members: members,
				Status: trn.EntryRegistered, RegisteredBy: me, Version: 1}
			if party == nil {
				s := me
				e.Student = &s
			}
			if err := h.Repo.InsertEntry(ctx, e); err != nil {
				return err
			}
		}
		if active+1 >= t.MaxEntries {
			if err := h.closeRegistration(ctx, t, now); err != nil {
				return err
			}
		}
		cards, err := h.cardsOf(ctx, tenant, members)
		if err != nil {
			return err
		}
		out = EntryResult{Entries: []map[string]any{view.Entry(e, cards, "", nil)}}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

func activeCount(entries []write.EntryRow) int {
	n := 0
	for _, e := range entries {
		if e.Status != trn.EntryWithdrawn && e.Status != trn.EntryDisqualified {
			n++
		}
	}
	return n
}

// cardsOf reads the public cards (nickname, avatar) of students. Nil Cards reader → no card (ids only).
func (h Tournaments) cardsOf(ctx context.Context, tenant uuid.UUID, students []uuid.UUID) (map[uuid.UUID]port.Card, error) {
	if h.Cards == nil {
		return map[uuid.UUID]port.Card{}, nil
	}
	return h.Cards.Cards(ctx, tenant, students)
}

func (h Tournaments) registerOnBehalf(ctx context.Context, id uuid.UUID, in EntryInput) (out EntryResult, err error) {
	p, tenant, err := organizer(ctx)
	if err != nil {
		return out, err
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		t, c, err := h.tournamentScope(ctx, tenant, id)
		if err != nil {
			return err
		}
		if t == nil || !Covers(p, t.School) {
			return problem.New(problem.NotFound, "")
		}
		if t.ParticipantType == trn.ParticipantTeam {
			return problem.Invalid("student_ids", problem.SubInvalid, "a team tournament is registered by the party leaders")
		}
		if t.Status != trn.StatusRegistrationOpen {
			return trn.ErrInvalidTransition
		}
		candidates := in.StudentIDs
		if len(candidates) == 0 && c != nil {
			if candidates, err = h.audience(ctx, *c); err != nil {
				return err
			}
		}
		info, err := h.Repo.Students(ctx, tenant, candidates, now)
		if err != nil {
			return err
		}
		entries, err := h.Repo.Entries(ctx, tenant, t.ID)
		if err != nil {
			return err
		}
		active := activeCount(entries)
		out = EntryResult{Entries: []map[string]any{}, Skipped: []Skipped{}}
		var created []write.EntryRow
		for _, sid := range candidates {
			s, ok := info[sid]
			switch {
			case !ok || !visible(t, c, sid, s):
				out.Skipped = append(out.Skipped, Skipped{sid, SkipNotEligible})
				continue
			case s.Restricted:
				out.Skipped = append(out.Skipped, Skipped{sid, SkipRestricted})
				continue
			}
			e, err := h.Repo.EntryOfMember(ctx, tenant, t.ID, sid)
			if err != nil {
				return err
			}
			if e != nil && e.Status != trn.EntryWithdrawn {
				out.Skipped = append(out.Skipped, Skipped{sid, SkipAlreadyRegistered})
				continue
			}
			if active >= t.MaxEntries {
				out.Skipped = append(out.Skipped, Skipped{sid, SkipFull})
				continue
			}
			if e != nil {
				e.Status, e.RegisteredBy, e.CheckedInAt = trn.EntryRegistered, p.UserID, nil
				if err := h.Repo.UpdateEntry(ctx, *e); err != nil {
					return err
				}
				e.Version++
				created = append(created, *e)
			} else {
				st := sid
				ne := write.EntryRow{ID: ids.New(), Tournament: t.ID, Tenant: tenant, School: s.School, Student: &st, Members: []uuid.UUID{sid},
					Status: trn.EntryRegistered, RegisteredBy: p.UserID, Version: 1}
				if err := h.Repo.InsertEntry(ctx, ne); err != nil {
					return err
				}
				created = append(created, ne)
			}
			active++
		}
		if active >= t.MaxEntries {
			if err := h.closeRegistration(ctx, t, now); err != nil {
				return err
			}
		}
		for _, e := range created {
			out.Entries = append(out.Entries, view.Entry(e, nil, "", nil))
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// closeRegistration moves registration_open → registration_closed (close_at reached or max_entries, §10.3); auto-confirm
// templates confirm every registered entry at once (F107-AC-09) and move to seeding.
func (h Tournaments) closeRegistration(ctx context.Context, t *write.TournamentRow, now time.Time) error {
	if err := trn.Move(t.Status, trn.StatusRegistrationClosed); err != nil {
		return err
	}
	t.Status = trn.StatusRegistrationClosed
	if t.AutoConfirm {
		if _, err := h.confirm(ctx, t, nil, now); err != nil {
			return err
		}
		t.Status = trn.StatusSeeding
	}
	if err := h.Repo.UpdateTournament(ctx, *t); err != nil {
		return err
	}
	t.Version++
	return nil
}

// confirm confirms the registered entries (all, or ids) whose students are still active; the others stay registered
// (out of the bracket) and inactive ones are withdrawn.
func (h Tournaments) confirm(ctx context.Context, t *write.TournamentRow, only []uuid.UUID, now time.Time) ([]write.EntryRow, error) {
	entries, err := h.Repo.Entries(ctx, t.Tenant, t.ID)
	if err != nil {
		return nil, err
	}
	var students []uuid.UUID
	for _, e := range entries {
		students = append(students, e.Members...)
	}
	info, err := h.Repo.Students(ctx, t.Tenant, students, now)
	if err != nil {
		return nil, err
	}
	var out []write.EntryRow
	for _, e := range entries {
		if e.Status != trn.EntryRegistered || (len(only) > 0 && !slices.Contains(only, e.ID)) {
			if e.Status == trn.EntryConfirmed {
				out = append(out, e)
			}
			continue
		}
		active := 0
		for _, m := range e.Members {
			if s, ok := info[m]; ok && s.Active {
				active++
			}
		}
		need := 1
		if t.TeamSize != nil {
			need = *t.TeamSize - 1
		}
		if active < need {
			e.Status = trn.EntryWithdrawn
		} else {
			e.Status = trn.EntryConfirmed
		}
		if err := h.Repo.UpdateEntry(ctx, e); err != nil {
			return nil, err
		}
		e.Version++
		if e.Status == trn.EntryConfirmed {
			out = append(out, e)
		}
	}
	return out, nil
}

// ConfirmTournamentEntries implements confirmTournamentEntries (F107-AC-09): registration_closed → seeding with the
// confirmed entries (all registered ones, or entry_ids); audit tournament.entries_confirmed.
func (h Tournaments) ConfirmTournamentEntries(ctx context.Context, id uuid.UUID, only []uuid.UUID) (out EntryResult, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.ConfirmTournamentEntries")
	defer func() { done(err) }()
	err = h.organizerTournament(ctx, id, func(ctx context.Context, p authz.Principal, t *write.TournamentRow) error {
		if err := trn.Move(t.Status, trn.StatusSeeding); err != nil {
			return err
		}
		confirmed, err := h.confirm(ctx, t, only, h.now())
		if err != nil {
			return err
		}
		t.Status = trn.StatusSeeding
		if err := h.Repo.UpdateTournament(ctx, *t); err != nil {
			return err
		}
		t.Version++
		if err := h.Audit.Record(ctx, audit.Event{Action: AuditEntriesConfirmed, TargetType: "tournament", TargetID: t.ID.String(),
			Result: audit.ResultSuccess, SchoolID: schoolOf(t), Metadata: map[string]any{"confirmed": len(confirmed)}}); err != nil {
			return err
		}
		out = EntryResult{Entries: make([]map[string]any, 0, len(confirmed))}
		for _, e := range confirmed {
			out.Entries = append(out.Entries, view.Entry(e, nil, "", nil))
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, trnError(err)
}

// WithdrawTournamentEntry implements withdrawTournamentEntry (F107-AC-09): the student (a member of the entry) or the
// organizer withdraws an entry before the bracket locks (status withdrawn, the row stays — R21); once scheduled its
// bracket slot becomes a bye; after the lock → 422 INVALID_STATE_TRANSITION.
func (h Tournaments) WithdrawTournamentEntry(ctx context.Context, id, entryID uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.WithdrawTournamentEntry")
	defer func() { done(err) }()
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return problem.New(problem.Unauthenticated, "")
	}
	var tenant, me uuid.UUID
	staff := false
	switch {
	case p.UserType == authz.UserTypeStudent:
		if tenant, me, err = student(ctx); err != nil {
			return err
		}
	case p.Channel == authz.ChannelGame:
		return problem.New(problem.GameAccessStudentOnly, "")
	default:
		if p, tenant, err = organizer(ctx); err != nil {
			return err
		}
		staff = true
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		t, err := h.Repo.Tournament(ctx, tenant, id, true)
		if err != nil {
			return err
		}
		if t == nil || (staff && !Covers(p, t.School)) || (!staff && !studentVisible(t.Status)) {
			return problem.New(problem.NotFound, "")
		}
		e, err := h.Repo.Entry(ctx, tenant, id, entryID, true)
		if err != nil {
			return err
		}
		if e == nil || (!staff && !slices.Contains(e.Members, me)) {
			return problem.New(problem.NotFound, "")
		}
		if trn.Locked(t.Status) || e.Status == trn.EntryWithdrawn {
			return trn.ErrInvalidTransition
		}
		e.Status = trn.EntryWithdrawn
		if err := h.Repo.UpdateEntry(ctx, *e); err != nil {
			return err
		}
		if t.Status == trn.StatusScheduled {
			return h.withdrawFromBracket(ctx, t, map[uuid.UUID]bool{e.ID: true})
		}
		return nil
	})
	return trnError(err)
}

// CheckInTournamentEntry implements checkInTournamentEntry (F107-AC-10): a member of a confirmed entry checks in while
// the check-in window is open (check_in_opens_at to starts_at of a scheduled tournament); otherwise 422
// TOURNAMENT_CHECK_IN_CLOSED.
func (h Tournaments) CheckInTournamentEntry(ctx context.Context, id, entryID uuid.UUID) (out map[string]any, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.CheckInTournamentEntry")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return nil, err
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		t, err := h.Repo.Tournament(ctx, tenant, id, false)
		if err != nil {
			return err
		}
		if t == nil || !studentVisible(t.Status) {
			return problem.New(problem.NotFound, "")
		}
		e, err := h.Repo.Entry(ctx, tenant, id, entryID, true)
		if err != nil {
			return err
		}
		if e == nil || !slices.Contains(e.Members, me) {
			return problem.New(problem.NotFound, "")
		}
		if t.Status != trn.StatusScheduled || now.Before(t.CheckInOpensAt) || !now.Before(t.StartsAt) ||
			(e.Status != trn.EntryConfirmed && e.Status != trn.EntryCheckedIn) {
			return problem.New(problem.TournamentCheckInClosed, "")
		}
		if e.Status == trn.EntryConfirmed {
			at := now
			e.Status, e.CheckedInAt = trn.EntryCheckedIn, &at
			if err := h.Repo.UpdateEntry(ctx, *e); err != nil {
				return err
			}
			e.Version++
		}
		cards, err := h.cardsOf(ctx, tenant, e.Members)
		if err != nil {
			return err
		}
		out = view.Entry(*e, cards, "", nil)
		return nil
	})
	return out, trnError(err)
}
