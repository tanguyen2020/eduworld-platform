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

// FriendsEnabledFlag is the bootstrap feature flag of the friend features (module chat, F095-AC-23).
const FriendsEnabledFlag = "social.friends_enabled"

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
		if err := r.featureFlags(ctx, tx, &out); err != nil {
			return err
		}
		// F095-AC-23 (ADR-0078, CHILD_SAFETY §3.6): friends and friend codes are off by default in a common tenant, so the
		// client hides the buttons; a feature flag row of the same key (System Admin, tenant override) decides when present.
		if _, set := out.FeatureFlags[FriendsEnabledFlag]; !set {
			out.FeatureFlags[FriendsEnabledFlag] = !out.IsCommonTenant
		}
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
		placed      bool
	)
	// tenancyguard:ok org.students/org.schools under the TenantScope (RLS); deleted_at filtered; catalog.provinces is global
	err := tx.QueryRowContext(ctx, `SELECT s.student_id, s.full_name, s.school_id, s.grade_level, sc.name, sc.name_i18n,
			sc.school_level, sc.is_virtual, p.code, p.name_i18n, s.status = 'active'
		FROM org.students s
		JOIN org.schools sc ON sc.school_id = s.school_id AND sc.deleted_at IS NULL
		LEFT JOIN catalog.provinces p ON p.province_id = sc.province_id AND p.deleted_at IS NULL
		WHERE s.student_id = $1 AND s.deleted_at IS NULL`, userID).
		Scan(&st.StudentID, &st.DisplayName, &st.SchoolID, &st.GradeLevel, &st.SchoolName, &schoolI18n, &schoolLevel, &virtual, &provCode, &provI18n, &placed)
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
	// The grade is the grade of the student (ADR-0095 mục 2: no class); an effective student is placed.
	out.Placed = placed
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

// featureFlags fills PlayerBootstrap.feature_flags with the flags of kind feature only (menu/action flags
// concern the Admin Portal, F015-AC-29): the tenant override, else the platform default.
func (r BootstrapReader) featureFlags(ctx context.Context, tx *sql.Tx, out *dto.PlayerBootstrap) error {
	// tenancyguard:ok platform.feature_flags is global (G); deleted_at filtered; the override of the scope tenant
	rows, err := tx.QueryContext(ctx, `SELECT flag_key, coalesce((tenant_overrides ->> $1::text)::boolean, default_enabled)
		FROM platform.feature_flags WHERE flag_kind = 'feature' AND deleted_at IS NULL`, out.TenantID.String())
	if err != nil {
		return fmt.Errorf("bootstrap flags: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var k string
		var v bool
		if err := rows.Scan(&k, &v); err != nil {
			return fmt.Errorf("bootstrap flags: %w", err)
		}
		out.FeatureFlags[k] = v
	}
	return rows.Err()
}
