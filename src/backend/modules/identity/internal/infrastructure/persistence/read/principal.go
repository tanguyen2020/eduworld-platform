package read

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// PrincipalModel implements read.PrincipalReadModel. resolvePrincipal reads shared roles (tenant_id NULL)
// together with the user roles of one tenant, so it reads with the platform pool (platform scope of the
// identity system component "principal-resolve") and every statement carries its own explicit user,
// tenant (or tenant_id IS NULL), validity and deleted_at conditions (AGENTS R27).
type PrincipalModel struct{ Session *entx.ReadSession }

var _ read.PrincipalReadModel = PrincipalModel{}

func (m PrincipalModel) read(ctx context.Context, fn func(tx *sql.Tx) error) error {
	sc, err := authz.SystemPlatformScope("principal-resolve")
	if err != nil {
		return err
	}
	return m.Session.Read(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		tx, ok := entx.Tx(ctx)
		if !ok {
			return entx.ErrNoTransaction
		}
		return fn(tx)
	})
}

func scanUser(row *sql.Row) (*read.PrincipalUser, error) {
	var u read.PrincipalUser
	var primary uuid.NullUUID
	err := row.Scan(&u.UserID, &u.UserType, &u.Status, &primary, &u.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if primary.Valid {
		id := primary.UUID
		u.PrimaryTenantID = &id
	}
	return &u, nil
}

// UserBySubject finds the user of an active external identity.
func (m PrincipalModel) UserBySubject(ctx context.Context, issuer, subject string) (out *read.PrincipalUser, err error) {
	err = m.read(ctx, func(tx *sql.Tx) error {
		// tenancyguard:ok identity.users and identity.external_identities are global (G, no tenant_id)
		out, err = scanUser(tx.QueryRowContext(ctx, `SELECT u.user_id, u.user_type, u.status, u.primary_tenant_id, u.version
			FROM identity.external_identities e JOIN identity.users u ON u.user_id = e.user_id AND u.deleted_at IS NULL
			WHERE e.issuer = $1 AND e.subject = $2 AND e.status = 'active' AND e.deleted_at IS NULL`, issuer, subject))
		return err
	})
	return out, err
}

// UserByID returns a live user.
func (m PrincipalModel) UserByID(ctx context.Context, userID uuid.UUID) (out *read.PrincipalUser, err error) {
	err = m.read(ctx, func(tx *sql.Tx) error {
		// tenancyguard:ok identity.users is global (G, no tenant_id)
		out, err = scanUser(tx.QueryRowContext(ctx, `SELECT user_id, user_type, status, primary_tenant_id, version
			FROM identity.users WHERE user_id = $1 AND deleted_at IS NULL`, userID))
		return err
	})
	return out, err
}

// Membership returns the membership of the user in an active tenant with the student's placement.
func (m PrincipalModel) Membership(ctx context.Context, userID, tenantID uuid.UUID) (out *read.PrincipalMembership, err error) {
	err = m.read(ctx, func(tx *sql.Tx) error {
		var kind string
		var mem read.PrincipalMembership
		// tenancyguard:ok explicit tenant condition; the view filters deleted_at of its sources
		err := tx.QueryRowContext(ctx, `SELECT t.tenant_type, m.member_kind FROM identity.tenant_memberships_v m
			JOIN platform.tenants t ON t.tenant_id = m.tenant_id AND t.status = 'active' AND t.deleted_at IS NULL
			WHERE m.user_id = $1 AND m.tenant_id = $2 LIMIT 1`, userID, tenantID).Scan(&mem.TenantType, &kind)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if kind == "student" {
			if err := studentPlacement(ctx, tx, userID, tenantID, &mem); err != nil {
				return err
			}
		}
		out = &mem
		return nil
	})
	return out, err
}

func studentPlacement(ctx context.Context, tx *sql.Tx, userID, tenantID uuid.UUID, mem *read.PrincipalMembership) error {
	// tenancyguard:ok explicit tenant and deleted_at conditions
	rows, err := tx.QueryContext(ctx, `SELECT school_id, class_id FROM org.enrollments
		WHERE student_id = $1 AND tenant_id = $2 AND status = 'active' AND deleted_at IS NULL`, userID, tenantID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var school, class uuid.UUID
		if err := rows.Scan(&school, &class); err != nil {
			return err
		}
		mem.SchoolIDs = append(mem.SchoolIDs, school)
		mem.ClassIDs = append(mem.ClassIDs, class)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	var grade sql.NullInt64
	// tenancyguard:ok explicit tenant and deleted_at conditions
	err = tx.QueryRowContext(ctx, `SELECT grade_level FROM org.students WHERE student_id = $1 AND tenant_id = $2 AND deleted_at IS NULL`,
		userID, tenantID).Scan(&grade)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if grade.Valid {
		g := int(grade.Int64)
		mem.GradeLevel = &g
	}
	var conn uuid.NullUUID
	var ext sql.NullString
	// tenancyguard:ok explicit tenant and deleted_at conditions
	err = tx.QueryRowContext(ctx, `SELECT sis_connection_id, student_external_id FROM identity.sis_identity_links
		WHERE user_id = $1 AND tenant_id = $2 AND status = 'active' AND deleted_at IS NULL`, userID, tenantID).Scan(&conn, &ext)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return err
	default:
		id, e := conn.UUID, ext.String
		mem.SisConnectionID, mem.StudentExternalID = &id, &e
	}
	return nil
}

// Grants returns the valid user roles of the user in the tenant (nil: platform user roles) with the live
// permissions of their roles.
func (m PrincipalModel) Grants(ctx context.Context, userID uuid.UUID, tenantID *uuid.UUID) (out []read.PrincipalGrantRow, err error) {
	err = m.read(ctx, func(tx *sql.Tx) error {
		cond, args := "ur.tenant_id IS NULL", []any{userID}
		if tenantID != nil {
			cond, args = "ur.tenant_id = $2", append(args, *tenantID)
		}
		// tenancyguard:ok explicit tenant (or tenant_id IS NULL), validity and deleted_at conditions
		rows, err := tx.QueryContext(ctx, `SELECT r.code, r.role_scope_kind, ur.scope_type, ur.scope_id, ur.school_id,
				coalesce(string_agg(p.permission_code, ',' ORDER BY p.permission_code), '')
			FROM identity.user_roles ur
			JOIN identity.roles r ON r.role_id = ur.role_id AND r.deleted_at IS NULL AND (r.tenant_id IS NULL OR r.tenant_id = ur.tenant_id)
			LEFT JOIN identity.role_permissions rp ON rp.role_id = r.role_id AND rp.deleted_at IS NULL
			LEFT JOIN identity.permissions p ON p.permission_code = rp.permission_code AND p.deleted_at IS NULL
			WHERE ur.user_id = $1 AND `+cond+` AND ur.deleted_at IS NULL
			  AND ur.valid_from <= now() AND (ur.valid_to IS NULL OR ur.valid_to > now())
			GROUP BY ur.user_role_id, r.code, r.role_scope_kind, ur.scope_type, ur.scope_id, ur.school_id
			ORDER BY r.code`, args...)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var g read.PrincipalGrantRow
			var scope, school uuid.NullUUID
			var perms string
			if err := rows.Scan(&g.RoleCode, &g.RoleScopeKind, &g.ScopeType, &scope, &school, &perms); err != nil {
				return err
			}
			if scope.Valid {
				id := scope.UUID
				g.ScopeID = &id
			}
			if school.Valid {
				id := school.UUID
				g.SchoolID = &id
			}
			if perms != "" {
				g.Permissions = strings.Split(perms, ",")
			}
			out = append(out, g)
		}
		return rows.Err()
	})
	return out, err
}

