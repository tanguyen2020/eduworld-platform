// Package query holds the read-context use cases of the sample module: read models only, no
// UnitOfWork, no outbox, no write ports (ADR-0087; checked by tools/archgen).
package query

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/internal/application/port/read"
)

// ListWidgetsHandler lists the widgets of the session tenant.
type ListWidgetsHandler interface {
	Handle(ctx context.Context) ([]dto.WidgetView, error)
}

// List implements ListWidgetsHandler.
type List struct{ Model read.WidgetReadModel }

// Handle runs the query.
func (h List) Handle(ctx context.Context) (out []dto.WidgetView, err error) {
	ctx, done := observability.UseCase(ctx, "sample.query.ListWidgets")
	defer func() { done(err) }()
	p, ok := authz.PrincipalFrom(ctx)
	if !ok {
		return nil, problem.New(problem.Unauthenticated, "")
	}
	if err = authz.Require(ctx, p, authz.PermLearningContentRead, authz.Resource{TenantID: p.TenantID}); err != nil {
		return nil, err
	}
	return h.Model.List(ctx)
}
