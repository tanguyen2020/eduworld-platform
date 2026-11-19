// Package featuretoggle implements featuregate.Source (F012 port, F015 T042, ADR-0090 §3) and
// port.FeatureToggles from the portal menu catalog and the feature flags of module identity, with a Valkey
// cache ew:fg:<generation>:<tenant_id|platform> (TTL feature_toggle_cache_ttl). A platform-wide change bumps
// the generation; a tenant change drops that tenant's entry.
package featuretoggle

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/featuregate"
)

const (
	keyPrefix = "ew:fg:"
	genKey    = keyPrefix + "gen"
)

// Source computes the disabled menus/actions of a scope: no row = enabled; tenant override else default;
// action = action AND parent menu; feature flags are not part of it (F015-AC-27, AC-29).
type Source struct {
	Reader read.FeatureFlagReader
	C      valkey.Client // nil: no shared cache (featuregate keeps its own short in-process cache)
	TTL    time.Duration // feature_toggle_cache_ttl
}

var (
	_ featuregate.Source      = Source{}
	_ port.FeatureToggles     = Source{}
	_ port.FeatureToggleCache = Source{}
)

type cached struct {
	Menus   []string `json:"m"`
	Actions []string `json:"a"`
}

func scope(tenant *uuid.UUID) string {
	if tenant == nil {
		return "platform"
	}
	return tenant.String()
}

func (s Source) generation(ctx context.Context) string {
	if s.C == nil {
		return "0"
	}
	g, err := s.C.Do(ctx, s.C.B().Get().Key(genKey).Build()).ToString()
	if err != nil {
		return "0"
	}
	return g
}

// Disabled implements port.FeatureToggles.
func (s Source) Disabled(ctx context.Context, tenant *uuid.UUID) (menus, actions []string, err error) {
	key := keyPrefix + s.generation(ctx) + ":" + scope(tenant)
	if s.C != nil {
		if raw, err := s.C.Do(ctx, s.C.B().Get().Key(key).Build()).AsBytes(); err == nil {
			var c cached
			if json.Unmarshal(raw, &c) == nil {
				return c.Menus, c.Actions, nil
			}
		}
	}
	_, items, err := s.Reader.Catalog(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("feature toggles: %w", err)
	}
	rows, err := s.Reader.Flags(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("feature toggles: %w", err)
	}
	flags := map[string]settings.Flag{}
	for _, f := range rows {
		if f.Kind != "feature" {
			flags[f.Key] = f
		}
	}
	menus, actions = settings.Disabled(items, flags, tenant)
	if menus == nil {
		menus = []string{}
	}
	if actions == nil {
		actions = []string{}
	}
	if s.C != nil && s.TTL > 0 {
		raw, _ := json.Marshal(cached{Menus: menus, Actions: actions})
		if err := s.C.Do(ctx, s.C.B().Set().Key(key).Value(valkey.BinaryString(raw)).Px(s.TTL).Build()).Error(); err != nil {
			slog.WarnContext(ctx, "feature toggle cache write failed", "error", err)
		}
	}
	return menus, actions, nil
}

// DisabledFor implements featuregate.Source.
func (s Source) DisabledFor(ctx context.Context, tenant *uuid.UUID) (featuregate.Set, error) {
	m, a, err := s.Disabled(ctx, tenant)
	if err != nil {
		return featuregate.Set{}, err
	}
	return featuregate.NewSet(m, a), nil
}

// Invalidate implements command.FeatureToggleCache: a tenant scope drops its entry; nil (platform default
// changed) bumps the generation so every scope recomputes.
func (s Source) Invalidate(ctx context.Context, tenant *uuid.UUID) error {
	if s.C == nil {
		return nil
	}
	if tenant == nil {
		return s.C.Do(ctx, s.C.B().Incr().Key(genKey).Build()).Error()
	}
	return s.C.Do(ctx, s.C.B().Del().Key(keyPrefix+s.generation(ctx)+":"+tenant.String()).Build()).Error()
}
