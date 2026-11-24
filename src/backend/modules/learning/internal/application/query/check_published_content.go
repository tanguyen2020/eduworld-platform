package query

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// CheckPublishedContentHandler is the typed interface of the facade query checkPublishedLearningContent (ADR-0087).
type CheckPublishedContentHandler interface {
	Check(ctx context.Context, refs []read.PublishedRef) ([]bool, error)
}

// CheckPublishedContent is the facade query checkPublishedLearningContent (x-allowed-callers: game, F078-AC-07): module
// game asks it when a game config version is submitted for review (challenge F076, quest F072, mini-game F073) whether
// the platform learning content it references is released. Read only; no content, answer or scoring rule is returned.
type CheckPublishedContent struct {
	Session read.Session
	Content read.PublishedContentReadModel
}

var _ CheckPublishedContentHandler = CheckPublishedContent{}

// Check implements CheckPublishedContentHandler.
func (q CheckPublishedContent) Check(ctx context.Context, refs []read.PublishedRef) (out []bool, err error) {
	ctx, done := observability.UseCase(ctx, "learning.facade.CheckPublishedLearningContent")
	defer func() { done(err) }()
	if _, err := principal(ctx); err != nil {
		return nil, err
	}
	if len(refs) == 0 || len(refs) > 200 {
		return nil, problem.Invalid("/refs", problem.SubInvalid, "1 to 200 references")
	}
	for _, r := range refs {
		switch r.ContentType {
		case "activity", "question_set", "lesson":
		default:
			return nil, problem.Invalid("/refs/content_type", problem.SubInvalid, "activity, question_set or lesson")
		}
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		out, err = q.Content.Published(ctx, refs)
		return err
	})
	return out, err
}
