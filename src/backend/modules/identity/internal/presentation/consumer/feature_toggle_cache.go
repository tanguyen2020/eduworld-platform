package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// FeatureToggleCacheName is the consumer that drops cached menu/action toggles (F015 T042, F015-AC-27):
// every replica of core-api subscribes (ephemeral consumer per instance) so featuregate sees a change in
// at most feature_toggle_cache_ttl.
const FeatureToggleCacheName = "identity.feature_toggle_cache"

// FeatureToggleCacheSubject is the subject it handles.
const FeatureToggleCacheSubject = "ew.evt.identity.platform_settings.changed"

// FeatureToggleCache invalidates the cache on platform_settings.changed with scope feature_flag.
type FeatureToggleCache struct {
	Cache port.FeatureToggleCache
}

// Handle implements outbox.Handler.
func (c FeatureToggleCache) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.EventType != "platform_settings.changed" {
		return outbox.Permanent(fmt.Errorf("unexpected event %s", e.EventType))
	}
	var p struct {
		Scope    string     `json:"scope"`
		TenantID *uuid.UUID `json:"tenant_id"`
	}
	if err := json.Unmarshal(e.Payload, &p); err != nil {
		return outbox.Permanent(fmt.Errorf("platform_settings.changed: invalid payload"))
	}
	if p.Scope != "feature_flag" {
		return nil
	}
	return c.Cache.Invalidate(ctx, p.TenantID)
}
