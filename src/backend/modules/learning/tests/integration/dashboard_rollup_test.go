//go:build integration

package integration

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
)

type rollupRow struct {
	active, activeAll                                   sql.NullInt64
	acts, accNum, accDen, rapidNum, rapidDen, sec, less int64
	final                                               bool
}

func (r rollupRow) String() string {
	return fmt.Sprintf("active=%v all=%v acts=%d acc=%d/%d rapid=%d/%d sec=%d final=%v", r.active.Int64, r.activeAll.Int64, r.acts,
		r.accNum, r.accDen, r.rapidNum, r.rapidDen, r.sec, r.final)
}

// rollup reads one rollup row (as learning_platform: BYPASSRLS) by its scope condition.
func (k *dashKit) rollup(ctx context.Context, t *testing.T, table, cond string, args ...any) rollupRow {
	t.Helper()
	var r rollupRow
	err := k.c.platform.QueryRowContext(ctx, `SELECT active_students, active_students_all, activities_completed, accuracy_num, accuracy_den,
			rapid_guess_num, rapid_guess_den, active_seconds, lessons_completed, is_final
		FROM learning.`+table+` WHERE metric_version = 1 AND `+cond, args...).Scan(&r.active, &r.activeAll, &r.acts, &r.accNum, &r.accDen,
		&r.rapidNum, &r.rapidDen, &r.sec, &r.less, &r.final)
	if err != nil {
		t.Fatalf("%s %s: %v", table, cond, err)
	}
	return r
}

func expectRow(t *testing.T, what string, got rollupRow, active, activeAll, acts, accNum, accDen, rapidNum, rapidDen, sec int64, final bool) {
	t.Helper()
	if got.active.Int64 != active || got.activeAll.Int64 != activeAll || got.acts != acts || got.accNum != accNum || got.accDen != accDen ||
		got.rapidNum != rapidNum || got.rapidDen != rapidDen || got.sec != sec || got.final != final {
		t.Errorf("%s: got %s, want active=%d all=%d acts=%d acc=%d/%d rapid=%d/%d sec=%d final=%v", what, got, active, activeAll, acts,
			accNum, accDen, rapidNum, rapidDen, sec, final)
	}
}

