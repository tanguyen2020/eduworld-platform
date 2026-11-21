//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// planFacade is the facade of the question plans over the attempt engine of the test environment.
func (c *curEnv) planFacade(ctx context.Context, t *testing.T) facade.Facade {
	t.Helper()
	plans := command.QuestionPlans{Attempts: c.att, Plans: write.QuestionPlanRepo{}, Recent: write.SelfStudyRepo{},
		Config: command.DefaultQuestionPlanConfig()}
	return facade.Facade{Plans: plans, Reveals: query.QuestionPlans{Session: c.read, Model: read.QuestionPlanModel{}, Delivery: c.delivery(ctx, t),
		Clock: c.clk.Now}}
}

func createPlan(ctx context.Context, f facade.Facade, caller string, body learningapi.QuestionPlanRequest) (learningapi.QuestionPlan, error) {
	res, err := asModule(ctx, caller, "createQuestionPlan", newKey(), func(ctx context.Context) (learningapi.CreateQuestionPlanResponseObject, error) {
		return f.CreateQuestionPlan(ctx, learningapi.CreateQuestionPlanRequestObject{Body: &body})
	})
	if err != nil {
		return learningapi.QuestionPlan{}, err
	}
	return learningapi.QuestionPlan(res.(learningapi.CreateQuestionPlan201JSONResponse)), nil
}

func scorePlan(ctx context.Context, f facade.Facade, plan uuid.UUID, index int, student uuid.UUID, response string, at time.Time) (learningapi.PlanAnswerScore, error) {
	var r learningapi.QuestionResponse
	_ = r.UnmarshalJSON([]byte(response))
	body := learningapi.PlanAnswerScoreRequest{Index: index, StudentId: student, Response: r, ReceivedAt: at}
	res, err := asModule(ctx, "competition", "scorePlanAnswer", newKey(), func(ctx context.Context) (learningapi.ScorePlanAnswerResponseObject, error) {
		return f.ScorePlanAnswer(ctx, learningapi.ScorePlanAnswerRequestObject{PlanId: plan, Body: &body})
	})
	if err != nil {
		return learningapi.PlanAnswerScore{}, err
	}
	return learningapi.PlanAnswerScore(res.(learningapi.ScorePlanAnswer200JSONResponse)), nil
}

func revealPlan(ctx context.Context, f facade.Facade, plan uuid.UUID, index int, closed time.Time) (learningapi.PlanItemReveal, error) {
	res, err := asModule(ctx, "competition", "revealPlanItem", "", func(ctx context.Context) (learningapi.RevealPlanItemResponseObject, error) {
		return f.RevealPlanItem(ctx, learningapi.RevealPlanItemRequestObject{PlanId: plan, Index: index,
			Params: learningapi.RevealPlanItemParams{ClosedAt: closed}})
	})
	if err != nil {
		return learningapi.PlanItemReveal{}, err
	}
	return learningapi.PlanItemReveal(res.(learningapi.RevealPlanItem200JSONResponse)), nil
}

