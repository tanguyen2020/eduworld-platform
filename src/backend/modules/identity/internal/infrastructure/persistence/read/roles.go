package read

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

var _ read.RoleReader = PrincipalModel{}

// Permissions returns the live catalog copy (tenant permissions only when tenantOnly).
func (m PrincipalModel) Permissions(ctx context.Context, tenantOnly bool) (out []read.PermissionView, version string, err error) {
	err = m.read(ctx, func(tx *sql.Tx) error {
		// tenancyguard:ok identity.permissions is global (G, no tenant_id); deleted_at filtered
		q := `SELECT permission_code, scope_kind, group_code, name_vi, name_en, catalog_version FROM identity.permissions WHERE deleted_at IS NULL`
		if tenantOnly {
			q += ` AND scope_kind = 'tenant'`
		}
		// tenancyguard:ok identity.permissions is global (G, no tenant_id)
		rows, err := tx.QueryContext(ctx, q+` ORDER BY group_code, permission_code`)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var p read.PermissionView
			var en string
			if err := rows.Scan(&p.Code, &p.ScopeKind, &p.GroupCode, &p.Name.Vi, &en, &version); err != nil {
				return err
			}
			p.Name.En = &en
			out = append(out, p)
		}
		return rows.Err()
	})
	return out, version, err
}

func roleQuery(f read.RoleFilter, extra string, args []any) (string, []any) {
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	tenants := "r.tenant_id IS NULL"
	if f.AnyTenant {
		tenants = "TRUE"
	} else if len(f.TenantIDs) > 0 {
		tenants = "(r.tenant_id IS NULL OR r.tenant_id = ANY(" + arg(uuidArray(f.TenantIDs)) + "::uuid[]))"
	}
	conds := []string{"r.deleted_at IS NULL", tenants}
	if f.ScopeKind != "" {
		conds = append(conds, "(r.tenant_id IS NOT NULL OR r.role_scope_kind = "+arg(f.ScopeKind)+")")
	}
	active := "TRUE"
	if f.ActiveIn != nil {
		active = "ur.tenant_id = " + arg(*f.ActiveIn)
	}
	if extra != "" {
		conds = append(conds, extra)
	}
	return `SELECT r.role_id, r.code, r.name_vi, r.name_en, r.description, r.role_scope_kind, r.is_system, r.tenant_id, r.version, r.updated_at,
			coalesce((SELECT string_agg(rp.permission_code, ',' ORDER BY rp.permission_code) FROM identity.role_permissions rp
				WHERE rp.role_id = r.role_id AND rp.deleted_at IS NULL), ''),
			(SELECT count(*) FROM identity.user_roles ur WHERE ur.role_id = r.role_id AND ur.deleted_at IS NULL
				AND (ur.valid_to IS NULL OR ur.valid_to > now()) AND ` + active + `)
		FROM identity.roles r WHERE ` + strings.Join(conds, " AND "), args
}

func scanRoles(rows *sql.Rows) ([]read.RoleView, error) {
	var out []read.RoleView
	for rows.Next() {
		var v read.RoleView
		var en, desc sql.NullString
		var tid uuid.NullUUID
		var perms string
		if err := rows.Scan(&v.RoleID, &v.Code, &v.Name.Vi, &en, &desc, &v.ScopeKind, &v.IsSystem, &tid, &v.Version, &v.UpdatedAt, &perms,
			&v.ActiveUserRoles); err != nil {
			return nil, err
		}
		if en.Valid {
			v.Name.En = &en.String
		}
		if desc.Valid {
			v.Description = &desc.String
		}
		v.TenantID = nullID(tid)
		v.PermissionCodes = []string{}
		if perms != "" {
			v.PermissionCodes = strings.Split(perms, ",")
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Roles lists live roles of the filter, newest first.
func (m PrincipalModel) Roles(ctx context.Context, f read.RoleFilter) (out []read.RoleView, err error) {
	err = m.read(ctx, func(tx *sql.Tx) error {
		extra, args := "", []any{}
		if f.AfterID != nil {
			args = append(args, *f.AfterID)
			extra = "r.role_id < $1"
		}
		q, args := roleQuery(f, extra, args)
		args = append(args, f.Limit+1)
		// tenancyguard:ok shared roles (tenant_id NULL) or the filter's tenants; deleted_at filtered
		rows, err := tx.QueryContext(ctx, q+` ORDER BY r.role_id DESC LIMIT $`+strconv.Itoa(len(args)), args...)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		out, err = scanRoles(rows)
		return err
	})
	return out, err
}

// Role returns one live role visible with the filter.
func (m PrincipalModel) Role(ctx context.Context, id uuid.UUID, f read.RoleFilter) (out read.RoleView, err error) {
	err = m.read(ctx, func(tx *sql.Tx) error {
		q, args := roleQuery(f, "r.role_id = $1", []any{id})
		// tenancyguard:ok shared roles (tenant_id NULL) or the filter's tenants; deleted_at filtered
		rows, err := tx.QueryContext(ctx, q, args...)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		list, err := scanRoles(rows)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			return problem.New(problem.NotFound, "")
		}
		out = list[0]
		return nil
	})
	return out, err
}
