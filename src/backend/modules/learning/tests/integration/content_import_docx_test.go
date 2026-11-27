//go:build integration

package integration

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	fx "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/docximport/docxfixture"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// pandocCommand is the converter of the Word import tests: EW_PANDOC_CMD (e.g. "docker run --rm -i --network none
// eduworld/media-tools:local pandoc", quickstart §3) or pandoc on PATH; nil when neither exists.
func pandocCommand() []string {
	if c := strings.Fields(os.Getenv("EW_PANDOC_CMD")); len(c) > 0 {
		return c
	}
	if _, err := exec.LookPath("pandoc"); err == nil {
		return []string{"pandoc"}
	}
	return nil
}

func (c *curEnv) needPandoc(t *testing.T) {
	t.Helper()
	if c.docx.Converter == nil {
		t.Skip("no Pandoc: set EW_PANDOC_CMD (quickstart §3)")
	}
}

// Codes of qbSetup (curriculum QB_MATH, grade 3).
func docxHeader(d *fx.Doc) {
	fx.Header(d, "math", "3", "QB_MATH", "MATH.G3.T01")
	d.P(fx.T("Phiên bản chương trình: 1")) // the draft version of qbSetup (no release yet)
}

// docxQuestion writes one valid single choice question with a fraction (TEST FIXTURE, placeholder content).
func docxQuestion(d *fx.Doc, n int, lo string) {
	d.P(fx.B("Câu "+strconv.Itoa(n)+"."), fx.T(" Tính "), fx.M(fx.Frac(fx.MR("3"), fx.MR("4"))), fx.T(" của 8 (câu mẫu)."))
	d.P(fx.T("A. 4"))
	d.P(fx.B("B. 6"))
	d.P(fx.T("C. 2"))
	d.P(fx.T("Gợi ý 1: Chia 8 thành 4 phần."))
	d.P(fx.T("Giải thích: Lời giải mẫu."))
	d.P(fx.T("LO: " + lo))
	d.P(fx.T("Độ khó: 2"))
}

// docxWithErrors is n valid questions then a question without answer, a MathType OLE question and one with an
// unknown LO (TEST FIXTURE).
func docxWithErrors(n int) []byte {
	d := fx.New()
	docxHeader(d)
	for i := 1; i <= n; i++ {
		docxQuestion(d, i, "MATH.G3.NUM.MUL")
	}
	d.P(fx.T("Câu " + strconv.Itoa(n+1) + ". Câu không có đáp án."))
	d.P(fx.T("A. 1"), fx.Br())
	d.P(fx.T("B. 2"))
	d.P(fx.T("LO: MATH.G3.NUM.MUL"))
	d.P(fx.T("Độ khó: 1"))
	d.P(fx.T("Câu "+strconv.Itoa(n+2)+". Công thức MathType: "), d.OLEEquation("ole1"))
	d.P(fx.B("A. 1"))
	d.P(fx.T("B. 2"))
	d.P(fx.T("LO: MATH.G3.NUM.MUL"))
	d.P(fx.T("Độ khó: 1"))
	docxQuestion(d, n+3, "MATH.G3.UNKNOWN")
	return d.Bytes()
}

// allItems pages listContentImportItems with limit until has_more is false.
func (c *curEnv) allItems(t *testing.T, sub, importID, status string, limit int) []map[string]any {
	t.Helper()
	var out []map[string]any
	cursor := ""
	for range 50 {
		q := url.Values{"limit": {strconv.Itoa(limit)}}
		if status != "" {
			q.Set("status", status)
		}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		r := must(t, c.do(t, http.MethodGet, "/cms/learning/imports/"+importID+"/items?"+q.Encode(), sub, nil, nil), http.StatusOK, "items")
		page := r.items("data")
		if len(page) > limit {
			t.Fatalf("page of %d > limit %d", len(page), limit)
		}
		out = append(out, page...)
		pg, _ := r.body["page"].(map[string]any)
		if pg["has_more"] != true {
			return out
		}
		cursor, _ = pg["next_cursor"].(string)
	}
	t.Fatal("too many pages")
	return nil
}