// F105-AC-01, F105-AC-02 (T010, TC-LRN-007) and the learning side of the facade test (T011): createQuestionPlan picks
// published questions of the tenant for the grade and subject only, avoids what a participant met in 7 days, pins the
// versions, sends render payloads without any answer data and is idempotent by match; a module outside
// x-allowed-callers is refused; another tenant cannot use the students of tenant A.
func TestQuestionPlanCreate(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil) // 3 published grade-3 questions of tenant A; hs.an.3a, hs.binh.3a (A), hs.dung.7a (B)
	for i := 0; i < 2; i++ {
		s.questions = append(s.questions, c.published(ctx, t, qlcmA, s.qc, nil))
	}
	grade7 := c.published(ctx, t, qlcmA, s.qc, map[string]any{"grade_levels": []int{7}})
	c.question(t, qlcmA, s.qc, nil) // a draft: never delivered
	f := c.planFacade(ctx, t)
	an, binh := authzPrincipal(c, hsAn), authzPrincipal(c, hsBinh)
	sctx := c.studentOf(ctx, hsAn)
	subject := uuid.MustParse(s.qc.subject)
	ttl := 20
	body := learningapi.QuestionPlanRequest{MatchId: uuid.New(), Mode: "battle_1v1", GradeLevel: 3, SubjectId: &subject, QuestionCount: 3,
		TimePerQuestionSeconds: &ttl, Participants: []uuid.UUID{an.UserID, binh.UserID}}
	p1, err := createPlan(sctx, f, "competition", body)
	if err != nil {
		t.Fatal(err)
	}
	if len(p1.Items) != 3 || p1.PlanVersion == nil || len(*p1.PlanVersion) > 64 {
		t.Fatalf("plan: %+v", p1)
	}
	raw, _ := json.Marshal(p1)
	if keys := answerKeysIn(string(raw)); len(keys) > 0 {
		t.Fatalf("answer data in the plan: %v", keys)
	}
	published := map[string]bool{}
	for _, q := range s.questions {
		published[q] = true
	}
	first := map[uuid.UUID]bool{}
	for i, it := range p1.Items {
		if it.Index != i || !published[it.QuestionVersionId.String()] || it.QuestionVersionId.String() == grade7 || it.TimeLimitSeconds != 20 {
			t.Fatalf("item %d: %+v", i, it)
		}
		if it.RenderPayload["stem"] == nil {
			t.Fatalf("item %d has no stem", i)
		}
		first[it.QuestionVersionId] = true
	}
	again, err := createPlan(sctx, f, "competition", body)
	if err != nil || again.PlanId != p1.PlanId || *again.PlanVersion != *p1.PlanVersion {
		t.Fatalf("createQuestionPlan is not idempotent by match: %+v %v", again, err)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.question_plans WHERE match_id = $1`, body.MatchId); n != 1 {
		t.Fatalf("%d plans for one match", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE mode = 'competition' AND context_type = 'match'
		AND context_id = $1`, body.MatchId); n != 2 {
		t.Fatalf("%d match attempts, want one per participant", n)
	}
	// avoid_recent: the next match of the same students draws the two questions they have not met.
	body2 := body
	body2.MatchId, body2.QuestionCount = uuid.New(), 2
	p2, err := createPlan(sctx, f, "competition", body2)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range p2.Items {
		if first[it.QuestionVersionId] {
			t.Fatalf("question %s met in the last 7 days drawn again", it.QuestionVersionId)
		}
	}
	// x-allowed-callers: [competition].
	if _, err := createPlan(sctx, f, "chat", body); code(err) != problem.CallerNotAllowed {
		t.Fatalf("chat creates a plan: %v", err)
	}
	// Tenant B cannot plan for students of tenant A.
	body3 := body
	body3.MatchId = uuid.New()
	if _, err := createPlan(c.studentOf(ctx, hsDung), f, "competition", body3); code(err) != problem.ValidationFailed {
		t.Fatalf("plan of tenant B for students of tenant A: %v", err)
	}
	// Not enough grade-3 questions of the subject → QUESTION_POOL_INSUFFICIENT (the room aborts with plan_failure).
	body4 := body
	body4.MatchId, body4.QuestionCount = uuid.New(), 30
	if _, err := createPlan(sctx, f, "competition", body4); code(err) != problem.QuestionPoolInsufficient {
		t.Fatalf("pool too small: %v", err)
	}
}

