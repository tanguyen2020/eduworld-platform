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
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/economysql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/progressionsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/questsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/worldsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/realtime"
	rewardinfra "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/reward"
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
	// Learning is createAttempt of the learning facade (F072: learning steps of the quests); nil = startQuestActivity 503.
	Learning modules.LearningFacade
}

// Module is the registered module.
type Module struct {
	facade   Facade
	hub      *modules.LateHub
	learning *modules.LateLearning
}

// Facade returns the in-process facade of module game (nil in a binary that does not load its use cases).
func (m *Module) Facade() Facade { return m.facade }

// Wire connects the facades of other modules (in process, pkg/modcall).
func (m *Module) Wire(in Imports) {
	if m.hub != nil {
		m.hub.Set(in.Competition)
	}
	if m.learning != nil {
		m.learning.Set(in.Learning)
	}
}

// Register wires the module into the running binary (OQ F014-7, F010-2): health checks everywhere; the F014 routes in
// core-api; the outbox relay and the consumers game.projection_identity, game.session_end_on_revoke,
// game.platform_settings (platform_curfew), game.manifest_on_learning_content and the F071 consumers
// game.manifest_on_game_content, game.unlock_on_events, game.mastery_projection and the F072 consumers
// game.quest_on_activity_graded, game.quest_on_discovery_found, game.periodic_quests_on_session_started in worker events;
// the F071 world and F072 quest routes; the jobs session-timeout, curfew-session-closer, quest-period-expiry and
// quest-failure-timer in worker jobs; the F074 consumers game.reward_engine, game.reward_engine_game,
// game.grade_transition, the progression routes and the jobs curve-rollout, xp-reconcile; the F075 economy routes, the
// consumers game.achievement_evaluator (game, learning, competition streams) and game.collection_on_mastery and the job
// wallet-reconcile; createRealtimeTicket and the NATS command
// ew.cmd.game.session.heartbeat of the realtime gateway in core-api (F090).
func Register(r *module.Registry) (*Module, error) {
	pool, err := r.Database(Name)
	if err != nil {
		return nil, fmt.Errorf("game: open database: %w", err)
	}
	r.Health().Register("db:"+Name, health.Readiness, health.PostgresCheck(pool))
	r.Health().Register("atlas:"+Name, health.Readiness, health.AtlasRevisionCheck(pool, migrations.Latest()))
	if !r.ServesHTTP() && !r.RunsEvents() && !r.RunsJobs() {
		return &Module{hub: &modules.LateHub{}, learning: &modules.LateLearning{}}, nil
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
	observability.RegisterQuest(r.Metrics())
	observability.RegisterProgression(r.Metrics())
	observability.RegisterEconomy(r.Metrics())
	profileCfg, err := gameconfig.ProfileFrom(r.Config())
	if err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	worldCfg, err := gameconfig.WorldFrom(r.Config(), cfg.UseCase.HeartbeatTimeout)
	if err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	questCfg, err := gameconfig.QuestFrom(r.Config())
	if err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	policy, err := gameconfig.ProgressionFrom(r.Config())
	if err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	ecoPolicy, err := gameconfig.EconomyFrom(r.Config())
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
	// F072 quests: createAttempt of module learning when wired (Module.Wire), 503 otherwise.
	learningLate := &modules.LateLearning{}
	questUC := command.Quests{UoW: core.UoW, AfterCommit: entx.AfterCommit, Content: questsql.Content{}, World: worldsql.Content{},
		Store: write.QuestRepo{}, Sessions: write.SessionRepo{}, Events: write.Publisher{W: core.Writer}, Audit: write.Auditor{R: core.Recorder},
		Presence: presence, Learning: learningLate, Realtime: realtime.QuestPublisher{C: vk}, Metrics: observability.QuestMetrics{}, Config: questCfg}
	// F074 progression: the Reward Engine, level-ups, curve rollout, grade transition, XP reconciliation, ledger merge.
	progressionUC := command.Progression{UoW: core.UoW, AfterCommit: entx.AfterCommit, Store: write.ProgressionRepo{}, Scan: write.ProgressionRepo{},
		Profiles: write.ProfileRepo{}, Students: worldsql.Content{}, Rules: progressionsql.Content{}, Curves: progressionsql.Content{},
		Events: write.Publisher{W: core.Writer}, Audit: write.Auditor{R: core.Recorder},
		Realtime: realtime.ProgressionPublisher{C: vk}, Quests: questUC, Metrics: observability.ProgressionMetrics{}, Policy: policy}
	// F075 economy: the appliers currency, item, badge, collection_item join the registry of the Reward Engine; the
	// economy grants its own rewards (tiers, completions, cards) through the same registry.
	economyUC := &command.Economy{UoW: core.UoW, AfterCommit: entx.AfterCommit, Store: write.EconomyRepo{}, Grants: write.ProgressionRepo{},
		Achievements: write.AchievementRepo{}, Adjustments: write.AdjustmentRepo{}, Scan: write.EconomyRepo{}, Content: economysql.Content{},
		Students: worldsql.Content{}, LiveOps: economysql.NoLiveOps{}, Events: write.Publisher{W: core.Writer}, Audit: write.Auditor{R: core.Recorder},
		Metrics: observability.EconomyMetrics{}, Policy: ecoPolicy}
	appliers := rewardinfra.WithEconomy(economyUC)
	economyUC.Registry = appliers
	progressionUC.Appliers = appliers
	economyUC.XP = progressionUC
	m := &Module{hub: hub, learning: learningLate, facade: facade.Facade{Lifecycle: command.StudentLifecycle{UoW: core.UoW, Sessions: write.SessionRepo{},
		Profiles: write.ProfileRepo{}, Presets: write.PresetRepo{}, XP: progressionUC}}}
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
		World: worldUC, WorldQueries: query.World{Read: core.ReadSession, Content: worldsql.Content{}, Manifests: read.Manifests{}},
		Quests: questUC, QuestQueries: query.Quests{Read: core.ReadSession, Content: questsql.Content{}, World: worldsql.Content{}, Cursors: cursors,
			Reset: questCfg.Reset},
		ProgressionQueries: query.Progression{Read: core.ReadSession, Curves: progressionsql.Content{}, Data: read.Progression{}, Cursors: cursors,
			DailyCap: policy.DailyCap},
		EconomyQueries: query.Economy{Read: core.ReadSession, Data: read.Economy{}, Cursors: cursors}, EconomyCommands: economyUC}
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
			Store: write.ManifestRepo{}, Tenants: write.ManifestRepo{}, Content: worldsql.Content{}, Quests: questsql.Content{}, Events: write.Publisher{W: core.Writer},
			Metrics: observability.ManifestMetrics{}, BuildSeconds: observability.ManifestBuild}}, core.UoW, core.App, js)
	})
	r.Consumer("game.unlock_on_events", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.UnlockOnEventsOptions(consumer.UnlockOnEvents{World: worldUC}, core.UoW, core.App, js)
	})
	r.Consumer("game.mastery_projection", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.MasteryProjectionOptions(consumer.MasteryProjection{World: worldUC, Mastery: write.MasteryRepo{}}, core.UoW, core.App, js)
	})
	r.Consumer("game.quest_on_activity_graded", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.QuestOnActivityGradedOptions(consumer.QuestOnActivityGraded{Quests: questUC}, core.UoW, core.App, js)
	})
	r.Consumer("game.quest_on_discovery_found", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.QuestOnDiscoveryFoundOptions(consumer.QuestOnDiscoveryFound{Quests: questUC}, core.UoW, core.App, js)
	})
	r.Consumer("game.periodic_quests_on_session_started", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.PeriodicQuestsOptions(consumer.PeriodicQuestsOnSessionStarted{Quests: questUC}, core.UoW, core.App, js)
	})
	r.Consumer("game.reward_engine", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.RewardEngineOptions(consumer.RewardEngine{Awards: progressionUC}, core.UoW, core.App, js)
	})
	r.Consumer("game.reward_engine_game", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.RewardEngineGameOptions(consumer.RewardEngine{Awards: progressionUC}, core.UoW, core.App, js)
	})
	r.Consumer("game.grade_transition", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.GradeTransitionOptions(consumer.GradeTransition{Progression: progressionUC}, core.UoW, core.App, js)
	})
	r.Consumer("game.achievement_evaluator", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.AchievementEvaluatorOptions(consumer.AchievementConsumer{Achievements: economyUC}, core.UoW, core.App, js)
	})
	r.Consumer("game.achievement_evaluator_learning", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.AchievementEvaluatorLearningOptions(consumer.AchievementConsumer{Achievements: economyUC}, core.UoW, core.App, js)
	})
	r.Consumer("game.achievement_evaluator_competition", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.AchievementEvaluatorCompetitionOptions(consumer.AchievementConsumer{Achievements: economyUC}, core.UoW, core.App, js)
	})
	r.Consumer("game.collection_on_mastery", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.CollectionOnMasteryOptions(consumer.CollectionOnMastery{Cards: economyUC}, core.UoW, core.App, js)
	})
	r.Job(Name, job.WalletReconcile, gameconfig.WalletReconcileInterval, module.JobFunc(job.WalletReconcileJob(economyUC)))
	r.Job(Name, job.CurveRollout, gameconfig.CurveRolloutInterval, module.JobFunc(job.CurveRolloutJob(progressionUC)))
	r.Job(Name, job.XPReconcile, gameconfig.XPReconcileInterval, module.JobFunc(job.XPReconcileJob(progressionUC)))
	r.Job(Name, job.QuestPeriodExpiry, questCfg.JobInterval, module.JobFunc(job.QuestPeriodExpiryJob(questUC)))
	r.Job(Name, job.QuestFailureTimer, questCfg.JobInterval, module.JobFunc(job.QuestFailureTimerJob(questUC)))
	r.Job(Name, job.SessionTimeout, cfg.TimeoutScan, module.JobFunc(job.SessionTimeoutJob(sessions)))
	r.Job(Name, job.CurfewSessionCloser, job.CurfewCloserInterval, module.JobFunc(job.CurfewSessionCloserJob(sessions)))
	return m, nil
}
