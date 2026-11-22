// Package chat is the composition entry of module chat (ADR-0074 §1).
// cmd/<binary> may only call Register (and later Facade/Wire); everything else lives in internal/.
package chat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/valkey-io/valkey-go"

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
	purgestore "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/purge"
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
	observability.RegisterSafety(r.Metrics())
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
		Friends: friends, Metrics: observability.Social{}, Holds: persistence.Safety{}, Config: cfg.UseCase}
	// Platform Safety Team (F121): reports, cases, sanctions, school notifications; cross-tenant reads only in query.Safety
	// through tenancy.Bypass(moderation_case) (ADR-0072).
	safetyCmd := command.Safety{UoW: core.UoW, Store: persistence.Safety{}, Events: persistence.Publisher{W: core.Writer},
		Audit: persistence.Auditor{R: core.Recorder}, Cache: cache.Sanctions{C: vk}, Push: realtime.ChatFanout{P: rt.Publisher{C: vk}},
		Recent: recent, Metrics: observability.Safety{}, Lock: cache.JobLock{C: vk}, AfterCommit: entx.AfterCommit}
	safetyReads := query.Safety{Read: core.ReadSession, Reader: persistence.Safety{}, Audit: core.Recorder}

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
		safetyReads.Cursors = cursors
		catalogReads := query.Catalog{Read: core.ReadSession, Reader: persistence.Catalog{}, Moderation: persistence.ModerationReader{},
			Engine: pipe.Validator.Engine, Audit: core.Recorder, Cursors: cursors}
		h := chathttp.Handlers{Commands: social, Queries: query.Social{Read: core.ReadSession, Relations: persistence.Reader{},
			Presence: presence, Cursors: cursors}, Chat: &chathttp.ChatHandlers{Send: send, Queries: reads,
			Quick: query.QuickMessages{Catalog: pipe.Quick}}, Catalog: &chathttp.CatalogHandlers{Commands: catalogAdmin, Queries: catalogReads},
			Switches: &chathttp.SwitchHandlers{Commands: switchAdmin, Queries: query.Switches{Read: core.ReadSession, Reader: persistence.Switches{},
				Moderation: persistence.ModerationReader{}, Cursors: cursors}},
			Safety: &chathttp.SafetyHandlers{Commands: safetyCmd, Queries: safetyReads}}
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
		// chat.receipt.read (F117-AC-13): the read cursor of a member, coalesced ≤ 1 durable write per 2 s, then
		// chat.unread.updated to every device of the member.
		if nc := r.NATS(); nc != nil {
			receipts := command.Receipts{UoW: core.UoW, Facts: facts, Repo: persistence.Gameplay{}, Buffer: cache.Cursor{C: vk}, Recent: recent,
				Push: fanout}
			r.Go("chat.receipts", func(ctx context.Context) error { return natscmd.ServeReceipts(ctx, nc, receipts) })
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
	purgeCfg, err := chatconfig.PurgeFrom(r.Config())
	if err != nil {
		return nil, fmt.Errorf("chat: %w", err)
	}
	observability.RegisterGameplay(r.Metrics())
	// gameplay-chat-purge + chat-partition-maintainer (worker-chat, also worker all; ADR-0033, F117 US4): the ONLY physical
	// delete of the platform, on the pool of the role chat_purger (EW_DB_CHAT_PURGER_DSN / _URL_FILE), never in another
	// profile or binary. Without that DSN the jobs do not run and eduworld_chat_purge_blocked{reason="purger_not_configured"}
	// = 1 raises the alert (fail-closed: data kept longer, never deleted by another role).
	if err := registerPurge(r, runsChat, purgeCfg, vk, factsCache); err != nil {
		return nil, err
	}
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
	// chat.gameplay_channels (F117 US1): party.* and match.* of module competition open, fill and close the always-on
	// gameplay channels (deterministic ids of pkg/chatid; purge_after fixed at closing from the tenant retention).
	gameplay := command.Gameplay{Channels: command.Channels{UoW: core.UoW, AfterCommit: entx.AfterCommit, Repo: persistence.Channels{},
		Audit: persistence.Auditor{R: core.Recorder}, Facts: facts}, Gameplay: persistence.Gameplay{},
		Events: persistence.Publisher{W: core.Writer}, Metrics: observability.Gameplay{}, Seq: cache.Seq{C: vk},
		Config: command.GameplayConfig{DefaultRetentionDays: purgeCfg.GameplayRetentionDays}}
	r.Consumer("chat.gameplay_channels", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.GameplayOptions(consumer.Gameplay{H: gameplay}, core.UoW, core.App, js)
	})
	// chat.moderation_cases (F121-AC-03): chat.message_held opens the case of the held message.
	r.Consumer("chat.moderation_cases", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.MessageHeldOptions(consumer.MessageHeld{H: safetyCmd}, core.UoW, core.App, js)
	})
	expiry := command.Expiry{UoW: core.UoW, Repo: persistence.Expiry{}}
	r.Job(Name, job.ExpireFriendRequests, cfg.ExpiryInterval, module.JobFunc(job.ExpireFriendRequestsJob(expiry)))
	// moderation-switch-expirer (F116-AC-25, AC-32): switches back on at expires_at, pending requests expired after 24 h.
	switchExpiry := command.SwitchExpiry{Admin: switchAdmin, Lock: cache.JobLock{C: vk}}
	r.Job(Name, job.ModerationSwitchExpirer, command.ExpiryTick, module.JobFunc(job.ModerationSwitchExpirerJob(switchExpiry)))
	// Safety Team jobs (F121): SLA escalation + queue gauges, school reminders, sanction expiry (≤ 60 s after ends_at).
	r.Job(Name, job.ModerationSLAEscalator, command.SafetyTick, module.JobFunc(job.SLAEscalatorJob(safetyCmd, observability.Safety{},
		safetyReads.QueueCounts)))
	r.Job(Name, job.SchoolNotificationReminder, command.SafetyTick, module.JobFunc(job.ReminderJob(safetyCmd)))
	r.Job(Name, job.SanctionExpirer, command.SafetyTick, module.JobFunc(job.SanctionExpirerJob(safetyCmd)))
	return mod, nil
}

