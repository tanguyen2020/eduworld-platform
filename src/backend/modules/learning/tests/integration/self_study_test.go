//go:build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

func hintPath(attemptID string, position int) string {
	return "/learning/activity-attempts/" + attemptID + "/items/" + strconv.Itoa(position) + "/hints:next"
}

// F032-AC-01…AC-20, AC-34, AC-09 (US1, US2, US3, US7; quickstart 1–7, 13, 14; TC-LRN-002, 007, 009, 011, 012, 013,
// TC-TEN-001, 011, 024): a selection practice adapted to the level, immediate feedback with explanation and option
// feedback, retries, hints in order with hints_used, rapid_guess kept out of mastery and of the response, the review
// list, the friendly mastery view, the student-only routes of the session tenant.
func TestSelfStudyPracticeHintsRapidGuess(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	c.publishCurriculum(ctx, t, s.qc.version)
	k := c.selfKit(ctx, t)
	authztest.Cover(t, "revealNextHint", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listMyWrongQuestions", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getMyRecommendations", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getMyMastery", authztest.CrossTenant, authztest.RoleDenied)
	right, wrong := correctOption()
	body := map[string]any{"mode": "practice", "source": map[string]any{"kind": "selection", "topic_id": s.qc.topic, "question_count": 3}}
	key := idem()
	r := must(t, c.adhoc(t, hsAn, key, body), http.StatusCreated, "start")
	id := r.str("attempt_id")
	// AC-01, AC-04, AC-05: no deadline, three items at the difficulty of beginning (1–2: the fixtures are 2), no answer data.
	if r.str("mode") != "practice" || r.body["deadline_at"] != nil || len(r.items("items")) != 3 || len(answerKeysIn(r.raw)) > 0 {
		t.Fatalf("start: %s", r.raw)
	}
	if strings.Contains(r.raw, "Em đã thuộc") {
		t.Fatal("a hint reached the render payload")
	}
	// AC-08: the same key replays, another key resumes the open attempt of the same source.
	if again := must(t, c.adhoc(t, hsAn, key, body), http.StatusCreated, "replay"); again.str("attempt_id") != id ||
		again.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %s", again.raw)
	}
	if again := must(t, c.adhoc(t, hsAn, nil, body), http.StatusCreated, "resume"); again.str("attempt_id") != id {
		t.Fatalf("resume: %s", again.raw)
	}
	// AC-07: question_count out of 1–50, a wrong enum, a client score ignored.
	for _, bad := range []map[string]any{
		{"mode": "practice", "source": map[string]any{"kind": "selection", "topic_id": s.qc.topic, "question_count": 99}},
		{"mode": "bogus", "source": map[string]any{"kind": "selection", "topic_id": s.qc.topic}},
		{"mode": "diagnostic", "source": map[string]any{"kind": "diagnostic", "topic_id": s.qc.topic, "interleave": true}},
	} {
		if r := c.adhoc(t, hsAn, nil, bad); r.status != 422 || r.str("code") != "VALIDATION_FAILED" {
			t.Fatalf("invalid %v: %d %s", bad, r.status, r.raw)
		}
	}

	// US2: hints of position 1 in order, replay with the same key, exhausted, other students.
	hints := metricSum(t, observability.HintRequests(), nil)
	hk := idem()
	h1 := must(t, c.do(t, http.MethodPost, hintPath(id, 1), hsAn, hk, nil), http.StatusOK, "hint 1")
	if h1.str("level") != "1" || h1.str("total_levels") != "2" || h1.str("hint", "text") != "Em đã thuộc 7 × 7 chưa?" {
		t.Fatalf("hint 1: %s", h1.raw)
	}
	if rp := must(t, c.do(t, http.MethodPost, hintPath(id, 1), hsAn, hk, nil), http.StatusOK, "hint replay"); rp.str("level") != "1" ||
		rp.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("hint replay: %s", rp.raw)
	}
	if h2 := must(t, c.do(t, http.MethodPost, hintPath(id, 1), hsAn, idem(), nil), http.StatusOK, "hint 2"); h2.str("level") != "2" {
		t.Fatalf("hint 2: %s", h2.raw)
	}
	if h3 := c.do(t, http.MethodPost, hintPath(id, 1), hsAn, idem(), nil); h3.status != 409 || h3.str("code") != "HINTS_EXHAUSTED" {
		t.Fatalf("hint 3: %d %s", h3.status, h3.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT hints_revealed FROM assessment.attempt_items WHERE attempt_id = $1 AND position = 1`, id); n != 2 {
		t.Fatalf("hints_revealed %d", n)
	}
	// MONITORING §3: eduworld_hint_requests_total{subject,grade_band} moves once per opened level (not on the replay nor
	// the exhausted call); labels are a catalog code/"unknown" and a band.
	if got := metricSum(t, observability.HintRequests(), nil) - hints; got != 2 {
		t.Fatalf("hint_requests_total delta %v", got)
	}
	for _, b := range metricLabelValues(t, observability.HintRequests(), "grade_band", nil) {
		if !slices.Contains([]string{"g1_3", "g4_5", "g6_7", "g8_9", "unknown"}, b) {
			t.Fatalf("grade_band label %q", b)
		}
	}
	for _, sub := range []string{hsBinh, hsDung} {
		if r := c.do(t, http.MethodPost, hintPath(id, 1), sub, idem(), nil); r.status != 404 {
			t.Fatalf("%s opens a hint: %d %s", sub, r.status, r.raw)
		}
	}
	if r := c.do(t, http.MethodPost, hintPath(id, 1), qlcmA, idem(), nil); r.status != 403 {
		t.Fatalf("staff: %d", r.status)
	}
	if r := c.do(t, http.MethodPost, hintPath(id, 9), hsAn, idem(), nil); r.status != 422 {
		t.Fatalf("position 9: %d %s", r.status, r.raw)
	}

	// US1/US3: position 1 right after the hints, position 2 wrong with retries, position 3 a rapid guess.
	a1 := must(t, c.answerAfter(t, 10*time.Second, hsAn, id, 1, right), http.StatusOK, "answer 1")
	if a1.str("is_correct") != "true" || a1.str("hints_used") != "2" {
		t.Fatalf("answer 1: %s", a1.raw)
	}
	a2 := must(t, c.answerAfter(t, 10*time.Second, hsAn, id, 2, wrong), http.StatusOK, "answer 2")
	if a2.str("is_correct") != "false" || !strings.HasPrefix(a2.str("feedback", "text"), "54 là 6 × 9") {
		t.Fatalf("answer 2 (AC-02): %s", a2.raw)
	}
	must(t, c.do(t, http.MethodPost, hintPath(id, 2), hsAn, idem(), nil), http.StatusOK, "hint after the scored answer")
	for i := 0; i < 2; i++ {
		must(t, c.answerAfter(t, 5*time.Second, hsAn, id, 2, wrong), http.StatusOK, "retry")
	}
	a3 := must(t, c.answerAfter(t, 800*time.Millisecond, hsAn, id, 3, right), http.StatusOK, "rapid answer")
	if strings.Contains(a3.raw, "rapid") || a3.str("is_correct") != "true" {
		t.Fatalf("AC-20: %s", a3.raw)
	}
	if r := c.answerAfter(t, 5*time.Second, hsAn, id, 2, wrong); r.status != 409 || r.str("code") != "ANSWER_LIMIT_REACHED" {
		t.Fatalf("AC-03 limit: %d %s", r.status, r.raw)
	}
	for _, q := range []struct {
		where string
		want  int
	}{
		{`position = 1 AND attempt_no = 1 AND hints_used = 2 AND NOT rapid_guess AND response_ms = 10000`, 1},
		{`position = 2 AND attempt_no = 1 AND hints_used = 0 AND is_scored`, 1},
		{`position = 2 AND NOT is_scored`, 2},
		{`position = 3 AND rapid_guess AND response_ms = 800`, 1},
	} {
		if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempt_answers WHERE attempt_id = $1 AND `+q.where, id); n != q.want {
			t.Fatalf("attempt_answers %s: %d", q.where, n)
		}
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.learning_events WHERE event_type = 'question.answered'
		AND payload->>'attempt_id' = $1 AND ((payload->>'position' = '1' AND payload->>'hints_used' = '2')
		OR (payload->>'position' = '3' AND payload->>'rapid_guess' = 'true'))`, id); n != 2 {
		t.Fatalf("question.answered hints_used/rapid_guess: %d", n)
	}
	// AC-17: the attempt keeps the score of the hinted and the rapid answers.
	practice := map[string]string{"mode": "practice"}
	tooFastN, tooFastSum := metricSum(t, observability.AnswersTooFast(), practice), histogramSum(t, observability.AnswersTooFast(), practice)
	sub := must(t, c.submit(t, hsAn, id), http.StatusOK, "submit")
	if sub.str("evaluation", "score") != "2" {
		t.Fatalf("score: %s", sub.raw)
	}
	// ANTI_CHEAT §10: one observation of eduworld_answers_too_fast_ratio = 1 rapid of 3 scored answers (retries unscored).
	if n, s := metricSum(t, observability.AnswersTooFast(), practice)-tooFastN, histogramSum(t, observability.AnswersTooFast(), practice)-tooFastSum; n != 1 || s < 0.333 || s > 0.334 {
		t.Fatalf("eduworld_answers_too_fast_ratio: +%v observations, +%v sum; want 1, 1/3", n, s)
	}
	// ACTIVITY_ENGINE §11.1: activity.submitted carries the real rapid_guess_count (1 flagged scored answer), not 0.
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'activity.submitted'
		AND envelope->'payload'->>'attempt_id' = $1 AND (envelope->'payload'->>'rapid_guess_count')::int = 1`, id); n != 1 {
		t.Fatalf("outbox activity.submitted with rapid_guess_count 1: %d", n)
	}
	// AC-18: the rapid guess is no evidence (2 of 3 scored answers).
	k.gradedOnly(ctx, t, id)
	if m := c.masteryOf(ctx, t, c.users[hsAn].UserID, s.qc.lo); m.evidence != 2 {
		t.Fatalf("evidence %d", m.evidence)
	}
	// AC-21, AC-23: the first-try wrong question is in the review list of the student only, no answer data.
	wl := must(t, c.do(t, http.MethodGet, "/learning/wrong-questions", hsAn, nil, nil), http.StatusOK, "wrong list")
	items := wl.items("data")
	if len(items) != 1 || len(answerKeysIn(wl.raw)) > 0 || items[0]["wrong_count"] != float64(1) || items[0]["next_review_at"] == nil {
		t.Fatalf("wrong list: %s", wl.raw)
	}
	if r := must(t, c.do(t, http.MethodGet, "/learning/wrong-questions", hsBinh, nil, nil), http.StatusOK, "binh"); len(r.items("data")) != 0 {
		t.Fatalf("binh sees: %s", r.raw)
	}

	// AC-09: staff and another tenant in X-Tenant-Id.
	for _, op := range []string{"/learning/recommendations/me", "/learning/wrong-questions", "/learning/mastery/me"} {
		if r := c.do(t, http.MethodGet, op, adA, nil, nil); r.status != 403 {
			t.Fatalf("staff %s: %d", op, r.status)
		}
		if r := c.do(t, http.MethodGet, op, hsAn, hdr{"X-Tenant-Id": c.tenantB.String()}, nil); r.status != 403 || r.str("code") != "TENANT_MISMATCH" {
			t.Fatalf("tenant mismatch %s: %d %s", op, r.status, r.raw)
		}
	}

	// AC-34: grade 3 sees a level and an icon, no number; grade 7 sees the number; no TT27/TT22 label.
	mm := must(t, c.do(t, http.MethodGet, "/learning/mastery/me?subject_id="+s.qc.subject, hsAn, nil, nil), http.StatusOK, "my mastery")
	mi := mm.items("items")
	if mm.str("grade_band") != "g1_3" || len(mi) != 1 || mi[0]["value"] != nil || mi[0]["icon_code"] != "g1_3."+mi[0]["level"].(string) ||
		mi[0]["objective_code"] != "MATH.G3.NUM.MUL" {
		t.Fatalf("my mastery g1_3: %s", mm.raw)
	}
	pr := must(t, c.do(t, http.MethodGet, "/learning/progress/me", hsAn, nil, nil), http.StatusOK, "progress")
	if o := pr.items("objectives"); len(o) != 1 || !strings.HasPrefix(o[0]["icon_code"].(string), "g1_3.") {
		t.Fatalf("progress icon: %s", pr.raw)
	}
	var school uuid.UUID
	if err := c.platform.QueryRowContext(ctx, `SELECT school_id FROM ref.students WHERE student_id = $1`, c.users[hsDung].UserID).Scan(&school); err != nil {
		t.Fatal(err)
	}
	// tenancyguard:ok TEST fixture under learning_platform: a mastery of the grade-7 student of tenant B
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO learning.objective_masteries (objective_mastery_id, tenant_id, school_id, student_id,
		objective_id, value, level, evidence_count, distinct_days, last_evidence_at, last_evidence_date, model_version)
		VALUES ($1, $2, $3, $4, $5, 0.640, 'proficient', 5, 2, $6, $7::date, 'we-1.0')`, ids.New(), c.tenantB, school,
		c.users[hsDung].UserID, s.qc.lo2, c.clk.Now(), c.clk.Now().Format(time.DateOnly)); err != nil {
		t.Fatal(err)
	}
	dm := must(t, c.do(t, http.MethodGet, "/learning/mastery/me", hsDung, nil, nil), http.StatusOK, "dung mastery")
	if di := dm.items("items"); dm.str("grade_band") != "g6_7" || len(di) != 1 || di[0]["value"] != 0.64 || di[0]["icon_code"] != "g6_7.proficient" {
		t.Fatalf("my mastery g6_7: %s", dm.raw)
	}
	if strings.Contains(mm.raw+dm.raw, "TT27") || strings.Contains(mm.raw+dm.raw, "TT22") {
		t.Fatal("TT27/TT22 label")
	}
}
