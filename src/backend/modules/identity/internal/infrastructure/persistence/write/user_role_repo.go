package write

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// UserRoleRepo implements write.UserRoleRepository with explicit SQL: shared roles (tenant_id NULL) are
// invisible to the ent T* filter under a TenantScope, so every statement carries its own tenant condition.
type UserRoleRepo struct{}

var _ write.UserRoleRepository = UserRoleRepo{}

func txOf(ctx context.Context) (*sql.Tx, error) {
	tx, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	return tx, nil
}

// Role returns a shared role or a custom role of tenant with its live permissions.
func (UserRoleRepo) Role(ctx context.Context, roleID uuid.UUID, tenant *uuid.UUID) (*write.RoleInfo, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var r write.RoleInfo
	var tid uuid.NullUUID
	var perms string
	// tenancyguard:ok shared roles (tenant_id NULL) or the custom roles of the given tenant; deleted_at filtered
	err = tx.QueryRowContext(ctx, `SELECT r.role_id, r.tenant_id, r.code, r.role_scope_kind, r.is_system,
			coalesce((SELECT string_agg(rp.permission_code, ',') FROM identity.role_permissions rp
				JOIN identity.permissions p ON p.permission_code = rp.permission_code AND p.deleted_at IS NULL
				WHERE rp.role_id = r.role_id AND rp.deleted_at IS NULL), '')
		FROM identity.roles r
		WHERE r.role_id = $1 AND r.deleted_at IS NULL AND (r.tenant_id IS NULL OR r.tenant_id = $2::uuid)`, roleID, tenant).
		Scan(&r.ID, &tid, &r.Code, &r.ScopeKind, &r.IsSystem, &perms)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if tid.Valid {
		id := tid.UUID
		r.TenantID = &id
	}
	if perms != "" {
		r.Permissions = strings.Split(perms, ",")
	}
	return &r, nil
}

// Target returns a live user.
func (UserRoleRepo) Target(ctx context.Context, userID uuid.UUID) (*write.GrantTarget, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var t write.GrantTarget
	var home uuid.NullUUID
	// tenancyguard:ok identity.users is global (G, no tenant_id)
	err = tx.QueryRowContext(ctx, `SELECT user_type, primary_tenant_id FROM identity.users WHERE user_id = $1 AND deleted_at IS NULL`, userID).
		Scan(&t.UserType, &home)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if home.Valid {
		id := home.UUID
		t.PrimaryTenantID = &id
	}
	return &t, nil
}

