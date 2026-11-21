//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.yaml.in/yaml/v3"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/regulation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// rvA is a tenant_content_reviewer of tenant A (learning_content.review, F044 US5).
const rvA = "rv.noidung.a"

// answerSeed is one synthetic graded answer (fixture of F044 T003): attempt score ratio, correctness, rapid guess, hint, option.
type answerSeed struct {
	student    uuid.UUID
	correct    bool
	total      float64 // attempt score out of 10
	rapid      bool
	hints      int
	option     string
	responseMS int
}

// seedAnswers inserts one graded attempt per seed with a single item of version (first scored answer). mode exam with a
// paper when paper != nil. Returns the attempt ids.
func (c *curEnv) seedAnswers(ctx context.Context, t *testing.T, tenant, school, version uuid.UUID, paper *uuid.UUID, seeds []answerSeed) []uuid.UUID {
	t.Helper()
	now := c.clk.Now().UTC()
	out := make([]uuid.UUID, 0, len(seeds))
	for i, s := range seeds {
		c.exec(ctx, t, `INSERT INTO ref.students (student_id, tenant_id, school_id, grade_level, source_version) VALUES ($1, $2, $3, 3, 1)
			ON CONFLICT (student_id) DO NOTHING`, s.student, tenant, school)
		a := uuid.New()
		mode, ctxType, kind, src, ctxID := "practice", "free_play", "selection", version, uuid.New()
		if paper != nil {
			mode, ctxType, kind, src = "exam", "exam_session", "exam_paper", *paper
		}
		c.exec(ctx, t, `INSERT INTO assessment.attempts (attempt_id, tenant_id, created_at, updated_at, school_id, student_id, mode, context_type,
				context_id, activity_type, source_kind, source_version_id, status, seed, started_at, last_activity_at, feedback_timing,
				display_language, item_count, objective_ids, idempotency_key, score, max_score, graded_at, submitted_at, exam_paper_id)
			VALUES ($1,$2,$3,$3,$4,$5,$6,$7,$8,'quiz',$9,$10,'graded',1,$3,$3,'after_submit','vi',1,'[]',$11,$12,10,$3,$3,$13)`,
			a, tenant, now, school, s.student, mode, ctxType, ctxID, kind, src, "seed-"+a.String()[:8], s.total, paper)
		score := 0.0
		if s.correct {
			score = 1
		}
		c.exec(ctx, t, `INSERT INTO assessment.attempt_items (attempt_item_id, tenant_id, created_at, updated_at, attempt_id, position,
				question_version_id, question_type, objective_ids, presentation, presented_payload_hash, max_score, final_score, final_is_correct,
				answer_count, hints_revealed)
			VALUES ($1,$2,$3,$3,$4,1,$5,'single_choice','[]','{}','h',1,$6,$7,1,$8)`, uuid.New(), tenant, now, a, version, score, s.correct, s.hints)
		resp, _ := json.Marshal(map[string]string{"option_id": s.option})
		c.exec(ctx, t, `INSERT INTO assessment.attempt_answers (attempt_answer_id, tenant_id, created_at, attempt_id, position, school_id,
				question_version_id, response, is_correct, score, answered_at, response_ms, attempt_no, is_scored, hints_used, rapid_guess, idempotency_key)
			VALUES ($1,$2,$3,$4,1,$5,$6,$7,$8,$9,$3,$10,1,true,$11,$12,$13)`, uuid.New(), tenant, now.Add(time.Duration(i)*time.Second), a, school,
			version, resp, s.correct, score, s.responseMS, s.hints, s.rapid, "ans-"+a.String()[:8])
		out = append(out, a)
	}
	return out
}

func tenantCtx(ctx context.Context, t *testing.T, tenant uuid.UUID) context.Context {
	t.Helper()
	sc, err := authz.EventScope(tenant)
	if err != nil {
		t.Fatal(err)
	}
	return tenancy.WithScope(tenancy.WithModule(ctx, "learning"), sc)
}

