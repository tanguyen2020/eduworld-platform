package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/gameconfig"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// JobConfigApply is the reason of the system platform scope of the configuration apply (consumer and job).
const JobConfigApply = "competition-config-apply"

// DefinitionMaterializer copies the leaderboard definitions in effect into competition.leaderboard_definitions (F106
// Leaderboards.Materialize).
type DefinitionMaterializer interface {
	Materialize(ctx context.Context) (int, error)
}

// ConfigApply applies the competition configurations in effect to the runtime tables (F108-AC-08): the seasons
// configuration to competition.seasons (by code, season_id kept) and the leaderboard definitions to
// competition.leaderboard_definitions. Matchmaking and Ranking Rules need no copy: the runtime reads the version in
// effect by the release timeline for every new match (matches and tournaments already started keep their pinned
// version); templates are pinned by the challenge created from them (F107). Idempotent: run by the consumer of
// competition_config.published and by the job competition-config-apply (a release whose effective_from is in the future
// is applied by the first run after it).
type ConfigApply struct {
	UoW    *entx.UnitOfWork
	Store  write.ConfigApplyStore
	Boards DefinitionMaterializer
	Now    func() time.Time
}

func (h ConfigApply) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// Apply applies the configuration type a release concerns ("" = every applied type). It returns the rows changed.
func (h ConfigApply) Apply(ctx context.Context, configType string) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.ApplyPublishedConfig")
	defer func() { done(err) }()
	if configType == "" || configType == gameconfig.TypeSeasons {
		k, err := h.applySeasons(ctx)
		if err != nil {
			return n, err
		}
		n += k
	}
	if (configType == "" || configType == gameconfig.TypeLeaderboards) && h.Boards != nil {
		k, err := h.Boards.Materialize(ctx)
		if err != nil {
			return n, err
		}
		n += k
	}
	return n, nil
}

// ApplyDue is the job competition-config-apply: every applied type.
func (h ConfigApply) ApplyDue(ctx context.Context) (int, error) { return h.Apply(ctx, "") }

func (h ConfigApply) applySeasons(ctx context.Context) (int, error) {
	ps, err := authz.SystemPlatformScope(JobConfigApply)
	if err != nil {
		return 0, err
	}
	now := h.now()
	n := 0
	err = h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		version, payload, err := h.Store.EffectivePayload(ctx, gameconfig.TypeSeasons, now)
		if err != nil || version == nil {
			return err
		}
		seasons, err := gameconfig.ParseSeasons(payload)
		if err != nil {
			return err
		}
		for _, s := range seasons {
			id, err := uuid.Parse(s.ID)
			if err != nil {
				continue
			}
			ok, err := h.Store.UpsertSeason(ctx, write.SeasonSync{ID: id, Code: s.Code, Name: s.Name, Theme: s.Theme, StartsAt: s.StartsAt,
				EndsAt: s.EndsAt}, *version, now)
			if err != nil {
				return err
			}
			if ok {
				n++
			}
		}
		return nil
	})
	return n, err
}
