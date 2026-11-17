//go:build integration

package projection_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/projection"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample"
)

type sourceRow struct {
	id      uuid.UUID
	name    string
	version int64
}

// F003-AC-39 (TC-SVC-014): rebuild through the facade, upsert by source_version, idempotent, no wipe on
// facade failure.
func TestRebuild(t *testing.T) {
	ctx := context.Background()
	e := sample.Setup(ctx, t)
	tenancy.Configure(tenancy.Hooks{Recorder: e.Audit})
	t.Cleanup(func() { tenancy.Configure(tenancy.Hooks{}) })
	cols := []string{"tenant_id", "name"}
	upsertSQL, err := projection.UpsertSQL("sample.ref_widgets", "id", cols)
	if err != nil {
		t.Fatal(err)
	}
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	source := []sourceRow{{a, "A v2", 2}, {b, "B v1", 1}, {c, "C v1", 1}}
	failAfter := -1
	calls := 0
	fetch := func(ctx context.Context) error {
		calls++
		_, err := modcall.Call(tenancy.WithModule(ctx, "learning"), modcall.Op{Caller: "learning", Callee: "identity", Operation: "listProjectionSnapshot",
			AllowedCallers: []string{"learning", "game", "competition", "chat"}}, func(context.Context) ([]projection.Row, error) {
			if failAfter >= 0 && calls > failAfter {
				return nil, errors.New("identity database down")
			}
			return nil, nil
		})
		return err
	}
	paged := func(ctx context.Context, cursor string) ([]projection.Row, string, error) {
		if err := fetch(ctx); err != nil {
			return nil, "", err
		}
		start := 0
		if cursor != "" {
			start = 2
		}
		end := min(start+2, len(source))
		var rows []projection.Row
		for _, s := range source[start:end] {
			rows = append(rows, projection.Row{Key: s.id.String(), SourceVersion: s.version,
				Values: map[string]any{"tenant_id": authztest.TenantA, "name": s.name}})
		}
		next := ""
		if end < len(source) {
			next = "page2"
		}
		return rows, next, nil
	}
	upsert := func(ctx context.Context, rows []projection.Row) error {
		bctx, err := tenancy.Bypass(ctx, tenancy.ReasonPlatformJob, tenancy.WithMode(tenancy.ModeWrite))
		if err != nil {
			return err
		}
		return e.UoW.Do(bctx, func(ctx context.Context) error {
			tx, _ := entx.Tx(ctx)
			for _, r := range rows {
				if _, err := tx.ExecContext(ctx, upsertSQL, projection.Args(r, cols)...); err != nil {
					return err
				}
			}
			return nil
		})
	}
	var reg projection.Registry
	if err := reg.Register(projection.Table{Module: "learning", Source: "identity", Name: "ref.widgets", Fetch: paged, Upsert: upsert}); err != nil {
		t.Fatal(err)
	}

	// Drift: A stale, B missing.
	if _, err := e.Platform.Exec(`INSERT INTO sample.ref_widgets VALUES ($1, $2, 'A v1', 1)`, a, authztest.TenantA); err != nil {
		t.Fatal(err)
	}
	snapshot := func() map[uuid.UUID]string {
		rows, err := e.Platform.Query(`SELECT id, name || '@' || source_version FROM sample.ref_widgets`)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rows.Close() }()
		out := map[uuid.UUID]string{}
		for rows.Next() {
			var id uuid.UUID
			var v string
			_ = rows.Scan(&id, &v)
			out[id] = v
		}
		return out
	}
	stats, err := reg.Rebuild(ctx, "learning", "identity", "")
	if err != nil {
		t.Fatal(err)
	}
	want := map[uuid.UUID]string{a: "A v2@2", b: "B v1@1", c: "C v1@1"}
	if got := snapshot(); len(got) != 3 || got[a] != want[a] || got[b] != want[b] || got[c] != want[c] || stats["ref.widgets"].Rows != 3 {
		t.Fatalf("after rebuild: %v (stats %v)", got, stats)
	}
	// Idempotent.
	if _, err := reg.Rebuild(ctx, "learning", "identity", "ref.widgets"); err != nil {
		t.Fatal(err)
	}
	if got := snapshot(); got[a] != want[a] || len(got) != 3 {
		t.Fatalf("second run changed data: %v", got)
	}
	// A newer version written by the consumer meanwhile is kept.
	if _, err := e.Platform.Exec(`UPDATE sample.ref_widgets SET name = 'A v5', source_version = 5 WHERE id = $1`, a); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Rebuild(ctx, "learning", "identity", ""); err != nil {
		t.Fatal(err)
	}
	if got := snapshot(); got[a] != "A v5@5" {
		t.Fatalf("newer row overwritten: %v", got[a])
	}
	// Facade failure on page 2: error, projection intact.
	calls, failAfter = 0, 1
	if _, err := reg.Rebuild(ctx, "learning", "identity", ""); err == nil {
		t.Fatal("facade failure not reported")
	}
	if got := snapshot(); len(got) != 3 || got[a] != "A v5@5" {
		t.Fatalf("projection damaged after failure: %v", got)
	}
	if _, err := reg.Rebuild(ctx, "learning", "nobody", ""); err == nil {
		t.Fatal("unknown source accepted")
	}
}
