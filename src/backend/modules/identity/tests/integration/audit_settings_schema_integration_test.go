//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// F015-AC-02, AC-03, AC-06 (schema part), AC-23, AC-24 (DB layer): the audit log is append-only for the
// application roles, partitioned by month without default partition, anonymized in place by the retention
// function; the portal menu catalog is read-only for identity_app; menu/action flags must exist in the catalog.
func TestAuditSettingsSchema(t *testing.T) {
	ctx := context.Background()
	cl := testkit.Isolated(ctx, t)
	app, platform := open(t, cl.ModuleDSN("identity", "app")), open(t, cl.ModuleDSN("identity", "platform"))
	migrator, partAdmin := open(t, cl.ModuleDSN("identity", "migrator")), open(t, cl.ModuleDSN("identity", "partition_admin"))
	tenantA, tenantB := uuid.NewString(), uuid.NewString()

	insert := `INSERT INTO audit.audit_logs (audit_id, occurred_at, actor_id, actor_type, tenant_id, action, target_type, target_id, result, metadata, producer)
		VALUES ($1, $2::timestamptz, $3, 'user', $4::uuid, 'tenant.updated', 'tenant', 'x', 'success', $5::jsonb, 'identity/test')`
	actor := uuid.New()
	if _, err := platform.ExecContext(ctx, insert, uuid.New(), "now", actor, tenantA, `{"tenant_type":"school"}`); err != nil {
		t.Fatalf("platform insert: %v", err)
	}
	if _, err := platform.ExecContext(ctx, insert, uuid.New(), "now", actor, tenantB, `{}`); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ExecContext(ctx, insert, uuid.New(), "now", actor, tenantA, `{}`); err == nil {
		t.Fatal("identity_app inserted into the audit log")
	}
	// AC-02: no UPDATE/DELETE/TRUNCATE for the application roles.
	for _, q := range []string{`UPDATE audit.audit_logs SET reason = 'x'`, `DELETE FROM audit.audit_logs`, `TRUNCATE audit.audit_logs`} {
		if _, err := app.ExecContext(ctx, q); err == nil {
			t.Fatalf("identity_app: %s allowed", q)
		}
		if _, err := platform.ExecContext(ctx, q); err == nil {
			t.Fatalf("identity_platform: %s allowed", q)
		}
	}
	// RLS: identity_app sees only its tenant.
	tx, err := app.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantA)
	if n := count(ctx, t, tx, `SELECT count(*) FROM audit.audit_logs`); n != 1 {
		t.Fatalf("app sees %d audit rows of its tenant", n)
	}
	_ = tx.Rollback()

	// AC-03: no default partition — a month beyond the created ones is refused until the job creates it.
	future := "now() + interval '20 months'"
	if _, err := platform.ExecContext(ctx, `INSERT INTO audit.audit_logs (audit_id, occurred_at, actor_type, action, target_type, target_id, result, producer)
		VALUES ($1, `+future+`, 'system', 'x.y', 't', 'x', 'success', 'identity/test')`, uuid.New()); err == nil {
		t.Fatal("row stored without its monthly partition")
	}
	var created int
	if err := partAdmin.QueryRowContext(ctx, `SELECT audit.ensure_monthly_partitions((now() + interval '12 months')::date, 12)`).Scan(&created); err != nil || created < 9 {
		t.Fatalf("ensure partitions = %d %v", created, err)
	}
	if _, err := platform.ExecContext(ctx, `INSERT INTO audit.audit_logs (audit_id, occurred_at, actor_type, action, target_type, target_id, result, producer)
		VALUES ($1, `+future+`, 'system', 'x.y', 't', 'x', 'success', 'identity/test')`, uuid.New()); err != nil {
		t.Fatalf("after the job: %v", err)
	}
	var ahead int
	if err := partAdmin.QueryRowContext(ctx, `SELECT audit.monthly_partitions_ahead('audit.audit_logs', now()::date)`).Scan(&ahead); err != nil || ahead < 12 {
		t.Fatalf("months ahead = %d %v", ahead, err)
	}
	if _, err := app.ExecContext(ctx, `SELECT audit.ensure_monthly_partitions(now()::date, 1)`); err == nil {
		t.Fatal("identity_app may maintain partitions")
	}

	// AC-06: retention anonymizes in place (actor, ip, identifier keys of metadata), idempotent, no delete.
	old := uuid.New()
	if _, err := platform.ExecContext(ctx, `INSERT INTO audit.audit_logs (audit_id, occurred_at, actor_id, actor_type, tenant_id, action, target_type, target_id, result, metadata, ip_hash, producer)
		VALUES ($1, now() - interval '20 days', $2, 'user', $3::uuid, 'role.assigned', 'user', 'x', 'success', '{"user_id":"u1","tenant_type":"school","ip_hash":"h"}', '\x01', 'identity/test')`, old, actor, tenantA); err != nil {
		t.Fatal(err)
	}
	var n int64
	if err := partAdmin.QueryRowContext(ctx, `SELECT audit.anonymize_expired(now() - interval '10 days')`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("anonymized = %d %v", n, err)
	}
	var meta string
	var actorNull, ipNull bool
	if err := platform.QueryRowContext(ctx, `SELECT metadata::text, actor_id IS NULL, ip_hash IS NULL FROM audit.audit_logs WHERE audit_id = $1`, old).
		Scan(&meta, &actorNull, &ipNull); err != nil || meta != `{"tenant_type": "school"}` || !actorNull || !ipNull {
		t.Fatalf("anonymized row = %s %v %v %v", meta, actorNull, ipNull, err)
	}
	if err := partAdmin.QueryRowContext(ctx, `SELECT audit.anonymize_expired(now() - interval '10 days')`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("second run = %d %v", n, err)
	}

	// AC-23: the catalog is written by identity_migrator only.
	if _, err := migrator.ExecContext(ctx, `INSERT INTO platform.portal_menu_catalog (item_key, item_kind, workspace_key, label_key, catalog_version)
		VALUES ('school_lms', 'workspace', 'school_lms', 'ws.school_lms', 'v1') ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	if _, err := migrator.ExecContext(ctx, `INSERT INTO platform.portal_menu_catalog (item_key, item_kind, workspace_key, parent_key, path, label_key, catalog_version)
		VALUES ('menu.school_lms.test_menu', 'menu', 'school_lms', 'school_lms', '/x', 'm.x', 'v1') ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ExecContext(ctx, `UPDATE platform.portal_menu_catalog SET can_disable = false`); err == nil {
		t.Fatal("identity_app wrote the catalog")
	}
	if _, err := platform.ExecContext(ctx, `INSERT INTO platform.portal_menu_catalog (item_key, item_kind, workspace_key, label_key, catalog_version)
		VALUES ('live_ops', 'workspace', 'live_ops', 'x', 'v1')`); err == nil {
		t.Fatal("identity_platform wrote the catalog")
	}
	// AC-24: menu/action flags must name catalog items; actions need their parent menu.
	flag := `INSERT INTO platform.feature_flags (flag_key, flag_kind, parent_key, default_enabled) VALUES ($1, $2, $3, false)`
	if _, err := platform.ExecContext(ctx, flag, "menu.school_lms.unknown", "menu", nil); err == nil {
		t.Fatal("menu flag outside the catalog accepted")
	}
	if _, err := platform.ExecContext(ctx, flag, "action.school_lms.test_menu.publish", "action", nil); err == nil {
		t.Fatal("action flag without parent accepted")
	}
	if _, err := platform.ExecContext(ctx, flag, "menu.school_lms.test_menu", "menu", nil); err != nil {
		t.Fatalf("menu flag in the catalog: %v", err)
	}
	if _, err := platform.ExecContext(ctx, flag, "preview_game_frame", "feature", nil); err != nil {
		t.Fatalf("feature flag: %v", err)
	}
	if _, err := app.ExecContext(ctx, flag, "another_feature", "feature", nil); err == nil {
		t.Fatal("identity_app wrote a feature flag")
	}
	if _, err := app.ExecContext(ctx, `INSERT INTO platform.platform_settings (key, value) VALUES ('platform_curfew', '{}')`); err == nil {
		t.Fatal("identity_app wrote platform settings")
	}
}
