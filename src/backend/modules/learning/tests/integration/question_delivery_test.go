//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F022-AC-22, AC-24, AC-25, AC-26, AC-28 (TC-ACT-002, TC-TEN-019): the port Delivery serves only the current
// release of questions visible to the student's tenant, renders without any answer data, grades on the server and
// the pool cache is dropped by the release consumer.
func TestQuestionDelivery(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	vk := valkeyClient(ctx, t)
	d := query.Delivery{Session: c.read, Reader: read.Delivery{}, Cache: cache.Pool{C: vk}, Registry: question.Full(), Schema: c.schema,
		Metrics: observability.QuestionCounter{}}
	student := authztest.Ctx(ctx, authztest.Student(c.tenantA))
	studentB := authztest.Ctx(ctx, authztest.Student(c.tenantB))

	published := map[string]string{} // version id → type
	var single, numeric, ordering string
	for _, typ := range []string{"single_choice", "numeric", "ordering", "true_false"} {
		r := must(t, c.createQ(t, qlcmA, qc.body(qbFixture(t, typ), nil)), http.StatusCreated, "create "+typ)
		vid := c.publishQuestion(ctx, t, r.str("content_id"), 1)
		published[vid] = typ
		switch typ {
		case "single_choice":
			single = vid
		case "numeric":
			numeric = vid
		case "ordering":
			ordering = vid
		}
	}
	// Excluded from the pool: an exam-only question, a draft, a platform question not assigned to tenant A,
	// a question of tenant B, an efficacy instrument.
	exam := must(t, c.createQ(t, qlcmA, qc.body(qbFixture(t, "single_choice"), map[string]any{"exam_only": true})), http.StatusCreated, "exam only")
	examVID := c.publishQuestion(ctx, t, exam.str("content_id"), 1)
	must(t, c.createQ(t, qlcmA, qc.body(qbFixture(t, "single_choice"), nil)), http.StatusCreated, "draft")
	plat := must(t, c.createQ(t, cmNoidung, qc.body(qbFixture(t, "single_choice"), map[string]any{"owner_scope": "platform"})), http.StatusCreated, "platform")
	platVID := c.publishQuestion(ctx, t, plat.str("content_id"), 1)
	// An efficacy instrument (platform, exam_only) assigned to tenant A never reaches a pool, exams included.
	eff := must(t, c.createQ(t, cmNoidung, qc.body(qbFixture(t, "true_false"), map[string]any{"owner_scope": "platform", "efficacy_instrument": true,
		"exam_only": true})), http.StatusCreated, "efficacy")
	effVID := c.publishQuestion(ctx, t, eff.str("content_id"), 1)
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO infra.tenant_resource_assignments
		(assignment_id, tenant_id, created_at, updated_at, resource_type, resource_id, assigned_by, assigned_at)
		VALUES ($1, $2, now(), now(), 'question', $3, $1, now())`, uuid.New(), c.tenantA, eff.str("content_id")); err != nil {
		t.Fatal(err)
	}

	subject := uuid.MustParse(qc.subject)
	rule := func(n int, examOnly bool) selection.Request {
		return selection.Request{Rule: selection.Rule{Filters: selection.Filters{SubjectID: &subject}, Count: n, AllowExamOnly: examOnly}, Seed: 7}
	}
	got, err := d.Select(student, rule(4, false))
	if err != nil {
		t.Fatal(err)
	}
	ids := func(cs []selection.Candidate) []string {
		out := []string{}
		for _, x := range cs {
			out = append(out, x.VersionID.String())
		}
		slices.Sort(out)
		return out
	}
	want := []string{}
	for vid := range published {
		want = append(want, vid)
	}
	slices.Sort(want)
	if !slices.Equal(ids(got), want) {
		t.Fatalf("pool of A = %v, want %v", ids(got), want)
	}
	// AC-26: the same seed gives the same selection (from the cache now).
	again, err := d.Select(student, rule(4, false))
	if err != nil || !slices.Equal(ids(again), ids(got)) {
		t.Fatalf("reproducible selection %v %v", ids(again), err)
	}
	var pe *problem.Error
	if _, err := d.Select(student, rule(5, false)); !errors.As(err, &pe) || pe.Code != problem.QuestionPoolInsufficient {
		t.Fatalf("5 of 4: %v", err)
	}
	if withExam, err := d.Select(student, rule(5, true)); err != nil || !slices.Contains(ids(withExam), examVID) || slices.Contains(ids(withExam), effVID) {
		t.Fatalf("exam pool %v %v", ids(withExam), err)
	}
	if _, err := d.Select(studentB, rule(1, false)); !errors.As(err, &pe) || pe.Code != problem.QuestionPoolInsufficient {
		t.Fatalf("tenant B sees questions of A: %v", err)
	}
	for _, x := range []string{platVID, effVID} {
		if slices.Contains(ids(got), x) {
			t.Fatalf("%s in the pool", x)
		}
	}

	// AC-22, AC-25: render without answer data, reproducible by seed; versions of another tenant are not rendered.
	vids := []uuid.UUID{uuid.MustParse(single), uuid.MustParse(numeric), uuid.MustParse(ordering)}
	r1, p1, err := d.Render(student, vids, 42)
	if err != nil || len(r1) != 3 || len(p1) != 3 {
		t.Fatalf("render %d %v", len(r1), err)
	}
	b, _ := json.Marshal(r1)
	for _, leak := range []string{"correct_option_id", "feedback", "misconception", "answer_key", "Em đã thuộc", "tolerance", "\"value\":12.5"} {
		if strings.Contains(string(b), leak) {
			t.Fatalf("render leaks %q: %s", leak, b)
		}
	}
	r2, _, err := d.Render(student, vids, 42)
	b2, _ := json.Marshal(r2)
	if err != nil || string(b2) != string(b) {
		t.Fatal("render is not reproducible for the same seed")
	}
	if rb, _, err := d.Render(studentB, vids, 42); err == nil && len(rb) != 0 {
		t.Fatalf("tenant B renders questions of A: %d", len(rb))
	}

	// Grading on the server (AC-14…21 covered by the domain table; here through the stored key).
	graded := testutil.ToFloat64(observability.QBGradeTotal().WithLabelValues("single_choice", "correct"))
	g, err := d.Grade(student, uuid.MustParse(single), p1[0], json.RawMessage(`{"option_id":"o_3b2e"}`))
	if err != nil || !g.IsCorrect || g.Score != 1 {
		t.Fatalf("grade single %+v %v", g, err)
	}
	if testutil.ToFloat64(observability.QBGradeTotal().WithLabelValues("single_choice", "correct")) != graded+1 {
		t.Fatal("eduworld_qb_grade_total not counted")
	}
	g, err = d.Grade(student, uuid.MustParse(numeric), p1[1], json.RawMessage(`{"value":"12,5"}`))
	if err != nil || !g.IsCorrect {
		t.Fatalf("grade numeric %+v %v", g, err)
	}
	if _, err := d.Grade(student, uuid.MustParse(single), p1[0], json.RawMessage(`{"value":"1"}`)); !errors.As(err, &pe) || pe.Code != problem.InvalidResponse {
		t.Fatalf("invalid response: %v", err)
	}
	if ex, err := d.Explanation(student, uuid.MustParse(single)); err != nil {
		t.Fatalf("explanation %s %v", ex, err)
	}

	// AC-24: the readonly role cannot read an answer column, neither from the view nor from the table.
	if _, err := c.readonly.ExecContext(ctx, `SELECT answer_key FROM content.question_delivery_v LIMIT 1`); err == nil {
		t.Fatal("the delivery view exposes answer_key")
	}
	if _, err := c.readonly.ExecContext(ctx, `SELECT answer_key FROM content.question_versions LIMIT 1`); err == nil {
		t.Fatal("learning_readonly reads content.question_versions")
	}
	for _, col := range []string{"answer_key", "scoring_rule", "explanation", "option_feedback", "hints", "min_response_ms"} {
		if n := count(ctx, t, c.platform, `SELECT count(*) FROM information_schema.columns WHERE table_schema = 'content'
			AND table_name = 'question_delivery_v' AND column_name = $1`, col); n != 0 {
			t.Fatalf("question_delivery_v has %s", col)
		}
	}

	// Current release only: version 2 published and pointed at replaces version 1 in the pool after invalidation.
	singleQ := ""
	_ = c.platform.QueryRowContext(ctx, `SELECT question_id::text FROM content.question_versions WHERE question_version_id = $1`, single).Scan(&singleQ)
	must(t, c.do(t, http.MethodPost, "/cms/learning/questions/"+singleQ+"/versions", qlcmA, idem(), map[string]any{"from_version": 1}),
		http.StatusCreated, "v2")
	v2 := c.publishQuestion(ctx, t, singleQ, 2)

	// AC-28: the release consumer drops the pools of the tenant (tenant event) or of every tenant (platform event).
	keys := func(prefix string) []string {
		out, err := vk.Do(ctx, vk.B().Keys().Pattern(prefix+"*").Build()).AsStrSlice()
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	if len(keys(cache.PoolPrefix+c.tenantA.String())) == 0 {
		t.Fatal("the pool of A is not cached")
	}
	inv := consumer.QuestionPoolInvalidation{Cache: cache.Invalidator{C: vk}}
	if err := inv.Handle(ctx, outbox.Envelope{EventType: "content.published", TenantID: &c.tenantA,
		Payload: json.RawMessage(`{"item_type":"activity"}`)}); err != nil || len(keys(cache.PoolPrefix+c.tenantA.String())) == 0 {
		t.Fatalf("an activity event dropped the question pools: %v", err)
	}
	if err := inv.Handle(ctx, outbox.Envelope{EventType: "content.published", TenantID: &c.tenantA,
		Payload: json.RawMessage(`{"item_type":"question"}`)}); err != nil || len(keys(cache.PoolPrefix+c.tenantA.String())) != 0 {
		t.Fatalf("tenant invalidation: %v", err)
	}
	got, err = d.Select(student, rule(4, false))
	if err != nil || !slices.Contains(ids(got), v2) || slices.Contains(ids(got), single) {
		t.Fatalf("pool after release of v2: %v %v", ids(got), err)
	}
	if err := inv.Handle(ctx, outbox.Envelope{EventType: "content.published", Payload: json.RawMessage(`{"item_type":"question"}`)}); err != nil ||
		len(keys(cache.PoolPrefix)) != 0 {
		t.Fatalf("platform invalidation: %v", err)
	}
}
