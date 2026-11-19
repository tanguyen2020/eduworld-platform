//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// F013-AC-30, AC-31 (schema part): memberships_for_user returns exactly the user's memberships to identity_app
// (no BYPASSRLS) and nothing else; login_events rows with tenant_id NULL are written by identity_platform only;
// the journal is append-only.
func TestGameLoginSchema(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	a := seedTenant(ctx, t, e.platform, "pgd-gl-a")
	b := seedTenant(ctx, t, e.platform, "pgd-gl-b")
	st := enrolled(ctx, t, e.platform, a, "hs.gl")
	other := enrolled(ctx, t, e.platform, b, "hs.gl.b")

	// identity_app without any tenant set: RLS hides the enrollments, the function still returns the user's rows.
	if n := count(ctx, t, e.app, `SELECT count(*) FROM org.enrollments`); n != 0 {
		t.Fatalf("app sees %d enrollments without a tenant", n)
	}
	if n := count(ctx, t, e.app, `SELECT count(*) FROM identity.memberships_for_user($1)`, st); n != 1 {
		t.Fatalf("memberships of the student = %d", n)
	}
	var tenant string
	if err := e.app.QueryRowContext(ctx, `SELECT tenant_id FROM identity.memberships_for_user($1)`, st).Scan(&tenant); err != nil || tenant != a.tenant {
		t.Fatalf("membership tenant = %s %v", tenant, err)
	}
	if n := count(ctx, t, e.app, `SELECT count(*) FROM identity.memberships_for_user($1)`, uuid.New()); n != 0 {
		t.Fatalf("unknown user memberships = %d", n)
	}
	_ = other

	insert := `INSERT INTO identity.login_events (login_event_id, tenant_id, created_at, occurred_at, client, result)
		VALUES ($1, $2::uuid, now(), now(), 'eduworld-game-backend', 'invalid_credentials')`
	// tenant_id NULL by identity_app → refused by RLS; by identity_platform → accepted.
	if _, err := e.app.ExecContext(ctx, insert, uuid.New(), nil); err == nil {
		t.Fatal("identity_app wrote a login event without tenant")
	}
	if _, err := e.platform.ExecContext(ctx, insert, uuid.New(), nil); err != nil {
		t.Fatalf("identity_platform: %v", err)
	}
	// A tenant row by identity_app within its tenant.
	tx, err := e.app.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, a.tenant)
	if _, err := tx.ExecContext(ctx, insert, uuid.New(), a.tenant); err != nil {
		t.Fatalf("tenant row: %v", err)
	}
	if _, err := tx.ExecContext(ctx, insert, uuid.New(), b.tenant); err == nil {
		t.Fatal("identity_app wrote a row of another tenant")
	}
	_ = tx.Rollback()
	// Append-only.
	if _, err := e.platform.ExecContext(ctx, `UPDATE identity.login_events SET result = 'success'`); err == nil {
		t.Fatal("login_events is updatable")
	}
}
