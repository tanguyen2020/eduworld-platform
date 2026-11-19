package read

import (
	"context"
	"database/sql"
	"strings"

	"github.com/google/uuid"

	entclass "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/class"
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

// ClassStudents returns the students with an active enrollment in a class of the school (listTenantClassStudents),
// ordered by (display_name, student_id). The class is looked up through ent (tenant + soft-delete
// interceptors): outside the school or the scope → NotFound. Source follows listTenantUsers: an active SIS
// identity link → sis, otherwise manual.
func (m OrgModel) ClassStudents(ctx context.Context, f read.ClassStudentFilter) (out []dto.ClassStudent, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		ok, err := cl(ctx).Class.Query().Where(entclass.ID(f.ClassID), entclass.SchoolID(f.SchoolID)).Exist(ctx)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.NotFound, "")
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
		// tenancyguard:ok roster of one class: tenant_id = ANY(scope tenants) and deleted_at IS NULL on enrollments, students and users
		rows, err := tx.QueryContext(ctx, `SELECT st.student_id, coalesce(u.display_name, st.full_name), st.student_code, u.username, u.status,
				u.last_login_at,
				EXISTS (SELECT 1 FROM identity.sis_identity_links l WHERE l.user_id = u.user_id AND l.tenant_id = e.tenant_id
					AND l.status = 'active' AND l.deleted_at IS NULL)
			FROM org.enrollments e
			JOIN org.students st ON st.student_id = e.student_id AND st.tenant_id = e.tenant_id AND st.deleted_at IS NULL
			JOIN identity.users u ON u.user_id = st.student_id AND u.deleted_at IS NULL AND u.status <> 'anonymized'
			WHERE e.tenant_id = ANY($1::uuid[]) AND e.deleted_at IS NULL AND e.status = 'active' AND e.class_id = $2 AND e.school_id = $3
			  AND ($4::text = '' OR lower(coalesce(u.display_name, st.full_name)) LIKE $4 ESCAPE '\'
			       OR lower(u.username) LIKE $4 ESCAPE '\' OR lower(coalesce(st.student_code, '')) LIKE $4 ESCAPE '\')
			  AND ($5::text IS NULL OR (coalesce(u.display_name, st.full_name), st.student_id) > ($5::text, $6::uuid))
			ORDER BY coalesce(u.display_name, st.full_name), st.student_id LIMIT $7`,
			scopeTenants(ctx), f.ClassID, f.SchoolID, pattern, afterKey, afterID, f.Limit+1)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var s dto.ClassStudent
			var code sql.NullString
			var last sql.NullTime
			var sis bool
			if err := rows.Scan(&s.StudentID, &s.DisplayName, &code, &s.Username, &s.Status, &last, &sis); err != nil {
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
