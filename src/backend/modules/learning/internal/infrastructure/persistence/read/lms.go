package read

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
)

// LMSModel implements read.LMSReadModel with explicit SQL in the read transaction of ctx (TenantScope: RLS).
type LMSModel struct{}

var _ read.LMSReadModel = LMSModel{}

func nullUUID(n uuid.NullUUID) *uuid.UUID {
	if !n.Valid {
		return nil
	}
	id := n.UUID
	return &id
}

func optionalArray(ids []uuid.UUID) any {
	if len(ids) == 0 {
		return nil
	}
	return uuidArray(ids)
}

// Courses implements read.LMSReadModel.
func (LMSModel) Courses(ctx context.Context, f read.CourseFilter) ([]dto.CourseView, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	var status any
	if f.Status != "" {
		status = f.Status
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL; ref projections of the same tenant
	rows, err := q.QueryContext(ctx, `SELECT c.course_id, c.tenant_id, c.school_id, c.class_id, COALESCE(k.name, ''), c.subject_id, c.grade_level,
			c.academic_year_id, c.curriculum_version_id, c.textbook_series_id, c.status, c.version,
			(SELECT count(DISTINCT e.student_id) FROM ref.enrollments e WHERE e.class_id = c.class_id AND e.status = 'active'
				AND e.tenant_id = c.tenant_id)
		FROM lms.courses c LEFT JOIN ref.classes k ON k.class_id = c.class_id AND k.tenant_id = c.tenant_id
		WHERE c.tenant_id = infra.current_tenant_id() AND c.deleted_at IS NULL
			AND ($1::uuid IS NULL OR c.school_id = $1::uuid) AND ($2::uuid IS NULL OR c.class_id = $2::uuid)
			AND ($3::uuid IS NULL OR c.subject_id = $3::uuid) AND ($4::uuid IS NULL OR c.academic_year_id = $4::uuid)
			AND ($5::varchar IS NULL OR c.status = $5::varchar) AND ($6::uuid[] IS NULL OR c.class_id = ANY($6::uuid[]))
		ORDER BY c.school_id, k.name, c.subject_id, c.course_id LIMIT 5000`,
		f.SchoolID, f.ClassID, f.SubjectID, f.AcademicYearID, status, optionalArray(f.ClassIDs))
	if err != nil {
		return nil, fmt.Errorf("courses: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []dto.CourseView
	for rows.Next() {
		var c dto.CourseView
		var series uuid.NullUUID
		if err := rows.Scan(&c.CourseID, &c.TenantID, &c.SchoolID, &c.ClassID, &c.ClassName, &c.SubjectID, &c.Grade, &c.AcademicYearID,
			&c.CurriculumVersionID, &series, &c.Status, &c.Version, &c.StudentCount); err != nil {
			return nil, err
		}
		c.TextbookSeriesID = nullUUID(series)
		out = append(out, c)
	}
	return out, rows.Err()
}

// Course implements read.LMSReadModel.
func (m LMSModel) Course(ctx context.Context, id uuid.UUID) (*dto.CourseView, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	var school, class uuid.UUID
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	err = q.QueryRowContext(ctx, `SELECT school_id, class_id FROM lms.courses WHERE course_id = $1 AND tenant_id = infra.current_tenant_id()
		AND deleted_at IS NULL`, id).Scan(&school, &class)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("course: %w", err)
	}
	cs, err := m.Courses(ctx, read.CourseFilter{SchoolID: &school, ClassID: &class})
	if err != nil {
		return nil, err
	}
	for _, c := range cs {
		if c.CourseID == id {
			return &c, nil
		}
	}
	return nil, nil
}

func pathView(ctx context.Context, q *sql.Tx, where string, args ...any) (*dto.LearningPathView, error) {
	var p dto.LearningPathView
	var course, series uuid.NullUUID
	var tenant uuid.NullUUID
	// tenancyguard:ok hybrid lms.learning_paths: RLS hybrid_read (session tenant or platform template), deleted_at IS NULL in every caller
	err := q.QueryRowContext(ctx, `SELECT path_id, tenant_id, course_id, textbook_series_id, title, unlock_mode, version, updated_at
		FROM lms.learning_paths WHERE deleted_at IS NULL AND status = 'active' AND `+where+` LIMIT 1`, args...).
		Scan(&p.PathID, &tenant, &course, &series, &p.Title, &p.UnlockMode, &p.Version, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("learning path: %w", err)
	}
	p.IsTemplate, p.TextbookSeriesID, p.UpdatedAt = !tenant.Valid, nullUUID(series), p.UpdatedAt.UTC()
	if course.Valid {
		p.CourseID = course.UUID
	}
	// tenancyguard:ok hybrid lms.path_nodes: RLS hybrid_read (session tenant or platform template), deleted_at IS NULL
	rows, err := q.QueryContext(ctx, `SELECT node_id, position, content_type, content_id, content_version_id, COALESCE(title, ''),
		array_to_string(objective_ids, ','), unlock_rule FROM lms.path_nodes WHERE path_id = $1 AND deleted_at IS NULL ORDER BY position`, p.PathID)
	if err != nil {
		return nil, fmt.Errorf("path nodes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	p.Nodes = []dto.PathNodeView{}
	for rows.Next() {
		var n dto.PathNodeView
		var los string
		var rule []byte
		if err := rows.Scan(&n.NodeID, &n.Position, &n.ContentRef.ContentType, &n.ContentID, &n.ContentRef.ContentVersionID, &n.Title, &los,
			&rule); err != nil {
			return nil, err
		}
		n.ObjectiveIDs = idsOf(los)
		if string(rule) != "{}" {
			n.UnlockRule = json.RawMessage(rule)
		}
		p.Nodes = append(p.Nodes, n)
	}
	return &p, rows.Err()
}

// CoursePath implements read.LMSReadModel.
func (LMSModel) CoursePath(ctx context.Context, c dto.CourseView) (*dto.LearningPathView, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	p, err := pathView(ctx, q, `course_id = $1 AND tenant_id = infra.current_tenant_id()`, c.CourseID)
	if err != nil || p != nil {
		return p, err
	}
	// The template of the textbook series of the course, otherwise of its subject and grade [Proposed].
	p, err = pathView(ctx, q, `tenant_id IS NULL AND (($1::uuid IS NOT NULL AND textbook_series_id = $1::uuid)
		OR ($1::uuid IS NULL AND subject_id = $2 AND grade_level = $3)) ORDER BY updated_at DESC`, c.TextbookSeriesID, c.SubjectID, c.Grade)
	if p != nil {
		p.CourseID = c.CourseID
	}
	return p, err
}

// Schedule implements read.LMSReadModel.
func (LMSModel) Schedule(ctx context.Context, f read.ScheduleFilter) ([]dto.ScheduleItemView, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL on both tables
	rows, err := q.QueryContext(ctx, `SELECT s.schedule_item_id, s.course_id, s.school_id, c.subject_id, to_char(s.date, 'YYYY-MM-DD'), s.period,
			s.kind, s.ref_type, s.ref_id, s.note, s.version
		FROM lms.schedule_items s JOIN lms.courses c ON c.course_id = s.course_id AND c.tenant_id = s.tenant_id AND c.deleted_at IS NULL
		WHERE s.tenant_id = infra.current_tenant_id() AND s.deleted_at IS NULL AND s.date BETWEEN $1::date AND $2::date
			AND ($3::uuid[] IS NULL OR s.course_id = ANY($3::uuid[])) AND ($4::uuid IS NULL OR c.class_id = $4::uuid)
		ORDER BY s.date, s.period NULLS LAST, s.schedule_item_id LIMIT 5000`, f.From, f.To, optionalArray(f.CourseIDs), f.ClassID)
	if err != nil {
		return nil, fmt.Errorf("schedule: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []dto.ScheduleItemView
	for rows.Next() {
		var s dto.ScheduleItemView
		var period sql.NullInt32
		var refType, note sql.NullString
		var ref uuid.NullUUID
		if err := rows.Scan(&s.ScheduleItemID, &s.CourseID, &s.SchoolID, &s.SubjectID, &s.Date, &period, &s.Kind, &refType, &ref, &note,
			&s.Version); err != nil {
			return nil, err
		}
		if period.Valid {
			p := int(period.Int32)
			s.Period = &p
		}
		if refType.Valid {
			s.RefType = &refType.String
		}
		if note.Valid {
			s.Note = &note.String
		}
		s.RefID = nullUUID(ref)
		out = append(out, s)
	}
	return out, rows.Err()
}

func groupMembers(ctx context.Context, q *sql.Tx, id uuid.UUID) ([]uuid.UUID, error) {
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	rows, err := q.QueryContext(ctx, `SELECT student_id FROM lms.study_group_members WHERE study_group_id = $1
		AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL ORDER BY added_at, student_id`, id)
	if err != nil {
		return nil, fmt.Errorf("study group members: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []uuid.UUID{}
	for rows.Next() {
		var s uuid.UUID
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func groupRows(ctx context.Context, q *sql.Tx, where string, args ...any) ([]dto.StudyGroupView, error) {
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL in every caller condition
	rows, err := q.QueryContext(ctx, `SELECT study_group_id, school_id, name, class_id, grade_level, member_count, created_by, updated_at, version
		FROM lms.study_groups WHERE tenant_id = infra.current_tenant_id() AND deleted_at IS NULL AND `+where, args...)
	if err != nil {
		return nil, fmt.Errorf("study groups: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []dto.StudyGroupView
	for rows.Next() {
		var g dto.StudyGroupView
		var class uuid.NullUUID
		if err := rows.Scan(&g.StudyGroupID, &g.SchoolID, &g.Name, &class, &g.GradeLevel, &g.MemberCount, &g.CreatedBy, &g.UpdatedAt,
			&g.Version); err != nil {
			return nil, err
		}
		g.ClassID, g.UpdatedAt = nullUUID(class), g.UpdatedAt.UTC()
		out = append(out, g)
	}
	return out, rows.Err()
}

func (LMSModel) groups(ctx context.Context, q *sql.Tx, where string, args ...any) ([]dto.StudyGroupView, error) {
	out, err := groupRows(ctx, q, where, args...)
	if err != nil {
		return nil, err
	}
	for i := range out {
		if out[i].MemberIDs, err = groupMembers(ctx, q, out[i].StudyGroupID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// StudyGroups implements read.LMSReadModel.
func (m LMSModel) StudyGroups(ctx context.Context, schoolID uuid.UUID, classID *uuid.UUID, grade *int) ([]dto.StudyGroupView, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	return m.groups(ctx, q, `school_id = $1 AND ($2::uuid IS NULL OR class_id = $2::uuid) AND ($3::int IS NULL OR grade_level = $3::int)
		ORDER BY lower(name), study_group_id LIMIT 2000`, schoolID, classID, grade)
}

// StudyGroup implements read.LMSReadModel.
func (m LMSModel) StudyGroup(ctx context.Context, schoolID, id uuid.UUID) (*dto.StudyGroupView, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	gs, err := m.groups(ctx, q, `school_id = $1 AND study_group_id = $2`, schoolID, id)
	if err != nil || len(gs) == 0 {
		return nil, err
	}
	return &gs[0], nil
}

// CurriculumSettings implements read.LMSReadModel.
func (LMSModel) CurriculumSettings(ctx context.Context, schoolID uuid.UUID, yearID *uuid.UUID) (dto.CurriculumSettingsView, error) {
	out := dto.CurriculumSettingsView{SchoolID: schoolID, Items: []dto.CurriculumSettingView{}}
	q, err := sqlTx(ctx)
	if err != nil {
		return out, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()); the version counts the deleted rows too (optimistic lock)
	if err := q.QueryRowContext(ctx, `SELECT 1 + count(*) FROM lms.school_curriculum_settings WHERE school_id = $1
		AND tenant_id = infra.current_tenant_id()`, schoolID).Scan(&out.Version); err != nil {
		return out, fmt.Errorf("curriculum settings version: %w", err)
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	rows, err := q.QueryContext(ctx, `SELECT academic_year_id, grade_level, curriculum_version_id, set_by, updated_at
		FROM lms.school_curriculum_settings WHERE school_id = $1 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL
			AND ($2::uuid IS NULL OR academic_year_id = $2::uuid)
		ORDER BY academic_year_id, grade_level NULLS FIRST`, schoolID, yearID)
	if err != nil {
		return out, fmt.Errorf("curriculum settings: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var s dto.CurriculumSettingView
		var grade sql.NullInt32
		if err := rows.Scan(&s.AcademicYearID, &grade, &s.CurriculumVersionID, &s.SetBy, &s.UpdatedAt); err != nil {
			return out, err
		}
		if grade.Valid {
			g := int(grade.Int32)
			s.GradeLevel = &g
		}
		s.UpdatedAt = s.UpdatedAt.UTC()
		out.Items = append(out.Items, s)
	}
	return out, rows.Err()
}

// StudentClasses implements read.LMSReadModel.
func (LMSModel) StudentClasses(ctx context.Context, studentID uuid.UUID) ([]uuid.UUID, error) {
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok projection ref.enrollments (no deleted_at, status active) of the session tenant (RLS + infra.current_tenant_id())
	rows, err := q.QueryContext(ctx, `SELECT DISTINCT class_id FROM ref.enrollments WHERE student_id = $1 AND status = 'active'
		AND tenant_id = infra.current_tenant_id()`, studentID)
	if err != nil {
		return nil, fmt.Errorf("student classes: %w", err)
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

// CompletedContent implements read.LMSReadModel: a graded attempt of the activity or of the question set, or a completed
// lesson session.
func (LMSModel) CompletedContent(ctx context.Context, studentID uuid.UUID, contentIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	out := map[uuid.UUID]bool{}
	if len(contentIDs) == 0 {
		return out, nil
	}
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session on attempts and lesson sessions (infra.current_tenant_id() / RLS) and deleted_at IS NULL
	rows, err := q.QueryContext(ctx, `SELECT DISTINCT x FROM (
			SELECT COALESCE(a.activity_id, a.context_id) AS x FROM assessment.attempts a WHERE a.tenant_id = infra.current_tenant_id()
				AND a.student_id = $1 AND a.status = 'graded' AND a.deleted_at IS NULL
				AND (a.activity_id = ANY($2::uuid[]) OR (a.source_kind = 'question_set' AND a.context_id = ANY($2::uuid[])))
			UNION SELECT l.lesson_id FROM learning.lesson_sessions l WHERE l.tenant_id = infra.current_tenant_id() AND l.student_id = $1
				AND l.status = 'completed' AND l.deleted_at IS NULL AND l.lesson_id = ANY($2::uuid[])) s`, studentID, uuidArray(contentIDs))
	if err != nil {
		return nil, fmt.Errorf("completed content: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// UnlockedNodes implements read.LMSReadModel.
func (LMSModel) UnlockedNodes(ctx context.Context, studentID uuid.UUID, nodeIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	out := map[uuid.UUID]bool{}
	if len(nodeIDs) == 0 {
		return out, nil
	}
	q, err := sqlTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	rows, err := q.QueryContext(ctx, `SELECT node_id FROM lms.path_node_unlocks WHERE student_id = $1 AND node_id = ANY($2::uuid[])
		AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, studentID, uuidArray(nodeIDs))
	if err != nil {
		return nil, fmt.Errorf("unlocked nodes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}
