//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// asModule calls a facade operation the way module caller does: pkg/modcall with the original Principal (the student
// on whose behalf module game acts), its TenantScope and an Idempotency-Key.
func asModule[T any](ctx context.Context, caller, op, key string, fn func(ctx context.Context) (T, error)) (T, error) {
	if key != "" {
		ctx = modcall.WithIdempotencyKey(ctx, key)
	}
	return modcall.Call(ctx, modcall.Op{Caller: caller, Callee: "learning", Operation: op, Kind: modcall.Write, Deadline: 30 * time.Second, // the suite runs on 2 cores: the production deadline is the caller's

		AllowedCallers: learningapi.AllowedCallers[op], ContextRequired: learningapi.ContextRequired[op]}, fn)
}

// F030-AC-25, AC-26 (US5, quickstart 20): module game opens quest/mini-game attempts through the facade (idempotent,
// mode checked), the student then works the attempt through the public API; getAttemptResult has no answer data and
// the TenantScope of the call decides (404 elsewhere); a module outside x-allowed-callers is refused.
func TestAttemptFacade(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	f := facade.Facade{AttemptOpener: c.att, AttemptResults: c.attq, Idempotency: idempotency.NewStore(c.app, 0, nil)}
	an := authz.NewPrincipal(c.users[hsAn])
	sctx := authztest.Ctx(ctx, an)
	questStep := uuid.New()
	body := learningapi.CreateAttemptJSONRequestBody{StudentId: an.UserID, Mode: learningapi.AttemptModeQuest,
		Activity: learningapi.ActivityRef{ContentType: learningapi.ActivityRefContentTypeActivity, ContentId: uuid.MustParse(s.activityID)}}
	body.Context.ContextType, body.Context.ContextId = "quest_step", questStep
	create := func(ctx context.Context, key string, b learningapi.CreateAttemptJSONRequestBody) (learningapi.CreateAttemptResponseObject, error) {
		return asModule(ctx, "game", "createAttempt", key, func(ctx context.Context) (learningapi.CreateAttemptResponseObject, error) {
			return f.CreateAttempt(ctx, learningapi.CreateAttemptRequestObject{Body: &b})
		})
	}
	key := newKey()
	res, err := create(sctx, key, body)
	if err != nil {
		t.Fatal(err)
	}
	created := res.(learningapi.CreateAttempt201JSONResponse)
	if created.Status != "in_progress" || created.QuestionCount != 3 || created.ContentVersionId == nil {
		t.Fatalf("created: %+v", created)
	}
	again, err := create(sctx, key, body)
	if err != nil || again.(learningapi.CreateAttempt201JSONResponse).AttemptId != created.AttemptId {
		t.Fatalf("idempotent create: %+v %v", again, err)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE attempt_id = $1 AND mode = 'quest'
		AND context_type = 'quest_step' AND context_id = $2`, created.AttemptId, questStep); n != 1 {
		t.Fatal("quest attempt row")
	}
	// Mode other than quest/minigame, mismatched context, another module.
	bad := body
	bad.Mode = learningapi.AttemptModePractice
	if _, err := create(sctx, newKey(), bad); code(err) != problem.ValidationFailed {
		t.Fatalf("practice through the facade: %v", err)
	}
	if _, err := asModule(sctx, "chat", "createAttempt", newKey(), func(ctx context.Context) (learningapi.CreateAttemptResponseObject, error) {
		return f.CreateAttempt(ctx, learningapi.CreateAttemptRequestObject{Body: &body})
	}); code(err) != problem.CallerNotAllowed {
		t.Fatalf("chat calls createAttempt: %v", err)
	}
	// The student works the attempt through the public API; module game reads the result (no answer data).
	id := created.AttemptId.String()
	right, _ := correctOption()
	must(t, c.answer(t, hsAn, id, 1, map[string]any{"option_id": right}, nil), http.StatusOK, "answer")
	must(t, c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":submit", hsAn, idem(), nil), http.StatusOK, "submit")
	result := func(ctx context.Context) (learningapi.GetAttemptResultResponseObject, error) {
		return asModule(ctx, "game", "getAttemptResult", "", func(ctx context.Context) (learningapi.GetAttemptResultResponseObject, error) {
			return f.GetAttemptResult(ctx, learningapi.GetAttemptResultRequestObject{AttemptId: created.AttemptId})
		})
	}
	rr, err := result(sctx)
	if err != nil {
		t.Fatal(err)
	}
	got := rr.(learningapi.GetAttemptResult200JSONResponse)
	if got.Status != "graded" || got.Score == nil || *got.Score != 1 || got.MaxScore == nil || *got.MaxScore != 3 || got.CorrectCount == nil ||
		*got.CorrectCount != 1 || got.QuestionCount != 3 || got.StudentId != an.UserID {
		t.Fatalf("result: %+v", got)
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'activity.submitted'
		AND envelope->'payload'->>'attempt_id' = $1 AND envelope->'payload'->>'context_type' = 'quest_step'`, id); n != 1 {
		t.Fatalf("activity.submitted with context quest_step: %d", n)
	}
	// AC-26: the TenantScope of another tenant does not see the attempt.
	dung := authztest.Ctx(ctx, authz.NewPrincipal(c.users[hsDung]))
	if _, err := result(dung); code(err) != problem.AttemptNotFound {
		t.Fatalf("tenant B result: %v", err)
	}
	if _, err := asModule(sctx, "competition", "getAttemptResult", "", func(ctx context.Context) (learningapi.GetAttemptResultResponseObject, error) {
		return f.GetAttemptResult(ctx, learningapi.GetAttemptResultRequestObject{AttemptId: created.AttemptId})
	}); code(err) != problem.CallerNotAllowed {
		t.Fatalf("competition reads a result: %v", err)
	}
}

