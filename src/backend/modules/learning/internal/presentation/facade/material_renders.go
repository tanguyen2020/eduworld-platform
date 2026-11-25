package facade

import (
	"context"
	"encoding/json"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// GetLearningMaterialRenders implements getLearningMaterialRenders (x-allowed-callers: game, F103 research R3): the
// display blocks of published learning materials (clue cards), never a question or a solution.
func (f Facade) GetLearningMaterialRenders(ctx context.Context, req learningapi.GetLearningMaterialRendersRequestObject) (learningapi.GetLearningMaterialRendersResponseObject, error) {
	if req.Body == nil {
		return nil, problem.New(problem.BadRequest, "body is required")
	}
	if f.Materials == nil {
		return nil, problem.New(problem.Internal, "learning material renders are not wired")
	}
	refs := make([]query.MaterialRef, len(req.Body.Refs))
	for i, r := range req.Body.Refs {
		refs[i] = query.MaterialRef{ID: r.Id, Version: r.Version}
	}
	rs, err := f.Materials.Renders(ctx, refs)
	if err != nil {
		return nil, err
	}
	data := make([]map[string]any, 0, len(rs))
	for _, r := range rs {
		m := map[string]any{"id": r.ID, "version": r.Version, "blocks": r.Blocks}
		if len(r.Title) > 0 {
			m["title"] = r.Title
		}
		data = append(data, m)
	}
	raw, err := json.Marshal(map[string]any{"data": data})
	if err != nil {
		return nil, err
	}
	var out learningapi.GetLearningMaterialRenders200JSONResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
