// Package game is the composition entry of module game (ADR-0074 §1, BACKEND_DESIGN §3.3).
// cmd/<binary> may only call Register, Facade and Wire; everything else lives in internal/.
package game

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/cache"
	gameconfig "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/modules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/worldsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/security"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/facade"
	gamehttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/job"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/natscmd"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/migrations"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"

	_ "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/ent/runtime" // ent mixin hooks and interceptors (R27)
)

// Name is the module name used for its database (eduworld_game), env prefix and checks.
const Name = "game"

// Facade is the part of the game facade (api.StrictServerInterface, specs/openapi/modules/game.internal.v1.yaml)
// implemented in this release: the student lifecycle operations of the identity sagas (F016) and mergeUserData (F070 T039). The other
// platform-ops operations (assignments, trash, clone) are not implemented by module game yet (OQ F045-3): the
// identity gateway answers 503 UNAVAILABLE for them, never a silent success.
type Facade interface {
	CleanupStudentData(ctx context.Context, req api.CleanupStudentDataRequestObject) (api.CleanupStudentDataResponseObject, error)
	RestoreStudentData(ctx context.Context, req api.RestoreStudentDataRequestObject) (api.RestoreStudentDataResponseObject, error)
	AnonymizeUserData(ctx context.Context, req api.AnonymizeUserDataRequestObject) (api.AnonymizeUserDataResponseObject, error)
	MergeUserData(ctx context.Context, req api.MergeUserDataRequestObject) (api.MergeUserDataResponseObject, error)
}

// Imports are the facades module game calls: getHubPosition of module competition (F071 T049, interaction distance of a
// discovery); nil = presence only. Everything else is projections and events (ADR-0037).
type Imports struct {
	Competition modules.CompetitionHub
}

// Module is the registered module.
type Module struct {
	facade Facade
	hub    *modules.LateHub
}

// Facade returns the in-process facade of module game (nil in a binary that does not load its use cases).
func (m *Module) Facade() Facade { return m.facade }

// Wire connects the facades of other modules (in process, pkg/modcall).
func (m *Module) Wire(in Imports) {
	if m.hub != nil {
		m.hub.Set(in.Competition)
	}
}

