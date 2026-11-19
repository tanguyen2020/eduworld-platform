package consumer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// PlatformCurfewApplier stores platform_curfew (command.PlatformSettings).
type PlatformCurfewApplier interface {
	ApplyPlatformCurfew(ctx context.Context, w write.CurfewWindow, sourceVersion int64) (bool, error)
}

// PlatformSettings is the consumer game.platform_settings (OQ F014-2, F015 Clarifications 2026-11-20): it keeps
// platform_curfew of module identity (putPlatformCurfewSettings → platform_settings.changed, key platform_curfew,
// scope platform) in ref.platform_settings, so createGameSession and the job curfew-session-closer apply a changed
// curfew without a restart. Other keys and scopes are acknowledged without effect. Idempotency per event_id is
// pkg/outbox.Consumer (processed_events); an older event (envelope time) never overrides a newer value.
type PlatformSettings struct {
	Settings PlatformCurfewApplier
	Now      func() time.Time // lag metric clock; nil = time.Now
}

// PlatformSettingsOptions returns the outbox consumer options (platform_settings.changed of scope platform is a
// platform event: Platform true).
func PlatformSettingsOptions(h PlatformSettings, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "game", Name: messaging.PlatformSettingsName, Stream: messaging.IdentityStream,
		Subjects: messaging.Subjects(messaging.PlatformSettingsEvents), Platform: true, UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

type platformSettingsChanged struct {
	Key          string              `json:"key"`
	Scope        string              `json:"scope"`
	CurfewWindow *write.CurfewWindow `json:"curfew_window"`
	After        *write.CurfewWindow `json:"after"`
}

func (c PlatformSettings) reject(e outbox.Envelope, why string) error {
	observability.ConsumerRejected().WithLabelValues(messaging.PlatformSettingsName, "invalid_payload").Inc()
	return outbox.Permanent(fmt.Errorf("%w: %s: %s", errPayload, e.EventType, why))
}

// Handle implements outbox.Handler.
func (c PlatformSettings) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.EventType != "platform_settings.changed" {
		return outbox.Permanent(fmt.Errorf("unexpected event %s", e.EventType))
	}
	var p platformSettingsChanged
	if err := decode(e, &p); err != nil {
		return c.reject(e, "not a JSON object of the schema")
	}
	if p.Key != write.KeyPlatformCurfew || p.Scope != "platform" {
		return nil // another setting (student_cleanup, chat_retention, feature flags, tenant/school settings): not used here
	}
	if e.TenantID != nil {
		return c.reject(e, "platform_curfew is a platform setting: the envelope has a tenant")
	}
	// curfew_window is the value after the change (F015 T032); after carries the same {start, end} for this key.
	w := p.CurfewWindow
	if w == nil {
		w = p.After
	}
	if w == nil || len(w.Start) > 5 || len(w.End) > 5 {
		return c.reject(e, "curfew_window {start, end} is required for platform_curfew")
	}
	applied, err := c.Settings.ApplyPlatformCurfew(ctx, *w, e.OccurredAt.UTC().UnixMicro())
	switch {
	case errors.Is(err, command.ErrInvalidPlatformCurfew):
		return c.reject(e, err.Error())
	case err != nil:
		return err // transient → NAK with delay, DLQ after MaxDeliver
	}
	if applied {
		now := time.Now
		if c.Now != nil {
			now = c.Now
		}
		observability.ProjectionLag().WithLabelValues("platform_settings").Observe(now().Sub(e.OccurredAt).Seconds())
	}
	return nil
}
