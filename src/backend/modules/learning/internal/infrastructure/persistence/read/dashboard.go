package read

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// DashboardModel is the query catalog of the dashboards (F043 T022, DASHBOARD_DATA §8 rule 6): every statement reads the
// rollups (rollup_*, progress_daily, mastery_snapshots_weekly) or the state read models (rm_*, objective_masteries,
// rm_student_recommendations, lms.assignment_submissions/assignments, ref.*), never learning_events, attempts nor
// attempt_answers; it filters the tenant(s) explicitly (RLS on top for the role learning_dashboard), the scope of the grant
// and a bounded range. Derived tables have no deleted_at (DASHBOARD_DATA §3.1); sources are filtered on deleted_at IS NULL.
type DashboardModel struct{}

var _ read.DashboardReader = DashboardModel{}

// Catalog lists the query ids of the dashboards with one representative statement each (EXPLAIN test, TC-DASH-001).
var Catalog = []string{"school_totals", "grade_totals", "school_series", "active_students", "grades",
	"objective_stats", "attention", "assignments", "self_study", "grade_students", "profile_subjects", "profile_topics",
	"profile_trend", "profile_assignments", "profile_recommendations", "school_cells", "tenant_cells", "available_from"}

var grainTables = map[analytics.Grain]string{analytics.Day: "daily", analytics.Week: "weekly", analytics.Month: "monthly"}

func suffix(rg analytics.Range) string {
	if s, ok := grainTables[rg.Grain]; ok {
		return s
	}
	return "daily"
}

