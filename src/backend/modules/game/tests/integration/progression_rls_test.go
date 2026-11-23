//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
)

// F074-AC-12 (TC-ECO-008): the ledger, the level history and the reward grants are append-only — game_app (and
// game_platform) UPDATE/DELETE are refused — and tenant A never reads the rows of tenant B (RLS).
func TestProgressionRLSAndAppendOnly(t *testing.T) {
	ctx := context.Background()
	e := setupProgression(ctx, t)
	uc := e.progressionUC()
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	e.mustDeliver(ctx, t, uc, e.questEvent(an, "main")) // 150 XP: level 2 with the seed curve
	e.mustDeliver(ctx, t, uc, e.questEvent(dung, "main"))
	for _, table := range []string{"gamification.xp_ledger", "game.level_history"} {
		var n int
		err := e.uow.InTenantTx(ctx, scopeOf(t, e.tenantA), func(ctx context.Context) error {
			return txOf(ctx).QueryRowContext(ctx, `SELECT count(*) FROM `+table+` WHERE student_id = $1`, dung.id).Scan(&n)
		})
		if err != nil || n != 0 {
			t.Fatalf("%s of tenant B visible in A: %d %v", table, n, err)
		}
		err = e.uow.InTenantTx(ctx, scopeOf(t, e.tenantA), func(ctx context.Context) error {
			return txOf(ctx).QueryRowContext(ctx, `SELECT count(*) FROM `+table+` WHERE student_id = $1`, an.id).Scan(&n)
		})
		if err != nil || n != 1 {
			t.Fatalf("%s of A in A: %d %v", table, n, err)
		}
	}
	for _, db := range []struct {
		name string
		exec func(ctx context.Context, q string) error
	}{
		{"game_app", func(ctx context.Context, q string) error { _, err := e.app.ExecContext(ctx, q); return err }},
		{"game_platform", func(ctx context.Context, q string) error { _, err := e.platform.ExecContext(ctx, q); return err }},
	} {
		for _, stmt := range []string{
			`UPDATE gamification.xp_ledger SET amount = 9999`,
			`DELETE FROM gamification.xp_ledger`,
			`UPDATE game.level_history SET to_level = 60`,
			`DELETE FROM game.level_history`,
			`UPDATE gamification.reward_grants SET grants = '[]'::jsonb`,
			`DELETE FROM gamification.reward_grants`,
			`UPDATE game.progression_levels SET xp_cumulative = 0`,
		} {
			err := db.exec(ctx, stmt)
			if err == nil || !strings.Contains(err.Error(), "permission denied") {
				t.Fatalf("%s may run %q: %v", db.name, stmt, err)
			}
		}
	}
	if rows := e.ledger(ctx, t, an); len(rows) != 1 || rows[0].amount != 150 {
		t.Fatalf("ledger changed: %+v", rows)
	}
}
