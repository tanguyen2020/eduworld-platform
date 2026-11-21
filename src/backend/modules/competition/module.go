// Package competition is the composition entry of module competition (ADR-0074 §1).
// cmd/<binary> may only call Register, Facade and Wire; everything else lives in internal/.
package competition

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/battlehost"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/cache"
	hubconfig "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/modules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/roomhost"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/facade"
	competitionhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/job"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/natscmd"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/subscriber"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/migrations"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"

	_ "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/ent/runtime" // ent mixin hooks and interceptors (R27)
)

// Name is the module name used for its database (eduworld_competition), env prefix and checks.
const Name = "competition"

// Imports are the facades module competition calls: moderateDisplayText of module chat (party names, F100-AC-05) and
// the party activities of module game (quest/boss sessions, F101 research R1). Chat nil (module chat implements it with
// F116): every typed name falls back to the generated default name. Game nil (module game implements it with F103):
// quest and boss are not offered and answer 503 UNAVAILABLE; explore needs no facade.
type Imports struct {
	Chat modules.ChatModeration
	Game modules.GameActivities
	// Learning is the question plans of module learning for the battle engine (F105): nil → every new match aborts
	// with plan_failure (no question can be served).
	Learning modules.LearningPlans
}

// Facade is the part of the competition facade (api.StrictServerInterface, specs/openapi/modules/competition.internal.v1.yaml)
// implemented in this release: the student lifecycle and user merge operations of the identity sagas (F100 T042, OQ
// F016-2, F045-3). The other platform-ops operations (assignments, trash, clone) are not implemented yet: the identity
// gateway answers 503 UNAVAILABLE for them, never a silent success.
type Facade interface {
	CleanupStudentData(ctx context.Context, req api.CleanupStudentDataRequestObject) (api.CleanupStudentDataResponseObject, error)
	RestoreStudentData(ctx context.Context, req api.RestoreStudentDataRequestObject) (api.RestoreStudentDataResponseObject, error)
	AnonymizeUserData(ctx context.Context, req api.AnonymizeUserDataRequestObject) (api.AnonymizeUserDataResponseObject, error)
	MergeUserData(ctx context.Context, req api.MergeUserDataRequestObject) (api.MergeUserDataResponseObject, error)
	// GetHubPosition is called by module game (F071 T049) to check the interaction distance of a discovery.
	GetHubPosition(ctx context.Context, req api.GetHubPositionRequestObject) (api.GetHubPositionResponseObject, error)
}

// Module is the registered module.
type Module struct {
	moderator *modules.LateModerator
	game      *modules.LateGame
	learning  *modules.LateLearning
	deadlines modules.LearningDeadlines
	facade    Facade
}

// Facade returns the in-process facade of module competition (nil in a binary that does not load its use cases).
func (m *Module) Facade() Facade { return m.facade }

// Wire connects the facades of other modules (in process, pkg/modcall).
func (m *Module) Wire(in Imports) {
	if m.moderator != nil {
		m.moderator.Set(in.Chat)
	}
	if m.game != nil {
		m.game.Set(in.Game)
	}
	if m.learning != nil {
		m.learning.Set(in.Learning, m.deadlines)
	}
}

