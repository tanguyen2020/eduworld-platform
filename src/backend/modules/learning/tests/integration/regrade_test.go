//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

const regradePath = "/cms/learning/regrade-requests"

// correctedVersion publishes version 2 of the question of qv with the other option as the key (o_a91c).
func (c *curEnv) correctedVersion(ctx context.Context, t *testing.T, qv string) (string, string) {
	t.Helper()
	var qid string
	if err := c.platform.QueryRowContext(ctx, `SELECT question_id FROM content.question_versions WHERE question_version_id = $1`, qv).Scan(&qid); err != nil {
		t.Fatal(err)
	}
	must(t, c.do(t, http.MethodPost, "/cms/learning/questions/"+qid+"/versions", qlcmA, idem(), map[string]any{"from_version": 1}),
		http.StatusCreated, "v2")
	_, wrong := correctOption()
	if _, err := c.platform.ExecContext(ctx, `UPDATE content.question_versions SET answer_key = jsonb_build_object('correct_option_id', $2::text)
		WHERE question_id = $1 AND version_no = 2`, qid, wrong); err != nil {
		t.Fatal(err)
	}
	return qid, c.publishQuestion(ctx, t, qid, 2)
}

// quizAnswers runs the quiz as sub with one option per position and submits it.
func (c *curEnv) quizAnswers(t *testing.T, sub, activityID string, options ...string) string {
	t.Helper()
	id := must(t, c.start(t, sub, activityID, nil, nil), http.StatusCreated, "start").str("attempt_id")
	for i, o := range options {
		c.clk.Advance(answerTime) // read the question: not a rapid guess (F032-AC-16)
		must(t, c.answer(t, sub, id, i+1, map[string]any{"option_id": o}, nil), http.StatusOK, "answer")
	}
	must(t, c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":submit", sub, idem(), nil), http.StatusOK, "submit")
	return id
}

type evalRow struct {
	id, source, status string
	effective          bool
	score              float64
}

