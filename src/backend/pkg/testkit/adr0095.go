//go:build integration

package testkit

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// RepoFile returns the content of a file given relative to the repository root (scripts, migrations).
func RepoFile(t testing.TB, rel string) string {
	t.Helper()
	_, here, _, _ := runtime.Caller(0) // src/backend/pkg/testkit/adr0095.go
	b, err := os.ReadFile(filepath.Join(filepath.Dir(here), "..", "..", "..", "..", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// RollbackRoundTrip checks the backup plan of ADR-0095 mục 8 (step 4) on an isolated database of module where every
// migration is applied: the hand-written rollback script recreates the dropped objects (relations: regclass names;
// columns: "schema.table.column"), then the contract migration applies again and drops them all. Both files run as
// the module's migrator role, exactly as the runbook does.
func RollbackRoundTrip(ctx context.Context, t testing.TB, module, migration, rollback string, relations, columns []string) {
	t.Helper()
	cl := Isolated(ctx, t)
	db := open(t, cl.ModuleDSN(module, "migrator"))
	exists := func() (rel, col int) {
		t.Helper()
		for _, r := range relations {
			var ok bool
			if err := db.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, r).Scan(&ok); err != nil {
				t.Fatal(err)
			}
			if ok {
				rel++
			}
		}
		for _, c := range columns {
			var n int
			if err := db.QueryRowContext(ctx, `SELECT count(*) FROM information_schema.columns
				WHERE table_schema || '.' || table_name || '.' || column_name = $1`, c).Scan(&n); err != nil {
				t.Fatal(err)
			}
			col += n
		}
		return rel, col
	}
	if r, c := exists(); r != 0 || c != 0 {
		t.Fatalf("after the migrations %d relations and %d columns of the ADR-0095 list still exist", r, c)
	}
	apply := func(name, file string) {
		t.Helper()
		tx, err := db.BeginTx(ctx, &sql.TxOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, RepoFile(t, file)); err != nil {
			_ = tx.Rollback()
			t.Fatalf("%s %s: %v", name, file, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	apply("rollback", rollback)
	if r, c := exists(); r != len(relations) || c != len(columns) {
		t.Fatalf("rollback recreated %d/%d relations and %d/%d columns", r, len(relations), c, len(columns))
	}
	apply("contract migration", migration)
	if r, c := exists(); r != 0 || c != 0 {
		t.Fatalf("re-applied migration left %d relations and %d columns", r, c)
	}
}
