package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Regrades reads regrade requests (getRegradeRequest, F031 US4): learning_content.re_evaluate; platform staff reads
// platform requests, a tenant user the requests of its session tenant; anything else is 404.
type Regrades struct {
	Session read.Session
	Model   read.RegradeReadModel
}

// List is listRegradeRequests (F058-AC-23): the history of one question or activity version in the scope of the caller,
// newest first; one of the two version ids is required.
func (q Regrades) List(ctx context.Context, questionVersionID, activityVersionID *uuid.UUID, limit int) (out []dto.RegradeRequestView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListRegradeRequests")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if !holds(p, authz.PermLearningContentReEvaluate) {
		return nil, authz.Require(ctx, p, authz.PermLearningContentReEvaluate, authz.Resource{TenantID: p.TenantID})
	}
	if (questionVersionID == nil) == (activityVersionID == nil) {
		return nil, problem.Invalid("/question_version_id", problem.SubRequired, "exactly one of question_version_id and activity_version_id is required")
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	var sc tenancy.Scope
	if p.TenantID != nil {
		sc, err = authz.ScopeFor(p)
	} else {
		sc, err = authz.PlatformScope(p)
	}
	if err != nil {
		return nil, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	err = q.Session.Read(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		out, err = q.Model.Requests(ctx, questionVersionID, activityVersionID, p.TenantID == nil, limit)
		return err
	})
	return out, err
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