func issueCodes(it map[string]any) []string {
	var out []string
	is, _ := it["issues"].([]any)
	for _, x := range is {
		if m, ok := x.(map[string]any); ok {
			out = append(out, m["code"].(string))
		}
	}
	return out
}

// F028-AC-01, AC-02, AC-04, AC-05, AC-07, AC-08: a Word file is analysed into one report item per question, nothing
// is created even with dry_run = false, the job report counts and lists the errors by question, the items page by
// item_no and filter by status.
func TestDocxImportAnalyse(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	c.needPandoc(t)
	c.qbSetup(ctx, t)
	before := count(ctx, t, c.platform, `SELECT count(*) FROM content.questions`)
	job := c.importFile(ctx, t, qlcmA, "lessons_docx", docxWithErrors(45), false)
	if job.str("status") != "succeeded" || job.str("created_count") != "45" || job.str("rejected_count") != "3" ||
		job.str("result", "items_total") != "48" || job.str("result", "items_importable") != "45" {
		t.Fatalf("job %s", job.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.questions`); n != before {
		t.Fatalf("questions created by the analysis: %d → %d", before, n)
	}
	errs := job.items("errors")
	codes := map[float64]string{}
	for _, e := range errs {
		codes[e["row"].(float64)] += e["code"].(string) + " "
	}
	if !strings.Contains(codes[46], "ANSWER_MISSING") || !strings.Contains(codes[47], "FORMULA_NEEDS_REWRITE") || !strings.Contains(codes[48], "UNKNOWN_CODE") {
		t.Fatalf("errors %v", codes)
	}
	if n := c.audits(ctx, t, command.ActionContentImported, &c.tenantA); n != 1 {
		t.Fatalf("audit content.imported %d", n)
	}
	id := job.str("import_id")
	items := c.allItems(t, qlcmA, id, "", 20)
	if len(items) != 48 {
		t.Fatalf("items %d", len(items))
	}
	for i, it := range items {
		if it["item_no"] != float64(i+1) {
			t.Fatalf("order: item %d is %v", i, it["item_no"])
		}
	}
	first := items[0]
	fs, _ := first["formulas"].([]any)
	if first["status"] != "ready" || first["question_type"] != "single_choice" || first["source_label"] != "Câu 1" || len(fs) != 1 ||
		fs[0].(map[string]any)["latex"] != `\frac{3}{4}` || !strings.Contains(first["stem_preview"].(string), "của 8") {
		t.Fatalf("item 1 %v", first)
	}
	q, _ := first["question"].(map[string]any)
	if q["topic_code"] != "MATH.G3.T01" || q["answer_key"] == nil {
		t.Fatalf("question %v", q)
	}
	bad := c.allItems(t, qlcmA, id, "error", 2)
	if len(bad) != 3 || issueCodes(bad[1])[0] != "FORMULA_NEEDS_REWRITE" {
		t.Fatalf("error items %v", bad)
	}
}

// F028-AC-06: file-level refusals — macro, not a .docx, no question, too large; nothing stored.
func TestDocxImportFileErrors(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	c.needPandoc(t)
	c.qbSetup(ctx, t)
	macro := fx.New()
	docxHeader(macro)
	docxQuestion(macro, 1, "MATH.G3.NUM.MUL")
	macro.Macro = true
	empty := fx.New()
	docxHeader(empty)
	empty.P(fx.T("Không có câu hỏi nào."))
	for name, tc := range map[string]struct {
		data []byte
		code string
	}{"macro": {macro.Bytes(), "MACRO_NOT_ALLOWED"}, "not docx": {[]byte("PK not a zip"), "INVALID_FORMAT"}, "empty": {empty.Bytes(), "NO_QUESTIONS"}} {
		job := c.importFile(ctx, t, qlcmA, "lessons_docx", tc.data, false)
		if job.str("status") != "failed" || !strings.Contains(job.raw, tc.code) || len(c.allItems(t, qlcmA, job.str("import_id"), "", 100)) != 0 {
			t.Fatalf("%s: %s", name, job.raw)
		}
	}
	big := c.assets.put(docxWithErrors(1))
	c.assets.sizes[big] = 11 << 20
	r := must(t, c.do(t, http.MethodPost, "/cms/learning/imports", qlcmA, idem(), map[string]any{"asset_id": big.String(), "kind": "lessons_docx"}),
		http.StatusAccepted, "large")
	c.runImports(ctx, t)
	r = must(t, c.do(t, http.MethodGet, "/cms/learning/imports/"+r.str("import_id"), qlcmA, nil, nil), http.StatusOK, "large job")
	if r.str("status") != "failed" || !strings.Contains(r.raw, "FILE_TOO_LARGE") {
		t.Fatalf("large %s", r.raw)
	}
}

// svgFor renders every formula of an item with the TEST FIXTURE SVG (the portal uses MathJax 4).
func svgFor(it map[string]any) []any {
	var out []any
	fs, _ := it["formulas"].([]any)
	for _, f := range fs {
		out = append(out, map[string]any{"latex": f.(map[string]any)["latex"], "svg": mathSVG, "speech_text": "công thức"})
	}
	return out
}

func draftPath(id string, no int) string {
	return "/cms/learning/imports/" + id + "/items/" + strconv.Itoa(no) + ":create-draft"
}

// F028-AC-09…13: drafts from the analysed questions — version 1 draft with the stored SVG, idempotent replay,
// 409 for created/error items, 422 for missing/extra formulas, draft_failed then retry, pictures into the media
// pipeline.
func TestDocxImportCreateDraft(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t, withMedia)
	c.needPandoc(t)
	c.qbSetup(ctx, t)
	d := fx.New()
	docxHeader(d)
	docxQuestion(d, 1, "MATH.G3.NUM.MUL")
	docxQuestion(d, 2, "MATH.G3.NUM.DIV")
	d.P(fx.T("Câu 3. Quan sát hình "), d.Image("image1.png", "Hình tam giác mẫu", fx.PNG1x1), fx.T(" rồi chọn."))
	d.P(fx.B("A. Tam giác"))
	d.P(fx.T("B. Hình vuông"))
	d.P(fx.T("LO: MATH.G3.NUM.MUL"))
	d.P(fx.T("Độ khó: 1"))
	d.P(fx.T("Câu 4. Không có phương án."))
	d.P(fx.T("LO: MATH.G3.NUM.MUL"))
	d.P(fx.T("Độ khó: 1"))
	job := c.importFile(ctx, t, qlcmA, "lessons_docx", d.Bytes(), true)
	id := job.str("import_id")
	if job.str("status") != "succeeded" || job.str("created_count") != "3" {
		t.Fatalf("job %s", job.raw)
	}
	items := c.allItems(t, qlcmA, id, "", 100)
	drafts := func() int {
		return count(ctx, t, c.platform, `SELECT count(*) FROM content.question_versions WHERE tenant_id = $1 AND status = 'draft'`, c.tenantA)
	}
	before := drafts()

	// AC-10: a missing or a foreign formula is 422 MATH_RENDER_FAILED; nothing is created.
	must(t, c.do(t, http.MethodPost, draftPath(id, 1), qlcmA, idem(), map[string]any{"math_assets": []any{}}), http.StatusUnprocessableEntity, "no svg")
	foreign := append(svgFor(items[0]), map[string]any{"latex": `x^{2}`, "svg": mathSVG, "speech_text": "x"})
	r := must(t, c.do(t, http.MethodPost, draftPath(id, 1), qlcmA, idem(), map[string]any{"math_assets": foreign}), http.StatusUnprocessableEntity, "foreign")
	if !strings.Contains(r.raw, "MATH_RENDER_FAILED") || drafts() != before {
		t.Fatalf("foreign %s", r.raw)
	}

	// AC-09: created, idempotent replay, audit with the import.
	key := idem()
	body := map[string]any{"math_assets": svgFor(items[0])}
	r = must(t, c.do(t, http.MethodPost, draftPath(id, 1), qlcmA, key, body), http.StatusCreated, "draft 1")
	qid := r.str("question_id")
	if r.str("status") != "created" || qid == "" || drafts() != before+1 {
		t.Fatalf("draft %s", r.raw)
	}
	again := must(t, c.do(t, http.MethodPost, draftPath(id, 1), qlcmA, key, body), http.StatusCreated, "replay")
	if again.header.Get("Idempotent-Replayed") != "true" || again.str("question_id") != qid || drafts() != before+1 {
		t.Fatalf("replay %s", again.raw)
	}
	v := must(t, c.do(t, http.MethodGet, "/cms/learning/questions/"+qid+"/versions/1", qlcmA, nil, nil), http.StatusOK, "version")
	if v.str("status") != "draft" || !strings.Contains(v.raw, `math/`) || !strings.Contains(v.raw, "Chia 8 thành 4 phần") {
		t.Fatalf("version %s", v.raw)
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'content.version_created' AND envelope->'payload'->'metadata'->>'import_id' = $1`, id); n != 1 {
		t.Fatalf("audit with import_id %d", n)
	}

	// AC-11: created again with another key → 409; an error item → 409.
	r = must(t, c.do(t, http.MethodPost, draftPath(id, 1), qlcmA, idem(), body), http.StatusConflict, "created twice")
	if !strings.Contains(r.raw, command.SubImportItemCreated) {
		t.Fatalf("created twice %s", r.raw)
	}
	r = must(t, c.do(t, http.MethodPost, draftPath(id, 4), qlcmA, idem(), map[string]any{"math_assets": []any{}}), http.StatusConflict, "error item")
	if !strings.Contains(r.raw, command.SubImportItemNotImportable) {
		t.Fatalf("error item %s", r.raw)
	}
	must(t, c.do(t, http.MethodPost, draftPath(id, 99), qlcmA, idem(), map[string]any{"math_assets": []any{}}), http.StatusNotFound, "no item")

	// AC-12: the validators refuse (the LO was removed since the analysis) → 422, item draft_failed; restored → created.
	// tenancyguard:ok TEST helper under learning_platform
	if _, err := c.platform.ExecContext(ctx, `UPDATE curriculum.learning_objectives SET deleted_at = now() WHERE code = 'MATH.G3.NUM.DIV'`); err != nil {
		t.Fatal(err)
	}
	r = must(t, c.do(t, http.MethodPost, draftPath(id, 2), qlcmA, idem(), map[string]any{"math_assets": svgFor(items[1])}),
		http.StatusUnprocessableEntity, "refused")
	failed := c.allItems(t, qlcmA, id, "draft_failed", 10)
	if len(failed) != 1 || failed[0]["item_no"] != float64(2) || drafts() != before+1 {
		t.Fatalf("draft_failed %v %s", failed, r.raw)
	}
	// tenancyguard:ok TEST helper under learning_platform
	if _, err := c.platform.ExecContext(ctx, `UPDATE curriculum.learning_objectives SET deleted_at = NULL WHERE code = 'MATH.G3.NUM.DIV'`); err != nil {
		t.Fatal(err)
	}
	r = must(t, c.do(t, http.MethodPost, draftPath(id, 2), qlcmA, idem(), map[string]any{"math_assets": svgFor(items[1])}), http.StatusCreated, "retry")
	if r.str("status") != "created" || strings.Contains(r.raw, `"source":"validator"`) {
		t.Fatalf("retry %s", r.raw)
	}

	// AC-13: the picture goes into the media pipeline (processing + media.uploaded) and the draft references it.
	assetsBefore := count(ctx, t, c.platform, `SELECT count(*) FROM content.media_assets WHERE tenant_id = $1`, c.tenantA)
	r = must(t, c.do(t, http.MethodPost, draftPath(id, 3), qlcmA, idem(), map[string]any{"math_assets": []any{}}), http.StatusCreated, "picture")
	var asset, status string
	// tenancyguard:ok TEST helper under learning_platform
	if err := c.platform.QueryRowContext(ctx, `SELECT asset_id::text, status FROM content.media_assets WHERE tenant_id = $1 AND alt_text = 'Hình tam giác mẫu'`,
		c.tenantA).Scan(&asset, &status); err != nil {
		t.Fatal(err)
	}
	v = must(t, c.do(t, http.MethodGet, "/cms/learning/questions/"+r.str("question_id")+"/versions/1", qlcmA, nil, nil), http.StatusOK, "picture version")
	if status != "processing" || !strings.Contains(v.raw, asset) ||
		count(ctx, t, c.platform, `SELECT count(*) FROM content.media_assets WHERE tenant_id = $1`, c.tenantA) != assetsBefore+1 {
		t.Fatalf("picture asset %s %s %s", asset, status, v.raw)
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'media.uploaded' AND envelope->'payload'->>'asset_id' = $1`, asset); n != 1 {
		t.Fatalf("media.uploaded %d", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.question_versions WHERE tenant_id = $1 AND status = 'published'`, c.tenantA); n != 0 {
		t.Fatalf("published %d", n)
	}
}

// F028-AC-14 (TC-TEN-001): learning_content.import on the three operations; a job of tenant A is 404 for tenant B.
func TestDocxImportAccess(t *testing.T) {
	authztest.Cover(t, "listContentImportItems", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createDraftFromImportItem", authztest.CrossTenant, authztest.RoleDenied)
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	c.needPandoc(t)
	c.qbSetup(ctx, t)
	d := fx.New()
	docxHeader(d)
	docxQuestion(d, 1, "MATH.G3.NUM.MUL")
	job := c.importFile(ctx, t, qlcmA, "lessons_docx", d.Bytes(), false)
	id := job.str("import_id")
	items := c.allItems(t, qlcmA, id, "", 10)
	body := map[string]any{"math_assets": svgFor(items[0])}
	must(t, c.do(t, http.MethodPost, "/cms/learning/imports", noRoleA, idem(), map[string]any{"asset_id": c.assets.put(d.Bytes()).String(),
		"kind": "lessons_docx"}), http.StatusForbidden, "create without permission")
	must(t, c.do(t, http.MethodGet, "/cms/learning/imports/"+id+"/items", noRoleA, nil, nil), http.StatusForbidden, "list without permission")
	must(t, c.do(t, http.MethodPost, draftPath(id, 1), noRoleA, idem(), body), http.StatusForbidden, "draft without permission")
	must(t, c.do(t, http.MethodGet, "/cms/learning/imports/"+id, qlcmB, nil, nil), http.StatusNotFound, "job of A for B")
	must(t, c.do(t, http.MethodGet, "/cms/learning/imports/"+id+"/items", qlcmB, nil, nil), http.StatusNotFound, "items of A for B")
	must(t, c.do(t, http.MethodPost, draftPath(id, 1), qlcmB, idem(), body), http.StatusNotFound, "draft of A for B")
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.question_versions WHERE tenant_id = $1`, c.tenantB); n != 0 {
		t.Fatalf("tenant B drafts %d", n)
	}
	// the drafts belong to the owner of the job
	r := must(t, c.do(t, http.MethodPost, draftPath(id, 1), qlcmA, idem(), body), http.StatusCreated, "draft A")
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.questions WHERE question_id = $1 AND tenant_id = $2`, r.str("question_id"),
		c.tenantA); n != 1 {
		t.Fatalf("owner %d", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.import_items WHERE import_id = $1 AND tenant_id = $2`, id, c.tenantA); n != 1 {
		t.Fatalf("item tenant %d", n)
	}
}
