package read

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
)

// ReviewTaskFilter selects review tasks of the queue (listContentReviewTasks, F025 US2).
type ReviewTaskFilter struct {
	Lane        string      // tenant | platform_learning
	Status      string      // open by default
	ContentType string      // optional
	SchoolIDs   []uuid.UUID // a school-scoped reviewer sees the tasks of its schools only
	AssigneeID  *uuid.UUID  // assigned_to_me
	After       *uuid.UUID  // cursor: task id
	Limit       int         // rows to return (limit+1 to know has_more)
}

// ReviewTaskReadModel reads content.review_tasks with the scope of ctx (session tenant, PlatformScope or the
// CrossTenantScope of System Admin), oldest first.
type ReviewTaskReadModel interface {
	List(ctx context.Context, f ReviewTaskFilter) ([]dto.ReviewTaskView, error)
}