func strIDs(ids []uuid.UUID) []string {
	if ids == nil {
		return nil
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

func day(t time.Time) string { return t.Format(time.DateOnly) }

// MapError is dashErr for test models of the catalog (TEST hook).
func MapError(what string, err error) error { return dashErr(what, err) }

// dashErr maps a statement cancelled by statement_timeout to 504 DASHBOARD_QUERY_TIMEOUT (F043-AC-19).
func dashErr(what string, err error) error {
	if err == nil {
		return nil
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "57014" {
		return problem.New(problem.DashboardQueryTimeout, "narrow the range or the scope").Wrap(err)
	}
	return fmt.Errorf("dashboard %s: %w", what, err)
}

// meta accumulates the computed_at / is_final of the rows read.
type metaAcc struct{ m read.RollupMeta }

func (a *metaAcc) add(at sql.NullTime, partial bool) {
	if at.Valid {
		t := at.Time.UTC()
		a.m.Merge(read.RollupMeta{Oldest: &t, Partial: partial})
	} else if partial {
		a.m.Partial = true
	}
}

// dq is the read transaction of one catalog query; when the context carries a plan collector (test hook of the EXPLAIN
// check, TC-DASH-001) the plan of every statement is recorded before it runs.
type dq struct {
	tx *sql.Tx
	id string
}

func dtx(ctx context.Context, id string) (dq, error) {
	tx, err := sqlTx(ctx)
	return dq{tx: tx, id: id}, err
}

func (d dq) explain(ctx context.Context, q string, args []any) {
	p, ok := ctx.Value(plansKey{}).(*Plans)
	if !ok {
		return
	}
	rows, err := d.tx.QueryContext(ctx, "EXPLAIN "+q, args...)
	if err != nil {
		p.add(d.id, "EXPLAIN failed: "+err.Error())
		return
	}
	defer func() { _ = rows.Close() }()
	var lines []string
	for rows.Next() {
		var l string
		if rows.Scan(&l) == nil {
			lines = append(lines, l)
		}
	}
	if err := rows.Err(); err != nil {
		lines = append(lines, "EXPLAIN failed: "+err.Error())
	}
	p.add(d.id, strings.Join(lines, "\n"))
}

func (d dq) query(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	d.explain(ctx, q, args)
	return d.tx.QueryContext(ctx, q, args...)
}

func (d dq) row(ctx context.Context, q string, args ...any) *sql.Row {
	d.explain(ctx, q, args)
	return d.tx.QueryRowContext(ctx, q, args...)
}

// Plans collects the query plans of the catalog (TEST hook of the EXPLAIN check: never set by a binary).
type Plans struct {
	mu      sync.Mutex
	ByQuery map[string][]string
}

type plansKey struct{}

// WithPlans returns ctx collecting the plans of the catalog queries into p.
func WithPlans(ctx context.Context, p *Plans) context.Context {
	return context.WithValue(ctx, plansKey{}, p)
}

func (p *Plans) add(id, plan string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ByQuery == nil {
		p.ByQuery = map[string][]string{}
	}
	p.ByQuery[id] = append(p.ByQuery[id], plan)
}

// Prepare implements read.DashboardReader: statement_timeout of the transaction (the role default is the same value).
func (DashboardModel) Prepare(ctx context.Context, timeout time.Duration) error {
	q, err := sqlTx(ctx)
	if err != nil {
		return err
	}
	if timeout <= 0 {
		return nil
	}
	if _, err := q.ExecContext(ctx, `SELECT set_config('statement_timeout', $1, true)`, strconv.FormatInt(timeout.Milliseconds(), 10)); err != nil {
		return dashErr("prepare", err)
	}
	return nil
}

// SchoolExists implements read.DashboardReader.
func (DashboardModel) SchoolExists(ctx context.Context, tenantID, schoolID uuid.UUID) (bool, error) {
	q, err := dtx(ctx, "school")
	if err != nil {
		return false, err
	}
	var ok bool
	// tenancyguard:ok ref.schools projection (no deleted_at), explicit tenant_id + RLS
	err = q.row(ctx, `SELECT EXISTS (SELECT 1 FROM ref.schools WHERE tenant_id = $1 AND school_id = $2)`, tenantID, schoolID).Scan(&ok)
	return ok, dashErr("school", err)
}

// Student implements read.DashboardReader (a deleted student is not found, spec edge case).
func (DashboardModel) Student(ctx context.Context, tenantID, studentID uuid.UUID) (*read.DashStudent, error) {
	q, err := dtx(ctx, "student")
	if err != nil {
		return nil, err
	}
	var s read.DashStudent
	err = q.row(ctx, `SELECT student_id, school_id FROM ref.students WHERE tenant_id = $1 AND student_id = $2 AND deleted_at IS NULL`,
		tenantID, studentID).Scan(&s.StudentID, &s.SchoolID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dashErr("student", err)
	}
	return &s, nil
}

// totalsCols are the summed counters of a rollup row set r, then the meta columns.
const totalsCols = `COALESCE(sum(r.activities_completed), 0), COALESCE(sum(r.lessons_completed), 0), COALESCE(sum(r.accuracy_num), 0),
	COALESCE(sum(r.accuracy_den), 0), COALESCE(sum(r.rapid_guess_num), 0), COALESCE(sum(r.rapid_guess_den), 0),
	COALESCE(sum(r.active_seconds), 0), COALESCE(sum(r.assignments_due), 0), COALESCE(sum(r.assignments_completed), 0),
	COALESCE(sum(r.assignments_on_time), 0), min(r.computed_at), COALESCE(bool_or(NOT r.is_final), false)`

func scanTotals(row interface{ Scan(...any) error }, t *read.RollupTotals, extra ...any) error {
	var at sql.NullTime
	var partial bool
	dest := append(extra, &t.Activities, &t.Lessons, &t.AccNum, &t.AccDen, &t.RapidNum, &t.RapidDen, &t.ActiveSeconds, &t.AssignDue,
		&t.AssignCompleted, &t.AssignOnTime, &at, &partial)
	if err := row.Scan(dest...); err != nil {
		return err
	}
	var a metaAcc
	a.add(at, partial)
	t.RollupMeta = a.m
	return nil
}

// grades is the SQL int[] literal of the visible grades of a scope (nil = no filter).
func grades(gs []int) any {
	if gs == nil {
		return nil
	}
	parts := make([]string, len(gs))
	for i, g := range gs {
		parts[i] = strconv.Itoa(g)
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// SchoolTotals implements read.DashboardReader (query school_totals: rollup_school_* of the visible grades of the scope).
func (DashboardModel) SchoolTotals(ctx context.Context, sc read.DashScope, rg analytics.Range) (read.RollupTotals, error) {
	q, err := dtx(ctx, "school_totals")
	if err != nil {
		return read.RollupTotals{}, err
	}
	var t read.RollupTotals
	// tenancyguard:ok derived rollup (no deleted_at): explicit tenant_id, scope and bounded bucket range
	err = scanTotals(q.row(ctx, fmt.Sprintf(`SELECT `+totalsCols+` FROM learning.rollup_school_%s r
		WHERE r.tenant_id = $1 AND ($2::uuid[] IS NULL OR r.school_id = ANY($2::uuid[])) AND ($3::int IS NULL OR r.grade_level = $3)
			AND ($4::uuid[] IS NULL OR r.subject_id = ANY($4::uuid[])) AND r.bucket_date BETWEEN $5::date AND $6::date AND r.metric_version = 1
			AND ($7::int[] IS NULL OR r.grade_level = ANY($7::int[]))`,
		suffix(rg)), sc.TenantID, strIDs(sc.SchoolIDs), sc.GradeLevel, strIDs(sc.SubjectIDs), day(rg.QueryFrom()), day(rg.To), grades(sc.Grades)), &t)
	return t, dashErr("school_totals", err)
}

// GradeTotals implements read.DashboardReader (query grade_totals: rollup_school_* per grade, grade at event time).
func (DashboardModel) GradeTotals(ctx context.Context, sc read.DashScope, rg analytics.Range) (map[int]read.RollupTotals, read.RollupMeta, error) {
	q, err := dtx(ctx, "grade_totals")
	if err != nil {
		return nil, read.RollupMeta{}, err
	}
	// tenancyguard:ok derived rollup (no deleted_at): explicit tenant_id, scope and bounded bucket range
	rows, err := q.query(ctx, fmt.Sprintf(`SELECT r.grade_level, `+totalsCols+` FROM learning.rollup_school_%s r
		WHERE r.tenant_id = $1 AND ($2::uuid[] IS NULL OR r.school_id = ANY($2::uuid[])) AND ($3::int IS NULL OR r.grade_level = $3)
			AND ($4::uuid[] IS NULL OR r.subject_id = ANY($4::uuid[])) AND r.bucket_date BETWEEN $5::date AND $6::date AND r.metric_version = 1
		GROUP BY r.grade_level`, suffix(rg)), sc.TenantID, strIDs(sc.SchoolIDs), sc.GradeLevel, strIDs(sc.SubjectIDs),
		day(rg.QueryFrom()), day(rg.To))
	if err != nil {
		return nil, read.RollupMeta{}, dashErr("grade_totals", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[int]read.RollupTotals{}
	var m read.RollupMeta
	for rows.Next() {
		var g int
		var t read.RollupTotals
		if err := scanTotals(rows, &t, &g); err != nil {
			return nil, m, dashErr("grade_totals", err)
		}
		out[g] = t
		m.Merge(t.RollupMeta)
	}
	return out, m, dashErr("grade_totals", rows.Err())
}

// SchoolSeries implements read.DashboardReader (query school_series): per bucket, the active students are the "all
// subjects" distinct count of each grade summed, or the per-subject counts when subjects are limited; visible grades only.
func (DashboardModel) SchoolSeries(ctx context.Context, sc read.DashScope, rg analytics.Range) ([]read.SeriesPoint, read.RollupMeta, error) {
	q, err := dtx(ctx, "school_series")
	if err != nil {
		return nil, read.RollupMeta{}, err
	}
	// tenancyguard:ok derived rollup (no deleted_at): explicit tenant_id, scope and bounded bucket range
	rows, err := q.query(ctx, fmt.Sprintf(`SELECT x.bucket_date, bool_and(x.fin), sum(x.act), sum(x.acts), sum(x.less), sum(x.an),
			sum(x.ad), sum(x.sec), sum(x.due), sum(x.done), min(x.at), bool_or(NOT x.fin)
		FROM (SELECT r.bucket_date, bool_and(r.is_final) AS fin,
				CASE WHEN $4::uuid[] IS NULL THEN COALESCE(max(r.active_students_all), 0) ELSE COALESCE(sum(r.active_students), 0) END AS act,
				sum(r.activities_completed) AS acts, sum(r.lessons_completed) AS less, sum(r.accuracy_num) AS an, sum(r.accuracy_den) AS ad,
				sum(r.active_seconds) AS sec, sum(r.assignments_due) AS due, sum(r.assignments_completed) AS done, min(r.computed_at) AS at
			FROM learning.rollup_school_%s r
			WHERE r.tenant_id = $1 AND ($2::uuid[] IS NULL OR r.school_id = ANY($2::uuid[]))
				AND ($3::int IS NULL OR r.grade_level = $3) AND ($4::uuid[] IS NULL OR r.subject_id = ANY($4::uuid[]))
				AND r.bucket_date BETWEEN $5::date AND $6::date AND r.metric_version = 1
				AND ($7::int[] IS NULL OR r.grade_level = ANY($7::int[]))
			GROUP BY r.bucket_date, r.school_id, r.grade_level) x
		GROUP BY x.bucket_date ORDER BY x.bucket_date LIMIT 120`, suffix(rg)),
		sc.TenantID, strIDs(sc.SchoolIDs), sc.GradeLevel, strIDs(sc.SubjectIDs), day(rg.QueryFrom()), day(rg.To), grades(sc.Grades))
	if err != nil {
		return nil, read.RollupMeta{}, dashErr("school_series", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.SeriesPoint
	var a metaAcc
	for rows.Next() {
		var p read.SeriesPoint
		var at sql.NullTime
		var partial bool
		if err := rows.Scan(&p.Bucket, &p.Final, &p.ActiveStudents, &p.Activities, &p.Lessons, &p.AccNum, &p.AccDen, &p.ActiveSec, &p.AssignDue,
			&p.AssignCompleted, &at, &partial); err != nil {
			return nil, a.m, dashErr("school_series", err)
		}
		p.Bucket = analytics.DateOf(p.Bucket)
		a.add(at, partial)
		out = append(out, p)
	}
	return out, a.m, dashErr("school_series", rows.Err())
}

// ActiveStudents implements read.DashboardReader (query active_students: progress_daily, bounded window; grade = grade
// recorded on the row, visible grades only).
func (DashboardModel) ActiveStudents(ctx context.Context, sc read.DashScope, from, to time.Time) (int, error) {
	q, err := dtx(ctx, "active_students")
	if err != nil {
		return 0, err
	}
	var n int
	// tenancyguard:ok derived progress_daily (no deleted_at): explicit tenant_id, scope and window
	err = q.row(ctx, `SELECT count(DISTINCT p.student_id) FROM learning.progress_daily p
		WHERE p.tenant_id = $1 AND ($2::uuid[] IS NULL OR p.school_id = ANY($2::uuid[]))
			AND ($3::int IS NULL OR p.grade_level = $3) AND ($4::int[] IS NULL OR COALESCE(p.grade_level, 0) = ANY($4::int[]))
			AND ($5::uuid[] IS NULL OR p.subject_id = ANY($5::uuid[])) AND p.bucket_date BETWEEN $6::date AND $7::date
			AND p.metric_version = 1`, sc.TenantID, strIDs(sc.SchoolIDs), sc.GradeLevel, grades(sc.Grades), strIDs(sc.SubjectIDs), day(from),
		day(to)).Scan(&n)
	return n, dashErr("active_students", err)
}

// Grades implements read.DashboardReader (query grades: rm_school_grade_summary per grade). Students is the size of the
// grade whatever the subject filter; the active students and the assignment completion are read on the subjects of the scope.
func (DashboardModel) Grades(ctx context.Context, sc read.DashScope) ([]read.GradeSummary, error) {
	q, err := dtx(ctx, "grades")
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok derived read model (no deleted_at): explicit tenant_id and scope
	rows, err := q.query(ctx, `SELECT s.grade_level, COALESCE(max(s.student_count), 0),
			COALESCE(max(s.participation_num) FILTER (WHERE $4::uuid[] IS NULL OR s.subject_id = ANY($4::uuid[])), 0),
			COALESCE(sum(s.assignment_completion_num) FILTER (WHERE $4::uuid[] IS NULL OR s.subject_id = ANY($4::uuid[])), 0),
			COALESCE(sum(s.assignment_completion_den) FILTER (WHERE $4::uuid[] IS NULL OR s.subject_id = ANY($4::uuid[])), 0),
			min(s.computed_at)
		FROM learning.rm_school_grade_summary s
		WHERE s.tenant_id = $1 AND ($2::uuid[] IS NULL OR s.school_id = ANY($2::uuid[])) AND ($3::int IS NULL OR s.grade_level = $3)
		GROUP BY s.grade_level ORDER BY s.grade_level LIMIT 13`, sc.TenantID, strIDs(sc.SchoolIDs), sc.GradeLevel, strIDs(sc.SubjectIDs))
	if err != nil {
		return nil, dashErr("grades", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.GradeSummary
	for rows.Next() {
		var g read.GradeSummary
		if err := rows.Scan(&g.GradeLevel, &g.Students, &g.Active7d, &g.CompNum, &g.CompDen, &g.ComputedAt); err != nil {
			return nil, dashErr("grades", err)
		}
		out = append(out, g)
	}
	return out, dashErr("grades", rows.Err())
}

// ObjectiveStats implements read.DashboardReader (query objective_stats: rm_grade_objective_stats, the LO heatmap of the
// grades of the scope).
func (DashboardModel) ObjectiveStats(ctx context.Context, sc read.DashScope) ([]read.ObjectiveStat, error) {
	q, err := dtx(ctx, "objective_stats")
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok derived read model (no deleted_at): explicit tenant_id and scope
	rows, err := q.query(ctx, `SELECT s.grade_level, s.objective_id, s.objective_code, COALESCE(s.yccd_ref, ''), s.level_dist,
			s.students_with_evidence, s.student_count, s.computed_at
		FROM learning.rm_grade_objective_stats s
		WHERE s.tenant_id = $1 AND ($2::uuid[] IS NULL OR s.school_id = ANY($2::uuid[])) AND ($3::int IS NULL OR s.grade_level = $3)
			AND ($4::uuid[] IS NULL OR s.subject_id = ANY($4::uuid[])) AND s.students_with_evidence > 0
		ORDER BY s.objective_code, s.objective_id, s.grade_level LIMIT 2000`,
		sc.TenantID, strIDs(sc.SchoolIDs), sc.GradeLevel, strIDs(sc.SubjectIDs))
	if err != nil {
		return nil, dashErr("objective_stats", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.ObjectiveStat
	for rows.Next() {
		var s read.ObjectiveStat
		var dist []int64
		if err := rows.Scan(&s.GradeLevel, &s.ObjectiveID, &s.Code, &s.YccdRef, &dist, &s.WithEvidence, &s.Students, &s.ComputedAt); err != nil {
			return nil, dashErr("objective_stats", err)
		}
		s.LevelDist = ints(dist)
		out = append(out, s)
	}
	return out, dashErr("objective_stats", rows.Err())
}

func ints(xs []int64) []int {
	out := make([]int, len(xs))
	for i, x := range xs {
		out[i] = int(x)
	}
	return out
}

// Attention implements read.DashboardReader (query attention: rm_student_attention of not deleted students, current grade
// of ref.students).
func (DashboardModel) Attention(ctx context.Context, sc read.DashScope, limit int) ([]read.AttentionEntry, int, error) {
	q, err := dtx(ctx, "attention")
	if err != nil {
		return nil, 0, err
	}
	// tenancyguard:ok derived read model (no deleted_at), ref.students with st.deleted_at IS NULL: explicit tenant_id and scope
	rows, err := q.query(ctx, `SELECT a.student_id, st.grade_level, a.reasons, a.computed_at, count(*) OVER ()
		FROM learning.rm_student_attention a
		JOIN ref.students st ON st.student_id = a.student_id AND st.tenant_id = a.tenant_id AND st.deleted_at IS NULL
		WHERE a.tenant_id = $1 AND ($2::uuid[] IS NULL OR a.school_id = ANY($2::uuid[])) AND ($3::int IS NULL OR st.grade_level = $3)
			AND cardinality(a.reasons) > 0
		ORDER BY cardinality(a.reasons) DESC, a.student_id LIMIT $4`, sc.TenantID, strIDs(sc.SchoolIDs), sc.GradeLevel, limit)
	if err != nil {
		return nil, 0, dashErr("attention", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.AttentionEntry
	total := 0
	for rows.Next() {
		var a read.AttentionEntry
		var grade sql.NullInt32
		if err := rows.Scan(&a.StudentID, &grade, &a.Reasons, &a.ComputedAt, &total); err != nil {
			return nil, 0, dashErr("attention", err)
		}
		if grade.Valid {
			g := int(grade.Int32)
			a.GradeLevel = &g
		}
		out = append(out, a)
	}
	return out, total, dashErr("attention", rows.Err())
}

// Assignments implements read.DashboardReader (query assignments: open assignments and those closed in the last 7 days,
// progress per grade of the students when targeted, else their current grade — lms.assignment_submissions, state read
// model, DASHBOARD_DATA §2).
func (DashboardModel) Assignments(ctx context.Context, sc read.DashScope, now time.Time, limit int) ([]read.AssignmentRow, error) {
	q, err := dtx(ctx, "assignments")
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok lms sources filtered on tenant and deleted_at IS NULL, ref.students projection: explicit tenant_id and scope
	rows, err := q.query(ctx, `WITH a AS (
			SELECT asg.assignment_id, asg.title, asg.subject_id, asg.status, asg.due_at FROM lms.assignments asg
			WHERE asg.tenant_id = $1 AND asg.deleted_at IS NULL AND ($2::uuid[] IS NULL OR asg.school_id = ANY($2::uuid[]))
				AND ($4::uuid[] IS NULL OR asg.subject_id = ANY($4::uuid[]))
				AND (asg.status = 'open' OR (asg.status IN ('closed', 'results_released') AND asg.due_at >= $5::timestamptz - interval '7 days'))
				AND EXISTS (SELECT 1 FROM lms.assignment_submissions s
					LEFT JOIN lms.assignment_target_students ts ON ts.assignment_id = s.assignment_id AND ts.student_id = s.student_id
						AND ts.tenant_id = s.tenant_id AND ts.deleted_at IS NULL
					LEFT JOIN ref.students st ON st.student_id = s.student_id AND st.tenant_id = s.tenant_id
					WHERE s.assignment_id = asg.assignment_id AND s.tenant_id = $1 AND s.deleted_at IS NULL
						AND ($3::int IS NULL OR COALESCE(ts.grade_level, st.grade_level, 0) = $3))
			ORDER BY asg.due_at, asg.assignment_id LIMIT $6),
		g AS (
			SELECT s.assignment_id, s.status, COALESCE(ts.grade_level, st.grade_level, 0)::int AS grade_level
			FROM a JOIN lms.assignment_submissions s ON s.assignment_id = a.assignment_id AND s.tenant_id = $1 AND s.deleted_at IS NULL
				AND s.status <> 'excused'
			LEFT JOIN lms.assignment_target_students ts ON ts.assignment_id = s.assignment_id AND ts.student_id = s.student_id
				AND ts.tenant_id = s.tenant_id AND ts.deleted_at IS NULL
			LEFT JOIN ref.students st ON st.student_id = s.student_id AND st.tenant_id = s.tenant_id)
		SELECT a.assignment_id, a.title, a.subject_id, a.status, a.due_at, g.grade_level,
			count(*) FILTER (WHERE g.status IN ('submitted', 'late')), count(*), count(*) FILTER (WHERE g.status = 'not_started'),
			count(*) FILTER (WHERE g.status = 'missed')
		FROM a JOIN g ON g.assignment_id = a.assignment_id
		WHERE ($3::int IS NULL OR g.grade_level = $3)
		GROUP BY a.assignment_id, a.title, a.subject_id, a.status, a.due_at, g.grade_level
		ORDER BY a.due_at, a.assignment_id, g.grade_level`, sc.TenantID, strIDs(sc.SchoolIDs), sc.GradeLevel, strIDs(sc.SubjectIDs), now, limit)
	if err != nil {
		return nil, dashErr("assignments", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.AssignmentRow
	for rows.Next() {
		var r read.AssignmentRow
		if err := rows.Scan(&r.AssignmentID, &r.Title, &r.SubjectID, &r.Status, &r.DueAt, &r.GradeLevel, &r.Submitted, &r.Total, &r.NotStarted,
			&r.Missed); err != nil {
			return nil, dashErr("assignments", err)
		}
		out = append(out, r)
	}
	return out, dashErr("assignments", rows.Err())
}

// SelfStudy implements read.DashboardReader (query self_study: rm_school_grade_summary of the visible grades).
func (DashboardModel) SelfStudy(ctx context.Context, sc read.DashScope) (read.SelfStudyFacts, error) {
	q, err := dtx(ctx, "self_study")
	if err != nil {
		return read.SelfStudyFacts{}, err
	}
	var f read.SelfStudyFacts
	var at sql.NullTime
	// tenancyguard:ok derived read model (no deleted_at): explicit tenant_id and scope
	err = q.row(ctx, `SELECT COALESCE(sum(s.student_count) FILTER (WHERE s.subject_id = $5), 0),
			COALESCE(sum(s.recommendation_started_num) FILTER (WHERE $4::uuid[] IS NULL OR s.subject_id = ANY($4::uuid[])), 0),
			COALESCE(sum(s.recommendation_started_den) FILTER (WHERE $4::uuid[] IS NULL OR s.subject_id = ANY($4::uuid[])), 0),
			COALESCE(sum(s.objectives_improved) FILTER (WHERE $4::uuid[] IS NULL OR s.subject_id = ANY($4::uuid[])), 0), min(s.computed_at)
		FROM learning.rm_school_grade_summary s
		WHERE s.tenant_id = $1 AND ($2::uuid[] IS NULL OR s.school_id = ANY($2::uuid[])) AND ($3::int IS NULL OR s.grade_level = $3)
			AND ($6::int[] IS NULL OR s.grade_level = ANY($6::int[]))`,
		sc.TenantID, strIDs(sc.SchoolIDs), sc.GradeLevel, strIDs(sc.SubjectIDs), uuid.Nil, grades(sc.Grades)).Scan(&f.Students, &f.RecNum, &f.RecDen,
		&f.Improved, &at)
	if at.Valid {
		t := at.Time.UTC()
		f.ComputedAtOldest = &t
	}
	return f, dashErr("self_study", err)
}

// GradeStudents implements read.DashboardReader (query grade_students: active, not deleted students of the grade of the
// school by student_id cursor, their progress_daily over the range and their attention reasons).
func (DashboardModel) GradeStudents(ctx context.Context, sc read.DashScope, rg analytics.Range, after *uuid.UUID, limit int) ([]read.StudentRow, error) {
	q, err := dtx(ctx, "grade_students")
	if err != nil {
		return nil, err
	}
	if sc.GradeLevel == nil || len(sc.SchoolIDs) == 0 {
		return nil, nil
	}
	// tenancyguard:ok ref.students with deleted_at IS NULL, derived progress_daily/rm (no deleted_at): explicit tenant_id and scope
	rows, err := q.query(ctx, `WITH r AS (
			SELECT st.student_id FROM ref.students st
			WHERE st.tenant_id = $1 AND st.school_id = ANY($2::uuid[]) AND st.grade_level = $3 AND st.deleted_at IS NULL
				AND st.status = 'active' AND ($4::uuid IS NULL OR st.student_id > $4)
			ORDER BY st.student_id LIMIT $5),
		p AS (SELECT p.student_id, sum(p.activities_completed) AS acts, sum(p.lessons_completed) AS less,
				sum(LEAST(GREATEST(p.items_correct - p.items_correct_rapid_guess, 0), GREATEST(p.items_answered - p.items_rapid_guess, 0))) AS an,
				sum(GREATEST(p.items_answered - p.items_rapid_guess, 0)) AS ad, sum(p.items_rapid_guess) AS rn, sum(p.items_answered) AS rd,
				sum(p.active_seconds) AS sec
			FROM learning.progress_daily p JOIN r ON r.student_id = p.student_id
			WHERE p.tenant_id = $1 AND p.bucket_date BETWEEN $6::date AND $7::date AND p.metric_version = 1
				AND ($8::uuid[] IS NULL OR p.subject_id = ANY($8::uuid[]))
			GROUP BY p.student_id)
		SELECT r.student_id, COALESCE(p.acts, 0), COALESCE(p.less, 0), COALESCE(p.an, 0), COALESCE(p.ad, 0), COALESCE(p.rn, 0),
			COALESCE(p.rd, 0), COALESCE(p.sec, 0), COALESCE(a.reasons, '{}')
		FROM r LEFT JOIN p ON p.student_id = r.student_id
		LEFT JOIN learning.rm_student_attention a ON a.tenant_id = $1 AND a.student_id = r.student_id
		ORDER BY r.student_id`, sc.TenantID, strIDs(sc.SchoolIDs), *sc.GradeLevel, after, limit, day(rg.From), day(rg.To), strIDs(sc.SubjectIDs))
	if err != nil {
		return nil, dashErr("grade_students", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.StudentRow
	for rows.Next() {
		var s read.StudentRow
		if err := rows.Scan(&s.StudentID, &s.Activities, &s.Lessons, &s.AccNum, &s.AccDen, &s.RapidNum, &s.RapidDen, &s.ActS, &s.Reasons); err != nil {
			return nil, dashErr("grade_students", err)
		}
		out = append(out, s)
	}
	return out, dashErr("grade_students", rows.Err())
}

// ProfileSubjects implements read.DashboardReader (query profile_subjects: progress_daily of one student).
func (DashboardModel) ProfileSubjects(ctx context.Context, sc read.DashScope, studentID uuid.UUID, rg analytics.Range) ([]read.SubjectTotals, read.RollupMeta, error) {
	q, err := dtx(ctx, "profile_subjects")
	if err != nil {
		return nil, read.RollupMeta{}, err
	}
	// tenancyguard:ok derived progress_daily (no deleted_at): explicit tenant_id, student, scope and bounded range
	rows, err := q.query(ctx, `SELECT p.subject_id, sum(p.activities_completed), sum(p.lessons_completed),
			sum(LEAST(GREATEST(p.items_correct - p.items_correct_rapid_guess, 0), GREATEST(p.items_answered - p.items_rapid_guess, 0))),
			sum(GREATEST(p.items_answered - p.items_rapid_guess, 0)), sum(p.items_rapid_guess), sum(p.items_answered), sum(p.active_seconds),
			min(p.computed_at), bool_or(NOT p.is_final)
		FROM learning.progress_daily p
		WHERE p.tenant_id = $1 AND p.student_id = $2 AND ($3::uuid[] IS NULL OR p.school_id = ANY($3::uuid[]))
			AND ($4::uuid[] IS NULL OR p.subject_id = ANY($4::uuid[])) AND p.bucket_date BETWEEN $5::date AND $6::date AND p.metric_version = 1
		GROUP BY p.subject_id ORDER BY p.subject_id LIMIT 50`,
		sc.TenantID, studentID, strIDs(sc.SchoolIDs), strIDs(sc.SubjectIDs), day(rg.From), day(rg.To))
	if err != nil {
		return nil, read.RollupMeta{}, dashErr("profile_subjects", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.SubjectTotals
	var a metaAcc
	for rows.Next() {
		var s read.SubjectTotals
		var at sql.NullTime
		var partial bool
		if err := rows.Scan(&s.SubjectID, &s.Activities, &s.Lessons, &s.AccNum, &s.AccDen, &s.RapidNum, &s.RapidDen, &s.ActS, &at, &partial); err != nil {
			return nil, a.m, dashErr("profile_subjects", err)
		}
		a.add(at, partial)
		out = append(out, s)
	}
	return out, a.m, dashErr("profile_subjects", rows.Err())
}

// ProfileTopics implements read.DashboardReader (query profile_topics: objective_masteries — state read model — by topic).
func (DashboardModel) ProfileTopics(ctx context.Context, sc read.DashScope, studentID uuid.UUID) ([]read.TopicLevels, error) {
	q, err := dtx(ctx, "profile_topics")
	if err != nil {
		return nil, err
	}
	rows, err := q.query(ctx, `SELECT t.subject_id, t.topic_id, max(t.name),
			ARRAY[count(*) FILTER (WHERE om.level = 'not_started'), count(*) FILTER (WHERE om.level = 'beginning'),
				count(*) FILTER (WHERE om.level = 'developing'), count(*) FILTER (WHERE om.level = 'proficient'),
				count(*) FILTER (WHERE om.level = 'mastered')]
		FROM learning.objective_masteries om
		JOIN curriculum.learning_objectives lo ON lo.objective_id = om.objective_id AND lo.deleted_at IS NULL
		JOIN curriculum.topics t ON t.topic_id = lo.topic_id AND t.deleted_at IS NULL
		WHERE om.tenant_id = $1 AND om.student_id = $2 AND om.deleted_at IS NULL AND ($3::uuid[] IS NULL OR t.subject_id = ANY($3::uuid[]))
		GROUP BY t.subject_id, t.topic_id ORDER BY t.subject_id, max(t.sort_order), t.topic_id LIMIT 500`,
		sc.TenantID, studentID, strIDs(sc.SubjectIDs))
	if err != nil {
		return nil, dashErr("profile_topics", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.TopicLevels
	for rows.Next() {
		var t read.TopicLevels
		var dist []int64
		if err := rows.Scan(&t.SubjectID, &t.TopicID, &t.TopicName, &dist); err != nil {
			return nil, dashErr("profile_topics", err)
		}
		t.LevelDist = ints(dist)
		out = append(out, t)
	}
	return out, dashErr("profile_topics", rows.Err())
}

// ProfileTrend implements read.DashboardReader (query profile_trend: mastery_snapshots_weekly).
func (DashboardModel) ProfileTrend(ctx context.Context, sc read.DashScope, studentID uuid.UUID, from, to time.Time) ([]read.TrendRow, error) {
	q, err := dtx(ctx, "profile_trend")
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok derived snapshot (no deleted_at): explicit tenant_id, student, scope and bounded weeks
	rows, err := q.query(ctx, `SELECT w.bucket_date, w.subject_id, w.mean_mastery::float8, w.mastered_count
		FROM learning.mastery_snapshots_weekly w
		WHERE w.tenant_id = $1 AND w.student_id = $2 AND ($3::uuid[] IS NULL OR w.subject_id = ANY($3::uuid[]))
			AND w.bucket_date BETWEEN $4::date AND $5::date
		ORDER BY w.bucket_date, w.subject_id LIMIT 400`, sc.TenantID, studentID, strIDs(sc.SubjectIDs), day(from), day(to))
	if err != nil {
		return nil, dashErr("profile_trend", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.TrendRow
	for rows.Next() {
		var t read.TrendRow
		if err := rows.Scan(&t.Week, &t.SubjectID, &t.MeanMastery, &t.MasteredCount); err != nil {
			return nil, dashErr("profile_trend", err)
		}
		t.Week = analytics.DateOf(t.Week)
		out = append(out, t)
	}
	return out, dashErr("profile_trend", rows.Err())
}

// ProfileAssignments implements read.DashboardReader (query profile_assignments: submissions, no answer).
func (DashboardModel) ProfileAssignments(ctx context.Context, sc read.DashScope, studentID uuid.UUID, limit int) ([]read.AssignmentHistory, error) {
	q, err := dtx(ctx, "profile_assignments")
	if err != nil {
		return nil, err
	}
	rows, err := q.query(ctx, `SELECT a.assignment_id, a.subject_id, a.title, s.status, s.score::float8, s.max_score::float8,
			COALESCE(s.due_at_override, a.due_at), s.submitted_at
		FROM lms.assignment_submissions s
		JOIN lms.assignments a ON a.assignment_id = s.assignment_id AND a.tenant_id = s.tenant_id AND a.deleted_at IS NULL
		WHERE s.tenant_id = $1 AND s.student_id = $2 AND s.deleted_at IS NULL AND a.status NOT IN ('draft', 'cancelled')
			AND ($3::uuid[] IS NULL OR a.subject_id = ANY($3::uuid[]))
		ORDER BY COALESCE(s.due_at_override, a.due_at) DESC, a.assignment_id LIMIT $4`, sc.TenantID, studentID, strIDs(sc.SubjectIDs), limit)
	if err != nil {
		return nil, dashErr("profile_assignments", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.AssignmentHistory
	for rows.Next() {
		var h read.AssignmentHistory
		var score, maxScore sql.NullFloat64
		var submitted sql.NullTime
		if err := rows.Scan(&h.AssignmentID, &h.SubjectID, &h.Title, &h.Status, &score, &maxScore, &h.DueAt, &submitted); err != nil {
			return nil, dashErr("profile_assignments", err)
		}
		if score.Valid {
			h.Score = &score.Float64
		}
		if maxScore.Valid {
			h.MaxScore = &maxScore.Float64
		}
		if submitted.Valid {
			t := submitted.Time.UTC()
			h.SubmittedAt = &t
		}
		out = append(out, h)
	}
	return out, dashErr("profile_assignments", rows.Err())
}

// ProfileRecommendations implements read.DashboardReader (query profile_recommendations: rm_student_recommendations shown).
func (DashboardModel) ProfileRecommendations(ctx context.Context, sc read.DashScope, studentID uuid.UUID, limit int) ([]read.RecommendationRow, error) {
	q, err := dtx(ctx, "profile_recommendations")
	if err != nil {
		return nil, err
	}
	rows, err := q.query(ctx, `SELECT r.recommendation_id, r.rule_code, r.reason_code, r.subject_id
		FROM learning.rm_student_recommendations r
		WHERE r.tenant_id = $1 AND r.student_id = $2 AND r.deleted_at IS NULL AND r.status = 'active' AND r.expires_at > now()
			AND ($3::uuid[] IS NULL OR r.subject_id IS NULL OR r.subject_id = ANY($3::uuid[]))
		ORDER BY r.priority, r.generated_at DESC LIMIT $4`, sc.TenantID, studentID, strIDs(sc.SubjectIDs), limit)
	if err != nil {
		return nil, dashErr("profile_recommendations", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.RecommendationRow
	for rows.Next() {
		var r read.RecommendationRow
		var subject uuid.NullUUID
		if err := rows.Scan(&r.RecommendationID, &r.RuleCode, &r.ReasonCode, &subject); err != nil {
			return nil, dashErr("profile_recommendations", err)
		}
		if subject.Valid {
			id := subject.UUID
			r.SubjectID = &id
		}
		out = append(out, r)
	}
	return out, dashErr("profile_recommendations", rows.Err())
}

const cellCols = `sum(r.active_students), sum(r.activities_completed), sum(r.accuracy_num), sum(r.accuracy_den), sum(r.active_seconds),
	sum(r.assignments_due), sum(r.assignments_completed), min(r.computed_at), bool_or(NOT r.is_final)`

func scanCells(rows *sql.Rows) ([]read.SummaryCell, read.RollupMeta, error) {
	var out []read.SummaryCell
	var a metaAcc
	for rows.Next() {
		var c read.SummaryCell
		var school uuid.NullUUID
		var act, acts, an, ad, sec, due, done sql.NullInt64
		var at sql.NullTime
		var partial sql.NullBool
		if err := rows.Scan(&c.TenantID, &school, &c.GradeLevel, &c.SubjectID, &c.Students, &act, &acts, &an, &ad, &sec, &due, &done, &at,
			&partial); err != nil {
			return nil, a.m, dashErr("cells", err)
		}
		if school.Valid {
			id := school.UUID
			c.SchoolID = &id
		}
		c.ActiveSum, c.Activities, c.AccNum, c.AccDen, c.ActS, c.AssignDue, c.AssignCompleted = act.Int64, acts.Int64, an.Int64, ad.Int64,
			sec.Int64, due.Int64, done.Int64
		a.add(at, partial.Bool)
		out = append(out, c)
	}
	return out, a.m, dashErr("cells", rows.Err())
}

// SchoolCells implements read.DashboardReader (query school_cells: rollup_school_* per school × grade × subject of one tenant,
// students = enrolled students of the school × grade from rm_school_grade_summary).
func (DashboardModel) SchoolCells(ctx context.Context, sc read.DashScope, rg analytics.Range) ([]read.SummaryCell, read.RollupMeta, error) {
	q, err := dtx(ctx, "school_cells")
	if err != nil {
		return nil, read.RollupMeta{}, err
	}
	// tenancyguard:ok derived rollup and read model (no deleted_at): explicit tenant_id, scope and bounded bucket range
	rows, err := q.query(ctx, fmt.Sprintf(`WITH n AS (SELECT s.school_id, s.grade_level, max(s.student_count) AS n
			FROM learning.rm_school_grade_summary s WHERE s.tenant_id = $1 AND ($2::uuid[] IS NULL OR s.school_id = ANY($2::uuid[]))
			GROUP BY s.school_id, s.grade_level)
		SELECT r.tenant_id, r.school_id, r.grade_level, r.subject_id, COALESCE(max(n.n), 0), `+cellCols+`
		FROM learning.rollup_school_%s r LEFT JOIN n ON n.school_id = r.school_id AND n.grade_level = r.grade_level
		WHERE r.tenant_id = $1 AND ($2::uuid[] IS NULL OR r.school_id = ANY($2::uuid[])) AND ($3::int IS NULL OR r.grade_level = $3)
			AND r.bucket_date BETWEEN $4::date AND $5::date AND r.metric_version = 1
		GROUP BY r.tenant_id, r.school_id, r.grade_level, r.subject_id
		ORDER BY r.school_id, r.grade_level, r.subject_id LIMIT 5000`, suffix(rg)),
		sc.TenantID, strIDs(sc.SchoolIDs), sc.GradeLevel, day(rg.QueryFrom()), day(rg.To))
	if err != nil {
		return nil, read.RollupMeta{}, dashErr("school_cells", err)
	}
	defer func() { _ = rows.Close() }()
	return scanCells(rows)
}

// TenantCells implements read.DashboardReader (query tenant_cells: rollup_tenant_* per tenant × grade × subject, explicit
// tenant list of the CrossTenantScope; students summed over the schools of the tenant × grade).
func (DashboardModel) TenantCells(ctx context.Context, tenantIDs []uuid.UUID, grade *int, rg analytics.Range) ([]read.SummaryCell, read.RollupMeta, error) {
	q, err := dtx(ctx, "tenant_cells")
	if err != nil {
		return nil, read.RollupMeta{}, err
	}
	// tenancyguard:ok derived rollup and read model (no deleted_at): tenant_id = ANY(tenants of the CrossTenantScope), bounded range
	rows, err := q.query(ctx, fmt.Sprintf(`WITH n AS (SELECT x.tenant_id, x.grade_level, sum(x.n) AS n FROM (
				SELECT s.tenant_id, s.school_id, s.grade_level, max(s.student_count) AS n FROM learning.rm_school_grade_summary s
				WHERE s.tenant_id = ANY($1::uuid[]) GROUP BY s.tenant_id, s.school_id, s.grade_level) x
			GROUP BY x.tenant_id, x.grade_level)
		SELECT r.tenant_id, NULL::uuid, r.grade_level, r.subject_id, COALESCE(max(n.n), 0)::int, `+cellCols+`
		FROM learning.rollup_tenant_%s r LEFT JOIN n ON n.tenant_id = r.tenant_id AND n.grade_level = r.grade_level
		WHERE r.tenant_id = ANY($1::uuid[]) AND ($2::int IS NULL OR r.grade_level = $2) AND r.bucket_date BETWEEN $3::date AND $4::date
			AND r.metric_version = 1
		GROUP BY r.tenant_id, r.grade_level, r.subject_id
		ORDER BY r.tenant_id, r.grade_level, r.subject_id LIMIT 5000`, suffix(rg)),
		strIDs(tenantIDs), grade, day(rg.QueryFrom()), day(rg.To))
	if err != nil {
		return nil, read.RollupMeta{}, dashErr("tenant_cells", err)
	}
	defer func() { _ = rows.Close() }()
	return scanCells(rows)
}

// Tenants implements read.DashboardReader.
func (DashboardModel) Tenants(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, []uuid.UUID, error) {
	q, err := dtx(ctx, "tenants")
	if err != nil {
		return nil, nil, err
	}
	// tenancyguard:ok ref.tenants is the tenant list (no deleted_at), read in the System Admin cross-tenant/platform scope
	rows, err := q.query(ctx, `SELECT tenant_id, is_qa FROM ref.tenants
		WHERE status = 'active' AND ($1::uuid[] IS NULL OR tenant_id = ANY($1::uuid[])) ORDER BY tenant_id`, strIDs(ids))
	if err != nil {
		return nil, nil, dashErr("tenants", err)
	}
	defer func() { _ = rows.Close() }()
	var active, qa []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		var isQA bool
		if err := rows.Scan(&id, &isQA); err != nil {
			return nil, nil, dashErr("tenants", err)
		}
		active = append(active, id)
		if isQA {
			qa = append(qa, id)
		}
	}
	return active, qa, dashErr("tenants", rows.Err())
}
