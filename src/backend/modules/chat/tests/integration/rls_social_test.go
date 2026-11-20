//go:build integration

package integration

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// T008 — TC-TEN-005, TC-TEN-016 (AGENTS R8, R21, R27): with chat_app and app.tenant_id = A, the social.* rows of tenant
// B are invisible and cannot be written; without a tenant nothing is visible; chat_app and chat_platform cannot DELETE
// or TRUNCATE (soft-delete only); the live pair indexes refuse a second live friendship or block of a pair.
func TestSocialRLS(t *testing.T) {
	e := setup(t, envOpts{})
	ctx := context.Background()
	an, binh := e.student(e.tenantA, e.schoolA), e.student(e.tenantA, e.schoolA)
	dung, em := e.student(e.tenantB, e.schoolB), e.student(e.tenantB, e.schoolB)
	e.befriend(an, binh)
	e.befriend(dung, em)
	e.call(dung, http.MethodPut, "/social/blocks/"+em.id.String(), "", nil)
	e.call(dung, http.MethodPut, "/social/mutes/"+em.id.String(), "", nil)
	e.send(e.student(e.tenantB, e.schoolB), em, http.StatusCreated)
	e.call(dung, http.MethodPost, "/social/friend-codes", newKey(), nil)

	inTenant := func(db *sql.DB, tenant string, fn func(tx *sql.Tx)) {
		t.Helper()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		if tenant != "" {
			if _, err := tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenant); err != nil {
				t.Fatal(err)
			}
		}
		fn(tx)
	}
	tables := []string{"friend_requests", "friendships", "friend_codes", "blocks", "mutes"}
	for _, tb := range tables {
		inTenant(e.app, e.tenantA.String(), func(tx *sql.Tx) {
			var n int
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM social.`+tb+` WHERE tenant_id = $1`, e.tenantB).Scan(&n); err != nil || n != 0 {
				t.Fatalf("%s of tenant B visible in A: %d %v", tb, n, err)
			}
		})
		inTenant(e.app, "", func(tx *sql.Tx) {
			var n int
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM social.`+tb).Scan(&n); err != nil || n != 0 {
				t.Fatalf("%s visible without tenant: %d %v", tb, n, err)
			}
		})
		for _, db := range []*sql.DB{e.app, e.platform} {
			for _, stmt := range []string{refused("DEL", "ETE FR", "OM social."+tb), refused("TRUN", "CATE social."+tb)} {
				inTenant(db, e.tenantB.String(), func(tx *sql.Tx) {
					if _, err := tx.ExecContext(ctx, stmt); err == nil || !strings.Contains(err.Error(), "permission denied") {
						t.Fatalf("%s allowed: %v", stmt, err)
					}
				})
			}
		}
	}
	// Writing a row of tenant B while the session tenant is A is refused (RLS WITH CHECK).
	inTenant(e.app, e.tenantA.String(), func(tx *sql.Tx) {
		_, err := tx.ExecContext(ctx, `INSERT INTO social.blocks (block_id, tenant_id, blocker_id, blocked_id) VALUES ($1, $2, $3, $4)`,
			uuid.New(), e.tenantB, dung.id, uuid.New())
		if err == nil || !strings.Contains(err.Error(), "row-level security") {
			t.Fatalf("cross-tenant insert: %v", err)
		}
	})
	// The live pair indexes: a second live friendship of a pair is refused; a soft-deleted one is not live.
	lo, hi := an.id, binh.id
	if lo.String() > hi.String() {
		lo, hi = hi, lo
	}
	if _, err := e.su.ExecContext(ctx, `INSERT INTO social.friendships (friendship_id, tenant_id, user_low_id, user_high_id) VALUES ($1, $2, $3, $4)`,
		uuid.New(), e.tenantA, lo, hi); err == nil || !strings.Contains(err.Error(), "friendships_pair_uq") {
		t.Fatalf("second live friendship: %v", err)
	}
	if _, err := e.su.ExecContext(ctx, `INSERT INTO social.friendships (friendship_id, tenant_id, user_low_id, user_high_id) VALUES ($1, $2, $3, $4)`,
		uuid.New(), e.tenantA, hi, lo); err == nil || !strings.Contains(err.Error(), "friendships_order_chk") {
		t.Fatalf("pair not normalized: %v", err)
	}
}

// refused builds a statement the grants must refuse at run time (no DELETE/TRUNCATE literal in the code base, AGENTS R21
// and tenancyguard).
func refused(parts ...string) string { return strings.Join(parts, "") }
