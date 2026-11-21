package read

import (
	"context"
	"time"

	"github.com/google/uuid"

	mm "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/matchmaking"
)

// MatchmakingReader reads what the matchmaking queries need under entx.ReadSession (F104): the published
// configuration and the matchmaking view of students of the session tenant.
type MatchmakingReader interface {
	ActiveConfig(ctx context.Context, at time.Time) (mm.Config, error)
	Players(ctx context.Context, tenant uuid.UUID, ids []uuid.UUID, now time.Time) (map[uuid.UUID]mm.Player, error)
}
