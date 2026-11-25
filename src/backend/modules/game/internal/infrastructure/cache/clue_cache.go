package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
)

// ClueKey is the Valkey key of the render of the clue card of a role (VALKEY_DESIGN, F103 research R3): the version is
// pinned, so the value is immutable while it lives (Rebuildable from module learning).
func ClueKey(versionID uuid.UUID, stepID, role string) string {
	return "ew:game:clue:" + versionID.String() + ":" + stepID + ":" + role
}

// ClueTTL is the lifetime of a cached clue render (1 h, F103 data-model).
const ClueTTL = time.Hour

// ClueSource reads the renders of learning materials (module learning, getLearningMaterialRenders).
type ClueSource interface {
	MaterialRenders(ctx context.Context, refs []quest.ClueRef) (map[string]port.ClueRender, error)
}

// ClueCache implements port.ClueRenders: the cached renders first, the missing ones from module learning (at most 4 per
// call), stored back for ClueTTL. The cache never decides who sees a card (module game does, F103-AC-21); a Valkey
// failure falls back to module learning.
type ClueCache struct {
	C      valkey.Client // nil: no cache
	Source ClueSource
}

var _ port.ClueRenders = ClueCache{}

// Renders implements port.ClueRenders.
func (c ClueCache) Renders(ctx context.Context, versionID uuid.UUID, stepID string, roles []quest.Role) (map[string]port.ClueRender, error) {
	out := map[string]port.ClueRender{}
	var missing []quest.Role
	for _, r := range roles {
		if c.C != nil {
			cctx, cancel := context.WithTimeout(ctx, worldTimeout)
			raw, err := c.C.Do(cctx, c.C.B().Get().Key(ClueKey(versionID, stepID, r.Role)).Build()).ToString()
			cancel()
			var render port.ClueRender
			if err == nil && json.Unmarshal([]byte(raw), &render) == nil {
				out[r.Role] = render
				continue
			}
		}
		missing = append(missing, r)
	}
	for i := 0; i < len(missing); i += 4 {
		batch := missing[i:min(i+4, len(missing))]
		refs := make([]quest.ClueRef, len(batch))
		for j, r := range batch {
			refs[j] = r.Clue
		}
		got, err := c.Source.MaterialRenders(ctx, refs)
		if err != nil {
			return nil, err
		}
		for _, r := range batch {
			render, ok := got[r.Clue.ID]
			if !ok {
				continue
			}
			out[r.Role] = render
			if c.C != nil {
				if b, err := json.Marshal(render); err == nil {
					cctx, cancel := context.WithTimeout(ctx, worldTimeout)
					_ = c.C.Do(cctx, c.C.B().Set().Key(ClueKey(versionID, stepID, r.Role)).Value(string(b)).Ex(ClueTTL).Build()).Error()
					cancel()
				}
			}
		}
	}
	return out, nil
}
