//go:build integration

package testkit_test

import (
	"context"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// F003-AC-31 (TC-TEN-005): the kit brings up PostgreSQL (migrated), NATS (module streams), Valkey and
// gives tenant-scoped contexts that set app.tenant_id.
func TestKit(t *testing.T) {
	k := testkit.New(t, "game")
	var rev string
	if err := k.Migrator.QueryRow(`SELECT version FROM atlas_schema_revisions.atlas_schema_revisions ORDER BY version DESC LIMIT 1`).Scan(&rev); err != nil || rev == "" {
		t.Fatalf("migrations not applied: %v", err)
	}
	if _, err := k.JS.Stream(context.Background(), natsx.EventStream("game")); err != nil {
		t.Fatalf("stream: %v", err)
	}
	if k.Valkey == "" {
		t.Fatal("valkey address empty")
	}
	for _, tenant := range []string{authztest.TenantA.String(), authztest.TenantB.String()} {
		ctx := k.As(authztest.Student(authztest.TenantA))
		if tenant == authztest.TenantB.String() {
			ctx = k.As(authztest.Student(authztest.TenantB))
		}
		if err := k.UoW.Do(ctx, func(ctx context.Context) error {
			tx, _ := entx.Tx(ctx)
			var got string
			if err := tx.QueryRowContext(ctx, `SELECT current_setting('app.tenant_id', true)`).Scan(&got); err != nil {
				return err
			}
			if got != tenant {
				t.Errorf("app.tenant_id = %s, want %s", got, tenant)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}
