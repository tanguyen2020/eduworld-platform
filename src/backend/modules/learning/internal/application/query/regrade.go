package query

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Regrades reads regrade requests (getRegradeRequest, F031 US4): learning_content.re_evaluate; platform staff reads
// platform requests, a tenant user the requests of its session tenant; anything else is 404.
type Regrades struct {
	Session read.Session
	Model   read.RegradeReadModel
	Cursors *page.Codec
}

// List is listRegradeRequests (F058-AC-23): the history of one question or activity version in the scope of the caller,
// newest first; one of the two version ids is required.
func (q Regrades) List(ctx context.Context, questionVersionID, activityVersionID *uuid.UUID, in PageInput) (out Page[dto.RegradeRequestView], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListRegradeRequests")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if !holds(p, authz.PermLearningContentReEvaluate) {
		return out, authz.Require(ctx, p, authz.PermLearningContentReEvaluate, authz.Resource{TenantID: p.TenantID})
	}
	if (questionVersionID == nil) == (activityVersionID == nil) {
		return out, problem.Invalid("/question_version_id", problem.SubRequired, "exactly one of question_version_id and activity_version_id is required")
	}
	limit := page.Limit(in.Limit)
	out = Page[dto.RegradeRequestView]{Limit: limit, Data: []dto.RegradeRequestView{}}
	var after *read.RegradeKey
	if in.Cursor != "" {
		cur, err := q.Cursors.Decode(in.Cursor)
		if err != nil || cur == nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
		at, err := time.Parse(time.RFC3339Nano, cur.Key)
		if err != nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
		after = &read.RegradeKey{CreatedAt: at, ID: cur.ID}
	}
	var sc tenancy.Scope
	if p.TenantID != nil {
		sc, err = authz.ScopeFor(p)
	} else {
		sc, err = authz.PlatformScope(p)
	}
	if err != nil {
		return out, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	var rows []dto.RegradeRequestView
	err = q.Session.Read(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		rows, err = q.Model.Requests(ctx, questionVersionID, activityVersionID, p.TenantID == nil, after, limit+1)
		return err
	})
	if err != nil {
		return out, err
	}
	if len(rows) > limit {
		out.HasMore, rows = true, rows[:limit]
		last := rows[limit-1]
		s := q.Cursors.Encode(page.Cursor{Key: last.CreatedAt.UTC().Format(time.RFC3339Nano), ID: last.RegradeRequestID})
		out.NextCursor = &s
	}
	out.Data = append(out.Data, rows...)
	return out, nil
}

// Get is getRegradeRequest.
func (q Regrades) Get(ctx context.Context, id uuid.UUID) (out dto.RegradeRequestView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetRegradeRequest")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if !holds(p, authz.PermLearningContentReEvaluate) {
		return out, authz.Require(ctx, p, authz.PermLearningContentReEvaluate, authz.Resource{TenantID: p.TenantID})
	}
	var sc tenancy.Scope
	if p.TenantID != nil {
		sc, err = authz.ScopeFor(p)
	} else {
		sc, err = authz.PlatformScope(p)
	}
	if err != nil {
		return out, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	err = q.Session.Read(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		v, err := q.Model.Request(ctx, id, p.TenantID == nil)
		if err != nil {
			return err
		}
		if v == nil {
			return problem.New(problem.NotFound, "")
		}
		out = *v
		return nil
	})
	return out, err
}
