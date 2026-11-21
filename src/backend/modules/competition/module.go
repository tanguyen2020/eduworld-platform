// Package competition is the composition entry of module competition (ADR-0074 §1).
// cmd/<binary> may only call Register, Facade and Wire; everything else lives in internal/.
package competition

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/cache"
	hubconfig "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/modules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/roomhost"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/consumer"
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

// Imports are the facades module competition calls: moderateDisplayText of module chat (party names, F100-AC-05).
// Chat nil (module chat implements it with F116): every typed name falls back to the generated default name.
type Imports struct {
	Chat modules.ChatModeration
}

// Module is the registered module.
type Module struct {
	moderator *modules.LateModerator
}

// Wire connects the facades of other modules (in process, pkg/modcall).
func (m *Module) Wire(in Imports) {
	if m.moderator != nil {
		m.moderator.Set(in.Chat)
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
	m := &Module{moderator: &modules.LateModerator{}}
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
	observability.Register(r.Metrics())
	observability.RegisterParty(r.Metrics())
	reg := cache.HubRegistry{C: vk}
	pub := rt.Publisher{C: vk}
	host := roomhost.New(roomhost.Config{InstanceID: cfg.InstanceID, TickHz: cfg.TickHz, Grace: cfg.Grace, IdleClose: cfg.IdleClose,
		Silence: cfg.Silence, EmoteRate: cfg.EmoteRate, LeaseTTL: cfg.LeaseTTL, LeaseEvery: cfg.LeaseEvery, WatchEvery: cfg.WatchEvery,
		ClientOnly: !r.ServesHTTP()}, r.NATS(), reg, pub, nil, nil, nil)
	hub := command.Hub{UoW: core.UoW, Directory: persistence.Directory{}, Store: persistence.Store{}, Registry: reg, Rooms: host,
		Presence: realtime.Presence{P: pub}, Audit: persistence.Auditor{R: core.Recorder}, Metrics: observability.Hub{},
		Config: command.HubConfig{Capacity: cfg.Capacity, TickHz: cfg.TickHz, Grace: cfg.Grace, IdleClose: cfg.IdleClose, EmoteRate: cfg.EmoteRate}}
	host.SetApp(hub, hub.LoadArea)

	presence := cache.PartyPresence{C: vk, TTL: partyCfg.PresenceTTL}
	parties := command.Parties{UoW: core.UoW, AfterCommit: entx.AfterCommit, Repo: persistence.Parties{}, Directory: persistence.PartyDirectory{},
		Events: persistence.Publisher{W: core.Writer}, Audit: persistence.Auditor{R: core.Recorder}, Moderator: m.moderator,
		Presence: presence, Realtime: realtime.PartyPublisher{P: pub}, Cache: cache.PartyState{C: vk}, Metrics: observability.Party{},
		Config: partyCfg.UseCase}

	if r.ServesHTTP() {
		h := competitionhttp.Handlers{Commands: parties, Queries: query.Parties{Read: core.ReadSession, Reader: persistence.Parties{},
			Presence: presence}}
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
	safety := command.PartySafety{Parties: parties, Sanctions: persistence.Sanctions{}}
	r.Consumer("competition.party_safety_chat", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.PartySafetyChatOptions(consumer.PartySafetyChat{S: safety}, core.UoW, core.App, js)
	})
	r.Consumer("competition.party_safety_identity", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.PartySafetyIdentityOptions(consumer.PartySafetyIdentity{S: safety}, core.UoW, core.App, js)
	})
	r.Job(Name, job.CloseIdleHubInstances, job.CloseIdleInterval, module.JobFunc(job.CloseIdleHubInstancesJob(hub)))
	expire := func(ctx context.Context, batch int) (int, error) {
		return parties.ExpireInvitations(ctx, persistence.Expiry{}, batch)
	}
	r.Job(Name, job.PartyInvitationExpiry, partyCfg.ExpiryInterval, module.JobFunc(job.PartyInvitationExpiryJob(expire)))
	return m, nil
}
