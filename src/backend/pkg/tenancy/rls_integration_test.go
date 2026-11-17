//go:build integration

package tenancy_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

func connect(t *testing.T, dsn string) *pgx.Conn {
	t.Helper()
	c, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close(context.Background()) })
	return c
}

func exec(t *testing.T, c *pgx.Conn, sql string, args ...any) {
	t.Helper()
	if _, err := c.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// countAs runs inside a transaction with app.tenant_id set (or not) like UnitOfWork/ReadSession.
func countAs(t *testing.T, c *pgx.Conn, tenant *uuid.UUID, sql string) int {
	t.Helper()
	ctx := context.Background()
	tx, err := c.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if tenant != nil {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenant.String()); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	if err := tx.QueryRow(ctx, sql).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

// F002-AC-13 (TC-TEN-005): policies from RLSPolicies return 0 rows without a session tenant or with
// another tenant, for every table kind; no policy contains deleted_at.
func TestRLSPoliciesIsolateTenants(t *testing.T) {
	ctx := context.Background()
	cl := testkit.StartCluster(ctx, t, true)
	mig := connect(t, cl.ModuleDSN("learning", "migrator"))
	plat := connect(t, cl.ModuleDSN("learning", "platform"))
	app := connect(t, cl.ModuleDSN("learning", "app"))
	a, b := uuid.New(), uuid.New()

	exec(t, mig, `CREATE TABLE IF NOT EXISTS infra.tenant_resource_assignments (
		resource_type varchar(64) NOT NULL, resource_id uuid NOT NULL, tenant_id uuid NOT NULL, deleted_at timestamptz)`)
	exec(t, mig, `GRANT SELECT ON infra.tenant_resource_assignments TO learning_app`)

	tables := map[tenancy.TableKind]string{
		tenancy.KindT: "content.k_t", tenancy.KindTA: "content.k_ta", tenancy.KindTStar: "content.k_tstar",
		tenancy.KindTAStar: "content.k_tastar", tenancy.KindH: "content.k_h",
	}
	for kind, table := range tables {
		exec(t, mig, `CREATE TABLE `+table+` (id uuid PRIMARY KEY, tenant_id uuid, deleted_at timestamptz)`)
		var opts []tenancy.RLSOption
		if kind == tenancy.KindH {
			opts = append(opts, tenancy.HybridResource("question", "id"))
		}
		sql, err := tenancy.RLSPolicies(table, kind, opts...)
		if err != nil {
			t.Fatal(err)
		}
		exec(t, mig, sql)
		// platform role (BYPASSRLS) writes one row per tenant and one platform row
		exec(t, plat, `INSERT INTO `+table+` VALUES ($1, $2, NULL), ($3, $4, NULL), ($5, NULL, NULL)`,
			uuid.New(), a, uuid.New(), b, uuid.New())

		if n := countAs(t, app, nil, "SELECT count(*) FROM "+table); n != 0 {
			t.Fatalf("%s: %d rows without app.tenant_id, want 0", kind, n)
		}
		if n := countAs(t, app, &a, "SELECT count(*) FROM "+table); n != 1 {
			t.Fatalf("%s: %d rows as tenant A, want 1 (no tenant B, no platform row)", kind, n)
		}
	}

	// H: a platform row becomes visible to tenant A once assigned (ADR-0029)
	var pid uuid.UUID
	if err := plat.QueryRow(ctx, `SELECT id FROM content.k_h WHERE tenant_id IS NULL`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	exec(t, mig, `INSERT INTO infra.tenant_resource_assignments VALUES ('question', $1, $2, NULL)`, pid, a)
	if n := countAs(t, app, &a, "SELECT count(*) FROM content.k_h"); n != 2 {
		t.Fatalf("H: %d rows as tenant A after assignment, want 2", n)
	}
	if n := countAs(t, app, &b, "SELECT count(*) FROM content.k_h"); n != 1 {
		t.Fatalf("H: %d rows as tenant B, want 1", n)
	}

	// No policy references deleted_at of the protected table. (a.deleted_at of the assignment row in the
	// H policy is the assignment's own soft-delete and is expected — DATABASE_DESIGN §4.)
	var bad int
	if err := mig.QueryRow(ctx, `SELECT count(*) FROM pg_policies WHERE schemaname = 'content'
		AND (coalesce(qual,'') ~ '(^|[^.a-z_])deleted_at' OR coalesce(with_check,'') ~ '(^|[^.a-z_])deleted_at')`).Scan(&bad); err != nil {
		t.Fatal(err)
	}
	if bad != 0 {
		t.Fatalf("%d policies reference deleted_at", bad)
	}
}

// F002-AC-29: soft-delete UPDATE … RETURNING as <module>_app succeeds under the generated policies; the
// row disappears from deleted_at IS NULL reads and from the _live view; _readonly cannot read the base table.
func TestSoftDeleteUnderRLS(t *testing.T) {
	ctx := context.Background()
	cl := testkit.StartCluster(ctx, t, true)
	mig := connect(t, cl.ModuleDSN("learning", "migrator"))
	app := connect(t, cl.ModuleDSN("learning", "app"))
	ro := connect(t, cl.ModuleDSN("learning", "readonly"))
	a := uuid.New()

	exec(t, mig, `CREATE TABLE assessment.attempts (id uuid PRIMARY KEY, tenant_id uuid NOT NULL, deleted_at timestamptz, deleted_by uuid)`)
	pol, _ := tenancy.RLSPolicies("assessment.attempts", tenancy.KindT)
	view, _ := tenancy.LiveView("assessment.attempts", "learning")
	exec(t, mig, pol)
	exec(t, mig, view)

	id := uuid.New()
	tx, err := app.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	exec(t, app, "SELECT 1") // keep conn warm
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", a.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO assessment.attempts (id, tenant_id) VALUES ($1, $2)`, id, a); err != nil {
		t.Fatal(err)
	}
	var got uuid.UUID
	if err := tx.QueryRow(ctx, `UPDATE assessment.attempts SET deleted_at = now(), deleted_by = $2 WHERE id = $1 RETURNING id`, id, uuid.New()).Scan(&got); err != nil {
		t.Fatalf("soft-delete under RLS failed: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	if n := countAs(t, app, &a, "SELECT count(*) FROM assessment.attempts WHERE deleted_at IS NULL"); n != 0 {
		t.Fatalf("soft-deleted row still read: %d", n)
	}
	if n := countAs(t, app, &a, "SELECT count(*) FROM assessment.attempts"); n != 1 {
		t.Fatalf("row physically missing (must be soft-deleted only): %d", n)
	}
	if n := countAs(t, ro, &a, "SELECT count(*) FROM assessment.attempts_live"); n != 0 {
		t.Fatalf("_live view shows deleted row: %d", n)
	}
	_, err = ro.Exec(ctx, "SELECT count(*) FROM assessment.attempts")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Fatalf("_readonly read the base table: %v", err)
	}
}
