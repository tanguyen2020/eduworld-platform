package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/ranking"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Seasons are the season jobs (worker jobs, platform seasons — G table): the scheduler (F106-AC-22, AC-23) and the
// weekly RD inflation of inactive ratings (F106-AC-06).
type Seasons struct {
	UoW     *entx.UnitOfWork
	Repo    write.SeasonRepository
	Ranking write.RankingRepository
	Events  write.EventPublisher
	Audit   write.Auditor
	Now     func() time.Time
}

func (h Seasons) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// Tick moves the seasons on (F106-AC-22): the active season past its end — or overtaken by a scheduled season whose start
// came — ends (season_results first, then ended + season.ended once); a scheduled season whose start came starts
// (the soft-reset ratings first, then active + season.started once). Idempotent by season_id: a transition happens only
// from the expected status.
func (h Seasons) Tick(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.SeasonTick")
	defer func() { done(err) }()
	now := h.now()
	ps, err := authz.SystemPlatformScope(JobSeasonScheduler)
	if err != nil {
		return 0, err
	}
	var seasons []write.SeasonRow
	if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		var err error
		seasons, err = h.Repo.Seasons(ctx)
		return err
	}); err != nil {
		return 0, err
	}
	var active, next *write.SeasonRow
	for i := range seasons {
		s := &seasons[i]
		switch {
		case s.Status == "active":
			active = s
		case s.Status == "scheduled" && !s.StartsAt.After(now) && (next == nil || s.StartsAt.Before(next.StartsAt)):
			next = s
		}
	}
	if active != nil && (!active.EndsAt.After(now) || next != nil) {
		if err := h.end(ctx, ps, *active, now); err != nil {
			return n, err
		}
		n++
		active.Status = "ended" // the previous season of the soft reset below
		active = nil
	}
	if next != nil && active == nil {
		if err := h.start(ctx, ps, *next, seasons, now); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// end stores the season results of every tenant, then marks the season ended and publishes season.ended (F106-AC-23:
// the results are never deleted; the history of the season stays readable).
func (h Seasons) end(ctx context.Context, ps tenancy.Scope, s write.SeasonRow, now time.Time) error {
	var tenants []uuid.UUID
	if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		var err error
		tenants, err = h.Repo.TenantsWithRatings(ctx, s.ID)
		return err
	}); err != nil {
		return err
	}
	results := 0
	for _, t := range tenants {
		sc, err := authz.EventScope(t)
		if err != nil {
			return err
		}
		if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
			k, err := h.Repo.WriteResults(ctx, t, s.ID)
			results += k
			return err
		}); err != nil {
			return err
		}
	}
	return h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		ok, err := h.Repo.SetStatus(ctx, s.ID, "active", "ended", now)
		if err != nil || !ok {
			return err
		}
		if err := h.Events.Publish(ctx, write.Event{Type: EventSeasonEnded, SubjectType: "season", SubjectID: s.ID.String(),
			PartitionKey: s.ID.String(), OccurredAt: now, Platform: true, Payload: map[string]any{"season_id": s.ID, "code": s.Code, "ended_at": now,
				"results_count": results, "source_version": now.UnixMicro()}}); err != nil {
			return err
		}
		return h.Audit.Record(ctx, audit.Event{Action: AuditSeasonEnded, TargetType: "season", TargetID: s.ID.String(), Result: audit.ResultSuccess,
			Metadata: map[string]any{"code": s.Code, "results": results}})
	})
}

// start creates the ratings of the new season by season_soft_reset from the previous season (volatility kept;
// Competition Points start from 0: a new season has no ledger), then marks it active and publishes season.started.
func (h Seasons) start(ctx context.Context, ps tenancy.Scope, s write.SeasonRow, all []write.SeasonRow, now time.Time) error {
	var prev *write.SeasonRow
	for i := range all {
		p := all[i]
		if p.ID != s.ID && p.Status == "ended" && p.StartsAt.Before(s.StartsAt) && (prev == nil || p.StartsAt.After(prev.StartsAt)) {
			prev = &all[i]
		}
	}
	if prev != nil {
		var tenants []uuid.UUID
		var rules ranking.Rules
		var rv uuid.UUID
		if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
			var err error
			if rules, rv, err = h.Ranking.Rules(ctx, now); err != nil {
				return err
			}
			tenants, err = h.Repo.TenantsWithRatings(ctx, prev.ID)
			return err
		}); err != nil {
			return err
		}
		for _, t := range tenants {
			sc, err := authz.EventScope(t)
			if err != nil {
				return err
			}
			if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
				_, err := h.Repo.SoftReset(ctx, t, prev.ID, s.ID, rv, rules.SeasonReset)
				return err
			}); err != nil {
				return err
			}
		}
	}
	return h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		ok, err := h.Repo.SetStatus(ctx, s.ID, "scheduled", "active", now)
		if err != nil || !ok {
			return err
		}
		var prevID *uuid.UUID
		if prev != nil {
			prevID = &prev.ID
		}
		if err := h.Events.Publish(ctx, write.Event{Type: EventSeasonStarted, SubjectType: "season", SubjectID: s.ID.String(),
			PartitionKey: s.ID.String(), OccurredAt: now, Platform: true, Payload: map[string]any{"season_id": s.ID, "code": s.Code, "name": s.Name,
				"theme_code": s.Theme, "starts_at": s.StartsAt, "ends_at": s.EndsAt, "previous_season_id": prevID,
				"source_version": now.UnixMicro()}}); err != nil {
			return err
		}
		return h.Audit.Record(ctx, audit.Event{Action: AuditSeasonStarted, TargetType: "season", TargetID: s.ID.String(), Result: audit.ResultSuccess,
			Metadata: map[string]any{"code": s.Code}})
	})
}

// InflateRD is the weekly job rd-inflation (F106-AC-06): the "no games" step of Glicko-2 for every rating of the active
// season not played for a week — RD up to RD max, never the rating (no rank decay, COMPETITION_SYSTEM §12).
func (h Seasons) InflateRD(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.InflateInactiveRD")
	defer func() { done(err) }()
	now := h.now()
	ps, err := authz.SystemPlatformScope(JobRDInflation)
	if err != nil {
		return 0, err
	}
	var tenants []uuid.UUID
	var rules ranking.Rules
	var season *write.SeasonRow
	if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		var err error
		if rules, _, err = h.Ranking.Rules(ctx, now); err != nil {
			return err
		}
		if season, err = h.Ranking.SeasonAt(ctx, now); err != nil || season == nil {
			return err
		}
		tenants, err = h.Repo.TenantsWithRatings(ctx, season.ID)
		return err
	}); err != nil || season == nil {
		return 0, err
	}
	step := func(r ranking.Rating) ranking.Rating { return ranking.NoGames(rules.Glicko2, r) }
	for _, t := range tenants {
		sc, err := authz.EventScope(t)
		if err != nil {
			return n, err
		}
		if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
			k, err := h.Repo.InflateInactive(ctx, t, season.ID, now.Add(-7*24*time.Hour), step)
			n += k
			return err
		}); err != nil {
			return n, err
		}
	}
	return n, nil
}
