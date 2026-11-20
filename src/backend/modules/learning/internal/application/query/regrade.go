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
