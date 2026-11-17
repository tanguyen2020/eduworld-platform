//go:build integration

package entx_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/ent"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/ent/widget"
)

// One cluster for all cases (each subtest uses its own business keys).
func TestEntx(t *testing.T) {
	e := sample.Setup(context.Background(), t)
	tenancy.Configure(tenancy.Hooks{Recorder: e.Audit})
	t.Cleanup(func() { tenancy.Configure(tenancy.Hooks{}) })
	a := authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantA, "school_admin"))
	b := authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantB, "school_admin"))
	for name, fn := range map[string]func(*testing.T, *sample.Env, context.Context, context.Context){
		"TenantFilterAndScope": tenantFilterAndScope, "PlatformScopeAndGlobal": platformScopeAndGlobal,
		"SoftDelete": softDelete, "UnitOfWorkCommitRollback": unitOfWorkCommitRollback, "ReadSessionIsReadOnly": readSessionIsReadOnly,
	} {
		t.Run(name, func(t *testing.T) { fn(t, e, a, b) })
	}
}

func outboxMsg() outbox.Message {
	return outbox.Message{Type: "sample.checked", Subject: outbox.Subject{Type: "sample", ID: "x"}, Payload: map[string]any{}}
}

func mustDo(t *testing.T, e *sample.Env, ctx context.Context, fn func(ctx context.Context) error) {
	t.Helper()
	if err := e.UoW.Do(ctx, fn); err != nil {
		t.Fatal(err)
	}
}

func createWidget(t *testing.T, e *sample.Env, ctx context.Context, code string) *ent.Widget {
	t.Helper()
	var w *ent.Widget
	mustDo(t, e, ctx, func(ctx context.Context) error {
		var err error
		w, err = sample.Client(ctx).Widget.Create().SetCode(code).SetName("W " + code).Save(ctx)
		return err
	})
	return w
}

