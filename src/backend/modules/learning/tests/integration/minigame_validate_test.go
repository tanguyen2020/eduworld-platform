//go:build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func ptr[T any](v T) *T { return &v }

// F073-AC-02 (T011, TC-ACT-003, TC-SVC-005/006): validateMinigameQuestionSource, called by module game under the
// PlatformScope of a publish, reads platform content only and answers the question types the source may return, the
// questions available and the reasons of a refusal (incompatible type, no type filter, too few questions, question set
// not published); never a question nor an answer; a module outside x-allowed-callers is refused.
func TestValidateMinigameQuestionSource(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	platform := map[string]any{"owner_scope": "platform"}
	var matching []string
	for i := 0; i < 2; i++ {
		id := must(t, c.createQ(t, cmNoidung, qc.body(qbFixture(t, "matching"), platform)), http.StatusCreated, "matching").str("content_id")
		matching = append(matching, c.publishQuestion(ctx, t, id, 1))
	}
	single := c.published(ctx, t, cmNoidung, qc, platform)
	// A matching question of tenant A is not platform content: never counted for a platform configuration.
	tq := must(t, c.createQ(t, qlcmA, qc.body(qbFixture(t, "matching"), nil)), http.StatusCreated, "tenant matching").str("content_id")
	c.publishQuestion(ctx, t, tq, 1)

	f := c.mgFacade()
	pctx := authztest.Ctx(ctx, authztest.Platform("content_admin"))
	check := func(caller string, b learningapi.MinigameQuestionSourceCheck) (learningapi.ValidateMinigameQuestionSource200JSONResponse, error) {
		res, err := asModule(pctx, caller, "validateMinigameQuestionSource", "", func(ctx context.Context) (learningapi.ValidateMinigameQuestionSourceResponseObject, error) {
			return f.ValidateMinigameQuestionSource(ctx, learningapi.ValidateMinigameQuestionSourceRequestObject{Body: &b})
		})
		if err != nil {
			return learningapi.ValidateMinigameQuestionSource200JSONResponse{}, err
		}
		return res.(learningapi.ValidateMinigameQuestionSource200JSONResponse), nil
	}
	subject := uuid.MustParse(qc.subject)
	rule := func(mt string, count int, types ...learningapi.QuestionType) learningapi.MinigameQuestionSourceCheck {
		b := learningapi.MinigameQuestionSourceCheck{MinigameType: learningapi.MinigameType(mt), QuestionCount: count}
		b.QuestionSource.Kind = "selection_rule"
		b.QuestionSource.Rule = &struct {
			DifficultyMax *int                        `json:"difficulty_max,omitempty"`
			DifficultyMin *int                        `json:"difficulty_min,omitempty"`
			GradeLevels   *[]int                      `json:"grade_levels,omitempty"`
			ObjectiveIds  *[]uuid.UUID                `json:"objective_ids,omitempty"`
			QuestionTypes *[]learningapi.QuestionType `json:"question_types,omitempty"`
			SubjectId     *uuid.UUID                  `json:"subject_id,omitempty"`
			TopicIds      *[]uuid.UUID                `json:"topic_ids,omitempty"`
		}{SubjectId: &subject}
		if len(types) > 0 {
			b.QuestionSource.Rule.QuestionTypes = &types
		}
		return b
	}
	codes := func(r learningapi.ValidateMinigameQuestionSource200JSONResponse) []string {
		var out []string
		for _, x := range r.Reasons {
			out = append(out, string(x.Code))
		}
		return out
	}
	// Compatible: two published platform matching questions for a matching game of 2.
	r, err := check("game", rule("matching", 2, "matching"))
	if err != nil || !r.Compatible || r.AvailableCount != 2 || !slices.Equal(r.QuestionTypes, []learningapi.QuestionType{"matching"}) || len(r.Reasons) != 0 {
		t.Fatalf("compatible: %+v %v", r, err)
	}
	// Incompatible type in the rule.
	if r, err = check("game", rule("matching", 2, "matching", "single_choice")); err != nil || r.Compatible ||
		!slices.Equal(codes(r), []string{"incompatible_question_type"}) {
		t.Fatalf("incompatible: %+v %v", r, err)
	}
	// Not enough questions (the tenant question is not counted).
	if r, err = check("game", rule("matching", 3, "matching")); err != nil || r.Compatible || r.AvailableCount != 2 ||
		!slices.Equal(codes(r), []string{"insufficient_questions"}) {
		t.Fatalf("insufficient: %+v %v", r, err)
	}
	// No type filter: unbounded for matching, compatible for puzzle (every auto-graded type).
	if r, err = check("game", rule("matching", 2)); err != nil || r.Compatible || !slices.Contains(codes(r), "unbounded_question_types") ||
		len(r.QuestionTypes) != 0 {
		t.Fatalf("unbounded: %+v %v", r, err)
	}
	if r, err = check("game", rule("puzzle", 3)); err != nil || !r.Compatible || r.AvailableCount != 3 {
		t.Fatalf("puzzle: %+v %v", r, err)
	}
	// Question sets: not published, then published (compatible), a mixed one (incompatible).
	setsPath := "/cms/learning/question-sets"
	set := must(t, c.do(t, http.MethodPost, setsPath, cmNoidung, idem(), setBody(qc, staticDef(matching...))), http.StatusCreated, "set").str("content_id")
	bySet := func(mt string, id string, count int) learningapi.MinigameQuestionSourceCheck {
		b := learningapi.MinigameQuestionSourceCheck{MinigameType: learningapi.MinigameType(mt), QuestionCount: count}
		b.QuestionSource.Kind = "question_set"
		sid := uuid.MustParse(id)
		b.QuestionSource.QuestionSetId = &sid
		return b
	}
	if r, err = check("game", bySet("matching", set, 2)); err != nil || r.Compatible || !slices.Equal(codes(r), []string{"question_set_not_published"}) {
		t.Fatalf("set not published: %+v %v", r, err)
	}
	c.publishDoc(ctx, t, "question_sets", "question_set_versions", "question_set_id", set, 1)
	if r, err = check("game", bySet("matching", set, 2)); err != nil || !r.Compatible || r.AvailableCount != 2 ||
		!slices.Equal(r.QuestionTypes, []learningapi.QuestionType{"matching"}) {
		t.Fatalf("published set: %+v %v", r, err)
	}
	mixed := must(t, c.do(t, http.MethodPost, setsPath, cmNoidung, idem(), setBody(qc, staticDef(matching[0], single))), http.StatusCreated, "mixed set").str("content_id")
	c.publishDoc(ctx, t, "question_sets", "question_set_versions", "question_set_id", mixed, 1)
	if r, err = check("game", bySet("matching", mixed, 2)); err != nil || r.Compatible || !slices.Equal(codes(r), []string{"incompatible_question_type"}) {
		t.Fatalf("mixed set: %+v %v", r, err)
	}
	b := bySet("matching", set, 2)
	b.QuestionSource.Version = ptr(9)
	if r, err = check("game", b); err != nil || !slices.Equal(codes(r), []string{"question_set_not_published"}) {
		t.Fatalf("unknown set version: %+v %v", r, err)
	}
	// The answer never carries a question, an answer key or a scoring rule; only module game may call.
	for _, reason := range r.Reasons {
		if strings.Contains(reason.Message, "answer") {
			t.Fatalf("reason: %s", reason.Message)
		}
	}
	if _, err := check("chat", rule("matching", 2, "matching")); code(err) != problem.CallerNotAllowed {
		t.Fatalf("chat calls validateMinigameQuestionSource: %v", err)
	}
	if _, err := check("game", rule("drag_drop", 2, "matching")); code(err) != problem.ValidationFailed {
		t.Fatalf("unknown minigame type: %v", err)
	}
}

