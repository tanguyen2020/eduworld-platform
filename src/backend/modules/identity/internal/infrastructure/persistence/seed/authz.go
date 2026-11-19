// Package seed writes the code catalog of pkg/authz into the database of module identity (F012 T007,
// ADR-0066): the read-only copy identity.permissions and the system roles with their permissions. It runs
// as identity_migrator from cmd/migrate after the Atlas migrations, in one transaction, and is idempotent:
// rows are upserted with deterministic ids; permissions or role permissions that left the catalog are
// soft-deleted (ADR-0028).
package seed

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
)

// namespace of the deterministic ids of system roles and their permissions.
var namespace = uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-00000000f012")

// RoleID is the id of a system role (same in every environment).
func RoleID(code string) uuid.UUID { return uuid.NewSHA1(namespace, []byte("role:"+code)) }

func rolePermissionID(role, perm string) uuid.UUID {
	return uuid.NewSHA1(namespace, []byte("role_permission:"+role+":"+perm))
}

// Version is the catalog version stored with each permission: a hash of the sorted codes and role grants.
func Version(perms []authz.Permission, roles []authz.RoleDef) string {
	var b strings.Builder
	codes := make([]string, 0, len(perms))
	for _, p := range perms {
		codes = append(codes, string(p))
	}
	sort.Strings(codes)
	b.WriteString(strings.Join(codes, ","))
	for _, r := range roles {
		b.WriteString("|" + r.Name + ":")
		for _, p := range r.Permissions {
			b.WriteString(string(p) + ",")
		}
	}
	sum := sha256.Sum256([]byte(b.String()))
	return "sha256:" + hex.EncodeToString(sum[:])[:16]
}

// group is the workspace group of a code ("learning_content.read" → "learning_content").
func group(code string) string {
	if i := strings.IndexByte(code, '.'); i > 0 {
		return code[:i]
	}
	return code
}

func scopeKind(p authz.Permission) string {
	if p.PlatformOnly() {
		return string(authz.RoleScopePlatform)
	}
	return string(authz.RoleScopeTenant)
}

// Stats counts what a sync wrote.
type Stats struct{ Permissions, Roles, RolePermissions, Retired int }

// SyncAuthz writes the catalog and the system roles. db must connect as identity_migrator.
func SyncAuthz(ctx context.Context, db *sql.DB, perms []authz.Permission, roles []authz.RoleDef) (st Stats, err error) {
	if err := authz.ValidateCatalog(roles); err != nil {
		return st, err
	}
	version := Version(perms, roles)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return st, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	codes := make([]string, 0, len(perms))
	for _, p := range perms {
		c := string(p)
		codes = append(codes, c)
		// Names default to the code [Proposed]: the portal translates codes; no name table exists yet.
		// tenancyguard:ok identity.permissions is global (G, no tenant_id): the catalog copy written by identity_migrator
		if _, err = tx.ExecContext(ctx, `INSERT INTO identity.permissions (permission_code, created_at, updated_at, scope_kind, group_code,
			name_vi, name_en, catalog_version, synced_at) VALUES ($1, now(), now(), $2, $3, $1, $1, $4, now())
			ON CONFLICT (permission_code) DO UPDATE SET scope_kind = EXCLUDED.scope_kind, group_code = EXCLUDED.group_code,
			catalog_version = EXCLUDED.catalog_version, synced_at = now(), updated_at = now(), deleted_at = NULL, deleted_by = NULL,
			delete_reason = NULL`, c, scopeKind(p), group(c), version); err != nil {
			return st, fmt.Errorf("seed: permission %s: %w", c, err)
		}
		st.Permissions++
	}
	// tenancyguard:ok identity.permissions is global (G, no tenant_id): the catalog copy written by identity_migrator
	res, err := tx.ExecContext(ctx, `UPDATE identity.permissions SET deleted_at = now(), delete_reason = 'catalog_removed', updated_at = now()
		WHERE deleted_at IS NULL AND NOT (permission_code = ANY($1))`, codes)
	if err != nil {
		return st, fmt.Errorf("seed: retire permissions: %w", err)
	}
	n, _ := res.RowsAffected()
	st.Retired += int(n)

	for _, r := range roles {
		id := RoleID(r.Name)
		// tenancyguard:ok system roles are shared rows (tenant_id NULL), written by identity_migrator only
		if _, err = tx.ExecContext(ctx, `INSERT INTO identity.roles (role_id, tenant_id, created_at, updated_at, code, name_vi, name_en,
			role_scope_kind, is_system) VALUES ($1, NULL, now(), now(), $2, $2, $2, $3, true)
			ON CONFLICT (role_id) DO UPDATE SET code = EXCLUDED.code, updated_at = now(), deleted_at = NULL, deleted_by = NULL,
			delete_reason = NULL`, id, r.Name, string(r.ScopeKind)); err != nil {
			return st, fmt.Errorf("seed: role %s: %w", r.Name, err)
		}
		st.Roles++
		granted := make([]string, 0, len(r.Permissions))
		for _, p := range r.Permissions {
			granted = append(granted, string(p))
			// tenancyguard:ok permissions of a shared system role (tenant_id NULL)
			if _, err = tx.ExecContext(ctx, `INSERT INTO identity.role_permissions (role_permission_id, tenant_id, created_at, updated_at,
				role_id, permission_code) VALUES ($1, NULL, now(), now(), $2, $3)
				ON CONFLICT (role_permission_id) DO UPDATE SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL, updated_at = now()`,
				rolePermissionID(r.Name, string(p)), id, string(p)); err != nil {
				return st, fmt.Errorf("seed: %s → %s: %w", r.Name, p, err)
			}
			st.RolePermissions++
		}
		// tenancyguard:ok permissions of a shared system role (tenant_id NULL)
		res, err = tx.ExecContext(ctx, `UPDATE identity.role_permissions SET deleted_at = now(), delete_reason = 'catalog_removed', updated_at = now()
			WHERE role_id = $1 AND tenant_id IS NULL AND deleted_at IS NULL AND NOT (permission_code = ANY($2))`, id, granted)
		if err != nil {
			return st, fmt.Errorf("seed: retire grants of %s: %w", r.Name, err)
		}
		n, _ := res.RowsAffected()
		st.Retired += int(n)
	}
	return st, tx.Commit()
}