func countWidgets(t *testing.T, e *sample.Env, ctx context.Context) int {
	t.Helper()
	var n int
	if err := e.Read.Read(ctx, func(ctx context.Context) error {
		var err error
		n, err = sample.Client(ctx).Widget.Query().Count(ctx)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return n
}

// F003-AC-02 (TC-TEN-021), F003-AC-06 (TC-TEN-005): scope filter, missing scope, role and app.tenant_id.
func tenantFilterAndScope(t *testing.T, e *sample.Env, a, b context.Context) {
	wa := createWidget(t, e, a, "a-1")
	createWidget(t, e, b, "b-1")
	if wa.TenantID != authztest.TenantA {
		t.Fatalf("tenant_id not taken from the scope: %v", wa.TenantID)
	}
	if n := countWidgets(t, e, a); n != 1 {
		t.Fatalf("tenant A sees %d widgets", n)
	}
	// No scope → ErrMissingTenantScope, no rows.
	err := e.Read.Read(context.Background(), func(ctx context.Context) error {
		_, err := sample.Client(ctx).Widget.Query().All(ctx)
		return err
	})
	if !errors.Is(err, entx.ErrMissingTenantScope) {
		t.Fatalf("no scope: %v", err)
	}
	// Outside UnitOfWork/ReadSession every statement fails.
	if _, err := sample.Client(a).Widget.Query().All(a); !errors.Is(err, entx.ErrNoTransaction) {
		t.Fatalf("outside tx: %v", err)
	}
	// Tenant scope runs as learning_app with app.tenant_id; RLS refuses a NULL tenant row from _app.
	mustDo(t, e, a, func(ctx context.Context) error {
		tx, _ := entx.Tx(ctx)
		var role, tenant string
		if err := tx.QueryRowContext(ctx, "SELECT current_user, current_setting('app.tenant_id', true)").Scan(&role, &tenant); err != nil {
			return err
		}
		if role != "learning_app" || tenant != authztest.TenantA.String() {
			t.Errorf("role %s tenant %s", role, tenant)
		}
		return nil
	})
	err = e.UoW.Do(a, func(ctx context.Context) error {
		tx, _ := entx.Tx(ctx)
		_, err := tx.ExecContext(ctx, `INSERT INTO sample.templates (id, tenant_id, created_at, updated_at, name) VALUES ($1, NULL, now(), now(), 'x')`, uuid.New())
		return err
	})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Fatalf("RLS must refuse a NULL tenant row from learning_app: %v", err)
	}
	// Create with a tenant outside the scope is refused by the hook.
	err = e.UoW.Do(a, func(ctx context.Context) error {
		_, err := sample.Client(ctx).Widget.Create().SetTenantID(authztest.TenantB).SetCode("x").SetName("x").Save(ctx)
		return err
	})
	if !errors.Is(err, entx.ErrTenantOutOfScope) {
		t.Fatalf("foreign tenant create: %v", err)
	}
	// Update/delete of another tenant's row affect nothing (mutation predicates).
	mustDo(t, e, b, func(ctx context.Context) error {
		n, err := sample.Client(ctx).Widget.Update().Where(widget.ID(wa.ID)).SetName("hijack").Save(ctx)
		if n != 0 {
			t.Errorf("tenant B updated %d rows of tenant A", n)
		}
		return err
	})
}

// T*, G and platform scope (MULTITENANCY §4.4).
func platformScopeAndGlobal(t *testing.T, e *sample.Env, a, b context.Context) {
	ps, err := authz.SystemPlatformScope("sample-job")
	if err != nil {
		t.Fatal(err)
	}
	pctx := tenancy.WithScope(context.Background(), ps)
	mustDo(t, e, pctx, func(ctx context.Context) error {
		tx, _ := entx.Tx(ctx)
		var role string
		if err := tx.QueryRowContext(ctx, "SELECT current_user").Scan(&role); err != nil {
			return err
		}
		if role != "learning_platform" {
			t.Errorf("platform scope runs as %s", role)
		}
		_, err := sample.Client(ctx).Template.Create().SetName("platform template").Save(ctx)
		return err
	})
	mustDo(t, e, a, func(ctx context.Context) error {
		_, err := sample.Client(ctx).Template.Create().SetName("tenant A template").Save(ctx)
		return err
	})
	count := func(ctx context.Context) (n int) {
		if err := e.Read.Read(ctx, func(ctx context.Context) error {
			var err error
			n, err = sample.Client(ctx).Template.Query().Count(ctx)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := count(pctx); n != 1 {
		t.Fatalf("platform scope sees %d templates (only tenant_id NULL)", n)
	}
	if n := count(a); n != 1 {
		t.Fatalf("tenant A sees %d templates (only its own)", n)
	}
	// Platform scope sees no T row.
	if n := countWidgets(t, e, pctx); n != 0 {
		t.Fatalf("platform scope sees %d widgets", n)
	}
	// G needs no scope.
	mustDo(t, e, context.Background(), func(ctx context.Context) error {
		u, err := sample.Client(ctx).Unit.Create().SetSymbol("kg").Save(ctx)
		if err != nil {
			return err
		}
		return sample.Client(ctx).Unit.DeleteOneID(u.ID).Exec(ctx)
	})
	var units int
	_ = e.Read.Read(context.Background(), func(ctx context.Context) error {
		var err error
		units, err = sample.Client(ctx).Unit.Query().Count(ctx)
		return err
	})
	if units != 0 {
		t.Fatalf("soft-deleted global row still visible: %d", units)
	}
}

// F003-AC-03, F003-AC-04, F003-AC-05, F003-AC-34 (TC-TEN-015, TC-TEN-022).
func softDelete(t *testing.T, e *sample.Env, a, b context.Context) {
	w := createWidget(t, e, a, "del-1")
	before := countWidgets(t, e, a)
	mustDo(t, e, a, func(ctx context.Context) error {
		return entx.SoftDelete(ctx, "widget", w.ID, "duplicate", func(ctx context.Context) error {
			return sample.Client(ctx).Widget.DeleteOneID(w.ID).Exec(ctx)
		})
	})
	if n := countWidgets(t, e, a); n != before-1 {
		t.Fatalf("deleted row still read: %d → %d", before, n)
	}
	var deletedAt, reason, deletedBy any
	if err := e.Platform.QueryRow(`SELECT deleted_at, delete_reason, deleted_by FROM sample.widgets WHERE id = $1`, w.ID).Scan(&deletedAt, &reason, &deletedBy); err != nil {
		t.Fatalf("row physically gone (must be soft-deleted): %v", err)
	}
	if deletedAt == nil || reason != "duplicate" || deletedBy == nil {
		t.Fatalf("deleted_at=%v reason=%v deleted_by=%v", deletedAt, reason, deletedBy)
	}
	var events string
	if err := e.Migrator.QueryRow(`SELECT string_agg(event_type, ',' ORDER BY event_type) FROM infra.outbox_events
		WHERE envelope->'subject'->>'id' = $1 OR envelope->'payload'->>'target_id' = $1`, w.ID.String()).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != "audit.recorded,widget.deleted" {
		t.Fatalf("outbox events = %q, want audit + widget.deleted in the same transaction", events)
	}
	// Only Bypass(trash) reads it back.
	tctx, err := tenancy.Bypass(a, tenancy.ReasonTrash, tenancy.WithMode(tenancy.ModeRead))
	if err != nil {
		t.Fatal(err)
	}
	var found int
	if err := e.Read.Read(tctx, func(ctx context.Context) error {
		var err error
		found, err = sample.Client(ctx).Widget.Query().Where(widget.ID(w.ID)).Count(ctx)
		return err
	}); err != nil || found != 1 {
		t.Fatalf("trash read: %d %v", found, err)
	}
	// Same business key can be created again (partial unique index).
	createWidget(t, e, a, "del-1")
	// TA has no soft-delete: delete is refused.
	err = e.UoW.Do(a, func(ctx context.Context) error {
		l, err := sample.Client(ctx).WidgetLog.Create().SetMessage("x").Save(ctx)
		if err != nil {
			return err
		}
		return sample.Client(ctx).WidgetLog.DeleteOneID(l.ID).Exec(ctx)
	})
	if !errors.Is(err, entx.ErrHardDeleteForbidden) {
		t.Fatalf("TA delete: %v", err)
	}
}

// F003-AC-08: commit on nil, rollback on error and panic, nested Do joins the outer transaction.
func unitOfWorkCommitRollback(t *testing.T, e *sample.Env, a, b context.Context) {
	before := countWidgets(t, e, a)
	errBoom := errors.New("boom")
	err := e.UoW.Do(a, func(ctx context.Context) error {
		if _, err := sample.Client(ctx).Widget.Create().SetCode("rb-1").SetName("x").Save(ctx); err != nil {
			return err
		}
		if _, err := e.Writer.Publish(ctx, outboxMsg()); err != nil {
			return err
		}
		return errBoom
	})
	if !errors.Is(err, errBoom) || countWidgets(t, e, a) != before {
		t.Fatalf("rollback on error: %v", err)
	}
	func() {
		defer func() { _ = recover() }()
		_ = e.UoW.Do(a, func(ctx context.Context) error {
			_, _ = sample.Client(ctx).Widget.Create().SetCode("rb-2").SetName("x").Save(ctx)
			panic("boom")
		})
	}()
	if countWidgets(t, e, a) != before {
		t.Fatal("panic did not roll back")
	}
	mustDo(t, e, a, func(ctx context.Context) error {
		outer, _ := entx.Tx(ctx)
		return e.UoW.Do(ctx, func(ctx context.Context) error {
			inner, _ := entx.Tx(ctx)
			if inner != outer {
				t.Error("nested Do opened a second transaction")
			}
			_, err := sample.Client(ctx).Widget.Create().SetCode("nested").SetName("x").Save(ctx)
			return err
		})
	})
	if countWidgets(t, e, a) != before+1 {
		t.Fatal("nested write not committed")
	}
	var unpublished int
	_ = e.Migrator.QueryRow(`SELECT count(*) FROM infra.outbox_events WHERE event_type = 'sample.checked'`).Scan(&unpublished)
	if unpublished != 0 {
		t.Fatal("outbox row survived the rollback")
	}
}

// F003-AC-09 (read side): ReadSession is READ ONLY.
func readSessionIsReadOnly(t *testing.T, e *sample.Env, a, b context.Context) {
	err := e.Read.Read(a, func(ctx context.Context) error {
		_, err := sample.Client(ctx).Widget.Create().SetCode("ro").SetName("x").Save(ctx)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("write in ReadSession: %v", err)
	}
}
