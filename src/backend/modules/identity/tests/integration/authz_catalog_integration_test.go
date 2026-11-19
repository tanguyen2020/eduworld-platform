//go:build integration

package integration

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/seed"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// F012-AC-01, AC-03 (TC-AUTH-076) and the layer-2 triggers of F012-AC-17, AC-21 on PostgreSQL.
func TestAuthzCatalogAndTriggers(t *testing.T) {
	ctx := context.Background()
	cl := testkit.Isolated(ctx, t)
	migrator, app, platform := open(t, cl.ModuleDSN("identity", "migrator")), open(t, cl.ModuleDSN("identity", "app")), open(t, cl.ModuleDSN("identity", "platform"))

	// AC-03: the catalog copy with scope kinds; idempotent.
	for range 2 {
		if _, err := identity.SyncAuthzCatalog(ctx, migrator); err != nil {
			t.Fatal(err)
		}
	}
	if n := count(ctx, t, platform, `SELECT count(*) FROM identity.permissions WHERE deleted_at IS NULL`); n != len(authz.Catalog()) {
		t.Fatalf("permissions = %d, catalog = %d", n, len(authz.Catalog()))
	}
	if n := count(ctx, t, platform, `SELECT count(*) FROM identity.permissions WHERE scope_kind = 'platform' AND permission_code LIKE 'game.%'`); n == 0 {
		t.Fatal("game.* must be platform permissions")
	}
	// AC-01: the 12 system roles, no teacher / tenant_moderator.
	if n := count(ctx, t, platform, `SELECT count(*) FROM identity.roles WHERE is_system AND tenant_id IS NULL AND deleted_at IS NULL`); n != 12 {
		t.Fatalf("system roles = %d", n)
	}
	if n := count(ctx, t, platform, `SELECT count(*) FROM identity.roles WHERE code IN ('teacher', 'tenant_moderator')`); n != 0 {
		t.Fatal("teacher/tenant_moderator must not exist (ADR-0072)")
	}
	grants := 0
	for _, r := range authz.SystemRoles() {
		grants += len(r.Permissions)
	}
	if n := count(ctx, t, platform, `SELECT count(*) FROM identity.role_permissions WHERE deleted_at IS NULL AND tenant_id IS NULL`); n != grants {
		t.Fatalf("system grants = %d, want %d", n, grants)
	}

	a := seedTenant(ctx, t, platform, "pgd-authz")
	fail := func(db *sql.DB, label, q string, args ...any) {
		t.Helper()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		if _, err := tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, a.tenant); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, q, args...); err == nil {
			t.Fatalf("%s: accepted", label)
		}
	}
	// The catalog copy is not writable by the application roles (ADR-0066).
	fail(app, "app writes permissions", `UPDATE identity.permissions SET scope_kind = 'tenant' WHERE permission_code = 'game.quest.write'`)
	fail(platform, "platform writes permissions", `INSERT INTO identity.permissions (permission_code, created_at, updated_at, scope_kind, group_code, name_vi, name_en, catalog_version, synced_at)
		VALUES ('x.y', now(), now(), 'tenant', 'x', 'x', 'x', 'v', now())`)
	// AC-03 layer 2: a tenant role holding a platform permission is refused by the trigger.
	custom := uuid.New()
	tx, err := app.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, a.tenant)
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity.roles (role_id, tenant_id, created_at, updated_at, code, name_vi, name_en, role_scope_kind, is_system)
		VALUES ($1, $2, now(), now(), 'gv_bo_mon', 'GV', 'GV', 'tenant', false)`, custom, a.tenant); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	fail(app, "tenant role + platform permission", `INSERT INTO identity.role_permissions (role_permission_id, tenant_id, created_at, updated_at, role_id, permission_code)
		VALUES ($1, $2, now(), now(), $3, 'game.quest.write')`, uuid.New(), a.tenant, custom)
	// AC-17 layer 2: system roles are read-only for the application.
	fail(platform, "update system role", `UPDATE identity.roles SET name_vi = 'x' WHERE role_id = $1`, seed.RoleID("school_admin"))
	fail(platform, "grant on system role", `INSERT INTO identity.role_permissions (role_permission_id, tenant_id, created_at, updated_at, role_id, permission_code)
		VALUES ($1, NULL, now(), now(), $2, 'tenant.audit.read')`, uuid.New(), seed.RoleID("academic_manager"))
	// Shared system roles are readable inside a tenant (RLS), custom roles of other tenants are not.
	if n := countIn(ctx, t, app, a.tenant, `SELECT count(*) FROM identity.roles WHERE is_system`); n != 12 {
		t.Fatalf("system roles visible in tenant = %d", n)
	}
	b := seedTenant(ctx, t, platform, "pgd-authz-b")
	if n := countIn(ctx, t, app, b.tenant, `SELECT count(*) FROM identity.roles WHERE role_id = $1`, custom); n != 0 {
		t.Fatal("custom role of another tenant is visible")
	}

	// AC-21 layer 2: a student never holds a portal role; ADR-0081 home-tenant rules.
	student := enrolled(ctx, t, platform, a, "hs.authz")
	staffID, platformStaff := uuid.New(), uuid.New()
	if _, err := platform.ExecContext(ctx, `INSERT INTO identity.users (user_id, created_at, updated_at, provisioned_at, user_type, username, email, display_name, primary_tenant_id)
		VALUES ($1, now(), now(), now(), 'staff', 'gv.authz', 'gv.authz@school.test', 'GV', $2), ($3, now(), now(), now(), 'staff', 'sa.authz', 'sa.authz@eduworld.test', 'SA', NULL)`,
		staffID, a.tenant, platformStaff); err != nil {
		t.Fatal(err)
	}
	grant := `INSERT INTO identity.user_roles (user_role_id, tenant_id, created_at, updated_at, user_id, role_id, scope_type, valid_from, granted_by, source)
		VALUES ($1, $2, now(), now(), $3, $4, $5, now(), $6, 'manual')`
	fail(platform, "student portal role", grant, uuid.New(), a.tenant, student, seed.RoleID("school_admin"), "tenant", platformStaff)
	fail(platform, "platform role to tenant staff", grant, uuid.New(), nil, staffID, seed.RoleID("system_admin"), "platform", platformStaff)
	fail(platform, "tenant role to platform staff", grant, uuid.New(), a.tenant, platformStaff, seed.RoleID("school_admin"), "tenant", platformStaff)
	fail(platform, "tenant role at platform scope", grant, uuid.New(), nil, staffID, seed.RoleID("school_admin"), "platform", platformStaff)
	fail(platform, "class scope", grant, uuid.New(), a.tenant, staffID, seed.RoleID("school_admin"), "class", platformStaff)
	if _, err := platform.ExecContext(ctx, grant, uuid.New(), a.tenant, staffID, seed.RoleID("school_admin"), "tenant", platformStaff); err != nil {
		t.Fatalf("valid grant: %v", err)
	}
	// Staff with a valid tenant role are members of the tenant (one row per tenant and user).
	if n := count(ctx, t, platform, `SELECT count(*) FROM identity.tenant_memberships_v WHERE user_id = $1 AND tenant_id = $2 AND member_kind = 'staff'`, staffID, a.tenant); n != 1 {
		t.Fatalf("staff membership rows = %d", n)
	}
	if strings.TrimSpace(seed.Version(authz.Catalog(), authz.SystemRoles())) == "" {
		t.Fatal("catalog version")
	}
}

// countIn counts inside a transaction scoped to tenant (RLS of the app role).
func countIn(ctx context.Context, t *testing.T, db *sql.DB, tenant, q string, args ...any) int {
	t.Helper()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenant); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := tx.QueryRowContext(ctx, q, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}
