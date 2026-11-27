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
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/chatasset"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/breakglass"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/cache"
	chatconfig "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/emoji"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/moderation"
	chatmodules "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/modules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/natsstream"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/policyschema"
	purgestore "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/purge"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/regression"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/security"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/storage"
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
// implemented: moderateDisplayText (F116, x-allowed-callers: competition — party and team names) and screenText (F077,
// x-allowed-callers: game — school event texts), the sticker pack assignments (F082) and the trash of the sticker packs
// (listTrash, restoreTrashItem — F057; x-allowed-callers: identity).
type Facade interface {
	// listOutboxEventIds / listProcessedEventIds (worker reconcile, F134).
	ListOutboxEventIds(ctx context.Context, req chatapi.ListOutboxEventIdsRequestObject) (chatapi.ListOutboxEventIdsResponseObject, error)
	ListProcessedEventIds(ctx context.Context, req chatapi.ListProcessedEventIdsRequestObject) (chatapi.ListProcessedEventIdsResponseObject, error)
	ModerateDisplayText(ctx context.Context, req chatapi.ModerateDisplayTextRequestObject) (chatapi.ModerateDisplayTextResponseObject, error)
	ScreenText(ctx context.Context, req chatapi.ScreenTextRequestObject) (chatapi.ScreenTextResponseObject, error)
	// F082 T040: the tenant assignments of the sticker packs (x-allowed-callers: identity).
	ApplyTenantAssignments(ctx context.Context, req chatapi.ApplyTenantAssignmentsRequestObject) (chatapi.ApplyTenantAssignmentsResponseObject, error)
	RevokeTenantAssignments(ctx context.Context, req chatapi.RevokeTenantAssignmentsRequestObject) (chatapi.RevokeTenantAssignmentsResponseObject, error)
	ListResourceAssignments(ctx context.Context, req chatapi.ListResourceAssignmentsRequestObject) (chatapi.ListResourceAssignmentsResponseObject, error)
	// F057: the trash of the sticker packs (x-allowed-callers: identity, ADR-0028).
	ListTrash(ctx context.Context, req chatapi.ListTrashRequestObject) (chatapi.ListTrashResponseObject, error)
	RestoreTrashItem(ctx context.Context, req chatapi.RestoreTrashItemRequestObject) (chatapi.RestoreTrashItemResponseObject, error)
}

// Module is the registered module; Facade() serves the facade operations of module chat.
type Module struct {
	facade     Facade
	principals *chatmodules.LatePrincipals // F124: resolvePrincipal of the identity facade (on-call moderators)
	learning   *chatmodules.LateLearning   // F123: listAssignmentRecipients of the learning facade (notifications)
}

// Imports are the facades of other modules called by module chat: resolvePrincipal of identity (F124-AC-12, the on-call
// moderator holds moderation.case.act) and listAssignmentRecipients of learning (F123, recipients of the notifications
// of bài giao).
type Imports struct {
	Identity chatmodules.IdentityPrincipals
	Learning chatmodules.LearningRecipients
}

