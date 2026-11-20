//go:build integration

package integration

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/reporting"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/storage"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// amB is an academic_manager of school SB (tenant B).
const amB = "dash.am.b"

// reportKit is the dashboard dataset of F043 plus a course 3A × math with one graded assignment (a1 8/10 submitted, a2 3/10
// with rapid guesses) and one practice assignment (not a results column), and a course 3A × Vietnamese.
type reportKit struct {
	*dashKit
	courseM, courseV, graded, practice uuid.UUID
}

func (c *curEnv) reportKit(ctx context.Context, t *testing.T) reportKit {
	t.Helper()
	k := reportKit{dashKit: c.dashKit(ctx, t), courseM: uuid.New(), courseV: uuid.New()}
	for _, cs := range []struct{ id, subject uuid.UUID }{{k.courseM, k.subjM}, {k.courseV, k.subjV}} {
		c.exec(ctx, t, `INSERT INTO lms.courses (course_id, tenant_id, school_id, class_id, subject_id, grade_level, academic_year_id,
			curriculum_version_id) VALUES ($1, $2, $3, $4, $5, 3, $6, $7)`, cs.id, c.tenantA, k.school, k.class3A, cs.subject, uuid.New(), uuid.New())
	}
	due := dashNow.Add(-24 * time.Hour)
	k.graded = k.assignment(ctx, t, c.tenantA, k.school, k.subjM, "closed", due, nil)
	k.practice = k.assignment(ctx, t, c.tenantA, k.school, k.subjM, "closed", due, nil)
	c.exec(ctx, t, `UPDATE lms.assignments SET mode = 'graded', course_id = $2, follow_latest = false, pinned_version_id = $3,
		score_policy = 'best' WHERE assignment_id = $1`, k.graded, k.courseM, uuid.New())
	c.exec(ctx, t, `UPDATE lms.assignments SET course_id = $2 WHERE assignment_id = $1`, k.practice, k.courseM)
	for _, s := range []struct {
		student      uuid.UUID
		status       string
		score, rapid int
	}{{k.a1, "submitted", 8, 0}, {k.a2, "late", 3, 2}} {
		c.exec(ctx, t, `INSERT INTO lms.assignment_submissions (assignment_submission_id, tenant_id, school_id, assignment_id, student_id,
			class_id, status, score, max_score, rapid_guess_count, attempts_used) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 10, $9, 1)`,
			uuid.New(), c.tenantA, k.school, k.graded, s.student, k.class3A, s.status, s.score, s.rapid)
	}
	return k
}

// runReports runs the worker until the queue is empty.
func (c *curEnv) runReports(ctx context.Context, t *testing.T) {
	t.Helper()
	for i := 0; i < 5; i++ {
		n, err := c.reports.RunReportJobs(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			return
		}
	}
}

func download(t *testing.T, url string) (int, []byte) {
	t.Helper()
	res, err := http.Get(url) //nolint:gosec,noctx // presigned URL of the test object storage
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, b
}

