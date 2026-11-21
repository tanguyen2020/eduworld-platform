//go:build integration

package integration

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/secret"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// takeWave starts and submits the test of a student on a session (right = every answer right).
func (c *curEnv) takeWave(t *testing.T, sub, session string, right bool) {
	t.Helper()
	a := must(t, c.startExam(t, sub, session, devA, nil), http.StatusCreated, "start "+sub)
	must(t, c.answerExam(t, sub, a, devA, right, idem()), http.StatusOK, "submit "+sub)
}

// F035-AC-18 (US4): withdrawals by scope before a wave opens: the students leave the audience of the scheduled sessions,
// a class withdrawn as a whole has its scheduled session cancelled; idempotent; audited.
func TestEfficacyWithdraw(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.effSetup(ctx, t)
	study := c.activeStudy(t, k, "math-g3-r")
	pre := must(t, c.wave(t, study, "pre", 0, nil), http.StatusCreated, "pre")
	sessB, sessA := sessionOfCohort(t, pre, k.school2), sessionOfCohort(t, pre, k.school)
	withdraw := studies + "/" + study + ":withdraw"
	body := map[string]any{"scope": "students", "student_ids": []string{c.users[hsChi].UserID.String()}, "reason": "Phụ huynh xin không tham gia",
		"school_request_ref": "CV-12/2026"}
	if x := c.do(t, http.MethodPost, withdraw, saAdmin, merge(idem(), tenantHdr(c.tenantA)), map[string]any{"scope": "students",
		"reason": "Phụ huynh xin không tham gia"}); x.status != http.StatusUnprocessableEntity {
		t.Fatalf("students without ids: %d %s", x.status, x.raw)
	}
	key := merge(idem(), tenantHdr(c.tenantA))
	w := must(t, c.do(t, http.MethodPost, withdraw, saAdmin, key, body), http.StatusOK, "withdraw hs.chi")
	if again := must(t, c.do(t, http.MethodPost, withdraw, saAdmin, key, body), http.StatusOK, "replay"); again.str("withdrawal_id") != w.str("withdrawal_id") {
		t.Fatalf("replay %s", again.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.exam_participations WHERE exam_session_id = $1 AND deleted_at IS NULL`,
		sessB); n != 0 {
		t.Fatalf("hs.chi still in the audience of 3B: %d", n)
	}
	if c.audits(ctx, t, "efficacy.participants_withdrawn", &c.tenantA) != 1 {
		t.Fatal("audit efficacy.participants_withdrawn")
	}
	cw := must(t, c.do(t, http.MethodPost, withdraw, saAdmin, merge(idem(), tenantHdr(c.tenantA)), map[string]any{"scope": "cohort",
		"school_id": k.school, "reason": "Trường rút cụm khối 3 khỏi nghiên cứu"}), http.StatusOK, "withdraw cohort A")
	// ADR-0095: the scope class is refused.
	if x := c.do(t, http.MethodPost, withdraw, saAdmin, merge(idem(), tenantHdr(c.tenantA)), map[string]any{"scope": "class",
		"reason": "Trường rút lớp khỏi nghiên cứu"}); x.status != http.StatusUnprocessableEntity {
		t.Fatalf("scope class: %d %s", x.status, x.raw)
	}
	if cw.num("cancelled_exam_session_count") != 1 {
		t.Fatalf("cancelled sessions %s", cw.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.exam_sessions WHERE exam_session_id = $1 AND status = 'cancelled'`,
		sessA); n != 1 {
		t.Fatal("session of 3A not cancelled")
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.efficacy_wave_cohort_sessions WHERE exam_session_id = $1 AND status = 'cancelled'`,
		sessA); n != 1 {
		t.Fatal("wave session of 3A not cancelled")
	}
	// The pre wave opens: 3A is withdrawn (no participant) and hs.chi, withdrawn by name, never becomes one.
	c.clk.Advance(2 * time.Minute)
	c.runWaves(ctx, t)
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.efficacy_participants WHERE study_id = $1 AND school_id = $2 AND status = 'active'`, study,
		k.school); n != 0 {
		t.Fatalf("participants of the withdrawn 3A: %d", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.efficacy_participants WHERE study_id = $1`, study); n != 0 {
		t.Fatalf("participants after the withdrawals: %d", n)
	}
}

// F035-AC-19, AC-28…AC-34 (US4, US6, US7, SC-003, SC-005): a participant withdrawn after the pre wave leaves the post
// audience, the snapshots and every later report and dataset; the post wave snapshots M_eff; the report aggregates with
// small cells hidden and descriptive_only; the pseudonymous CSV goes through the F044 export queue; anonymizeUserData
// excludes the student.
func TestEfficacyReportAndDataset(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t, withMedia)
	k := c.effSetup(ctx, t)
	authztest.Cover(t, "createEfficacyStudyReport", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getEfficacyStudyReport", authztest.CrossTenant, authztest.RoleDenied)
	study := c.activeStudy(t, k, "math-g3-x")
	pre := must(t, c.wave(t, study, "pre", 0, nil), http.StatusCreated, "pre")
	c.clk.Advance(2 * time.Minute)
	c.runWaves(ctx, t)
	c.takeWave(t, hsAn, sessionOfCohort(t, pre, k.school), false)
	c.takeWave(t, hsChi, sessionOfCohort(t, pre, k.school2), false)
	// hs.binh is withdrawn after the pre wave (AC-18, AC-19).
	must(t, c.do(t, http.MethodPost, studies+"/"+study+":withdraw", saAdmin, merge(idem(), tenantHdr(c.tenantA)), map[string]any{"scope": "students",
		"student_ids": []string{c.users[hsBinh].UserID.String()}, "reason": "Trường yêu cầu loại học sinh"}), http.StatusOK, "withdraw hs.binh")
	c.clk.Advance(2 * time.Hour)
	c.runExamWindows(ctx, t)
	c.runWaves(ctx, t)
	c.clk.Advance(8 * 7 * 24 * time.Hour)
	post := must(t, c.wave(t, study, "post", 0, nil), http.StatusCreated, "post")
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.exam_participations WHERE exam_session_id = $1 AND student_id = $2`,
		sessionOfCohort(t, post, k.school), c.users[hsBinh].UserID); n != 0 {
		t.Fatal("withdrawn hs.binh in the post audience")
	}
	c.clk.Advance(2 * time.Minute)
	c.runWaves(ctx, t)
	// AC-28: M_eff snapshot of the active intervention participants on the LO of the study.
	var interventionClass uuid.UUID
	if err := c.platform.QueryRowContext(ctx, `SELECT school_id FROM learning.efficacy_study_cohorts WHERE study_id = $1 AND arm = 'intervention'`,
		study).Scan(&interventionClass); err != nil {
		t.Fatal(err)
	}
	wantSnaps := 1 // hs.chi (3B) or hs.an (3A); hs.binh withdrawn
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.efficacy_mastery_snapshots WHERE study_id = $1 AND deleted_at IS NULL`,
		study); n != wantSnaps {
		t.Fatalf("snapshots %d (intervention %s)", n, interventionClass)
	}
	c.takeWave(t, hsAn, sessionOfCohort(t, post, k.school), true)
	c.takeWave(t, hsChi, sessionOfCohort(t, post, k.school2), true)
	c.clk.Advance(2 * time.Hour)
	c.runExamWindows(ctx, t)
	c.runWaves(ctx, t)
	must(t, c.do(t, http.MethodPost, studies+"/"+study+":complete", saAdmin, idem(), nil), http.StatusOK, "complete")
	// AC-29…AC-31: the report job; aggregates only, cells under 5 hidden, descriptive_only with 1 class per arm.
	if x := c.do(t, http.MethodPost, studies+"/"+study+"/reports", adA, idem(), map[string]any{}); x.status != http.StatusForbidden {
		t.Fatalf("report as school admin: %d %s", x.status, x.raw)
	}
	key := idem()
	rb := map[string]any{"excluded_question_version_ids": []string{k.qA[0]}}
	r := must(t, c.do(t, http.MethodPost, studies+"/"+study+"/reports", saAdmin, key, rb), http.StatusAccepted, "report")
	if again := must(t, c.do(t, http.MethodPost, studies+"/"+study+"/reports", saAdmin, key, rb), http.StatusAccepted, "replay"); again.str("report_id") !=
		r.str("report_id") || r.str("status") != "queued" {
		t.Fatalf("report replay %s / %s", again.raw, r.raw)
	}
	if n, err := c.eff.RunEfficacyReports(ctx); err != nil || n != 1 {
		t.Fatalf("efficacy-reports: %d %v", n, err)
	}
	got := must(t, c.do(t, http.MethodGet, studies+"/"+study+"/reports/"+r.str("report_id"), saAdmin, nil, nil), http.StatusOK, "get report")
	if got.str("status") != "succeeded" || got.body["descriptive_only"] != true {
		t.Fatalf("report %s", got.raw)
	}
	for _, sub := range []string{hsAn, hsBinh, hsChi} {
		if strings.Contains(got.raw, c.users[sub].UserID.String()) {
			t.Fatalf("report holds the id of %s", sub)
		}
	}
	if strings.Contains(got.raw, "3A") || strings.Contains(got.raw, "3B") || !strings.Contains(got.raw, `"suppressed":true`) {
		t.Fatalf("report privacy %s", got.raw)
	}
	if c.audits(ctx, t, "efficacy_report.generated", nil) != 1 {
		t.Fatal("audit efficacy_report.generated")
	}
	// AC-32…AC-34: export through /admin/reports (F044 queue): without the secret → failed, nothing written.
	exp := map[string]any{"report_type": "efficacy_dataset", "format": "csv", "from": "2026-09-01", "to": "2027-06-01",
		"params": map[string]any{"study_id": study}}
	for _, sub := range []string{adA, cmNoidung} {
		if x := c.do(t, http.MethodPost, "/admin/reports", sub, idem(), exp); x.status != http.StatusForbidden {
			t.Fatalf("export as %s: %d %s", sub, x.status, x.raw)
		}
	}
	if x := c.do(t, http.MethodPost, "/admin/reports", saAdmin, idem(), map[string]any{"report_type": "efficacy_dataset", "format": "xlsx",
		"from": "2026-09-01", "to": "2027-06-01", "params": map[string]any{"study_id": study}}); x.status != http.StatusUnprocessableEntity {
		t.Fatalf("xlsx: %d %s", x.status, x.raw)
	}
	noKey := must(t, c.do(t, http.MethodPost, "/admin/reports", saAdmin, idem(), exp), http.StatusAccepted, "export without key").str("report_id")
	c.runReports(ctx, t)
	if g := must(t, c.do(t, http.MethodGet, "/admin/reports/"+noKey, saAdmin, nil, nil), http.StatusOK, "failed export"); g.str("status") != "failed" ||
		g.str("error") != "pseudonym_key_missing" {
		t.Fatalf("export without key %s", g.raw)
	}
	keyFile := filepath.Join(t.TempDir(), "efficacy.key")
	if err := os.WriteFile(keyFile, []byte(strings.Repeat("s", 48)), 0o600); err != nil {
		t.Fatal(err)
	}
	c.eff.Keys = secret.PseudonymKey{Path: keyFile}
	c.reports.Efficacy = c.eff
	job := must(t, c.do(t, http.MethodPost, "/admin/reports", saAdmin, idem(), exp), http.StatusAccepted, "export").str("report_id")
	c.runReports(ctx, t)
	g := must(t, c.do(t, http.MethodGet, "/admin/reports/"+job, saAdmin, nil, nil), http.StatusOK, "export done")
	if g.str("status") != "succeeded" || g.num("row_count") != 2 || g.str("download_url") == "" {
		t.Fatalf("export %s", g.raw)
	}
	status, file := download(t, g.str("download_url"))
	if status != http.StatusOK {
		t.Fatalf("download %d", status)
	}
	csv := string(file)
	header := ""
	for _, line := range strings.Split(csv, "\n") {
		if strings.HasPrefix(line, "participant_code") {
			header = line
		}
	}
	if header == "" {
		t.Fatalf("no header: %s", csv)
	}
	if !slices.Contains(strings.Split(header, ","), "cohort_code") {
		t.Fatalf("no cohort_code column (ADR-0095): %s", header)
	}
	for _, col := range []string{"student_id", "name", "class_id", "class_code", "school", "tenant", "birth"} {
		if slices.Contains(strings.Split(header, ","), col) {
			t.Fatalf("identifying column %s", col)
		}
	}
	for _, sub := range []string{hsAn, hsBinh, hsChi} {
		if strings.Contains(csv, c.users[sub].UserID.String()) {
			t.Fatalf("dataset holds the id of %s", sub)
		}
	}
	for _, id := range []uuid.UUID{k.school, k.school2, c.tenantA, k.school} {
		if strings.Contains(csv, id.String()) {
			t.Fatalf("dataset holds the id %s", id)
		}
	}
	if c.audits(ctx, t, "efficacy.dataset_exported", nil) != 1 || c.audits(ctx, t, "report.exported", nil) < 1 {
		t.Fatal("audit efficacy.dataset_exported / report.exported")
	}
	// AC-19: anonymizeUserData excludes hs.an from the study and soft-deletes his snapshots; later datasets leave him out.
	f := facade.Facade{Efficacy: c.eff}
	actx := tenancy.WithScope(authztest.Ctx(ctx, authztest.Platform("system_admin")), scopeOf(t, c.tenantA))
	res, err := f.AnonymizeUserData(actx, learningapi.AnonymizeUserDataRequestObject{UserId: c.users[hsAn].UserID})
	if err != nil {
		t.Fatal(err)
	}
	if res.(learningapi.AnonymizeUserData200JSONResponse).Status != "done" {
		t.Fatalf("anonymize %+v", res)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.efficacy_participants WHERE study_id = $1 AND student_id = $2
		AND status = 'excluded' AND exclusion_reason = 'anonymized'`, study, c.users[hsAn].UserID); n != 1 {
		t.Fatal("hs.an not excluded")
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.efficacy_mastery_snapshots WHERE student_id = $1 AND deleted_at IS NULL`,
		c.users[hsAn].UserID); n != 0 {
		t.Fatal("snapshots of hs.an kept")
	}
	cols, rows, err := c.eff.Dataset(ctx, uuid.MustParse(study))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || cols[0] != "participant_code" {
		t.Fatalf("dataset after anonymization: %d rows", len(rows))
	}
}
