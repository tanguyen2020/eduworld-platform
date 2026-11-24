// Package game is the composition entry of module game (ADR-0074 §1, BACKEND_DESIGN §3.3).
// cmd/<binary> may only call Register, Facade and Wire; everything else lives in internal/.
package game

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/challengeschema"
	gameconfig "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/economyschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/liveopsschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/minigameschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/modules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/challengesql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/economysql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/liveopssql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/minigamesql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/progressionsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/questsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/worldsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/progressionschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/questschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/render"
	rewardinfra "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/reward"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/room"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/security"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/storage"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/facade"
	gamehttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/job"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/natscmd"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/migrations"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	pkgmedia "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/media"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/roomhost"

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
	// F078: tenant assignments of game resources (caller identity) and the school catalog (caller learning).
	ApplyTenantAssignments(ctx context.Context, req api.ApplyTenantAssignmentsRequestObject) (api.ApplyTenantAssignmentsResponseObject, error)
	RevokeTenantAssignments(ctx context.Context, req api.RevokeTenantAssignmentsRequestObject) (api.RevokeTenantAssignmentsResponseObject, error)
	ListResourceAssignments(ctx context.Context, req api.ListResourceAssignmentsRequestObject) (api.ListResourceAssignmentsResponseObject, error)
	ListAssignedGameActivities(ctx context.Context, req api.ListAssignedGameActivitiesRequestObject) (api.ListAssignedGameActivitiesResponseObject, error)
}

// Imports are the facades module game calls: getHubPosition of module competition (F071 T049, interaction distance of a
// discovery); nil = presence only. Everything else is projections and events (ADR-0037).
type Imports struct {
	Competition modules.CompetitionHub
	// Learning is createAttempt of the learning facade (F072: learning steps of the quests); nil = startQuestActivity 503.
	Learning modules.LearningFacade
	// Chat is screenText of the chat facade (F077: school event texts); nil = createSchoolEvent/updateSchoolEvent 503.
	Chat modules.ChatScreening
}

