//go:build integration

package integration

import (
	"context"
	"net/http"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/job"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

func seedDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(testkit.RepoRoot(t), "src", "backend", "modules", "learning", "seed", "curriculum")
}

// importFile queues an import of b as sub and runs the queue; it returns the final job.
func (c *curEnv) importFile(ctx context.Context, t *testing.T, sub, kind string, b []byte, dryRun bool) resp {
	t.Helper()
	asset := c.assets.put(b)
	key := idem()
	body := map[string]any{"asset_id": asset.String(), "kind": kind, "dry_run": dryRun}
	r := must(t, c.do(t, http.MethodPost, "/cms/learning/imports", sub, key, body), http.StatusAccepted, "create import")
	if r.str("status") != "queued" {
		t.Fatalf("job %s", r.raw)
	}
	again := must(t, c.do(t, http.MethodPost, "/cms/learning/imports", sub, key, body), http.StatusAccepted, "replay import")
	if again.str("import_id") != r.str("import_id") || again.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("import replay %s", again.raw)
	}
	if _, err := (job.ImportWorker{Runner: c.cmd}).RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	return must(t, c.do(t, http.MethodGet, "/cms/learning/imports/"+r.str("import_id"), sub, nil, nil), http.StatusOK, "get import")
}

// structure is the comparable tree of a version: topic codes, LO codes, prerequisites (codes) and yccd_ref.
func (c *curEnv) structure(t *testing.T, sub, version string) []string {
	t.Helper()
	topics := must(t, c.do(t, http.MethodGet, "/cms/learning/topics?limit=100&curriculum_version_id="+version, sub, nil, nil), http.StatusOK, "topics")
	los := must(t, c.do(t, http.MethodGet, "/cms/learning/objectives?limit=100&curriculum_version_id="+version, sub, nil, nil), http.StatusOK, "LOs")
	codes := map[string]string{}
	for _, lo := range los.items("data") {
		codes[lo["objective_id"].(string)] = lo["code"].(string)
	}
	var out []string
	for _, tp := range topics.items("data") {
		out = append(out, "topic "+tp["code"].(string))
	}
	for _, lo := range los.items("data") {
		var pre []string
		for _, p := range lo["prerequisite_ids"].([]any) {
			pre = append(pre, codes[p.(string)])
		}
		sort.Strings(pre)
		y, _ := lo["yccd_ref"].(string)
		out = append(out, "lo "+lo["code"].(string)+" pre="+strings.Join(pre, ",")+" yccd="+y)
	}
	sort.Strings(out)
	return out
}

