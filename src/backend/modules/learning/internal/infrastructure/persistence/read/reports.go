package read

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/reporting"
)

// ReportModel implements read.ReportReader (F044 US1) on the dashboard session (role learning_dashboard on the replica;
// learning_platform for the cross-tenant summary): every query is scoped to the tenant of the session (explicit tenant_id =
// infra.current_tenant_id() plus RLS), filters deleted rows of its sources, applies the scope of the requester (schools,
// subjects) stored in the job, never reads chat, attempts nor answers, and pages by keyset (two uuid columns).
type ReportModel struct{}

var (
	_ read.ReportReader    = ReportModel{}
	_ read.ReportJobReader = ReportModel{}
)

// Prepare implements read.ReportReader: statement_timeout of the transaction.
func (ReportModel) Prepare(ctx context.Context, timeout time.Duration) error {
	return DashboardModel{}.Prepare(ctx, timeout)
}

// levelLabels are the Vietnamese labels of the reference levels in the files (ASSESSMENT_REGULATION_PROFILE §4).
var levelLabels = map[string]string{"excellent": "Hoàn thành tốt", "completed": "Hoàn thành", "not_completed": "Chưa hoàn thành",
	"pass": "Đạt", "not_pass": "Chưa đạt", "insufficient_data": "Chưa đủ dữ liệu"}

// reportQuery is the SQL of one report type: the select list, the FROM/WHERE (with $n placeholders after the fixed ones), the
// two keyset columns, an optional GROUP BY, the column names and the row formatter.
type reportQuery struct {
	sel, from, group string
	where            []string
	args             []any
	keys             [2]string
	columns          []string
	scan             func(rows *sql.Rows) ([]string, [2]uuid.UUID, error)
}

func (q *reportQuery) arg(v any) string {
	q.args = append(q.args, v)
	return "$" + strconv.Itoa(len(q.args))
}

func uuids(ids []uuid.UUID) any {
	if ids == nil {
		return nil
	}
	return uuidArray(ids)
}

func f1(v sql.NullFloat64) string {
	if !v.Valid {
		return ""
	}
	return strconv.FormatFloat(v.Float64, 'f', 1, 64)
}

func pct(num, den int64) string {
	if den == 0 {
		return ""
	}
	return strconv.FormatFloat(float64(num)*100/float64(den), 'f', 1, 64)
}

func score10(score, maxScore sql.NullFloat64) string {
	if !score.Valid || !maxScore.Valid || maxScore.Float64 <= 0 {
		return ""
	}
	return strconv.FormatFloat(roundTo(score.Float64/maxScore.Float64*10, 1), 'f', 1, 64)
}

func roundTo(x float64, digits int) float64 {
	p := 1.0
	for i := 0; i < digits; i++ {
		p *= 10
	}
	if x < 0 {
		return -float64(int64(-x*p+0.5)) / p
	}
	return float64(int64(x*p+0.5)) / p
}

