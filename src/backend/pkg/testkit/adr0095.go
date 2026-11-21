//go:build integration

package testkit

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// RepoFile returns the content of a file given relative to the repository root (scripts, migrations).
func RepoFile(t testing.TB, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(RepoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// RollbackRoundTrip checks the backup plan of ADR-0095 mục 8 (step 4) on an isolated database of module where every
// migration is applied: the hand-written rollback script recreates the dropped objects (relations: regclass names;
// columns: "schema.table.column"), then the contract migration applies again and drops them all. Both files run as
// the module's migrator role, exactly as the runbook does. narrowed maps a CHECK constraint to the old values it must no
// longer accept once the migration ran on this database, where no row carries them.
func RollbackRoundTrip(ctx context.Context, t testing.TB, module, migration, rollback string, relations, columns []string,
	narrowed map[string][]string) {
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
	for con, old := range narrowed {
		var def string
		if err := db.QueryRowContext(ctx, `SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname = $1 LIMIT 1`, con).Scan(&def); err != nil {
			t.Fatalf("constraint %s: %v", con, err)
		}
		for _, v := range old {
			if strings.Contains(def, "'"+v+"'") {
				t.Errorf("%s still accepts %q: %s", con, v, def)
			}
		}
	}
}
