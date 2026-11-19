//go:build integration

package integration

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/tests/integration/fixtures"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F020-AC-21 (TC-TEN-015): deleting an item sets deleted_at, deleted_by, delete_reason, records audit
// content.deleted and writes <resource_type>.deleted in the same transaction. F020-AC-23 (TC-CMS-DEL-01): a
// deleted item is returned by no ordinary read.
func TestSoftDeleteContent(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, true)
	editor := authztest.Staff(e.tenantA, "tenant_content_editor")
	editorCtx := authztest.Ctx(ctx, editor)
	item := tenantItem(ctx, t, e, e.tenantA, "a-del")
	assigned := platformItem(ctx, t, e, "p-1")
	if err := fixtures.Assign(ctx, e.platform, assigned, e.tenantA); err != nil {
		t.Fatal(err)
	}

	if err := e.softDelete(editorCtx, t, item, "trùng nội dung"); err != nil {
		t.Fatal(err)
	}
	var reason string
	var by uuid.UUID
	if err := e.platform.QueryRowContext(ctx, `SELECT delete_reason, deleted_by FROM content.fixture_contents
		WHERE fixture_content_id = $1 AND deleted_at IS NOT NULL`, item).Scan(&reason, &by); err != nil || reason != "trùng nội dung" || by != editor.UserID {
		t.Fatalf("deleted row: %q %s %v", reason, by, err)
	}
	if n := e.audits(ctx, t, "content.deleted", &e.tenantA); n != 1 {
		t.Fatalf("audit content.deleted = %d", n)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'question.deleted' AND tenant_id = $1
		AND envelope->'payload'->>'resource_id' = $2 AND envelope->'payload'->>'deleted_by' = $3`, e.tenantA, item.String(), editor.UserID.String()); n != 1 {
		t.Fatalf("event question.deleted = %d", n)
	}
	// Deleted → invisible to every ordinary read (ent, explicit SQL, get).
	if err := e.read.Read(editorCtx, func(ctx context.Context) error {
		rows, err := fixtures.List(ctx)
		if err != nil {
			return err
		}
		sqlRows, err := fixtures.ListExplicit(ctx)
		if err != nil {
			return err
		}
		for _, r := range append(rows, sqlRows...) {
			if r.ID == item {
				t.Fatal("deleted item listed")
			}
		}
		_, err = fixtures.Get(ctx, item)
		if code(err) != problem.NotFound {
			t.Fatalf("get deleted item: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// Deleting it again, deleting assigned platform content or another tenant's item.
	if err := e.softDelete(editorCtx, t, item, "x"); code(err) != problem.NotFound {
		t.Fatalf("second delete: %v", err)
	}
	if err := e.softDelete(editorCtx, t, assigned, "x"); code(err) != problem.PermissionDenied {
		t.Fatalf("delete of assigned platform content: %v", err)
	}
	other := tenantItem(ctx, t, e, e.tenantB, "b-1")
	if err := e.softDelete(editorCtx, t, other, "x"); code(err) != problem.NotFound {
		t.Fatalf("delete of tenant B's item: %v", err)
	}
	// A released item (current_version_id set) cannot be deleted.
	released := tenantItem(ctx, t, e, e.tenantA, "a-released")
	if _, err := e.platform.ExecContext(ctx, `UPDATE content.fixture_contents SET current_version_id = $2 WHERE fixture_content_id = $1`,
		released, uuid.New()); err != nil {
		t.Fatal(err)
	}
	if err := e.softDelete(editorCtx, t, released, "x"); code(err) != problem.Conflict {
		t.Fatalf("delete of released item: %v", err)
	}
	// Without the permission of the type (school_admin has learning_content.write? use a role without it).
	viewer := authztest.Ctx(ctx, authztest.NoRole(e.tenantA))
	if err := e.softDelete(viewer, t, released, "x"); code(err) != problem.PermissionDenied {
		t.Fatalf("delete without permission: %v", err)
	}
}

// F020-AC-22 (TC-TEN-016, TC-TEN-022): ent Delete() becomes a soft-delete; DELETE/TRUNCATE by learning_app or
// learning_platform is refused by PostgreSQL.
func TestHardDeleteImpossible(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, true)
	item := tenantItem(ctx, t, e, e.tenantA, "a-1")
	if err := e.uow.Do(authztest.Ctx(ctx, authztest.Staff(e.tenantA, "tenant_content_editor")), func(ctx context.Context) error {
		return fixtures.Client(ctx).FixtureContent.DeleteOneID(item).Exec(ctx)
	}); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM content.fixture_contents WHERE fixture_content_id = $1 AND deleted_at IS NOT NULL`, item); n != 1 {
		t.Fatal("ent Delete() removed the row instead of soft-deleting it")
	}
	tables := []string{"content.fixture_contents", "content.content_releases", "content.content_transitions",
		"infra.tenant_resource_assignments", "ref.tenants", "ref.schools", "ref.students"}
	for _, db := range []struct {
		name string
		db   *sql.DB
	}{{"learning_app", e.app}, {"learning_platform", e.platform}} {
		for _, tbl := range tables {
			for _, stmt := range []string{"DELETE FROM " + tbl, "TRUNCATE " + tbl} {
				if _, err := db.db.ExecContext(ctx, stmt); pgCode(err) != "42501" {
					t.Errorf("%s: %s → %v (want 42501 insufficient_privilege)", db.name, stmt, err)
				}
			}
		}
	}
}
