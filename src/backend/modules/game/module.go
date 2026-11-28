// Package game is the composition entry of module game (ADR-0074 §1, BACKEND_DESIGN §3.3).
// cmd/<binary> may only call Register, Facade and Wire; everything else lives in internal/.
package game

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus/collectors"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/bossschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/challengeschema"
	gameconfig "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/economyschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/liveopsschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/minigameschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/modules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/analyticssql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/bosssql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/challengesql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/economysql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/liveopssql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/minigamesql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/partyquestsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/progressionsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/questsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/worldsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/progressionschema"
	promsource "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/prometheus"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/questschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/release"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/render"
	rewardinfra "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/reward"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/room"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/security"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/storage"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/translation"
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
// implemented in this release: the student lifecycle operations of the identity sagas (F016), mergeUserData (F070 T039), the
// tenant assignments (F078) and the trash of the game resources (listTrash, restoreTrashItem — F057). Clone
// (executeTenantCloneStep) is not implemented by module game: the identity gateway answers 503 UNAVAILABLE for it, never a
// silent success.
type Facade interface {
	// listOutboxEventIds / listProcessedEventIds (worker reconcile, F134).
	ListOutboxEventIds(ctx context.Context, req api.ListOutboxEventIdsRequestObject) (api.ListOutboxEventIdsResponseObject, error)
	ListProcessedEventIds(ctx context.Context, req api.ListProcessedEventIdsRequestObject) (api.ListProcessedEventIdsResponseObject, error)
	CleanupStudentData(ctx context.Context, req api.CleanupStudentDataRequestObject) (api.CleanupStudentDataResponseObject, error)
	RestoreStudentData(ctx context.Context, req api.RestoreStudentDataRequestObject) (api.RestoreStudentDataResponseObject, error)
	AnonymizeUserData(ctx context.Context, req api.AnonymizeUserDataRequestObject) (api.AnonymizeUserDataResponseObject, error)
	MergeUserData(ctx context.Context, req api.MergeUserDataRequestObject) (api.MergeUserDataResponseObject, error)
	// F078: tenant assignments of game resources (caller identity) and the school catalog (caller learning).
	ApplyTenantAssignments(ctx context.Context, req api.ApplyTenantAssignmentsRequestObject) (api.ApplyTenantAssignmentsResponseObject, error)
	RevokeTenantAssignments(ctx context.Context, req api.RevokeTenantAssignmentsRequestObject) (api.RevokeTenantAssignmentsResponseObject, error)
	ListResourceAssignments(ctx context.Context, req api.ListResourceAssignmentsRequestObject) (api.ListResourceAssignmentsResponseObject, error)
	// F057: the trash of the game resources (caller identity, ADR-0028).
	ListTrash(ctx context.Context, req api.ListTrashRequestObject) (api.ListTrashResponseObject, error)
	RestoreTrashItem(ctx context.Context, req api.RestoreTrashItemRequestObject) (api.RestoreTrashItemResponseObject, error)
	ListAssignedGameActivities(ctx context.Context, req api.ListAssignedGameActivitiesRequestObject) (api.ListAssignedGameActivitiesResponseObject, error)
	// F103: the party activities of the Party Lobby (caller competition).
	ListPartyActivityCandidates(ctx context.Context, req api.ListPartyActivityCandidatesRequestObject) (api.ListPartyActivityCandidatesResponseObject, error)
	CheckPartyActivityEligibility(ctx context.Context, req api.CheckPartyActivityEligibilityRequestObject) (api.CheckPartyActivityEligibilityResponseObject, error)
	StartPartyActivity(ctx context.Context, req api.StartPartyActivityRequestObject) (api.StartPartyActivityResponseObject, error)
}

// Imports are the facades module game calls: getHubPosition of module competition (F071 T049, interaction distance of a
// discovery); nil = presence only. Everything else is projections and events (ADR-0037).
type Imports struct {
	Competition modules.CompetitionHub
	// Learning is createAttempt of the learning facade (F072: learning steps of the quests); nil = startQuestActivity 503.
	Learning modules.LearningFacade
	// Chat is screenText of the chat facade (F077: school event texts); nil = createSchoolEvent/updateSchoolEvent 503.
	Chat modules.ChatScreening
	// ChatNickname is moderateDisplayText of the chat facade (F136: typed nicknames, context nickname); nil = a typed
	// nickname is 503 UNAVAILABLE (fail-closed), the suggestions keep working.
	ChatNickname modules.ChatNicknameModeration
}

