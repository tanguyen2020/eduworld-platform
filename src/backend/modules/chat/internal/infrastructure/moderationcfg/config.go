// Package moderationcfg loads the embedded configuration of the moderation engine (seed/normalization).
package moderationcfg

import (
	"encoding/json"
	"fmt"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/normalize"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/pii"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/seed/normalization"
)

type targets struct {
	BullyingTargets []string `json:"bullying_targets"`
}

// Load decodes the embedded configuration (normalization.v1.json).
func Load() (moderation.Config, error) {
	var n normalize.Config
	var p pii.Config
	var t targets
	for _, v := range []any{&n, &p, &t} {
		if err := json.Unmarshal(normalization.V1, v); err != nil {
			return moderation.Config{}, fmt.Errorf("chat: moderation config: %w", err)
		}
	}
	if n.Version < 1 || len(n.Leet) == 0 || len(p.NumberWords) == 0 || len(p.SocialApps) == 0 {
		return moderation.Config{}, fmt.Errorf("chat: moderation config: incomplete (version %d)", n.Version)
	}
	return moderation.Config{Normalize: n, PII: p, BullyingTargets: t.BullyingTargets}, nil
}

// MustEngine returns the engine of the embedded configuration (tests, wiring); it panics on a broken embedded file,
// which TestModerationConfig guards.
func MustEngine() *moderation.Engine {
	c, err := Load()
	if err != nil {
		panic(err)
	}
	return moderation.NewEngine(c)
}
