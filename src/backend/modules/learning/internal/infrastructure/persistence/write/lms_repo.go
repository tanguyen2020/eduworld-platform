package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/i18ntext"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// LMSRepo implements write.LMSStore with explicit SQL in the transaction of ctx: the lms tables are tenant (RLS T,
// infra.current_tenant_id()), learning paths hybrid (platform templates readable by every tenant), soft-delete only.
type LMSRepo struct{}

var _ write.LMSStore = LMSRepo{}

func one[T any](row T, err error) (*T, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func affected(res sql.Result, err error) (bool, error) {
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ---------- courses

const courseColumns = `course_id, tenant_id, school_id, class_id, subject_id, grade_level, academic_year_id, COALESCE(academic_year, ''),
	curriculum_version_id, textbook_series_id, status, version`

func scanCourse(row interface{ Scan(...any) error }) (write.CourseRow, error) {
	var r write.CourseRow
	var series uuid.NullUUID
	err := row.Scan(&r.ID, &r.TenantID, &r.SchoolID, &r.ClassID, &r.SubjectID, &r.Grade, &r.AcademicYearID, &r.AcademicYear,
		&r.CurriculumVersionID, &series, &r.Status, &r.Version)
	if series.Valid {
		s := series.UUID
		r.TextbookSeriesID = &s
	}
	return r, err
}

// InsertCourse implements write.LMSStore (ON CONFLICT DO NOTHING: one course per class, subject and year).
func (LMSRepo) InsertCourse(ctx context.Context, r write.CourseRow) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	var year any
	if r.AcademicYear != "" {
		year = r.AcademicYear
	}
	// tenancyguard:ok insert of the session tenant (RLS insert policy); conflict on the live unique (tenant, class, subject, year)
	ok, err := affected(tx.ExecContext(ctx, `INSERT INTO lms.courses (course_id, tenant_id, school_id, class_id, subject_id, grade_level,
		academic_year_id, academic_year, curriculum_version_id, textbook_series_id, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'active')
		ON CONFLICT (tenant_id, class_id, subject_id, academic_year_id) WHERE deleted_at IS NULL DO NOTHING`,
		r.ID, r.TenantID, r.SchoolID, r.ClassID, r.SubjectID, r.Grade, r.AcademicYearID, year, r.CurriculumVersionID, r.TextbookSeriesID))
	if err != nil {
		return false, fmt.Errorf("insert course: %w", err)
	}
	return ok, nil
}

// Course implements write.LMSStore.
func (LMSRepo) Course(ctx context.Context, id uuid.UUID) (*write.CourseRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	r, err := one(scanCourse(tx.QueryRowContext(ctx, `SELECT `+courseColumns+` FROM lms.courses
		WHERE course_id = $1 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, id)))
	if err != nil {
		return nil, fmt.Errorf("course: %w", err)
	}
	return r, nil
}

// UpdateCourseStatus implements write.LMSStore (optimistic on version).
func (LMSRepo) UpdateCourseStatus(ctx context.Context, id uuid.UUID, status string, ifVersion int64) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok tenant of the session (RLS update policy + infra.current_tenant_id()) and deleted_at IS NULL
	return affected(tx.ExecContext(ctx, `UPDATE lms.courses SET status = $2, version = version + 1, updated_at = now()
		WHERE course_id = $1 AND version = $3 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, id, status, ifVersion))
}

// ArchiveClassCourses implements write.LMSStore.
func (LMSRepo) ArchiveClassCourses(ctx context.Context, classID uuid.UUID) (int, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return 0, err
	}
	// tenancyguard:ok tenant of the session (RLS update policy + infra.current_tenant_id()) and deleted_at IS NULL
	res, err := tx.ExecContext(ctx, `UPDATE lms.courses SET status = 'archived', version = version + 1, updated_at = now()
		WHERE class_id = $1 AND status <> 'archived' AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, classID)
	if err != nil {
		return 0, fmt.Errorf("archive courses: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// ---------- curriculum of the school

// CurriculumSettings implements write.LMSStore: the live settings and the version of the settings (1 + every row
// ever written for the school: a PUT soft-deletes and inserts, so it only grows).
func (LMSRepo) CurriculumSettings(ctx context.Context, schoolID uuid.UUID) ([]write.CurriculumSettingRow, int64, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, 0, err
	}
	var version int64
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()); the version counts the deleted rows too (optimistic lock)
	if err := tx.QueryRowContext(ctx, `SELECT 1 + count(*) FROM lms.school_curriculum_settings
		WHERE school_id = $1 AND tenant_id = infra.current_tenant_id()`, schoolID).Scan(&version); err != nil {
		return nil, 0, fmt.Errorf("curriculum settings version: %w", err)
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT setting_id, tenant_id, school_id, academic_year_id, grade_level, curriculum_version_id, set_by,
		updated_at FROM lms.school_curriculum_settings WHERE school_id = $1 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL
		ORDER BY academic_year_id, grade_level NULLS FIRST`, schoolID)
	if err != nil {
		return nil, 0, fmt.Errorf("curriculum settings: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.CurriculumSettingRow
	for rows.Next() {
		var r write.CurriculumSettingRow
		var grade sql.NullInt32
		if err := rows.Scan(&r.ID, &r.TenantID, &r.SchoolID, &r.AcademicYearID, &grade, &r.CurriculumVersionID, &r.SetBy, &r.UpdatedAt); err != nil {
			return nil, 0, err
		}
		if grade.Valid {
			g := int(grade.Int32)
			r.Grade = &g
		}
		r.UpdatedAt = r.UpdatedAt.UTC()
		out = append(out, r)
	}
	return out, version, rows.Err()
}

// ReplaceCurriculumSettings implements write.LMSStore: the live settings of the years are soft-deleted, rows inserted.
func (LMSRepo) ReplaceCurriculumSettings(ctx context.Context, schoolID uuid.UUID, years []uuid.UUID, rows []write.CurriculumSettingRow,
	by uuid.UUID) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok soft-delete (R21) in the session tenant (RLS update policy + infra.current_tenant_id()), deleted_at IS NULL
	if _, err := tx.ExecContext(ctx, `UPDATE lms.school_curriculum_settings SET deleted_at = now(), deleted_by = $3, delete_reason = 'replaced',
		version = version + 1, updated_at = now()
		WHERE school_id = $1 AND academic_year_id = ANY($2::uuid[]) AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`,
		schoolID, uuidArray(years), by); err != nil {
		return fmt.Errorf("replace curriculum settings: %w", err)
	}
	for _, r := range rows {
		// tenancyguard:ok insert of the session tenant (RLS insert policy)
		if _, err := tx.ExecContext(ctx, `INSERT INTO lms.school_curriculum_settings (setting_id, tenant_id, school_id, academic_year_id,
			grade_level, curriculum_version_id, set_by) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			r.ID, r.TenantID, r.SchoolID, r.AcademicYearID, r.Grade, r.CurriculumVersionID, r.SetBy); err != nil {
			return fmt.Errorf("insert curriculum setting: %w", err)
		}
	}
	return nil
}

// SettingFor implements write.LMSStore: the setting of the grade, otherwise the one of every grade of the year.
func (LMSRepo) SettingFor(ctx context.Context, schoolID, yearID uuid.UUID, grade int) (*uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var id uuid.UUID
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	r, err := one(id, tx.QueryRowContext(ctx, `SELECT curriculum_version_id FROM lms.school_curriculum_settings
		WHERE school_id = $1 AND academic_year_id = $2 AND (grade_level = $3 OR grade_level IS NULL)
			AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL
		ORDER BY grade_level NULLS LAST LIMIT 1`, schoolID, yearID, grade).Scan(&id))
	if err != nil {
		return nil, fmt.Errorf("curriculum setting: %w", err)
	}
	if r == nil {
		return nil, nil
	}
	return &id, nil
}

// CurriculumVersion implements write.LMSStore: a curriculum version visible in the scope of ctx (tenant rows or
// assigned platform rows through RLS; in a platform scope, platform rows).
func (LMSRepo) CurriculumVersion(ctx context.Context, versionID uuid.UUID) (*write.VersionAccess, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var a write.VersionAccess
	var tenant uuid.NullUUID
	// tenancyguard:ok hybrid curriculum: RLS content_read keeps tenant rows and assigned platform rows; deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT v.curriculum_id, v.curriculum_version_id, v.status, c.tenant_id FROM curriculum.curriculum_versions v
		JOIN curriculum.curricula c ON c.curriculum_id = v.curriculum_id AND c.deleted_at IS NULL
		WHERE v.curriculum_version_id = $1 AND v.deleted_at IS NULL`, versionID).Scan(&a.ItemID, &a.VersionID, &a.Status, &tenant)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("curriculum version: %w", err)
	}
	a.Platform, a.Visible = !tenant.Valid, true
	return &a, nil
}

// LatestCurriculumVersion implements write.LMSStore: the latest published version visible to the tenant (own or
// assigned) that has subjects for the grade.
func (LMSRepo) LatestCurriculumVersion(ctx context.Context, grade int) (*uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var id uuid.UUID
	// tenancyguard:ok hybrid curriculum: RLS content_read keeps tenant rows and assigned platform rows; deleted_at IS NULL on every table
	r, err := one(id, tx.QueryRowContext(ctx, `SELECT v.curriculum_version_id FROM curriculum.curriculum_versions v
		JOIN curriculum.curricula c ON c.curriculum_id = v.curriculum_id AND c.deleted_at IS NULL
		WHERE v.status = 'published' AND v.deleted_at IS NULL AND EXISTS (SELECT 1 FROM curriculum.subject_grades g
			WHERE g.curriculum_version_id = v.curriculum_version_id AND g.grade_level = $1 AND g.deleted_at IS NULL)
		ORDER BY (c.tenant_id IS NULL), v.published_at DESC NULLS LAST, v.curriculum_version_id LIMIT 1`, grade).Scan(&id))
	if err != nil {
		return nil, fmt.Errorf("latest curriculum version: %w", err)
	}
	if r == nil {
		return nil, nil
	}
	return &id, nil
}

// SubjectsOfGrade implements write.LMSStore.
func (LMSRepo) SubjectsOfGrade(ctx context.Context, versionID uuid.UUID, grade int) ([]uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok hybrid curriculum: RLS content_read; deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT subject_id FROM curriculum.subject_grades
		WHERE curriculum_version_id = $1 AND grade_level = $2 AND deleted_at IS NULL ORDER BY subject_id`, versionID, grade)
	if err != nil {
		return nil, fmt.Errorf("subjects of grade: %w", err)
	}
	return scanIDs(rows)
}

func scanIDs(rows *sql.Rows) ([]uuid.UUID, error) {
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

// ---------- content of the nodes

// contentTables maps a node content type to its item and version tables.
var contentTables = map[string][2]string{
	"lesson":               {"lessons", "lesson"},
	"activity":             {"activities", "activity"},
	"question_set":         {"question_sets", "question_set"},
	"assessment_blueprint": {"assessment_blueprints", "assessment_blueprint"},
}

// ContentVersion implements write.LMSStore: a version of a content item visible in the scope of ctx (RLS content_read:
// tenant rows and assigned platform rows); nil when invisible or of another type.
func (LMSRepo) ContentVersion(ctx context.Context, contentType string, versionID uuid.UUID) (*write.VersionAccess, error) {
	t, ok := contentTables[contentType]
	if !ok {
		return nil, nil
	}
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	item, key := t[0], t[1]
	var a write.VersionAccess
	var tenant, subject uuid.NullUUID
	var title, los, topics []byte
	// tenancyguard:ok hybrid content: RLS content_read keeps tenant rows and assigned platform rows; deleted_at IS NULL (fixed identifiers)
	err = tx.QueryRowContext(ctx, `SELECT v.`+key+`_id, v.`+key+`_version_id, v.status, i.tenant_id, v.title, v.subject_id,
			COALESCE(v.objective_ids, '[]'::jsonb), COALESCE(v.topic_ids, '[]'::jsonb)
		FROM content.`+key+`_versions v JOIN content.`+item+` i ON i.`+key+`_id = v.`+key+`_id AND i.deleted_at IS NULL
		WHERE v.`+key+`_version_id = $1 AND v.deleted_at IS NULL`, versionID).
		Scan(&a.ItemID, &a.VersionID, &a.Status, &tenant, &title, &subject, &los, &topics)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("content version: %w", err)
	}
	a.Platform, a.Visible = !tenant.Valid, true
	if resolved, _ := i18ntext.Resolve(title, "vi", "vi"); json.Unmarshal(resolved, &a.Title) != nil {
		a.Title = ""
	}
	if rs := []rune(a.Title); len(rs) > 255 {
		a.Title = string(rs[:255])
	}
	if subject.Valid {
		s := subject.UUID
		a.SubjectID = &s
	}
	_ = json.Unmarshal(los, &a.ObjectiveIDs)
	_ = json.Unmarshal(topics, &a.TopicIDs)
	return &a, nil
}

// ---------- learning paths

const pathColumns = `path_id, tenant_id, course_id, school_id, textbook_series_id, subject_id, grade_level, title, unlock_mode, status,
	version, updated_at`

func scanPath(row interface{ Scan(...any) error }) (write.PathRow, error) {
	var p write.PathRow
	var tenant, course, school, series, subject uuid.NullUUID
	var grade sql.NullInt32
	err := row.Scan(&p.ID, &tenant, &course, &school, &series, &subject, &grade, &p.Title, &p.UnlockMode, &p.Status, &p.Version, &p.UpdatedAt)
	p.TenantID, p.CourseID, p.SchoolID, p.TextbookSeriesID, p.SubjectID = nullID(tenant), nullID(course), nullID(school), nullID(series),
		nullID(subject)
	if grade.Valid {
		g := int(grade.Int32)
		p.Grade = &g
	}
	p.UpdatedAt = p.UpdatedAt.UTC()
	return p, err
}

func nullID(n uuid.NullUUID) *uuid.UUID {
	if !n.Valid {
		return nil
	}
	id := n.UUID
	return &id
}

// pathNodes loads the live nodes of a path in position order.
func pathNodes(ctx context.Context, tx *sql.Tx, pathID uuid.UUID) ([]write.NodeRow, error) {
	// tenancyguard:ok hybrid lms.path_nodes: RLS hybrid_read (session tenant or platform template), deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT node_id, position, content_type, content_id, content_version_id, COALESCE(title, ''),
		array_to_string(objective_ids, ','), unlock_rule FROM lms.path_nodes WHERE path_id = $1 AND deleted_at IS NULL ORDER BY position`, pathID)
	if err != nil {
		return nil, fmt.Errorf("path nodes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.NodeRow
	for rows.Next() {
		var n write.NodeRow
		var los string
		if err := rows.Scan(&n.ID, &n.Position, &n.ContentType, &n.ContentID, &n.ContentVersionID, &n.Title, &los, &n.UnlockRule); err != nil {
			return nil, err
		}
		n.ObjectiveIDs = parseIDs(los)
		out = append(out, n)
	}
	return out, rows.Err()
}

func (LMSRepo) path(ctx context.Context, where string, args ...any) (*write.PathRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok hybrid lms.learning_paths: RLS hybrid_read (session tenant or platform template), deleted_at IS NULL
	p, err := one(scanPath(tx.QueryRowContext(ctx, `SELECT `+pathColumns+` FROM lms.learning_paths WHERE deleted_at IS NULL AND `+where, args...)))
	if err != nil || p == nil {
		return p, err
	}
	p.Nodes, err = pathNodes(ctx, tx, p.ID)
	return p, err
}

// ActivePath implements write.LMSStore.
func (r LMSRepo) ActivePath(ctx context.Context, courseID uuid.UUID) (*write.PathRow, error) {
	return r.path(ctx, `course_id = $1 AND status = 'active' AND tenant_id = infra.current_tenant_id()`, courseID)
}

// Path implements write.LMSStore.
func (r LMSRepo) Path(ctx context.Context, id uuid.UUID) (*write.PathRow, error) {
	return r.path(ctx, `path_id = $1 AND tenant_id = infra.current_tenant_id()`, id)
}

func rule(r json.RawMessage) []byte {
	if len(r) == 0 || string(r) == "null" {
		return []byte("{}")
	}
	return r
}

func insertNode(ctx context.Context, tx *sql.Tx, p write.PathRow, n write.NodeRow) error {
	// tenancyguard:ok insert of the session tenant (RLS insert policy)
	_, err := tx.ExecContext(ctx, `INSERT INTO lms.path_nodes (node_id, tenant_id, path_id, school_id, position, content_type, content_id,
		content_version_id, title, objective_ids, unlock_rule) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::uuid[], $11::jsonb)`,
		n.ID, p.TenantID, p.ID, p.SchoolID, n.Position, n.ContentType, n.ContentID, n.ContentVersionID, n.Title, uuidArray(n.ObjectiveIDs),
		rule(n.UnlockRule))
	if err != nil {
		return fmt.Errorf("insert path node: %w", err)
	}
	return nil
}

// InsertPath implements write.LMSStore (a second active path of a course is 409 CONFLICT).
func (LMSRepo) InsertPath(ctx context.Context, p write.PathRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok insert of the session tenant (RLS insert policy); conflict on the live active path of the course
	ok, err := affected(tx.ExecContext(ctx, `INSERT INTO lms.learning_paths (path_id, tenant_id, owner_scope, course_id, school_id,
		textbook_series_id, subject_id, grade_level, title, unlock_mode, status, created_by, updated_by)
		VALUES ($1, $2, 'tenant', $3, $4, $5, $6, $7, $8, $9, 'active', $10, $10)
		ON CONFLICT (tenant_id, course_id) WHERE deleted_at IS NULL AND status = 'active' AND course_id IS NOT NULL DO NOTHING`,
		p.ID, p.TenantID, p.CourseID, p.SchoolID, p.TextbookSeriesID, p.SubjectID, p.Grade, p.Title, p.UnlockMode, p.By))
	if err != nil {
		return fmt.Errorf("insert learning path: %w", err)
	}
	if !ok {
		return problem.New(problem.Conflict, "the course already has a learning path: update it")
	}
	for _, n := range p.Nodes {
		if err := insertNode(ctx, tx, p, n); err != nil {
			return err
		}
	}
	return nil
}

// UpdatePath implements write.LMSStore.
func (LMSRepo) UpdatePath(ctx context.Context, p write.PathRow, ifVersion int64) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok tenant of the session (RLS update policy + infra.current_tenant_id()) and deleted_at IS NULL
	ok, err := affected(tx.ExecContext(ctx, `UPDATE lms.learning_paths SET title = $2, unlock_mode = $3, updated_by = $4, version = version + 1,
		updated_at = now() WHERE path_id = $1 AND version = $5 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`,
		p.ID, p.Title, p.UnlockMode, p.By, ifVersion))
	if err != nil || !ok {
		return ok, err
	}
	old, err := pathNodes(ctx, tx, p.ID)
	if err != nil {
		return false, err
	}
	keep := map[uuid.UUID]bool{}
	for _, n := range p.Nodes {
		if n.ID != uuid.Nil {
			keep[n.ID] = true
		}
	}
	// Positions are unique on live rows: the old nodes leave (or move out of the way) before the new order is written.
	for _, n := range old {
		if !keep[n.ID] {
			// tenancyguard:ok soft-delete (R21) in the session tenant (RLS update policy + infra.current_tenant_id())
			if _, err := tx.ExecContext(ctx, `UPDATE lms.path_nodes SET deleted_at = now(), deleted_by = $2, delete_reason = 'path_updated',
				updated_at = now() WHERE node_id = $1 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, n.ID, p.By); err != nil {
				return false, fmt.Errorf("remove path node: %w", err)
			}
			continue
		}
		// tenancyguard:ok tenant of the session (RLS update policy + infra.current_tenant_id()) and deleted_at IS NULL
		if _, err := tx.ExecContext(ctx, `UPDATE lms.path_nodes SET position = position + 1000, updated_at = now()
			WHERE node_id = $1 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, n.ID); err != nil {
			return false, fmt.Errorf("move path node: %w", err)
		}
	}
	for _, n := range p.Nodes {
		if n.ID == uuid.Nil {
			n.ID = ids.New()
			if err := insertNode(ctx, tx, p, n); err != nil {
				return false, err
			}
			continue
		}
		// tenancyguard:ok tenant of the session (RLS update policy + infra.current_tenant_id()) and deleted_at IS NULL
		if _, err := tx.ExecContext(ctx, `UPDATE lms.path_nodes SET position = $2, content_version_id = $3, title = $4, objective_ids = $5::uuid[],
			unlock_rule = $6::jsonb, version = version + 1, updated_at = now()
			WHERE node_id = $1 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`,
			n.ID, n.Position, n.ContentVersionID, n.Title, uuidArray(n.ObjectiveIDs), rule(n.UnlockRule)); err != nil {
			return false, fmt.Errorf("update path node: %w", err)
		}
	}
	return true, nil
}

// InsertUnlock implements write.LMSStore (a second unlock of the same node for the student adds nothing).
func (LMSRepo) InsertUnlock(ctx context.Context, tenant, school, pathID, nodeID, studentID uuid.UUID, reason string, by uuid.UUID) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok insert of the session tenant (RLS insert policy); conflict on the live unique (tenant, node, student)
	return affected(tx.ExecContext(ctx, `INSERT INTO lms.path_node_unlocks (unlock_id, tenant_id, school_id, path_id, node_id, student_id, reason,
		unlocked_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (tenant_id, node_id, student_id) WHERE deleted_at IS NULL DO NOTHING`,
		ids.New(), tenant, school, pathID, nodeID, studentID, reason, by))
}

// ---------- schedule

const scheduleColumns = `schedule_item_id, tenant_id, school_id, course_id, to_char(date, 'YYYY-MM-DD'), period, kind, ref_type, ref_id, note,
	source_event_id, created_by, version`

func scanSchedule(row interface{ Scan(...any) error }) (write.ScheduleRow, error) {
	var r write.ScheduleRow
	var period sql.NullInt32
	var refType, note sql.NullString
	var ref, source, by uuid.NullUUID
	err := row.Scan(&r.ID, &r.TenantID, &r.SchoolID, &r.CourseID, &r.Date, &period, &r.Kind, &refType, &ref, &note, &source, &by, &r.Version)
	if period.Valid {
		p := int(period.Int32)
		r.Period = &p
	}
	if refType.Valid {
		r.RefType = &refType.String
	}
	if note.Valid {
		r.Note = &note.String
	}
	r.RefID, r.SourceEventID, r.CreatedBy = nullID(ref), nullID(source), nullID(by)
	return r, err
}

// InsertSchedule implements write.LMSStore (false when an item of the same source event exists).
func (LMSRepo) InsertSchedule(ctx context.Context, r write.ScheduleRow) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok insert of the session tenant (RLS insert policy); conflict on the live source event
	return affected(tx.ExecContext(ctx, `INSERT INTO lms.schedule_items (schedule_item_id, tenant_id, school_id, course_id, date, period, kind,
		ref_type, ref_id, note, source_event_id, created_by) VALUES ($1, $2, $3, $4, $5::date, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (tenant_id, source_event_id) WHERE deleted_at IS NULL AND source_event_id IS NOT NULL DO NOTHING`,
		r.ID, r.TenantID, r.SchoolID, r.CourseID, r.Date, r.Period, r.Kind, r.RefType, r.RefID, r.Note, r.SourceEventID, r.CreatedBy))
}

// Schedule implements write.LMSStore.
func (LMSRepo) Schedule(ctx context.Context, id uuid.UUID) (*write.ScheduleRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	return one(scanSchedule(tx.QueryRowContext(ctx, `SELECT `+scheduleColumns+` FROM lms.schedule_items
		WHERE schedule_item_id = $1 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, id)))
}

// UpdateSchedule implements write.LMSStore (optimistic on version).
func (LMSRepo) UpdateSchedule(ctx context.Context, r write.ScheduleRow, ifVersion int64) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok tenant of the session (RLS update policy + infra.current_tenant_id()) and deleted_at IS NULL
	return affected(tx.ExecContext(ctx, `UPDATE lms.schedule_items SET course_id = $2, school_id = $3, date = $4::date, period = $5, kind = $6,
		ref_type = $7, ref_id = $8, note = $9, version = version + 1, updated_at = now()
		WHERE schedule_item_id = $1 AND version = $10 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`,
		r.ID, r.CourseID, r.SchoolID, r.Date, r.Period, r.Kind, r.RefType, r.RefID, r.Note, ifVersion))
}

// DeleteSchedule implements write.LMSStore: soft-delete (ADR-0028).
func (LMSRepo) DeleteSchedule(ctx context.Context, id uuid.UUID, by uuid.UUID) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok soft-delete (R21) in the session tenant (RLS update policy + infra.current_tenant_id()), deleted_at IS NULL
	return affected(tx.ExecContext(ctx, `UPDATE lms.schedule_items SET deleted_at = now(), deleted_by = $2, delete_reason = 'user_request',
		version = version + 1, updated_at = now() WHERE schedule_item_id = $1 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`,
		id, by))
}

// ---------- study groups

// groupMembers loads the live members of a group.
func groupMembers(ctx context.Context, tx *sql.Tx, groupID uuid.UUID) ([]uuid.UUID, error) {
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT student_id FROM lms.study_group_members WHERE study_group_id = $1
		AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL ORDER BY added_at, student_id`, groupID)
	if err != nil {
		return nil, fmt.Errorf("study group members: %w", err)
	}
	return scanIDs(rows)
}

func addMembers(ctx context.Context, tx *sql.Tx, g write.StudyGroupRow, members []uuid.UUID, by uuid.UUID) error {
	now := time.Now().UTC()
	for _, m := range members {
		// tenancyguard:ok insert of the session tenant (RLS insert policy)
		if _, err := tx.ExecContext(ctx, `INSERT INTO lms.study_group_members (study_group_member_id, study_group_id, tenant_id, school_id,
			student_id, added_by, added_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`, ids.New(), g.ID, g.TenantID, g.SchoolID, m, by, now); err != nil {
			return fmt.Errorf("add study group member: %w", err)
		}
	}
	return nil
}

// isNameConflict reports the unique violation of the group name (409 CONFLICT).
func isNameConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), "study_groups_name_uq")
}