// minigameActivity is the config of a learning activity minigame wrapping configID (version nil = the current one).
func minigameActivity(configID uuid.UUID, version *int, override map[string]any) map[string]any {
	content := map[string]any{"minigame_config_id": configID, "minigame_config_version": version}
	if override != nil {
		content["question_set_override"] = override
	}
	return map[string]any{"content": content, "interaction": map[string]any{"feedback_display": "end_of_round"},
		"evaluation": map[string]any{"strategies": []string{"auto"}, "max_score": 10}, "feedback": map[string]any{"timing": "immediate"},
		"completion_rule": map[string]any{"mode": "graded"}}
}

// F073-AC-15 (T025, TC-ACT-003) and F073-AC-14: a minigame activity wrapping a configuration that is not in the projection
// (or a version number it does not have), or whose question set override returns question types the configuration does
// not accept, is 422 ACTIVITY_CONFIG_INVALID on save and on publish; a valid one is published and started through
// startActivityAttempt with the minigame plugin (version pinned, override as question source).
func TestMinigameActivityValidate(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	mc := mgSnapshot(uuid.New(), 1, "multiple_choice", selectionSource(s.qc.subject, "single_choice"), 3, nil)
	tf := mgSnapshot(uuid.New(), 1, "true_false", selectionSource(s.qc.subject, "true_false"), 3, nil)
	c.project(ctx, t, mc)
	c.project(ctx, t, tf)
	set := must(t, c.do(t, http.MethodPost, "/cms/learning/question-sets", qlcmA, idem(), setBody(s.qc, staticDef(s.questions...))),
		http.StatusCreated, "set").str("content_id")
	c.publishDoc(ctx, t, "question_sets", "question_set_versions", "question_set_id", set, 1)
	override := map[string]any{"question_set_id": set, "question_set_version": nil}
	create := func(cfg map[string]any) resp {
		return c.do(t, http.MethodPost, "/cms/learning/activities", qlcmA, idem(), activityBody(s.qc, "minigame", cfg))
	}
	invalid := func(r resp, field string) {
		t.Helper()
		if r.status != http.StatusUnprocessableEntity || r.str("code") != "ACTIVITY_CONFIG_INVALID" || !strings.Contains(r.raw, field) {
			t.Fatalf("want ACTIVITY_CONFIG_INVALID on %s: %d %s", field, r.status, r.raw)
		}
	}
	invalid(create(minigameActivity(uuid.New(), nil, nil)), "/config/content/minigame_config_id")
	invalid(create(minigameActivity(mc.configID, ptr(9), nil)), "/config/content/minigame_config_version")
	invalid(create(minigameActivity(tf.configID, nil, override)), "/config/content/question_set_override")
	invalid(create(minigameActivity(mc.configID, nil, map[string]any{"question_set_id": uuid.New()})), "/config/content/question_set_override")
	// Valid: published through the workflow (the publish check runs again), started by the student.
	id := must(t, create(minigameActivity(mc.configID, ptr(1), override)), http.StatusCreated, "valid minigame activity").str("activity_id")
	c.publishActivity(ctx, t, id, 1)
	st := must(t, c.start(t, hsAn, id, nil, nil), http.StatusCreated, "start minigame activity")
	if st.str("activity_type") != "minigame" || st.str("activity_id") != id || len(st.items("items")) != 3 ||
		st.str("render", "render_payload", "minigame_type") != "multiple_choice" ||
		st.str("render", "render_payload", "minigame_config_version_id") != mc.versionID.String() ||
		st.str("render", "render_payload", "config", "feedback_display") != "end_of_round" || len(answerKeysIn(st.raw)) > 0 {
		t.Fatalf("minigame activity attempt: %s", st.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE attempt_id = $1 AND source_kind = 'activity'
		AND minigame_config_version_id = $2 AND game_score = 0`, st.str("attempt_id"), mc.versionID); n != 1 {
		t.Fatal("the activity attempt pins the configuration version")
	}
	// A draft whose stored config lost its configuration is refused at publish.
	id2 := must(t, create(minigameActivity(mc.configID, nil, nil)), http.StatusCreated, "second").str("activity_id")
	// tenancyguard:ok TEST helper under learning_platform: points the draft at a configuration that is not projected
	if _, err := c.platform.ExecContext(ctx, `UPDATE content.activity_versions SET body = jsonb_set(body, '{config,content,minigame_config_id}', to_jsonb($2::text))
		WHERE activity_id = $1`, id2, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	c.approve(ctx, t, qlcmA, "activity", id2, 1)
	invalid(c.publishV(ctx, t, qlcmA, "activity", id2, 1, nil, nil), "/config/content/minigame_config_id")
}