// F021-AC-22, AC-23, AC-24, AC-25 (US5): import job (dry run, row errors, idempotency, size and format), seed twice,
// export → import round trip (SC-001, SC-004).
func TestCurriculumImportSeedExport(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)

	// AC-23 / SC-001: the seed runs through the import use case; a second run creates nothing.
	first, err := job.Seed(ctx, c.cmd, seedDir(t))
	if err != nil {
		t.Fatal(err)
	}
	res := first.Curricula["gdpt2018_math_g1_5.yaml"]
	if first.SubjectsCreated != 3 || res.Created == 0 || res.Skipped {
		t.Fatalf("first seed %+v", first)
	}
	second, err := job.Seed(ctx, c.cmd, seedDir(t))
	if err != nil {
		t.Fatal(err)
	}
	if second.SubjectsCreated != 0 || !second.Curricula["gdpt2018_math_g1_5.yaml"].Skipped {
		t.Fatalf("second seed %+v", second)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM curriculum.learning_objectives lo JOIN curriculum.curricula c
		ON c.curriculum_id = lo.curriculum_id WHERE c.code = 'GDPT2018_SAMPLE' AND lo.deleted_at IS NULL`); n != 16 {
		t.Fatalf("%d seeded LOs, want 16", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM curriculum.curricula WHERE code = 'GDPT2018_SAMPLE' AND current_version_id IS NOT NULL`); n != 1 {
		t.Fatal("the seed is not published once")
	}
	seedVersion := res.VersionID.String()
	seedCur := res.CurriculumID.String()
	seeded := c.structure(t, cmNoidung, seedVersion)

	// AC-24: export YAML, import it as tenant A (dry run first: nothing written), same structure.
	exp := must(t, c.do(t, http.MethodGet, "/cms/learning/curricula/"+seedCur+"/versions/1:export?format=yaml", cmNoidung, nil, nil), http.StatusOK, "export")
	if !strings.HasPrefix(exp.header.Get("Content-Type"), "application/yaml") || !strings.Contains(exp.raw, "MATH.G3.NUM.MUL_TABLE.2") {
		t.Fatalf("export %v %s", exp.header, exp.raw)
	}
	dry := c.importFile(ctx, t, qlcmA, "curriculum_yaml", []byte(exp.raw), true)
	if dry.str("status") != "succeeded" || dry.str("created_count") == "0" || dry.str("result") != "" {
		t.Fatalf("dry run %s", dry.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM curriculum.curricula WHERE tenant_id = $1`, c.tenantA); n != 0 {
		t.Fatal("dry run wrote data")
	}
	done := c.importFile(ctx, t, qlcmA, "curriculum_yaml", []byte(exp.raw), false)
	if done.str("status") != "succeeded" || done.str("result", "curriculum_version_id") == "" {
		t.Fatalf("import %s", done.raw)
	}
	imported := c.structure(t, qlcmA, done.str("result", "curriculum_version_id"))
	if !slices.Equal(seeded, imported) {
		t.Fatalf("round trip differs:\n%v\n%v", seeded, imported)
	}
	v := must(t, c.do(t, http.MethodGet, "/cms/learning/curricula/"+done.str("result", "curriculum_id")+"/versions/1", qlcmA, nil, nil), http.StatusOK, "imported")
	if v.str("status") != "draft" {
		t.Fatalf("imported version %s", v.raw)
	}
	// CSV export → import as tenant B gives the same topics, LOs and prerequisites.
	csv := must(t, c.do(t, http.MethodGet, "/cms/learning/curricula/"+seedCur+"/versions/1:export?format=csv", cmNoidung, nil, nil), http.StatusOK, "csv")
	done = c.importFile(ctx, t, qlcmB, "curriculum_csv", []byte(csv.raw), false)
	if done.str("status") != "succeeded" {
		t.Fatalf("csv import %s", done.raw)
	}
	if got := c.structure(t, qlcmB, done.str("result", "curriculum_version_id")); !slices.Equal(seeded, got) {
		t.Fatalf("csv round trip differs:\n%v\n%v", seeded, got)
	}

	// AC-22: rejected rows are reported with row, field and code; nothing is written.
	bad := strings.Replace(exp.raw, "code: MATH.G3.NUM.DIV_REMAINDER", "code: math-g3-bad", 1)
	bad = strings.Replace(bad, "format_version: 1", "format_version: 1\nextra_field: x", 1)
	bad = strings.Replace(bad, "GDPT2018_SAMPLE", "BAD_IMPORT", 1)
	failed := c.importFile(ctx, t, qlcmA, "curriculum_yaml", []byte(bad), false)
	if failed.str("status") != "failed" || failed.str("rejected_count") == "0" {
		t.Fatalf("bad import %s", failed.raw)
	}
	errs := failed.items("errors")
	if len(errs) == 0 || errs[0]["row"] == nil || errs[0]["code"] == nil {
		t.Fatalf("row errors %s", failed.raw)
	}
	if !strings.Contains(failed.raw, "extra_field") {
		t.Fatalf("unknown field not reported %s", failed.raw)
	}
	// Business rules are reported once the file is well formed (same validators as the CMS).
	bad = strings.Replace(exp.raw, "code: MATH.G3.NUM.DIV_REMAINDER", "code: MATH.G4.NUM.DIV_REMAINDER", 1)
	bad = strings.Replace(bad, "GDPT2018_SAMPLE", "BAD_RULES", 1)
	failed = c.importFile(ctx, t, qlcmA, "curriculum_yaml", []byte(bad), false)
	if failed.str("status") != "failed" || !strings.Contains(failed.raw, `"field":"code"`) {
		t.Fatalf("rule errors %s", failed.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM curriculum.curricula WHERE code IN ('BAD_IMPORT','BAD_RULES')`); n != 0 {
		t.Fatal("a rejected import wrote data")
	}
	// AC-25: larger than csv_import_max or malformed → failed with a clear error.
	asset := c.assets.put([]byte(exp.raw))
	c.assets.sizes[asset] = 10<<20 + 1
	r := must(t, c.do(t, http.MethodPost, "/cms/learning/imports", qlcmA, idem(), map[string]any{"asset_id": asset.String(),
		"kind": "curriculum_yaml"}), http.StatusAccepted, "large import")
	if _, err := (job.ImportWorker{Runner: c.cmd}).RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	r = must(t, c.do(t, http.MethodGet, "/cms/learning/imports/"+r.str("import_id"), qlcmA, nil, nil), http.StatusOK, "large job")
	if r.str("status") != "failed" || !strings.Contains(r.raw, "FILE_TOO_LARGE") {
		t.Fatalf("too large %s", r.raw)
	}
	malformed := c.importFile(ctx, t, qlcmA, "curriculum_yaml", []byte("format_version: [1\ncurriculum: {"), false)
	if malformed.str("status") != "failed" || !strings.Contains(malformed.raw, "INVALID_FORMAT") {
		t.Fatalf("malformed %s", malformed.raw)
	}
	malformedCSV := c.importFile(ctx, t, qlcmA, "curriculum_csv", []byte("a,b\n1,2\n"), false)
	if malformedCSV.str("status") != "failed" || !strings.Contains(malformedCSV.raw, "INVALID_FORMAT") {
		t.Fatalf("malformed csv %s", malformedCSV.raw)
	}
	// Question imports are accepted since F024 (their rows are checked by the worker); tenant B never sees the jobs of tenant A.
	must(t, c.do(t, http.MethodPost, "/cms/learning/imports", qlcmA, idem(), map[string]any{"asset_id": asset.String(), "kind": "questions_csv"}),
		http.StatusAccepted, "question import")
	must(t, c.do(t, http.MethodGet, "/cms/learning/imports/"+r.str("import_id"), qlcmB, nil, nil), http.StatusNotFound, "B reads A's job")
	if c.audits(ctx, t, "content.imported", nil) < 5 {
		t.Fatal("imports not audited")
	}
}
