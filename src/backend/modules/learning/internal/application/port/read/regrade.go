package read

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
)

// RegradeReadModel reads assessment.regrade_requests: a platform request (tenant_id NULL) when platform is true, else a
// request of the session tenant (nil when invisible).
type RegradeReadModel interface {
	Request(ctx context.Context, id uuid.UUID, platform bool) (*dto.RegradeRequestView, error)
}
