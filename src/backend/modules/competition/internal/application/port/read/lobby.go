package read

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
)

// LobbyReader reads what the Party Lobby queries need (projections ref.* and the running activity), under
// entx.ReadSession with explicit tenant_id and deleted_at conditions.
type LobbyReader interface {
	// Subjects returns the policy view of the students of tenant at now (unknown students are inactive).
	Subjects(ctx context.Context, tenant uuid.UUID, ids []uuid.UUID, now time.Time) ([]party.Subject, error)
	// Areas returns the active world areas (at most limit, ordered by id).
	Areas(ctx context.Context, limit int) ([]party.AreaRule, error)
	// UnlockedAreas returns the unlocked areas of each student.
	UnlockedAreas(ctx context.Context, tenant uuid.UUID, students []uuid.UUID) (map[uuid.UUID]map[string]bool, error)
	// RunningRoom returns the room of the running activity of a party, nil when none.
	RunningRoom(ctx context.Context, tenant, partyID uuid.UUID) (*uuid.UUID, error)
}
