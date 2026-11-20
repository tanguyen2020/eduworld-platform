//go:build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// F063 backend fills (F030 T058, F032 T055): QuestionRender.hint_count is the number of hint levels of the pinned version
// (never the hints); AttemptReview.recommendations[] carries the recommendation_id of the active recommendations about
// the weak objectives.
func TestRenderHintCountAndReviewRecommendations(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	c.enroll(ctx, t, hsAn, 3)
	hint := func(text string) map[string]any {
		return map[string]any{"blocks": []any{map[string]any{"type": "paragraph", "text": text}}}
	}
	q1 := c.published(ctx, t, qlcmA, qc, map[string]any{"hints": []any{hint("Nhớ bảng nhân 7"), hint("7 × 8 = 7 × 4 × 2")}})
	q2 := c.published(ctx, t, qlcmA, qc, map[string]any{"hints": []any{hint("Một gợi ý")}})
	act := c.createActivity(t, qlcmA, activityBody(qc, "quiz", quizConfig(q1, q2)))
	c.publishActivity(ctx, t, act, 1)
	a := must(t, c.start(t, hsAn, act, nil, nil), http.StatusCreated, "start")
	items := a.items("items")
	if len(items) != 2 || items[0]["hint_count"] != float64(2) || items[1]["hint_count"] != float64(1) {
		t.Fatalf("hint_count: %s", a.raw)
	}
	if leaks := answerKeysIn(a.raw); slices.Contains(leaks, "hints") {
		t.Fatalf("hints leaked: %v", leaks)
	}
	// A recommendation of a lesson about the objective of the wrong answer.
	rec, lesson := ids.New(), ids.New()
	d := c.users[hsAn]
	var school string
	if err := c.platform.QueryRowContext(ctx, `SELECT school_id FROM ref.students WHERE student_id = $1`, d.UserID).Scan(&school); err != nil {
		t.Fatal(err)
	}
	// tenancyguard:ok TEST fixture under learning_platform: an active recommendation the engine of F032 would write
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO learning.rm_student_recommendations (recommendation_id, tenant_id, school_id, student_id,
		rule_code, kind, target_type, target_id, objective_ids, reason, reason_code, priority, generated_at, expires_at)
		VALUES ($1, $2, $3, $4, 'R6', 'lesson', 'lesson', $5, ARRAY[$6::uuid], 'next_topic', 'rec.next_topic', 4, now(), now() + interval '7 days')`,
		rec, *d.TenantID, school, d.UserID, lesson, qc.lo); err != nil {
		t.Fatal(err)
	}
	_, wrong := correctOption()
	right, _ := correctOption()
	for pos, opt := range []string{wrong, right} {
		c.clk.Advance(answerTime)
		must(t, c.answer(t, hsAn, a.str("attempt_id"), pos+1, map[string]any{"option_id": opt}, nil), http.StatusOK, "answer")
	}
	must(t, c.do(t, http.MethodPost, "/learning/activity-attempts/"+a.str("attempt_id")+":submit", hsAn, idem(), nil), http.StatusOK, "submit")
	rv := must(t, c.do(t, http.MethodGet, "/learning/activity-attempts/"+a.str("attempt_id")+"/review", hsAn, nil, nil), http.StatusOK, "review")
	recs := rv.items("recommendations")
	if len(recs) != 1 || recs[0]["recommendation_id"] != rec.String() || recs[0]["kind"] != "lesson" || recs[0]["ref_id"] != lesson.String() {
		t.Fatalf("review recommendations: %s", rv.raw)
	}
}
