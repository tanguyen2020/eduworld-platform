package read

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
)

// RegradeReadModel reads assessment.regrade_requests: a platform request (tenant_id NULL) when platform is true, else a
// request of the session tenant (nil when invisible).
type RegradeReadModel interface {
	// Requests lists the requests of a question or activity version, newest first (limit ≤ 100).
	Requests(ctx context.Context, questionVersionID, activityVersionID *uuid.UUID, platform bool, limit int) ([]dto.RegradeRequestView, error)
	Request(ctx context.Context, id uuid.UUID, platform bool) (*dto.RegradeRequestView, error)
}