// F030-AC-27 (US6, quickstart 21): ad-hoc practice from a published question set or a selection of the pool of the
// tenant, seed stored, QUESTION_POOL_INSUFFICIENT when the pool is too small.
func TestAttemptAdHoc(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	authztest.Cover(t, "startAdHocAttempt", authztest.CrossTenant, authztest.RoleDenied)
	adhoc := func(sub string, src map[string]any) resp {
		return c.do(t, http.MethodPost, "/learning/activity-attempts", sub, idem(), map[string]any{"mode": "practice", "source": src})
	}
	// selection by topic: the pool of tenant A has 3 published questions.
	r := must(t, adhoc(hsAn, map[string]any{"kind": "selection", "topic_id": s.qc.topic, "question_count": 3}), http.StatusCreated, "selection")
	if len(r.items("items")) != 3 || r.str("context_type") != "selection" || r.str("activity_type") != "quiz" || r.body["activity_id"] != nil ||
		len(answerKeysIn(r.raw)) > 0 {
		t.Fatalf("selection: %s", r.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE attempt_id = $1 AND source_kind = 'selection' AND seed > 0`,
		r.str("attempt_id")); n != 1 {
		t.Fatal("seed stored")
	}
	// Another source (the open attempt of the topic is resumed, F032-AC-08): the objective has 3 questions only.
	if r := adhoc(hsAn, map[string]any{"kind": "selection", "objective_id": s.qc.lo, "question_count": 10}); r.status != 422 ||
		r.str("code") != "QUESTION_POOL_INSUFFICIENT" {
		t.Fatalf("pool too small: %d %s", r.status, r.raw)
	}
	// Tenant B has no question in its pool.
	if r := adhoc(hsDung, map[string]any{"kind": "selection", "topic_id": s.qc.topic, "question_count": 1}); r.status != 422 {
		t.Fatalf("tenant B pool: %d %s", r.status, r.raw)
	}
	// A published static question set.
	set := must(t, c.do(t, http.MethodPost, "/cms/learning/question-sets", qlcmA, idem(), setBody(s.qc, staticDef(s.questions[0], s.questions[1]))),
		http.StatusCreated, "set").str("content_id")
	c.publishDoc(ctx, t, "question_sets", "question_set_versions", "question_set_id", set, 1)
	qs := must(t, adhoc(hsAn, map[string]any{"kind": "question_set", "question_set_id": set}), http.StatusCreated, "question set")
	if len(qs.items("items")) != 2 || qs.str("context_type") != "question_set" || qs.str("context_id") != set {
		t.Fatalf("question set: %s", qs.raw)
	}
	if again := must(t, adhoc(hsAn, map[string]any{"kind": "question_set", "question_set_id": set}), http.StatusCreated, "resume set"); again.str("attempt_id") != qs.str("attempt_id") {
		t.Fatalf("open practice on the same set is resumed: %s", again.raw)
	}
	if r := adhoc(hsDung, map[string]any{"kind": "question_set", "question_set_id": set}); r.status != http.StatusNotFound {
		t.Fatalf("tenant B set: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodPost, "/learning/activity-attempts", hsAn, idem(), map[string]any{"mode": "retry_wrong",
		"source": map[string]any{"kind": "retry_wrong"}}); r.status != 422 || r.str("code") != "QUESTION_POOL_INSUFFICIENT" {
		t.Fatalf("retry_wrong without wrong question (F032): %d %s", r.status, r.raw)
	}
	if r := adhoc(qlcmA, map[string]any{"kind": "selection", "topic_id": s.qc.topic}); r.status != http.StatusForbidden {
		t.Fatalf("staff: %d", r.status)
	}
}

// F076-AC-08, AC-09 (learning side of the F076 delta): module game opens a challenge attempt (mode challenge, context
// challenge_attempt) with the difficulty band of the level and the grade of the student; module learning draws only
// published questions of that grade inside the band (never another grade, never outside the band) and pins them; a
// band without question is QUESTION_POOL_INSUFFICIENT; mode challenge without band, another grade or another context
// is 422.
func TestAttemptFacadeChallenge(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil) // 3 questions of grade 3, difficulty 2 (the activity's own items)
	hard := c.published(ctx, t, qlcmA, s.qc, map[string]any{"difficulty": 4})
	c.published(ctx, t, qlcmA, s.qc, map[string]any{"difficulty": 3, "grade_levels": []int{4}}) // in the band, another grade
	c.published(ctx, t, qlcmA, s.qc, map[string]any{"difficulty": 5})                           // grade 3, outside the band
	f := facade.Facade{AttemptOpener: c.att, AttemptResults: c.attq, Idempotency: idempotency.NewStore(c.app, 0, nil)}
	an := authz.NewPrincipal(c.users[hsAn])
	sctx := authztest.Ctx(ctx, an)
	challenge := uuid.New()
	objective := uuid.MustParse(s.qc.lo)
	band := func(lo, hi, grade int, count *int) learningapi.CreateAttemptJSONRequestBody {
		b := learningapi.CreateAttemptJSONRequestBody{StudentId: an.UserID, Mode: learningapi.AttemptModeChallenge,
			Activity: learningapi.ActivityRef{ContentType: learningapi.ActivityRefContentTypeActivity, ContentId: uuid.MustParse(s.activityID),
				DifficultyMin: &lo, DifficultyMax: &hi, GradeLevel: &grade, QuestionCount: count, ObjectiveIds: &[]uuid.UUID{objective}}}
		b.Context.ContextType, b.Context.ContextId = "challenge_attempt", challenge
		return b
	}
	create := func(key string, b learningapi.CreateAttemptJSONRequestBody) (learningapi.CreateAttemptResponseObject, error) {
		return asModule(sctx, "game", "createAttempt", key, func(ctx context.Context) (learningapi.CreateAttemptResponseObject, error) {
			return f.CreateAttempt(ctx, learningapi.CreateAttemptRequestObject{Body: &b})
		})
	}
	one := 1
	body := band(3, 4, 3, &one)
	key := newKey()
	res, err := create(key, body)
	if err != nil {
		t.Fatal(err)
	}
	created := res.(learningapi.CreateAttempt201JSONResponse)
	if created.QuestionCount != 1 {
		t.Fatalf("created: %+v", created)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE attempt_id = $1 AND mode = 'challenge'
		AND context_type = 'challenge_attempt' AND context_id = $2 AND student_id = $3`, created.AttemptId, challenge, an.UserID); n != 1 {
		t.Fatal("challenge attempt row of the student")
	}
	// The only question of grade 3 in the band 3–4 is the difficulty 4 one (SC-002: never another grade or band).
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempt_items WHERE attempt_id = $1 AND question_version_id = $2
		AND deleted_at IS NULL`, created.AttemptId, hard); n != 1 {
		t.Fatal("the item is the grade 3 question of the band")
	}
	again, err := create(key, body)
	if err != nil || again.(learningapi.CreateAttempt201JSONResponse).AttemptId != created.AttemptId {
		t.Fatalf("idempotent create: %+v %v", again, err)
	}
	// Two questions asked: the band holds one question of the grade.
	two := 2
	if _, err := create(newKey(), band(3, 4, 3, &two)); code(err) != problem.QuestionPoolInsufficient {
		t.Fatalf("band too small: %v", err)
	}
	// Mode challenge without a band, with another grade than the student's, with a quest context: 422.
	noBand := band(3, 4, 3, &one)
	noBand.Activity.DifficultyMin = nil
	if _, err := create(newKey(), noBand); code(err) != problem.ValidationFailed {
		t.Fatalf("no band: %v", err)
	}
	if _, err := create(newKey(), band(3, 4, 4, &one)); code(err) != problem.ValidationFailed {
		t.Fatalf("another grade: %v", err)
	}
	if _, err := create(newKey(), band(4, 3, 3, &one)); code(err) != problem.ValidationFailed {
		t.Fatalf("min > max: %v", err)
	}
	wrongCtx := band(3, 4, 3, &one)
	wrongCtx.Context.ContextType = "quest_step"
	if _, err := create(newKey(), wrongCtx); code(err) != problem.ValidationFailed {
		t.Fatalf("quest context for a challenge: %v", err)
	}
}