// F043-AC-01, F043-AC-03, F043-AC-05 (TC-DASH-004, SC-001): the golden dataset through the consumer and the day close gives the
// counters computed by hand at every level (school × grade at event time, tenant; day, week, month — ADR-0095: no class),
// exact distinct active students, accuracy without rapid guesses, numerator/denominator pairs, is_final and a bumped data version.
func TestDashboardRollupGolden(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.dashKit(ctx, t)
	k.golden(ctx, t)
	A := c.tenantA
	d1, d2 := date("2026-12-07"), date("2026-12-08")

	// F043-AC-01: provisional rows of the event day at every level (is_final = false, distinct counts not yet computed).
	r := k.rollup(ctx, t, "rollup_school_daily", "tenant_id = $1 AND school_id = $2 AND grade_level = 3 AND subject_id = $3 AND bucket_date = $4",
		A, k.school, k.subjM, d1)
	if r.final || r.active.Valid || r.acts != 3 || r.accNum != 12 || r.accDen != 14 || r.rapidNum != 5 || r.rapidDen != 19 || r.sec != 960 {
		t.Fatalf("provisional school × grade row: %s", r)
	}
	// ADR-0095: every progress row records the grade of the student at event time (the class level is dropped, WP-R).
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.progress_daily WHERE tenant_id = $1 AND grade_level IS NULL`, A); n != 0 {
		t.Fatalf("progress_daily rows without a grade: %d", n)
	}
	r = k.rollup(ctx, t, "rollup_tenant_daily", "tenant_id = $1 AND grade_level = 3 AND subject_id = $2 AND bucket_date = $3", A, k.subjM, d1)
	if r.final || r.acts != 3 || r.accNum != 12 {
		t.Fatalf("provisional tenant row: %s", r)
	}

	n, err := k.jobs.CloseDays(ctx)
	if err != nil || n != 3 { // A: 2026-12-07, 2026-12-08; B: 2026-12-07
		t.Fatalf("close days: %d %v", n, err)
	}
	// F043-AC-05 accuracy (8−1)+5+(1−1) / (10−2)+5+(4−3) = 12/14, rapid 5/19; exam attempts count answers and time but no
	// completed activity (LEARNING_ANALYTICS §4): S g3 M d2 = b1's exam only.
	expectRow(t, "S g3 M d2 exam", k.rollup(ctx, t, "rollup_school_daily", "tenant_id = $1 AND school_id = $2 AND grade_level = 3 AND subject_id = $3 AND bucket_date = $4",
		A, k.school, k.subjM, d2), 1, 2, 0, 7, 10, 0, 10, 900, true)
	// School × grade: distinct students of the subject, and of every subject (a1, a2, b1 on d1).
	expectRow(t, "S g3 M d1", k.rollup(ctx, t, "rollup_school_daily", "tenant_id = $1 AND school_id = $2 AND grade_level = 3 AND subject_id = $3 AND bucket_date = $4",
		A, k.school, k.subjM, d1), 2, 3, 3, 12, 14, 5, 19, 960, true)
	expectRow(t, "S g3 V d1", k.rollup(ctx, t, "rollup_school_daily", "tenant_id = $1 AND school_id = $2 AND grade_level = 3 AND subject_id = $3 AND bucket_date = $4",
		A, k.school, k.subjV, d1), 1, 3, 1, 3, 6, 0, 6, 120, true)
	// Tenant × grade.
	expectRow(t, "A g3 M d1", k.rollup(ctx, t, "rollup_tenant_daily", "tenant_id = $1 AND grade_level = 3 AND subject_id = $2 AND bucket_date = $3",
		A, k.subjM, d1), 2, 3, 3, 12, 14, 5, 19, 960, true)
	expectRow(t, "A g4 M d1", k.rollup(ctx, t, "rollup_tenant_daily", "tenant_id = $1 AND grade_level = 4 AND subject_id = $2 AND bucket_date = $3",
		A, k.subjM, d1), 1, 1, 1, 3, 3, 0, 3, 180, true)
	// Weeks and months are rebuilt from the day table with exact distinct counts over the window; the week of 2026-12-07
	// ends after the last closable day (2026-12-09): provisional.
	week := date("2026-12-07")
	expectRow(t, "S g3 M week", k.rollup(ctx, t, "rollup_school_weekly", "tenant_id = $1 AND school_id = $2 AND grade_level = 3 AND subject_id = $3 AND bucket_date = $4",
		A, k.school, k.subjM, week), 3, 3, 3, 19, 24, 5, 29, 1860, false)
	expectRow(t, "S g3 V week", k.rollup(ctx, t, "rollup_school_weekly", "tenant_id = $1 AND school_id = $2 AND grade_level = 3 AND subject_id = $3 AND bucket_date = $4",
		A, k.school, k.subjV, week), 2, 3, 2, 9, 13, 1, 14, 360, false)
	expectRow(t, "A g3 M month", k.rollup(ctx, t, "rollup_tenant_monthly", "tenant_id = $1 AND grade_level = 3 AND subject_id = $2 AND bucket_date = $3",
		A, k.subjM, date("2026-12-01")), 3, 3, 3, 19, 24, 5, 29, 1860, false)
	// Tenant B is rolled up in its own rows only.
	expectRow(t, "B g3 M d1", k.rollup(ctx, t, "rollup_tenant_daily", "tenant_id = $1 AND grade_level = 3 AND subject_id = $2 AND bucket_date = $3",
		c.tenantB, k.subjM, d1), 1, 1, 1, 1, 2, 0, 2, 100, true)
	// F043-AC-03: days closed, progress_daily final, cache version of each tenant bumped.
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.rollup_days WHERE tenant_id = $1 AND is_final AND NOT needs_reclose`, A); n != 2 {
		t.Fatalf("closed days of A: %d", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.progress_daily WHERE tenant_id = $1 AND NOT is_final`, A); n != 0 {
		t.Fatalf("progress_daily not final: %d", n)
	}
	if k.versions.count(A) != 1 || k.versions.count(c.tenantB) != 1 {
		t.Fatalf("version bumps %d %d", k.versions.count(A), k.versions.count(c.tenantB))
	}
	// A second run has nothing to close and bumps nothing.
	if n, err := k.jobs.CloseDays(ctx); err != nil || n != 0 || k.versions.count(A) != 1 {
		t.Fatalf("second close: %d %v bumps %d", n, err, k.versions.count(A))
	}
}

// F043-AC-02, F043-AC-04 (TC-DASH-005, AC-DASH-007): a replayed event_id is counted once; an event 30 h late on a closed day
// is added and the day is reclosed; an event 50 h late leaves the closed day unchanged and is counted in
// eduworld_rollup_late_events_total.
func TestDashboardRollupLateAndDuplicate(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.dashKit(ctx, t)
	A := c.tenantA
	d9 := date("2026-12-09")
	raw := k.run(ctx, t, A, k.a1, k.school, k.subjM, ict(2026, 12, 9, 8), "practice", 30, write.ItemCounts{Answered: 2, Correct: 2})
	k.run(ctx, t, A, k.a2, k.school, k.subjM, ict(2026, 12, 8, 10), "practice", 30, write.ItemCounts{Answered: 1, Correct: 1})
	// F043-AC-02: the same event_id again → processed_events keeps it once, no second count.
	if _, err := k.js.Publish(ctx, natsx.EventSubject("learning", "activity.graded"), raw); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)
	r := k.rollup(ctx, t, "rollup_school_daily", "tenant_id = $1 AND school_id = $2 AND grade_level = 3 AND subject_id = $3 AND bucket_date = $4",
		A, k.school, k.subjM, d9)
	if r.acts != 1 || r.accDen != 2 {
		t.Fatalf("replayed event counted twice: %s", r)
	}
	if n, err := k.jobs.CloseDays(ctx); err != nil || n != 2 {
		t.Fatalf("close: %d %v", n, err)
	}
	before := testutil.ToFloat64(observability.LateEvents().WithLabelValues("learning", ">48h"))
	within := testutil.ToFloat64(observability.LateEvents().WithLabelValues("learning", "<=48h"))
	// 30 hours late (2026-12-09 06:00 ICT, processed 2026-12-10 12:00): added to the closed day, day marked for reclosing.
	k.run(ctx, t, A, k.b1, k.school, k.subjM, dashNow.Add(-30*time.Hour), "practice", 60, write.ItemCounts{Answered: 4, Correct: 4})
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.rollup_days WHERE tenant_id = $1 AND bucket_date = $2 AND needs_reclose`, A, d9); n != 1 {
		t.Fatal("30 h late event did not mark the day for reclosing")
	}
	// 50 hours late (2026-12-08 10:00 ICT): rollups of the closed day unchanged, metric only.
	k.run(ctx, t, A, k.b1, k.school, k.subjM, dashNow.Add(-50*time.Hour), "practice", 60, write.ItemCounts{Answered: 9, Correct: 9})
	if got := testutil.ToFloat64(observability.LateEvents().WithLabelValues("learning", ">48h")) - before; got != 1 {
		t.Fatalf("late events >48h: %v", got)
	}
	if got := testutil.ToFloat64(observability.LateEvents().WithLabelValues("learning", "<=48h")) - within; got != 1 {
		t.Fatalf("late events <=48h: %v", got)
	}
	if n, err := k.jobs.CloseDays(ctx); err != nil || n != 1 {
		t.Fatalf("reclose: %d %v", n, err)
	}
	expectRow(t, "S g3 M d9 reclosed", k.rollup(ctx, t, "rollup_school_daily",
		"tenant_id = $1 AND school_id = $2 AND grade_level = 3 AND subject_id = $3 AND bucket_date = $4", A, k.school, k.subjM, d9),
		2, 2, 2, 6, 6, 0, 6, 90, true)
	expectRow(t, "S g3 M d8 unchanged", k.rollup(ctx, t, "rollup_school_daily",
		"tenant_id = $1 AND school_id = $2 AND grade_level = 3 AND subject_id = $3 AND bucket_date = $4", A, k.school, k.subjM, date("2026-12-08")),
		1, 1, 1, 1, 1, 0, 1, 30, true)
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.rollup_days WHERE tenant_id = $1 AND needs_reclose`, A); n != 0 {
		t.Fatal("reclose flag kept")
	}
}

// F043-AC-06, F043-AC-28 (TC-LMS-003): the refresh job rebuilds the state read models (school × grade × objective level
// distribution, school × grade × subject summary with the assignment completion by grade when targeted), the "needs
// attention" list with the reasons of dashboard_attention_thresholds, the grade × objective week and the weekly mastery
// snapshot (ADR-0095: no class read model).
func TestDashboardReadModelRefresh(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.dashKit(ctx, t)
	k.golden(ctx, t)
	k.seedState(ctx, t)
	if n, err := k.jobs.Refresh(ctx); err != nil || n != 2 {
		t.Fatalf("refresh: %d %v", n, err)
	}
	A := c.tenantA
	var dist []int64
	var with, size int
	// O1: a1, a2 beginning, b1 proficient; a3, b2 without level → not_started 2 (5 students of S × grade 3).
	if err := c.platform.QueryRowContext(ctx, `SELECT level_dist, students_with_evidence, student_count FROM learning.rm_grade_objective_stats
		WHERE tenant_id = $1 AND school_id = $2 AND grade_level = 3 AND objective_id = $3`, A, k.school, k.objectives[0]).Scan(&dist, &with, &size); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(dist) != "[2 2 0 1 0]" || with != 3 || size != 5 {
		t.Fatalf("S × 3 × O1 level dist %v with %d of %d", dist, with, size)
	}
	if err := c.platform.QueryRowContext(ctx, `SELECT level_dist FROM learning.rm_grade_objective_stats
		WHERE tenant_id = $1 AND school_id = $2 AND grade_level = 3 AND objective_id = $3`, A, k.school, k.objectives[2]).Scan(&dist); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(dist) != "[3 1 0 0 1]" {
		t.Fatalf("S × 3 × O3 level dist %v", dist)
	}
	// Tenant B has its own heatmap rows only.
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.rm_grade_objective_stats WHERE tenant_id = $1 AND school_id <> $2`,
		c.tenantB, k.schoolB); n != 0 {
		t.Fatalf("tenant B rows of another school: %d", n)
	}
	var sc, partNum, partDen int
	var compNum, compDen int64
	if err := c.platform.QueryRowContext(ctx, `SELECT student_count, participation_num, participation_den, assignment_completion_num,
			assignment_completion_den FROM learning.rm_school_grade_summary
		WHERE tenant_id = $1 AND school_id = $2 AND grade_level = 3 AND subject_id = $3`, A, k.school, k.subjM).Scan(&sc, &partNum, &partDen,
		&compNum, &compDen); err != nil {
		t.Fatal(err)
	}
	// Open math assignment: a1 submitted, the other four not started.
	if sc != 5 || partNum != 3 || partDen != 5 || compNum != 1 || compDen != 5 {
		t.Fatalf("school × grade 3 × M: %d %d/%d completion %d/%d", sc, partNum, partDen, compNum, compDen)
	}
	if err := c.platform.QueryRowContext(ctx, `SELECT assignment_completion_num, assignment_completion_den FROM learning.rm_school_grade_summary
		WHERE tenant_id = $1 AND school_id = $2 AND grade_level = 3 AND subject_id = $3`, A, k.school, k.subjV).Scan(&compNum, &compDen); err != nil {
		t.Fatal(err)
	}
	if compNum != 0 || compDen != 2 { // b1 missed two Vietnamese assignments
		t.Fatalf("school × grade 3 × V completion %d/%d", compNum, compDen)
	}
	reasons := map[uuid.UUID]string{}
	rows, err := c.platform.QueryContext(ctx, `SELECT student_id, array_to_string(reasons, ',') FROM learning.rm_student_attention
		WHERE tenant_id = $1`, A)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id uuid.UUID
		var r string
		if err := rows.Scan(&id, &r); err != nil {
			t.Fatal(err)
		}
		reasons[id] = r
	}
	_ = rows.Close()
	// F043-AC-08: a1 three objectives at beginning; a2 rapid guesses ≥ 30 %; b1 two missed assignments in 14 days; c1 none.
	if reasons[k.a1] != "beginning_objectives" || reasons[k.a2] != "rapid_guess" || reasons[k.b1] != "missed_assignments" || reasons[k.c1] != "" {
		t.Fatalf("attention reasons %v", reasons)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.mastery_snapshots_weekly WHERE tenant_id = $1 AND bucket_date = $2`,
		A, date("2026-12-07")); n != 3 {
		t.Fatalf("weekly mastery snapshots: %d", n)
	}
	// The grade × objective week gets the current level distribution at each refresh.
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.rollup_grade_objective_weekly WHERE tenant_id = $1 AND bucket_date = $2
		AND school_id = $3 AND grade_level = 3 AND level_dist[2] = 2`, A, date("2026-12-07"), k.school); n != 1 {
		t.Fatalf("grade × objective week with levels: %d", n)
	}
}

