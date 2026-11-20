//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// T028 (F091-AC-23, AC-25): joining a hub instance sends presence.update (area, room) to the gateway on the student's
// own channel, leaving it (grace over / other area) clears or replaces it; no client message or REST operation returns
// presence in F091 and no PostgreSQL table stores it.
func TestHubPresenceUpdate(t *testing.T) {
	e := setup(t, envOpts{grace: 500 * time.Millisecond})
	e.node("core-1")
	an := e.student(e.tenantA, e.schoolA, 3)
	user := e.channel(rt.UserChannel(an.id))
	ra := e.mustJoin(an, "main_hub")
	c := next(t, user, "control:presence.update", nil, 5*time.Second)["presence"].(map[string]any)
	if c["area_id"] != "main_hub" || c["room_id"] != ra.RoomID {
		t.Fatalf("join presence %v", c)
	}
	e.presence(an, rt.PresenceOffline, rt.PresenceDisconnected)
	c = next(t, user, "control:presence.update", nil, 5*time.Second)["presence"].(map[string]any)
	if c["area_id"] != nil || c["room_id"] != nil {
		t.Fatalf("leave presence %v", c)
	}
	// AC-25: no presence table in eduworld_competition, no presence in any server message of the hub.
	if n := e.count(`SELECT count(*) FROM information_schema.tables WHERE table_name ILIKE '%presence%'`); n != 0 {
		t.Fatalf("%d presence tables", n)
	}
	b, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), "specs", "asyncapi", "eduworld-realtime.v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// The only presence message to clients is presence.friend.updated, sent by module chat (F095), never by F091.
	if strings.Count(string(b), "name: presence.") != 2 {
		t.Fatal("unexpected presence message in the AsyncAPI")
	}
	_ = context.Background()
}

// T010 (TC-TEN-005, TC-TEN-016): the tables of F091 are tenant-isolated by RLS (no tenant set → 0 rows; another tenant →
// 0 rows) and the application roles cannot DELETE or TRUNCATE them.
func TestRLSAndNoDelete(t *testing.T) {
	e := setup(t, envOpts{})
	e.node("core-1")
	an := e.student(e.tenantA, e.schoolA, 3)
	e.mustJoin(an, "main_hub")
	e.exec(`INSERT INTO competition.integrity_signals (signal_id, tenant_id, student_id, signal_type) VALUES (gen_random_uuid(), $1, $2, 'hub_speed')`,
		an.tenant, an.id)
	ctx := context.Background()
	tables := []string{"multiplayer.rooms", "multiplayer.session_memberships", "competition.integrity_signals", "ref.students",
		"ref.player_profiles", "ref.friendships", "ref.blocks", "ref.area_unlocks"}
	for _, tb := range tables {
		var n int
		if err := e.app.QueryRowContext(ctx, `SELECT count(*) FROM `+tb).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s without tenant: %d rows (%v)", tb, n, err)
		}
		tx, err := e.app.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, e.tenantB.String()); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM `+tb).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s as tenant B: %d rows (%v)", tb, n, err)
		}
		_ = tx.Rollback()
		// tenancyguard:ok negative test: the application role must be refused DELETE/TRUNCATE (ADR-0028)
		for _, stmt := range []string{"DELETE FROM " + tb, "TRUNCATE " + tb} {
			// tenancyguard:ok negative test: the application role must be refused DELETE/TRUNCATE (ADR-0028)
			if _, err := e.app.ExecContext(ctx, stmt); err == nil || !strings.Contains(err.Error(), "permission denied") {
				t.Errorf("%s allowed for competition_app: %v", stmt, err)
			}
		}
	}
	// own tenant sees its rows
	tx, _ := e.app.BeginTx(ctx, nil)
	defer func() { _ = tx.Rollback() }()
	_, _ = tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, an.tenant.String())
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM multiplayer.rooms`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("own tenant rooms %d %v", n, err)
	}
}