// Register wires the module into the running binary: health checks everywhere; in core-api the room host of the hub
// instances, the NATS commands hub.instance.join / hub.player.emote of the realtime gateway (F091), the party routes and
// the party presence store (F100); in worker events the outbox relay and the consumers competition.projection_identity,
// competition.projection_game, competition.projection_social (relation events of module chat, F095),
// competition.hub_access_revoked, competition.party_safety_chat and competition.party_safety_identity (F100); in worker
// jobs close-idle-hub-instances and party-invitation-expiry.
func Register(r *module.Registry) (*Module, error) {
	db, err := r.Database(Name)
	if err != nil {
		return nil, fmt.Errorf("competition: open database: %w", err)
	}
	r.Health().Register("db:"+Name, health.Readiness, health.PostgresCheck(db))
	r.Health().Register("atlas:"+Name, health.Readiness, health.AtlasRevisionCheck(db, migrations.Latest()))
	m := &Module{moderator: &modules.LateModerator{}, game: &modules.LateGame{}, learning: &modules.LateLearning{}}
	if !r.ServesHTTP() && !r.RunsEvents() && !r.RunsJobs() {
		return m, nil
	}
	core, err := r.Core(Name)
	if err != nil {
		return nil, fmt.Errorf("competition: %w", err)
	}
	vk, err := r.Valkey()
	if err != nil {
		return nil, fmt.Errorf("competition: %w", err)
	}
	cfg, err := hubconfig.HubFrom(r.Config())
	if err != nil {
		return nil, fmt.Errorf("competition: %w", err)
	}
	partyCfg, err := hubconfig.PartyFrom(r.Config())
	if err != nil {
		return nil, fmt.Errorf("competition: %w", err)
	}
	lobbyCfg, err := hubconfig.LobbyFrom(r.Config())
	if err != nil {
		return nil, fmt.Errorf("competition: %w", err)
	}
	observability.Register(r.Metrics())
	observability.RegisterParty(r.Metrics())
	observability.RegisterLobby(r.Metrics())
	observability.RegisterMatchmaking(r.Metrics())
	mmCfg, err := hubconfig.MatchmakingFrom(r.Config())
	if err != nil {
		return nil, fmt.Errorf("competition: %w", err)
	}
	reg := cache.HubRegistry{C: vk}
	pub := rt.Publisher{C: vk}
	host := roomhost.New(roomhost.Config{InstanceID: cfg.InstanceID, TickHz: cfg.TickHz, Grace: cfg.Grace, IdleClose: cfg.IdleClose,
		Silence: cfg.Silence, EmoteRate: cfg.EmoteRate, LeaseTTL: cfg.LeaseTTL, LeaseEvery: cfg.LeaseEvery, WatchEvery: cfg.WatchEvery,
		ClientOnly: !r.ServesHTTP()}, r.NATS(), reg, pub, nil, nil, nil)
	hub := command.Hub{UoW: core.UoW, Directory: persistence.Directory{}, Store: persistence.Store{}, Registry: reg, Rooms: host,
		Presence: realtime.Presence{P: pub}, Audit: persistence.Auditor{R: core.Recorder}, Metrics: observability.Hub{},
		Config:     command.HubConfig{Capacity: cfg.Capacity, TickHz: cfg.TickHz, Grace: cfg.Grace, IdleClose: cfg.IdleClose, EmoteRate: cfg.EmoteRate},
		PartyRooms: persistence.Activities{}}
	host.SetApp(hub, hub.LoadArea)

	presence := cache.PartyPresence{C: vk, TTL: partyCfg.PresenceTTL}
	lobbies := cache.LobbyStore{C: vk}
	parties := command.Parties{UoW: core.UoW, AfterCommit: entx.AfterCommit, Repo: persistence.Parties{}, Directory: persistence.PartyDirectory{},
		Events: persistence.Publisher{W: core.Writer}, Audit: persistence.Auditor{R: core.Recorder}, Moderator: m.moderator,
		Presence: presence, Realtime: realtime.PartyPublisher{P: pub}, Cache: cache.PartyState{C: vk}, Metrics: observability.Party{},
		Config: partyCfg.UseCase,
		Lobby:  lobbies, Activities: persistence.Activities{}, Areas: persistence.Areas{}, Game: m.game, Statuses: presence,
		LobbyMetrics: observability.Lobby{}, LobbyConfig: lobbyCfg, Sweeps: persistence.Sweeps{}, Locks: lobbies}

	// F104 — matchmaking (tickets and proposals in Valkey, matches and duels in PostgreSQL).
	mmStore := cache.MatchmakingStore{C: vk}
	matchmaking := command.Matchmaking{UoW: core.UoW, AfterCommit: entx.AfterCommit, Repo: persistence.Matchmaking{},
		Directory: persistence.PartyDirectory{}, Parties: persistence.Parties{}, Events: persistence.Publisher{W: core.Writer},
		Audit: persistence.Auditor{R: core.Recorder}, Store: mmStore, Realtime: realtime.PartyPublisher{P: pub}, Presence: presence,
		Metrics: observability.Matchmaking{}, Settings: mmCfg.Settings, Configs: command.NewConfigCache(mmCfg.Settings.ConfigTTL),
		Tracker: command.NewTicketTracker()}

	// F105 — battle engine: match rooms hosted by core-api (lease + checkpoint in Valkey), results in PostgreSQL.
	btCfg, err := hubconfig.BattleFrom(r.Config())
	if err != nil {
		return nil, fmt.Errorf("competition: %w", err)
	}
	observability.RegisterBattle(r.Metrics())
	m.deadlines = modules.LearningDeadlines{Plan: btCfg.PlanDeadline, Score: btCfg.ScoreDeadline}
	battles := command.Battles{UoW: core.UoW, Repo: persistence.Battle{}, Plans: m.learning, Cards: persistence.PartyDirectory{},
		Events: persistence.Publisher{W: core.Writer}, Audit: persistence.Auditor{R: core.Recorder}, Metrics: observability.Battle{},
		Instance: btCfg.InstanceID}
	battleHost := battlehost.New(battlehost.Config{InstanceID: btCfg.InstanceID, TickHz: btCfg.TickHz, CheckpointEvery: btCfg.Checkpoint,
		ClientOnly: !r.ServesHTTP()}, r.NATS(), cache.BattleStore{C: vk}, realtime.BattleOutput{P: pub}, battles, nil)

	if r.ServesHTTP() {
		h := competitionhttp.Handlers{Commands: parties, Queries: query.Parties{Read: core.ReadSession, Reader: persistence.Parties{},
			Presence: presence, Lobby: lobbies, LobbyReader: persistence.LobbyRead{}, Game: m.game, PrivateCapacity: lobbyCfg.PrivateCapacity},
			Matchmaking: matchmaking, MatchmakingQueries: query.Matchmaking{Read: core.ReadSession, Reader: persistence.Matchmaking{}, Store: mmStore},
			Battles: query.Battles{Read: core.ReadSession, Reader: persistence.BattleRead{}, Plans: m.learning}}
		r.HTTP(Name, func(s *httpx.Server) error { return competitionhttp.Register(s, h) })
	}
	// Room host, realtime commands (F091 US1, US2) and the party presence store (F100): core-api.
	if nc := r.NATS(); nc != nil && r.ServesHTTP() {
		r.Go("competition.roomhost", host.Run)
		r.Go("competition.cmd.hub_instance_join", func(ctx context.Context) error {
			return rt.ServeCommand(ctx, nc, natscmd.Module, natscmd.TypeInstanceJoin, natscmd.InstanceJoin(hub), 0)
		})
		r.Go("competition.cmd.hub_player_emote", func(ctx context.Context) error {
			return rt.ServeCommand(ctx, nc, natscmd.Module, natscmd.TypePlayerEmote, natscmd.PlayerEmote(hub), 0)
		})
		r.Go("competition.party_presence", func(ctx context.Context) error { return subscriber.PartyPresence(ctx, nc, presence) })
		// Party Lobby (F101) and matchmaking (F104) commands of the realtime gateway: the same use cases as REST.
		for typ, h := range map[string]rt.CommandHandler{natscmd.TypeReadySet: natscmd.ReadySet(parties),
			natscmd.TypeActivitySelect: natscmd.ActivitySelect(parties), natscmd.TypeSessionStart: natscmd.SessionStart(parties),
			natscmd.TypeTicketCancel: natscmd.TicketCancel(matchmaking), natscmd.TypeProposalAccept: natscmd.ProposalAccept(matchmaking),
			natscmd.TypeProposalDecline: natscmd.ProposalDecline(matchmaking)} {
			r.Go("competition.cmd."+typ, func(ctx context.Context) error { return rt.ServeCommand(ctx, nc, natscmd.Module, typ, h, 0) })
		}
		// Match rooms (F105): the battle room host and the battle/co-op commands of the realtime gateway.
		r.Go("competition.battlehost", battleHost.Run)
		bm := observability.Battle{}
		for typ, h := range map[string]rt.CommandHandler{natscmd.TypeBattleAnswer: natscmd.BattleAnswer(battleHost, bm),
			natscmd.TypeBattleState: natscmd.BattleState(battleHost, bm), natscmd.TypeCoopAnswer: natscmd.CoopAnswer(battleHost, bm),
			natscmd.TypeCoopAction: natscmd.CoopAction(battleHost, bm)} {
			r.Go("competition.cmd."+typ, func(ctx context.Context) error { return rt.ServeCommand(ctx, nc, natscmd.Module, typ, h, 0) })
		}
	}

	// Outbox relay (worker events): EVT_COMPETITION (party.*, audit.recorded).
	if err := r.Relay(Name); err != nil {
		return nil, fmt.Errorf("competition: %w", err)
	}
	projection := command.Projection{Refs: persistence.Refs{}}
	r.Consumer("competition.projection_identity", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.IdentityProjectionOptions(consumer.IdentityProjection{P: projection}, core.UoW, core.App, js)
	})
	r.Consumer("competition.projection_game", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.GameProjectionOptions(consumer.GameProjection{P: projection, Hub: hub}, core.UoW, core.App, js)
	})
	r.Consumer("competition.projection_social", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.SocialProjectionOptions(consumer.SocialProjection{P: projection}, core.UoW, core.App, js)
	})
	r.Consumer("competition.hub_access_revoked", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.HubAccessRevokedOptions(consumer.HubAccessRevoked{Hub: hub}, core.UoW, core.App, js)
	})
	m.facade = facade.Facade{Lifecycle: command.PartyLifecycle{Parties: parties}, Hub: &query.HubPosition{Rooms: reg, Positions: reg}}
	safety := command.PartySafety{Parties: parties, Sanctions: persistence.Sanctions{}}
	r.Consumer("competition.party_safety_chat", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.PartySafetyChatOptions(consumer.PartySafetyChat{S: safety}, core.UoW, core.App, js)
	})
	r.Consumer("competition.party_safety_identity", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.PartySafetyIdentityOptions(consumer.PartySafetyIdentity{S: safety}, core.UoW, core.App, js)
	})
	r.Consumer("competition.party_activity_end", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.PartyActivityEndOptions(consumer.PartyActivityEnd{H: parties}, core.UoW, core.App, js)
	})
	r.Job(Name, job.CloseIdleHubInstances, job.CloseIdleInterval, module.JobFunc(job.CloseIdleHubInstancesJob(hub)))
	expire := func(ctx context.Context, batch int) (int, error) {
		return parties.ExpireInvitations(ctx, persistence.Expiry{}, batch)
	}
	r.Job(Name, job.PartyInvitationExpiry, partyCfg.ExpiryInterval, module.JobFunc(job.PartyInvitationExpiryJob(expire)))
	r.Consumer("competition.matchmaking_party", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.MatchmakingPartyOptions(consumer.MatchmakingParty{H: matchmaking}, core.UoW, core.App, js)
	})
	r.Consumer("competition.battle_open", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.BattleOpenOptions(consumer.BattleOpen{R: battleHost}, core.UoW, core.App, js)
	})
	r.Job(Name, job.Matchmaker, mmCfg.Tick, module.JobFunc(job.MatchmakingJob(matchmaking.Tick)))
	r.Job(Name, job.MatchmakingTicketExpiry, mmCfg.Tick, module.JobFunc(job.MatchmakingJob(matchmaking.ExpireTickets)))
	expireDuels := func(ctx context.Context) (int, error) {
		return matchmaking.ExpireDuels(ctx, persistence.Matchmaking{}, 500)
	}
	r.Job(Name, job.DuelExpiry, job.DuelExpiryInterval, module.JobFunc(job.MatchmakingJob(expireDuels)))
	r.Job(Name, job.PartyPresenceSweeper, job.SweepInterval,
		module.JobFunc(job.PartyPresenceSweeperJob(command.NewSweeper(parties, job.SweepInterval))))
	return m, nil
}
