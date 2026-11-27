package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	mm "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/matchmaking"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// DuelInput is the body of createDuel.
type DuelInput struct {
	OpponentID uuid.UUID
	SubjectID  uuid.UUID
	TopicID    *uuid.UUID
}

// socialScope is the refusal of a duel without saying which condition failed (F104-AC-19).
func socialScope() error { return problem.New(problem.SocialScopeViolation, "") }

// CreateDuel implements createDuel (F104-AC-19, TC-CMP-010): a friend of the same grade band, in the tenant of the
// session, without block either way; both may play (H5). Pending 60 s, duel.sent + audit. Duels count in mm_ticket_rate
// (no separate parameter — Clarifications 2026-11-21).
func (h Matchmaking) CreateDuel(ctx context.Context, in DuelInput) (res map[string]any, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.CreateDuel")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return nil, err
	}
	if in.OpponentID == uuid.Nil || in.SubjectID == uuid.Nil {
		return nil, problem.Invalid("opponent_id", problem.SubRequired, "opponent_id and subject_id are required")
	}
	if in.OpponentID == me {
		return nil, socialScope()
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		now := h.now()
		cfg, err := h.config(ctx, now)
		if err != nil {
			return err
		}
		ps, err := h.Repo.Players(ctx, tenant, []uuid.UUID{me, in.OpponentID}, now)
		if err != nil {
			return err
		}
		mine, ok := ps[me]
		if !ok {
			return problem.New(problem.Unavailable, "student not projected yet").Retry()
		}
		if err := mm.Eligible(mine); err != nil {
			return err
		}
		opp, ok := ps[in.OpponentID]
		if !ok || mm.Eligible(opp) != nil || mm.BandOf(opp.Grade) != mm.BandOf(mine.Grade) {
			return socialScope()
		}
		friends, err := h.Directory.Friends(ctx, tenant, me, in.OpponentID)
		if err != nil {
			return err
		}
		blocked, err := h.Directory.Blocked(ctx, tenant, me, []uuid.UUID{in.OpponentID})
		if err != nil {
			return err
		}
		if !friends || blocked {
			return socialScope()
		}
		if ok, topicOK := subjectFor(cfg, mine.Grade, in.SubjectID, in.TopicID); !ok {
			return mm.ValidationError{Field: "subject_id", Message: "subject not available for this grade"}
		} else if !topicOK {
			return mm.ValidationError{Field: "topic_id", Message: "topic not available for this grade"}
		}
		times, err := h.Repo.DuelsSince(ctx, tenant, me, now.Add(-time.Hour))
		if err != nil {
			return err
		}
		if limit := cfg.TicketRate.PerHour; limit > 0 && len(times) >= limit {
			return rateLimited(retryAt(times[len(times)-limit], time.Hour, now))
		}
		d := mm.NewDuel(ticketID(ctx, me, "duel"), tenant, mine.SchoolID, me, in.OpponentID, in.SubjectID, in.TopicID, now)
		if err := h.Repo.CreateDuel(ctx, d); err != nil {
			return err
		}
		if err := h.duelEvent(ctx, "duel.sent", d, now); err != nil {
			return err
		}
		if err := h.duelAudit(ctx, AuditDuelSent, d); err != nil {
			return err
		}
		res = view.Duel(d)
		if err := idempotency.SetResult(ctx, res); err != nil {
			return err
		}
		cards, err := h.Directory.Cards(ctx, tenant, []uuid.UUID{me})
		if err != nil {
			return err
		}
		challenger, ok := cards[me]
		if !ok {
			challenger = port.Card{UserID: me, Level: 1}
		}
		return h.later(ctx, func(ctx context.Context) {
			h.send(ctx, []uuid.UUID{d.OpponentID}, rtDuelReceived, view.DuelReceived(d, challenger))
		})
	})
	return res, mmError(err)
}

func subjectFor(cfg mm.Config, grade int, subject uuid.UUID, topic *uuid.UUID) (bool, bool) {
	typ := mm.TypeSkill
	if topic != nil {
		typ = mm.TypeTopic
	}
	err := cfg.CheckPlayers(mm.Request{Mode: mm.ModeBattle1v1, Type: typ, SubjectID: subject, TopicID: topic},
		[]mm.Player{{Grade: grade, Active: true}})
	var ve mm.ValidationError
	if errors.As(err, &ve) {
		return ve.Field != "subject_id", false
	}
	return true, true
}

// duelOf loads a duel of the tenant for its opponent; any other student gets 404 (F104-AC-24).
func (h Matchmaking) duelOf(ctx context.Context, tenant, me, id uuid.UUID) (*mm.Duel, error) {
	d, err := h.Repo.Duel(ctx, tenant, id, true)
	if err != nil {
		return nil, err
	}
	if d == nil || d.OpponentID != me {
		return nil, problem.New(problem.NotFound, "")
	}
	return d, nil
}