// TestItemAnalysisAndCalibration checks F044-AC-16…AC-21: the nightly job computes the statistics of the version from the
// first scored answers without rapid guesses (hand computed), per version; the published version does not change; the
// statistics of a question of tenant A are 404 for tenant B; the weekly job proposes the difficulty of the table; a tenant
// reviewer accepts it (a new draft version with change_note difficulty_calibration, idempotent, audited) and cannot accept
// the proposal of a platform question (403); without learning_content.review → 403.
func TestItemAnalysisAndCalibration(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.asgSetup(ctx, t)
	c.user(rvA, authztest.Staff(c.tenantA, "tenant_content_reviewer").PrincipalData, true)
	version := uuid.MustParse(c.published(ctx, t, qlcmA, k.s.qc, nil))
	var question uuid.UUID
	var declared int
	var hash []byte
	if err := c.platform.QueryRowContext(ctx, `SELECT question_id, difficulty, content_hash FROM content.question_versions WHERE question_version_id = $1`,
		version).Scan(&question, &declared, &hash); err != nil {
		t.Fatal(err)
	}
	ok, wrong := correctOption()
	right := 30 // p = 0.75 → difficulty 2; when declared is 2, 10 right → p = 0.25 → 5
	if declared == 2 {
		right = 10
	}
	var seeds []answerSeed
	for i := 0; i < 40; i++ {
		s := answerSeed{student: uuid.New(), correct: i < right, total: 2 + float64(i%5), option: wrong, responseMS: 4000}
		if s.correct {
			s.option, s.total = ok, s.total+4
		}
		seeds = append(seeds, s)
	}
	seeds = append(seeds, answerSeed{student: uuid.New(), correct: true, total: 10, rapid: true, option: ok, responseMS: 300},
		answerSeed{student: uuid.New(), correct: true, total: 9, hints: 1, option: ok, responseMS: 9000})
	c.seedAnswers(ctx, t, c.tenantA, k.school, version, nil, seeds)
	if n, err := c.quality.ComputeQuestionStats(ctx); err != nil || n < 1 {
		t.Fatalf("item analysis = %d %v", n, err)
	}
	r := must(t, c.do(t, http.MethodGet, "/cms/learning/questions/"+question.String()+"/stats", qlcmA, nil, nil), http.StatusOK, "stats")
	var pooled map[string]any
	for _, it := range r.items("items") {
		if it["grade_level"] == 0.0 {
			pooled = it
		}
	}
	wantP := float64(right) / 40
	if pooled == nil || pooled["p_value"] != wantP || pooled["first_attempt_count"] != 42.0 || pooled["excluded_rapid_guess_count"] != 1.0 ||
		pooled["with_hint_count"] != 1.0 || pooled["sufficient"] != true || pooled["discrimination"] == nil {
		t.Fatalf("pooled stats = %v (want p %v)", pooled, wantP)
	}
	if !strings.Contains(r.raw, "needs_calibration") || r.body["read_only"] != false {
		t.Fatalf("flags/read_only: %s", r.raw)
	}
	var after []byte
	var status string
	if err := c.platform.QueryRowContext(ctx, `SELECT status, content_hash FROM content.question_versions WHERE question_version_id = $1`,
		version).Scan(&status, &after); err != nil || status != "published" || string(after) != string(hash) {
		t.Fatalf("published version changed: %s %v", status, err)
	}
	if x := c.do(t, http.MethodGet, "/cms/learning/questions/"+question.String()+"/stats", qlcmB, nil, nil); x.status != http.StatusNotFound {
		t.Fatalf("tenant B reads stats of tenant A: %d", x.status)
	}

	// Weekly calibration (sample threshold lowered to the fixture size; the table is QUESTION_BANK §9.1).
	c.quality.Config.Calibration.MinResponses = 30
	if n, err := c.quality.ProposeCalibrations(ctx); err != nil || n != 1 {
		t.Fatalf("proposals = %d %v", n, err)
	}
	if n, _ := c.quality.ProposeCalibrations(ctx); n != 0 {
		t.Fatal("a pending proposal must not be proposed again")
	}
	list := must(t, c.do(t, http.MethodGet, "/cms/learning/calibration-proposals?status=pending", rvA, nil, nil), http.StatusOK, "queue")
	props := list.items("data")
	if len(props) != 1 || props[0]["current_difficulty"] != float64(declared) || props[0]["sample_size"] != 40.0 {
		t.Fatalf("queue = %s", list.raw)
	}
	pid := props[0]["proposal_id"].(string)
	proposed := props[0]["proposed_difficulty"].(float64)
	if x := c.do(t, http.MethodPost, "/cms/learning/calibration-proposals/"+pid+":accept", edA, idem(), nil); x.status != http.StatusForbidden {
		t.Fatalf("without learning_content.review = %d", x.status)
	}
	key := idem()
	acc := must(t, c.do(t, http.MethodPost, "/cms/learning/calibration-proposals/"+pid+":accept", rvA, key, nil), http.StatusOK, "accept")
	replay := must(t, c.do(t, http.MethodPost, "/cms/learning/calibration-proposals/"+pid+":accept", rvA, key, nil), http.StatusOK, "replay")
	if acc.str("status") != "accepted" || acc.str("created_version_id") == "" || replay.str("created_version_id") != acc.str("created_version_id") {
		t.Fatalf("accept = %s / %s", acc.raw, replay.raw)
	}
	var note string
	var diff int
	if err := c.platform.QueryRowContext(ctx, `SELECT status, change_note, difficulty FROM content.question_versions WHERE question_version_id = $1`,
		acc.str("created_version_id")).Scan(&status, &note, &diff); err != nil || status != "draft" || note != "difficulty_calibration" ||
		diff != int(proposed) {
		t.Fatalf("new version = %s %s %d %v", status, note, diff, err)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.question_versions WHERE question_id = $1`, question); n != 2 {
		t.Fatalf("versions = %d (replay must not create another)", n)
	}
	if err := c.platform.QueryRowContext(ctx, `SELECT status, difficulty FROM content.question_versions WHERE question_version_id = $1`,
		version).Scan(&status, &diff); err != nil || status != "published" || diff != declared {
		t.Fatalf("published version touched: %s %d", status, diff)
	}
	if c.audits(ctx, t, "content.calibration_accepted", &c.tenantA) != 1 {
		t.Fatal("content.calibration_accepted not audited")
	}

	// A platform question assigned to tenant A: its proposal is visible read-only and refused to the tenant reviewer.
	pqs, pvs := c.question(t, cmNoidung, k.s.qc, map[string]any{"owner_scope": "platform"})
	pq, pv := uuid.MustParse(pqs), uuid.MustParse(pvs)
	c.assignResource(ctx, t, "question", pqs, c.tenantA)
	platformProposal := uuid.New()
	c.exec(ctx, t, `INSERT INTO content.difficulty_calibration_proposals (proposal_id, tenant_id, owner_scope, question_id, based_on_version_id,
		current_difficulty, proposed_difficulty, p_value, sample_size, grade_level) VALUES ($1, NULL, 'platform', $2, $3, 3, 5, 0.2, 120, 3)`,
		platformProposal, pq, pv)
	if x := c.do(t, http.MethodPost, "/cms/learning/calibration-proposals/"+platformProposal.String()+":accept", rvA, idem(), nil); x.status != http.StatusForbidden {
		t.Fatalf("tenant reviewer on a platform proposal = %d %s", x.status, x.raw)
	}
	dis := must(t, c.do(t, http.MethodPost, "/cms/learning/calibration-proposals/"+platformProposal.String()+":dismiss", cmDuyet, idem(), nil),
		http.StatusOK, "platform reviewer dismisses")
	if dis.str("status") != "dismissed" {
		t.Fatalf("dismiss = %s", dis.raw)
	}
}

// TestExamQualityReport checks F044-AC-22…AC-25 (TC-EXM-001, TC-EXM-004, TC-TEN-001): a 4-variant independent_draw session
// gets expected_p per variant and the balance result; once the official session of 32 students is closed the worker
// computes KR-20 and SEM of the variant with ≥ 30 students, the distribution and the p-value of the items without rapid
// guesses; another tenant gets 404, a student 403; no named answer in the report.
func TestExamQualityReport(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.examSetup(ctx, t, 6, map[string]any{"strategy": "independent_draw", "count": 4, "balance": map[string]any{"enabled": true}})
	sess := must(t, c.do(t, http.MethodPost, examSessionsPath, qlcmA, idem(), c.sessionBody(k, nil)), http.StatusCreated, "session")
	bal, _ := sess.body["balance"].(map[string]any)
	if bal == nil || len(bal["variants"].([]any)) != 4 || bal["max_mean_p_diff"] != 0.05 {
		t.Fatalf("balance = %s", sess.raw)
	}
	id := uuid.MustParse(sess.str("exam_session_id"))
	c.exec(ctx, t, `UPDATE assessment.exam_sessions SET is_official = true WHERE exam_session_id = $1`, id)
	papers := must(t, c.do(t, http.MethodGet, examSessionsPath+"/"+id.String()+"/papers", qlcmA, nil, nil), http.StatusOK, "papers").items("data")
	if len(papers) != 4 || papers[0]["expected_p"] == nil {
		t.Fatalf("papers = %v", papers)
	}
	paper := uuid.MustParse(papers[0]["exam_paper_id"].(string))
	var items []uuid.UUID
	for _, s := range papers[0]["sections"].([]any) {
		for _, it := range s.(map[string]any)["items"].([]any) {
			items = append(items, uuid.MustParse(it.(map[string]any)["question_version_id"].(string)))
		}
	}
	// 32 students on variant 1, one item of the paper per attempt (fixture), item 1 right for the top 20.
	ok, wrong := correctOption()
	var seeds []answerSeed
	for i := 0; i < 32; i++ {
		s := answerSeed{student: uuid.New(), correct: i < 20, total: float64(i%10) + 1, option: wrong, responseMS: 5000, rapid: i == 31}
		if s.correct {
			s.option = ok
		}
		seeds = append(seeds, s)
	}
	attempts := c.seedAnswers(ctx, t, c.tenantA, k.school, items[0], &paper, seeds)
	for i, s := range seeds {
		c.exec(ctx, t, `INSERT INTO assessment.exam_participations (exam_session_id, student_id, tenant_id, school_id, status, attempts_used,
			last_attempt_id, effective_duration_sec) VALUES ($1, $2, $3, $4, 'graded', 1, $5, 900)`, id, s.student, c.tenantA, k.school, attempts[i])
	}
	c.exec(ctx, t, `UPDATE assessment.exam_sessions SET status = 'closed', closed_at = now() WHERE exam_session_id = $1`, id)
	before := must(t, c.do(t, http.MethodGet, examSessionsPath+"/"+id.String()+"/report", qlcmA, nil, nil), http.StatusOK, "report before")
	if before.body["computed"] != false {
		t.Fatalf("report before close job = %s", before.raw)
	}
	tctx := tenantCtx(ctx, t, c.tenantA)
	if err := c.uow.Do(tctx, func(ctx context.Context) error { return c.quality.ComputeExamQuality(ctx, id) }); err != nil {
		t.Fatal(err)
	}
	rep := must(t, c.do(t, http.MethodGet, examSessionsPath+"/"+id.String()+"/report", qlcmA, nil, nil), http.StatusOK, "report")
	if rep.body["computed"] != true || rep.num("student_count") != 32 || len(rep.body["score_distribution"].([]any)) != 21 {
		t.Fatalf("report = %s", rep.raw)
	}
	v1 := rep.items("variants")[0]
	if v1["n"] != 32.0 || v1["enough_data"] != true {
		t.Fatalf("variant = %v", v1)
	}
	var item map[string]any
	for _, it := range rep.items("items") {
		if it["question_version_id"] == items[0].String() {
			item = it
		}
	}
	// 31 answers without the rapid guess: 20 right → p = 20/31.
	if item == nil || item["n"] != 31.0 || item["p_value"].(float64) < 0.645 || item["p_value"].(float64) > 0.646 {
		t.Fatalf("item = %v", item)
	}
	for _, s := range seeds {
		if strings.Contains(rep.raw, s.student.String()) {
			t.Fatal("the report names a student")
		}
	}
	if x := c.do(t, http.MethodGet, examSessionsPath+"/"+id.String()+"/report", adB, nil, nil); x.status != http.StatusNotFound {
		t.Fatalf("tenant B = %d", x.status)
	}
	if x := c.do(t, http.MethodGet, examSessionsPath+"/"+id.String()+"/report", hsAn, nil, nil); x.status != http.StatusForbidden {
		t.Fatalf("student = %d", x.status)
	}
}

// TestReferenceLevels checks F044-AC-11, AC-12, AC-14, AC-15 (TC-LRN-018): the nightly computation maps the masteries of
// the period scope (excellent with every LO proficient and half mastered; no evidence → insufficient_data); the report
// carries the disclaimer, the rapid-guess ratio and the profile version; a new profile version writes new rows and keeps
// the rows of the earlier version; an academic_manager outside the subject gets 403; the export of the reference report
// is audited learning.reference_report_exported.
func TestReferenceLevels(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t, withMedia)
	k := c.asgSetup(ctx, t)
	for _, lo := range []struct {
		id, level string
	}{{k.s.qc.lo, "mastered"}, {k.s.qc.lo2, "proficient"}} {
		c.exec(ctx, t, `INSERT INTO learning.objective_masteries (objective_mastery_id, tenant_id, school_id, student_id, objective_id, value,
			level, evidence_count, model_version) VALUES ($1, $2, $3, $4, $5, 0.9, $6, 5, 'we-1.0')`, uuid.New(), c.tenantA, k.school,
			c.users[hsAn].UserID, lo.id, lo.level)
	}
	tctx := tenantCtx(ctx, t, c.tenantA)
	if n, err := c.quality.ReferenceLevelsFor(tctx, 2026, regulation.PeriodEndTerm2); err != nil || n < 2 {
		t.Fatalf("reference levels = %d %v", n, err)
	}
	path := "/lms/schools/" + k.school.String() + "/reference-levels?subject_id=" + k.s.qc.subject + "&period=end_term_2&academic_year=2026"
	r := must(t, c.do(t, http.MethodGet, path, qlcmA, nil, nil), http.StatusOK, "reference report")
	levels := map[string]map[string]any{}
	for _, it := range r.items("items") {
		levels[it["student_id"].(string)] = it
	}
	an, binh := levels[c.users[hsAn].UserID.String()], levels[c.users[hsBinh].UserID.String()]
	if an == nil || an["level_code"] != "excellent" || an["profile_version"] != 1.0 || binh == nil || binh["level_code"] != "insufficient_data" {
		t.Fatalf("levels = %s", r.raw)
	}
	if !strings.Contains(r.str("disclaimer"), "không thay thế đánh giá chính thức") || an["rapid_guess_ratio"] == nil {
		t.Fatalf("disclaimer/rapid ratio: %s", r.raw)
	}
	// ADR-0095: the grade of the student when the level was computed, and the grade filter (no class).
	if an["grade_level"] != 3.0 || strings.Contains(r.raw, "class_id") {
		t.Fatalf("grade of the level: %s", r.raw)
	}
	if g := must(t, c.do(t, http.MethodGet, path+"&grade_level=3", qlcmA, nil, nil), http.StatusOK, "grade 3"); len(g.items("items")) != len(r.items("items")) {
		t.Fatalf("grade 3 filter: %s", g.raw)
	}
	if g := must(t, c.do(t, http.MethodGet, path+"&grade_level=4", qlcmA, nil, nil), http.StatusOK, "grade 4"); len(g.items("items")) != 0 {
		t.Fatalf("grade 4 filter: %s", g.raw)
	}
	// New thresholds → version 2; the rows of version 1 stay.
	rules := regulation.DefaultRules()
	rules.ExcellentMasteredRatio = 0.9
	if _, err := c.quality.PublishProfileVersion(tctx, regulation.ProfilePrimaryTT27, rules, nil); err == nil {
		t.Fatal("a tenant cannot supersede the platform profile in its own session")
	}
	pctx, err := tenancy.Bypass(tenancy.WithModule(ctx, "learning"), tenancy.ReasonPlatformJob)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.quality.PublishProfileVersion(pctx, regulation.ProfilePrimaryTT27, rules, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.quality.ReferenceLevelsFor(tctx, 2026, regulation.PeriodEndTerm2); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, c.platform, `SELECT count(DISTINCT profile_version) FROM learning.rm_student_subject_reference_levels
		WHERE student_id = $1`, c.users[hsAn].UserID); n != 2 {
		t.Fatalf("profile versions kept = %d", n)
	}
	r = must(t, c.do(t, http.MethodGet, path, qlcmA, nil, nil), http.StatusOK, "reference report v2")
	for _, it := range r.items("items") {
		if it["student_id"] == c.users[hsAn].UserID.String() && (it["profile_version"] != 2.0 || it["level_code"] != "completed") {
			t.Fatalf("v2 = %v", it)
		}
	}
	other := uuid.New()
	c.user("qlcm.van.a", authztest.Staff(c.tenantA, "academic_manager", authztest.AtSchool(k.school), authztest.ForSubject(other)).PrincipalData, true)
	if x := c.do(t, http.MethodGet, path, "qlcm.van.a", nil, nil); x.status != http.StatusForbidden || x.str("code") != "PERMISSION_DENIED" {
		t.Fatalf("subject out of scope = %d %s", x.status, x.raw)
	}
	// Export of the reference report: disclaimer at the top and bottom, audit learning.reference_report_exported.
	job := must(t, c.do(t, http.MethodPost, "/lms/reports", qlcmA, idem(), map[string]any{"report_type": "regulation_reference", "format": "csv",
		"school_id": k.school.String(), "subject_id": k.s.qc.subject, "period": "end_term_2", "academic_year": 2026}), http.StatusAccepted,
		"reference export").str("report_id")
	c.runReports(ctx, t)
	got := must(t, c.do(t, http.MethodGet, "/lms/reports/"+job, qlcmA, nil, nil), http.StatusOK, "reference export done")
	_, file := download(t, got.str("download_url"))
	s := string(file)
	if strings.Count(s, "không thay thế đánh giá chính thức") != 2 || !strings.Contains(s, "Hoàn thành") {
		t.Fatalf("reference file = %s", s)
	}
	if c.audits(ctx, t, "learning.reference_report_exported", &c.tenantA) != 1 {
		t.Fatal("learning.reference_report_exported not audited")
	}
}

// TestNoCircularLevelForStudents checks F044-AC-13 (contract): no operation reachable by students (game client channel:
// /me, /game, /exams/attempts) returns a reference level or its schema.
func TestNoCircularLevelForStudents(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), "specs", "openapi", "eduworld-api.v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	for p, ops := range spec.Paths {
		if !strings.HasPrefix(p, "/me") && !strings.HasPrefix(p, "/game/") && !strings.HasPrefix(p, "/exams/attempts") {
			continue
		}
		b, _ := yaml.Marshal(ops)
		for _, banned := range []string{"RegulationReferenceReport", "level_code", "AssessmentPeriod"} {
			if strings.Contains(string(b), banned) {
				t.Errorf("student operation %s exposes %s", p, banned)
			}
		}
	}
}
