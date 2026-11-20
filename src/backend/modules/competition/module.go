// Package competition is the composition entry of module competition (ADR-0074 §1).
// cmd/<binary> may only call Register (and later Facade/Wire); everything else lives in internal/.
package competition

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/cache"
	hubconfig "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/roomhost"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/job"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/natscmd"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/migrations"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"

	_ "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/ent/runtime" // ent mixin hooks and interceptors (R27)
)

// Name is the module name used for its database (eduworld_competition), env prefix and checks.
const Name = "competition"

// Module is the registered module. Facade() and Wire(Imports) are added when the module exposes or
// consumes facade operations (specs/openapi/modules/*.internal.v1.yaml).
type Module struct{}

// Register wires the module into the running binary (F091): health checks everywhere; in core-api the room host of
// the hub instances and the NATS commands hub.instance.join / hub.player.emote of the realtime gateway; in worker
// events the outbox relay and the consumers competition.projection_identity, competition.projection_game,
// competition.projection_social (relation events of module chat, F095) and competition.hub_access_revoked; in worker jobs
// the job close-idle-hub-instances.
func Register(r *module.Registry) (*Module, error) {
	db, err := r.Database(Name)
	if err != nil {
		return nil, fmt.Errorf("competition: open database: %w", err)
	}
	r.Health().Register("db:"+Name, health.Readiness, health.PostgresCheck(db))
	r.Health().Register("atlas:"+Name, health.Readiness, health.AtlasRevisionCheck(db, migrations.Latest()))
	if !r.ServesHTTP() && !r.RunsEvents() && !r.RunsJobs() {
		return &Module{}, nil
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
	observability.Register(r.Metrics())
	reg := cache.HubRegistry{C: vk}
	pub := rt.Publisher{C: vk}
	host := roomhost.New(roomhost.Config{InstanceID: cfg.InstanceID, TickHz: cfg.TickHz, Grace: cfg.Grace, IdleClose: cfg.IdleClose,
		Silence: cfg.Silence, EmoteRate: cfg.EmoteRate, LeaseTTL: cfg.LeaseTTL, LeaseEvery: cfg.LeaseEvery, WatchEvery: cfg.WatchEvery,
		ClientOnly: !r.ServesHTTP()}, r.NATS(), reg, pub, nil, nil, nil)
	hub := command.Hub{UoW: core.UoW, Directory: persistence.Directory{}, Store: persistence.Store{}, Registry: reg, Rooms: host,
		Presence: realtime.Presence{P: pub}, Audit: persistence.Auditor{R: core.Recorder}, Metrics: observability.Hub{},
		Config: command.HubConfig{Capacity: cfg.Capacity, TickHz: cfg.TickHz, Grace: cfg.Grace, IdleClose: cfg.IdleClose, EmoteRate: cfg.EmoteRate}}
	host.SetApp(hub, hub.LoadArea)

	// Room host and realtime commands (F091 US1, US2): core-api, queue group "competition".
	if nc := r.NATS(); nc != nil && r.ServesHTTP() {
		r.Go("competition.roomhost", host.Run)
		r.Go("competition.cmd.hub_instance_join", func(ctx context.Context) error {
			return rt.ServeCommand(ctx, nc, natscmd.Module, natscmd.TypeInstanceJoin, natscmd.InstanceJoin(hub), 0)
		})
		r.Go("competition.cmd.hub_player_emote", func(ctx context.Context) error {
			return rt.ServeCommand(ctx, nc, natscmd.Module, natscmd.TypePlayerEmote, natscmd.PlayerEmote(hub), 0)
		})
	}

	// Outbox relay (worker events): EVT_COMPETITION (audit.recorded of the hub violations, OQ F010-2).
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
	r.Job(Name, job.CloseIdleHubInstances, job.CloseIdleInterval, module.JobFunc(job.CloseIdleHubInstancesJob(hub)))
	return &Module{}, nil
}
