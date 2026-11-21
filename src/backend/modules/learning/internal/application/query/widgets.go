package query

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/scope"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/widget"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// QuestionWidgets lists the interactive widgets registered at build time (F023-AC-13, learning_content.read).
type QuestionWidgets struct {
	Widgets *widget.Registry
}

// List returns the widgets, their versions, config schemas and validators.
func (q QuestionWidgets) List(ctx context.Context) (out []widget.Info, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListQuestionWidgets")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := scope.RequireContent(ctx, p, authz.PermLearningContentRead, nil); err != nil {
		return nil, err
	}
	return q.Widgets.List(), nil
}
