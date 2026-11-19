package read

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
)

var _ read.PlatformUserReader = PrincipalModel{}

// maxUserTenantIDs bounds User.tenant_ids (OpenAPI maxItems 50).
const maxUserTenantIDs = 50

// likeLiteral escapes the LIKE wildcards of a search term (used with ESCAPE '\').
func likeLiteral(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.ToLower(s))
}

// PlatformUsers lists accounts for System Admin (F059-1): platform staff (no home tenant) without TenantIDs,
// otherwise the members of TenantIDs through identity.tenant_memberships_v. Never a password, token or IP.
func (m PrincipalModel) PlatformUsers(ctx context.Context, f read.PlatformUserFilter) (out []read.UserView, err error) {
	err = m.read(ctx, func(tx *sql.Tx) error {
		var args []any
		arg := func(v any) string {
			args = append(args, v)
			return "$" + strconv.Itoa(len(args))
		}
		conds := []string{"u.deleted_at IS NULL"}
		var from, tenants, sis, roleTenant string
		if len(f.TenantIDs) > 0 {
			t := arg(uuidArray(f.TenantIDs))
			from = `(SELECT user_id, array_to_string(array_agg(DISTINCT tenant_id::text), ',') AS tenant_ids
				FROM identity.tenant_memberships_v WHERE tenant_id = ANY(` + t + `::uuid[]) GROUP BY user_id) m
				JOIN identity.users u ON u.user_id = m.user_id`
			tenants = "m.tenant_ids"
			sis = `EXISTS (SELECT 1 FROM identity.sis_identity_links l WHERE l.user_id = u.user_id AND l.tenant_id = ANY(` + t + `::uuid[])
				AND l.status = 'active' AND l.deleted_at IS NULL)`
			roleTenant = "ur.tenant_id = ANY(" + t + "::uuid[])"
		} else {
			from = "identity.users u"
			tenants, sis, roleTenant = "''", "false", "ur.tenant_id IS NULL"
			conds = append(conds, "u.primary_tenant_id IS NULL", "u.user_type = 'staff'")
		}
		if f.Q != "" {
			p := arg("%" + likeLiteral(f.Q) + "%")
			conds = append(conds, "(lower(u.username) LIKE "+p+` ESCAPE '\' OR lower(coalesce(u.email, '')) LIKE `+p+
				` ESCAPE '\' OR lower(coalesce(u.display_name, '')) LIKE `+p+` ESCAPE '\')`)
		}
		if f.UserType != "" {
			conds = append(conds, "u.user_type = "+arg(f.UserType))
		}
		if f.Status != "" {
			conds = append(conds, "u.status = "+arg(f.Status))
		}
		if f.RoleCode != "" {
			conds = append(conds, `EXISTS (SELECT 1 FROM identity.user_roles ur JOIN identity.roles r ON r.role_id = ur.role_id
				WHERE ur.user_id = u.user_id AND `+roleTenant+` AND ur.deleted_at IS NULL AND r.deleted_at IS NULL AND r.code = `+arg(f.RoleCode)+`
				AND ur.valid_from <= now() AND (ur.valid_to IS NULL OR ur.valid_to > now()))`)
		}
		if f.AfterID != nil {
			conds = append(conds, "u.user_id < "+arg(*f.AfterID))
		}
		// tenancyguard:ok platform staff (primary_tenant_id IS NULL) or members of the explicit tenant_ids (CrossTenantScope); deleted_at filtered
		rows, err := tx.QueryContext(ctx, `SELECT u.user_id, u.user_type, u.primary_tenant_id, u.status, u.username, u.email, coalesce(u.display_name, ''),
				u.locale, u.last_login_at, u.version, `+sis+`, `+tenants+`
			FROM `+from+`
			WHERE `+strings.Join(conds, " AND ")+` ORDER BY u.user_id DESC LIMIT `+arg(f.Limit+1), args...)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var v read.UserView
			var home uuid.NullUUID
			var email sql.NullString
			var last sql.NullTime
			var fromSIS bool
			var tenantList string
			if err := rows.Scan(&v.UserID, &v.UserType, &home, &v.Status, &v.Username, &email, &v.DisplayName, &v.Locale, &last, &v.Version,
				&fromSIS, &tenantList); err != nil {
				return err
			}
			v.PrimaryTenantID = nullID(home)
			if email.Valid {
				v.Email = &email.String
			}
			if last.Valid {
				t := last.Time
				v.LastLoginAt = &t
			}
			v.Source = "manual"
			if fromSIS {
				v.Source = "sis"
			}
			if len(f.TenantIDs) > 0 {
				v.TenantIDs = []uuid.UUID{}
				for _, s := range strings.Split(tenantList, ",") {
					if id, err := uuid.Parse(s); err == nil && len(v.TenantIDs) < maxUserTenantIDs {
						v.TenantIDs = append(v.TenantIDs, id)
					}
				}
			}
			v.UserRoles = []read.UserRoleView{}
			out = append(out, v)
		}
		return rows.Err()
	})
	return out, err
}
