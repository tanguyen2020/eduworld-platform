package modules

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/minigame"
	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

var _ port.MiniGameSources = LearningAttempts{}

// ValidateSource implements port.MiniGameSources on validateMinigameQuestionSource of the learning facade (F073-AC-02):
// called on the publish path only, under the PlatformScope of the release. A facade that cannot answer is
// port.ErrLearningUnavailable (the publish fails, nothing is written).
func (l LearningAttempts) ValidateSource(ctx context.Context, c minigame.Config) (minigame.SourceCheck, error) {
	var out minigame.SourceCheck
	if l.F == nil {
		return out, port.ErrLearningUnavailable
	}
	body := learningapi.MinigameQuestionSourceCheck{MinigameType: learningapi.MinigameType(c.Type), QuestionCount: c.QuestionCount}
	// question_source of minigame-config.schema.json and of MinigameQuestionSourceCheck have the same JSON shape.
	raw, err := json.Marshal(c.Source)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &body.QuestionSource); err != nil {
		return out, problem.Invalid("question_source", problem.SubInvalid, "cannot be read")
	}
	op := modcall.Op{Caller: "game", Callee: "learning", Operation: "validateMinigameQuestionSource", Kind: modcall.Read,
		AllowedCallers: learningapi.AllowedCallers["validateMinigameQuestionSource"], ContextRequired: learningapi.ContextRequired["validateMinigameQuestionSource"]}
	resp, err := modcall.Call(ctx, op, func(ctx context.Context) (learningapi.ValidateMinigameQuestionSourceResponseObject, error) {
		return l.F.ValidateMinigameQuestionSource(ctx, learningapi.ValidateMinigameQuestionSourceRequestObject{Body: &body})
	})
	var pe *problem.Error
	switch {
	case errors.As(err, &pe) && problem.Status(pe.Code) < 500:
		return out, err
	case err != nil:
		return out, errors.Join(port.ErrLearningUnavailable, err)
	}
	v, ok := resp.(learningapi.ValidateMinigameQuestionSource200JSONResponse)
	if !ok {
		return out, port.ErrLearningUnavailable
	}
	out = minigame.SourceCheck{Compatible: v.Compatible, AvailableCount: v.AvailableCount}
	for _, t := range v.QuestionTypes {
		out.QuestionTypes = append(out.QuestionTypes, string(t))
	}
	for _, r := range v.Reasons {
		out.Reasons = append(out.Reasons, string(r.Code)+": "+r.Message)
	}
	return out, nil
}

var _ port.MiniGameSources = (*LateLearning)(nil)

// ValidateSource implements port.MiniGameSources.
func (l *LateLearning) ValidateSource(ctx context.Context, c minigame.Config) (minigame.SourceCheck, error) {
	l.mu.RLock()
	f := l.f
	l.mu.RUnlock()
	return LearningAttempts{F: f}.ValidateSource(ctx, c)
}