// Module is the registered module.
type Module struct {
	facade   Facade
	hub      *modules.LateHub
	learning *modules.LateLearning
	chat     *modules.LateChat
	nick     *modules.LateNicknameScreens
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
	if m.nick != nil {
		m.nick.SetChat(in.ChatNickname)
	}
}

// WireIdentity connects screenNicknamePersonalData of the identity facade (F136), built after module game by the
// composition root (identity calls game in its sagas); until then a typed nickname is 503 UNAVAILABLE.
func (m *Module) WireIdentity(f modules.IdentityNicknameScreen) {
	if m.nick != nil {
		m.nick.SetIdentity(f)
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
	if !r.ServesHTTP() && !r.RunsEvents() && !r.RunsJobs() && !r.Reconciles() {
		return &Module{hub: &modules.LateHub{}, learning: &modules.LateLearning{}, chat: &modules.LateChat{}, nick: &modules.LateNicknameScreens{}}, nil
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
	nickScreens := &modules.LateNicknameScreens{}
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
	// F103 boss battles and co-op quests: one room per live encounter / shared quest session on the same room host
	// (deadlines, reconnect grace), question plans and grading by module learning (createQuestionPlan, scorePlanAnswer),
	// clue cards from module learning behind the Valkey cache.
	bossCfg, err := gameconfig.BossFrom(r.Config(), challengeCfg, cfg.UseCase.HeartbeatTimeout)
	if err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	coopCfg, err := gameconfig.PartyQuestFrom(r.Config(), cfg.UseCase.HeartbeatTimeout)
	if err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	observability.RegisterBoss(r.Metrics())
	bossLearning := modules.LateBossLearning{L: learningLate}
	bossPush := realtime.BossPublisher{C: vk}
	clues := cache.ClueCache{C: vk, Source: bossLearning}
	bossUC := command.Bosses{UoW: core.UoW, AfterCommit: entx.AfterCommit, Content: bosssql.Content{}, World: worldsql.Content{},
		Quests: questsql.Content{}, Store: bosssql.Store{}, Events: write.Publisher{W: core.Writer}, Audit: write.Auditor{R: core.Recorder},
		Learning: bossLearning, Realtime: bossPush, Presence: bosssql.Presence{}, Names: bosssql.Reads{}, Metrics: observability.BossMetrics{},
		Config: bossCfg}
	bossRoom := room.Rooms{Registry: rooms, Type: gameconfig.BossRoomType}
	bossUC.Room = bossRoom
	bossRoom.Use = bossUC
	coopUC := command.PartyQuests{UoW: core.UoW, AfterCommit: entx.AfterCommit, Content: questsql.Content{}, Party: questsql.Content{},
		World: worldsql.Content{}, Store: partyquestsql.Store{}, Quests: questUC, Events: write.Publisher{W: core.Writer},
		Audit: write.Auditor{R: core.Recorder}, Messages: bossPush, Renders: clues, Presence: bosssql.Presence{}, Names: bosssql.Reads{},
		Metrics: observability.BossMetrics{}, Config: coopCfg}
	coopRoom := room.Rooms{Registry: rooms, Type: gameconfig.PartyQuestRoomType}
	coopUC.Room = coopRoom
	coopRoom.Use = coopUC
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
		Challenges: &command.ChallengePublishRules{Schema: challengeschema.Validator{}, Rules: challengeCfg.Rules, Content: challengesql.Content{}},
		Bosses:     &command.BossPublishRules{Schema: bossschema.Validator{}, Content: bosssql.Content{}}}
	adminUC := command.GameAdmin{UoW: core.UoW, Store: write.GameConfigRepo{}, Kernel: kernel, Learning: learningLate,
		Events: write.Publisher{W: core.Writer}, Audit: write.Auditor{R: core.Recorder}, Metrics: observability.GameAdminMetrics{}, Policy: adminCfg.Policy,
		Translations: write.TranslationRepo{}}
	urls := storage.PublicURL{BaseURL: adminCfg.CDNBase}
	simRate, err := gameconfig.XPSimulationRateFrom(r.Config())
	if err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	adminQueries := query.GameAdmin{Read: core.ReadSession, Data: read.GameConfigs{}, Cursors: cursors, URLs: urls,
		Simulation: query.SimulationPolicy{MinuteRate: simRate, DailyCap: policy.DailyCap}}
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
	// F082: translation files of game content (export/import jobs) and the backfill of translation states.
	observability.RegisterTranslations(r.Metrics())
	translationsUC := command.Translations{UoW: core.UoW, Store: write.TranslationRepo{}, Configs: write.GameConfigRepo{}, Media: write.GameMediaRepo{},
		Admin: adminUC, Storage: mediaUC.Storage, Codecs: map[string]port.TranslationCodec{"xliff": translation.XLIFF{}, "csv": translation.CSV{}},
		Audit: write.Auditor{R: core.Recorder}, Metrics: observability.TranslationMetrics{}}
	translationQueries := query.TranslationJobs{Read: core.ReadSession, Jobs: read.TranslationJobs{}, Storage: mediaUC.Storage,
		LinkTTL: adminCfg.ReportLinkTTL}
	backfillUC := command.TranslationBackfill{UoW: core.UoW, Store: write.TranslationRepo{}, Required: adminCfg.Policy.RequiredLanguages}
	assignUC := command.GameAssignments{UoW: core.UoW, Store: write.TenantAssignmentRepo{}, Manifests: gameManifest,
		Audit: write.Auditor{R: core.Recorder}, Cursors: cursors}
	// F080 Game Analytics: telemetry, the analytics consumers, rollup/close/retention/export jobs, the dashboard reads on
	// their own session (game_dashboard / game_platform on the replica) and the Live dashboard from Prometheus.
	anaCfg, err := gameconfig.AnalyticsFrom(r.Config())
	if err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	observability.RegisterAnalytics(r.Metrics())
	// School installer page (F134 US3): the signed school-kit index of the release CDN, filtered by the school grades.
	instCfg, err := gameconfig.InstallerFrom(r.Config())
	if err != nil {
		return nil, err
	}
	observability.RegisterInstaller(r.Metrics())
	installers := query.SchoolInstallers{Read: core.ReadSession, Reads: read.Installers{}, Metrics: observability.InstallerMetrics{},
		WebClientURL: instCfg.WebClientURL}
	if instCfg.ReleasesBaseURL != "" {
		ix, err := release.New(release.Config{BaseURL: instCfg.ReleasesBaseURL, PublicKeyPEM: instCfg.PublicKeyPEM}, nil, cache.Dashboard{C: vk})
		if err != nil {
			return nil, fmt.Errorf("game: %w", err)
		}
		installers.Index = ix
	}
	// DB performance KPI of the Live dashboard (FR-OPS-003): pool statistics go_sql_* of the game database (label db_name).
	_ = r.Metrics().Register(collectors.NewDBStatsCollector(core.App, "eduworld_game"))
	ingest := command.AnalyticsIngest{UoW: core.UoW, Store: analyticssql.Store{}, Keys: anaCfg.Keys, Rate: cache.RateLimiter{C: vk},
		Metrics: observability.AnalyticsMetrics{}, Config: anaCfg.Config}
	dashSession, err := dashboardSession(r, r.ServesHTTP() || r.RunsJobs())
	if err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	gameAnalytics := query.GameAnalytics{Model: analyticssql.Dashboard{}, Cache: cache.Dashboard{C: vk}, Limiter: cache.NewDashLimiter(vk),
		Metrics: observability.AnalyticsMetrics{}, Config: anaCfg.Config, Cursors: cursors}
	if dashSession != nil {
		gameAnalytics.Session = dashSession
	}
	exportsUC := command.AnalyticsExports{UoW: core.UoW, Store: analyticssql.Exports{}, Renderer: gameAnalytics, Audit: core.Recorder, Config: anaCfg.Config}
	if s3 := mediaUC.Storage; s3 != nil {
		if st, ok := s3.(*storage.S3); ok {
			exportsUC.Files = storage.Exports{S3: st, Bucket: gameconfig.ExportBucket}
		}
	}
	rollupUC := command.AnalyticsRollup{UoW: core.UoW, Store: analyticssql.Store{}, Versions: cache.Dashboard{C: vk}, Metrics: observability.AnalyticsMetrics{},
		Config: anaCfg.Config}
	overview := query.LiveOpsOverview{Thresholds: anaCfg.Thresholds}
	if anaCfg.PrometheusURL != "" {
		overview.Source = promsource.Source{BaseURL: anaCfg.PrometheusURL}
	}
	var anonymizer command.AnalyticsAnonymizer
	if len(anaCfg.Keys.All()) > 0 {
		anonymizer = ingest
	}
	m := &Module{hub: hub, learning: learningLate, chat: chatLate, nick: nickScreens, facade: facade.Facade{Ops: r.ReconcileSource(Name), Lifecycle: command.StudentLifecycle{UoW: core.UoW, Sessions: write.SessionRepo{},
		Profiles: write.ProfileRepo{}, Presets: write.PresetRepo{}, XP: progressionUC, Analytics: anonymizer, Audit: write.Auditor{R: core.Recorder}},
		GameAdmin: facade.GameAdmin{Assignments: assignUC, Catalog: adminQueries},
		Trash: facade.Trash{TrashList: query.Trash{Read: core.ReadSession, Model: read.Trash{}, Cursors: cursors},
			TrashRestore: command.RestoreTrash{UoW: core.UoW, Store: write.TrashRepo{}, Events: write.Publisher{W: core.Writer},
				Audit: write.Auditor{R: core.Recorder}}, Idempotency: core.Idempotency},
		PartyActivities: facade.PartyActivities{Queries: query.PartyActivities{Read: core.ReadSession, Bosses: bosssql.Content{},
			BossReads: bosssql.Reads{}, Party: questsql.Content{}, Quests: questsql.Content{}, World: worldsql.Content{},
			AttemptsPerDay: bossCfg.AttemptsPerDay, Reset: bossCfg.Reset}, Bosses: bossUC, Quests: coopUC}}}
	options := cache.NicknameOptionStore{C: vk}
	// F136: typed nicknames go through the chat moderation and the identity screen (facades wired after Register, fail-closed
	// until then) and the attempt counter nickname_attempt_rate.
	profiles := command.Profiles{UoW: core.UoW, AfterCommit: entx.AfterCommit, Repo: write.ProfileRepo{}, Catalog: write.CatalogRepo{},
		Owned: write.OwnershipRepo{}, Presets: write.PresetRepo{}, Directory: write.Directory{}, Schools: write.Directory{},
		Options: options, Events: write.Publisher{W: core.Writer}, Metrics: observability.ProfileMetrics{}, Config: profileCfg,
		Audit: write.Auditor{R: core.Recorder}, Moderation: nickScreens, Personal: nickScreens,
		Attempts:        cache.NicknameAttempts{C: vk, Limit: profileCfg.AttemptLimit, Window: profileCfg.AttemptWindow},
		NicknameMetrics: observability.ProfileMetrics{}}

	h := gamehttp.Handlers{Sessions: sessions, ClientConfig: cfg.ClientConfig,
		Manifest: query.GetContentManifest{Read: core.ReadSession, Manifests: read.Manifests{}, MinClientVersion: cfg.UseCase.MinClientVersion,
			Students: worldsql.Content{}},
		LoginEvents: query.LoginEvents{Read: core.ReadSession, Events: read.LoginEvents{}, Students: read.Students{}, Cursors: cursors},
		Tickets:     command.RealtimeTickets{Sessions: sessions, Store: realtime.TicketStore{C: vk}, Config: cfg.Tickets},
		Profiles:    profiles, ProfileQueries: query.Profile{Read: core.ReadSession, Profiles: read.Profiles{}, Catalog: read.Catalogs{},
			Options: options, TTL: profileCfg.SuggestionTTL, Policy: profileCfg.Nickname},
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
		Bosses: bossUC, BossQueries: query.Bosses{Read: core.ReadSession, Content: bosssql.Content{}, Reads: bosssql.Reads{}, World: worldsql.Content{},
			Cursors: cursors},
		CoopQuests: coopUC, CoopQuestQueries: query.QuestClues{Read: core.ReadSession, Reads: partyquestsql.Reads{}, Content: questsql.Content{},
			Renders: clues, Names: bosssql.Reads{}},
		Admin:         gamehttp.GameAdminHandlers{Commands: adminUC, Queries: adminQueries, Media: mediaUC},
		Analytics:     gamehttp.AnalyticsHandlers{Ingest: ingest, Queries: gameAnalytics, Exports: exportsUC, Overview: overview},
		TelemetryFlag: query.ProductTelemetryFlag{Read: core.ReadSession, Flags: analyticssql.Store{}, Platform: anaCfg.Config.ProductTelemetry},
		Translations:  gamehttp.TranslationHandlers{Commands: translationsUC, Queries: translationQueries}, Installers: installers}
	if len(cfg.IPHashKey) > 0 {
		h.IPHasher = security.NewIPHasher(cfg.IPHashKey)
	}
	r.HTTP(Name, func(s *httpx.Server) error { return gamehttp.Register(s, h) })
	// NATS command of the realtime gateway (F090-AC-12): served by core-api, queue group "game".
	if nc := r.NATS(); nc != nil && r.ServesHTTP() {
		r.Go("game.cmd.session_heartbeat", func(ctx context.Context) error {
			return rt.ServeCommand(ctx, nc, rt.SessionHeartbeatModule, rt.SessionHeartbeatType, natscmd.SessionHeartbeat(sessions), 0)
		})
		// F103: boss.encounter.ready and boss.answer.submit (the answer waits for the grading of module learning).
		r.Go("game.cmd.boss_ready", func(ctx context.Context) error {
			return rt.ServeCommand(ctx, nc, natscmd.Module, natscmd.TypeBossReady, natscmd.BossReady(bossUC), 0)
		})
		r.Go("game.cmd.boss_answer", func(ctx context.Context) error {
			return rt.ServeCommand(ctx, nc, natscmd.Module, natscmd.TypeBossAnswer, natscmd.BossAnswer(bossUC), 5*time.Second)
		})
	}

	if err := r.Relay(Name); err != nil {
		return nil, fmt.Errorf("game: %w", err)
	}
	r.Consumer("game.projection_identity", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.IdentityProjectionOptions(consumer.IdentityProjection{Upsert: command.UpsertProjection{Refs: write.RefRepo{}}},
			core.UoW, core.App, js)
	})
	r.Consumer(messaging.SanctionNicknameResetName, func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.SanctionNicknameResetOptions(consumer.SanctionNicknameReset{Reset: profiles}, core.UoW, core.App, js)
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
	host := roomhost.NewHost(roomCfg, rooms, room.Checkpoints{gameconfig.ChallengeRoomType: challengeRoom, gameconfig.BossRoomType: bossRoom,
		gameconfig.PartyQuestRoomType: coopRoom}, nil, roomhost.PromMetrics{})
	host.Handle(gameconfig.ChallengeRoomType, challengeRoom)
	host.Handle(gameconfig.BossRoomType, bossRoom)
	host.Handle(gameconfig.PartyQuestRoomType, coopRoom)
	// F103 consumers: party membership (competition stream), shared steps and boss steps (game stream), collective and
	// interdependent steps (learning stream).
	r.Consumer(messaging.BossPartyMembershipName, func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.PartyMembershipOptions(consumer.PartyMembership{Bosses: bossUC, Quests: coopUC}, core.UoW, core.App, js)
	})
	r.Consumer(messaging.PartyQuestStepName, func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.PartyQuestOnStepOptions(consumer.PartyQuestOnStep{Quests: coopUC}, core.UoW, core.App, js)
	})
	r.Consumer(messaging.PartyQuestGradedName, func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.PartyQuestOnGradedOptions(consumer.PartyQuestOnGraded{Quests: coopUC}, core.UoW, core.App, js)
	})
	r.Consumer(messaging.QuestBossName, func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.QuestOnBossOptions(consumer.QuestOnBoss{Quests: questUC}, core.UoW, core.App, js)
	})
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
	// F080: analytics consumers (only with the player key) and jobs.
	if len(anaCfg.Keys.All()) > 0 {
		ah := consumer.Analytics{Ingest: ingest}
		r.Consumer(messaging.AnalyticsGameName, func(js jetstream.JetStream) outbox.ConsumerOptions {
			return consumer.AnalyticsGameOptions(ah, core.UoW, core.App, js)
		})
		r.Consumer(messaging.AnalyticsCompetitionName, func(js jetstream.JetStream) outbox.ConsumerOptions {
			return consumer.AnalyticsCompetitionOptions(ah, core.UoW, core.App, js)
		})
		r.Consumer(messaging.AnalyticsLearningName, func(js jetstream.JetStream) outbox.ConsumerOptions {
			return consumer.AnalyticsLearningOptions(ah, core.UoW, core.App, js)
		})
		r.Consumer(messaging.AnalyticsChatName, func(js jetstream.JetStream) outbox.ConsumerOptions {
			return consumer.AnalyticsChatOptions(ah, core.UoW, core.App, js)
		})
	} else if r.RunsEvents() {
		slog.Warn("game analytics consumers disabled: EW_ANALYTICS_PLAYER_KEYS is not set")
	}
	r.Job(Name, job.AnalyticsRollup, gameconfig.AnalyticsRollupInterval, module.JobFunc(job.AnalyticsRollupJob(rollupUC)))
	r.Job(Name, job.AnalyticsDayClose, gameconfig.AnalyticsDayCloseInterval, module.JobFunc(job.AnalyticsDayCloseJob(rollupUC)))
	r.Job(Name, job.AnalyticsRetention, gameconfig.AnalyticsRetentionInterval, module.JobFunc(job.AnalyticsRetentionJob(rollupUC,
		gameconfig.AnalyticsRetentionBatch)))
	// Without object storage (EW_S3_*) the export worker leaves the jobs queued.
	r.Job(Name, job.AnalyticsExport, gameconfig.AnalyticsExportInterval, module.JobFunc(job.AnalyticsExportJob(exportsUC)))
	r.Job(Name, job.TranslationJobs, gameconfig.TranslationJobsInterval, module.JobFunc(job.TranslationJobsJob(translationsUC)))
	r.Job(Name, job.TranslationBackfill, gameconfig.TranslationBackfillInterval, module.JobFunc(job.TranslationBackfillJob(backfillUC)))
	r.Job(Name, job.SessionTimeout, cfg.TimeoutScan, module.JobFunc(job.SessionTimeoutJob(sessions)))
	r.Job(Name, job.CurfewSessionCloser, job.CurfewCloserInterval, module.JobFunc(job.CurfewSessionCloserJob(sessions)))
	return m, nil
}