func dateArg(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// build returns the query of a spec.
func build(s read.ReportSpec) (*reportQuery, error) {
	p := s.Params
	q := &reportQuery{}
	tenant := "infra.current_tenant_id()"
	switch s.Type {
	case reporting.TypeAssignmentResults, reporting.TypeCourseResults:
		q.sel = `a.assignment_id, a.title, s.student_id, COALESCE(cl.name, ''), s.status, s.score::float8, s.max_score::float8, s.submitted_at,
			s.rapid_guess_count`
		q.from = `lms.assignment_submissions s
			JOIN lms.assignments a ON a.assignment_id = s.assignment_id AND a.tenant_id = s.tenant_id AND a.deleted_at IS NULL
			JOIN ref.students st ON st.student_id = s.student_id AND st.tenant_id = s.tenant_id AND st.deleted_at IS NULL
			LEFT JOIN ref.classes cl ON cl.class_id = s.class_id AND cl.tenant_id = s.tenant_id`
		q.where = []string{"s.tenant_id = " + tenant, "s.deleted_at IS NULL"}
		switch {
		case s.Type == reporting.TypeCourseResults:
			q.where = append(q.where, "a.course_id = "+q.arg(p.CourseID), "a.mode = 'graded'")
		case p.AssignmentID != nil:
			q.where = append(q.where, "a.assignment_id = "+q.arg(p.AssignmentID))
		default:
			q.where = append(q.where, "a.due_at >= "+q.arg(dateArg(p.From))+"::date", "a.due_at < "+q.arg(dateArg(p.To))+"::date + 1")
		}
		if p.SubjectIDs != nil {
			q.where = append(q.where, "a.subject_id = ANY("+q.arg(uuids(p.SubjectIDs))+"::uuid[])")
		}
		if ids := schoolFilter(p); ids != nil {
			q.where = append(q.where, "a.school_id = ANY("+q.arg(uuids(ids))+"::uuid[])")
		}
		q.keys = [2]string{"a.assignment_id", "s.student_id"}
		q.columns = []string{"assignment_id", "assignment_title", "student_id", "class", "status", "score_10", "submitted_at", "too_fast"}
		q.scan = func(rows *sql.Rows) ([]string, [2]uuid.UUID, error) {
			var asg, student uuid.UUID
			var title, class, status string
			var score, maxScore sql.NullFloat64
			var submitted sql.NullTime
			var rapid int
			if err := rows.Scan(&asg, &title, &student, &class, &status, &score, &maxScore, &submitted, &rapid); err != nil {
				return nil, [2]uuid.UUID{}, err
			}
			sub := ""
			if submitted.Valid {
				sub = submitted.Time.UTC().Format(time.RFC3339)
			}
			return []string{asg.String(), title, student.String(), class, status, score10(score, maxScore), sub, strconv.FormatBool(rapid > 0)},
				[2]uuid.UUID{asg, student}, nil
		}
	case reporting.TypeRegulationReference:
		q.sel = `r.class_id, r.student_id, COALESCE(cl.name, ''), r.level_code, r.coverage_ratio::float8, r.proficient_ratio::float8,
			r.mastered_ratio::float8, r.official_score::float8, r.rapid_guess_ratio::float8, cardinality(r.low_reliability_exam_ids), r.profile_version`
		q.from = `learning.rm_student_subject_reference_levels r
			JOIN ref.students st ON st.student_id = r.student_id AND st.tenant_id = r.tenant_id AND st.deleted_at IS NULL
			LEFT JOIN ref.classes cl ON cl.class_id = r.class_id AND cl.tenant_id = r.tenant_id`
		q.where = []string{"r.tenant_id = " + tenant, "r.school_id = " + q.arg(p.SchoolID), "r.subject_id = " + q.arg(p.SubjectID),
			"r.period = " + q.arg(p.Period), "r.academic_year = " + q.arg(p.AcademicYear),
			`r.profile_version = (SELECT max(x.profile_version) FROM learning.rm_student_subject_reference_levels x WHERE x.tenant_id = r.tenant_id
				AND x.student_id = r.student_id AND x.subject_id = r.subject_id AND x.academic_year = r.academic_year AND x.period = r.period)`}
		if p.ClassID != nil {
			q.where = append(q.where, "r.class_id = "+q.arg(p.ClassID))
		}
		q.keys = [2]string{"r.class_id", "r.student_id"}
		q.columns = []string{"class", "student_id", "level_code", "level", "coverage_ratio", "proficient_ratio", "mastered_ratio",
			"official_score", "rapid_guess_ratio", "low_reliability_exams", "profile_version"}
		q.scan = func(rows *sql.Rows) ([]string, [2]uuid.UUID, error) {
			var class, student uuid.UUID
			var name, level string
			var cov, prof, mast, rapid float64
			var official sql.NullFloat64
			var low, ver int
			if err := rows.Scan(&class, &student, &name, &level, &cov, &prof, &mast, &official, &rapid, &low, &ver); err != nil {
				return nil, [2]uuid.UUID{}, err
			}
			ff := func(x float64) string { return strconv.FormatFloat(x, 'f', 4, 64) }
			return []string{name, student.String(), level, levelLabels[level], ff(cov), ff(prof), ff(mast), f1(official), ff(rapid),
				strconv.Itoa(low), strconv.Itoa(ver)}, [2]uuid.UUID{class, student}, nil
		}
	case reporting.TypeSkillMap:
		q.sel = `m.student_id, m.objective_id, o.code, m.level, m.value::float8, m.evidence_count`
		q.from = `learning.objective_masteries m
			JOIN ref.enrollments e ON e.student_id = m.student_id AND e.tenant_id = m.tenant_id AND e.status = 'active'
			JOIN ref.students st ON st.student_id = m.student_id AND st.tenant_id = m.tenant_id AND st.deleted_at IS NULL
			JOIN curriculum.learning_objectives o ON o.objective_id = m.objective_id AND o.deleted_at IS NULL
			JOIN curriculum.topics t ON t.topic_id = o.topic_id AND t.deleted_at IS NULL`
		q.where = []string{"m.tenant_id = " + tenant, "m.deleted_at IS NULL", "e.class_id = " + q.arg(p.ClassID),
			"t.subject_id = " + q.arg(p.SubjectID)}
		q.keys = [2]string{"m.student_id", "m.objective_id"}
		q.columns = []string{"student_id", "objective_code", "mastery_level", "mastery_value", "evidence_count"}
		q.scan = func(rows *sql.Rows) ([]string, [2]uuid.UUID, error) {
			var student, objective uuid.UUID
			var code, level string
			var value float64
			var n int
			if err := rows.Scan(&student, &objective, &code, &level, &value, &n); err != nil {
				return nil, [2]uuid.UUID{}, err
			}
			return []string{student.String(), code, level, strconv.FormatFloat(value, 'f', 3, 64), strconv.Itoa(n)},
				[2]uuid.UUID{student, objective}, nil
		}
	case reporting.TypeSchoolMonthly, reporting.TypeClassProgress:
		q.sel = `r.class_id, r.subject_id, COALESCE(max(cl.name), ''), sum(r.activities_completed), sum(r.accuracy_num), sum(r.accuracy_den),
			sum(r.rapid_guess_num), sum(r.rapid_guess_den), sum(r.active_seconds), sum(r.assignments_due), sum(r.assignments_completed),
			sum(r.assignments_on_time)`
		q.from = `learning.rollup_class_daily r LEFT JOIN ref.classes cl ON cl.class_id = r.class_id AND cl.tenant_id = r.tenant_id`
		q.where = []string{"r.tenant_id = " + tenant, "r.subject_id <> '00000000-0000-0000-0000-000000000000'::uuid",
			"r.bucket_date >= " + q.arg(dateArg(p.From)) + "::date", "r.bucket_date <= " + q.arg(dateArg(p.To)) + "::date"}
		if p.SchoolID != nil {
			q.where = append(q.where, "r.school_id = "+q.arg(p.SchoolID))
		}
		if ids := schoolFilter(p); ids != nil {
			q.where = append(q.where, "r.school_id = ANY("+q.arg(uuids(ids))+"::uuid[])")
		}
		if p.SubjectIDs != nil {
			q.where = append(q.where, "r.subject_id = ANY("+q.arg(uuids(p.SubjectIDs))+"::uuid[])")
		}
		if p.GradeLevel != nil {
			q.where = append(q.where, "r.grade_level = "+q.arg(*p.GradeLevel))
		}
		q.keys, q.group = [2]string{"r.class_id", "r.subject_id"}, "r.class_id, r.subject_id"
		q.columns = []string{"class_id", "class", "subject_id", "activities_completed", "accuracy_pct", "rapid_guess_pct", "active_minutes",
			"assignments_due", "assignments_completed", "assignments_on_time"}
		q.scan = func(rows *sql.Rows) ([]string, [2]uuid.UUID, error) {
			var class, subject uuid.UUID
			var name string
			var acts, an, ad, rn, rd, secs, due, done, onTime int64
			if err := rows.Scan(&class, &subject, &name, &acts, &an, &ad, &rn, &rd, &secs, &due, &done, &onTime); err != nil {
				return nil, [2]uuid.UUID{}, err
			}
			i := func(x int64) string { return strconv.FormatInt(x, 10) }
			return []string{class.String(), name, subject.String(), i(acts), pct(an, ad), pct(rn, rd), i(secs / 60), i(due), i(done), i(onTime)},
				[2]uuid.UUID{class, subject}, nil
		}
	case reporting.TypeLearningSummary:
		return summaryQuery(s)
	default:
		return nil, fmt.Errorf("report type %q has no query", s.Type)
	}
	return q, nil
}

// summaryQuery is learning_summary: per school × subject of the tenant, or per tenant × subject across tenants (System
// Admin, tenants of the job) with the cells under analytics_min_cell_size students hidden (F044-AC-06).
func summaryQuery(s read.ReportSpec) (*reportQuery, error) {
	p := s.Params
	q := &reportQuery{}
	group, table := "r.school_id", "learning.rollup_school_daily"
	q.where = []string{"r.tenant_id = infra.current_tenant_id()"}
	if s.TenantID == nil {
		group, table = "r.tenant_id", "learning.rollup_tenant_daily"
		q.where = []string{"r.tenant_id = ANY(" + q.arg(uuids(p.TenantIDs)) + "::uuid[])"}
	}
	minCell := q.arg(max(s.MinCellSize, 1))
	q.sel = group + `, r.subject_id, max(COALESCE(r.active_students, 0)),
		CASE WHEN max(COALESCE(r.active_students, 0)) < ` + minCell + ` THEN NULL ELSE sum(r.activities_completed) END,
		CASE WHEN max(COALESCE(r.active_students, 0)) < ` + minCell + ` THEN NULL ELSE sum(r.accuracy_num) END, sum(r.accuracy_den),
		CASE WHEN max(COALESCE(r.active_students, 0)) < ` + minCell + ` THEN NULL ELSE sum(r.active_seconds) END`
	q.from = table + " r"
	q.where = append(q.where, "r.subject_id <> '00000000-0000-0000-0000-000000000000'::uuid",
		"r.bucket_date >= "+q.arg(dateArg(p.From))+"::date", "r.bucket_date <= "+q.arg(dateArg(p.To))+"::date")
	if s.TenantID != nil {
		if ids := schoolFilter(p); ids != nil {
			q.where = append(q.where, "r.school_id = ANY("+q.arg(uuids(ids))+"::uuid[])")
		}
	}
	if p.GradeLevel != nil {
		q.where = append(q.where, "r.grade_level = "+q.arg(*p.GradeLevel))
	}
	q.keys, q.group = [2]string{group, "r.subject_id"}, group+", r.subject_id"
	first := "school_id"
	if s.TenantID == nil {
		first = "tenant_id"
	}
	q.columns = []string{first, "subject_id", "students", "activities_completed", "accuracy_pct", "active_minutes"}
	q.scan = func(rows *sql.Rows) ([]string, [2]uuid.UUID, error) {
		var g, subject uuid.UUID
		var students int64
		var acts, an, secs sql.NullInt64
		var ad int64
		if err := rows.Scan(&g, &subject, &students, &acts, &an, &ad, &secs); err != nil {
			return nil, [2]uuid.UUID{}, err
		}
		hidden := !acts.Valid
		cell := func(v sql.NullInt64, f func(int64) string) string {
			if hidden {
				return ""
			}
			return f(v.Int64)
		}
		st := strconv.FormatInt(students, 10)
		if hidden {
			st = "<" + strconv.Itoa(max(s.MinCellSize, 1))
		}
		return []string{g.String(), subject.String(), st, cell(acts, func(x int64) string { return strconv.FormatInt(x, 10) }),
				cell(an, func(x int64) string { return pct(x, ad) }), cell(secs, func(x int64) string { return strconv.FormatInt(x/60, 10) })},
			[2]uuid.UUID{g, subject}, nil
	}
	return q, nil
}

// schoolFilter is the narrowest school list of the job: the requested school_ids within the schools of the requester.
func schoolFilter(p write.ReportParams) []uuid.UUID {
	if p.SchoolIDs == nil {
		return p.ScopeSchoolIDs
	}
	if p.ScopeSchoolIDs == nil {
		return p.SchoolIDs
	}
	out := []uuid.UUID{}
	for _, a := range p.SchoolIDs {
		for _, b := range p.ScopeSchoolIDs {
			if a == b {
				out = append(out, a)
			}
		}
	}
	return out
}

func (q *reportQuery) sql(keyset bool, limit int, after [2]uuid.UUID) string {
	where := append([]string{}, q.where...)
	if keyset {
		where = append(where, "("+q.keys[0]+", "+q.keys[1]+") > ("+q.arg(after[0])+"::uuid, "+q.arg(after[1])+"::uuid)")
	}
	s := "SELECT " + q.sel + " FROM " + q.from + " WHERE " + strings.Join(where, " AND ")
	if q.group != "" {
		s += " GROUP BY " + q.group
	}
	if keyset {
		s += " ORDER BY " + q.keys[0] + ", " + q.keys[1] + " LIMIT " + q.arg(limit)
	}
	return s
}

// CountReportRows implements read.ReportReader.
func (ReportModel) CountReportRows(ctx context.Context, s read.ReportSpec) (int64, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return 0, err
	}
	q, err := build(s)
	if err != nil {
		return 0, err
	}
	var n int64
	// tenancyguard:ok the built query has tenant_id = infra.current_tenant_id() (or the tenants of a System Admin job) and deleted_at IS NULL on its sources
	err = tx.QueryRowContext(ctx, "SELECT count(*) FROM ("+q.sql(false, 0, [2]uuid.UUID{})+") c", q.args...).Scan(&n)
	return n, dashErr("report count", err)
}

