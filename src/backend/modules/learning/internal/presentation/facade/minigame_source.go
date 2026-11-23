package facade

import (
	"context"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins/minigame"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ValidateMinigameQuestionSource implements validateMinigameQuestionSource (x-allowed-callers: game, F073-AC-02): module
// game checks the question source of a mini-game configuration when it publishes it; read-only, no route serves it (R29).
func (f Facade) ValidateMinigameQuestionSource(ctx context.Context, req learningapi.ValidateMinigameQuestionSourceRequestObject) (
	learningapi.ValidateMinigameQuestionSourceResponseObject, error) {
	if req.Body == nil {
		return nil, problem.New(problem.BadRequest, "body is required")
	}
	if f.MinigameSources == nil {
		return nil, problem.New(problem.Internal, "the mini-game source check is not wired")
	}
	b := *req.Body
	src := minigame.Source{Kind: string(b.QuestionSource.Kind), QuestionSetID: b.QuestionSource.QuestionSetId, Version: b.QuestionSource.Version}
	if r := b.QuestionSource.Rule; r != nil {
		rule := minigame.Rule{SubjectID: r.SubjectId, DifficultyMin: r.DifficultyMin, DifficultyMax: r.DifficultyMax}
		if r.GradeLevels != nil {
			rule.GradeLevels = *r.GradeLevels
		}
		if r.TopicIds != nil {
			rule.TopicIDs = *r.TopicIds
		}
		if r.ObjectiveIds != nil {
			rule.ObjectiveIDs = *r.ObjectiveIds
		}
		if r.QuestionTypes != nil {
			for _, t := range *r.QuestionTypes {
				rule.QuestionTypes = append(rule.QuestionTypes, string(t))
			}
		}
		src.Rule = &rule
	}
	res, err := f.MinigameSources.Validate(ctx, query.MinigameSourceInput{MinigameType: string(b.MinigameType), Source: src, QuestionCount: b.QuestionCount})
	if err != nil {
		return nil, err
	}
	out := learningapi.ValidateMinigameQuestionSource200JSONResponse{Compatible: res.Compatible, AvailableCount: res.AvailableCount,
		QuestionTypes: make([]learningapi.QuestionType, 0, len(res.QuestionTypes))}
	for _, t := range res.QuestionTypes {
		out.QuestionTypes = append(out.QuestionTypes, learningapi.QuestionType(t))
	}
	out.Reasons = make([]struct {
		Code    learningapi.MinigameQuestionSourceResultReasonsCode `json:"code"`
		Message string                                              `json:"message"`
	}, 0, len(res.Reasons))
	for _, r := range res.Reasons {
		out.Reasons = append(out.Reasons, struct {
			Code    learningapi.MinigameQuestionSourceResultReasonsCode `json:"code"`
			Message string                                              `json:"message"`
		}{Code: learningapi.MinigameQuestionSourceResultReasonsCode(r.Code), Message: r.Message})
	}
	return out, nil
}
