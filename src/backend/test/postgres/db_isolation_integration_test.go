//go:build integration

// Package tests verifies the PostgreSQL bootstrap of deploy/postgres/init and the module baselines
// (F001 US2): one database per module on one cluster, no cross-database CONNECT, no DELETE/TRUNCATE.
package tests

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

var roleSuffixes = []string{"app", "platform", "migrator", "readonly", "relay", "partition_admin"}

var databases = []string{"eduworld_identity", "eduworld_learning", "eduworld_game", "eduworld_competition", "eduworld_chat", "keycloak"}

type role struct{ name, ownDB string }

func allRoles() []role {
	var rs []role
	for _, m := range testkit.Modules {
		for _, s := range roleSuffixes {
			rs = append(rs, role{name: m + "_" + s, ownDB: "eduworld_" + m})
		}
	}
	return append(rs, role{name: "chat_purger", ownDB: "eduworld_chat"})
}

// F001-AC-08, F001-AC-09: every (role, database) pair — only the module's own database accepts the role.
func TestRolesConnectOnlyToOwnDatabase(t *testing.T) {
	ctx := context.Background()
	c := testkit.StartCluster(ctx, t, false)

	for _, r := range allRoles() {
		for _, db := range databases {
			conn, err := pgx.Connect(ctx, c.DSN(r.name, db))
			if err == nil {
				conn.Close(ctx)
			}
			switch {
			case db == r.ownDB && err != nil:
				t.Errorf("%s → %s: want connect, got %v", r.name, db, err)
			case db != r.ownDB && err == nil:
				t.Errorf("%s → %s: connected, want permission denied", r.name, db)
			case db != r.ownDB && !isPermissionDenied(err):
				t.Errorf("%s → %s: want permission denied, got %v", r.name, db, err)
			}
		}
	}
}

// F001-AC-10: <module>_app and <module>_platform get SELECT/INSERT/UPDATE on new tables of every module
// schema (default privileges of the baseline) but never DELETE or TRUNCATE.
func TestAppRolesCannotDeleteOrTruncate(t *testing.T) {
	ctx := context.Background()
	c := testkit.StartCluster(ctx, t, true)

	for _, m := range testkit.Modules {
		mig, err := pgx.Connect(ctx, c.ModuleDSN(m, "migrator"))
		if err != nil {
			t.Fatal(err)
		}
		schemas := moduleSchemas(ctx, t, mig)
		if len(schemas) == 0 {
			t.Fatalf("%s: no module schema after baseline", m)
		}
		for _, s := range schemas {
			if _, err := mig.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.f001_probe (id int PRIMARY KEY, v int)`, s)); err != nil {
				t.Fatalf("%s.%s: create probe: %v", m, s, err)
			}
		}
		mig.Close(ctx)

		for _, suffix := range []string{"app", "platform"} {
			conn, err := pgx.Connect(ctx, c.ModuleDSN(m, suffix))
			if err != nil {
				t.Fatal(err)
			}
			for i, s := range schemas {
				tbl := s + ".f001_probe"
				id := i + 1
				if suffix == "platform" {
					id += 1000
				}
				mustExec(t, ctx, conn, fmt.Sprintf(`INSERT INTO %s VALUES (%d, 1)`, tbl, id))
				mustExec(t, ctx, conn, fmt.Sprintf(`UPDATE %s SET v = 2 WHERE id = %d`, tbl, id))
				mustExec(t, ctx, conn, fmt.Sprintf(`SELECT count(*) FROM %s`, tbl))
				for _, stmt := range []string{"DELETE FROM " + tbl, "TRUNCATE " + tbl} {
					if _, err := conn.Exec(ctx, stmt); !isPermissionDenied(err) {
						t.Errorf("%s_%s: %q = %v, want permission denied", m, suffix, stmt, err)
					}
				}
			}
			conn.Close(ctx)
		}
	}
}

func moduleSchemas(ctx context.Context, t *testing.T, conn *pgx.Conn) []string {
	t.Helper()
	rows, err := conn.Query(ctx, `SELECT nspname FROM pg_namespace
		WHERE nspname NOT LIKE 'pg\_%' AND nspname NOT IN ('information_schema','public','atlas_schema_revisions')
		ORDER BY nspname`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func mustExec(t *testing.T, ctx context.Context, conn *pgx.Conn, sql string) {
	t.Helper()
	if _, err := conn.Exec(ctx, sql); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func isPermissionDenied(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "42501"
	}
	return err != nil && strings.Contains(err.Error(), "permission denied")
}