// ReportRows implements read.ReportReader.
func (ReportModel) ReportRows(ctx context.Context, s read.ReportSpec, after string, limit int) (read.ReportBatch, error) {
	var b read.ReportBatch
	tx, err := sqlTx(ctx)
	if err != nil {
		return b, err
	}
	q, err := build(s)
	if err != nil {
		return b, err
	}
	b.Columns = q.columns
	var key [2]uuid.UUID
	if after != "" {
		a, c, ok := strings.Cut(after, "|")
		if key[0], err = uuid.Parse(a); err == nil && ok {
			key[1], err = uuid.Parse(c)
		}
		if err != nil || !ok {
			return b, fmt.Errorf("report cursor: invalid")
		}
	}
	// tenancyguard:ok the built query has tenant_id = infra.current_tenant_id() (or the tenants of a System Admin job) and deleted_at IS NULL on its sources
	rows, err := tx.QueryContext(ctx, q.sql(true, limit, key), q.args...)
	if err != nil {
		return b, dashErr("report rows", err)
	}
	defer func() { _ = rows.Close() }()
	var last [2]uuid.UUID
	for rows.Next() {
		row, k, err := q.scan(rows)
		if err != nil {
			return b, err
		}
		b.Rows, last = append(b.Rows, row), k
	}
	if err := rows.Err(); err != nil {
		return b, dashErr("report rows", err)
	}
	if len(b.Rows) == limit {
		b.Next = last[0].String() + "|" + last[1].String()
	}
	return b, nil
}

