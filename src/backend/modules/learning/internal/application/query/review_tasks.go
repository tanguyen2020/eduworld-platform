package query

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/workflow"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// ReviewTasks is the review queue (F025 US2, learning_content.review).
type ReviewTasks struct {
	Session read.Session
	Model   read.ReviewTaskReadModel
	Cursors *page.Codec
}

// ReviewTaskInput is the request of listContentReviewTasks.
type ReviewTaskInput struct {
	Lane         string
	Status       string
	ContentType  string
	AssignedToMe bool
	Page         PageInput
}

// reviewSchools are the schools of the review grants of p when every one is school-scoped (nil = whole tenant).
func reviewSchools(p authz.Principal) []uuid.UUID {
	var out []uuid.UUID
	for _, g := range p.Grants {
		if !slices.Contains(g.Permissions, authz.PermLearningContentReview) {
			continue
		}
		if g.SchoolID == nil {
			return nil
		}
		out = append(out, *g.SchoolID)
	}
	return out
}

// List pages the open review tasks visible to the caller, oldest first (F025-AC-12…14): a tenant reviewer sees the
// tenant lane of its session tenant (tenant_ids ignored, the schools of a school-scoped grant only); platform staff see
// the platform_learning lane; System Admin with tenant_ids sees the tenant lane of those tenants (CrossTenantScope of
// httpx, audited cross_tenant.accessed).
func (q ReviewTasks) List(ctx context.Context, in ReviewTaskInput) (out Page[dto.ReviewTaskView], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListContentReviewTasks")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	// A school-scoped review grant opens the queue of its schools (filter below); Require records the denial otherwise.
	if !p.Has(authz.PermLearningContentReview) {
		return out, authz.Require(ctx, p, authz.PermLearningContentReview, authz.Resource{})
	}
	f := read.ReviewTaskFilter{Status: in.Status, ContentType: in.ContentType}
	if f.Status == "" {
		f.Status = string(workflow.TaskOpen)
	}
	switch workflow.TaskStatus(f.Status) {
	case workflow.TaskOpen, workflow.TaskApproved, workflow.TaskRejected, workflow.TaskCancelled:
	default:
		return out, problem.Invalid("status", problem.SubInvalid, "is not a review task status")
	}
	if f.ContentType != "" {
		if _, err := contentmodel.ParseContentType(f.ContentType); err != nil {
			return out, problem.Invalid("content_type", problem.SubInvalid, "is not a learning content type")
		}
	}
	cross := false
	if s, ok := tenancy.ScopeFrom(ctx); ok {
		_, cross = s.(tenancy.CrossTenantScope)
	}
	lane := workflow.LanePlatform
	if p.TenantID != nil || cross {
		lane = workflow.LaneTenant
		f.SchoolIDs = reviewSchools(p)
	}
	f.Lane = string(lane)
	limit := page.Limit(in.Page.Limit)
	out = Page[dto.ReviewTaskView]{Limit: limit, Data: []dto.ReviewTaskView{}}
	if in.Lane != "" && in.Lane != f.Lane {
		return out, nil // the other lane is never visible to the caller
	}
	if in.AssignedToMe {
		f.AssigneeID = &p.UserID
	}
	cur, err := q.Cursors.Decode(in.Page.Cursor)
	if err != nil {
		return out, problem.New(problem.BadRequest, "invalid cursor")
	}
	if cur != nil {
		f.After = &cur.ID
	}
	f.Limit = limit + 1
	var rows []dto.ReviewTaskView
	if err := q.Session.Read(ctx, func(ctx context.Context) error {
		var err error
		rows, err = q.Model.List(ctx, f)
		return err
	}); err != nil {
		return out, err
	}
	if len(rows) > limit {
		out.HasMore, rows = true, rows[:limit]
		s := q.Cursors.Encode(page.Cursor{ID: rows[limit-1].TaskID})
		out.NextCursor = &s
	}
	out.Data = append(out.Data, rows...)
	return out, nil
}