// InsertGroup implements write.LMSStore (a live group of the same name in the school is 409 CONFLICT).
func (LMSRepo) InsertGroup(ctx context.Context, g write.StudyGroupRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// The name check runs in a savepoint so a duplicate keeps the transaction usable.
	if _, err := tx.ExecContext(ctx, `SAVEPOINT study_group_insert`); err != nil {
		return err
	}
	// tenancyguard:ok insert of the session tenant (RLS insert policy)
	_, err = tx.ExecContext(ctx, `INSERT INTO lms.study_groups (study_group_id, tenant_id, school_id, class_id, grade_level, name, member_count,
		created_by, updated_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)`, g.ID, g.TenantID, g.SchoolID, g.ClassID, g.Grade, g.Name,
		len(g.Members), g.CreatedBy)
	if isNameConflict(err) {
		_, _ = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT study_group_insert`)
		return problem.New(problem.Conflict, "a study group of the school has this name")
	}
	if err != nil {
		return fmt.Errorf("insert study group: %w", err)
	}
	return addMembers(ctx, tx, g, g.Members, g.CreatedBy)
}

// Group implements write.LMSStore.
func (LMSRepo) Group(ctx context.Context, schoolID, id uuid.UUID) (*write.StudyGroupRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var g write.StudyGroupRow
	var class uuid.NullUUID
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	err = tx.QueryRowContext(ctx, `SELECT study_group_id, tenant_id, school_id, class_id, grade_level, name, created_by, updated_by, version,
		updated_at FROM lms.study_groups WHERE study_group_id = $1 AND school_id = $2 AND tenant_id = infra.current_tenant_id()
		AND deleted_at IS NULL`, id, schoolID).Scan(&g.ID, &g.TenantID, &g.SchoolID, &class, &g.Grade, &g.Name, &g.CreatedBy, &g.UpdatedBy,
		&g.Version, &g.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("study group: %w", err)
	}
	g.ClassID, g.UpdatedAt = nullID(class), g.UpdatedAt.UTC()
	g.Members, err = groupMembers(ctx, tx, g.ID)
	return &g, err
}

// UpdateGroup implements write.LMSStore: the group row (optimistic on version) and its members (added inserted,
// removed soft-deleted).
func (LMSRepo) UpdateGroup(ctx context.Context, g write.StudyGroupRow, added, removed []uuid.UUID, ifVersion int64) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `SAVEPOINT study_group_update`); err != nil {
		return false, err
	}
	// tenancyguard:ok tenant of the session (RLS update policy + infra.current_tenant_id()) and deleted_at IS NULL
	ok, err := affected(tx.ExecContext(ctx, `UPDATE lms.study_groups SET class_id = $2, grade_level = $3, name = $4, member_count = $5,
		updated_by = $6, version = version + 1, updated_at = now()
		WHERE study_group_id = $1 AND version = $7 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`,
		g.ID, g.ClassID, g.Grade, g.Name, len(g.Members), g.UpdatedBy, ifVersion))
	if isNameConflict(err) {
		_, _ = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT study_group_update`)
		return false, problem.New(problem.Conflict, "a study group of the school has this name")
	}
	if err != nil || !ok {
		return ok, err
	}
	if len(removed) > 0 {
		// tenancyguard:ok soft-delete (R21) in the session tenant (RLS update policy + infra.current_tenant_id()), deleted_at IS NULL
		if _, err := tx.ExecContext(ctx, `UPDATE lms.study_group_members SET deleted_at = now(), deleted_by = $3, delete_reason = 'removed',
			version = version + 1, updated_at = now() WHERE study_group_id = $1 AND student_id = ANY($2::uuid[])
			AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, g.ID, uuidArray(removed), g.UpdatedBy); err != nil {
			return false, fmt.Errorf("remove study group members: %w", err)
		}
	}
	return true, addMembers(ctx, tx, g, added, g.UpdatedBy)
}

// DeleteGroup implements write.LMSStore: soft-delete of the group and its members (ADR-0028).
func (LMSRepo) DeleteGroup(ctx context.Context, g write.StudyGroupRow, by uuid.UUID) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok soft-delete (R21) in the session tenant (RLS update policy + infra.current_tenant_id()), deleted_at IS NULL
	if _, err := tx.ExecContext(ctx, `UPDATE lms.study_group_members SET deleted_at = now(), deleted_by = $2, delete_reason = 'group_deleted',
		version = version + 1, updated_at = now() WHERE study_group_id = $1 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`,
		g.ID, by); err != nil {
		return fmt.Errorf("delete study group members: %w", err)
	}
	// tenancyguard:ok soft-delete (R21) in the session tenant (RLS update policy + infra.current_tenant_id()), deleted_at IS NULL
	if _, err := tx.ExecContext(ctx, `UPDATE lms.study_groups SET deleted_at = now(), deleted_by = $2, delete_reason = 'user_request',
		version = version + 1, updated_at = now() WHERE study_group_id = $1 AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`,
		g.ID, by); err != nil {
		return fmt.Errorf("delete study group: %w", err)
	}
	return nil
}

// ---------- projections

// Class implements write.LMSStore.
func (LMSRepo) Class(ctx context.Context, classID uuid.UUID) (*write.ClassRef, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var c write.ClassRef
	var year uuid.NullUUID
	// tenancyguard:ok projection ref.classes (no deleted_at) of the session tenant (RLS + infra.current_tenant_id())
	err = tx.QueryRowContext(ctx, `SELECT class_id, school_id, grade_level, academic_year_id FROM ref.classes
		WHERE class_id = $1 AND tenant_id = infra.current_tenant_id()`, classID).Scan(&c.ClassID, &c.SchoolID, &c.Grade, &year)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("class: %w", err)
	}
	c.AcademicYearID = nullID(year)
	return &c, nil
}

// ActiveStudents implements write.LMSStore.
func (LMSRepo) ActiveStudents(ctx context.Context, schoolID uuid.UUID, idList []uuid.UUID) ([]uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok projection ref.students of the session tenant (RLS + infra.current_tenant_id()) and deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, `SELECT student_id FROM ref.students WHERE student_id = ANY($2::uuid[]) AND school_id = $1
		AND status = 'active' AND tenant_id = infra.current_tenant_id() AND deleted_at IS NULL`, schoolID, uuidArray(idList))
	if err != nil {
		return nil, fmt.Errorf("active students: %w", err)
	}
	return scanIDs(rows)
}

// InClass implements write.LMSStore.
func (LMSRepo) InClass(ctx context.Context, classID uuid.UUID, idList []uuid.UUID) ([]uuid.UUID, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok projection ref.enrollments (no deleted_at, status active) of the session tenant (RLS + infra.current_tenant_id())
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT student_id FROM ref.enrollments WHERE class_id = $1 AND student_id = ANY($2::uuid[])
		AND status = 'active' AND tenant_id = infra.current_tenant_id()`, classID, uuidArray(idList))
	if err != nil {
		return nil, fmt.Errorf("students of the class: %w", err)
	}
	return scanIDs(rows)
}