func (c *curEnv) evaluations(ctx context.Context, t *testing.T, attemptID string) []evalRow {
	t.Helper()
	rows, err := c.platform.QueryContext(ctx, `SELECT evaluation_id, source, status, effective, score::float8 FROM assessment.evaluations
		WHERE attempt_id = $1 ORDER BY created_at`, attemptID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []evalRow
	for rows.Next() {
		var e evalRow
		if err := rows.Scan(&e.id, &e.source, &e.status, &e.effective, &e.score); err != nil {
			t.Fatal(err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// F031-AC-18…AC-22 (TC-LRN-015, TC-TEN-002; quickstart 16–19): an academic manager asks the system to re-evaluate the
// attempts of a faulty question with the corrected key; the job supersedes each effective evaluation, keeps the attempt
// and its answers, emits attempt.re_evaluated and the mastery is recomputed with the new item results.
func TestRegradeUseNewKey(t *testing.T) {
	authztest.Cover(t, "createRegradeRequest", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getRegradeRequest", authztest.CrossTenant, authztest.RoleDenied)
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	k := c.masteryKit(ctx, t)
	right, wrong := correctOption()
	an := c.users[hsAn].UserID
	aAn := c.quizAnswers(t, hsAn, s.activityID, right, right, right)
	aBinh := c.quizAnswers(t, hsBinh, s.activityID, wrong, right, right)
	for _, a := range []string{aAn, aBinh} {
		k.deliver(ctx, t, k.graded(ctx, t, a))
	}
	before := c.masteryOf(ctx, t, an, s.qc.lo)
	_, corrected := c.correctedVersion(ctx, t, s.questions[0])
	var attemptScore float64
	if err := c.platform.QueryRowContext(ctx, `SELECT score::float8 FROM assessment.attempts WHERE attempt_id = $1`, aAn).Scan(&attemptScore); err != nil {
		t.Fatal(err)
	}
	answers := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempt_answers WHERE attempt_id = ANY($1::uuid[])`,
		"{"+aAn+","+aBinh+"}")

	body := func(extra map[string]any) map[string]any {
		b := map[string]any{"scope": "question_version", "question_version_id": s.questions[0], "action": "use_new_key",
			"corrected_version_id": corrected, "reason": "Đáp án câu 7 × 8 bị nhập sai, sửa theo version 2"}
		for k, v := range extra {
			b[k] = v
		}
		return b
	}
	// F031-AC-19: school_admin and roles without learning_content.re_evaluate → 403; subject outside the grant → 403;
	// tenant B does not see the question → 404.
	c.user("qlcm.mon.khac", authztest.Staff(c.tenantA, "academic_manager", authztest.ForSubject(uuid.New())).PrincipalData, true)
	for _, tc := range []struct {
		sub  string
		want int
	}{{adA, 403}, {noRoleA, 403}, {"qlcm.mon.khac", 403}, {qlcmB, 404}, {cmNoidung, 404}} {
		if r := c.do(t, http.MethodPost, regradePath, tc.sub, idem(), body(nil)); r.status != tc.want {
			t.Errorf("%s: %d, want %d (%s)", tc.sub, r.status, tc.want, r.raw)
		}
	}
	// F031-AC-18: validation — corrected version of another question, scope activity_version with use_new_key, exam_session.
	for _, b := range []map[string]any{body(map[string]any{"corrected_version_id": s.questions[1]}),
		{"scope": "activity_version", "activity_version_id": s.questions[0], "action": "use_new_key", "corrected_version_id": corrected,
			"reason": "Lý do đủ dài để kiểm tra"},
		{"scope": "exam_session", "exam_session_id": uuid.NewString(), "action": "drop_item", "reason": "Lý do đủ dài để kiểm tra"},
		body(map[string]any{"reason": "ngắn"})} {
		if r := c.do(t, http.MethodPost, regradePath, qlcmA, idem(), b); r.status != http.StatusUnprocessableEntity {
			t.Errorf("invalid body %v: %d %s", b["scope"], r.status, r.raw)
		}
	}
	audits := `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded' AND envelope->'payload'->>'action' = 'content.re_evaluated'`
	auditsBefore := count(ctx, t, c.migrator, audits)
	key := idem()
	r := must(t, c.do(t, http.MethodPost, regradePath, qlcmA, key, body(nil)), http.StatusAccepted, "create")
	id := r.str("regrade_request_id")
	if r.str("status") != "queued" || r.str("action") != "use_new_key" {
		t.Fatalf("created: %s", r.raw)
	}
	// F031-AC-22: same key → same request; another key while queued → 409 CONFLICT.
	if again := must(t, c.do(t, http.MethodPost, regradePath, qlcmA, key, body(nil)), http.StatusAccepted, "replay"); again.str("regrade_request_id") != id {
		t.Fatalf("replay: %s", again.raw)
	}
	if dup := c.do(t, http.MethodPost, regradePath, qlcmA, idem(), body(nil)); dup.status != http.StatusConflict || dup.str("code") != "CONFLICT" {
		t.Fatalf("second request: %d %s", dup.status, dup.raw)
	}
	if n := count(ctx, t, c.migrator, audits); n != auditsBefore+1 {
		t.Fatalf("content.re_evaluated audits = %d, want %d", n, auditsBefore+1)
	}
	// getRegradeRequest: in scope 200, tenant B 404, no permission 403.
	must(t, c.do(t, http.MethodGet, regradePath+"/"+id, qlcmA, nil, nil), http.StatusOK, "get")
	if g := c.do(t, http.MethodGet, regradePath+"/"+id, qlcmB, nil, nil); g.status != http.StatusNotFound {
		t.Fatalf("tenant B get: %d", g.status)
	}
	if g := c.do(t, http.MethodGet, regradePath+"/"+id, adA, nil, nil); g.status != http.StatusForbidden {
		t.Fatalf("school admin get: %d", g.status)
	}

	// F031-AC-20: the job re-evaluates both graded attempts.
	n, err := c.rgd.RunOnce(ctx)
	if err != nil || n != 2 {
		t.Fatalf("regrade run: %d %v", n, err)
	}
	g := must(t, c.do(t, http.MethodGet, regradePath+"/"+id, qlcmA, nil, nil), http.StatusOK, "get done")
	if g.str("status") != "completed" || g.body["affected_attempts"] != float64(2) || g.body["re_evaluated_attempts"] != float64(2) {
		t.Fatalf("finished: %s", g.raw)
	}
	for _, tc := range []struct {
		attempt string
		sign    float64
	}{{aAn, -1}, {aBinh, +1}} {
		ev := c.evaluations(ctx, t, tc.attempt)
		if len(ev) != 2 || ev[0].status != "superseded" || ev[0].effective || ev[1].source != "re_evaluation" || !ev[1].effective ||
			(ev[1].score-ev[0].score)*tc.sign <= 0 {
			t.Fatalf("evaluations of %s: %+v", tc.attempt, ev)
		}
	}
	var after float64
	if err := c.platform.QueryRowContext(ctx, `SELECT score::float8 FROM assessment.attempts WHERE attempt_id = $1`, aAn).Scan(&after); err != nil ||
		after != attemptScore {
		t.Fatalf("attempt changed: %v → %v (%v)", attemptScore, after, err)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempt_answers WHERE attempt_id = ANY($1::uuid[])`,
		"{"+aAn+","+aBinh+"}"); n != answers {
		t.Fatalf("answers changed: %d → %d", answers, n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.learning_events WHERE event_type = 'attempt.re_evaluated'
		AND actor_type = 'staff'`); n != 2 {
		t.Fatalf("attempt.re_evaluated learning events = %d", n)
	}
	if n := count(ctx, t, c.migrator, audits); n != auditsBefore+2 {
		t.Fatalf("tenant audit of the run missing: %d", n)
	}
	// F031-AC-22: a superseded evaluation is not editable (use case and database trigger).
	old := c.evaluations(ctx, t, aAn)[0].id
	scope, err := authz.EventScope(c.tenantA)
	if err != nil {
		t.Fatal(err)
	}
	err = c.uow.InTenantTx(ctx, scope, func(ctx context.Context) error { return write.RegradeRepo{}.Supersede(ctx, uuid.MustParse(old)) })
	var pe *problem.Error
	if !errors.As(err, &pe) || pe.Code != problem.EvaluationNotEditable {
		t.Fatalf("supersede a superseded evaluation: %v", err)
	}
	if _, err := c.platform.ExecContext(ctx, `UPDATE assessment.evaluations SET status = 'final', effective = true WHERE evaluation_id = $1`, old); err == nil ||
		!strings.Contains(err.Error(), "EVALUATION_NOT_EDITABLE") {
		t.Fatalf("trigger: %v", err)
	}
	// Nothing queued any more.
	if n, err := c.rgd.RunOnce(ctx); n != 0 || err != nil {
		t.Fatalf("idle run: %d %v", n, err)
	}

	// F031-AC-21: attempt.re_evaluated recomputes the mastery from the original learning events with the new item result.
	var raw []byte
	if err := c.migrator.QueryRowContext(ctx, `SELECT envelope FROM infra.outbox_events WHERE event_type = 'attempt.re_evaluated'
		AND envelope->'payload'->>'attempt_id' = $1`, aAn).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var env struct {
		Payload map[string]any `json:"payload"`
	}
	_ = json.Unmarshal(raw, &env)
	if env.Payload["action"] != "use_new_key" || env.Payload["regrade_request_id"] != id || env.Payload["student_id"] != an.String() {
		t.Fatalf("outbox payload: %v", env.Payload)
	}
	k.deliverTo(ctx, t, raw, "learning.mastery_on_graded")
	m := c.masteryOf(ctx, t, an, s.qc.lo)
	if m.value >= before.value || m.evidence != 3 {
		t.Fatalf("mastery after re-evaluation: %+v (before %+v)", m, before)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.mastery_history WHERE student_id = $1 AND source_kind = 're_evaluation'`, an); n != 1 {
		t.Fatalf("re_evaluation history rows = %d", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.learning_events WHERE event_type = 'question.answered' AND student_id = $1`, an); n != 3 {
		t.Fatalf("original evidence changed: %d", n)
	}
}

// F031-AC-18, AC-20 (platform): content_admin re-evaluates a platform question without a session tenant; the request has
// no tenant, tenant users do not see it, and the job reaches the attempts of the tenants through
// tenancy.Bypass(re_evaluation) with accept_all.
func TestRegradePlatformContent(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	pqID, _ := c.question(t, cmNoidung, s.qc, map[string]any{"owner_scope": "platform"})
	pq := c.publishQuestion(ctx, t, pqID, 1)
	c.assignResource(ctx, t, "question", pqID, c.tenantA)
	act := c.createActivity(t, qlcmA, activityBody(s.qc, "quiz", quizConfig(pq)))
	c.publishActivity(ctx, t, act, 1)
	_, wrong := correctOption()
	a := c.quizAnswers(t, hsAn, act, wrong)
	r := must(t, c.do(t, http.MethodPost, regradePath, cmNoidung, idem(), map[string]any{"scope": "question_version", "question_version_id": pq,
		"action": "accept_all", "reason": "Câu hỏi platform có hai đáp án đúng"}), http.StatusAccepted, "platform request")
	id := r.str("regrade_request_id")
	if _, has := r.body["tenant_id"]; has {
		t.Fatalf("platform request has a tenant: %s", r.raw)
	}
	if g := c.do(t, http.MethodGet, regradePath+"/"+id, qlcmA, nil, nil); g.status != http.StatusNotFound {
		t.Fatalf("tenant user reads a platform request: %d", g.status)
	}
	if n, err := c.rgd.RunOnce(ctx); n != 1 || err != nil {
		t.Fatalf("run: %d %v", n, err)
	}
	ev := c.evaluations(ctx, t, a)
	if len(ev) != 2 || ev[1].source != "re_evaluation" || ev[1].score <= ev[0].score {
		t.Fatalf("accept_all: %+v", ev)
	}
	g := must(t, c.do(t, http.MethodGet, regradePath+"/"+id, cmNoidung, nil, nil), http.StatusOK, "platform get")
	if g.str("status") != "completed" || g.body["re_evaluated_attempts"] != float64(1) {
		t.Fatalf("platform request: %s", g.raw)
	}
	if n := count(ctx, t, c.migrator, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'cross_tenant.accessed' AND envelope->'payload'->'metadata'->>'reason' = 're_evaluation'`); n == 0 {
		t.Fatal("no cross_tenant.accessed audit for the re_evaluation bypass")
	}
}