// registerPurge declares the jobs of worker-chat on the chat_purger pool (active only where runsChat).
func registerPurge(r *module.Registry, runsChat bool, cfg chatconfig.Purge, vk valkey.Client, facts port.Cache) error {
	purgeName, partitionName := Name+"."+job.GameplayChatPurge, Name+"."+job.ChatPartitionMaintainer
	if !runsChat {
		r.Background(Name, module.KindJob, purgeName, false, nil)
		r.Background(Name, module.KindJob, partitionName, false, nil)
		return nil
	}
	db, err := r.SQL(Name, "purger")
	if err != nil {
		if !errors.Is(err, module.ErrDatabaseNotConfigured) {
			return fmt.Errorf("chat: purger pool: %w", err)
		}
		slog.Error("chat: EW_DB_CHAT_PURGER_DSN not configured: gameplay-chat-purge does not run (nothing deleted)", "alert", "P2")
		observability.Purge{}.Blocked("purger_not_configured", true)
		r.Background(Name, module.KindJob, purgeName, false, nil)
		r.Background(Name, module.KindJob, partitionName, false, nil)
		return nil
	}
	w, err := outbox.NewWriter(Name, "purge", nil)
	if err != nil {
		return fmt.Errorf("chat: %w", err)
	}
	rec := outbox.NewAuditRecorder(w, db)
	uow, err := entx.NewUnitOfWork(entx.Options{Module: Name, DBs: entx.DBs{App: db, Platform: db}, Events: w, Audit: rec})
	if err != nil {
		return fmt.Errorf("chat: %w", err)
	}
	purge := command.GameplayPurge{UoW: uow, Store: purgestore.Store{}, Events: persistence.Publisher{W: w},
		Audit: persistence.Auditor{R: rec}, Lock: cache.JobLock{C: vk}, Metrics: observability.Purge{},
		Facts: service.Facts{Cache: facts},
		Config: command.PurgeConfig{EvidenceContext: cfg.EvidenceContext, Batch: cfg.Batch, BacklogAlert: cfg.BacklogAlert,
			EphemeralRetention: cfg.EphemeralRetention, LockTTL: cfg.Interval}}
	partitions := command.PartitionMaintainer{UoW: uow, Store: purgestore.Store{}, Metrics: observability.Purge{},
		Days: cfg.PartitionDaysAhead}
	r.Background(Name, module.KindJob, partitionName, true, job.Every(partitionName, cfg.Interval, partitions))
	r.Background(Name, module.KindJob, purgeName, true, job.Every(purgeName, cfg.Interval, purge))
	return nil
}
