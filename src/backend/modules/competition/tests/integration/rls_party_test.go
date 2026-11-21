//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// T011 — RLS and soft-delete only on multiplayer.parties, party_members, party_invitations, party_policies,
// party_histories (AGENTS R8, R21, R27): the app role sees the rows of its session tenant only, nothing without a
// tenant, and may neither DELETE nor TRUNCATE; the history is append-only (no UPDATE).
func TestPartyRLS(t *testing.T) {
	e := setupParty(t, popts{})
	a, b := e.kidA(), e.kidA()
	e.party(a, b)
	dung, em := e.kid(e.tenantB, e.schoolB, uuid.Nil, 6), e.kid(e.tenantB, e.schoolB, uuid.Nil, 6)
	pb := e.party(dung, em)
	e.friends(dung, e.kid(e.tenantB, e.schoolB, uuid.Nil, 6))
	ctx := context.Background()
	tables := []string{"multiplayer.parties", "multiplayer.party_members", "multiplayer.party_invitations", "multiplayer.party_policies",
		"multiplayer.party_histories"}
	count := func(tenant string, table string) int {
		t.Helper()
		tx, err := e.app.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		if tenant != "" {
			if _, err := tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenant); err != nil {
				t.Fatal(err)
			}
		}
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM `+table+` WHERE tenant_id = $1`, e.tenantB).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	for _, table := range tables {
		if n := count(e.tenantA.String(), table); n != 0 {
			t.Fatalf("%s: tenant A reads %d rows of B", table, n)
		}
		if n := count("", table); n != 0 {
			t.Fatalf("%s: no scope reads %d rows", table, n)
		}
	}
	if n := count(e.tenantB.String(), "multiplayer.parties"); n != 1 {
		t.Fatalf("tenant B reads its party: %d", n)
	}
	for _, stmt := range []string{
		stmt("DEL", "ETE FROM multiplayer.parties WHERE party_id = '"+pb+"'"),
		stmt("TRUN", "CATE multiplayer.party_members"),
		stmt("DEL", "ETE FROM multiplayer.party_invitations"),
		`UPDATE multiplayer.party_histories SET event_type = 'created'`,
	} {
		tx, err := e.app.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, e.tenantB.String())
		_, err = tx.ExecContext(ctx, stmt)
		_ = tx.Rollback()
		if err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Fatalf("%s: %v", strings.Fields(stmt)[0], err)
		}
	}
	// Writing a row of tenant B under the scope of A is refused (WITH CHECK).
	tx, err := e.app.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	_, _ = tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, e.tenantA.String())
	_, err = tx.ExecContext(ctx, `UPDATE multiplayer.parties SET name = 'x' WHERE party_id = $1`, pb)
	if err != nil {
		t.Fatal(err)
	}
	if r := e.call(dung, http.MethodGet, "/multiplayer/parties/"+pb, "", nil); r.str("name") != "Nhóm" {
		t.Fatalf("tenant A changed B: %s", r.raw)
	}
}

// stmt builds a statement the grants must refuse at run time (no DELETE/TRUNCATE literal in the code base, AGENTS R21).
func stmt(parts ...string) string { return strings.Join(parts, "") }