// UpcomingLessons implements write.LMSStore.
func (LMSRepo) UpcomingLessons(ctx context.Context, studentID uuid.UUID, from, to string) ([]write.UpcomingContent, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant of the session on lms tables and ref.enrollments (infra.current_tenant_id() / RLS), deleted_at IS NULL; content through RLS content_read
	rows, err := tx.QueryContext(ctx, `SELECT x.content_type, x.content_id, c.subject_id, x.title, x.objectives, to_char(s.date, 'YYYY-MM-DD')
		FROM lms.schedule_items s
		JOIN lms.courses c ON c.course_id = s.course_id AND c.tenant_id = s.tenant_id AND c.status = 'active' AND c.deleted_at IS NULL
		JOIN LATERAL (
			SELECT 'lesson' AS content_type, l.lesson_id AS content_id, v.title, COALESCE(v.objective_ids, '[]'::jsonb) AS objectives
				FROM content.lessons l JOIN content.lesson_versions v ON v.lesson_version_id = l.current_version_id AND v.deleted_at IS NULL
				WHERE l.lesson_id = s.ref_id AND l.deleted_at IS NULL
			UNION ALL SELECT 'activity', a.activity_id, v.title, COALESCE(v.objective_ids, '[]'::jsonb)
				FROM content.activities a JOIN content.activity_versions v ON v.activity_version_id = a.current_version_id AND v.deleted_at IS NULL
				WHERE a.activity_id = s.ref_id AND a.deleted_at IS NULL) x ON true
		WHERE s.tenant_id = infra.current_tenant_id() AND s.deleted_at IS NULL AND s.kind = 'lesson' AND s.date BETWEEN $2::date AND $3::date
			AND c.class_id IN (SELECT e.class_id FROM ref.enrollments e WHERE e.student_id = $1 AND e.status = 'active'
				AND e.tenant_id = infra.current_tenant_id())
		ORDER BY s.date, s.schedule_item_id LIMIT 100`, studentID, from, to)
	if err != nil {
		return nil, fmt.Errorf("upcoming lessons: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.UpcomingContent
	for rows.Next() {
		var u write.UpcomingContent
		var title, los []byte
		if err := rows.Scan(&u.ContentType, &u.ContentID, &u.SubjectID, &title, &los, &u.Date); err != nil {
			return nil, err
		}
		if resolved, _ := i18ntext.Resolve(title, "vi", "vi"); json.Unmarshal(resolved, &u.Title) != nil {
			u.Title = ""
		}
		if rs := []rune(u.Title); len(rs) > 200 {
			u.Title = string(rs[:200])
		}
		_ = json.Unmarshal(los, &u.ObjectiveIDs)
		out = append(out, u)
	}
	return out, rows.Err()
}
