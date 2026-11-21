// Package chat is the composition entry of module chat (ADR-0074 §1).
// cmd/<binary> may only call Register (and later Facade/Wire); everything else lives in internal/.
package chat

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"

	chatapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/cache"
	chatconfig "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/natsstream"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/regression"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/security"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/consumer"
	chatfacade "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/facade"
	chathttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/job"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/natscmd"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/persister"
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

// Facade is the part of the chat facade (api.StrictServerInterface, specs/openapi/modules/chat.internal.v1.yaml)
// implemented: moderateDisplayText (F116, x-allowed-callers: competition — party and team names).
type Facade interface {
	ModerateDisplayText(ctx context.Context, req chatapi.ModerateDisplayTextRequestObject) (chatapi.ModerateDisplayTextResponseObject, error)
}

// Module is the registered module; Facade() serves the facade operations of module chat.
type Module struct {
	facade Facade
}

// Facade returns the in-process facade of module chat (nil in a binary that does not load the moderation pipeline).
func (m *Module) Facade() Facade {
	if m == nil || m.facade == nil {
		return nil
	}
	return m.facade
}

// Register wires the module into the running binary: health checks everywhere; in core-api the social routes (F095),
// the chat routes and the NATS chat commands of the gateway (F115: one send use case, JetStream before "Sent") and the
// presence fan-out subscriber (ew.rt.presence.*, queue group chat.presence_fanout); in worker events the outbox relay
// and the consumers chat.projection_identity and chat.projection_game; in worker jobs the job expire-friend-requests;
// in worker chat (and all) chat-persister (F115); in worker jobs moderation-switch-expirer (F116). Gameplay purge comes
// with F117.
func Register(r *module.Registry) (*Module, error) {
	db, err := r.Database(Name)
	if err != nil {
		return nil, fmt.Errorf("chat: open database: %w", err)
	}
	r.Health().Register("db:"+Name, health.Readiness, health.PostgresCheck(db))
	r.Health().Register("atlas:"+Name, health.Readiness, health.AtlasRevisionCheck(db, migrations.Latest()))
	runsChat := r.Binary() == "worker" && (r.Profile() == "chat" || r.Profile() == "all")
	if !r.ServesHTTP() && !r.RunsEvents() && !r.RunsJobs() && !runsChat {
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
	chatCfg, err := chatconfig.ChatFrom(r.Config())
	if err != nil {
		return nil, fmt.Errorf("chat: %w", err)
	}
	observability.Register(r.Metrics())
	observability.RegisterChat(r.Metrics())
	observability.RegisterModeration(r.Metrics())
	recent := cache.Recent{C: vk, Size: chatCfg.RecentBuffer, TTL: chatCfg.RecentTTL}
	factsCache := cache.Facts{C: vk, StaleTTL: chatCfg.StaleTTL, OnStale: observability.StaleFacts}
	// NFR-SAFE-001, F115-AC-06, F116: the send path (core-api) and worker-chat refuse to start without a real
	// ContentValidator; the real one is the F116 pipeline (compiled catalogue, switches, sanctions, spam, decision log).
	pipe, err := moderation.Real(moderation.Deps{Read: core.ReadSession, Cache: factsCache, Valkey: vk, UoW: core.UoW,
		Events: persistence.Publisher{W: core.Writer}, Audit: persistence.Auditor{R: core.Recorder},
		Push: realtime.ChatFanout{P: rt.Publisher{C: vk}}})
	if err != nil {
		return nil, fmt.Errorf("chat: %w", err)
	}
	var validator port.ContentValidator = pipe.Validator
	mod := &Module{facade: chatfacade.Facade{Moderation: pipe.Validator}}
	if r.ServesHTTP() || runsChat {
		if err := command.RequireValidator(chatCfg.Env, validator); err != nil {
			return nil, err
		}
	}
	facts := service.Facts{Read: core.ReadSession, Reader: persistence.ChatReader{}, Recent: recent,
		Cache: factsCache,
		Config: service.FactsConfig{FactsFresh: chatCfg.FactsFresh, PolicyCache: chatCfg.PolicyCache, CurfewFresh: chatCfg.CurfewFresh,
			PlatformCurfew: chatCfg.PlatformCurfew, CurfewLocation: chatCfg.CurfewLocation}}
	stream := natsstream.StreamOptions{MaxAge: chatCfg.StreamMaxAge, Replicas: chatCfg.StreamReplicas}
	friends := cache.Friends{C: vk, TTL: cache.FriendsTTL}
	presence := cache.Presence{C: vk, TTL: cfg.PresenceTTL}
	// Moderation switches (F116 US3): the gauge eduworld_chat_policy_disabled{tenant,step} of this process follows the
	// stored switches (synced after each change, by the expirer and every 30 s in core-api).
	gauge := &service.SwitchGauge{Read: core.ReadSession, Reader: persistence.Switches{}, Set: observability.PolicyDisabled}
	switchAdmin := command.SwitchAdmin{UoW: core.UoW, AfterCommit: entx.AfterCommit, Repo: persistence.Switches{},
		Reader: persistence.ModerationReader{}, Events: persistence.Publisher{W: core.Writer}, Audit: persistence.Auditor{R: core.Recorder},
		Cache: factsCache, Notifier: cache.Notifier{C: vk}, Gauge: gauge}
	social := command.Social{UoW: core.UoW, AfterCommit: entx.AfterCommit, Repo: persistence.Social{}, Directory: persistence.Directory{},
		Events: persistence.Publisher{W: core.Writer}, Audit: persistence.Auditor{R: core.Recorder}, Codes: security.Codes{},
		Friends: friends, Metrics: observability.Social{}, Config: cfg.UseCase}

	if r.ServesHTTP() {
		// The compiled catalogue must be loaded before the instance takes traffic (F116 edge case).
		r.Health().Register("moderation:"+Name, health.Readiness, pipe.Ready)
		r.Go("chat.moderation", pipe.Run)
		r.Go("chat.switch_gauge", gauge.Run)
	}
	if r.ServesHTTP() {
		cursors, err := r.Cursors()
		if err != nil {
			return nil, fmt.Errorf("chat: %w", err)
		}
		var js jetstream.JetStream
		if r.NATS() != nil {
			if js, err = r.JetStream(); err != nil {
				return nil, fmt.Errorf("chat: %w", err)
			}
		}
		fanout := realtime.ChatFanout{P: rt.Publisher{C: vk}}
		send := command.Send{Facts: facts, Validator: validator, Seq: cache.Seq{C: vk}, Recent: recent, Rate: cache.Rate{C: vk},
			Stream: natsstream.Publisher{JS: js, Timeout: chatCfg.PublishAckTimeout}, Fanout: fanout,
			Dedupe: cache.Dedupe{C: vk, TTL: chatCfg.CmidTTL}, Metrics: observability.Chat{}, UoW: core.UoW,
			Audit: persistence.Auditor{R: core.Recorder}, Config: command.SendConfig{RatePerUser: chatCfg.RatePerUser}}
		reads := query.Chat{Read: core.ReadSession, Reader: persistence.ChatReader{}, Facts: facts, Recent: recent, Fanout: fanout,
			Config: query.ChatConfig{RecentBuffer: chatCfg.RecentBuffer}}
		// Sensitive-term catalogue administration (F116 US2): writes stay drafts until a publish passes the regression
		// suite (embedded placeholder corpus; the operational corpus is a regression.Source outside the repository).
		corpus, err := regression.LoadEmbedded()
		if err != nil {
			return nil, fmt.Errorf("chat: %w", err)
		}
		catalogAdmin := command.CatalogAdmin{UoW: core.UoW, AfterCommit: entx.AfterCommit, Repo: persistence.Catalog{},
			Events: persistence.Publisher{W: core.Writer}, Audit: persistence.Auditor{R: core.Recorder},
			Regression: regression.Suite{Engine: pipe.Validator.Engine, Source: corpus}, Notifier: cache.Notifier{C: vk},
			OnPublished: observability.CatalogVersion}
		catalogReads := query.Catalog{Read: core.ReadSession, Reader: persistence.Catalog{}, Moderation: persistence.ModerationReader{},
			Engine: pipe.Validator.Engine, Audit: core.Recorder, Cursors: cursors}
		h := chathttp.Handlers{Commands: social, Queries: query.Social{Read: core.ReadSession, Relations: persistence.Reader{},
			Presence: presence, Cursors: cursors}, Chat: &chathttp.ChatHandlers{Send: send, Queries: reads,
			Quick: query.QuickMessages{Catalog: pipe.Quick}}, Catalog: &chathttp.CatalogHandlers{Commands: catalogAdmin, Queries: catalogReads},
			Switches: &chathttp.SwitchHandlers{Commands: switchAdmin, Queries: query.Switches{Read: core.ReadSession, Reader: persistence.Switches{},
				Moderation: persistence.ModerationReader{}, Cursors: cursors}}}
		r.HTTP(Name, func(s *httpx.Server) error { return chathttp.Register(s, h) })
		// Chat commands of the gateway (F115): ew.cmd.chat.message.send, channel.join/leave, history.fetch, channel.sync.
		if nc := r.NATS(); nc != nil {
			r.Go("chat.commands", func(ctx context.Context) error {
				if err := natsstream.EnsureStream(ctx, js, stream); err != nil {
					return err
				}
				return natscmd.Serve(ctx, nc, send, reads)
			})
		}
		// Presence fan-out (F095 US4): core-api, queue group chat.presence_fanout (one replica per signal).
		if nc := r.NATS(); nc != nil {
			fanout := command.PresenceFanout{Read: core.ReadSession, Relations: persistence.Reader{}, Store: presence, Cache: friends,
				Realtime: realtime.Publisher{P: rt.Publisher{C: vk}}, Metrics: observability.Social{}}
			r.Go("chat.presence_fanout", func(ctx context.Context) error { return subscriber.PresenceFanout(ctx, nc, fanout) })
		}
	}

	// chat-persister (worker-chat, also worker all): the only writer of chat messages into PostgreSQL (R25, F115 US3).
	persist := command.Persist{UoW: core.UoW, Store: persistence.Messages{}, Events: persistence.Publisher{W: core.Writer}}
	r.Background(Name, module.KindConsumer, persister.Name, runsChat, func(ctx context.Context) error {
		js, err := r.JetStream()
		if err != nil {
			return err
		}
		if err := natsstream.EnsureStream(ctx, js, stream); err != nil {
			return err
		}
		w, err := persister.New(ctx, persister.Options{JS: js, Persist: persist, DB: core.App, Batch: chatCfg.PersisterBatch,
			Wait: chatCfg.PersisterWait, MaxDeliver: chatCfg.MaxDeliver, AckWait: chatCfg.AckWait, Stream: stream})
		if err != nil {
			return err
		}
		return w.Run(ctx)
	})
	if runsChat && !r.RunsEvents() && !r.RunsJobs() {
		// Profile chat: the persister only; the outbox relay of EVT_CHAT runs in worker events.
		if err := r.Relay(Name); err != nil {
			return nil, fmt.Errorf("chat: %w", err)
		}
		return mod, nil
	}

	// Outbox relay (worker events): EVT_CHAT (friend.*, user.*, audit.recorded, chat.message_created).
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
	// moderation-switch-expirer (F116-AC-25, AC-32): switches back on at expires_at, pending requests expired after 24 h.
	switchExpiry := command.SwitchExpiry{Admin: switchAdmin, Lock: cache.JobLock{C: vk}}
	r.Job(Name, job.ModerationSwitchExpirer, command.ExpiryTick, module.JobFunc(job.ModerationSwitchExpirerJob(switchExpiry)))
	return mod, nil
}