// Module is the registered module.
type Module struct {
	facade   Facade
	hub      *modules.LateHub
	learning *modules.LateLearning
	chat     *modules.LateChat
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
	if m.chat != nil {
		m.chat.Set(in.Chat)
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
// wallet-reconcile; the F076 challenge routes, the consumer game.challenge_progress, the room host of the challenge
// rooms (worker events) and the job challenge-deadline; the F077 live-ops routes, the consumers game.liveops_progress (game, learning, competition streams) and
// game.season_projection and the jobs event-scheduler, goal-recompute; createRealtimeTicket and the NATS command
// ew.cmd.game.session.heartbeat of the realtime gateway in core-api (F090).
func Register(r *module.Registry) (*Module, error) {
	pool, err := r.Database(Name)
	if err != nil {
		return nil, fmt.Errorf("game: open database: %w", err)
	}
	r.Health().Register("db:"+Name, health.Readiness, health.PostgresCheck(pool))
	r.Health().Register("atlas:"+Name, health.Readiness, health.AtlasRevisionCheck(pool, migrations.Latest()))
	if !r.ServesHTTP() && !r.RunsEvents() && !r.RunsJobs() {
		return &Module{hub: &modules.LateHub{}, learning: &modules.LateLearning{}, chat: &modules.LateChat{}}, nil
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
	observability.RegisterMiniGame(r.Metrics())
	observability.RegisterProgression(r.Metrics())
	observability.RegisterEconomy(r.Metrics())
	observability.RegisterLiveOps(r.Metrics())
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
	liveCfg, err := gameconfig.LiveOpsFrom(r.Config())
	if err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	challengeCfg, err := gameconfig.ChallengeFrom(r.Config())
	if err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	roomCfg, err := gameconfig.ChallengeRoomFrom(r.Config())
	if err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	observability.RegisterChallenge(r.Metrics())
	roomhost.RegisterMetrics(r.Metrics())
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
		Presence: presence, Learning: learningLate, MiniGames: minigamesql.Content{}, Realtime: realtime.QuestPublisher{C: vk},
		Metrics: observability.QuestMetrics{}, Config: questCfg}
	// F073 mini-games: free plays through createAttempt of module learning (plugin minigame), results from activity.graded.
	miniGameUC := command.MiniGames{UoW: core.UoW, Content: minigamesql.Content{}, World: worldsql.Content{}, Plays: write.MiniGamePlayRepo{},
		Events: write.Publisher{W: core.Writer}, Learning: learningLate, Metrics: observability.MiniGameMetrics{}, LearningRetry: questCfg.LearningRetry}
	// F074 progression: the Reward Engine, level-ups, curve rollout, grade transition, XP reconciliation, ledger merge.
	progressionUC := command.Progression{UoW: core.UoW, AfterCommit: entx.AfterCommit, Store: write.ProgressionRepo{}, Scan: write.ProgressionRepo{},
		Profiles: write.ProfileRepo{}, Students: worldsql.Content{}, Rules: progressionsql.Content{}, Curves: progressionsql.Content{},
		Events: write.Publisher{W: core.Writer}, Audit: write.Auditor{R: core.Recorder},
		Realtime: realtime.ProgressionPublisher{C: vk}, Quests: questUC, Metrics: observability.ProgressionMetrics{}, Policy: policy}
	// F075 economy: the appliers currency, item, badge, collection_item join the registry of the Reward Engine; the
	// economy grants its own rewards (tiers, completions, cards) through the same registry.
	economyUC := &command.Economy{UoW: core.UoW, AfterCommit: entx.AfterCommit, Store: write.EconomyRepo{}, Grants: write.ProgressionRepo{},
		Achievements: write.AchievementRepo{}, Adjustments: write.AdjustmentRepo{}, Scan: write.EconomyRepo{}, Content: economysql.Content{},
		Students: worldsql.Content{}, LiveOps: liveopssql.Store{}, Events: write.Publisher{W: core.Writer}, Audit: write.Auditor{R: core.Recorder},
		Metrics: observability.EconomyMetrics{}, Policy: ecoPolicy}
	appliers := rewardinfra.WithEconomy(economyUC)
	economyUC.Registry = appliers
	progressionUC.Appliers = appliers
	economyUC.XP = progressionUC
	// F077 live-ops: runs, participation, community goals, season track, school events; rewards through the same registry
	// (Xu cap kept); school texts screened by module chat (wired by Module.Wire, 503 until then).
	gameManifest := command.GameManifest{UoW: core.UoW, Store: write.ManifestRepo{}, Tenants: write.ManifestRepo{}, Content: worldsql.Content{},
		Quests: questsql.Content{}, LiveOps: liveopssql.Store{}, Events: write.Publisher{W: core.Writer}, Metrics: observability.ManifestMetrics{},
		BuildSeconds: observability.ManifestBuild}
	chatLate := &modules.LateChat{}
	liveOpsUC := &command.LiveOps{UoW: core.UoW, AfterCommit: entx.AfterCommit, Store: liveopssql.Store{}, Students: worldsql.Content{},
		Tenants: write.ManifestRepo{}, Manifests: gameManifest, Grants: write.ProgressionRepo{}, Registry: appliers, EventSchema: liveopsschema.Event{},
		Events: write.Publisher{W: core.Writer}, Audit: write.Auditor{R: core.Recorder}, Screener: chatLate, Counter: cache.GoalCounter{C: vk},
		Realtime: realtime.LiveOpsPublisher{C: vk}, Lock: cache.JobLock{C: vk}, Metrics: observability.LiveOpsMetrics{}, Policy: liveCfg.Command}
	// F076 solo challenges: createAttempt of module learning (mode challenge) when wired; one room per attempt in progress
	// on the shared room host (timer on the server clock), the job challenge-deadline as fallback from PostgreSQL.
	rooms := roomhost.ValkeyRegistry{C: vk, Index: gameconfig.ChallengeRoomIndex}
	challengeUC := command.Challenges{UoW: core.UoW, AfterCommit: entx.AfterCommit, Content: challengesql.Content{}, World: worldsql.Content{},
		Quests: questsql.Content{}, Store: challengesql.Store{}, Events: write.Publisher{W: core.Writer}, Learning: learningLate,
		Realtime: realtime.ChallengePublisher{C: vk}, Metrics: observability.ChallengeMetrics{}, Config: challengeCfg}
	challengeRoom := room.Challenges{Registry: rooms, Type: gameconfig.ChallengeRoomType, Expirer: challengeUC}
	challengeUC.Room = challengeRoom
	// F078 Game Operations Console: the release core with every type module (the same checks as the seed), the review
	// workflow, scheduled publications, previews, game media and renders, tenant assignments and the school catalog.
	adminCfg, err := gameconfig.GameAdminFrom(r.Config())
	if err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	observability.RegisterGameAdmin(r.Metrics())
	kernel := &command.PublishGameContent{UoW: core.UoW, AfterCommit: entx.AfterCommit, Store: write.GameContentRepo{}, Content: worldsql.Content{},
		Events: write.Publisher{W: core.Writer}, Audit: write.Auditor{R: core.Recorder}, Cache: cache.GameSnapshotCache{C: vk},
		QuestContent: questsql.Content{}, Quests: &command.QuestPublishRules{Schema: questschema.Validator{},
			Validator: quest.ValidatorConfig{PassRule: questCfg.PassRule, BlockMaxByBand: questCfg.BlockMax}},
		Progression: &command.ProgressionPublishRules{CurveSchema: progressionschema.Curve{}, RuleSchema: progressionschema.Rule{},
			Appliers: rewardinfra.Types(), Levels: write.GameContentRepo{}, Curves: progressionsql.Content{}},
		Economy: &command.EconomyPublishRules{ItemSchema: economyschema.Item{}, ShopSchema: economyschema.ShopItem{},
			CollectionSchema: economyschema.Collection{}, AchievementSchema: economyschema.Achievement{}, Appliers: rewardinfra.Types(),
			Items: economysql.Content{}, CollectionItems: write.GameContentRepo{}},
		LiveOps: &command.LiveOpsPublishRules{EventSchema: liveopsschema.Event{}, TemplateSchema: liveopsschema.Template{},
			TrackSchema: liveopsschema.Track{}, Reader: liveopssql.Store{}, Hook: liveOpsUC, Policy: liveCfg.Command.Rules},
		MiniGames:  &command.MiniGamePublishRules{Schema: minigameschema.Validator{}, Sources: learningLate},
		Challenges: &command.ChallengePublishRules{Schema: challengeschema.Validator{}, Rules: challengeCfg.Rules, Content: challengesql.Content{}}}
	adminUC := command.GameAdmin{UoW: core.UoW, Store: write.GameConfigRepo{}, Kernel: kernel, Learning: learningLate,
		Events: write.Publisher{W: core.Writer}, Audit: write.Auditor{R: core.Recorder}, Metrics: observability.GameAdminMetrics{}, Policy: adminCfg.Policy}
	urls := storage.PublicURL{BaseURL: adminCfg.CDNBase}
	adminQueries := query.GameAdmin{Read: core.ReadSession, Data: read.GameConfigs{}, Cursors: cursors, URLs: urls}
	mediaUC := command.GameMedia{UoW: core.UoW, Store: write.GameMediaRepo{}, Events: write.Publisher{W: core.Writer},
		Audit: write.Auditor{R: core.Recorder}, Metrics: observability.GameAdminMetrics{}, RenderBackoff: adminCfg.RenderBackoff,
		RenderMaxAttempts: adminCfg.RenderMaxAttempts, Batch: gameconfig.GameAdminJobBatch, Queue: render.LazyQueue{Get: r.JetStream}}
	if adminCfg.S3Endpoint != "" && adminCfg.S3AccessKey != "" {
		s3, err := storage.New(storage.Config{Endpoint: adminCfg.S3Endpoint, Region: adminCfg.S3Region, AccessKey: adminCfg.S3AccessKey,
			SecretKey: adminCfg.S3SecretKey, Timeout: adminCfg.S3Timeout}, nil)
		if err != nil {
			return nil, fmt.Errorf("game: %w", err)
		}
		mediaUC.Storage = s3
	}
	if adminCfg.ClamdAddr != "" {
		mediaUC.Scanner = storage.Scanner{Clamd: pkgmedia.Clamd{Addr: adminCfg.ClamdAddr}}
	}
	assignUC := command.GameAssignments{UoW: core.UoW, Store: write.TenantAssignmentRepo{}, Manifests: gameManifest,
		Audit: write.Auditor{R: core.Recorder}, Cursors: cursors}
	m := &Module{hub: hub, learning: learningLate, chat: chatLate, facade: facade.Facade{Lifecycle: command.StudentLifecycle{UoW: core.UoW, Sessions: write.SessionRepo{},
		Profiles: write.ProfileRepo{}, Presets: write.PresetRepo{}, XP: progressionUC},
		GameAdmin: facade.GameAdmin{Assignments: assignUC, Catalog: adminQueries}}}
	options := cache.NicknameOptionStore{C: vk}
	profiles := command.Profiles{UoW: core.UoW, AfterCommit: entx.AfterCommit, Repo: write.ProfileRepo{}, Catalog: write.CatalogRepo{},
		Owned: write.OwnershipRepo{}, Presets: write.PresetRepo{}, Directory: write.Directory{}, Schools: write.Directory{},
		Options: options, Events: write.Publisher{W: core.Writer}, Metrics: observability.ProfileMetrics{}, Config: profileCfg}

	h := gamehttp.Handlers{Sessions: sessions, ClientConfig: cfg.ClientConfig,
		Manifest: query.GetContentManifest{Read: core.ReadSession, Manifests: read.Manifests{}, MinClientVersion: cfg.UseCase.MinClientVersion,
			Students: worldsql.Content{}},
		LoginEvents: query.LoginEvents{Read: core.ReadSession, Events: read.LoginEvents{}, Students: read.Students{}, Cursors: cursors},
		Tickets:     command.RealtimeTickets{Sessions: sessions, Store: realtime.TicketStore{C: vk}, Config: cfg.Tickets},
		Profiles:    profiles, ProfileQueries: query.Profile{Read: core.ReadSession, Profiles: read.Profiles{}, Catalog: read.Catalogs{},
			Options: options, TTL: profileCfg.SuggestionTTL},
		World: worldUC, WorldQueries: query.World{Read: core.ReadSession, Content: worldsql.Content{}, Manifests: read.Manifests{}},
		Quests: questUC, QuestQueries: query.Quests{Read: core.ReadSession, Content: questsql.Content{}, World: worldsql.Content{}, Pages: read.Quests{}, Cursors: cursors,
			Reset: questCfg.Reset},
		ProgressionQueries: query.Progression{Read: core.ReadSession, Curves: progressionsql.Content{}, Data: read.Progression{}, Cursors: cursors,
			DailyCap: policy.DailyCap},
		EconomyQueries: query.Economy{Read: core.ReadSession, Data: read.Economy{}, Cursors: cursors}, EconomyCommands: economyUC,
		LiveOpsQueries: query.LiveOps{Read: core.ReadSession, Data: liveopssql.Store{}, Students: worldsql.Content{}, Counter: cache.GoalCounter{C: vk},
			Cursors: cursors, Policy: liveCfg.Query}, LiveOpsCommands: liveOpsUC, MiniGames: miniGameUC,
		Challenges: challengeUC, ChallengeQueries: query.Challenges{Read: core.ReadSession, Content: challengesql.Content{}, World: worldsql.Content{},
			Quests: questsql.Content{}, Cursors: cursors, Rules: challengeCfg.Rules, Reset: challengeCfg.Reset},
		Admin: gamehttp.GameAdminHandlers{Commands: adminUC, Queries: adminQueries, Media: mediaUC}}
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
		return consumer.SessionEndOnRevokeOptions(consumer.SessionEndOnRevoke{Sessions: sessions, Challenges: challengeUC}, core.UoW, core.App, js)
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
		return consumer.ManifestOnGameContentOptions(consumer.ManifestOnGameContent{Manifests: gameManifest}, core.UoW, core.App, js)
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
	r.Consumer(messaging.MiniGameActivityGradedName, func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.MiniGameOnActivityGradedOptions(consumer.MiniGameOnActivityGraded{MiniGames: miniGameUC}, core.UoW, core.App, js)
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
	r.Consumer("game.liveops_progress", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.LiveOpsProgressOptions(consumer.LiveOpsProgress{LiveOps: liveOpsUC}, core.UoW, core.App, js)
	})
	r.Consumer("game.liveops_progress_learning", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.LiveOpsProgressLearningOptions(consumer.LiveOpsProgress{LiveOps: liveOpsUC}, core.UoW, core.App, js)
	})
	r.Consumer("game.liveops_progress_competition", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.LiveOpsProgressCompetitionOptions(consumer.LiveOpsProgress{LiveOps: liveOpsUC}, core.UoW, core.App, js)
	})
	r.Consumer("game.challenge_progress", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.ChallengeProgressOptions(consumer.ChallengeProgress{Challenges: challengeUC}, core.UoW, core.App, js)
	})
	// The room host of the challenge rooms (pkg/roomhost): worker events, several instances share the rooms by lease.
	host := roomhost.NewHost(roomCfg, rooms, challengeRoom, nil, roomhost.PromMetrics{})
	host.Handle(gameconfig.ChallengeRoomType, challengeRoom)
	r.Background(Name, "roomhost", "game.challenge_room_host", r.RunsEvents(), host.Run)
	r.Consumer("game.season_projection", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.SeasonProjectionOptions(consumer.SeasonProjection{Seasons: liveOpsUC}, core.UoW, core.App, js)
	})
	r.Job(Name, job.EventScheduler, gameconfig.EventSchedulerInterval, module.JobFunc(job.EventSchedulerJob(liveOpsUC, gameconfig.LiveOpsJobBatch)))
	r.Job(Name, job.GoalRecompute, gameconfig.GoalRecomputeInterval, module.JobFunc(job.GoalRecomputeJob(liveOpsUC, gameconfig.LiveOpsJobBatch)))
	r.Job(Name, job.WalletReconcile, gameconfig.WalletReconcileInterval, module.JobFunc(job.WalletReconcileJob(economyUC)))
	r.Job(Name, job.CurveRollout, gameconfig.CurveRolloutInterval, module.JobFunc(job.CurveRolloutJob(progressionUC)))
	r.Job(Name, job.XPReconcile, gameconfig.XPReconcileInterval, module.JobFunc(job.XPReconcileJob(progressionUC)))
	r.Job(Name, job.ChallengeDeadline, gameconfig.ChallengeDeadlineInterval, module.JobFunc(job.ChallengeDeadlineJob(challengeUC,
		challengeCfg.JobBatch)))
	r.Job(Name, job.QuestPeriodExpiry, questCfg.JobInterval, module.JobFunc(job.QuestPeriodExpiryJob(questUC)))
	r.Job(Name, job.QuestFailureTimer, questCfg.JobInterval, module.JobFunc(job.QuestFailureTimerJob(questUC)))
	// F078: renders of the published 3D content (work queue GAME_RENDER_JOBS of the game-renderer container) and their
	// results; scheduled publications, the game media pipeline and the render retries.
	r.Consumer(messaging.RenderThumbnailsName, func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.RenderThumbnailsOptions(consumer.RenderThumbnails{Renders: mediaUC}, core.UoW, core.App, js)
	})
	r.Background(Name, "consumer", "game.render_results", r.RunsEvents(), func(ctx context.Context) error {
		js, err := r.JetStream()
		if err != nil {
			return err
		}
		return render.ConsumeResults(ctx, js, mediaUC.RecordRenderResult, slog.Default())
	})
	r.Job(Name, job.ScheduledPublish, gameconfig.ScheduledPublishInterval, module.JobFunc(job.ScheduledPublishJob(adminUC)))
	r.Job(Name, job.MediaPipeline, gameconfig.MediaPipelineInterval, module.JobFunc(job.MediaPipelineJob(mediaUC)))
	r.Job(Name, job.RenderRetry, gameconfig.RenderRetryInterval, module.JobFunc(job.RenderRetryJob(mediaUC)))
	r.Job(Name, job.SessionTimeout, cfg.TimeoutScan, module.JobFunc(job.SessionTimeoutJob(sessions)))
	r.Job(Name, job.CurfewSessionCloser, job.CurfewCloserInterval, module.JobFunc(job.CurfewSessionCloserJob(sessions)))
	return m, nil
}
