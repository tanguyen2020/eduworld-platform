package query

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Matchmaking are the read use cases of matchmaking (F104): the ticket of the caller (Valkey) and the competition
// modes available to its grade (read model + published configuration).
type Matchmaking struct {
	Read   read.ReadSession
	Reader read.MatchmakingReader
	Store  port.MatchmakingStore
	Now    func() time.Time
}

func (q Matchmaking) now() time.Time {
	if q.Now != nil {
		return q.Now().UTC()
	}
	return time.Now().UTC()
}

// GetTicket implements getMatchmakingTicket (F104-AC-24): only a student of the ticket in the session tenant; a ticket
// lost with Valkey is 404 (the client creates a new one).
func (q Matchmaking) GetTicket(ctx context.Context, id uuid.UUID) (res view.Ticket, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.GetMatchmakingTicket")
	defer func() { done(err) }()
	tenant, me, err := caller(ctx)
	if err != nil {
		return res, err
	}
	t, err := q.Store.Ticket(ctx, id)
	if err != nil {
		return res, err
	}
	if t == nil || t.TenantID != tenant || !t.Has(me) {
		return res, problem.New(problem.NotFound, "")
	}
	now := q.now()
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		cfg, err := q.Reader.ActiveConfig(ctx, now)
		if err != nil {
			return err
		}
		res = view.NewTicket(*t, cfg, now)
		return nil
	})
	return res, err
}

// ListCompetitionModes implements listCompetitionModes (F104-AC-22): the queueable modes of the configuration in
// effect whose minimum grade the caller reaches, with their matchmaking types and the subjects of its grade.
func (q Matchmaking) ListCompetitionModes(ctx context.Context) (res []map[string]any, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.ListCompetitionModes")
	defer func() { done(err) }()
	tenant, me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	now := q.now()
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		cfg, err := q.Reader.ActiveConfig(ctx, now)
		if err != nil {
			return err
		}
		ps, err := q.Reader.Players(ctx, tenant, []uuid.UUID{me}, now)
		if err != nil {
			return err
		}
		p, ok := ps[me]
		if !ok {
			return problem.New(problem.Unavailable, "student not projected yet").Retry()
		}
		res = []map[string]any{}
		if p.Grade < 1 { // no grade projected: no queue can be chosen
			return nil
		}
		subjects := cfg.SubjectsFor(p.Grade)
		for _, m := range cfg.Modes {
			if p.Grade < m.MinGrade {
				continue
			}
			res = append(res, view.CompetitionMode(m, len(m.RankedTypes) > 0, subjects))
		}
		return nil
	})
	return res, err
}
