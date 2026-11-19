//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// qbFixture reads a QUESTION_BANK §3 example (internal/domain/question/testdata/qb_examples).
func qbFixture(t *testing.T, typ string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), "src", "backend", "modules", "learning", "internal", "domain", "question", "testdata",
		"qb_examples", typ+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

var qbTypes = []string{"single_choice", "multiple_choice", "true_false", "fill_in_blank", "numeric", "matching", "ordering", "drag_drop"}

// qbCurriculum creates a platform curriculum (math grade 3, one topic, two LOs) assigned to tenant A.
type qbCurriculum struct {
	subject, version, topic, lo, lo2, cur string
}

func (c *curEnv) qbSetup(ctx context.Context, t *testing.T) qbCurriculum {
	t.Helper()
	q := qbCurriculum{subject: c.subject(t, "math")}
	q.cur, q.version = c.curriculum(t, cmNoidung, "QB_MATH")
	q.topic = c.topic(t, cmNoidung, q.version, q.subject, "MATH.G3.T01", 3, "")
	q.lo = must(t, c.objective(t, cmNoidung, q.topic, "MATH.G3.NUM.MUL", nil), http.StatusCreated, "LO").str("objective_id")
	q.lo2 = must(t, c.objective(t, cmNoidung, q.topic, "MATH.G3.NUM.DIV", nil), http.StatusCreated, "LO2").str("objective_id")
	c.assign(ctx, t, q.cur, c.tenantA)
	return q
}

// body builds a QuestionAuthoringInput from a fixture and the metadata of qc.
func (q qbCurriculum) body(fx map[string]any, extra map[string]any) map[string]any {
	b := map[string]any{"grade_levels": []int{3}, "subject_id": q.subject, "topic_id": q.topic, "curriculum_version_id": q.version,
		"objective_ids": []string{q.lo}, "difficulty": 2, "content_language": "vi"}
	for k, v := range fx {
		b[k] = v
	}
	for k, v := range extra {
		b[k] = v
	}
	return b
}

func (c *curEnv) createQ(t *testing.T, sub string, body map[string]any) resp {
	t.Helper()
	return c.do(t, http.MethodPost, "/cms/learning/questions", sub, idem(), body)
}