// seedState seeds the masteries and assignments of the state read models: an open math assignment targeting the five
// grade-3 students of S (grade when targeted 3: a1 submitted), two closed Vietnamese assignments missed by b1.
func (k *dashKit) seedState(ctx context.Context, t *testing.T) {
	t.Helper()
	c, A := k.c, k.c.tenantA
	for _, o := range k.objectives {
		k.mastery(ctx, t, A, k.school, k.a1, o, "beginning", 0.2)
	}
	k.mastery(ctx, t, A, k.school, k.a2, k.objectives[0], "beginning", 0.25)
	k.mastery(ctx, t, A, k.school, k.a2, k.objectives[2], "mastered", 0.95)
	k.mastery(ctx, t, A, k.school, k.b1, k.objectives[0], "proficient", 0.8)
	k.mastery(ctx, t, c.tenantB, k.schoolB, k.x1, k.objectives[0], "developing", 0.5)
	due := dashNow.Add(24 * time.Hour)
	k.assignment(ctx, t, A, k.school, k.subjM, "open", due, map[uuid.UUID][2]any{k.a1: {3, "submitted"}, k.a2: {3, "not_started"},
		k.b1: {3, "not_started"}, k.a3: {3, "not_started"}, k.b2: {3, "not_started"}})
	past := dashNow.Add(-72 * time.Hour)
	for i := 0; i < 2; i++ {
		k.assignment(ctx, t, A, k.school, k.subjV, "closed", past, map[uuid.UUID][2]any{k.b1: {3, "missed"}})
	}
}
