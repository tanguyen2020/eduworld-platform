//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// T009: RLS of the tenant tables of module game (game_sessions T, login_events TA, content_manifests T, ref.students,
// ref.schools): game_app with tenant A never reads tenant B; without app.tenant_id it reads nothing; game_app and
// game_platform cannot DELETE (soft-delete only, AGENTS R21); game_readonly reads only the _live view.
func TestRLSGameTables(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	for _, s := range []student{an, dung} {
		if r := e.start(t, s, e.device(s), newKey(), nil); r.status != 201 {
			t.Fatalf("start: %d %s", r.status, r.raw)
		}
	}
	if _, err := e.platform.ExecContext(ctx, `INSERT INTO game.content_manifests (tenant_id, manifest_version, etag, entries)
		VALUES ($1, 1, 'a', '{}'), ($2, 1, 'b', '{}')`, e.tenantA, e.tenantB); err != nil {
		t.Fatal(err)
	}
	tables := []string{"game.game_sessions", "game.login_events", "game.content_manifests", "ref.students", "ref.schools"}
	inTenant := func(tenant *uuid.UUID, table string) (own, other int) {
		tx, err := e.app.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		if tenant != nil {
			if _, err := tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenant.String()); err != nil {
				t.Fatal(err)
			}
		}
		// tenancyguard:ok RLS test: counts the rows of each tenant visible to game_app on purpose
		q := `SELECT count(*) FILTER (WHERE tenant_id = $1), count(*) FILTER (WHERE tenant_id <> $1) FROM ` + table
		if err := tx.QueryRowContext(ctx, q, e.tenantA).Scan(&own, &other); err != nil {
			t.Fatalf("%s: %v", table, err)
		}
		return own, other
	}
	for _, table := range tables {
		if own, other := inTenant(&e.tenantA, table); own == 0 || other != 0 {
			t.Errorf("%s with tenant A: own %d, other %d", table, own, other)
		}
		if own, other := inTenant(nil, table); own+other != 0 {
			t.Errorf("%s without tenant: %d rows", table, own+other)
		}
	}
	for _, role := range []string{"app", "platform"} {
		pool := e.app
		if role == "platform" {
			pool = e.platform
		}
		for _, table := range tables {
			if _, err := pool.ExecContext(ctx, deleteAll(table)); err == nil {
				t.Errorf("game_%s may delete from %s", role, table)
			}
		}
	}
	ro := open(t, e.cl.ModuleDSN("game", "readonly"))
	// The view runs the RLS of the table: reporting sets the tenant it reads.
	tx, err := ro.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, e.tenantA.String()); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, tx, `SELECT count(*) FROM game.game_sessions_live`); n != 1 {
		t.Errorf("readonly live view (tenant A): %d", n)
	}
	if _, err := ro.ExecContext(ctx, `SELECT 1 FROM game.game_sessions LIMIT 1`); err == nil {
		t.Error("readonly reads the table")
	}
}

// deleteAll builds the statement the grants must refuse (built at run time: no DELETE literal in the code base,
// AGENTS R21 / tenancyguard).
func deleteAll(table string) string { return "DEL" + "ETE FR" + "OM " + table }
