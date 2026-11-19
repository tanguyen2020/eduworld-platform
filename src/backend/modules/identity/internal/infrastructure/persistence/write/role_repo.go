package write

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// RoleRepo implements write.RoleRepository with explicit SQL (shared roles have tenant_id NULL).
type RoleRepo struct{}

var _ write.RoleRepository = RoleRepo{}

func codesArray(codes []string) string { return "{" + strings.Join(codes, ",") + "}" }

// PermissionScopes returns the scope kinds of known codes.
func (RoleRepo) PermissionScopes(ctx context.Context, codes []string) (map[string]string, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok identity.permissions is global (G, no tenant_id)
	rows, err := tx.QueryContext(ctx, `SELECT permission_code, scope_kind FROM identity.permissions
		WHERE permission_code = ANY($1::varchar[]) AND deleted_at IS NULL`, codesArray(codes))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]string{}
	for rows.Next() {
		var c, k string
		if err := rows.Scan(&c, &k); err != nil {
			return nil, err
		}
		out[c] = k
	}
	return out, rows.Err()
}

// Load returns a live role with its permissions.
func (RoleRepo) Load(ctx context.Context, id uuid.UUID, tenant *uuid.UUID) (*write.RoleRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var r write.RoleRow
	var tid uuid.NullUUID
	var en, desc sql.NullString
	var perms string
	// tenancyguard:ok shared roles (tenant_id NULL) or the given tenant's roles; deleted_at filtered
	err = tx.QueryRowContext(ctx, `SELECT r.role_id, r.tenant_id, r.code, r.name_vi, r.name_en, r.description, r.role_scope_kind, r.is_system,
			r.version, r.updated_at,
			coalesce((SELECT string_agg(rp.permission_code, ',' ORDER BY rp.permission_code) FROM identity.role_permissions rp
				WHERE rp.role_id = r.role_id AND rp.deleted_at IS NULL), '')
		FROM identity.roles r WHERE r.role_id = $1 AND r.deleted_at IS NULL
		  AND ($2::uuid IS NULL OR r.tenant_id IS NULL OR r.tenant_id = $2::uuid)`, id, tenant).
		Scan(&r.ID, &tid, &r.Code, &r.NameVi, &en, &desc, &r.ScopeKind, &r.IsSystem, &r.Version, &r.UpdatedAt, &perms)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, problem.New(problem.NotFound, "")
	}
	if err != nil {
		return nil, err
	}
	if tid.Valid {
		id := tid.UUID
		r.TenantID = &id
	}
	if en.Valid {
		r.NameEn = &en.String
	}
	if desc.Valid {
		r.Description = &desc.String
	}
	if perms != "" {
		r.Permissions = strings.Split(perms, ",")
	}
	return &r, nil
}

func mapRoleErr(err error) error {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		switch pe.Code {
		case "23505":
			return problem.New(problem.Conflict, "a role with this code exists").Wrap(err)
		case "23514":
			return problem.New(problem.ValidationFailed, pe.Message).Wrap(err)
		case "42501":
			return problem.New(problem.PermissionDenied, "system roles are read-only").Wrap(err)
		}
	}
	return err
}

func grantPermissions(ctx context.Context, tx *sql.Tx, role uuid.UUID, tenant *uuid.UUID, codes []string, actor uuid.UUID) error {
	for _, c := range codes {
		// tenancyguard:ok tenant_id = the role's tenant (NULL for shared roles)
		if _, err := tx.ExecContext(ctx, `INSERT INTO identity.role_permissions (role_permission_id, tenant_id, created_at, created_by, updated_at,
			role_id, permission_code) VALUES ($1, $2::uuid, now(), $3, now(), $4, $5)`, ids.New(), tenant, actor, role, c); err != nil {
			return mapRoleErr(err)
		}
	}
	return nil
}

// Create inserts a role and its permissions.
func (RoleRepo) Create(ctx context.Context, row *write.RoleRow, actor uuid.UUID) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok tenant_id = the custom role's tenant, NULL for a shared role (platform pool)
	err = tx.QueryRowContext(ctx, `INSERT INTO identity.roles (role_id, tenant_id, created_at, created_by, updated_at, updated_by, code, name_vi,
			name_en, description, role_scope_kind, is_system) VALUES ($1, $2::uuid, now(), $3, now(), $3, $4, $5, $6, $7, $8, false)
		RETURNING version, updated_at`, row.ID, row.TenantID, actor, row.Code, row.NameVi, row.NameEn, row.Description, row.ScopeKind).
		Scan(&row.Version, &row.UpdatedAt)
	if err != nil {
		return mapRoleErr(err)
	}
	return grantPermissions(ctx, tx, row.ID, row.TenantID, row.Permissions, actor)
}

// Update changes a role and its permissions.
func (RoleRepo) Update(ctx context.Context, row *write.RoleRow, add, remove []string, actor uuid.UUID) (int64, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return 0, err
	}
	var v int64
	// tenancyguard:ok the role was loaded in the scope; optimistic version
	err = tx.QueryRowContext(ctx, `UPDATE identity.roles SET name_vi = $3, name_en = $4, description = $5, version = version + 1, updated_at = now(),
		updated_by = $6 WHERE role_id = $1 AND version = $2 AND deleted_at IS NULL RETURNING version`,
		row.ID, row.Version, row.NameVi, row.NameEn, row.Description, actor).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, problem.New(problem.VersionMismatch, "")
	}
	if err != nil {
		return 0, mapRoleErr(err)
	}
	if len(remove) > 0 {
		// tenancyguard:ok permissions of a role loaded in the scope
		if _, err := tx.ExecContext(ctx, `UPDATE identity.role_permissions SET deleted_at = now(), deleted_by = $3, delete_reason = 'role_updated',
			updated_at = now() WHERE role_id = $1 AND permission_code = ANY($2::varchar[]) AND deleted_at IS NULL`, row.ID, codesArray(remove), actor); err != nil {
			return 0, mapRoleErr(err)
		}
	}
	return v, grantPermissions(ctx, tx, row.ID, row.TenantID, add, actor)
}

// ActiveUserRoles counts the valid user roles of a role.
func (RoleRepo) ActiveUserRoles(ctx context.Context, id uuid.UUID) (int, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	// tenancyguard:ok every grant of the role (a custom role is only granted in its tenant); deleted_at filtered
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM identity.user_roles WHERE role_id = $1 AND deleted_at IS NULL
		AND (valid_to IS NULL OR valid_to > now())`, id).Scan(&n)
	return n, err
}

// SoftDelete deletes a role (ADR-0028).
func (RoleRepo) SoftDelete(ctx context.Context, id uuid.UUID, version int64, actor uuid.UUID, reason string) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok the role was loaded in the scope; optimistic version
	res, err := tx.ExecContext(ctx, `UPDATE identity.roles SET deleted_at = now(), deleted_by = $3, delete_reason = $4, version = version + 1,
		updated_at = now() WHERE role_id = $1 AND version = $2 AND deleted_at IS NULL`, id, version, actor, reason)
	if err != nil {
		return mapRoleErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return problem.New(problem.VersionMismatch, "")
	}
	return nil
}
