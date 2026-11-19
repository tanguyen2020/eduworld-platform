package read

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// BootstrapReader implements read.BootstrapReader with explicit SQL in a READ ONLY session under the
// TenantScope of the chosen tenant (identity_app + RLS); every statement filters deleted_at and the tenant
// of the scope is enforced by RLS on org.* (AGENTS R27). users, platform.tenants and catalog.provinces are
// global.
type BootstrapReader struct {
	Session *entx.ReadSession
}

var _ read.BootstrapReader = BootstrapReader{}

// Bootstrap implements read.BootstrapReader.
func (r BootstrapReader) Bootstrap(ctx context.Context, userID uuid.UUID) (out dto.PlayerBootstrap, err error) {
	err = r.Session.Read(ctx, func(ctx context.Context) error {
		tx, ok := entx.Tx(ctx)
		if !ok {
			return entx.ErrNoTransaction
		}
		var display sql.NullString
		// tenancyguard:ok identity.users is global (G, no tenant_id); deleted_at filtered
		err := tx.QueryRowContext(ctx, `SELECT user_id, user_type, status, display_name FROM identity.users
			WHERE user_id = $1 AND deleted_at IS NULL`, userID).
			Scan(&out.User.UserID, &out.User.UserType, &out.User.Status, &display)
		if errors.Is(err, sql.ErrNoRows) {
			return problem.New(problem.AccountNotProvisioned, "")
		}
		if err != nil {
			return fmt.Errorf("bootstrap user: %w", err)
		}
		out.User.DisplayName, out.User.Roles, out.FeatureFlags = display.String, []any{}, map[string]bool{}
		// tenancyguard:ok the tenant of the scope (current_setting app.tenant_id); platform.tenants is global
		err = tx.QueryRowContext(ctx, `SELECT tenant_id, tenant_type, name FROM platform.tenants
			WHERE tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid AND deleted_at IS NULL`).
			Scan(&out.TenantID, &out.TenantType, &out.TenantName)
		if err != nil {
			return fmt.Errorf("bootstrap tenant: %w", err)
		}
		out.IsCommonTenant = out.TenantType == "common"
		return r.student(ctx, tx, userID, &out)
	})
	return out, err
}

func (r BootstrapReader) student(ctx context.Context, tx *sql.Tx, userID uuid.UUID, out *dto.PlayerBootstrap) error {
	var (
		st          dto.StudentSummary
		schoolI18n  []byte
		schoolLevel string
		virtual     bool
		provCode    sql.NullString
		provI18n    []byte
	)
	// tenancyguard:ok org.students/org.schools under the TenantScope (RLS); deleted_at filtered; catalog.provinces is global
	err := tx.QueryRowContext(ctx, `SELECT s.student_id, s.full_name, s.school_id, s.grade_level, sc.name, sc.name_i18n,
			sc.school_level, sc.is_virtual, p.code, p.name_i18n
		FROM org.students s
		JOIN org.schools sc ON sc.school_id = s.school_id AND sc.deleted_at IS NULL
		LEFT JOIN catalog.provinces p ON p.province_id = sc.province_id AND p.deleted_at IS NULL
		WHERE s.student_id = $1 AND s.deleted_at IS NULL`, userID).
		Scan(&st.StudentID, &st.DisplayName, &st.SchoolID, &st.GradeLevel, &st.SchoolName, &schoolI18n, &schoolLevel, &virtual, &provCode, &provI18n)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // no student row in this tenant: NO_ACTIVE_ENROLLMENT is decided by the caller
	}
	if err != nil {
		return fmt.Errorf("bootstrap student: %w", err)
	}
	if out.User.DisplayName != "" {
		st.DisplayName = out.User.DisplayName
	}
	school := &dto.PlayerSchool{SchoolID: st.SchoolID, Name: localized(schoolI18n, st.SchoolName), SchoolLevel: schoolLevel, IsVirtual: virtual}
	if provCode.Valid {
		n := localized(provI18n, provCode.String)
		school.Province = &dto.ProvinceRef{Code: provCode.String, Name: map[string]string{"vi": n.VI, "en": orVI(n)}}
	}
	var (
		class dto.PlayerClass
		year  sql.NullString
		grade int
	)
	// tenancyguard:ok org.enrollments/org.classes/org.academic_years under the TenantScope (RLS); deleted_at filtered
	err = tx.QueryRowContext(ctx, `SELECT c.class_id, c.name, c.grade_level, y.name
		FROM org.enrollments e
		JOIN org.classes c ON c.class_id = e.class_id AND c.deleted_at IS NULL
		LEFT JOIN org.academic_years y ON y.academic_year_id = c.academic_year_id AND y.deleted_at IS NULL
		WHERE e.student_id = $1 AND e.status = 'active' AND e.deleted_at IS NULL`, userID).
		Scan(&class.ClassID, &class.Name, &grade, &year)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return fmt.Errorf("bootstrap class: %w", err)
	default:
		class.GradeLevel, class.AcademicYear = grade, year.String
		out.Class, out.Enrolled = &class, true
		id := class.ClassID
		st.ClassID, st.ClassName, st.GradeLevel = &id, class.Name, grade
	}
	out.Student, out.School = &st, school
	sid := st.SchoolID
	out.SchoolID = &sid
	out.GradeBand = dto.GradeBand(st.GradeLevel)
	return nil
}

// localized reads a {"vi","en"} JSON name with a fallback for vi.
func localized(raw []byte, fallback string) dto.LocalizedText {
	var n dto.LocalizedText
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &n)
	}
	if n.VI == "" {
		n.VI = fallback
	}
	return n
}

// orVI is the English name, or the Vietnamese one when the catalog has none (LocalizedName requires both).
func orVI(n dto.LocalizedText) string {
	if n.EN != "" {
		return n.EN
	}
	return n.VI
}