// TestReportExportJob checks F044-AC-01…AC-03 (TC-DASH-008): 202 with the same report_id on replay; the worker writes an
// XLSX with the watermark to ew-exports, only the rows of the assignment, no chat column; the link works for its creator
// within dashboard_export_link_ttl_minutes and the object storage answers 403 once it expired; audit report.exported.
func TestReportExportJob(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t, withMedia)
	k := c.reportKit(ctx, t)
	key := idem()
	body := map[string]any{"report_type": "assignment_results", "format": "xlsx", "assignment_id": k.graded.String()}
	first := must(t, c.do(t, http.MethodPost, "/lms/reports", amM, key, body), http.StatusAccepted, "create report")
	again := must(t, c.do(t, http.MethodPost, "/lms/reports", amM, key, body), http.StatusAccepted, "replay")
	if first.str("report_id") == "" || again.str("report_id") != first.str("report_id") || first.str("status") != "queued" {
		t.Fatalf("report ids %q %q status %q", first.str("report_id"), again.str("report_id"), first.str("status"))
	}
	c.runReports(ctx, t)
	got := must(t, c.do(t, http.MethodGet, "/lms/reports/"+first.str("report_id"), amM, nil, nil), http.StatusOK, "get report")
	if got.str("status") != "succeeded" || got.str("download_url") == "" || got.num("row_count") != 2 {
		t.Fatalf("report = %s", got.raw)
	}
	exp, _ := time.Parse(time.RFC3339, got.str("download_expires_at"))
	if d := exp.Sub(c.clk.Now()); d != 15*time.Minute {
		t.Fatalf("link valid for %v, want dashboard_export_link_ttl_minutes = 15 min", d)
	}
	status, file := download(t, got.str("download_url"))
	if status != http.StatusOK {
		t.Fatalf("download = %d", status)
	}
	x, err := excelize.OpenReader(bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := x.GetRows("Report")
	all := ""
	for _, r := range rows {
		all += strings.Join(r, "|") + "\n"
	}
	user := c.users[amM].UserID.String()
	if !strings.Contains(rows[1][0], user) || !strings.Contains(rows[len(rows)-1][0], user) {
		t.Fatalf("watermark missing (top/bottom): %v", rows)
	}
	if !strings.Contains(all, k.a1.String()) || !strings.Contains(all, k.a2.String()) || strings.Contains(all, k.b1.String()) ||
		strings.Contains(strings.ToLower(all), "chat") || !strings.Contains(all, "8.0") {
		t.Fatalf("file rows = %s", all)
	}
	// Past the link TTL the object storage refuses the presigned GET (403).
	stale, _, err := storage.ExportBucket{S3: c.media.s3}.ReportURL(ctx, reporting.StorageKey(&c.tenantA, uuid.MustParse(first.str("report_id")), "xlsx"),
		time.Now().Add(-20*time.Minute), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if status, _ := download(t, stale); status != http.StatusForbidden {
		t.Fatalf("expired link = %d, want 403", status)
	}
	if n := c.audits(ctx, t, "report.exported", &c.tenantA); n != 1 {
		t.Fatalf("report.exported audits = %d", n)
	}
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'report.completed'`); n != 1 {
		t.Fatalf("report.completed events = %d", n)
	}
}

// TestReportExportScopeAndLimits checks F044-AC-03…AC-05 and AC-07 (TC-DASH-008, TC-TEN-001, TC-TEN-024, TC-DASH-002): an
// academic_manager limited to math cannot export the Vietnamese course (403), another school is 404; a job is read by its
// creator only (another user 404, another tenant 404, a student 403); at most 2 running exports per user (429
// EXPORT_LIMIT_REACHED + Retry-After); over dashboard_export_max_rows → 422; invalid enum → 422; range > 730 days → 422
// DASHBOARD_RANGE_TOO_LARGE.
func TestReportExportScopeAndLimits(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t, withMedia)
	k := c.reportKit(ctx, t)
	vBody := map[string]any{"report_type": "course_results", "format": "csv", "course_id": k.courseV.String()}
	if r := c.do(t, http.MethodPost, "/lms/reports", amM, idem(), vBody); r.status != http.StatusForbidden || r.str("code") != "PERMISSION_DENIED" {
		t.Fatalf("subject out of scope = %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodPost, "/lms/reports", adS2, idem(), vBody); r.status != http.StatusNotFound {
		t.Fatalf("other school = %d %s", r.status, r.raw)
	}
	mBody := map[string]any{"report_type": "course_results", "format": "csv", "course_id": k.courseM.String()}
	job := must(t, c.do(t, http.MethodPost, "/lms/reports", amM, idem(), mBody), http.StatusAccepted, "course export").str("report_id")
	c.user(amB, authztest.Staff(c.tenantB, "academic_manager", authztest.AtSchool(k.schoolB)).PrincipalData, true)
	for _, sub := range []string{taDash, amB} {
		if r := c.do(t, http.MethodGet, "/lms/reports/"+job, sub, nil, nil); r.status != http.StatusNotFound {
			t.Fatalf("%s reads the job of another user: %d %s", sub, r.status, r.raw)
		}
	}
	if r := c.do(t, http.MethodGet, "/tenant/reports/"+job, amM, nil, nil); r.status != http.StatusNotFound {
		t.Fatalf("job read through another route = %d", r.status)
	}
	if r := c.do(t, http.MethodGet, "/lms/reports/"+job, hsA1, nil, nil); r.status != http.StatusForbidden {
		t.Fatalf("student = %d", r.status)
	}
	must(t, c.do(t, http.MethodPost, "/lms/reports", amM, idem(), mBody), http.StatusAccepted, "second running export")
	busy := c.do(t, http.MethodPost, "/lms/reports", amM, idem(), mBody)
	if busy.status != http.StatusTooManyRequests || busy.str("code") != "EXPORT_LIMIT_REACHED" || busy.header.Get("Retry-After") == "" {
		t.Fatalf("third export = %d %s", busy.status, busy.raw)
	}
	c.runReports(ctx, t)
	// The worker applied the subject scope of the requester: the file of course M lists both students of the graded column only.
	got := must(t, c.do(t, http.MethodGet, "/lms/reports/"+job, amM, nil, nil), http.StatusOK, "course export done")
	if got.str("status") != "succeeded" || got.num("row_count") != 2 {
		t.Fatalf("course export = %s", got.raw)
	}
	c.reports.Config.MaxRows = 1
	if r := c.do(t, http.MethodPost, "/lms/reports", amM, idem(), mBody); r.status != http.StatusUnprocessableEntity ||
		r.str("code") != "VALIDATION_FAILED" {
		t.Fatalf("over max rows = %d %s", r.status, r.raw)
	}
	c.reports.Config = reporting.DefaultConfig()
	if r := c.do(t, http.MethodPost, "/lms/reports", amM, idem(), map[string]any{"report_type": "course_results", "format": "docx",
		"course_id": k.courseM.String()}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("format docx = %d", r.status)
	}
	if r := c.do(t, http.MethodPost, "/tenant/reports", taDash, idem(), map[string]any{"report_type": "class_progress", "format": "csv",
		"from": "2023-01-01", "to": "2026-12-01"}); r.status != http.StatusUnprocessableEntity || r.str("code") != "DASHBOARD_RANGE_TOO_LARGE" {
		t.Fatalf("range = %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodPost, "/lms/reports", hsA1, idem(), mBody); r.status != http.StatusForbidden {
		t.Fatalf("student export = %d", r.status)
	}
	// A staff member without report.export: 403 on create and on read (F044-AC-05).
	if r := c.do(t, http.MethodPost, "/lms/reports", noRoleA, idem(), mBody); r.status != http.StatusForbidden {
		t.Fatalf("export without report.export = %d", r.status)
	}
	if r := c.do(t, http.MethodGet, "/lms/reports/"+job, noRoleA, nil, nil); r.status != http.StatusForbidden {
		t.Fatalf("read without report.export = %d", r.status)
	}
}

// TestPlatformReport checks F044-AC-06 (TC-TEN-004, TC-TEN-018): System Admin exports a cross-tenant summary with a tenant
// column, the cells under analytics_min_cell_size students hidden, audited cross_tenant.accessed and report.exported; a
// tenant role on /admin/reports gets 403.
func TestPlatformReport(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t, withMedia)
	k := c.reportKit(ctx, t)
	for _, r := range []struct {
		tenant   uuid.UUID
		students int
	}{{c.tenantA, 12}, {c.tenantB, 3}} {
		c.exec(ctx, t, `INSERT INTO learning.rollup_tenant_daily (tenant_id, grade_level, subject_id, bucket_date, active_students,
			activities_completed, accuracy_num, accuracy_den, active_seconds, is_final) VALUES ($1, 3, $2, date '2026-12-07', $3, 40, 30, 40, 3600, true)`,
			r.tenant, k.subjM, r.students)
	}
	body := map[string]any{"report_type": "learning_summary", "format": "csv", "from": "2026-12-01", "to": "2026-12-09",
		"tenant_ids": []string{c.tenantA.String(), c.tenantB.String()}}
	if r := c.do(t, http.MethodPost, "/admin/reports", taDash, idem(), body); r.status != http.StatusForbidden {
		t.Fatalf("tenant role on /admin/reports = %d %s", r.status, r.raw)
	}
	before := c.audits(ctx, t, audit.ActionCrossTenantAccessed, nil)
	job := must(t, c.do(t, http.MethodPost, "/admin/reports", saDsh, idem(), body), http.StatusAccepted, "platform report").str("report_id")
	c.runReports(ctx, t)
	got := must(t, c.do(t, http.MethodGet, "/admin/reports/"+job, saDsh, nil, nil), http.StatusOK, "platform report done")
	if got.str("status") != "succeeded" || got.num("row_count") != 2 {
		t.Fatalf("platform report = %s", got.raw)
	}
	_, file := download(t, got.str("download_url"))
	s := string(file)
	if !strings.Contains(s, "tenant_id") || !strings.Contains(s, c.tenantA.String()+","+k.subjM.String()+",12,40,75.0,60") ||
		!strings.Contains(s, c.tenantB.String()+","+k.subjM.String()+",<5,,,") {
		t.Fatalf("platform file = %s", s)
	}
	if c.audits(ctx, t, audit.ActionCrossTenantAccessed, nil) <= before || c.audits(ctx, t, "report.exported", nil) < 1 {
		t.Fatal("cross_tenant.accessed / report.exported not audited")
	}
}

// TestCourseResults checks F044-AC-08…AC-10 (TC-TEN-002, TC-TEN-003): one column per graded assignment with its score
// policy, the effective score on the 10-point scale, status and the "too fast" flag; no XP, rating nor ranking; the
// Vietnamese course is 403 for a math academic_manager, another school or tenant 404.
func TestCourseResults(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.reportKit(ctx, t)
	r := must(t, c.do(t, http.MethodGet, "/lms/courses/"+k.courseM.String()+"/results", adS, nil, nil), http.StatusOK, "course results")
	cols := r.items("columns")
	if len(cols) != 1 || cols[0]["assignment_id"] != k.graded.String() || cols[0]["score_policy"] != "best" {
		t.Fatalf("columns = %v", cols)
	}
	scores := map[string]map[string]any{}
	for _, row := range r.items("rows") {
		sc := row["scores"].([]any)[0].(map[string]any)
		scores[row["student_id"].(string)] = sc
	}
	if a1 := scores[k.a1.String()]; a1["score_10"] != 8.0 || a1["status"] != "submitted" || a1["too_fast"] != false {
		t.Fatalf("a1 = %v", a1)
	}
	if a2 := scores[k.a2.String()]; a2["score_10"] != 3.0 || a2["too_fast"] != true {
		t.Fatalf("a2 = %v", a2)
	}
	for _, banned := range []string{"xp", "rating", "competition", "rank"} {
		if strings.Contains(strings.ToLower(r.raw), banned) {
			t.Fatalf("course results expose %q: %s", banned, r.raw)
		}
	}
	// Re-evaluation changed the effective score: the table follows (edge case of F044-AC-08).
	c.exec(ctx, t, `UPDATE lms.assignment_submissions SET score = 9 WHERE assignment_id = $1 AND student_id = $2`, k.graded, k.a1)
	r = must(t, c.do(t, http.MethodGet, "/lms/courses/"+k.courseM.String()+"/results", amM, nil, nil), http.StatusOK, "after regrade")
	if !strings.Contains(r.raw, `"score_10":9`) {
		t.Fatalf("effective score after re-evaluation: %s", r.raw)
	}
	if x := c.do(t, http.MethodGet, "/lms/courses/"+k.courseV.String()+"/results", amM, nil, nil); x.status != http.StatusForbidden ||
		x.str("code") != "PERMISSION_DENIED" {
		t.Fatalf("subject out of scope = %d %s", x.status, x.raw)
	}
	for _, sub := range []string{adS2, adSB} {
		if x := c.do(t, http.MethodGet, "/lms/courses/"+k.courseM.String()+"/results", sub, nil, nil); x.status != http.StatusNotFound {
			t.Fatalf("%s = %d", sub, x.status)
		}
	}
}
