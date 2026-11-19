//go:build integration

package integration

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/job"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

func (c *curEnv) runTranslations(ctx context.Context, t *testing.T) {
	t.Helper()
	if _, err := (job.TranslationWorker{Runner: c.tr}).RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
}

// F025-AC-22, AC-23, AC-24 (US5, quickstart 14): export of the missing translations to ew-exports with a presigned
// download; import creates a new draft per item (the published version is unchanged) and reports unknown keys; another
// tenant's item is reported as not found; a caller without learning_content.translate is denied.
func TestTranslationExportImport(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t, withMedia)
	qc := c.qbSetup(ctx, t)
	authztest.Cover(t, "exportLearningTranslations", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "importLearningTranslations", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getLearningTranslationJob", authztest.CrossTenant, authztest.RoleDenied)
	q1, _ := c.question(t, qlcmA, qc, map[string]any{"grade_levels": []int{4}})
	pub := c.publishQuestion(ctx, t, q1, 1)
	q2, _ := c.question(t, qlcmA, qc, map[string]any{"grade_levels": []int{4}})
	c.assign(ctx, t, qc.cur, c.tenantB)
	qB, _ := c.question(t, qlcmB, qc, map[string]any{"grade_levels": []int{4}})
	// AC-24: permission.
	body := map[string]any{"content_ids": []string{q1, q2, qB}, "target_language": "en", "format": "csv", "only_missing": true}
	if r := c.do(t, http.MethodPost, "/cms/learning/translations:export", edA, idem(), body); r.status != http.StatusForbidden {
		t.Fatalf("editor without translate: %d %s", r.status, r.raw)
	}
	// AC-22: export.
	r := must(t, c.do(t, http.MethodPost, "/cms/learning/translations:export", qlcmA, idem(), body), http.StatusAccepted, "export")
	jobID := r.str("job_id")
	if r.str("status") != "queued" || r.str("kind") != "export" {
		t.Fatalf("export job: %s", r.raw)
	}
	c.runTranslations(ctx, t)
	r = must(t, c.do(t, http.MethodGet, "/cms/learning/translations/jobs/"+jobID, qlcmA, nil, nil), http.StatusOK, "job")
	if r.str("status") != "succeeded" || r.str("download_url") == "" || r.str("report", "updated") != "2" || r.str("report", "skipped") != "1" {
		t.Fatalf("export result: %s", r.raw)
	}
	if !strings.Contains(r.raw, qB) {
		t.Fatalf("tenant B item not reported: %s", r.raw)
	}
	if g := c.do(t, http.MethodGet, "/cms/learning/translations/jobs/"+jobID, qlcmB, nil, nil); g.status != http.StatusNotFound {
		t.Fatalf("job of tenant A read by tenant B: %d", g.status)
	}
	res, err := http.Get(r.str("download_url")) //nolint:noctx // presigned test download
	if err != nil {
		t.Fatal(err)
	}
	file, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.Contains(string(file), "question/"+q1+"/1/stem/blocks/0/text") {
		t.Fatalf("export file %d: %s", res.StatusCode, file)
	}
	// AC-23: fill the en column of the stem of q1 and q2, add an unknown key, import (dry run first).
	var out []string
	for i, line := range strings.Split(strings.TrimSpace(string(file)), "\n") {
		cols := strings.Split(strings.TrimRight(line, "\r"), ",")
		if i > 0 && strings.HasSuffix(cols[0], "/stem/blocks/0/text") {
			cols[2] = "What is 7 x 8?"
		}
		out = append(out, strings.Join(cols, ","))
	}
	out = append(out, "question/"+qB+"/1/stem/blocks/0/text,x,Foreign", "nonsense,a,b")
	asset := c.assets.put([]byte(strings.Join(out, "\n") + "\n"))
	imp := map[string]any{"asset_id": asset.String(), "format": "csv", "target_language": "en", "dry_run": true}
	must(t, c.do(t, http.MethodPost, "/cms/learning/translations:import", qlcmA, idem(), imp), http.StatusAccepted, "dry run")
	c.runTranslations(ctx, t)
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.question_versions WHERE question_id IN ($1, $2)`, q1, q2); n != 2 {
		t.Fatalf("dry run wrote %d versions", n)
	}
	imp["dry_run"] = false
	r = must(t, c.do(t, http.MethodPost, "/cms/learning/translations:import", qlcmA, idem(), imp), http.StatusAccepted, "import")
	c.runTranslations(ctx, t)
	r = must(t, c.do(t, http.MethodGet, "/cms/learning/translations/jobs/"+r.str("job_id"), qlcmA, nil, nil), http.StatusOK, "import job")
	if r.str("status") != "succeeded" || r.str("report", "updated") != "1" || !strings.Contains(r.raw, "nonsense") || !strings.Contains(r.raw, qB) {
		t.Fatalf("import report: %s", r.raw)
	}
	// q1 (published) gets version 2 in draft with the translation; version 1 is unchanged; q2 still has its open draft.
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.question_versions WHERE question_id = $1 AND version_no = 2 AND status = 'draft'
		AND stem->'blocks'->0->'text'->>'en' = 'What is 7 x 8?' AND based_on_version_id = $2`, q1, pub); n != 1 {
		t.Fatal("no translated draft of the published question")
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.question_versions WHERE question_version_id = $1 AND status = 'published'
		AND jsonb_typeof(stem->'blocks'->0->'text') = 'string'`, pub); n != 1 {
		t.Fatal("the published version changed")
	}
	if !strings.Contains(r.raw, "open draft") {
		t.Fatalf("q2 with an open draft not reported: %s", r.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.question_versions WHERE question_id = $1`, qB); n != 1 {
		t.Fatal("tenant B question changed by tenant A import")
	}
}
