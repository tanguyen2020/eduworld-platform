package read

import (
	"context"
	"database/sql"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// likeEscaper escapes the LIKE wildcards of a search term (ESCAPE '\').
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func scopeTenants(ctx context.Context) []string {
	tenants := []string{}
	if s, ok := tenancy.ScopeFrom(ctx); ok && s != nil {
		for _, t := range s.TenantIDs() {
			tenants = append(tenants, t.String())
		}
	}
	return tenants
}

// GradeStudents returns the effective students (status active, live — ADR-0095 mục 2) of one grade of a school
// (listTenantGradeStudents), ordered by (display_name, student_id). The school is looked up through ent (tenant +
// soft-delete interceptors): outside the scope → NotFound. Source follows listTenantUsers: an active SIS identity link
// → sis, otherwise manual.
func (m OrgModel) GradeStudents(ctx context.Context, f read.GradeStudentFilter) (out []dto.GradeStudent, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		if _, err := cl(ctx).School.Get(ctx, f.SchoolID); err != nil {
			if ent.IsNotFound(err) {
				return problem.New(problem.NotFound, "")
			}
			return err
		}
		pattern := ""
		if f.Q != "" {
			pattern = "%" + likeEscaper.Replace(strings.ToLower(f.Q)) + "%"
		}
		var afterKey *string
		var afterID *uuid.UUID
		if f.After != nil {
			afterKey, afterID = &f.After.Key, &f.After.ID
		}
		tx, _ := entx.Tx(ctx)
		// tenancyguard:ok students of one grade: tenant_id = ANY(scope tenants) and deleted_at IS NULL on students and users
		rows, err := tx.QueryContext(ctx, `SELECT st.student_id, coalesce(u.display_name, st.full_name), st.student_code, u.username, u.status,
				u.last_login_at, st.grade_level,
				EXISTS (SELECT 1 FROM identity.sis_identity_links l WHERE l.user_id = u.user_id AND l.tenant_id = st.tenant_id
					AND l.status = 'active' AND l.deleted_at IS NULL)
			FROM org.students st
			JOIN identity.users u ON u.user_id = st.student_id AND u.deleted_at IS NULL AND u.status <> 'anonymized'
			WHERE st.tenant_id = ANY($1::uuid[]) AND st.deleted_at IS NULL AND st.status = 'active' AND st.school_id = $2 AND st.grade_level = $3
			  AND ($4::text = '' OR lower(coalesce(u.display_name, st.full_name)) LIKE $4 ESCAPE '\'
			       OR lower(u.username) LIKE $4 ESCAPE '\' OR lower(coalesce(st.student_code, '')) LIKE $4 ESCAPE '\')
			  AND ($5::text IS NULL OR (coalesce(u.display_name, st.full_name), st.student_id) > ($5::text, $6::uuid))
			ORDER BY coalesce(u.display_name, st.full_name), st.student_id LIMIT $7`,
			scopeTenants(ctx), f.SchoolID, f.Grade, pattern, afterKey, afterID, f.Limit+1)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var s dto.GradeStudent
			var code sql.NullString
			var last sql.NullTime
			var sis bool
			if err := rows.Scan(&s.StudentID, &s.DisplayName, &code, &s.Username, &s.Status, &last, &s.GradeLevel, &sis); err != nil {
				return err
			}
			if code.Valid {
				s.StudentCode = &code.String
			}
			if last.Valid {
				t := last.Time.UTC()
				s.LastLoginAt = &t
			}
			s.Source = "manual"
			if sis {
				s.Source = "sis"
			}
			out = append(out, s)
		}
		return rows.Err()
	})
	return out, err
}