// F022-AC-01, AC-02, AC-03, AC-05, AC-06, AC-10, AC-12 (US1): the 8 types are created through the HTTP chain,
// structural and semantic errors carry JSON pointers, feedback leaves the render payload, the CMS reads the answer.
func TestQuestionAuthoring(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	for _, typ := range qbTypes {
		key := idem()
		body := qc.body(qbFixture(t, typ), nil)
		r := must(t, c.do(t, http.MethodPost, "/cms/learning/questions", qlcmA, key, body), http.StatusCreated, "create "+typ)
		if r.str("status") != "draft" || r.str("version") != "1" || r.header.Get("ETag") == "" || r.str("owner_scope") != "tenant" {
			t.Fatalf("%s: %s", typ, r.raw)
		}
		again := must(t, c.do(t, http.MethodPost, "/cms/learning/questions", qlcmA, key, body), http.StatusCreated, "replay "+typ)
		if again.header.Get("Idempotent-Replayed") != "true" || again.str("content_id") != r.str("content_id") {
			t.Fatalf("%s replay %s", typ, again.raw)
		}
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.questions WHERE tenant_id = $1`, c.tenantA); n != 8 {
		t.Fatalf("%d questions", n)
	}
	// AC-02: the types of F023 are enabled since F023 (question.Full); a registry without them still answers
	// TYPE_NOT_ENABLED (domain tests of question.Base).
	r := c.createQ(t, qlcmA, qc.body(map[string]any{"type": "short_answer", "stem": map[string]any{"blocks": []any{map[string]any{"type": "paragraph", "text": "x"}}},
		"render_payload": map[string]any{"max_length": 10}, "answer_key": map[string]any{"accepted": []string{"x"}}, "scoring_rule": map[string]any{"mode": "all_or_nothing"}}, nil))
	must(t, r, http.StatusCreated, "short_answer")
	// AC-03: semantic errors with JSON pointers.
	sc := qbFixture(t, "single_choice")
	sc["answer_key"] = map[string]any{"correct_option_id": "o_9999"}
	r = c.createQ(t, qlcmA, qc.body(sc, nil))
	if must(t, r, http.StatusUnprocessableEntity, "unknown key"); !slices.Contains(r.errorCodes(), "/answer_key/correct_option_id:INVALID") {
		t.Fatalf("single key %v", r.errorCodes())
	}
	ord := qbFixture(t, "ordering")
	ord["scoring_rule"] = map[string]any{"mode": "partial", "partial_method": "right_minus_wrong"}
	r = c.createQ(t, qlcmA, qc.body(ord, nil))
	if must(t, r, http.StatusUnprocessableEntity, "partial method"); !slices.Contains(r.errorCodes(), "/scoring_rule/partial_method:INVALID") {
		t.Fatalf("ordering partial %v", r.errorCodes())
	}
	mt := qbFixture(t, "matching")
	mt["answer_key"] = map[string]any{"pairs": [][]string{{"l_1", "r_9"}}}
	r = c.createQ(t, qlcmA, qc.body(mt, nil))
	if must(t, r, http.StatusUnprocessableEntity, "matching"); !slices.Contains(r.errorCodes(), "/answer_key/pairs/0/1:INVALID") {
		t.Fatalf("matching %v", r.errorCodes())
	}
	bad := qbFixture(t, "single_choice")
	bad["render_payload"].(map[string]any)["options"].([]any)[0].(map[string]any)["id"] = "A"
	r = c.createQ(t, qlcmA, qc.body(bad, nil))
	if must(t, r, http.StatusUnprocessableEntity, "id A"); !slices.Contains(r.errorCodes(), "/render_payload/options/0/id:INVALID") {
		t.Fatalf("id A %v", r.errorCodes())
	}
	// AC-12: an accepted answer of 300 characters.
	fb := qbFixture(t, "fill_in_blank")
	fb["answer_key"].(map[string]any)["blanks"].(map[string]any)["b_1"] = map[string]any{"accepted": []string{strings.Repeat("x", 300)}}
	r = c.createQ(t, qlcmA, qc.body(fb, nil))
	if must(t, r, http.StatusUnprocessableEntity, "accepted 300"); !slices.Contains(r.errorCodes(), "/answer_key/blanks/b_1/accepted/0:INVALID") {
		t.Fatalf("accepted 300 %v", r.errorCodes())
	}
	// AC-05: NFD stored as NFC; worked_example block refused; more than 50 media refused.
	nfd := qbFixture(t, "single_choice")
	nfd["stem"] = map[string]any{"blocks": []any{map[string]any{"type": "paragraph", "text": norm.NFD.String("Bảy nhân tám bằng mấy?")}}}
	r = must(t, c.createQ(t, qlcmA, qc.body(nfd, nil)), http.StatusCreated, "NFD stem")
	var stem string
	if err := c.platform.QueryRowContext(ctx, `SELECT stem->'blocks'->0->>'text' FROM content.question_versions WHERE question_version_id = $1`,
		r.str("content_version_id")).Scan(&stem); err != nil || stem != norm.NFC.String("Bảy nhân tám bằng mấy?") || !norm.NFC.IsNormalString(stem) {
		t.Fatalf("NFC stem %q %v", stem, err)
	}
	we := qbFixture(t, "single_choice")
	we["stem"] = map[string]any{"blocks": []any{map[string]any{"type": "worked_example"}}}
	must(t, c.createQ(t, qlcmA, qc.body(we, nil)), http.StatusUnprocessableEntity, "worked_example block")
	media := qbFixture(t, "single_choice")
	blocks := []any{}
	for i := 0; i < 51; i++ {
		blocks = append(blocks, map[string]any{"type": "image", "media_ref": uuid.NewString(), "alt": "hình"})
	}
	media["stem"] = map[string]any{"blocks": blocks}
	r = c.createQ(t, qlcmA, qc.body(media, nil))
	if must(t, r, http.StatusUnprocessableEntity, "51 media"); !slices.Contains(r.errorCodes(), "/media_refs:INVALID") {
		t.Fatalf("51 media %v", r.errorCodes())
	}
	// AC-06, AC-10: feedback stored apart from the render payload; the CMS sees answer key, feedback and hints.
	r = must(t, c.createQ(t, qlcmA, qc.body(qbFixture(t, "single_choice"), nil)), http.StatusCreated, "single")
	var render, feedback string
	if err := c.platform.QueryRowContext(ctx, `SELECT render_payload::text, option_feedback::text FROM content.question_versions
		WHERE question_version_id = $1`, r.str("content_version_id")).Scan(&render, &feedback); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(render, "feedback") || strings.Contains(render, "misconception") || !strings.Contains(feedback, "off_by_one_factor") {
		t.Fatalf("stored render %s / feedback %s", render, feedback)
	}
	g := must(t, c.do(t, http.MethodGet, "/cms/learning/questions/"+r.str("content_id"), qlcmA, nil, nil), http.StatusOK, "get")
	if !strings.Contains(g.raw, `"correct_option_id":"o_3b2e"`) || !strings.Contains(g.raw, "off_by_one_factor") || !strings.Contains(g.raw, "Em đã thuộc 7 × 7 chưa?") {
		t.Fatalf("CMS view %s", g.raw)
	}
	gv := must(t, c.do(t, http.MethodGet, "/cms/learning/questions/"+r.str("content_id")+"/versions/1", qlcmA, nil, nil), http.StatusOK, "get version")
	if gv.str("content_version_id") != r.str("content_version_id") || gv.header.Get("ETag") != `W/"1"` {
		t.Fatalf("version %v %s", gv.header, gv.raw)
	}
	// A topic of another curriculum version is refused.
	_, otherVer := c.curriculum(t, cmNoidung, "QB_OTHER")
	r = c.createQ(t, qlcmA, qc.body(qbFixture(t, "numeric"), map[string]any{"curriculum_version_id": otherVer}))
	if must(t, r, http.StatusUnprocessableEntity, "topic of another version"); !slices.Contains(r.errorCodes(), "/topic_id:INVALID") {
		t.Fatalf("topic %v", r.errorCodes())
	}
}

// publishQuestion marks version 1 published and points the release at it (TEST helper: the publish flow belongs
// to F026; SQL as learning_platform, allowed by the immutability trigger from draft).
func (c *curEnv) publishQuestion(ctx context.Context, t *testing.T, questionID string, version int) string {
	t.Helper()
	var vid string
	if err := c.platform.QueryRowContext(ctx, `UPDATE content.question_versions SET status = 'published', published_at = now(), reviewer_id = author_id
		WHERE question_id = $1 AND version_no = $2 RETURNING question_version_id`, questionID, version).Scan(&vid); err != nil {
		t.Fatal(err)
	}
	if _, err := c.platform.ExecContext(ctx, `UPDATE content.questions SET current_version_id = $2 WHERE question_id = $1`, questionID, vid); err != nil {
		t.Fatal(err)
	}
	return vid
}

// F022-AC-07, AC-08 (TC-CMS-002): drafts only change with If-Match; a new version copies a published one; one open
// draft per question; the triggers refuse a change of a published version and a release pointer to a draft.
func TestQuestionVersioning(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	r := must(t, c.createQ(t, qlcmA, qc.body(qbFixture(t, "numeric"), nil)), http.StatusCreated, "create")
	id := r.str("content_id")
	path := "/cms/learning/questions/" + id + "/versions/1"
	must(t, c.do(t, http.MethodPatch, path, qlcmA, nil, map[string]any{"difficulty": 3}), http.StatusPreconditionRequired, "no If-Match")
	must(t, c.do(t, http.MethodPatch, path, qlcmA, ifm(`W/"9"`), map[string]any{"difficulty": 3}), http.StatusPreconditionFailed, "stale")
	p := must(t, c.do(t, http.MethodPatch, path, qlcmA, ifm(r.header.Get("ETag")), map[string]any{"difficulty": 3, "hints": []any{
		map[string]any{"blocks": []any{map[string]any{"type": "paragraph", "text": "Chia đôi 25"}}}}}), http.StatusOK, "patch")
	if p.str("difficulty") != "3" || !strings.Contains(p.raw, "Chia đôi 25") || p.header.Get("ETag") != `W/"2"` {
		t.Fatalf("patched %v %s", p.header, p.raw)
	}
	bad := map[string]any{"answer_key": map[string]any{"value": "abc"}}
	must(t, c.do(t, http.MethodPatch, path, qlcmA, ifm(p.header.Get("ETag")), bad), http.StatusUnprocessableEntity, "patch revalidated")
	c.publishQuestion(ctx, t, id, 1)
	r = c.do(t, http.MethodPatch, path, qlcmA, ifm(p.header.Get("ETag")), map[string]any{"difficulty": 4})
	if must(t, r, http.StatusConflict, "patch published"); r.str("code") != "CONTENT_NOT_EDITABLE" {
		t.Fatalf("published %s", r.raw)
	}
	if _, err := c.platform.ExecContext(ctx, `UPDATE content.question_versions SET difficulty = 5 WHERE question_id = $1`, id); err == nil {
		t.Fatal("trigger let a published version change")
	}
	nv := must(t, c.do(t, http.MethodPost, "/cms/learning/questions/"+id+"/versions", qlcmA, idem(), map[string]any{"from_version": 1, "note": "sửa đơn vị"}),
		http.StatusCreated, "new version")
	if nv.str("version") != "2" || nv.str("status") != "draft" || nv.str("difficulty") != "3" || nv.str("change_note") != "sửa đơn vị" {
		t.Fatalf("new version %s", nv.raw)
	}
	var based string
	_ = c.platform.QueryRowContext(ctx, `SELECT based_on_version_id::text FROM content.question_versions WHERE question_version_id = $1`,
		nv.str("content_version_id")).Scan(&based)
	if based == "" {
		t.Fatal("based_on_version_id not set")
	}
	r = c.do(t, http.MethodPost, "/cms/learning/questions/"+id+"/versions", qlcmA, idem(), map[string]any{"from_version": 1})
	if must(t, r, http.StatusConflict, "second draft"); r.str("code") != "CONFLICT" {
		t.Fatalf("second draft %s", r.raw)
	}
	if _, err := c.platform.ExecContext(ctx, `UPDATE content.questions SET current_version_id = $2 WHERE question_id = $1`, id,
		nv.str("content_version_id")); err == nil || !strings.Contains(err.Error(), "RELEASE_TARGET_INVALID") {
		t.Fatalf("release pointer to a draft: %v", err)
	}
	g := must(t, c.do(t, http.MethodGet, "/cms/learning/questions/"+id, qlcmA, nil, nil), http.StatusOK, "get latest")
	if g.str("version") != "2" || g.str("current_version_id") == "" {
		t.Fatalf("latest %s", g.raw)
	}
}

// F022-AC-09, AC-11, AC-13 (TC-TEN-001, TC-TEN-015): permissions, tenant isolation, search filters, soft-delete into
// the trash with the versions kept.
func TestQuestionAuthzSearchDelete(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	own := must(t, c.createQ(t, qlcmA, qc.body(qbFixture(t, "numeric"), nil)), http.StatusCreated, "own numeric").str("content_id")
	must(t, c.createQ(t, qlcmA, qc.body(qbFixture(t, "single_choice"), map[string]any{"difficulty": 4, "objective_ids": []string{qc.lo2}})),
		http.StatusCreated, "own single")
	plat := must(t, c.createQ(t, cmNoidung, qc.body(qbFixture(t, "numeric"), map[string]any{"owner_scope": "platform"})), http.StatusCreated, "platform numeric")
	if plat.str("owner_scope") != "platform" {
		t.Fatalf("platform question %s", plat.raw)
	}
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO infra.tenant_resource_assignments
		(assignment_id, tenant_id, created_at, updated_at, resource_type, resource_id, assigned_by, assigned_at)
		VALUES ($1, $2, now(), now(), 'question', $3, $1, now())`, uuid.New(), c.tenantA, plat.str("content_id")); err != nil {
		t.Fatal(err)
	}
	// AC-09: tenant B 404, assigned platform question read-only 403, no permission 403.
	must(t, c.do(t, http.MethodGet, "/cms/learning/questions/"+own, qlcmB, nil, nil), http.StatusNotFound, "B reads A")
	must(t, c.do(t, http.MethodGet, "/cms/learning/questions/"+own, noRoleA, nil, nil), http.StatusForbidden, "no role reads")
	must(t, c.createQ(t, noRoleA, qc.body(qbFixture(t, "numeric"), nil)), http.StatusForbidden, "no role writes")
	must(t, c.createQ(t, qlcmA, qc.body(qbFixture(t, "numeric"), map[string]any{"owner_scope": "platform"})), http.StatusForbidden, "tenant creates platform")
	pv := must(t, c.do(t, http.MethodGet, "/cms/learning/questions/"+plat.str("content_id")+"/versions/1", qlcmA, nil, nil), http.StatusOK, "A reads assigned")
	if pv.str("assigned") != "true" {
		t.Fatalf("assigned flag %s", pv.raw)
	}
	must(t, c.do(t, http.MethodPatch, "/cms/learning/questions/"+plat.str("content_id")+"/versions/1", qlcmA, ifm(pv.header.Get("ETag")),
		map[string]any{"difficulty": 5}), http.StatusForbidden, "A patches assigned platform")
	must(t, c.do(t, http.MethodGet, "/cms/learning/questions/"+plat.str("content_id"), qlcmB, nil, nil), http.StatusNotFound, "B reads unassigned platform")
	// AC-13: filters.
	cases := []struct {
		q    string
		want int
	}{
		{"", 3}, {"?source=own", 2}, {"?source=assigned", 1}, {"?type=numeric", 2}, {"?difficulty=4", 1}, {"?difficulty=2&difficulty=4", 3},
		{"?objective_id=" + qc.lo2, 1}, {"?topic_id=" + qc.topic, 3}, {"?subject_id=" + qc.subject + "&grade_level=3", 3}, {"?grade_level=4", 0},
		{"?status=draft&source=own&type=numeric", 1},
	}
	for _, x := range cases {
		r := must(t, c.do(t, http.MethodGet, "/cms/learning/questions"+x.q, qlcmA, nil, nil), http.StatusOK, "search "+x.q)
		if len(r.items("data")) != x.want {
			t.Errorf("search %q: %d, want %d", x.q, len(r.items("data")), x.want)
		}
	}
	if r := must(t, c.do(t, http.MethodGet, "/cms/learning/questions", qlcmB, nil, nil), http.StatusOK, "B search"); len(r.items("data")) != 0 {
		t.Fatalf("B sees %s", r.raw)
	}
	if r := must(t, c.do(t, http.MethodGet, "/cms/learning/questions?source=platform", cmNoidung, nil, nil), http.StatusOK, "platform"); len(r.items("data")) != 1 {
		t.Fatalf("platform staff sees %s", r.raw)
	}
	// AC-11: soft-delete a draft question (trash, audit, event); a released question cannot be deleted.
	tag := must(t, c.do(t, http.MethodGet, "/cms/learning/questions/"+own, qlcmA, nil, nil), http.StatusOK, "get").header.Get("ETag")
	must(t, c.do(t, http.MethodDelete, "/cms/learning/questions/"+own, qlcmA, nil, nil), http.StatusPreconditionRequired, "delete without If-Match")
	must(t, c.do(t, http.MethodDelete, "/cms/learning/questions/"+own, qlcmA, ifm(tag), map[string]any{"reason": "trùng"}), http.StatusNoContent, "delete")
	must(t, c.do(t, http.MethodGet, "/cms/learning/questions/"+own, qlcmA, nil, nil), http.StatusNotFound, "deleted")
	if !slices.Contains(listQuestionTrash(ctx, t, c), own) || c.audits(ctx, t, "content.deleted", &c.tenantA) != 1 {
		t.Fatal("trash/audit of the deleted question")
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.question_versions WHERE question_id = $1 AND deleted_at IS NULL`, own); n != 1 {
		t.Fatal("the versions of a deleted question must stay readable (attempt review)")
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'question.deleted'`); n != 1 {
		t.Fatalf("%d question.deleted", n)
	}
	// System Admin restores it from the trash (F020 use case with the question ResourceHandler).
	sctx, _ := systemAdmin(ctx)
	if _, err := (command.RestoreResource{TrashDeps: c.trashDeps(c.cmd.Resources)}).Handle(sctx,
		command.RestoreInput{ResourceType: "question", ID: uuid.MustParse(own), Reason: "khôi phục"}); err != nil {
		t.Fatal(err)
	}
	must(t, c.do(t, http.MethodGet, "/cms/learning/questions/"+own, qlcmA, nil, nil), http.StatusOK, "restored")
	released := must(t, c.createQ(t, qlcmA, qc.body(qbFixture(t, "ordering"), nil)), http.StatusCreated, "released").str("content_id")
	c.publishQuestion(ctx, t, released, 1)
	tag = must(t, c.do(t, http.MethodGet, "/cms/learning/questions/"+released, qlcmA, nil, nil), http.StatusOK, "get released").header.Get("ETag")
	must(t, c.do(t, http.MethodDelete, "/cms/learning/questions/"+released, qlcmA, ifm(tag), nil), http.StatusConflict, "delete released")
}

// listQuestionTrash lists the ids in the trash of resource type question through the F020 use case (System Admin).
func listQuestionTrash(ctx context.Context, t *testing.T, c *curEnv) []string {
	t.Helper()
	codec, _ := page.NewCodec([]byte(strings.Repeat("k", 32)))
	q := query.ListTrash{Session: c.read, Resources: c.cmd.Resources, Cursors: codec}
	p, err := q.Handle(authztest.Ctx(ctx, authztest.Platform("system_admin")), query.ListTrashInput{ResourceType: "question", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(p.Data))
	for i, e := range p.Data {
		out[i] = e.ResourceID.String()
	}
	return out
}
