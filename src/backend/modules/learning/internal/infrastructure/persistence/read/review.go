package read

import (
	"context"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	entrt "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/reviewtask"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
)

// ReviewTaskModel implements read.ReviewTaskReadModel with ent (entx.TenantOptional filter of the scope of ctx).
type ReviewTaskModel struct{}

var _ read.ReviewTaskReadModel = ReviewTaskModel{}

// List implements read.ReviewTaskReadModel.
func (ReviewTaskModel) List(ctx context.Context, f read.ReviewTaskFilter) ([]dto.ReviewTaskView, error) {
	q := client(ctx).ReviewTask.Query().Where(entrt.ReviewLane(f.Lane), entrt.Status(f.Status))
	if f.ContentType != "" {
		q = q.Where(entrt.ContentType(f.ContentType))
	}
	if len(f.SchoolIDs) > 0 {
		q = q.Where(entrt.SchoolIDIn(f.SchoolIDs...))
	}
	if f.AssigneeID != nil {
		q = q.Where(entrt.AssigneeID(*f.AssigneeID))
	}
	if f.After != nil {
		q = q.Where(entrt.IDGT(*f.After))
	}
	rows, err := q.Order(entrt.ByID(entsql.OrderAsc())).Limit(f.Limit).All(ctx)
	if err != nil {
		return nil, notFound(err)
	}
	out := make([]dto.ReviewTaskView, len(rows))
	for i, r := range rows {
		out[i] = reviewTaskView(r)
	}
	return out, nil
}

func reviewTaskView(r *ent.ReviewTask) dto.ReviewTaskView {
	v := dto.ReviewTaskView{TaskID: r.ID, ContentType: r.ContentType, ContentID: r.ContentID, ContentVersionID: r.ContentVersionID,
		Version: r.VersionNo, Lane: r.ReviewLane, Status: r.Status, AuthorID: r.AuthorID, ReviewerID: r.ResolvedBy, SchoolID: r.SchoolID,
		TenantID: r.TenantID, SubmittedAt: r.SubmittedAt.UTC(), DueAt: r.DueAt, ResolvedAt: r.ResolvedAt}
	if r.Title != nil {
		v.Title = *r.Title
	}
	return v
}
