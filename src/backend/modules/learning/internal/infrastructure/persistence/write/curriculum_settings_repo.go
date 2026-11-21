package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/i18ntext"
)

// CurriculumSettingsRepo implements write.CurriculumSettingsStore with explicit SQL in the transaction of ctx:
// lms.school_curriculum_settings is tenant (RLS T, infra.current_tenant_id()), soft-delete only (F021 FR-CUR-008,
// moved from F040 by ADR-0095).
type CurriculumSettingsRepo struct{}

var _ write.CurriculumSettingsStore = CurriculumSettingsRepo{}

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

// ---------- curriculum of the school

// CurriculumSettings implements write.CurriculumSettingsStore: the live settings and the version of the settings (1 + every row
// ever written for the school: a PUT soft-deletes and inserts, so it only grows).
func (CurriculumSettingsRepo) CurriculumSettings(ctx context.Context, schoolID uuid.UUID) ([]write.CurriculumSettingRow, int64, error) {
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

// ReplaceCurriculumSettings implements write.CurriculumSettingsStore: the live settings of the years are soft-deleted, rows inserted.
func (CurriculumSettingsRepo) ReplaceCurriculumSettings(ctx context.Context, schoolID uuid.UUID, years []uuid.UUID, rows []write.CurriculumSettingRow,
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

// CurriculumVersion implements write.CurriculumSettingsStore: a curriculum version visible in the scope of ctx (tenant rows or
// assigned platform rows through RLS; in a platform scope, platform rows).
func (CurriculumSettingsRepo) CurriculumVersion(ctx context.Context, versionID uuid.UUID) (*write.VersionAccess, error) {
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

// ---------- content versions (assignments, exams)

// contentTables maps a content type to its item and version tables.
var contentTables = map[string][2]string{
	"lesson":               {"lessons", "lesson"},
	"activity":             {"activities", "activity"},
	"question_set":         {"question_sets", "question_set"},
	"assessment_blueprint": {"assessment_blueprints", "assessment_blueprint"},
}

// contentVersion is a version of a content item visible in the scope of ctx (RLS content_read:
// tenant rows and assigned platform rows); nil when invisible or of another type.
func contentVersion(ctx context.Context, contentType string, versionID uuid.UUID) (*write.VersionAccess, error) {
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

func nullID(n uuid.NullUUID) *uuid.UUID {
	if !n.Valid {
		return nil
	}
	id := n.UUID
	return &id
}
