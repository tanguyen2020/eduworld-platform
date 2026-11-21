// Package moderation wires the ContentValidator of the send path: Real builds the F116 pipeline.Validator on its
// adapters (compiled catalogue store, effective switches, sanctions, spam windows, system mutes, decision log). The
// labelled test double stays for unit tests of the send path only; command.RequireValidator refuses it outside
// EW_ENV=test (F115-AC-06, R17).
package moderation

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/pipeline"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/moderationcfg"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
)

// Deps are the adapters the real pipeline is built on.
type Deps struct {
	Read   service.ReadRunner
	Cache  port.Cache // Valkey read-through cache of the facts (switches)
	Valkey valkey.Client
	UoW    write.UnitOfWork
	Events write.EventPublisher
	Audit  write.Auditor
	Push   command.SanctionPusher // nil: none
	Now    func() time.Time
}

// Pipeline is the wired pipeline and its background parts.
type Pipeline struct {
	Validator *pipeline.Validator
	Catalogs  *service.Catalogs
	Switches  service.Switches
	Decisions *persistence.DecisionWriter
	Quick     *service.QuickMessages
	Mute      command.ApplySystemMute
	vk        valkey.Client
}

// ErrMissingDeps: the pipeline cannot be built without its stores.
var ErrMissingDeps = errors.New("chat: moderation pipeline needs the read session, Valkey, the cache and the unit of work")

// Real builds the real pipeline (F116 pipeline.Validator).
func Real(d Deps) (*Pipeline, error) {
	if d.Read == nil || d.Valkey == nil || d.UoW == nil || d.Cache == nil {
		return nil, ErrMissingDeps
	}
	cfg, err := moderationcfg.Load()
	if err != nil {
		return nil, err
	}
	engine := moderation.NewEngine(cfg)
	reader := persistence.ModerationReader{}
	cats := &service.Catalogs{Read: d.Read, Reader: reader, Engine: engine, Refresh: moderation.PropagationMax / 2, Now: d.Now,
		OnInvalid: observability.InvalidPatterns}
	switches := service.Switches{Read: d.Read, Reader: reader, Cache: d.Cache, Fresh: moderation.PropagationMax / 2, Now: d.Now}
	sanctionCache := cache.Sanctions{C: d.Valkey}
	mute := command.ApplySystemMute{UoW: d.UoW, Repo: persistence.Sanctions{}, Events: d.Events, Audit: d.Audit, Cache: sanctionCache,
		Push: d.Push, Now: d.Now}
	decisions := &persistence.DecisionWriter{UoW: d.UoW, OnDropped: observability.Moderation{}.DecisionDropped}
	quick := &service.QuickMessages{Read: d.Read, Reader: reader}
	v := &pipeline.Validator{Engine: engine, Catalogs: cats, Switches: switches,
		Sanctions: service.Sanctions{Read: d.Read, Reader: reader, Cache: sanctionCache}, Quick: quick, Spam: cache.SpamWindow{C: d.Valkey},
		Muter: mute, Decisions: decisions, Metrics: observability.Moderation{}, Now: d.Now}
	return &Pipeline{Validator: v, Catalogs: cats, Switches: switches, Decisions: decisions, Quick: quick, Mute: mute, vk: d.Valkey}, nil
}

// Ready is the readiness check of the instance: the platform catalogue is compiled (never an empty catalogue).
func (p *Pipeline) Ready(context.Context) error {
	if !p.Catalogs.Ready() {
		return service.ErrCatalogNotLoaded
	}
	return nil
}

// Run runs the background parts until ctx ends: catalogue warm-up and reconcile, the decision log writer and the
// change notifications (catalogue versions, tenant category settings).
func (p *Pipeline) Run(ctx context.Context) error {
	go func() { _ = p.Decisions.Run(ctx) }()
	go func() {
		_ = cache.Subscribe(ctx, p.vk, func(n cache.Notification) {
			switch n.Channel {
			case cache.SensitiveVersionChannel:
				p.Catalogs.Invalidate(ctx, n.TenantID)
			case cache.SwitchesChangedChannel:
				// A switch change: drop the cached switches of the tenant (platform: of every tenant this instance serves) so
				// the next message re-reads PostgreSQL; the cache freshness (≤ 30 s) bounds the effect anyway (F116-AC-20).
				var keys []string
				if n.TenantID != nil { // a category setting of the tenant may have changed too
					p.Catalogs.Invalidate(ctx, n.TenantID)
					keys = append(keys, service.SwitchKey(*n.TenantID))
				} else {
					for _, t := range p.Catalogs.Tenants() {
						keys = append(keys, service.SwitchKey(t))
					}
				}
				if err := p.Switches.Cache.Invalidate(ctx, keys...); err != nil {
					slog.WarnContext(ctx, "chat: cached switches not dropped (freshness bounds the effect)", "error", err)
				}
			}
		})
	}()
	return p.Catalogs.Run(ctx)
}

// TestDoubleValidator is a TEST DOUBLE (R17) — NOT a moderation pipeline. It runs no check and answers "unchecked"
// with the text unchanged, so unit tests of the send path can run; command.RequireValidator refuses it outside
// EW_ENV=test.
type TestDoubleValidator struct{}

var _ port.TestDouble = TestDoubleValidator{}

// TestDouble marks the test double.
func (TestDoubleValidator) TestDouble() bool { return true }

// Validate implements port.ContentValidator: decision unchecked, every step reported as skipped.
func (TestDoubleValidator) Validate(_ context.Context, in port.ValidationInput) (port.Verdict, error) {
	return port.Verdict{Decision: message.Unchecked, Display: in.Text, StepsSkipped: []string{"test_double"}}, nil
}