// ActiveTenantType returns the type of an active tenant.
func (m PrincipalModel) ActiveTenantType(ctx context.Context, tenantID uuid.UUID) (out *string, err error) {
	err = m.read(ctx, func(tx *sql.Tx) error {
		var tt string
		// tenancyguard:ok platform.tenants is keyed by tenant_id (explicit condition)
		err := tx.QueryRowContext(ctx, `SELECT tenant_type FROM platform.tenants WHERE tenant_id = $1 AND status = 'active' AND deleted_at IS NULL`,
			tenantID).Scan(&tt)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err == nil {
			out = &tt
		}
		return err
	})
	return out, err
}

var _ read.MembershipChecker = PrincipalModel{}

// CheckMembers checks users against the tenant (and school/class of an active enrollment, or a school-scoped
// valid user role for staff). Unknown users are returned as non-members.
func (m PrincipalModel) CheckMembers(ctx context.Context, tenantID uuid.UUID, userIDs []uuid.UUID, schoolID, classID *uuid.UUID) (out []read.MemberRow, err error) {
	err = m.read(ctx, func(tx *sql.Tx) error {
		ids := make([]string, 0, len(userIDs))
		for _, id := range userIDs {
			ids = append(ids, id.String())
		}
		// tenancyguard:ok explicit tenant and deleted_at conditions in every branch
		rows, err := tx.QueryContext(ctx, `SELECT u.user_id, u.user_type, u.status, s.grade_level,
				EXISTS (SELECT 1 FROM identity.tenant_memberships_v m WHERE m.user_id = u.user_id AND m.tenant_id = $2)
				AND ($3::uuid IS NULL OR EXISTS (
					SELECT 1 FROM org.enrollments e WHERE e.student_id = u.user_id AND e.tenant_id = $2 AND e.school_id = $3
					  AND e.status = 'active' AND e.deleted_at IS NULL
					UNION ALL
					SELECT 1 FROM identity.user_roles ur WHERE ur.user_id = u.user_id AND ur.tenant_id = $2 AND ur.deleted_at IS NULL
					  AND ur.valid_from <= now() AND (ur.valid_to IS NULL OR ur.valid_to > now())
					  AND (ur.scope_type = 'tenant' OR ur.school_id = $3 OR ur.scope_id = $3)))
				AND ($4::uuid IS NULL OR EXISTS (
					SELECT 1 FROM org.enrollments e WHERE e.student_id = u.user_id AND e.tenant_id = $2 AND e.class_id = $4
					  AND e.status = 'active' AND e.deleted_at IS NULL))
			FROM identity.users u
			LEFT JOIN org.students s ON s.student_id = u.user_id AND s.tenant_id = $2 AND s.deleted_at IS NULL
			WHERE u.user_id = ANY($1::uuid[]) AND u.deleted_at IS NULL`, "{"+strings.Join(ids, ",")+"}", tenantID, schoolID, classID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		found := map[uuid.UUID]read.MemberRow{}
		for rows.Next() {
			var r read.MemberRow
			var grade sql.NullInt64
			if err := rows.Scan(&r.UserID, &r.UserType, &r.Status, &grade, &r.IsMember); err != nil {
				return err
			}
			if grade.Valid {
				g := int(grade.Int64)
				r.GradeLevel = &g
			}
			found[r.UserID] = r
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for _, id := range userIDs {
			if r, ok := found[id]; ok {
				out = append(out, r)
			} else {
				out = append(out, read.MemberRow{UserID: id})
			}
		}
		return nil
	})
	return out, err
}

var _ read.MeReadModel = PrincipalModel{}

// MyTenants returns the memberships of one user in every active tenant (the user's own data on the login
// path; explicit user condition) with its valid tenant role codes.
func (m PrincipalModel) MyTenants(ctx context.Context, userID uuid.UUID) (out []read.MyTenantRow, platform bool, err error) {
	err = m.read(ctx, func(tx *sql.Tx) error {
		// tenancyguard:ok the session user's own memberships across tenants (ADR-0067); explicit user, status and deleted_at conditions
		rows, err := tx.QueryContext(ctx, `SELECT t.tenant_id, t.code, t.name, t.tenant_type, t.status,
				coalesce((SELECT string_agg(DISTINCT r.code, ',') FROM identity.user_roles ur
					JOIN identity.roles r ON r.role_id = ur.role_id AND r.deleted_at IS NULL
					WHERE ur.user_id = $1 AND ur.tenant_id = t.tenant_id AND ur.deleted_at IS NULL
					  AND ur.valid_from <= now() AND (ur.valid_to IS NULL OR ur.valid_to > now())), '')
			FROM (SELECT DISTINCT tenant_id FROM identity.tenant_memberships_v WHERE user_id = $1) m
			JOIN platform.tenants t ON t.tenant_id = m.tenant_id AND t.status = 'active' AND t.deleted_at IS NULL
			ORDER BY t.name, t.tenant_id`, userID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var r read.MyTenantRow
			var roles string
			if err := rows.Scan(&r.TenantID, &r.Code, &r.Name, &r.TenantType, &r.Status, &roles); err != nil {
				return err
			}
			if roles != "" {
				r.RoleCodes = strings.Split(roles, ",")
			}
			out = append(out, r)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		// tenancyguard:ok platform user roles have tenant_id NULL; explicit user, validity and deleted_at conditions
		return tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM identity.user_roles WHERE user_id = $1 AND tenant_id IS NULL
			AND deleted_at IS NULL AND valid_from <= now() AND (valid_to IS NULL OR valid_to > now()))`, userID).Scan(&platform)
	})
	return out, platform, err
}

// DisplayName returns the display name of a live user.
func (m PrincipalModel) DisplayName(ctx context.Context, userID uuid.UUID) (out string, err error) {
	err = m.read(ctx, func(tx *sql.Tx) error {
		var name sql.NullString
		// tenancyguard:ok identity.users is global (G, no tenant_id)
		err := tx.QueryRowContext(ctx, `SELECT display_name FROM identity.users WHERE user_id = $1 AND deleted_at IS NULL`, userID).Scan(&name)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		out = name.String
		return err
	})
	return out, err
}

var _ read.UserRoleReader = PrincipalModel{}

// UserRoles lists live user roles of the filter (explicit tenant or tenant_id IS NULL condition).
func (m PrincipalModel) UserRoles(ctx context.Context, f read.UserRoleFilter) (out []read.UserRoleView, err error) {
	if !f.Platform && len(f.TenantIDs) == 0 {
		return nil, errors.New("read: user roles need tenants or the platform")
	}
	err = m.read(ctx, func(tx *sql.Tx) error {
		conds, args := []string{"ur.deleted_at IS NULL"}, []any{}
		arg := func(v any) string {
			args = append(args, v)
			return "$" + strconv.Itoa(len(args))
		}
		if f.Platform {
			conds = append(conds, "ur.tenant_id IS NULL")
		} else {
			conds = append(conds, "ur.tenant_id = ANY("+arg(uuidArray(f.TenantIDs))+"::uuid[])")
		}
		if f.SchoolIDs != nil {
			conds = append(conds, "ur.school_id = ANY("+arg(uuidArray(f.SchoolIDs))+"::uuid[])")
		}
		if f.UserID != nil {
			conds = append(conds, "ur.user_id = "+arg(*f.UserID))
		}
		if f.RoleID != nil {
			conds = append(conds, "ur.role_id = "+arg(*f.RoleID))
		}
		if f.ScopeType != "" {
			conds = append(conds, "ur.scope_type = "+arg(f.ScopeType))
		}
		if f.ScopeID != nil {
			conds = append(conds, "ur.scope_id = "+arg(*f.ScopeID))
		}
		if f.AfterID != nil {
			conds = append(conds, "ur.user_role_id < "+arg(*f.AfterID))
		}
		// tenancyguard:ok explicit tenant (or tenant_id IS NULL) and deleted_at conditions built above
		rows, err := tx.QueryContext(ctx, `SELECT ur.user_role_id, ur.user_id, ur.role_id, r.code, ur.tenant_id, ur.scope_type, ur.scope_id,
				ur.valid_from, ur.valid_to, ur.source, ur.granted_by, ur.created_at
			FROM identity.user_roles ur JOIN identity.roles r ON r.role_id = ur.role_id
			WHERE `+strings.Join(conds, " AND ")+` ORDER BY ur.user_role_id DESC LIMIT `+arg(f.Limit+1), args...)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var v read.UserRoleView
			var tid, scope, by uuid.NullUUID
			var validTo sql.NullTime
			if err := rows.Scan(&v.UserRoleID, &v.UserID, &v.RoleID, &v.RoleCode, &tid, &v.ScopeType, &scope, &v.ValidFrom, &validTo, &v.Source,
				&by, &v.CreatedAt); err != nil {
				return err
			}
			v.TenantID, v.ScopeID, v.GrantedBy = nullID(tid), nullID(scope), nullID(by)
			if validTo.Valid {
				t := validTo.Time
				v.ValidTo = &t
			}
			out = append(out, v)
		}
		return rows.Err()
	})
	return out, err
}

func uuidArray(ids []uuid.UUID) string {
	s := make([]string, 0, len(ids))
	for _, id := range ids {
		s = append(s, id.String())
	}
	return "{" + strings.Join(s, ",") + "}"
}

func nullID(n uuid.NullUUID) *uuid.UUID {
	if !n.Valid {
		return nil
	}
	id := n.UUID
	return &id
}
