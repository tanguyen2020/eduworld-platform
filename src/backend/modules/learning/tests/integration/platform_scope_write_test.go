//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/scope"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/tests/integration/fixtures"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func learningClient(ctx context.Context) *ent.Client {
	return ent.NewClient(ent.Driver(entx.Driver(ctx)))
}

func pgCode(err error) string {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

// F020-AC-32 (ADR-0089 §2–3): rows with tenant_id NULL (platform content H, platform content_releases TA*) are
// written by learning_platform under PlatformScope; learning_app writing such a row is refused by RLS; a
// tenant session reads platform rows only once assigned. F020-AC-12: tenant content without a target tenant
// is refused for System Admin.
func TestPlatformScopeWrites(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, true)
	sa := authztest.Platform("system_admin")
	saCtx := authztest.Ctx(ctx, sa)

	sc, err := scope.ForWrite(sa, contentmodel.OwnerPlatform, nil)
	if err != nil {
		t.Fatal(err)
	}
	var item uuid.UUID
	if err := e.uow.InTenantTx(saCtx, sc, func(ctx context.Context) error {
		if item, err = fixtures.CreateItem(ctx, "p-std"); err != nil {
			return err
		}
		return learningClient(ctx).ContentRelease.Create().SetItemType("question").SetItemID(item).SetAction("publish").
			SetToVersionID(ids.New()).SetReleasedBy(sa.UserID).SetIdempotencyKey(ids.New().String()).Exec(ctx)
	}); err != nil {
		t.Fatalf("platform writes under PlatformScope: %v", err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM content.fixture_contents WHERE fixture_content_id = $1
		AND tenant_id IS NULL AND owner_scope = 'platform' AND created_by = $2`, item, sa.UserID); n != 1 {
		t.Fatal("platform item not written")
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM content.content_releases WHERE item_id = $1 AND tenant_id IS NULL`, item); n != 1 {
		t.Fatal("platform release not written")
	}

	// learning_app (tenant session) cannot write tenant_id NULL rows: RLS WITH CHECK refuses them.
	tx, err := e.app.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, e.tenantA.String()); err != nil {
		t.Fatal(err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO content.fixture_contents (fixture_content_id, created_at, updated_at, owner_scope,
		latest_version_no, code, title) VALUES ($1, now(), now(), 'platform', 0, 'p-app', 'x')`, ids.New())
	_ = tx.Rollback()
	if pgCode(err) != "42501" {
		t.Fatalf("learning_app inserting a platform item: %v (want RLS violation 42501)", err)
	}
	tx, _ = e.app.BeginTx(ctx, nil)
	_, _ = tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, e.tenantA.String())
	_, err = tx.ExecContext(ctx, `INSERT INTO content.content_releases (release_id, created_at, item_type, item_id, action,
		to_version_id, released_by, released_at, idempotency_key) VALUES ($1, now(), 'question', $1, 'publish', $1, $1, now(), 'k')`, ids.New())
	_ = tx.Rollback()
	if pgCode(err) != "42501" {
		t.Fatalf("learning_app inserting a platform release: %v (want 42501)", err)
	}
	// A PlatformScope create of a tenant row is refused by the entx hook.
	err = e.uow.InTenantTx(saCtx, sc, func(ctx context.Context) error {
		return fixtures.Client(ctx).FixtureContent.Create().SetTenantID(e.tenantA).SetOwnerScope("tenant").SetCode("x").SetTitle("x").Exec(ctx)
	})
	if !errors.Is(err, entx.ErrTenantOutOfScope) {
		t.Fatalf("tenant row under PlatformScope: %v", err)
	}

	// Tenant A sees the platform item only once assigned; never the platform release (TA*: tenant rows only).
	staffA := authztest.Ctx(ctx, authztest.Staff(e.tenantA, "school_admin"))
	visible := func() (bool, int) {
		var ok bool
		var releases int
		if err := e.read.Read(staffA, func(ctx context.Context) error {
			_, gerr := fixtures.Get(ctx, item)
			ok = gerr == nil
			var err error
			releases, err = learningClient(ctx).ContentRelease.Query().Count(ctx)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return ok, releases
	}
	if ok, rel := visible(); ok || rel != 0 {
		t.Fatalf("before assignment: visible=%v releases=%d", ok, rel)
	}
	if err := fixtures.Assign(ctx, e.platform, item, e.tenantA); err != nil {
		t.Fatal(err)
	}
	if ok, rel := visible(); !ok || rel != 0 {
		t.Fatalf("after assignment: visible=%v releases=%d", ok, rel)
	}

	// F020-AC-12: tenant content without a target tenant.
	if _, err := scope.ForWrite(sa, contentmodel.OwnerTenant, nil); code(err) != problem.ValidationFailed {
		t.Fatalf("tenant content without target: %v", err)
	}
	sc, err = scope.ForWrite(sa, contentmodel.OwnerTenant, &e.tenantB)
	if err != nil {
		t.Fatal(err)
	}
	var tb uuid.UUID
	if err := e.uow.InTenantTx(saCtx, sc, func(ctx context.Context) error {
		var err error
		tb, err = fixtures.CreateItem(ctx, "b-by-sa")
		return err
	}); err != nil {
		t.Fatalf("tenant content with target: %v", err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM content.fixture_contents WHERE fixture_content_id = $1 AND tenant_id = $2`, tb, e.tenantB); n != 1 {
		t.Fatal("tenant item of B not written")
	}
}
