package read

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
)

// CurriculumSettingsModel implements read.CurriculumSettingsReadModel with explicit SQL in the read transaction of ctx
// (TenantScope: RLS) — F021 FR-CUR-008, moved from F040 by ADR-0095.
type CurriculumSettingsModel struct{}

var _ read.CurriculumSettingsReadModel = CurriculumSettingsModel{}

func nullUUID(n uuid.NullUUID) *uuid.UUID {
	if !n.Valid {
		return nil
	}
	id := n.UUID
	return &id
}

// CurriculumSettings implements read.CurriculumSettingsReadModel.
func (CurriculumSettingsModel) CurriculumSettings(ctx context.Context, schoolID uuid.UUID, yearID *uuid.UUID) (dto.CurriculumSettingsView, error) {
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
