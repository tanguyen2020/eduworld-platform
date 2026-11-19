//go:build integration

package integration

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/tests/integration/fixtures"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// hybridWorld creates items of tenants A and B, two platform items (P1 assigned to A, P2 unassigned) and a
// deleted item of A.
type hybridWorld struct {
	a, b, p1, p2, deleted uuid.UUID
}

func newHybridWorld(ctx context.Context, t *testing.T, e *env) hybridWorld {
	t.Helper()
	var w hybridWorld
	create := func(ctx context.Context, code string, id *uuid.UUID) {
		t.Helper()
		if err := e.uow.Do(ctx, func(ctx context.Context) error {
			var err error
			*id, err = fixtures.CreateItem(ctx, code)
			return err
		}); err != nil {
			t.Fatalf("create %s: %v", code, err)
		}
	}
	staffA := authztest.Ctx(ctx, authztest.Staff(e.tenantA, "tenant_content_editor"))
	staffB := authztest.Ctx(ctx, authztest.Staff(e.tenantB, "tenant_content_editor"))
	platform := authztest.Ctx(ctx, authztest.Platform("content_admin"))
	create(staffA, "a-1", &w.a)
	create(staffA, "a-deleted", &w.deleted)
	create(staffB, "b-1", &w.b)
	create(platform, "p-1", &w.p1)
	create(platform, "p-2", &w.p2)
	if err := e.uow.Do(staffA, func(ctx context.Context) error {
		return fixtures.Client(ctx).FixtureContent.DeleteOneID(w.deleted).Exec(ctx)
	}); err != nil {
		t.Fatal(err)
	}
	if err := fixtures.Assign(ctx, e.platform, w.p1, e.tenantA); err != nil {
		t.Fatal(err)
	}
	return w
}

func idsOf(rows []fixtures.Row) []uuid.UUID {
	out := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		out[i] = r.ID
	}
	slices.SortFunc(out, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	return out
}

func sorted(ids ...uuid.UUID) []uuid.UUID {
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	return ids
}

// F020-AC-09 (TC-TEN-001, AC-ORG-009): a tenant reads its own rows plus the platform rows assigned to it, never
// deleted rows; an id outside the scope is 404 — through ent (global filter) and explicit SQL alike.
func TestHybridReadScope(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, true)
	w := newHybridWorld(ctx, t, e)
	cases := []struct {
		name string
		ctx  context.Context
		want []uuid.UUID
	}{
		{"tenant A", authztest.Ctx(ctx, authztest.Staff(e.tenantA, "school_admin")), sorted(w.a, w.p1)},
		{"tenant B", authztest.Ctx(ctx, authztest.Staff(e.tenantB, "school_admin")), sorted(w.b)},
		{"platform", authztest.Ctx(ctx, authztest.Platform("content_admin")), sorted(w.p1, w.p2)},
	}
	for _, c := range cases {
		var viaEnt, viaSQL []fixtures.Row
		if err := e.read.Read(c.ctx, func(ctx context.Context) error {
			var err error
			if viaEnt, err = fixtures.List(ctx); err != nil {
				return err
			}
			viaSQL, err = fixtures.ListExplicit(ctx)
			return err
		}); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := idsOf(viaEnt); !slices.Equal(got, c.want) {
			t.Errorf("%s via ent = %v, want %v", c.name, got, c.want)
		}
		if got := idsOf(viaSQL); !slices.Equal(got, c.want) {
			t.Errorf("%s via SQL = %v, want %v", c.name, got, c.want)
		}
	}
	// Ids outside the scope (other tenant, unassigned platform, deleted) answer 404.
	b := authztest.Ctx(ctx, authztest.Staff(e.tenantB, "school_admin"))
	for _, id := range []uuid.UUID{w.a, w.p1, w.p2, w.deleted} {
		err := e.read.Read(b, func(ctx context.Context) error { _, err := fixtures.Get(ctx, id); return err })
		if code(err) != problem.NotFound {
			t.Errorf("tenant B reading %s: %v (want 404)", id, err)
		}
	}
}

// F020-AC-10 (TC-TEN-005, TC-TEN-021): no scope → ErrMissingTenantScope; SQL without a tenant condition still
// sees only the session tenant (RLS), and nothing without app.tenant_id.
func TestHybridMissingScopeAndRLS(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, true)
	w := newHybridWorld(ctx, t, e)

	err := e.read.Read(ctx, func(ctx context.Context) error { _, err := fixtures.List(ctx); return err })
	if !errors.Is(err, entx.ErrMissingTenantScope) {
		t.Fatalf("ent without scope: %v", err)
	}
	err = e.read.Read(ctx, func(ctx context.Context) error { _, err := fixtures.ListExplicit(ctx); return err })
	if !errors.Is(err, entx.ErrMissingTenantScope) {
		t.Fatalf("explicit SQL without scope: %v", err)
	}
	rlsCount := func(tenant *uuid.UUID) int {
		t.Helper()
		tx, err := e.app.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		if tenant != nil {
			if _, err := tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenant.String()); err != nil {
				t.Fatal(err)
			}
		}
		var n int
		// No WHERE at all: RLS alone decides (deleted rows are filtered by the interceptor/_live, not RLS).
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM content.fixture_contents`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := rlsCount(nil); n != 0 {
		t.Fatalf("no app.tenant_id: %d rows, want 0", n)
	}
	if n := rlsCount(&e.tenantA); n != 3 { // a-1, a-deleted (soft-deleted, still under RLS), p-1 assigned
		t.Fatalf("tenant A under RLS: %d rows, want 3", n)
	}
	if n := rlsCount(&e.tenantB); n != 1 {
		t.Fatalf("tenant B under RLS: %d rows, want 1", n)
	}
	_ = w
	// A bypass other than trash keeps the soft-delete filter.
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, "learning"), tenancy.ReasonPlatformJob)
	if err != nil {
		t.Fatal(err)
	}
	var all []fixtures.Row
	if err := e.read.Read(bctx, func(ctx context.Context) error { var err error; all, err = fixtures.List(ctx); return err }); err != nil {
		t.Fatal(err)
	}
	if len(all) != 4 {
		t.Fatalf("platform_job bypass: %d rows, want 4 live rows", len(all))
	}
}