// F105-AC-04, F105-AC-09 (T020) and revealPlanItem (F105-AC-05): the answer is graded with the key of the pinned
// version and recorded once per (plan, index, student) as attempt_answer of the match attempt (mode competition) with
// its question.answered event, even under concurrent calls; late answers are not scored; the correct answer is revealed
// only once the question closed and only in the tenant of the plan.
func TestQuestionPlanScoreAndReveal(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	f := c.planFacade(ctx, t)
	an, binh := authzPrincipal(c, hsAn), authzPrincipal(c, hsBinh)
	sctx := c.studentOf(ctx, hsAn)
	subject := uuid.MustParse(s.qc.subject)
	body := learningapi.QuestionPlanRequest{MatchId: uuid.New(), Mode: "battle_1v1", GradeLevel: 3, SubjectId: &subject, QuestionCount: 3,
		Participants: []uuid.UUID{an.UserID, binh.UserID}}
	p, err := createPlan(sctx, f, "competition", body)
	if err != nil {
		t.Fatal(err)
	}
	right, wrong := correctOption()
	now := c.clk.Now()
	res, err := scorePlan(sctx, f, p.PlanId, 0, an.UserID, `{"option_id":"`+right+`"}`, now)
	if err != nil || !res.IsCorrect || res.ScoreFraction != 1 || res.Late == nil || *res.Late || res.AttemptAnswerId == nil {
		t.Fatalf("right answer: %+v %v", res, err)
	}
	// A second call (retry) keeps the first answer, even with another response.
	again, err := scorePlan(sctx, f, p.PlanId, 0, an.UserID, `{"option_id":"`+wrong+`"}`, now.Add(time.Second))
	if err != nil || !again.IsCorrect || again.ScoreFraction != 1 {
		t.Fatalf("second answer replaced the first: %+v %v", again, err)
	}
	res, err = scorePlan(sctx, f, p.PlanId, 0, binh.UserID, `{"option_id":"`+wrong+`"}`, now)
	if err != nil || res.IsCorrect || res.ScoreFraction != 0 {
		t.Fatalf("wrong answer: %+v %v", res, err)
	}
	// Concurrent calls for one (plan, index, student): one row.
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := scorePlan(sctx, f, p.PlanId, 1, an.UserID, `{"option_id":"`+right+`"}`, now); err != nil {
				t.Errorf("concurrent score: %v", err)
			}
		}()
	}
	wg.Wait()
	rows := func(student uuid.UUID, pos int) int {
		return count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempt_answers aa JOIN assessment.attempts a ON a.attempt_id = aa.attempt_id
			WHERE a.mode = 'competition' AND a.context_id = $1 AND a.student_id = $2 AND aa.position = $3`, body.MatchId, student, pos)
	}
	if rows(an.UserID, 1) != 1 || rows(binh.UserID, 1) != 1 || rows(an.UserID, 2) != 1 {
		t.Fatalf("attempt_answers: an q1 %d, binh q1 %d, an q2 %d", rows(an.UserID, 1), rows(binh.UserID, 1), rows(an.UserID, 2))
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.learning_events WHERE event_type = 'question.answered'
		AND context = 'competition' AND student_id = $1`, an.UserID); n != 2 {
		t.Fatalf("%d question.answered events with context competition", n)
	}
	// Late: received after expires_at → recorded, not scored.
	late, err := scorePlan(sctx, f, p.PlanId, 2, binh.UserID, `{"option_id":"`+right+`"}`, p.ExpiresAt.Add(time.Minute))
	if err != nil || late.Late == nil || !*late.Late || late.IsCorrect || late.ScoreFraction != 0 {
		t.Fatalf("late answer: %+v %v", late, err)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempt_answers aa JOIN assessment.attempts a ON a.attempt_id = aa.attempt_id
		WHERE a.context_id = $1 AND a.student_id = $2 AND aa.position = 3 AND NOT aa.is_scored`, body.MatchId, binh.UserID); n != 1 {
		t.Fatal("late answer not recorded unscored")
	}
	// Not a participant, index out of the plan, response of another type.
	if _, err := scorePlan(sctx, f, p.PlanId, 0, uuid.New(), `{"option_id":"`+right+`"}`, now); code(err) != problem.NotFound {
		t.Fatalf("stranger: %v", err)
	}
	if _, err := scorePlan(sctx, f, p.PlanId, 9, an.UserID, `{"option_id":"`+right+`"}`, now); code(err) != problem.ValidationFailed {
		t.Fatalf("index out of plan: %v", err)
	}
	if _, err := scorePlan(sctx, f, p.PlanId, 2, an.UserID, `{"value":true}`, now); code(err) != problem.InvalidResponse {
		t.Fatalf("response of another type: %v", err)
	}
	// Reveal: 409 before the question closed, the right option after; 404 in tenant B.
	if _, err := revealPlan(sctx, f, p.PlanId, 0, c.clk.Now().Add(time.Minute)); code(err) != problem.Conflict {
		t.Fatalf("reveal before close: %v", err)
	}
	rv, err := revealPlan(sctx, f, p.PlanId, 0, c.clk.Now())
	if err != nil || rv.CorrectResponse == nil {
		t.Fatalf("reveal: %+v %v", rv, err)
	}
	cr, _ := rv.CorrectResponse.MarshalJSON()
	if string(cr) != `{"option_id":"`+right+`"}` {
		t.Fatalf("correct response %s", cr)
	}
	if _, err := revealPlan(c.studentOf(ctx, hsDung), f, p.PlanId, 0, c.clk.Now()); code(err) != problem.NotFound {
		t.Fatalf("reveal in tenant B: %v", err)
	}
}
