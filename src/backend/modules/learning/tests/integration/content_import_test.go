//go:build integration

package integration

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
)

// questionsCSV builds a questions_csv file (TEST FIXTURE) with n valid rows plus a row with a bad difficulty and a
// row with a formula without rendered SVG.
func questionsCSV(t *testing.T, n int) []byte {
	t.Helper()
	var b strings.Builder
	w := csv.NewWriter(&b)
	cols := []string{"type", "subject_code", "grade_levels", "curriculum_code", "curriculum_version", "topic_code", "objective_codes", "difficulty",
		"content_language", "stem", "render_payload", "answer_key", "scoring_rule", "tags"}
	_ = w.Write(cols)
	js := func(v any) string { x, _ := json.Marshal(v); return string(x) }
	render := js(map[string]any{"options": []any{map[string]any{"id": "o_a", "content": rich("54")}, map[string]any{"id": "o_b", "content": rich("56")}}})
	row := func(diff, stem string) []string {
		return []string{"single_choice", "math", "3", "QB_MATH", "1", "MATH.G3.T01", "MATH.G3.NUM.MUL", diff, "vi", stem, render,
			`{"correct_option_id":"o_b"}`, `{"mode":"all_or_nothing"}`, "nhap|lop3"}
	}
	for i := range n {
		_ = w.Write(row("2", js(rich("Nhân vật Mèo hỏi câu "+string(rune('A'+i))))))
	}
	_ = w.Write(row("x", js(rich("dòng lỗi"))))
	_ = w.Write(row("2", js(map[string]any{"blocks": []any{map[string]any{"type": "math", "latex": `\frac{3}{4}`}}})))
	w.Flush()
	return []byte(b.String())
}

// F024-AC-13, AC-14, AC-15, AC-16: dry run then real import with errors by row, MATH_RENDER_FAILED, refused kinds and
// files, audit, content owned by the tenant of the job.
func TestQuestionImport(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	c.qbSetup(ctx, t)
	file := questionsCSV(t, 5)
	before := count(ctx, t, c.platform, `SELECT count(*) FROM content.questions`)
	dry := c.importFile(ctx, t, qlcmA, "questions_csv", file, true)
	if dry.str("status") != "succeeded" || dry.str("created_count") != "5" || dry.str("rejected_count") != "2" ||
		count(ctx, t, c.platform, `SELECT count(*) FROM content.questions`) != before {
		t.Fatalf("dry run: %s", dry.raw)
	}
	job := c.importFile(ctx, t, qlcmA, "questions_csv", file, false)
	errs := job.items("errors")
	if job.str("created_count") != "5" || len(errs) != 2 || errs[0]["row"] != float64(7) || errs[0]["field"] != "difficulty" ||
		errs[1]["row"] != float64(8) || errs[1]["code"] != "MATH_RENDER_FAILED" {
		t.Fatalf("import: %s", job.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.question_versions WHERE tenant_id = $1 AND status = 'draft'`, c.tenantA); n != 5 {
		t.Fatalf("tenant A drafts %d", n)
	}
	if n := c.audits(ctx, t, command.ActionContentImported, &c.tenantA); n != 2 {
		t.Fatalf("audit content.imported %d", n)
	}
	// AC-15: an unknown kind, oversized and malformed files (lessons_docx is supported since F028: TestDocxImport*).
	r := must(t, c.do(t, http.MethodPost, "/cms/learning/imports", qlcmA, idem(), map[string]any{"asset_id": c.assets.put([]byte("x")).String(),
		"kind": "lessons_pdf"}), http.StatusUnprocessableEntity, "unknown kind")
	if !strings.Contains(r.raw, "VALIDATION_FAILED") {
		t.Fatalf("unknown kind %s", r.raw)
	}
	big := c.assets.put(file)
	c.assets.sizes[big] = 11 << 20
	r = must(t, c.do(t, http.MethodPost, "/cms/learning/imports", qlcmA, idem(), map[string]any{"asset_id": big.String(), "kind": "questions_csv"}),
		http.StatusAccepted, "large")
	c.runImports(ctx, t)
	r = must(t, c.do(t, http.MethodGet, "/cms/learning/imports/"+r.str("import_id"), qlcmA, nil, nil), http.StatusOK, "large job")
	if r.str("status") != "failed" || !strings.Contains(r.raw, "FILE_TOO_LARGE") {
		t.Fatalf("large %s", r.raw)
	}
	bad := c.importFile(ctx, t, qlcmA, "questions_xlsx", []byte("not an excel file"), false)
	if bad.str("status") != "failed" || !strings.Contains(bad.raw, "INVALID_FORMAT") {
		t.Fatalf("bad format %s", bad.raw)
	}
	// content_json with ids; no learning_content.import → 403.
	must(t, c.do(t, http.MethodPost, "/cms/learning/imports", noRoleA, idem(), map[string]any{"asset_id": big.String(), "kind": "content_json"}),
		http.StatusForbidden, "no import permission")
}
