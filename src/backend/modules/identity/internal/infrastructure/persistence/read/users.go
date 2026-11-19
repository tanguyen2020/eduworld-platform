package read

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
)

var _ read.TenantUserReader = PrincipalModel{}

// TenantUsers lists the members of the tenant (identity.tenant_memberships_v) with their valid user roles.
func (m PrincipalModel) TenantUsers(ctx context.Context, f read.TenantUserFilter) (out []read.UserView, err error) {
	err = m.read(ctx, func(tx *sql.Tx) error {
		args := []any{f.TenantID}
		arg := func(v any) string {
			args = append(args, v)
			return "$" + strconv.Itoa(len(args))
		}
		conds := []string{"u.deleted_at IS NULL"}
		if f.Q != "" {
			p := arg("%" + strings.ToLower(f.Q) + "%")
			conds = append(conds, "(lower(u.username) LIKE "+p+" OR lower(coalesce(u.display_name, '')) LIKE "+p+")")
		}
		if f.UserType != "" {
			conds = append(conds, "u.user_type = "+arg(f.UserType))
		}
		if f.SchoolID != nil {
			s := arg(*f.SchoolID)
			conds = append(conds, `(m.school_id = `+s+` OR EXISTS (SELECT 1 FROM identity.user_roles ur WHERE ur.user_id = u.user_id
				AND ur.tenant_id = $1 AND ur.school_id = `+s+` AND ur.deleted_at IS NULL))`)
		}
		if f.RoleCode != "" {
			conds = append(conds, `EXISTS (SELECT 1 FROM identity.user_roles ur JOIN identity.roles r ON r.role_id = ur.role_id
				WHERE ur.user_id = u.user_id AND ur.tenant_id = $1 AND ur.deleted_at IS NULL AND r.code = `+arg(f.RoleCode)+`
				AND (ur.valid_to IS NULL OR ur.valid_to > now()))`)
		}
		if f.AfterID != nil {
			conds = append(conds, "u.user_id < "+arg(*f.AfterID))
		}
		// tenancyguard:ok members of the tenant through the membership view (explicit tenant condition); deleted_at filtered
		rows, err := tx.QueryContext(ctx, `SELECT u.user_id, u.user_type, u.primary_tenant_id, u.status, u.username, u.email, coalesce(u.display_name, ''),
				u.locale, u.last_login_at, u.version,
				EXISTS (SELECT 1 FROM identity.sis_identity_links l WHERE l.user_id = u.user_id AND l.tenant_id = $1 AND l.status = 'active'
					AND l.deleted_at IS NULL)
			FROM (SELECT user_id, min(school_id::text)::uuid AS school_id FROM identity.tenant_memberships_v WHERE tenant_id = $1 GROUP BY user_id) m
			JOIN identity.users u ON u.user_id = m.user_id
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
			var sis bool
			if err := rows.Scan(&v.UserID, &v.UserType, &home, &v.Status, &v.Username, &email, &v.DisplayName, &v.Locale, &last, &v.Version,
				&sis); err != nil {
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
			if sis {
				v.Source = "sis"
			}
			out = append(out, v)
		}
		return rows.Err()
	})
	if err != nil || len(out) == 0 {
		return out, err
	}
	roles, err := m.UserRoles(ctx, read.UserRoleFilter{TenantIDs: []uuid.UUID{f.TenantID}, Limit: 1000})
	if err != nil {
		return nil, err
	}
	byUser := map[uuid.UUID][]read.UserRoleView{}
	for _, r := range roles {
		byUser[r.UserID] = append(byUser[r.UserID], r)
	}
	for i := range out {
		out[i].UserRoles = byUser[out[i].UserID]
		if out[i].UserRoles == nil {
			out[i].UserRoles = []read.UserRoleView{}
		}
	}
	return out, nil
}
