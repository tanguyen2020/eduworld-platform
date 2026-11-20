// Package chat is the composition entry of module chat (ADR-0074 §1).
// cmd/<binary> may only call Register (and later Facade/Wire); everything else lives in internal/.
package chat

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/cache"
	chatconfig "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/security"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/consumer"
	chathttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/job"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/subscriber"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/migrations"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"

	_ "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/ent/runtime" // ent mixin hooks and interceptors (R27)
)

// Name is the module name used for its database (eduworld_chat), env prefix and checks.
const Name = "chat"

// Module is the registered module. Facade() and Wire(Imports) are added when the module exposes or
// consumes facade operations (specs/openapi/modules/*.internal.v1.yaml).
type Module struct{}

// Register wires the module into the running binary (F095): health checks everywhere; in core-api the social routes
// (friends, friend requests and codes, blocks, mutes) and the presence fan-out subscriber of the gateway signals
// (ew.rt.presence.*, queue group chat.presence_fanout); in worker events the outbox relay and the consumers
// chat.projection_identity and chat.projection_game; in worker jobs the job expire-friend-requests. Profile chat (chat
// persister, gameplay purge) gets its entry points with F115/F117.
func Register(r *module.Registry) (*Module, error) {
	db, err := r.Database(Name)
	if err != nil {
		return nil, fmt.Errorf("chat: open database: %w", err)
	}
	r.Health().Register("db:"+Name, health.Readiness, health.PostgresCheck(db))
	r.Health().Register("atlas:"+Name, health.Readiness, health.AtlasRevisionCheck(db, migrations.Latest()))
	if !r.ServesHTTP() && !r.RunsEvents() && !r.RunsJobs() {
		// Outbox relay declared (inactive here): EVT_CHAT also carries the identity audit sink (OQ F010-2).
		if err := r.Relay(Name); err != nil {
			return nil, fmt.Errorf("chat: %w", err)
		}
		return &Module{}, nil
	}
	core, err := r.Core(Name)
	if err != nil {
		return nil, fmt.Errorf("chat: %w", err)
	}
	cfg, err := chatconfig.SocialFrom(r.Config())
	if err != nil {
		return nil, fmt.Errorf("chat: %w", err)
	}
	vk, err := r.Valkey()
	if err != nil {
		return nil, fmt.Errorf("chat: %w", err)
	}
	observability.Register(r.Metrics())
	friends := cache.Friends{C: vk, TTL: cache.FriendsTTL}
	presence := cache.Presence{C: vk, TTL: cfg.PresenceTTL}
	social := command.Social{UoW: core.UoW, AfterCommit: entx.AfterCommit, Repo: persistence.Social{}, Directory: persistence.Directory{},
		Events: persistence.Publisher{W: core.Writer}, Audit: persistence.Auditor{R: core.Recorder}, Codes: security.Codes{},
		Friends: friends, Metrics: observability.Social{}, Config: cfg.UseCase}

	if r.ServesHTTP() {
		cursors, err := r.Cursors()
		if err != nil {
			return nil, fmt.Errorf("chat: %w", err)
		}
		h := chathttp.Handlers{Commands: social, Queries: query.Social{Read: core.ReadSession, Relations: persistence.Reader{},
			Presence: presence, Cursors: cursors}}
		r.HTTP(Name, func(s *httpx.Server) error { return chathttp.Register(s, h) })
		// Presence fan-out (F095 US4): core-api, queue group chat.presence_fanout (one replica per signal).
		if nc := r.NATS(); nc != nil {
			fanout := command.PresenceFanout{Read: core.ReadSession, Relations: persistence.Reader{}, Store: presence, Cache: friends,
				Realtime: realtime.Publisher{P: rt.Publisher{C: vk}}, Metrics: observability.Social{}}
			r.Go("chat.presence_fanout", func(ctx context.Context) error { return subscriber.PresenceFanout(ctx, nc, fanout) })
		}
	}

	// Outbox relay (worker events): EVT_CHAT (friend.*, user.*, audit.recorded).
	if err := r.Relay(Name); err != nil {
		return nil, fmt.Errorf("chat: %w", err)
	}
	projection := command.Projection{Refs: persistence.Refs{}, Social: persistence.Social{}}
	r.Consumer("chat.projection_identity", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.IdentityProjectionOptions(consumer.IdentityProjection{P: projection}, core.UoW, core.App, js)
	})
	r.Consumer("chat.projection_game", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.GameProjectionOptions(consumer.GameProjection{P: projection}, core.UoW, core.App, js)
	})
	expiry := command.Expiry{UoW: core.UoW, Repo: persistence.Expiry{}}
	r.Job(Name, job.ExpireFriendRequests, cfg.ExpiryInterval, module.JobFunc(job.ExpireFriendRequestsJob(expiry)))
	return &Module{}, nil
}
