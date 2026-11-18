//go:build integration

package integration

import (
	"context"
	"database/sql"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// F011 T006/T007: SIS tables apply; one active connection per school (SIS_SCHOOL_ALREADY_CONNECTED source),
// one active identity link per user and per source key, login keys unique, csv_import without secret,
// RLS on the T tables, partitioned staging accepts rows.
func TestSisSchema(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	a := seedTenant(ctx, t, e.platform, "pgd-sis-a")
	b := seedTenant(ctx, t, e.platform, "pgd-sis-b")
	exec := func(q string, args ...any) error {
		_, err := e.platform.ExecContext(ctx, q, args...)
		return err
	}
	conn1, conn2 := ids.New(), ids.New()
	for _, c := range []any{conn1, conn2} {
		if err := exec(`INSERT INTO sis.sis_connections (sis_connection_id, tenant_id, created_at, updated_at, provider_kind, status)
			VALUES ($1, $2, now(), now(), 'csv_import', 'active')`, c, a.tenant); err != nil {
			t.Fatal(err)
		}
	}
	if err := exec(`INSERT INTO sis.sis_connections (sis_connection_id, tenant_id, created_at, updated_at, provider_kind, status, secret_ref)
		VALUES ($1, $2, now(), now(), 'csv_import', 'active', 'vault:x')`, ids.New(), a.tenant); pgCode(err) != "23514" {
		t.Fatalf("csv_import with secret_ref: %v", err)
	}
	assign := func(conn any) error {
		return exec(`INSERT INTO sis.sis_connection_schools (connection_school_id, tenant_id, created_at, updated_at, sis_connection_id, school_id, active, assigned_by, assigned_at)
			VALUES ($1, $2, now(), now(), $3, $4, true, $5, now())`, ids.New(), a.tenant, conn, a.school, ids.New())
	}
	if err := assign(conn1); err != nil {
		t.Fatal(err)
	}
	if err := assign(conn2); pgCode(err) != "23505" {
		t.Fatalf("second active connection for a school: %v", err)
	}
	// Login keys: unique per connection/type/alias/key (NULL alias via coalesce).
	key := func(alias any) error {
		return exec(`INSERT INTO sis.student_login_keys (login_key_id, tenant_id, created_at, updated_at, sis_connection_id, login_type, idp_alias, login_key, student_external_id)
			VALUES ($1, $2, now(), now(), $3, CASE WHEN $4::text IS NULL THEN 'username' ELSE 'idp_subject' END, $4, 'hs.dup', 'HS-1')`, ids.New(), a.tenant, conn1, alias)
	}
	if err := key(nil); err != nil {
		t.Fatal(err)
	}
	if err := key(nil); pgCode(err) != "23505" {
		t.Fatalf("duplicate login key: %v", err)
	}
	// One active identity link per user and per source key (ADR-0046).
	tx, _ := e.platform.BeginTx(ctx, nil)
	user, err := insertStudent(ctx, tx, a, "hs.sis.link")
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	link := func(conn any, ext string) error {
		return exec(`INSERT INTO identity.sis_identity_links (link_id, tenant_id, created_at, updated_at, user_id, school_id, sis_connection_id, student_external_id, status, linked_at)
			VALUES ($1, $2, now(), now(), $3, $4, $5, $6, 'active', now())`, ids.New(), a.tenant, user, a.school, conn, ext)
	}
	if err := link(conn1, "HS-1"); err != nil {
		t.Fatal(err)
	}
	if err := link(conn2, "HS-2"); pgCode(err) != "23505" {
		t.Fatalf("second active link of a user: %v", err)
	}
	// Partitioned staging accepts rows.
	run := ids.New()
	if err := exec(`INSERT INTO sis.sync_runs (sync_run_id, tenant_id, created_at, updated_at, sis_connection_id, mode, status, stats, started_at)
		VALUES ($1, $2, now(), now(), $3, 'csv_import', 'running', '{}', now())`, run, a.tenant, conn1); err != nil {
		t.Fatal(err)
	}
	if err := exec(`INSERT INTO sis.staging_records (staging_record_id, created_at, sync_run_id, entity_kind, external_id, raw_hash, action)
		VALUES ($1, now(), $2, 'student', 'HS-1', 'h', 'create')`, ids.New(), run); err != nil {
		t.Fatal(err)
	}
	// RLS: tenant B sees nothing of tenant A's SIS rows.
	_ = inTenant(ctx, t, e.app, b.tenant, false, func(tx *sql.Tx) error {
		for _, table := range []string{"sis.sis_connections", "sis.sis_connection_schools", "sis.sync_runs", "sis.student_login_keys", "identity.sis_identity_links"} {
			if n := count(ctx, t, tx, "SELECT count(*) FROM "+table); n != 0 {
				t.Fatalf("%s: tenant B sees %d rows of A", table, n)
			}
		}
		return nil
	})
	_ = inTenant(ctx, t, e.app, a.tenant, false, func(tx *sql.Tx) error {
		if n := count(ctx, t, tx, "SELECT count(*) FROM sis.sis_connections"); n != 2 {
			t.Fatalf("tenant A connections = %d", n)
		}
		return nil
	})
}
