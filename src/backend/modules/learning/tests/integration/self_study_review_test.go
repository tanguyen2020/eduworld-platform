//go:build integration

package integration

import (
	"context"
	"database/sql"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

// selfStudySetup adds to the attempt setup a second topic MATH.G3.T02 with objective MATH.G3.NUM.ADD (prerequisites
// MATH.G3.NUM.MUL and MATH.G3.NUM.DIV) and two published questions of it; the curriculum version is published.
func (c *curEnv) selfStudySetup(ctx context.Context, t *testing.T) (attSetup, qbCurriculum) {
	t.Helper()
	s := c.attemptSetup(ctx, t, nil)
	qc2 := s.qc
	qc2.topic = c.topic(t, cmNoidung, s.qc.version, s.qc.subject, "MATH.G3.T02", 3, "")
	qc2.lo = must(t, c.objective(t, cmNoidung, qc2.topic, "MATH.G3.NUM.ADD", map[string]any{"prerequisite_ids": []string{s.qc.lo, s.qc.lo2}}),
		http.StatusCreated, "LO3").str("objective_id")
	for i := 0; i < 2; i++ {
		c.published(ctx, t, qlcmA, qc2, nil)
	}
	c.publishCurriculum(ctx, t, s.qc.version)
	return s, qc2
}

// itemQuestions lists the question ids of the items of an attempt by position.
func (c *curEnv) itemQuestions(ctx context.Context, t *testing.T, attemptID string) []uuid.UUID {
	t.Helper()
	rows, err := c.platform.QueryContext(ctx, `SELECT q.question_id FROM assessment.attempt_items i
		JOIN content.question_versions q ON q.question_version_id = i.question_version_id WHERE i.attempt_id = $1 ORDER BY i.position`, attemptID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		out = append(out, id)
	}
	return out
}

type priorRow struct {
	value, prior sql.NullFloat64
	source       string
	level        string
	evidence     int
}

func (c *curEnv) priorOf(ctx context.Context, t *testing.T, student uuid.UUID, objective string) *priorRow {
	t.Helper()
	var p priorRow
	err := c.platform.QueryRowContext(ctx, `SELECT value::float8, prior_value::float8, prior_source, level, evidence_count
		FROM learning.objective_masteries WHERE student_id = $1 AND objective_id = $2 AND deleted_at IS NULL`, student, objective).
		Scan(&p.value, &p.prior, &p.source, &p.level, &p.evidence)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return &p
}

// F032-AC-21…AC-28 (US4, US5; quickstart 8–10; TC-LRN-005, 013, 014): the review list on the 1–3–7 schedule with
// siblings and its clearing; interleaved practice with at most two questions in a row from one topic; the warm-up
// diagnostic setting M0 per objective and its prerequisites without evidence, again only where there is still none.
func TestSelfStudyReviewInterleaveDiagnostic(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s, qc2 := c.selfStudySetup(ctx, t)
	k := c.selfKit(ctx, t)
	right, wrong := correctOption()
	an := c.users[hsAn].UserID

	// A practice of topic 1: position 1 wrong at the first try.
	p := must(t, c.adhoc(t, hsAn, nil, map[string]any{"mode": "practice", "source": map[string]any{"kind": "selection",
		"topic_id": s.qc.topic, "question_count": 3}}), http.StatusCreated, "practice").str("attempt_id")
	must(t, c.answerAfter(t, 10*time.Second, hsAn, p, 1, wrong), http.StatusOK, "wrong")
	must(t, c.answerAfter(t, 10*time.Second, hsAn, p, 2, right), http.StatusOK, "right")
	must(t, c.answerAfter(t, 10*time.Second, hsAn, p, 3, right), http.StatusOK, "right")
	must(t, c.submit(t, hsAn, p), http.StatusOK, "submit")
	k.gradedOnly(ctx, t, p)
	wrongQ := c.itemQuestions(ctx, t, p)[0]
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.wrong_question_entries WHERE student_id = $1 AND question_id = $2
		AND status = 'active' AND stage = 0 AND next_due_at > now()`, an, wrongQ); n != 1 {
		t.Fatalf("entry: %d", n)
	}
	// AC-22: the review the day after gets a sibling (same objective, type and difficulty, another question).
	c.clk.Advance(25 * time.Hour)
	retry := func() string {
		r := must(t, c.adhoc(t, hsAn, nil, map[string]any{"mode": "retry_wrong", "source": map[string]any{"kind": "retry_wrong"}}),
			http.StatusCreated, "retry_wrong")
		if r.str("mode") != "retry_wrong" || r.str("context_type") != "retry_wrong" || len(r.items("items")) != 1 || r.body["deadline_at"] != nil {
			t.Fatalf("retry_wrong: %s", r.raw)
		}
		id := r.str("attempt_id")
		if q := c.itemQuestions(ctx, t, id); q[0] == wrongQ {
			t.Fatalf("the original question instead of a sibling")
		}
		return id
	}
	r1 := retry()
	must(t, c.answerAfter(t, 10*time.Second, hsAn, r1, 1, right), http.StatusOK, "review 1")
	must(t, c.submit(t, hsAn, r1), http.StatusOK, "submit review 1")
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.wrong_question_entries WHERE student_id = $1 AND question_id = $2
		AND status = 'active' AND correct_streak = 1 AND stage = 1`, an, wrongQ); n != 1 {
		t.Fatal("first correct review")
	}
	// The same day again does not count; another day clears the entry (AC-21).
	r2 := retry()
	must(t, c.answerAfter(t, 10*time.Second, hsAn, r2, 1, right), http.StatusOK, "review same day")
	must(t, c.submit(t, hsAn, r2), http.StatusOK, "submit review 2")
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.wrong_question_entries WHERE student_id = $1 AND status = 'active'
		AND correct_streak = 1`, an); n != 1 {
		t.Fatal("same day counted twice")
	}
	c.clk.Advance(24 * time.Hour)
	r3 := retry()
	must(t, c.answerAfter(t, 10*time.Second, hsAn, r3, 1, right), http.StatusOK, "review next day")
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.wrong_question_entries WHERE student_id = $1 AND status = 'cleared'
		AND cleared_at IS NOT NULL`, an); n != 1 {
		t.Fatal("entry not cleared")
	}
	if r := must(t, c.do(t, http.MethodGet, "/learning/wrong-questions", hsAn, nil, nil), 200, "list"); len(r.items("data")) != 0 {
		t.Fatalf("cleared entry listed: %s", r.raw)
	}
	must(t, c.submit(t, hsAn, r3), http.StatusOK, "submit review 3")
	if r := c.adhoc(t, hsAn, nil, map[string]any{"mode": "retry_wrong", "source": map[string]any{"kind": "retry_wrong"}}); r.status != 422 ||
		r.str("code") != "QUESTION_POOL_INSUFFICIENT" {
		t.Fatalf("nothing to review: %d %s", r.status, r.raw)
	}

	// AC-24: interleaving two topics, at most two in a row from one topic; with only one learned topic → 422.
	il := must(t, c.adhoc(t, hsAn, nil, map[string]any{"mode": "practice", "source": map[string]any{"kind": "selection", "interleave": true,
		"topic_ids": []string{s.qc.topic, qc2.topic}, "question_count": 4}}), http.StatusCreated, "interleave")
	var topics []string
	rows, err := c.platform.QueryContext(ctx, `SELECT q.topic_id FROM assessment.attempt_items i JOIN content.question_versions q
		ON q.question_version_id = i.question_version_id WHERE i.attempt_id = $1 ORDER BY i.position`, il.str("attempt_id"))
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var tp string
		_ = rows.Scan(&tp)
		topics = append(topics, tp)
	}
	rows.Close()
	if len(topics) != 4 {
		t.Fatalf("interleave items: %v", topics)
	}
	for i := 2; i < len(topics); i++ {
		if topics[i] == topics[i-1] && topics[i] == topics[i-2] {
			t.Fatalf("three in a row: %v", topics)
		}
	}
	if r := c.adhoc(t, hsAn, nil, map[string]any{"mode": "practice", "source": map[string]any{"kind": "selection", "interleave": true,
		"subject_id": s.qc.subject}}); r.status != 422 || r.str("code") != "VALIDATION_FAILED" {
		t.Fatalf("one learned topic: %d %s", r.status, r.raw)
	}

	// US5: hs.binh without evidence takes the warm-up of topic 2 (its objective and the prerequisites first).
	binh := c.users[hsBinh].UserID
	diag := func(option string) string {
		r := must(t, c.adhoc(t, hsBinh, nil, map[string]any{"mode": "diagnostic", "source": map[string]any{"kind": "diagnostic",
			"topic_id": qc2.topic}}), http.StatusCreated, "diagnostic")
		if r.str("mode") != "diagnostic" || r.str("feedback_timing") != "after_submit" || len(r.items("items")) != 5 || r.body["deadline_at"] != nil {
			t.Fatalf("AC-25: %s", r.raw)
		}
		id := r.str("attempt_id")
		if h := c.do(t, http.MethodPost, hintPath(id, 1), hsBinh, idem(), nil); h.status != 403 || h.str("code") != "HINTS_NOT_ALLOWED" {
			t.Fatalf("AC-11: %d %s", h.status, h.raw)
		}
		for pos := 1; pos <= 5; pos++ {
			a := must(t, c.answerAfter(t, 10*time.Second, hsBinh, id, pos, option), http.StatusOK, "diagnostic answer")
			if a.body["is_correct"] != nil {
				t.Fatalf("feedback during a diagnostic: %s", a.raw)
			}
		}
		must(t, c.submit(t, hsBinh, id), http.StatusOK, "submit diagnostic")
		k.gradedOnly(ctx, t, id)
		return id
	}
	d1 := diag(right)
	// AC-27: the events of the diagnostic carry mode diagnostic.
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'activity.graded'
		AND envelope->'payload'->>'attempt_id' = $1 AND envelope->'payload'->>'mode' = 'diagnostic'`, d1); n != 1 {
		t.Fatal("activity.graded mode diagnostic")
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.learning_events WHERE payload->>'attempt_id' = $1
		AND context = 'diagnostic'`, d1); n < 6 {
		t.Fatalf("learning events with context diagnostic: %d", n)
	}
	// AC-26: M0 = clamp(0.15 + 0.6 × 1) = 0.75 for both diagnosed objectives; MATH.G3.NUM.DIV (prerequisite without
	// questions) receives it by propagation; no evidence, level not_started.
	for lo, source := range map[string]string{s.qc.lo: "diagnostic", qc2.lo: "diagnostic", s.qc.lo2: "propagated"} {
		p := c.priorOf(ctx, t, binh, lo)
		if p == nil || p.source != source || p.prior.Float64 != 0.75 || p.evidence != 0 || p.level != "not_started" {
			t.Fatalf("prior of %s: %+v", lo, p)
		}
	}
	// AC-28: after evidence on MATH.G3.NUM.MUL, a new diagnostic all wrong changes the others only.
	pr := must(t, c.adhoc(t, hsBinh, nil, map[string]any{"mode": "practice", "source": map[string]any{"kind": "selection",
		"objective_id": s.qc.lo, "question_count": 1}}), http.StatusCreated, "binh practice").str("attempt_id")
	must(t, c.answerAfter(t, 10*time.Second, hsBinh, pr, 1, right), http.StatusOK, "binh answer")
	must(t, c.submit(t, hsBinh, pr), http.StatusOK, "binh submit")
	k.gradedOnly(ctx, t, pr)
	mul := c.priorOf(ctx, t, binh, s.qc.lo)
	// The first evidence starts from the prior 0.75 (α = 1/3): 0.75 + (1 − 0.75)/3 = 0.833.
	if mul.evidence != 1 || mul.value.Float64 != 0.833 {
		t.Fatalf("evidence from the prior: %+v", mul)
	}
	diag(wrong)
	if p := c.priorOf(ctx, t, binh, s.qc.lo); p.prior.Float64 != 0.75 || p.value.Float64 != 0.833 || p.evidence != 1 {
		t.Fatalf("an objective with evidence changed: %+v", p)
	}
	if p := c.priorOf(ctx, t, binh, qc2.lo); p.prior.Float64 != 0.15 || p.source != "diagnostic" {
		t.Fatalf("new prior: %+v", p)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE student_id = $1 AND mode = 'diagnostic'
		AND status = 'graded'`, binh); n != 2 {
		t.Fatalf("history of the diagnostics: %d", n)
	}
}

// F032 T058 (OQ F030 owner 2026-11-20): a rule_based question set is drawn at the start with selection.Select and the
// seed of the attempt, each question version pinned; a rule the pool cannot satisfy is 422 QUESTION_POOL_INSUFFICIENT.
func TestSelfStudyQuestionSetRules(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s, qc2 := c.selfStudySetup(ctx, t)
	rules := func(count int) map[string]any {
		return map[string]any{"kind": "rule_based", "purpose": "exercise", "selection_rules": []any{
			map[string]any{"filters": map[string]any{"topic_ids": []string{qc2.topic}}, "count": count},
			map[string]any{"filters": map[string]any{"topic_ids": []string{s.qc.topic}}, "count": 1},
		}}
	}
	set := func(count int) string {
		id := must(t, c.do(t, http.MethodPost, "/cms/learning/question-sets", qlcmA, idem(), setBody(s.qc, rules(count))),
			http.StatusCreated, "rule set").str("content_id")
		c.publishDoc(ctx, t, "question_sets", "question_set_versions", "question_set_id", id, 1)
		return id
	}
	ok := set(2)
	r := must(t, c.adhoc(t, hsAn, nil, map[string]any{"mode": "practice", "source": map[string]any{"kind": "question_set", "question_set_id": ok}}),
		http.StatusCreated, "rule set start")
	if len(r.items("items")) != 3 || r.str("context_type") != "question_set" || len(answerKeysIn(r.raw)) > 0 {
		t.Fatalf("rule set: %s", r.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(DISTINCT q.question_id) FROM assessment.attempt_items i
		JOIN content.question_versions q ON q.question_version_id = i.question_version_id WHERE i.attempt_id = $1`, r.str("attempt_id")); n != 3 {
		t.Fatalf("distinct questions: %d", n)
	}
	short := set(5)
	if r := c.adhoc(t, hsAn, nil, map[string]any{"mode": "practice", "source": map[string]any{"kind": "question_set", "question_set_id": short}}); r.status != 422 ||
		r.str("code") != "QUESTION_POOL_INSUFFICIENT" {
		t.Fatalf("pool too small: %d %s", r.status, r.raw)
	}
}
