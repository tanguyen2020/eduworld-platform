package write

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/analytics"
)

// RollupRepo implements write.RollupStore and write.RollupJobs on PostgreSQL (F043 US1, DASHBOARD_DATA §4–§5). Every
// statement runs in the TenantScope of ctx: tenant_id = infra.current_tenant_id() plus RLS. The rollup and read-model
// tables are derived (no deleted_at, DASHBOARD_DATA §3.1); their sources are filtered on deleted_at IS NULL.
type RollupRepo struct{}

var (
	_ write.RollupStore = RollupRepo{}
	_ write.RollupJobs  = RollupRepo{}
)

// dashZone is the calendar of the assignment due dates in SQL (analytics.Zone: UTC+7 without daylight saving).
const dashZone = "Asia/Ho_Chi_Minh"

// nilSubject is the subject of an activity without subject (progress_daily) and of the "all subjects" rows.
var nilSubject = uuid.Nil

func dateParam(t time.Time) string { return t.Format(time.DateOnly) }

// DayFinal implements write.RollupStore.
func (RollupRepo) DayFinal(ctx context.Context, day time.Time) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	var final bool
	// tenancyguard:ok derived rollup_days has no deleted_at; tenant of the session (RLS + infra.current_tenant_id())
	err = tx.QueryRowContext(ctx, `SELECT is_final FROM learning.rollup_days WHERE tenant_id = infra.current_tenant_id() AND bucket_date = $1::date`,
		dateParam(day)).Scan(&final)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("rollup day state: %w", err)
	}
	return final, nil
}

// MarkReclose implements write.RollupStore.
func (RollupRepo) MarkReclose(ctx context.Context, day time.Time) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok derived rollup_days has no deleted_at; tenant of the session (RLS + infra.current_tenant_id())
	if _, err := tx.ExecContext(ctx, `UPDATE learning.rollup_days SET needs_reclose = true, computed_at = now()
		WHERE tenant_id = infra.current_tenant_id() AND bucket_date = $1::date`, dateParam(day)); err != nil {
		return fmt.Errorf("mark reclose: %w", err)
	}
	return nil
}

