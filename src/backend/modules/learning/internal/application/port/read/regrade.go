package read

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
)

// RegradeKey is the keyset position of RegradeReadModel.Requests.
type RegradeKey struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// RegradeReadModel reads assessment.regrade_requests: a platform request (tenant_id NULL) when platform is true, else a
// request of the session tenant (nil when invisible).
type RegradeReadModel interface {
	// Requests lists the requests of a question or activity version, newest first (limit ≤ 100).
	// Requests is one keyset page, newest first (created_at DESC, id DESC), rows strictly after `after` (API_DESIGN §5).
	Requests(ctx context.Context, questionVersionID, activityVersionID *uuid.UUID, platform bool, after *RegradeKey, limit int) ([]dto.RegradeRequestView, error)
	Request(ctx context.Context, id uuid.UUID, platform bool) (*dto.RegradeRequestView, error)
}