// ReportJob implements read.ReportJobReader: the job visible in the session (tenant RLS, or learning_platform for System Admin).
func (ReportModel) ReportJob(ctx context.Context, id uuid.UUID) (write.ReportJobRow, bool, error) {
	tx, err := sqlTx(ctx)
	if err != nil {
		return write.ReportJobRow{}, false, err
	}
	// tenancyguard:ok one job by id; tenant of the session by RLS (System Admin: learning_platform, tenant_id IS NULL checked by the caller); deleted_at IS NULL
	j, err := ScanReportJob(tx.QueryRowContext(ctx, `SELECT report_id, tenant_id, scope_level, report_type, format, params,
		requested_by, status, row_count, storage_key, error, idempotency_key, run_count, started_at, finished_at, file_expires_at, created_at
		FROM learning.report_jobs WHERE report_id = $1 AND deleted_at IS NULL`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return j, false, nil
	}
	return j, err == nil, err
}

// ReportJobColumns are the columns of learning.report_jobs read by ScanReportJob.
const ReportJobColumns = `report_id, tenant_id, scope_level, report_type, format, params, requested_by, status, row_count, storage_key,
	error, idempotency_key, run_count, started_at, finished_at, file_expires_at, created_at`

// ScanReportJob scans the ReportJobColumns of a row.
func ScanReportJob(r interface{ Scan(...any) error }) (write.ReportJobRow, error) {
	var j write.ReportJobRow
	var tenant uuid.NullUUID
	var params []byte
	var rows sql.NullInt64
	var key, errText sql.NullString
	if err := r.Scan(&j.ID, &tenant, &j.Scope, &j.Type, &j.Format, &params, &j.RequestedBy, &j.Status, &rows, &key, &errText,
		&j.IdempotencyKey, &j.RunCount, &j.StartedAt, &j.FinishedAt, &j.FileExpiresAt, &j.CreatedAt); err != nil {
		return j, err
	}
	if tenant.Valid {
		t := tenant.UUID
		j.TenantID = &t
	}
	if rows.Valid {
		n := int(rows.Int64)
		j.RowCount = &n
	}
	if key.Valid {
		j.StorageKey = &key.String
	}
	if errText.Valid {
		j.Error = &errText.String
	}
	if err := json.Unmarshal(params, &j.Params); err != nil {
		return j, fmt.Errorf("report params: %w", err)
	}
	return j, nil
}