// Wire sets the facades of the other modules once they are built (cmd/internal/wiring.Compose).
func (m *Module) Wire(in Imports) {
	if m == nil {
		return
	}
	if m.principals != nil && in.Identity != nil {
		m.principals.Set(in.Identity)
	}
	if m.learning != nil && in.Learning != nil {
		m.learning.Set(in.Learning)
	}
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
	if !r.ServesHTTP() && !r.RunsEvents() && !r.RunsJobs() && !r.Reconciles() && !runsChat {
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
	observability.RegisterFeatures(r.Metrics())
	observability.RegisterPersistent(r.Metrics())
	persistentCfg, err := chatconfig.PersistentFrom(r.Config())
	if err != nil {
		return nil, fmt.Errorf("chat: %w", err)
	}
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
	learning := &chatmodules.LateLearning{}
	mod := &Module{facade: chatfacade.Facade{Ops: r.ReconcileSource(Name), Moderation: pipe.Validator}, principals: &chatmodules.LatePrincipals{}, learning: learning}
	opsCfg, err := chatconfig.SafetyOpsFrom(r.Config())
	if err != nil {
		return nil, fmt.Errorf("chat: %w", err)
	}
	observability.RegisterSafetyOps(r.Metrics())
	// In-game notifications (F123): stored in PostgreSQL, pushed after the commit on ew:rt:user:<id>, unread count cached
	// in Valkey; never a push outside the game (FR-NTF-003).
	observability.RegisterNotifications(r.Metrics())
	unread := cache.Unread{C: vk, TTL: cache.UnreadTTL}
	notifications := command.Notifications{UoW: core.UoW, AfterCommit: entx.AfterCommit, Store: persistence.Notifications{},
		Push: realtime.Notifications{P: rt.Publisher{C: vk}}, Unread: unread, Recipients: learning, Metrics: observability.Notifications{}}
	if r.ServesHTTP() || runsChat {
		if err := command.RequireValidator(chatCfg.Env, validator); err != nil {
			return nil, err
		}
	}
	facts := service.Facts{Read: core.ReadSession, Reader: persistence.ChatReader{}, Recent: recent,
		Cache: factsCache,
		Config: service.FactsConfig{FactsFresh: chatCfg.FactsFresh, PolicyCache: chatCfg.PolicyCache, CurfewFresh: chatCfg.CurfewFresh,
			PlatformCurfew: chatCfg.PlatformCurfew, CurfewLocation: chatCfg.CurfewLocation}, Gens: cache.PolicyGens{C: vk}}
	// F120: the implicit World/Area channels, the persistent channels (direct, club team), the policy levels and the clubs.
	rooms := cache.Rooms{C: vk, TTL: cfg.PresenceTTL}
	implicit := &service.Implicit{Reader: persistence.ChatReader{}, Policies: persistence.Policies{}, Chat: persistence.ChatReader{},
		Rooms: rooms, Cache: factsCache, Facts: facts}
	chatPush := realtime.ChatFanout{P: rt.Publisher{C: vk}}
	persistentCmd := command.Persistent{UoW: core.UoW, AfterCommit: entx.AfterCommit, Repo: persistence.Channels{},
		Status: persistence.Channels{}, Social: persistence.Social{}, Directory: persistence.Directory{},
		Audit: persistence.Auditor{R: core.Recorder}, Facts: facts, Implicit: implicit, Push: chatPush}
	clubCmd := command.Clubs{UoW: core.UoW, AfterCommit: entx.AfterCommit, Store: persistence.Clubs{}, Social: persistence.Social{},
		Directory: persistence.Directory{}, Channels: persistence.Channels{}, Status: persistence.Channels{}, Names: pipe.Validator,
		Events: persistence.Publisher{W: core.Writer}, Audit: persistence.Auditor{R: core.Recorder}, Facts: facts, Push: chatPush,
		Limits: persistentCfg.Clubs}
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
		Friends: friends, Relations: implicit, Metrics: observability.Social{}, Holds: persistence.Safety{}, Config: cfg.UseCase}
	// Platform Safety Team (F121): reports, cases, sanctions, school notifications; cross-tenant reads only in query.Safety
	// through tenancy.Bypass(moderation_case) (ADR-0072).
	safetyCmd := command.Safety{UoW: core.UoW, Store: persistence.Safety{}, Events: persistence.Publisher{W: core.Writer},
		Audit: persistence.Auditor{R: core.Recorder}, Cache: cache.Sanctions{C: vk}, Push: realtime.ChatFanout{P: rt.Publisher{C: vk}},
		Recent: recent, Metrics: observability.Safety{}, Lock: cache.JobLock{C: vk}, AfterCommit: entx.AfterCommit,
		// F124: on-call rotation of p1 cases and break-glass of the System Admin.
		Oncall: persistence.SafetyOps{}, Principals: mod.principals,
		OncallCfg:     command.OncallConfig{ShiftMax: opsCfg.ShiftMax, Ladder: command.DefaultOncallLadder()},
		OncallMetrics: observability.SafetyOps{}, Grants: persistence.SafetyOps{},
		BreakGlassLimits:  breakglass.Limits{MaxDuration: opsCfg.BreakGlassDuration, MaxWindow: opsCfg.BreakGlassWindow},
		BreakGlassMetrics: observability.SafetyOps{}}
	safetyReads := query.Safety{Read: core.ReadSession, Reader: persistence.Safety{}, Audit: core.Recorder}
	opsReads := query.SafetyOps{Read: core.ReadSession, Oncall: persistence.SafetyOps{}, Grants: persistence.SafetyOps{}}

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
		// F082 T040: tenant assignments of the sticker packs, called by module identity (platform operations, ADR-0029).
		mod.facade = chatfacade.Facade{Ops: r.ReconcileSource(Name), Moderation: pipe.Validator, Assignments: command.ChatAssignments{UoW: core.UoW, Store: persistence.Assignments{},
			Audit: persistence.Auditor{R: core.Recorder}, Cursors: cursors},
			// F057: the trash of the sticker packs (soft-deleted platform library rows, restore under tenancy.Bypass(trash)).
			Trash: chatfacade.Trash{TrashList: query.Trash{Read: core.ReadSession, Reader: persistence.Trash{}, Cursors: cursors},
				TrashRestore: command.RestoreTrash{UoW: core.UoW, Store: persistence.Trash{}, Events: persistence.Publisher{W: core.Writer},
					Audit: persistence.Auditor{R: core.Recorder}}, Idempotency: core.Idempotency}}
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
			Audit: persistence.Auditor{R: core.Recorder}, Config: command.SendConfig{RatePerUser: chatCfg.RatePerUser},
			Implicit: implicit, Ensure: persistentCmd.EnsureImplicit, Broadcast: fanout, HoursLoc: chatCfg.CurfewLocation}
		// F118: message features (edit/recall/forward/reactions/pins/typing/receipts) and the sticker/GIF library.
		assetsCfg, err := chatconfig.AssetsFrom(r.Config())
		if err != nil {
			return nil, fmt.Errorf("chat: %w", err)
		}
		emojiSet, err := emoji.Load()
		if err != nil {
			return nil, fmt.Errorf("chat: %w", err)
		}
		state := cache.FeatureState{C: vk}
		lookup := service.Lookup{Read: core.ReadSession, Reader: persistence.Features{}, Recent: recent, Index: state}
		send.Lookup, send.Assets, send.Emoji, send.AssetRate, send.Index, send.Notify = &lookup,
			service.Assets{Read: core.ReadSession, Reader: persistence.Features{}}, emojiSet, state, state, fanout
		send.Config.StickerGifPerMinute = assetsCfg.StickerGifPerMinute
		messaging := command.Messaging{Send: send, Lookup: lookup, Features: persistence.Features{},
			Changes: natsstream.Publisher{JS: js, Timeout: chatCfg.PublishAckTimeout}, Gate: state, Reactions: state, Pins: state,
			Typing: state, Fanout: fanout, Emoji: emojiSet, Metrics: observability.Features{}}
		// Game Studio uploads (F118-AC-17): the S3-compatible bucket EW_CHAT_ASSET_BUCKET when EW_S3_* is configured;
		// without it presign and the file checks answer 503 (nothing is published unchecked).
		var objects port.AssetStorage
		if r.Config().String("EW_S3_ENDPOINT") != "" && r.Config().String("EW_S3_ACCESS_KEY") != "" {
			s3, err := storage.New(storage.Config{Endpoint: r.Config().String("EW_S3_ENDPOINT"), Region: r.Config().String("EW_S3_REGION"),
				AccessKey: r.Config().String("EW_S3_ACCESS_KEY"), SecretKey: r.Config().String("EW_S3_SECRET_KEY"), Bucket: assetsCfg.Bucket,
				Timeout: r.Config().Duration("EW_S3_TIMEOUT", 0)}, nil)
			if err != nil {
				return nil, fmt.Errorf("chat: asset storage: %w", err)
			}
			objects = s3
		}
		featureReads := query.Features{Read: core.ReadSession, Reader: persistence.Features{}, Facts: facts, Lookup: lookup, Recent: recent}
		reads := query.Chat{Read: core.ReadSession, Reader: persistence.ChatReader{}, Facts: facts, Recent: recent, Fanout: fanout,
			Config: query.ChatConfig{RecentBuffer: chatCfg.RecentBuffer}}
		reads.Decor = &featureReads
		reads.Implicit = implicit
		reads.Sanctions = pipe.Validator.Sanctions // Channel.sendable_kinds and features (delta F119)
		policyAdmin := command.PolicyAdmin{UoW: core.UoW, AfterCommit: entx.AfterCommit, Store: persistence.Policies{},
			Schema: policyschema.Validator{}, Events: persistence.Publisher{W: core.Writer}, Audit: persistence.Auditor{R: core.Recorder},
			Gens: cache.PolicyGens{C: vk}, Curfew: facts.Curfew, PlatformCurfew: policy.Curfew{chatCfg.PlatformCurfew},
			Rejected: observability.Persistent{}.PolicyRejected}
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
		opsReads.Cursors = cursors
		catalogReads := query.Catalog{Read: core.ReadSession, Reader: persistence.Catalog{}, Moderation: persistence.ModerationReader{},
			Engine: pipe.Validator.Engine, Audit: core.Recorder, Cursors: cursors}
		h := chathttp.Handlers{Commands: social, Queries: query.Social{Read: core.ReadSession, Relations: persistence.Reader{},
			Presence: presence, Cursors: cursors}, Chat: &chathttp.ChatHandlers{Send: send, Queries: reads,
			Quick: query.QuickMessages{Catalog: pipe.Quick}}, Catalog: &chathttp.CatalogHandlers{Commands: catalogAdmin, Queries: catalogReads},
			Switches: &chathttp.SwitchHandlers{Commands: switchAdmin, Queries: query.Switches{Read: core.ReadSession, Reader: persistence.Switches{},
				Moderation: persistence.ModerationReader{}, Cursors: cursors}},
			Safety:    &chathttp.SafetyHandlers{Commands: safetyCmd, Queries: safetyReads},
			SafetyOps: &chathttp.SafetyOpsHandlers{Commands: safetyCmd, Queries: opsReads},
			Features:  &chathttp.FeatureHandlers{Commands: messaging, Queries: featureReads, AssetURL: assetsCfg.URL, Recorder: send.RecordResult},
			Assets: &chathttp.AssetHandlers{Commands: command.ChatAssets{UoW: core.UoW, Repo: persistence.Assets{}, Storage: objects,
				Inspector: storage.Inspector{Storage: objects}, Events: persistence.Publisher{W: core.Writer}, Audit: persistence.Auditor{R: core.Recorder},
				Metrics: observability.Features{}, Limits: chatasset.Limits{StickerMaxBytes: assetsCfg.StickerMaxBytes, GifMaxBytes: assetsCfg.GifMaxBytes,
					GifMaxPixels: assetsCfg.GifMaxPixels}},
				Queries: query.ChatAssets{Read: core.ReadSession, Reader: persistence.Assets{}, Cursors: cursors, Storage: objects}},
			Policies: &chathttp.PolicyHandlers{Commands: policyAdmin, Queries: query.Policies{Read: core.ReadSession, Reader: persistence.Policies{},
				Chat: persistence.ChatReader{}}},
			Persistent: &chathttp.PersistentHandlers{Direct: persistentCmd, Search: query.Search{Read: core.ReadSession, Reader: persistence.Search{},
				Cursors: cursors}, Caps: reads.Caps, Peer: reads.Peer,
				// delta F112-9: updateChannelMembership (notification level of the caller, audited).
				Settings: command.ChannelSettings{UoW: core.UoW, Facts: facts, Repo: persistence.Gameplay{},
					Audit: persistence.Auditor{R: core.Recorder}}},
			Clubs: &chathttp.ClubHandlers{Commands: clubCmd, Queries: query.Clubs{Read: core.ReadSession, Reader: persistence.ClubReads{},
				Directory: persistence.Directory{}, Cursors: cursors}, WeekLoc: chatCfg.CurfewLocation},
			Notifications: &chathttp.NotificationHandlers{Commands: notifications, Queries: query.Notifications{Read: core.ReadSession,
				Reader: persistence.NotificationReader{}, Unread: unread, Cursors: cursors}}}
		r.HTTP(Name, func(s *httpx.Server) error { return chathttp.Register(s, h) })
		// Chat commands of the gateway (F115): ew.cmd.chat.message.send, channel.join/leave, history.fetch, channel.sync.
		if nc := r.NATS(); nc != nil {
			r.Go("chat.commands", func(ctx context.Context) error {
				if err := natsstream.EnsureStream(ctx, js, stream); err != nil {
					return err
				}
				return natscmd.Serve(ctx, nc, send, reads)
			})
			// notification.item.mark_read of the gateway (F123-AC-09).
			r.Go("chat.notification_commands", func(ctx context.Context) error { return natscmd.ServeNotifications(ctx, nc, notifications) })
		}
		// chat.receipt.read (F117-AC-13): the read cursor of a member, coalesced ≤ 1 durable write per 2 s, then
		// chat.unread.updated to every device of the member.
		if nc := r.NATS(); nc != nil {
			receipts := command.Receipts{UoW: core.UoW, Facts: facts, Repo: persistence.Gameplay{}, Buffer: cache.Cursor{C: vk}, Recent: recent,
				Push: fanout, Cursors: persistence.Gameplay{}, Delivered: cache.Cursor{C: vk, Kind: "delivered:"}, Mentions: fanout, Fanout: fanout}
			r.Go("chat.receipts", func(ctx context.Context) error { return natscmd.ServeReceipts(ctx, nc, receipts) })
			r.Go("chat.features", func(ctx context.Context) error { return natscmd.ServeFeatures(ctx, nc, messaging, receipts) })
		}
		// Presence fan-out (F095 US4): core-api, queue group chat.presence_fanout (one replica per signal).
		if nc := r.NATS(); nc != nil {
			fanout := command.PresenceFanout{Read: core.ReadSession, Relations: persistence.Reader{}, Store: presence, Cache: friends,
				Realtime: realtime.Publisher{P: rt.Publisher{C: vk}}, Metrics: observability.Social{}, Rooms: rooms}
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
	// persister-evt (worker-chat, F118): the message changes (edit, recall, reactions, pins) applied by revision (R25).
	applyChange := command.ApplyChange{UoW: core.UoW, Store: persistence.Changes{}, Events: persistence.Publisher{W: core.Writer}}
	r.Background(Name, module.KindConsumer, persister.ChangesName, runsChat, func(ctx context.Context) error {
		js, err := r.JetStream()
		if err != nil {
			return err
		}
		if err := natsstream.EnsureStream(ctx, js, stream); err != nil {
			return err
		}
		w, err := persister.NewChanges(ctx, persister.Options{JS: js, DB: core.App, Batch: chatCfg.PersisterBatch, Wait: chatCfg.PersisterWait,
			MaxDeliver: chatCfg.MaxDeliver, AckWait: chatCfg.AckWait, Stream: stream}, applyChange)
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
	// persistent-chat-retention (worker-chat, also worker all; F120 US6): anonymisation of the persistent channels on the
	// pool of chat_app (UPDATE only, R21) — never chat_purger.
	retentionName := Name + "." + job.PersistentChatRetention
	retention := command.PersistentRetention{UoW: core.UoW, Store: persistence.Retention{}, Audit: persistence.Auditor{R: core.Recorder},
		Lock: cache.JobLock{C: vk}, Metrics: observability.Persistent{}, Config: command.RetentionConfig{Days: persistentCfg.RetentionDays,
			Batch: persistentCfg.RetentionBatch, EvidenceContext: purgeCfg.EvidenceContext}}
	r.Background(Name, module.KindJob, retentionName, runsChat, job.Every(retentionName, persistentCfg.RetentionInterval, retention))
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
	// chat.membership (F120-AC-15): user.blocked / user.unblocked freeze / reopen the direct channel of the pair.
	r.Consumer(messagingMembership, func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.MembershipOptions(consumer.Membership{H: persistentCmd}, core.UoW, core.App, js)
	})
	// chat.club_goals_* (F120-AC-21): verified learning/game events count the weekly club goals.
	goals := consumer.ClubGoals{H: clubCmd, Loc: chatCfg.CurfewLocation}
	r.Consumer(messagingClubGoalsLearning, func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.ClubGoalsLearningOptions(goals, core.UoW, core.App, js)
	})
	r.Consumer(messagingClubGoalsGame, func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.ClubGoalsGameOptions(goals, core.UoW, core.App, js)
	})
	// chat.moderation_cases (F121-AC-03): chat.message_held opens the case of the held message.
	r.Consumer("chat.moderation_cases", func(js jetstream.JetStream) outbox.ConsumerOptions {
		return consumer.MessageHeldOptions(consumer.MessageHeld{H: safetyCmd}, core.UoW, core.App, js)
	})
	// In-game notifications (F123 T001): one durable consumer per producer stream, with the other domain consumers
	// (worker events, where every module is composed and the learning facade is wired — Clarifications 2026-11-21).
	for _, name := range consumer.NotifierNames {
		r.Consumer(name, func(js jetstream.JetStream) outbox.ConsumerOptions {
			return consumer.NotifierOptions(name, consumer.Notifier{H: notifications}, consumer.NotifierConfig{}, core.UoW, core.App, js)
		})
	}
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
	// Safety Team operations (F124): on-call ladder of open p1 cases + coverage gauge, break-glass expiry.
	r.Job(Name, job.OncallEscalator, job.SafetyOpsTick, module.JobFunc(job.OncallEscalatorJob(safetyCmd)))
	r.Job(Name, job.BreakGlassExpirer, job.SafetyOpsTick, module.JobFunc(job.BreakGlassExpirerJob(safetyCmd)))
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

// Durable consumer names of F120 (internal/infrastructure/messaging).
const (
	messagingMembership        = "chat.membership"
	messagingClubGoalsLearning = "chat.club_goals_learning"
	messagingClubGoalsGame     = "chat.club_goals_game"
)
