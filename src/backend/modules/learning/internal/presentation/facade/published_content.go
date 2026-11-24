package facade

import (
	"context"

	openapi_types "github.com/oapi-codegen/runtime/types"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// CheckPublishedLearningContent implements checkPublishedLearningContent (caller module game, F078-AC-07): whether the
// platform learning content a game config references is released.
func (f Facade) CheckPublishedLearningContent(ctx context.Context, req learningapi.CheckPublishedLearningContentRequestObject) (
	learningapi.CheckPublishedLearningContentResponseObject, error) {
	if req.Body == nil {
		return nil, problem.New(problem.BadRequest, "body is required")
	}
	if f.PublishedContent == nil {
		return nil, problem.New(problem.Internal, "the published content check is not wired")
	}
	refs := make([]read.PublishedRef, 0, len(req.Body.Refs))
	for _, r := range req.Body.Refs {
		refs = append(refs, read.PublishedRef{ContentType: string(r.ContentType), ContentID: r.ContentId, Version: r.Version})
	}
	res, err := f.PublishedContent.Check(ctx, refs)
	if err != nil {
		return nil, err
	}
	out := learningapi.CheckPublishedLearningContent200JSONResponse{}
	for i, r := range req.Body.Refs {
		out.Results = append(out.Results, struct {
			ContentId   openapi_types.UUID                                       `json:"content_id"`
			ContentType learningapi.LearningContentCheckResultResultsContentType `json:"content_type"`
			Published   bool                                                     `json:"published"`
		}{ContentId: r.ContentId, ContentType: learningapi.LearningContentCheckResultResultsContentType(r.ContentType), Published: res[i]})
	}
	return out, nil
}