// Register wires the module into the running binary (OQ F014-7, F010-2): health checks everywhere; the F014 routes in
// core-api; the outbox relay and the consumers game.projection_identity, game.session_end_on_revoke,
// game.platform_settings (platform_curfew), game.manifest_on_learning_content and the F071 consumers
// game.manifest_on_game_content, game.unlock_on_events, game.mastery_projection in worker events; the F071 world routes; the jobs
// session-timeout and curfew-session-closer in worker jobs; createRealtimeTicket and the NATS command
// ew.cmd.game.session.heartbeat of the realtime gateway in core-api (F090).
func Register(r *module.Registry) (*Module, error) {
	pool, err := r.Database(Name)
	if err != nil {
		return nil, fmt.Errorf("game: open database: %w", err)
	}
	r.Health().Register("db:"+Name, health.Readiness, health.PostgresCheck(pool))
	r.Health().Register("atlas:"+Name, health.Readiness, health.AtlasRevisionCheck(pool, migrations.Latest()))
	if !r.ServesHTTP() && !r.RunsEvents() && !r.RunsJobs() {
		return &Module{hub: &modules.LateHub{}}, nil
	}
	core, err := r.Core(Name)
	if err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	cfg, err := gameconfig.SessionFrom(r.Config())
	if err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	cursors, err := r.Cursors()
	if err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	vk, err := r.Valkey()
	if err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	observability.Register(r.Metrics())
	observability.RegisterManifest(r.Metrics())
	observability.RegisterProfile(r.Metrics())
	observability.RegisterWorld(r.Metrics())
	profileCfg, err := gameconfig.ProfileFrom(r.Config())
	if err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	worldCfg, err := gameconfig.WorldFrom(r.Config(), cfg.UseCase.HeartbeatTimeout)
	if err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	presence := cache.PresenceStore{C: vk}
	hub := &modules.LateHub{}
	sessions := command.Sessions{UoW: core.UoW, AfterCommit: entx.AfterCommit, Sessions: write.SessionRepo{}, Logins: write.LoginEventRepo{},
		Profiles: write.ProfileRepo{}, Directory: write.Directory{}, Events: write.Publisher{W: core.Writer}, Audit: write.Auditor{R: core.Recorder},
		Realtime: realtime.ValkeyPublisher{C: vk}, Metrics: observability.SessionMetrics{}, Presence: presence, Config: cfg.UseCase}
	// F071 world: hub positions through the competition facade when wired (Module.Wire), presence only otherwise.
	worldUC := command.World{UoW: core.UoW, AfterCommit: entx.AfterCommit, Content: worldsql.Content{}, Store: write.WorldRepo{},
		Sessions: write.SessionRepo{}, Manifests: write.ManifestRepo{}, Events: write.Publisher{W: core.Writer}, Presence: presence,
		Rate: cache.RateLimiter{C: vk}, Realtime: realtime.WorldPublisher{C: vk}, Hub: hub, Metrics: observability.WorldMetrics{}, Config: worldCfg}
	m := &Module{hub: hub, facade: facade.Facade{Lifecycle: command.StudentLifecycle{UoW: core.UoW, Sessions: write.SessionRepo{},
		Profiles: write.ProfileRepo{}, Presets: write.PresetRepo{}}}}
	options := cache.NicknameOptionStore{C: vk}
	profiles := command.Profiles{UoW: core.UoW, AfterCommit: entx.AfterCommit, Repo: write.ProfileRepo{}, Catalog: write.CatalogRepo{},
		Owned: write.OwnershipRepo{}, Presets: write.PresetRepo{}, Directory: write.Directory{}, Schools: write.Directory{},
		Options: options, Events: write.Publisher{W: core.Writer}, Metrics: observability.ProfileMetrics{}, Config: profileCfg}

	h := gamehttp.Handlers{Sessions: sessions, ClientConfig: cfg.ClientConfig,
		Manifest:    query.GetContentManifest{Read: core.ReadSession, Manifests: read.Manifests{}, MinClientVersion: cfg.UseCase.MinClientVersion},
		LoginEvents: query.LoginEvents{Read: core.ReadSession, Events: read.LoginEvents{}, Students: read.Students{}, Cursors: cursors},
		Tickets:     command.RealtimeTickets{Sessions: sessions, Store: realtime.TicketStore{C: vk}, Config: cfg.Tickets},
		Profiles:    profiles, ProfileQueries: query.Profile{Read: core.ReadSession, Profiles: read.Profiles{}, Catalog: read.Catalogs{},
			Options: options, TTL: profileCfg.SuggestionTTL},
		World: worldUC, WorldQueries: query.World{Read: core.ReadSession, Content: worldsql.Content{}, Manifests: read.Manifests{}}}
	if len(cfg.IPHashKey) > 0 {
		h.IPHasher = security.NewIPHasher(cfg.IPHashKey)
	}
	r.HTTP(Name, func(s *httpx.Server) error { return gamehttp.Register(s, h) })
	// NATS command of the realtime gateway (F090-AC-12): served by core-api, queue group "game".
	if nc := r.NATS(); nc != nil && r.ServesHTTP() {
		r.Go("game.cmd.session_heartbeat", func(ctx context.Context) error {
			return rt.ServeCommand(ctx, nc, rt.SessionHeartbeatModule, rt.SessionHeartbeatType, natscmd.SessionHeartbeat(sessions), 0)
		})
	}

	if err := r.Relay(Name); err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	r.Consumer("game.projection_identity", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.IdentityProjectionOptions(consumer.IdentityProjection{Upsert: command.UpsertProjection{Refs: write.RefRepo{}}},
			core.UoW, core.App, js)
	})
	r.Consumer("game.session_end_on_revoke", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.SessionEndOnRevokeOptions(consumer.SessionEndOnRevoke{Sessions: sessions}, core.UoW, core.App, js)
	})
	r.Consumer("game.platform_settings", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.PlatformSettingsOptions(consumer.PlatformSettings{Settings: command.PlatformSettings{Refs: write.RefPlatformSettings{}}},
			core.UoW, core.App, js)
	})
	r.Consumer("game.manifest_on_learning_content", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.ManifestOnLearningContentOptions(consumer.ManifestOnLearningContent{Manifests: command.Manifests{UoW: core.UoW,
			Store: write.ManifestRepo{}, Events: write.Publisher{W: core.Writer}, Metrics: observability.ManifestMetrics{}}}, core.UoW, core.App, js)
	})
	r.Consumer("game.manifest_on_game_content", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.ManifestOnGameContentOptions(consumer.ManifestOnGameContent{Manifests: command.GameManifest{UoW: core.UoW,
			Store: write.ManifestRepo{}, Tenants: write.ManifestRepo{}, Content: worldsql.Content{}, Events: write.Publisher{W: core.Writer},
			Metrics: observability.ManifestMetrics{}, BuildSeconds: observability.ManifestBuild}}, core.UoW, core.App, js)
	})
	r.Consumer("game.unlock_on_events", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.UnlockOnEventsOptions(consumer.UnlockOnEvents{World: worldUC}, core.UoW, core.App, js)
	})
	r.Consumer("game.mastery_projection", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.MasteryProjectionOptions(consumer.MasteryProjection{World: worldUC, Mastery: write.MasteryRepo{}}, core.UoW, core.App, js)
	})
	r.Job(Name, job.SessionTimeout, cfg.TimeoutScan, module.JobFunc(job.SessionTimeoutJob(sessions)))
	r.Job(Name, job.CurfewSessionCloser, job.CurfewCloserInterval, module.JobFunc(job.CurfewSessionCloserJob(sessions)))
	return m, nil
}
