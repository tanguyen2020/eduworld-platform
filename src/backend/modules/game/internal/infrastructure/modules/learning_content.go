package modules

import (
	"context"
	"errors"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

var _ port.LearningContentCheck = LearningAttempts{}

// PublishedContent implements port.LearningContentCheck on checkPublishedLearningContent of the learning facade
// (submit_review of the Game Operations Console, F078-AC-07): read only, in process through pkg/modcall.
func (l LearningAttempts) PublishedContent(ctx context.Context, refs []port.LearningRef) ([]bool, error) {
	if l.F == nil {
		return nil, port.ErrLearningUnavailable
	}
	body := learningapi.CheckPublishedLearningContentJSONRequestBody{Refs: make([]learningapi.LearningContentRef, 0, len(refs))}
	for _, r := range refs {
		body.Refs = append(body.Refs, learningapi.LearningContentRef{ContentType: learningapi.LearningContentRefContentType(r.ContentType),
			ContentId: r.ContentID, Version: r.Version})
	}
	op := modcall.Op{Caller: "game", Callee: "learning", Operation: "checkPublishedLearningContent", Kind: modcall.Read,
		AllowedCallers: learningapi.AllowedCallers["checkPublishedLearningContent"], ContextRequired: learningapi.ContextRequired["checkPublishedLearningContent"]}
	resp, err := modcall.Call(ctx, op, func(ctx context.Context) (learningapi.CheckPublishedLearningContentResponseObject, error) {
		return l.F.CheckPublishedLearningContent(ctx, learningapi.CheckPublishedLearningContentRequestObject{Body: &body})
	})
	var pe *problem.Error
	switch {
	case errors.As(err, &pe) && problem.Status(pe.Code) < 500:
		return nil, err
	case err != nil:
		return nil, errors.Join(port.ErrLearningUnavailable, err)
	}
	v, ok := resp.(learningapi.CheckPublishedLearningContent200JSONResponse)
	if !ok || len(v.Results) != len(refs) {
		return nil, port.ErrLearningUnavailable
	}
	out := make([]bool, len(v.Results))
	for i, r := range v.Results {
		out[i] = r.Published
	}
	return out, nil
}

var _ port.LearningContentCheck = (*LateLearning)(nil)

// PublishedContent implements port.LearningContentCheck.
func (l *LateLearning) PublishedContent(ctx context.Context, refs []port.LearningRef) ([]bool, error) {
	l.mu.RLock()
	f := l.f
	l.mu.RUnlock()
	return LearningAttempts{F: f}.PublishedContent(ctx, refs)
}