// DashboardPoolSize is dashboard_db_pool_per_api_replica (CONFIG_DEFAULTS, DASHBOARD_DATA §7).
const DashboardPoolSize = 10

// dashboardSession opens the read session of Game Analytics (F080-AC-11, DASHBOARD_DATA §7): the role game_dashboard
// (EW_DB_GAME_DASHBOARD_DSN) and, for the cross-tenant platform reads (audited platform_analytics bypass), game_platform
// (EW_DB_GAME_DASHBOARD_PLATFORM_DSN), both on the read replica in staging/production (local: the primary). Without them
// the analytics routes answer 503; they never fall back to the business pools.
func dashboardSession(r *module.Registry, needed bool) (*entx.ReadSession, error) {
	if !needed {
		return nil, nil
	}
	dash, err := r.SQL(Name, "dashboard")
	if errors.Is(err, module.ErrDatabaseNotConfigured) {
		slog.Warn("game analytics dashboards disabled: EW_DB_GAME_DASHBOARD_DSN is not set (503)")
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	plat, err := r.SQL(Name, "dashboard_platform")
	if errors.Is(err, module.ErrDatabaseNotConfigured) {
		slog.Warn("game analytics dashboards disabled: EW_DB_GAME_DASHBOARD_PLATFORM_DSN is not set (503)")
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	dash.SetMaxOpenConns(DashboardPoolSize)
	plat.SetMaxOpenConns(DashboardPoolSize)
	return entx.NewReadSession(Name, entx.DBs{App: dash, Platform: plat})
}