// AcceptDuel implements acceptDuel (F104-AC-20): before the 60 s, the opponent starts an unranked friend match (no
// rating, no Competition Points) created idempotently from the duel; blocks and H5 are checked again.
func (h Matchmaking) AcceptDuel(ctx context.Context, id uuid.UUID) (res map[string]any, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.AcceptDuel")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return nil, err
	}
	var d *mm.Duel
	var matchID uuid.UUID
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		now := h.now()
		if d, err = h.duelOf(ctx, tenant, me, id); err != nil {
			return err
		}
		if !d.Open(now) {
			return mm.ErrDuelGone
		}
		cfg, err := h.config(ctx, now)
		if err != nil {
			return err
		}
		ps, err := h.Repo.Players(ctx, tenant, []uuid.UUID{d.ChallengerID, d.OpponentID}, now)
		if err != nil {
			return err
		}
		blocked, err := h.Directory.Blocked(ctx, tenant, d.ChallengerID, []uuid.UUID{d.OpponentID})
		if err != nil {
			return err
		}
		ch, ok1 := ps[d.ChallengerID]
		op, ok2 := ps[d.OpponentID]
		if blocked || !ok1 || !ok2 || mm.Eligible(ch) != nil || mm.Eligible(op) != nil {
			return mm.ErrDuelGone
		}
		matchID = uuid.NewSHA1(matchmakingNamespace, []byte("duel/"+d.ID.String()))
		if err := d.Accept(me, matchID, now); err != nil {
			return err
		}
		scope := mm.ScopeSchool
		if ch.SchoolID != op.SchoolID {
			scope = mm.ScopeTenant
		}
		duelID := d.ID
		m := write.NewMatch{ID: matchID, Mode: mm.ModeBattle1v1, Type: mm.TypeFriend, SubjectID: d.SubjectID, TopicID: d.TopicID,
			Grade: min(ch.Grade, op.Grade), Ranked: false, ScopeKind: scope, TenantIDs: []uuid.UUID{tenant}, DuelID: &duelID,
			RulesetVersionID: cfg.VersionID, At: now, Participants: []write.MatchParticipant{
				{StudentID: ch.ID, TenantID: tenant, SchoolID: ch.SchoolID, TeamNo: 0},
				{StudentID: op.ID, TenantID: tenant, SchoolID: op.SchoolID, TeamNo: 1}}}
		if err := h.createMatch(ctx, m); err != nil {
			return err
		}
		if err := h.Repo.SaveDuel(ctx, *d); err != nil {
			return err
		}
		if err := h.duelEvent(ctx, "duel.accepted", *d, now); err != nil {
			return err
		}
		if err := h.duelAudit(ctx, AuditDuelAccepted, *d); err != nil {
			return err
		}
		res = view.Duel(*d)
		if err := idempotency.SetResult(ctx, res); err != nil {
			return err
		}
		to := []uuid.UUID{d.ChallengerID, d.OpponentID}
		return h.later(ctx, func(ctx context.Context) {
			h.send(ctx, to, rtMatchAssigned, view.MatchAssigned(matchID, mm.ModeBattle1v1))
		})
	})
	return res, mmError(err)
}

// DeclineDuel implements declineDuel (F104-AC-20): the opponent refuses; duel.declined + audit.
func (h Matchmaking) DeclineDuel(ctx context.Context, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.DeclineDuel")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		now := h.now()
		d, err := h.duelOf(ctx, tenant, me, id)
		if err != nil {
			return err
		}
		if err := d.Decline(me, now); err != nil {
			return err
		}
		if err := h.Repo.SaveDuel(ctx, *d); err != nil {
			return err
		}
		if err := h.duelEvent(ctx, "duel.declined", *d, now); err != nil {
			return err
		}
		return h.duelAudit(ctx, AuditDuelDeclined, *d)
	})
	return mmError(err)
}

// ExpireDuels is one run of the job duel-expiry (F104-AC-20): pending duels past 60 s become expired with
// duel.expired and audit, each in its tenant transaction.
func (h Matchmaking) ExpireDuels(ctx context.Context, repo write.DuelExpiryRepository, batch int) (int, error) {
	ps, err := authz.SystemPlatformScope(DuelExpiryJob)
	if err != nil {
		return 0, err
	}
	now := h.now()
	var refs []write.DuelRef
	if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		var err error
		refs, err = repo.ExpiredDuels(ctx, now, batch)
		return err
	}); err != nil {
		return 0, err
	}
	n := 0
	for _, r := range refs {
		sc, err := authz.EventScope(r.TenantID)
		if err != nil {
			return n, err
		}
		err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
			d, err := h.Repo.Duel(ctx, r.TenantID, r.DuelID, true)
			if err != nil || d == nil || !d.Expire(now) {
				return err
			}
			if err := h.Repo.SaveDuel(ctx, *d); err != nil {
				return err
			}
			if err := h.duelEvent(ctx, "duel.expired", *d, now); err != nil {
				return err
			}
			n++
			return h.duelAudit(ctx, AuditDuelExpired, *d)
		})
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func (h Matchmaking) duelEvent(ctx context.Context, typ string, d mm.Duel, at time.Time) error {
	payload := map[string]any{"duel_id": d.ID, "tenant_id": d.TenantID, "challenger_id": d.ChallengerID, "opponent_id": d.OpponentID,
		"subject_id": d.SubjectID, "topic_id": d.TopicID}
	switch typ {
	case "duel.sent":
		payload["expires_at"] = d.ExpiresAt.UTC().Format(time.RFC3339Nano)
	case "duel.accepted":
		payload["match_id"] = d.MatchID
	}
	t := d.TenantID
	return h.Events.Publish(ctx, write.Event{Type: typ, SubjectType: "duel", SubjectID: d.ID.String(), PartitionKey: d.ID.String(),
		TenantID: &t, OccurredAt: at, Payload: payload})
}

func (h Matchmaking) duelAudit(ctx context.Context, action string, d mm.Duel) error {
	return h.Audit.Record(ctx, audit.Event{Action: action, TargetType: "duel", TargetID: d.ID.String(), Result: audit.ResultSuccess,
		SchoolID: d.SchoolID.String(), Metadata: map[string]any{"challenger_id": d.ChallengerID.String(), "opponent_id": d.OpponentID.String(),
			"status": d.Status}})
}