// AddRollups implements write.RollupStore: the grade of the event is the grade_level stored on its progress_daily row
// (AddDaily ran first in the same transaction: the grade of the student when the bucket was first written, ADR-0095),
// else the current grade of the student, else 0.
func (RollupRepo) AddRollups(ctx context.Context, d write.ProgressDelta) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	day := dateParam(d.Day)
	var grade int
	// tenancyguard:ok progress_daily is derived (no deleted_at); ref projection of the session tenant (RLS + explicit tenant_id)
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(p.grade_level,
			(SELECT st.grade_level FROM ref.students st WHERE st.student_id = p.student_id AND st.tenant_id = p.tenant_id), 0)
		FROM learning.progress_daily p
		WHERE p.tenant_id = infra.current_tenant_id() AND p.student_id = $1 AND p.subject_id = $2 AND p.bucket_date = $3::date
			AND p.metric_version = 1`, d.StudentID, d.SubjectID, day).Scan(&grade)
	if err != nil {
		return fmt.Errorf("rollup grade snapshot: %w", err)
	}
	accNum, accDen := analytics.Accuracy(int64(d.Items.Answered), int64(d.Items.Correct), int64(d.Items.RapidGuess), int64(d.Items.CorrectRapid))
	vals := []any{d.ActivitiesCompleted, d.LessonsCompleted, accNum, accDen, d.Items.RapidGuess, d.Items.Answered, d.ActiveSeconds}
	const sums = `activities_completed = r.activities_completed + EXCLUDED.activities_completed,
			lessons_completed = r.lessons_completed + EXCLUDED.lessons_completed,
			accuracy_num = r.accuracy_num + EXCLUDED.accuracy_num, accuracy_den = r.accuracy_den + EXCLUDED.accuracy_den,
			rapid_guess_num = r.rapid_guess_num + EXCLUDED.rapid_guess_num, rapid_guess_den = r.rapid_guess_den + EXCLUDED.rapid_guess_den,
			active_seconds = r.active_seconds + EXCLUDED.active_seconds, computed_at = now()`
	// tenancyguard:ok derived rollup (no deleted_at); insert/update of the session tenant (RLS)
	if _, err := tx.ExecContext(ctx, `INSERT INTO learning.rollup_school_daily AS r (tenant_id, school_id, grade_level, subject_id, bucket_date,
			metric_version, activities_completed, lessons_completed, accuracy_num, accuracy_den, rapid_guess_num, rapid_guess_den, active_seconds)
		VALUES (infra.current_tenant_id(), $1, $2, $3, $4::date, 1, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (tenant_id, school_id, grade_level, subject_id, bucket_date, metric_version) DO UPDATE SET `+sums,
		append([]any{d.SchoolID, grade, d.SubjectID, day}, vals...)...); err != nil {
		return fmt.Errorf("rollup school daily: %w", err)
	}
	// tenancyguard:ok derived rollup (no deleted_at); insert/update of the session tenant (RLS)
	if _, err := tx.ExecContext(ctx, `INSERT INTO learning.rollup_tenant_daily AS r (tenant_id, grade_level, subject_id, bucket_date,
			metric_version, activities_completed, lessons_completed, accuracy_num, accuracy_den, rapid_guess_num, rapid_guess_den, active_seconds)
		VALUES (infra.current_tenant_id(), $1, $2, $3::date, 1, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (tenant_id, grade_level, subject_id, bucket_date, metric_version) DO UPDATE SET `+sums,
		append([]any{grade, d.SubjectID, day}, vals...)...); err != nil {
		return fmt.Errorf("rollup tenant daily: %w", err)
	}
	return nil
}

// Tenants implements write.RollupJobs.
func (RollupRepo) Tenants(ctx context.Context) ([]uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok ref.tenants is the tenant list itself (no tenant column filter, no deleted_at), platform job bypass
	rows, err := tx.QueryContext(ctx, `SELECT tenant_id FROM ref.tenants WHERE status = 'active' ORDER BY tenant_id`)
	if err != nil {
		return nil, fmt.Errorf("rollup tenants: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// DaysToClose implements write.RollupJobs.
func (RollupRepo) DaysToClose(ctx context.Context, last time.Time, limit int) ([]time.Time, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok derived tables (no deleted_at); tenant of the session (RLS + infra.current_tenant_id())
	rows, err := tx.QueryContext(ctx, `SELECT d FROM (
			SELECT DISTINCT bucket_date AS d FROM learning.progress_daily
				WHERE tenant_id = infra.current_tenant_id() AND bucket_date <= $1::date AND NOT is_final
			UNION SELECT DISTINCT bucket_date FROM learning.rollup_school_daily
				WHERE tenant_id = infra.current_tenant_id() AND bucket_date <= $1::date AND NOT is_final
			UNION SELECT bucket_date FROM learning.rollup_days
				WHERE tenant_id = infra.current_tenant_id() AND bucket_date <= $1::date AND needs_reclose) x
		ORDER BY d LIMIT $2`, dateParam(last), limit)
	if err != nil {
		return nil, fmt.Errorf("days to close: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []time.Time
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out = append(out, analytics.DateOf(d))
	}
	return out, rows.Err()
}

// MarkClosed implements write.RollupJobs.
func (RollupRepo) MarkClosed(ctx context.Context, day, at time.Time) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok derived rollup_days has no deleted_at; insert/update of the session tenant (RLS)
	if _, err := tx.ExecContext(ctx, `INSERT INTO learning.rollup_days (tenant_id, bucket_date, is_final, needs_reclose, closed_at, computed_at)
		VALUES (infra.current_tenant_id(), $1::date, true, false, $2, now())
		ON CONFLICT (tenant_id, bucket_date) DO UPDATE SET is_final = true, needs_reclose = false, closed_at = EXCLUDED.closed_at,
			computed_at = now()`, dateParam(day), at); err != nil {
		return fmt.Errorf("mark closed: %w", err)
	}
	// tenancyguard:ok derived progress_daily has no deleted_at; update of the session tenant (RLS)
	if _, err := tx.ExecContext(ctx, `UPDATE learning.progress_daily SET is_final = true
		WHERE tenant_id = infra.current_tenant_id() AND bucket_date = $1::date AND NOT is_final`, dateParam(day)); err != nil {
		return fmt.Errorf("progress daily final: %w", err)
	}
	return nil
}

// stmt is one statement of a job step with its own arguments.
type stmt struct {
	q    string
	args []any
}

func execAll(ctx context.Context, tx *sql.Tx, what string, ss []stmt) error {
	for i, s := range ss {
		if _, err := tx.ExecContext(ctx, s.q, s.args...); err != nil {
			return fmt.Errorf("%s (%d): %w", what, i, err)
		}
	}
	return nil
}

var periodSuffixes = map[string]bool{"daily": true, "weekly": true, "monthly": true}

// counters of a progress row set (alias s): the accuracy pair excludes the rapid guesses (F043-AC-05).
const progressSums = `sum(s.activities_completed), sum(s.lessons_completed),
	sum(LEAST(GREATEST(s.items_correct - s.items_correct_rapid_guess, 0), GREATEST(s.items_answered - s.items_rapid_guess, 0))),
	sum(GREATEST(s.items_answered - s.items_rapid_guess, 0)), sum(s.items_rapid_guess), sum(s.items_answered), sum(s.active_seconds)`

const progressSet = `active_students = EXCLUDED.active_students, active_students_all = EXCLUDED.active_students_all,
	activities_completed = EXCLUDED.activities_completed, lessons_completed = EXCLUDED.lessons_completed,
	accuracy_num = EXCLUDED.accuracy_num, accuracy_den = EXCLUDED.accuracy_den, rapid_guess_num = EXCLUDED.rapid_guess_num,
	rapid_guess_den = EXCLUDED.rapid_guess_den, active_seconds = EXCLUDED.active_seconds, is_final = EXCLUDED.is_final, computed_at = now()`

const assignmentSet = `assignments_due = EXCLUDED.assignments_due, assignments_completed = EXCLUDED.assignments_completed,
	assignments_on_time = EXCLUDED.assignments_on_time, computed_at = now()`

const resetCounters = `activities_completed = 0, lessons_completed = 0, accuracy_num = 0, accuracy_den = 0, rapid_guess_num = 0,
	rapid_guess_den = 0, active_seconds = 0, assignments_due = 0, assignments_completed = 0, assignments_on_time = 0,
	active_students = NULL, active_students_all = NULL, computed_at = now()`

// progressRows is the progress_daily rows of [$2, $3] with the grade recorded on the row (grade at event time, ADR-0095),
// else the current grade of the student, else 0.
// tenancyguard:ok derived progress_daily (no deleted_at); ref projection of the session tenant (RLS + explicit tenant_id)
const progressRows = `WITH s AS (
	SELECT p.school_id, COALESCE(p.grade_level, st.grade_level, 0)::smallint AS grade_level, p.subject_id, p.student_id,
		p.activities_completed, p.lessons_completed, p.items_answered, p.items_correct, p.items_rapid_guess,
		p.items_correct_rapid_guess, p.active_seconds
	FROM learning.progress_daily p
	LEFT JOIN ref.students st ON st.student_id = p.student_id AND st.tenant_id = p.tenant_id
	WHERE p.tenant_id = infra.current_tenant_id() AND p.bucket_date BETWEEN $2::date AND $3::date AND p.metric_version = 1)`

// submissionGrade is the grade of an assignment submission s: the grade of the student when targeted (WP-L1), else the
// current grade of the student, else 0. Joins lms.assignment_target_students ts and ref.students st (aliases fixed).
const submissionGrade = `LEFT JOIN lms.assignment_target_students ts ON ts.assignment_id = s.assignment_id AND ts.student_id = s.student_id
		AND ts.tenant_id = s.tenant_id AND ts.deleted_at IS NULL
	LEFT JOIN ref.students st ON st.student_id = s.student_id AND st.tenant_id = s.tenant_id`

// Recompute implements write.RollupJobs: reset of the bucket rows, counters and exact distinct counts from progress_daily
// of [From, To] by school × grade × subject, assignment progress (daily: from the submissions due that day by grade of the
// student when targeted; weekly/monthly: sum of the school days), the tenant rows from the school rows, then is_final.
func (RollupRepo) Recompute(ctx context.Context, p write.Period) error {
	if !periodSuffixes[p.Suffix] {
		return fmt.Errorf("rollup period %q", p.Suffix)
	}
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	b, window, final := dateParam(p.Bucket), []any{dateParam(p.Bucket), dateParam(p.From), dateParam(p.To), p.Final}, p.Final
	ss := []stmt{
		// tenancyguard:ok derived rollup (no deleted_at); update of the session tenant (RLS)
		{fmt.Sprintf(`UPDATE learning.rollup_school_%s SET %s WHERE tenant_id = infra.current_tenant_id()
			AND bucket_date = $1::date AND metric_version = 1`, p.Suffix, resetCounters), []any{b}},
		// tenancyguard:ok derived rollup (no deleted_at); update of the session tenant (RLS)
		{fmt.Sprintf(`UPDATE learning.rollup_tenant_%s SET %s, active_schools = NULL
			WHERE tenant_id = infra.current_tenant_id() AND bucket_date = $1::date AND metric_version = 1`, p.Suffix, resetCounters), []any{b}},
		{fmt.Sprintf(progressRows+`, a AS (SELECT school_id, grade_level, count(DISTINCT student_id) AS n FROM s GROUP BY school_id, grade_level)
			INSERT INTO learning.rollup_school_%s AS r (tenant_id, school_id, grade_level, subject_id, bucket_date, metric_version,
				active_students, active_students_all, activities_completed, lessons_completed, accuracy_num, accuracy_den,
				rapid_guess_num, rapid_guess_den, active_seconds, is_final, computed_at)
			SELECT infra.current_tenant_id(), s.school_id, s.grade_level, s.subject_id, $1::date, 1, count(DISTINCT s.student_id), max(a.n),
				%s, $4, now()
			FROM s JOIN a ON a.school_id = s.school_id AND a.grade_level = s.grade_level
			GROUP BY s.school_id, s.grade_level, s.subject_id
			ON CONFLICT (tenant_id, school_id, grade_level, subject_id, bucket_date, metric_version) DO UPDATE SET %s`,
			p.Suffix, progressSums, progressSet), window},
		{fmt.Sprintf(progressRows+`, a AS (SELECT grade_level, count(DISTINCT student_id) AS n FROM s GROUP BY grade_level)
			INSERT INTO learning.rollup_tenant_%s AS r (tenant_id, grade_level, subject_id, bucket_date, metric_version, active_students,
				active_students_all, active_schools, activities_completed, lessons_completed, accuracy_num, accuracy_den,
				rapid_guess_num, rapid_guess_den, active_seconds, is_final, computed_at)
			SELECT infra.current_tenant_id(), s.grade_level, s.subject_id, $1::date, 1, count(DISTINCT s.student_id), max(a.n),
				count(DISTINCT s.school_id), %s, $4, now()
			FROM s JOIN a ON a.grade_level = s.grade_level
			GROUP BY s.grade_level, s.subject_id
			ON CONFLICT (tenant_id, grade_level, subject_id, bucket_date, metric_version) DO UPDATE SET
				active_schools = EXCLUDED.active_schools, %s`, p.Suffix, progressSums, progressSet), window},
	}
	if p.Suffix == "daily" {
		ss = append(ss, stmt{
			// tenancyguard:ok lms sources filtered on tenant and deleted_at IS NULL, ref.students projection; derived rollup of the session tenant (RLS)
			fmt.Sprintf(`WITH a AS (
				SELECT s.school_id, COALESCE(ts.grade_level, st.grade_level, 0)::smallint AS grade_level, asg.subject_id, count(*) AS due,
					count(*) FILTER (WHERE s.status IN ('submitted', 'late')) AS completed, count(*) FILTER (WHERE s.status = 'submitted') AS on_time
				FROM lms.assignment_submissions s
				JOIN lms.assignments asg ON asg.assignment_id = s.assignment_id AND asg.tenant_id = s.tenant_id AND asg.deleted_at IS NULL
				%s
				WHERE s.tenant_id = infra.current_tenant_id() AND s.deleted_at IS NULL AND s.status <> 'excused'
					AND asg.status IN ('open', 'closed', 'results_released', 'archived')
					AND (COALESCE(s.due_at_override, asg.due_at) AT TIME ZONE '%s')::date = $1::date
				GROUP BY 1, 2, 3)
			INSERT INTO learning.rollup_school_daily AS r (tenant_id, school_id, grade_level, subject_id, bucket_date, metric_version,
				assignments_due, assignments_completed, assignments_on_time, is_final, computed_at)
			SELECT infra.current_tenant_id(), a.school_id, a.grade_level, a.subject_id, $1::date, 1, a.due, a.completed, a.on_time, $2, now()
			FROM a
			ON CONFLICT (tenant_id, school_id, grade_level, subject_id, bucket_date, metric_version) DO UPDATE SET %s`,
				submissionGrade, dashZone, assignmentSet),
			[]any{b, final}})
	} else {
		ss = append(ss, stmt{
			// tenancyguard:ok derived rollups (no deleted_at) of the session tenant (RLS + infra.current_tenant_id())
			fmt.Sprintf(`INSERT INTO learning.rollup_school_%s AS r (tenant_id, school_id, grade_level, subject_id, bucket_date,
				metric_version, assignments_due, assignments_completed, assignments_on_time, is_final, computed_at)
			SELECT tenant_id, school_id, grade_level, subject_id, $1::date, 1, sum(assignments_due), sum(assignments_completed),
				sum(assignments_on_time), $4, now()
			FROM learning.rollup_school_daily
			WHERE tenant_id = infra.current_tenant_id() AND bucket_date BETWEEN $2::date AND $3::date AND metric_version = 1
			GROUP BY tenant_id, school_id, grade_level, subject_id HAVING sum(assignments_due) > 0
			ON CONFLICT (tenant_id, school_id, grade_level, subject_id, bucket_date, metric_version) DO UPDATE SET %s`, p.Suffix, assignmentSet),
			window})
	}
	ss = append(ss,
		stmt{
			// tenancyguard:ok derived rollups (no deleted_at) of the session tenant (RLS + infra.current_tenant_id())
			fmt.Sprintf(`INSERT INTO learning.rollup_tenant_%[1]s AS r (tenant_id, grade_level, subject_id, bucket_date, metric_version,
				assignments_due, assignments_completed, assignments_on_time, is_final, computed_at)
			SELECT tenant_id, grade_level, subject_id, $1::date, 1, sum(assignments_due), sum(assignments_completed),
				sum(assignments_on_time), $2, now()
			FROM learning.rollup_school_%[1]s
			WHERE tenant_id = infra.current_tenant_id() AND bucket_date = $1::date AND metric_version = 1
			GROUP BY tenant_id, grade_level, subject_id HAVING sum(assignments_due) > 0
			ON CONFLICT (tenant_id, grade_level, subject_id, bucket_date, metric_version) DO UPDATE SET %[2]s`, p.Suffix, assignmentSet),
			[]any{b, final}})
	for _, lvl := range []string{"school", "tenant"} {
		ss = append(ss, stmt{
			// tenancyguard:ok derived rollups (no deleted_at); update of the session tenant (RLS)
			fmt.Sprintf(`UPDATE learning.rollup_%s_%s SET is_final = $2 WHERE tenant_id = infra.current_tenant_id()
			AND bucket_date = $1::date AND metric_version = 1`, lvl, p.Suffix), []any{b, final}})
	}
	return execAll(ctx, tx, "recompute "+p.Suffix+" "+b, ss)
}

// ObjectiveWeek implements write.RollupJobs: school × grade × objective of the ISO week from the scored first answers, by
// the grade recorded on the learning event (grade at event time, ADR-0095).
func (RollupRepo) ObjectiveWeek(ctx context.Context, start, from, to time.Time, final, withLevels bool) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	b := dateParam(start)
	ss := []stmt{
		// tenancyguard:ok derived rollup (no deleted_at); update of the session tenant (RLS)
		{`UPDATE learning.rollup_grade_objective_weekly SET evidence_count = 0, correct_num = 0, correct_den = 0, computed_at = now()
			WHERE tenant_id = infra.current_tenant_id() AND bucket_date = $1::date AND metric_version = 1`, []any{b}},
		// tenancyguard:ok append-only learning_events (no deleted_at) of the session tenant (RLS + infra.current_tenant_id())
		{`WITH e AS (
				SELECT le.school_id, le.grade_level, o.objective_id, COALESCE((le.payload->>'is_correct')::boolean, false) AS correct,
					COALESCE((le.payload->>'rapid_guess')::boolean, false) AS rapid
				FROM learning.learning_events le CROSS JOIN LATERAL unnest(le.objective_ids) AS o(objective_id)
				WHERE le.tenant_id = infra.current_tenant_id() AND le.event_type = 'question.answered' AND le.grade_level IS NOT NULL
					AND le.efficacy_wave_id IS NULL
					AND le.occurred_at >= $2::timestamptz AND le.occurred_at < $3::timestamptz
					AND COALESCE((le.payload->>'is_scored')::boolean, true) AND COALESCE((le.payload->>'attempt_no')::int, 1) = 1)
			INSERT INTO learning.rollup_grade_objective_weekly AS r (tenant_id, school_id, grade_level, objective_id, bucket_date, metric_version,
				evidence_count, correct_num, correct_den, is_final, computed_at)
			SELECT infra.current_tenant_id(), e.school_id, e.grade_level, e.objective_id, $1::date, 1, count(*),
				count(*) FILTER (WHERE e.correct AND NOT e.rapid), count(*) FILTER (WHERE NOT e.rapid), $4, now()
			FROM e GROUP BY e.school_id, e.grade_level, e.objective_id
			ON CONFLICT (tenant_id, school_id, grade_level, objective_id, bucket_date, metric_version) DO UPDATE SET
				evidence_count = EXCLUDED.evidence_count, correct_num = EXCLUDED.correct_num, correct_den = EXCLUDED.correct_den,
				is_final = EXCLUDED.is_final, computed_at = now()`,
			[]any{b, from, to, final}},
	}
	if withLevels {
		ss = append(ss,
			// tenancyguard:ok derived tables (no deleted_at) of the session tenant (RLS + infra.current_tenant_id())
			stmt{`INSERT INTO learning.rollup_grade_objective_weekly AS r (tenant_id, school_id, grade_level, objective_id, bucket_date,
					metric_version, level_dist, is_final, computed_at)
				SELECT tenant_id, school_id, grade_level, objective_id, $1::date, 1, level_dist, $2, now()
				FROM learning.rm_grade_objective_stats
				WHERE tenant_id = infra.current_tenant_id() AND students_with_evidence > 0
				ON CONFLICT (tenant_id, school_id, grade_level, objective_id, bucket_date, metric_version) DO UPDATE SET
					level_dist = EXCLUDED.level_dist, computed_at = now()`, []any{b, final}},
			// tenancyguard:ok derived tables (no deleted_at) of the session tenant (RLS + infra.current_tenant_id())
			stmt{`UPDATE learning.rm_grade_objective_stats s SET accuracy_num = w.correct_num, accuracy_den = w.correct_den
				FROM learning.rollup_grade_objective_weekly w
				WHERE s.tenant_id = infra.current_tenant_id() AND w.tenant_id = s.tenant_id AND w.school_id = s.school_id
					AND w.grade_level = s.grade_level AND w.objective_id = s.objective_id AND w.bucket_date = $1::date
					AND w.metric_version = 1`, []any{b}})
	}
	// tenancyguard:ok derived rollup (no deleted_at); update of the session tenant (RLS)
	ss = append(ss, stmt{`UPDATE learning.rollup_grade_objective_weekly SET is_final = $2 WHERE tenant_id = infra.current_tenant_id()
		AND bucket_date = $1::date AND metric_version = 1`, []any{b, final}})
	return execAll(ctx, tx, "objective week "+b, ss)
}

// roster is the active, not deleted students of the session tenant with their current school and grade (ADR-0095: a
// student belongs to a school and a grade, no class), and the size of each school × grade.
// tenancyguard:ok ref.students projection of the session tenant (RLS + explicit tenant_id), deleted_at IS NULL
const roster = `roster AS (
	SELECT st.student_id, st.school_id, COALESCE(st.grade_level, 0)::smallint AS grade_level
	FROM ref.students st
	WHERE st.tenant_id = infra.current_tenant_id() AND st.deleted_at IS NULL AND st.status = 'active'),
size AS (SELECT school_id, grade_level, count(*) AS n FROM roster GROUP BY school_id, grade_level)`

// levelDist is the level distribution of the rows of m (level column) with not_started = grade size − levelled students.
const levelDist = `ARRAY[GREATEST(max(sz.n) - count(*) FILTER (WHERE m.level <> 'not_started'), 0),
	count(*) FILTER (WHERE m.level = 'beginning'), count(*) FILTER (WHERE m.level = 'developing'),
	count(*) FILTER (WHERE m.level = 'proficient'), count(*) FILTER (WHERE m.level = 'mastered')]::integer[]`

// RefreshReadModels implements write.RollupJobs (LEARNING_ANALYTICS §5): the stored level of objective_masteries is used
// (decay is applied when a student's own progress is read, F031-AC-11) [Proposed]; windows: rapid guesses and activity
// over the last RapidWindowDays days, assignments due in the last 14 days, recommendations generated in the last 7 days
// (started) and 28 days (objectives improved within 14 days).
func (RollupRepo) RefreshReadModels(ctx context.Context, now, today time.Time, th analytics.Thresholds) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	rapidFrom, to := dateParam(today.AddDate(0, 0, -(max(th.RapidWindowDays, 1)-1))), dateParam(today)
	ss := []stmt{
		// tenancyguard:ok derived read model (no deleted_at); update of the session tenant (RLS)
		{`UPDATE learning.rm_grade_objective_stats SET level_dist = '{0,0,0,0,0}', students_with_evidence = 0, student_count = 0,
				computed_at = now()
			WHERE tenant_id = infra.current_tenant_id()`, nil},
		// tenancyguard:ok objective_masteries and curriculum filtered on tenant and deleted_at IS NULL; read model of the session tenant
		{`WITH ` + roster + `,
			m AS (SELECT r.school_id, r.grade_level, om.objective_id, om.level
				FROM roster r JOIN learning.objective_masteries om ON om.student_id = r.student_id
					AND om.tenant_id = infra.current_tenant_id() AND om.deleted_at IS NULL)
			INSERT INTO learning.rm_grade_objective_stats AS x (tenant_id, school_id, grade_level, objective_id, subject_id, topic_id,
				objective_code, yccd_ref, student_count, level_dist, students_with_evidence, computed_at)
			SELECT infra.current_tenant_id(), m.school_id, m.grade_level, m.objective_id, (array_agg(t.subject_id))[1],
				(array_agg(t.topic_id))[1], max(lo.code), max(lo.yccd_ref), max(sz.n), ` + levelDist + `,
				count(*) FILTER (WHERE m.level <> 'not_started'), now()
			FROM m JOIN size sz ON sz.school_id = m.school_id AND sz.grade_level = m.grade_level
			JOIN curriculum.learning_objectives lo ON lo.objective_id = m.objective_id AND lo.deleted_at IS NULL
			JOIN curriculum.topics t ON t.topic_id = lo.topic_id AND t.deleted_at IS NULL
			GROUP BY m.school_id, m.grade_level, m.objective_id
			ON CONFLICT (tenant_id, school_id, grade_level, objective_id) DO UPDATE SET subject_id = EXCLUDED.subject_id,
				topic_id = EXCLUDED.topic_id, objective_code = EXCLUDED.objective_code, yccd_ref = EXCLUDED.yccd_ref,
				student_count = EXCLUDED.student_count, level_dist = EXCLUDED.level_dist,
				students_with_evidence = EXCLUDED.students_with_evidence, computed_at = now()`, nil},
		// tenancyguard:ok derived read model (no deleted_at); update of the session tenant (RLS)
		{`UPDATE learning.rm_school_grade_summary SET student_count = 0, curriculum_coverage_num = 0, curriculum_coverage_den = 0,
				participation_num = 0, participation_den = 0, recommendation_started_num = 0, recommendation_started_den = 0,
				objectives_improved = 0, assignment_completion_num = 0, assignment_completion_den = 0, computed_at = now()
			WHERE tenant_id = infra.current_tenant_id()`, nil},
		// tenancyguard:ok sources filtered on tenant and deleted_at IS NULL (derived progress_daily/history without it); read model of the session tenant
		{`WITH st AS (SELECT s.school_id, COALESCE(s.grade_level, 0)::smallint AS grade_level, s.student_id FROM ref.students s
				WHERE s.tenant_id = infra.current_tenant_id() AND s.deleted_at IS NULL AND s.status = 'active'),
			cnt AS (SELECT school_id, grade_level, count(*) AS n FROM st GROUP BY school_id, grade_level),
			act AS (SELECT st.school_id, st.grade_level, p.subject_id, count(DISTINCT p.student_id) AS n
				FROM learning.progress_daily p JOIN st ON st.student_id = p.student_id
				WHERE p.tenant_id = infra.current_tenant_id() AND p.bucket_date BETWEEN $1::date AND $2::date AND p.metric_version = 1
				GROUP BY st.school_id, st.grade_level, p.subject_id),
			lo AS (SELECT lo.objective_id, lo.stable_key, t.subject_id, t.grade_level
				FROM curriculum.learning_objectives lo JOIN curriculum.topics t ON t.topic_id = lo.topic_id AND t.deleted_at IS NULL
				WHERE lo.deleted_at IS NULL),
			den AS (SELECT subject_id, grade_level, count(DISTINCT stable_key) AS n FROM lo GROUP BY subject_id, grade_level),
			cov AS (SELECT st.school_id, st.grade_level, lo.subject_id, count(DISTINCT lo.stable_key) AS n
				FROM learning.objective_masteries om JOIN st ON st.student_id = om.student_id JOIN lo ON lo.objective_id = om.objective_id
				WHERE om.tenant_id = infra.current_tenant_id() AND om.deleted_at IS NULL AND om.level <> 'not_started'
				GROUP BY st.school_id, st.grade_level, lo.subject_id),
			rec AS (SELECT st.school_id, st.grade_level, COALESCE(r.subject_id, $5::uuid) AS subject_id, count(*) AS den,
					count(*) FILTER (WHERE r.started_attempt_id IS NOT NULL) AS num
				FROM learning.rm_student_recommendations r JOIN st ON st.student_id = r.student_id
				WHERE r.tenant_id = infra.current_tenant_id() AND r.deleted_at IS NULL AND r.generated_at >= $3::timestamptz
				GROUP BY st.school_id, st.grade_level, COALESCE(r.subject_id, $5::uuid)),
			imp AS (SELECT st.school_id, st.grade_level, COALESCE(r.subject_id, $5::uuid) AS subject_id,
					count(DISTINCT r.student_id::text || o.objective_id::text) AS n
				FROM learning.rm_student_recommendations r JOIN st ON st.student_id = r.student_id
				CROSS JOIN LATERAL unnest(r.objective_ids) AS o(objective_id)
				WHERE r.tenant_id = infra.current_tenant_id() AND r.deleted_at IS NULL AND r.generated_at >= $4::timestamptz
					AND EXISTS (SELECT 1 FROM learning.mastery_history h
						WHERE h.tenant_id = r.tenant_id AND h.student_id = r.student_id AND h.objective_id = o.objective_id
							AND h.occurred_at > r.generated_at AND h.occurred_at <= r.generated_at + interval '14 days'
							AND array_position(ARRAY['not_started','beginning','developing','proficient','mastered']::text[], h.level_after::text)
								> array_position(ARRAY['not_started','beginning','developing','proficient','mastered']::text[],
									COALESCE(h.level_before, 'not_started')::text))
				GROUP BY st.school_id, st.grade_level, COALESCE(r.subject_id, $5::uuid)),
			asg AS (SELECT s.school_id, COALESCE(ts.grade_level, st.grade_level, 0)::smallint AS grade_level, a.subject_id, count(*) AS den,
					count(*) FILTER (WHERE s.status IN ('submitted', 'late')) AS num
				FROM lms.assignment_submissions s
				JOIN lms.assignments a ON a.assignment_id = s.assignment_id AND a.tenant_id = s.tenant_id AND a.deleted_at IS NULL
				` + submissionGrade + `
				WHERE s.tenant_id = infra.current_tenant_id() AND s.deleted_at IS NULL AND s.status <> 'excused'
					AND a.status IN ('open', 'closed', 'results_released') AND COALESCE(s.due_at_override, a.due_at) >= $6::timestamptz
				GROUP BY 1, 2, 3),
			k AS (SELECT school_id, grade_level, $5::uuid AS subject_id FROM cnt UNION SELECT school_id, grade_level, subject_id FROM act
				UNION SELECT school_id, grade_level, subject_id FROM cov UNION SELECT school_id, grade_level, subject_id FROM rec
				UNION SELECT school_id, grade_level, subject_id FROM asg)
			INSERT INTO learning.rm_school_grade_summary AS x (tenant_id, school_id, grade_level, subject_id, student_count,
				curriculum_coverage_num, curriculum_coverage_den, participation_num, participation_den, recommendation_started_num,
				recommendation_started_den, objectives_improved, assignment_completion_num, assignment_completion_den, computed_at)
			SELECT infra.current_tenant_id(), k.school_id, k.grade_level, k.subject_id, COALESCE(cnt.n, 0),
				LEAST(COALESCE(cov.n, 0), COALESCE(den.n, 0)), COALESCE(den.n, 0), LEAST(COALESCE(act.n, 0), COALESCE(cnt.n, 0)), COALESCE(cnt.n, 0),
				COALESCE(rec.num, 0), COALESCE(rec.den, 0), COALESCE(imp.n, 0), COALESCE(asg.num, 0), COALESCE(asg.den, 0), now()
			FROM k LEFT JOIN cnt ON cnt.school_id = k.school_id AND cnt.grade_level = k.grade_level
			LEFT JOIN act ON act.school_id = k.school_id AND act.grade_level = k.grade_level AND act.subject_id = k.subject_id
			LEFT JOIN den ON den.subject_id = k.subject_id AND den.grade_level = k.grade_level
			LEFT JOIN cov ON cov.school_id = k.school_id AND cov.grade_level = k.grade_level AND cov.subject_id = k.subject_id
			LEFT JOIN rec ON rec.school_id = k.school_id AND rec.grade_level = k.grade_level AND rec.subject_id = k.subject_id
			LEFT JOIN imp ON imp.school_id = k.school_id AND imp.grade_level = k.grade_level AND imp.subject_id = k.subject_id
			LEFT JOIN asg ON asg.school_id = k.school_id AND asg.grade_level = k.grade_level AND asg.subject_id = k.subject_id
			ON CONFLICT (tenant_id, school_id, grade_level, subject_id) DO UPDATE SET student_count = EXCLUDED.student_count,
				curriculum_coverage_num = EXCLUDED.curriculum_coverage_num, curriculum_coverage_den = EXCLUDED.curriculum_coverage_den,
				participation_num = EXCLUDED.participation_num, participation_den = EXCLUDED.participation_den,
				recommendation_started_num = EXCLUDED.recommendation_started_num, recommendation_started_den = EXCLUDED.recommendation_started_den,
				objectives_improved = EXCLUDED.objectives_improved, assignment_completion_num = EXCLUDED.assignment_completion_num,
				assignment_completion_den = EXCLUDED.assignment_completion_den, computed_at = now()`,
			[]any{rapidFrom, to, now.Add(-7 * 24 * time.Hour), now.Add(-28 * 24 * time.Hour), nilSubject, now.Add(-14 * 24 * time.Hour)}},
	}
	return execAll(ctx, tx, "refresh read models", ss)
}

// AttentionSignals implements write.RollupJobs.
func (RollupRepo) AttentionSignals(ctx context.Context, now, today time.Time, th analytics.Thresholds) ([]write.AttentionRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	rapidFrom := today.AddDate(0, 0, -(max(th.RapidWindowDays, 1) - 1))
	missedFrom := now.Add(-time.Duration(max(th.MissedWindowDays, 1)) * 24 * time.Hour)
	// tenancyguard:ok sources filtered on tenant and deleted_at IS NULL (derived progress_daily without it)
	rows, err := tx.QueryContext(ctx, `WITH st AS (SELECT s.student_id, s.school_id FROM ref.students s
			WHERE s.tenant_id = infra.current_tenant_id() AND s.deleted_at IS NULL AND s.status = 'active'),
		mis AS (SELECT s.student_id, count(*) AS n FROM lms.assignment_submissions s
			JOIN lms.assignments a ON a.assignment_id = s.assignment_id AND a.tenant_id = s.tenant_id AND a.deleted_at IS NULL
			WHERE s.tenant_id = infra.current_tenant_id() AND s.deleted_at IS NULL AND s.status = 'missed'
				AND COALESCE(s.due_at_override, a.due_at) >= $1::timestamptz
			GROUP BY s.student_id),
		beg AS (SELECT om.student_id, count(*) AS n FROM learning.objective_masteries om
			WHERE om.tenant_id = infra.current_tenant_id() AND om.deleted_at IS NULL AND om.level = 'beginning' GROUP BY om.student_id),
		rg AS (SELECT p.student_id, sum(p.items_rapid_guess) AS num, sum(p.items_answered) AS den FROM learning.progress_daily p
			WHERE p.tenant_id = infra.current_tenant_id() AND p.bucket_date BETWEEN $2::date AND $3::date AND p.metric_version = 1
			GROUP BY p.student_id)
		SELECT st.student_id, st.school_id, COALESCE(mis.n, 0), COALESCE(beg.n, 0), COALESCE(rg.num, 0), COALESCE(rg.den, 0)
		FROM st LEFT JOIN mis ON mis.student_id = st.student_id
		LEFT JOIN beg ON beg.student_id = st.student_id LEFT JOIN rg ON rg.student_id = st.student_id
		ORDER BY st.student_id`, missedFrom, dateParam(rapidFrom), dateParam(today))
	if err != nil {
		return nil, fmt.Errorf("attention signals: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.AttentionRow
	for rows.Next() {
		var r write.AttentionRow
		if err := rows.Scan(&r.StudentID, &r.SchoolID, &r.Signals.Missed, &r.Signals.Beginning, &r.Signals.RapidNum,
			&r.Signals.RapidDen); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

const attentionBatch = 1000

// SaveAttention implements write.RollupJobs: upsert by batch (the grade of a student is read from ref.students,
// ADR-0095), then every row not written in this transaction (student gone, deleted or inactive) loses its
// reasons.
func (RollupRepo) SaveAttention(ctx context.Context, rows []write.AttentionRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	for start := 0; start < len(rows); start += attentionBatch {
		part := rows[start:min(start+attentionBatch, len(rows))]
		students, schools, reasons := make([]string, len(part)), make([]string, len(part)), make([]string, len(part))
		missed, beginning, rnum, rden := make([]int64, len(part)), make([]int64, len(part)), make([]int64, len(part)), make([]int64, len(part))
		for i, r := range part {
			students[i], schools[i] = r.StudentID.String(), r.SchoolID.String()
			rs := make([]string, len(r.Reasons))
			for j, x := range r.Reasons {
				rs[j] = string(x)
			}
			reasons[i] = strings.Join(rs, ",")
			missed[i], beginning[i], rnum[i], rden[i] = int64(r.Signals.Missed), int64(r.Signals.Beginning), r.Signals.RapidNum, r.Signals.RapidDen
		}
		// tenancyguard:ok derived read model (no deleted_at); insert/update of the session tenant (RLS)
		if _, err := tx.ExecContext(ctx, `INSERT INTO learning.rm_student_attention AS a (tenant_id, student_id, school_id, reasons,
				missed_assignments, beginning_objectives, rapid_guess_num, rapid_guess_den, computed_at)
			SELECT infra.current_tenant_id(), x.s::uuid, x.sc::uuid,
				CASE WHEN x.r = '' THEN '{}'::varchar(64)[] ELSE string_to_array(x.r, ',')::varchar(64)[] END, x.m, x.b, x.rn, x.rd, now()
			FROM unnest($1::text[], $2::text[], $3::text[], $4::bigint[], $5::bigint[], $6::bigint[], $7::bigint[])
				AS x(s, sc, r, m, b, rn, rd)
			ON CONFLICT (tenant_id, student_id) DO UPDATE SET school_id = EXCLUDED.school_id,
				reasons = EXCLUDED.reasons, missed_assignments = EXCLUDED.missed_assignments, beginning_objectives = EXCLUDED.beginning_objectives,
				rapid_guess_num = EXCLUDED.rapid_guess_num, rapid_guess_den = EXCLUDED.rapid_guess_den, computed_at = now()`,
			students, schools, reasons, missed, beginning, rnum, rden); err != nil {
			return fmt.Errorf("save attention: %w", err)
		}
	}
	// tenancyguard:ok derived read model (no deleted_at); update of the session tenant (RLS)
	if _, err := tx.ExecContext(ctx, `UPDATE learning.rm_student_attention SET reasons = '{}', computed_at = now()
		WHERE tenant_id = infra.current_tenant_id() AND computed_at < now() AND cardinality(reasons) > 0`); err != nil {
		return fmt.Errorf("clear attention: %w", err)
	}
	return nil
}

// SnapshotMastery implements write.RollupJobs.
func (RollupRepo) SnapshotMastery(ctx context.Context, start time.Time) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok objective_masteries, curriculum and students filtered on tenant and deleted_at IS NULL; snapshot of the session tenant
	if _, err := tx.ExecContext(ctx, `WITH m AS (
			SELECT om.student_id, om.school_id, om.value, om.level, t.subject_id
			FROM learning.objective_masteries om
			JOIN ref.students st ON st.student_id = om.student_id AND st.tenant_id = om.tenant_id AND st.deleted_at IS NULL
			JOIN curriculum.learning_objectives lo ON lo.objective_id = om.objective_id AND lo.deleted_at IS NULL
			JOIN curriculum.topics t ON t.topic_id = lo.topic_id AND t.deleted_at IS NULL
			WHERE om.tenant_id = infra.current_tenant_id() AND om.deleted_at IS NULL)
		INSERT INTO learning.mastery_snapshots_weekly AS x (tenant_id, student_id, subject_id, bucket_date, school_id, mean_mastery,
			mastered_count, level_dist, computed_at)
		SELECT infra.current_tenant_id(), m.student_id, m.subject_id, $1::date, (array_agg(m.school_id))[1], round(avg(m.value), 3),
			count(*) FILTER (WHERE m.level = 'mastered'),
			ARRAY[count(*) FILTER (WHERE m.level = 'not_started'), count(*) FILTER (WHERE m.level = 'beginning'),
				count(*) FILTER (WHERE m.level = 'developing'), count(*) FILTER (WHERE m.level = 'proficient'),
				count(*) FILTER (WHERE m.level = 'mastered')]::integer[], now()
		FROM m GROUP BY m.student_id, m.subject_id
		ON CONFLICT (tenant_id, student_id, subject_id, bucket_date) DO UPDATE SET school_id = EXCLUDED.school_id,
			mean_mastery = EXCLUDED.mean_mastery, mastered_count = EXCLUDED.mastered_count, level_dist = EXCLUDED.level_dist,
			computed_at = now()`, dateParam(start)); err != nil {
		return fmt.Errorf("mastery snapshot: %w", err)
	}
	return nil
}
