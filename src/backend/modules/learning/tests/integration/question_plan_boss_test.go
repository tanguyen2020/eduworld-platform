//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F103 T006 (research R1, R3) — the learning side of the boss and co-op facade: module game creates the question plan of
// one member for one boss round (mode boss, context boss_encounter of the encounter, attempt mode challenge, the
// questions of the published activity of the requirement), idempotent by the plan key, with render payloads only;
// scorePlanAnswer as module game records the answer for that student (attempt mode challenge) and reports rapid
// guesses; getLearningMaterialRenders returns the display blocks of a published learning material only (never a worked
// example or an activity), 404 for an unknown material; another module is refused.
func TestQuestionPlanBossAndMaterials(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	f := c.planFacade(ctx, t)
	an := authzPrincipal(c, hsAn)
	sctx := c.studentOf(ctx, hsAn)
	enc, key := uuid.New(), uuid.New()
	ctxType, mode := learningapi.QuestionPlanRequestContextTypeBossEncounter, learningapi.QuestionPlanRequestAttemptModeChallenge
	body := learningapi.QuestionPlanRequest{MatchId: key, Mode: learningapi.Boss, GradeLevel: 3, QuestionCount: 30, Participants: []uuid.UUID{an.UserID},
		ContextType: &ctxType, ContextId: &enc, AttemptMode: &mode}
	body.ActivityRef = &struct {
		ContentId uuid.UUID `json:"content_id"`
		Version   *int      `json:"version,omitempty"`
	}{ContentId: uuid.MustParse(s.activityID)}
	p, err := createPlan(sctx, f, "game", body)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Items) != len(s.questions) {
		t.Fatalf("the questions of the activity: %d of %d", len(p.Items), len(s.questions))
	}
	raw, _ := json.Marshal(p)
	if keys := answerKeysIn(string(raw)); len(keys) > 0 {
		t.Fatalf("answer data in the plan: %v", keys)
	}
	if again, err := createPlan(sctx, f, "game", body); err != nil || again.PlanId != p.PlanId {
		t.Fatalf("idempotent by the plan key: %v", err)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE mode = 'challenge' AND context_type = 'boss_encounter'
		AND context_id = $1 AND student_id = $2`, enc, an.UserID); n != 1 {
		t.Fatalf("one attempt mode challenge of the student: %d", n)
	}
	two := body
	two.MatchId, two.Participants = uuid.New(), []uuid.UUID{an.UserID, authzPrincipal(c, hsBinh).UserID}
	if _, err := createPlan(sctx, f, "game", two); code(err) != problem.ValidationFailed {
		t.Fatalf("a boss round has one participant: %v", err)
	}
	if _, err := createPlan(sctx, f, "chat", body); code(err) != problem.CallerNotAllowed {
		t.Fatalf("chat: %v", err)
	}
	// Grading for the student, as module game.
	c.clk.Advance(10 * time.Second)
	var r learningapi.QuestionResponse
	_, wrong := correctOption()
	_ = r.UnmarshalJSON([]byte(`{"option_id":"` + wrong + `"}`))
	sb := learningapi.PlanAnswerScoreRequest{Index: 0, StudentId: an.UserID, Response: r, ReceivedAt: c.clk.Now()}
	res, err := asModule(sctx, "game", "scorePlanAnswer", newKey(), func(ctx context.Context) (learningapi.ScorePlanAnswerResponseObject, error) {
		return f.ScorePlanAnswer(ctx, learningapi.ScorePlanAnswerRequestObject{PlanId: p.PlanId, Body: &sb})
	})
	if err != nil {
		t.Fatal(err)
	}
	sc := res.(learningapi.ScorePlanAnswer200JSONResponse)
	if sc.StudentId != an.UserID || sc.RapidGuess == nil || *sc.RapidGuess {
		t.Fatalf("graded for the student, 10 s is no rapid guess: %+v", sc)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempt_answers a JOIN assessment.attempts t ON t.attempt_id = a.attempt_id
		WHERE t.context_id = $1 AND t.student_id = $2`, enc, an.UserID); n != 1 {
		t.Fatalf("the learning record of the student: %d", n)
	}
	// Learning material renders (clue cards).
	mb := lessonBody(s.qc, map[string]any{"kind": "rich_text", "id": "b_clue", "content": rich("Cây cầu dài 12 m")}, workedExample("we_sol", 2, s.qc.lo))
	delete(mb, "completion_rule")
	mb["material_kind"] = "worked_example"
	m := must(t, c.do(t, http.MethodPost, materialsPath, qlcmA, idem(), mb), http.StatusCreated, "material")
	mid := m.str("content_id")
	c.publishDoc(ctx, t, "learning_materials", "learning_material_versions", "learning_material_id", mid, 1)
	mf := f
	mf.Materials = query.MaterialRenders{Session: c.read, Model: read.DocModel{}}
	renders := func(caller string, id uuid.UUID) (learningapi.GetLearningMaterialRendersResponseObject, error) {
		b := learningapi.GetLearningMaterialRendersJSONRequestBody{Refs: []struct {
			Id      uuid.UUID `json:"id"`
			Version *int      `json:"version,omitempty"`
		}{{Id: id}}}
		return asModule(sctx, caller, "getLearningMaterialRenders", "", func(ctx context.Context) (learningapi.GetLearningMaterialRendersResponseObject, error) {
			return mf.GetLearningMaterialRenders(ctx, learningapi.GetLearningMaterialRendersRequestObject{Body: &b})
		})
	}
	out, err := renders("game", uuid.MustParse(mid))
	if err != nil {
		t.Fatal(err)
	}
	data := out.(learningapi.GetLearningMaterialRenders200JSONResponse).Data
	rawR, _ := json.Marshal(data)
	if len(data) != 1 || len(data[0].Blocks) != 1 || !strings.Contains(string(rawR), "12 m") || strings.Contains(string(rawR), "worked_example") {
		t.Fatalf("display blocks only: %s", rawR)
	}
	if _, err := renders("game", uuid.New()); code(err) != problem.NotFound {
		t.Fatalf("unknown material: %v", err)
	}
	if _, err := renders("chat", uuid.MustParse(mid)); code(err) != problem.CallerNotAllowed {
		t.Fatalf("chat: %v", err)
	}
}
