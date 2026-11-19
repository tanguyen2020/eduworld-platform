//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/tests/integration/fixtures"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// F020-AC-31 (TC-TEN-015, ADR-0089 §1): no RLS policy of eduworld_learning references deleted_at of its table;
// soft-deleting an item as learning_app under RLS succeeds; the item disappears from the interceptor and from
// the _live view read by learning_readonly, which has no privilege on the base table.
func TestRLSAndSoftDelete(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, true)

	var bad int
	if err := e.migrator.QueryRowContext(ctx, `SELECT count(*) FROM pg_policies WHERE schemaname IN ('content','curriculum','ref','infra','assessment','learning','lms')
		AND (coalesce(qual,'') ~ '(^|[^.a-z_])deleted_at' OR coalesce(with_check,'') ~ '(^|[^.a-z_])deleted_at')`).Scan(&bad); err != nil {
		t.Fatal(err)
	}
	if bad != 0 {
		t.Fatalf("%d policies reference deleted_at", bad)
	}
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM pg_policies WHERE schemaname IN ('content','ref')`); n < 10 {
		t.Fatalf("only %d policies: migration or fixture RLS missing", n)
	}

	item := tenantItem(ctx, t, e, e.tenantA, "a-1")
	keep := tenantItem(ctx, t, e, e.tenantA, "a-2")
	liveCount := func() int {
		tx, err := e.readonly.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		if _, err := tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, e.tenantA.String()); err != nil {
			t.Fatal(err)
		}
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM content.fixture_contents_live`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := liveCount(); n != 2 {
		t.Fatalf("_live before delete = %d", n)
	}
	// learning_app under RLS (TenantScope → app.tenant_id = A) soft-deletes through ent.
	if err := e.uow.Do(authztest.Ctx(ctx, authztest.Staff(e.tenantA, "tenant_content_editor")), func(ctx context.Context) error {
		return fixtures.Client(ctx).FixtureContent.DeleteOneID(item).Exec(ctx)
	}); err != nil {
		t.Fatalf("soft-delete under RLS: %v", err)
	}
	if visibleTo(ctx, t, e, e.tenantA, item) || !visibleTo(ctx, t, e, e.tenantA, keep) {
		t.Fatal("interceptor: deleted item visible or live item hidden")
	}
	if n := liveCount(); n != 1 {
		t.Fatalf("_live after delete = %d", n)
	}
	if _, err := e.readonly.ExecContext(ctx, `SELECT 1 FROM content.fixture_contents LIMIT 1`); pgCode(err) != "42501" {
		t.Fatalf("learning_readonly on the base table: %v (want 42501)", err)
	}
	if _, err := e.readonly.ExecContext(ctx, `SELECT 1 FROM infra.tenant_resource_assignments LIMIT 1`); pgCode(err) != "42501" {
		t.Fatalf("learning_readonly on assignments: %v (want 42501)", err)
	}
	if _, err := e.readonly.ExecContext(ctx, `SELECT 1 FROM infra.tenant_resource_assignments_live LIMIT 1`); err != nil {
		t.Fatalf("learning_readonly on assignments _live: %v", err)
	}
}

// F020-AC-08 (DB side): CHECK (owner_scope = 'platform') = (tenant_id IS NULL) on H tables; data-model §3: the
// function content.forbid_published_update() keeps published/archived versions immutable except
// published → archived.
func TestOwnerCheckAndPublishedImmutability(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, true)
	for _, q := range []string{
		`INSERT INTO content.fixture_contents (fixture_content_id, tenant_id, created_at, updated_at, owner_scope, latest_version_no, code, title)
			VALUES (gen_random_uuid(), $1, now(), now(), 'platform', 0, 'x', 'x')`,
		`INSERT INTO content.fixture_contents (fixture_content_id, created_at, updated_at, owner_scope, latest_version_no, code, title)
			VALUES (gen_random_uuid(), now(), now(), 'tenant', 0, 'y', 'y')`,
	} {
		var err error
		if strings.Contains(q, "$1") {
			_, err = e.platform.ExecContext(ctx, q, e.tenantA)
		} else {
			_, err = e.platform.ExecContext(ctx, q)
		}
		if pgCode(err) != "23514" {
			t.Fatalf("owner_scope/tenant_id mismatch accepted: %v", err)
		}
	}
	item := tenantItem(ctx, t, e, e.tenantA, "a-1")
	var v uuid.UUID
	if err := e.uow.Do(authztest.Ctx(ctx, authztest.Staff(e.tenantA, "tenant_content_editor")), func(ctx context.Context) error {
		r, err := fixtures.Client(ctx).FixtureContentVersion.Create().SetFixtureContentID(item).SetVersionNo(1).SetStatus("published").
			SetAuthorID(uuid.New()).SetCurriculumVersionID(uuid.New()).SetBody("v1").Save(ctx)
		if err == nil {
			v = r.ID
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.platform.ExecContext(ctx, `UPDATE content.fixture_content_versions SET body = 'changed' WHERE fixture_content_version_id = $1`, v); pgCode(err) != "P0001" {
		t.Fatalf("published version changed: %v", err)
	}
	if _, err := e.platform.ExecContext(ctx, `UPDATE content.fixture_content_versions SET status = 'archived' WHERE fixture_content_version_id = $1`, v); err != nil {
		t.Fatalf("published → archived refused: %v", err)
	}
	if _, err := e.platform.ExecContext(ctx, `UPDATE content.fixture_content_versions SET status = 'draft' WHERE fixture_content_version_id = $1`, v); pgCode(err) != "P0001" {
		t.Fatalf("archived version changed: %v", err)
	}
}
