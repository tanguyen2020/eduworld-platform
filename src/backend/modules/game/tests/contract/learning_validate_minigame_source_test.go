package contract

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/minigame"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/modules"
	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

func exampleConfig(t *testing.T) minigame.Config {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "..", "specs", "examples", "minigame.math-g3-matching.json"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := minigame.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// T012 (F073-AC-02, facade test of the caller, AGENTS R29): the adapter of module game calls validateMinigameQuestionSource
// of the generated learning Fake under the platform scope of the release with a request valid against
// MinigameQuestionSourceCheck of the internal spec (the question source as authored), maps MinigameQuestionSourceResult
// (compatible, question types, available count, reasons), passes 4xx problems on and turns errors of module learning
// into port.ErrLearningUnavailable.
func TestLearningValidateMinigameSourceFacade(t *testing.T) {
	c := exampleConfig(t)
	reqSchema := internalSchema(t, "MinigameQuestionSourceCheck")
	respSchema := internalSchema(t, "MinigameQuestionSourceResult")
	var got learningapi.MinigameQuestionSourceCheck
	var gotCaller string
	answer := learningapi.MinigameQuestionSourceResult{Compatible: false, QuestionTypes: []learningapi.QuestionType{"matching", "single_choice"},
		AvailableCount: 12, Reasons: []struct {
			Code    learningapi.MinigameQuestionSourceResultReasonsCode `json:"code"`
			Message string                                              `json:"message"`
		}{{Code: learningapi.IncompatibleQuestionType, Message: "single_choice is not accepted by matching"}}}
	fake := &learningapi.Fake{ValidateMinigameQuestionSourceFunc: func(ctx context.Context, req learningapi.ValidateMinigameQuestionSourceRequestObject) (learningapi.ValidateMinigameQuestionSourceResponseObject, error) {
		got = *req.Body
		gotCaller = modcall.CallerFrom(ctx)
		return learningapi.ValidateMinigameQuestionSource200JSONResponse(answer), nil
	}}
	sc, err := authz.SystemPlatformScope("game-seed")
	if err != nil {
		t.Fatal(err)
	}
	ctx := tenancy.WithScope(context.Background(), sc)
	a := modules.LearningAttempts{F: fake}
	out, err := a.ValidateSource(ctx, c)
	if err != nil || out.Compatible || out.AvailableCount != 12 || len(out.QuestionTypes) != 2 || len(out.Reasons) != 1 {
		t.Fatalf("validate: %+v %v", out, err)
	}
	if gotCaller != "game" {
		t.Fatalf("caller %q", gotCaller)
	}
	b, _ := json.Marshal(got)
	var doc any
	_ = json.Unmarshal(b, &doc)
	if err := reqSchema.VisitJSON(doc); err != nil {
		t.Fatalf("MinigameQuestionSourceCheck of the internal spec: %v\n%s", err, b)
	}
	if got.MinigameType != "matching" || got.QuestionCount != 3 || got.QuestionSource.Kind != "selection_rule" || got.QuestionSource.Rule == nil ||
		got.QuestionSource.Rule.QuestionTypes == nil || len(*got.QuestionSource.Rule.QuestionTypes) != 1 {
		t.Fatalf("request: %s", b)
	}
	rb, _ := json.Marshal(answer)
	var rdoc any
	_ = json.Unmarshal(rb, &rdoc)
	if err := respSchema.VisitJSON(rdoc); err != nil {
		t.Fatalf("MinigameQuestionSourceResult of the internal spec: %v", err)
	}
	if err := c.CheckSource(out); !errors.Is(err, minigame.ErrSourceInvalid) {
		t.Fatalf("an incompatible answer is accepted: %v", err)
	}
	for name, cs := range map[string]struct {
		fail    error
		unavail bool
	}{
		"invalid":  {fail: problem.Invalid("/question_source", problem.SubInvalid, "x")},
		"internal": {fail: errors.New("boom"), unavail: true},
	} {
		fake.ValidateMinigameQuestionSourceFunc = func(context.Context, learningapi.ValidateMinigameQuestionSourceRequestObject) (learningapi.ValidateMinigameQuestionSourceResponseObject, error) {
			return nil, cs.fail
		}
		_, err := a.ValidateSource(ctx, c)
		var pe *problem.Error
		switch {
		case cs.unavail && !errors.Is(err, port.ErrLearningUnavailable):
			t.Errorf("%s: %v, want unavailable", name, err)
		case !cs.unavail && (!errors.As(err, &pe) || pe.Code != problem.ValidationFailed):
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := (modules.LearningAttempts{}).ValidateSource(ctx, c); !errors.Is(err, port.ErrLearningUnavailable) {
		t.Fatalf("no facade: %v", err)
	}
}

// T017 (F073-AC-06, AC-17, facade test of the caller): a free mini-game play calls createAttempt with content
// minigame_config pinned to the config version, mode minigame and context minigame_session of the play (valid against
// AttemptCreateRequest); a projection not ready (409) is port.ErrLearningUnavailable (503 + Retry-After upstream).
func TestLearningCreateMinigameAttemptFacade(t *testing.T) {
	student, tenant, config, version, play := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	key := uuid.Must(uuid.NewV7())
	reqSchema := internalSchema(t, "AttemptCreateRequest")
	var got learningapi.AttemptCreateRequest
	fake := &learningapi.Fake{CreateAttemptFunc: func(_ context.Context, req learningapi.CreateAttemptRequestObject) (learningapi.CreateAttemptResponseObject, error) {
		got = *req.Body
		return learningapi.CreateAttempt201JSONResponse{AttemptId: uuid.New(), Status: "in_progress", QuestionCount: 3, StartedAt: time.Now().UTC(),
			ContentVersionId: &version}, nil
	}}
	a := modules.LearningAttempts{F: fake}
	if _, err := a.CreateAttempt(studentCtx(t, student, tenant, key), port.LearningAttempt{StudentID: student, ContentID: config, ContentVersionID: &version,
		ContentType: port.ContentMiniGameConfig, MiniGamePlayID: &play}); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(got)
	var doc any
	_ = json.Unmarshal(b, &doc)
	if err := reqSchema.VisitJSON(doc); err != nil {
		t.Fatalf("AttemptCreateRequest: %v\n%s", err, b)
	}
	if got.Mode != learningapi.AttemptModeMinigame || got.Activity.ContentType != learningapi.ActivityRefContentTypeMinigameConfig ||
		got.Activity.ContentId != config || got.Activity.ContentVersionId == nil || *got.Activity.ContentVersionId != version ||
		got.Context.ContextType != learningapi.AttemptCreateRequestContextContextTypeMinigameSession || got.Context.ContextId != play || got.Context.StepId != nil {
		t.Fatalf("request: %s", b)
	}
	fake.CreateAttemptFunc = func(context.Context, learningapi.CreateAttemptRequestObject) (learningapi.CreateAttemptResponseObject, error) {
		return nil, problem.New(problem.Conflict, "projection not ready")
	}
	if _, err := a.CreateAttempt(studentCtx(t, student, tenant, key), port.LearningAttempt{StudentID: student, ContentID: config, ContentVersionID: &version,
		ContentType: port.ContentMiniGameConfig, MiniGamePlayID: &play}); !errors.Is(err, port.ErrLearningUnavailable) {
		t.Fatalf("projection not ready: %v", err)
	}
}