// SchoolInTenant checks a live school of the tenant.
func (UserRoleRepo) SchoolInTenant(ctx context.Context, schoolID, tenantID uuid.UUID) (bool, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return false, err
	}
	var ok bool
	// tenancyguard:ok explicit tenant and deleted_at conditions
	err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM org.schools WHERE school_id = $1 AND tenant_id = $2 AND deleted_at IS NULL)`,
		schoolID, tenantID).Scan(&ok)
	return ok, err
}

// Insert stores a user role.
func (UserRoleRepo) Insert(ctx context.Context, r *write.UserRoleRow) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok tenant_id is the scope tenant (NULL for platform roles, checked by RLS and the trigger)
	err = tx.QueryRowContext(ctx, `INSERT INTO identity.user_roles (user_role_id, tenant_id, created_at, created_by, updated_at, user_id, role_id,
			scope_type, scope_id, school_id, valid_from, valid_to, granted_by, source, training_completed_at)
		VALUES ($1, $2::uuid, now(), $3, now(), $4, $5, $6, $7::uuid, $8::uuid, $9, $10, $3, 'manual', $11) RETURNING created_at`,
		r.ID, r.TenantID, r.GrantedBy, r.UserID, r.RoleID, r.ScopeType, r.ScopeID, r.SchoolID, r.ValidFrom, r.ValidTo, r.Training).Scan(&r.CreatedAt)
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		switch pe.Code {
		case "23514": // check_violation: the layer-2 trigger or a CHECK
			return problem.New(problem.ValidationFailed, pe.Message).Wrap(err)
		case "23505":
			return problem.New(problem.Conflict, "the user already holds this role at this scope").Wrap(err)
		}
	}
	return err
}

// Get returns a live user role visible in the scope.
func (UserRoleRepo) Get(ctx context.Context, id uuid.UUID) (*write.UserRoleRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	var r write.UserRoleRow
	var tid, scope, school uuid.NullUUID
	var validTo, training sql.NullTime
	// tenancyguard:ok RLS limits the app role to its tenant; the platform pool sees platform rows; deleted_at filtered
	err = tx.QueryRowContext(ctx, `SELECT ur.user_role_id, ur.user_id, ur.role_id, r.code, ur.tenant_id, ur.scope_type, ur.scope_id, ur.school_id,
			ur.valid_from, ur.valid_to, ur.source, ur.granted_by, ur.training_completed_at, ur.created_at
		FROM identity.user_roles ur JOIN identity.roles r ON r.role_id = ur.role_id
		WHERE ur.user_role_id = $1 AND ur.deleted_at IS NULL`, id).
		Scan(&r.ID, &r.UserID, &r.RoleID, &r.RoleCode, &tid, &r.ScopeType, &scope, &school, &r.ValidFrom, &validTo, &r.Source, &r.GrantedBy,
			&training, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, problem.New(problem.NotFound, "")
	}
	if err != nil {
		return nil, err
	}
	if tid.Valid {
		v := tid.UUID
		r.TenantID = &v
	}
	if scope.Valid {
		v := scope.UUID
		r.ScopeID = &v
	}
	if school.Valid {
		v := school.UUID
		r.SchoolID = &v
	}
	if validTo.Valid {
		v := validTo.Time
		r.ValidTo = &v
	}
	if training.Valid {
		v := training.Time
		r.Training = &v
	}
	return &r, nil
}

// Revoke soft-deletes the user role (ADR-0028).
func (UserRoleRepo) Revoke(ctx context.Context, id, by uuid.UUID, reason string) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok RLS limits the app role to its tenant; deleted_at filtered
	res, err := tx.ExecContext(ctx, `UPDATE identity.user_roles SET deleted_at = now(), deleted_by = $2, delete_reason = $3, updated_at = now(),
		updated_by = $2, version = version + 1 WHERE user_role_id = $1 AND deleted_at IS NULL`, id, by, reason)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return problem.New(problem.NotFound, "")
	}
	return nil
}

// BumpUserVersion increments the Principal version of the user.
func (UserRoleRepo) BumpUserVersion(ctx context.Context, userID uuid.UUID) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok identity.users is global (G, no tenant_id)
	_, err = tx.ExecContext(ctx, `UPDATE identity.users SET version = version + 1, updated_at = now() WHERE user_id = $1 AND deleted_at IS NULL`, userID)
	return err
}

var _ write.UserRoleExpiry = UserRoleRepo{}

// Expired lists live user roles past valid_to (platform pool: every tenant).
func (UserRoleRepo) Expired(ctx context.Context, limit int) ([]write.UserRoleRow, error) {
	tx, err := txOf(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok job user-role-expiry runs under the platform_job bypass over every tenant; deleted_at filtered
	rows, err := tx.QueryContext(ctx, `SELECT ur.user_role_id, ur.user_id, ur.role_id, r.code, ur.tenant_id, ur.scope_type, ur.scope_id, ur.valid_from,
			ur.valid_to, ur.source
		FROM identity.user_roles ur JOIN identity.roles r ON r.role_id = ur.role_id
		WHERE ur.deleted_at IS NULL AND ur.valid_to IS NOT NULL AND ur.valid_to <= now() ORDER BY ur.valid_to LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []write.UserRoleRow
	for rows.Next() {
		var r write.UserRoleRow
		var tid, scope uuid.NullUUID
		var validTo sql.NullTime
		if err := rows.Scan(&r.ID, &r.UserID, &r.RoleID, &r.RoleCode, &tid, &r.ScopeType, &scope, &r.ValidFrom, &validTo, &r.Source); err != nil {
			return nil, err
		}
		r.TenantID, r.ScopeID = nullID(tid), nullID(scope)
		if validTo.Valid {
			t := validTo.Time
			r.ValidTo = &t
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MarkExpired soft-deletes an expired user role.
func (UserRoleRepo) MarkExpired(ctx context.Context, id uuid.UUID) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok job user-role-expiry (platform_job bypass); deleted_at filtered
	_, err = tx.ExecContext(ctx, `UPDATE identity.user_roles SET deleted_at = now(), delete_reason = 'expired', updated_at = now(), version = version + 1
		WHERE user_role_id = $1 AND deleted_at IS NULL`, id)
	return err
}

func nullID(n uuid.NullUUID) *uuid.UUID {
	if !n.Valid {
		return nil
	}
	id := n.UUID
	return &id
}
