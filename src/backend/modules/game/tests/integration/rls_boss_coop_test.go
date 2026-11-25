//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	gameapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/bosssql"
)

// F103 T011, F103-AC-27 (TC-TEN-001): RLS of the F103 tenant tables — game_app with tenant A never reads tenant B and
// reads nothing without app.tenant_id; no role deletes (soft-delete only, R21); the graded answers are append-only
// (no UPDATE); the online presence of the members is read from the game sessions of the tenant.
func TestRLSBossCoopTables(t *testing.T) {
	ctx := context.Background()
	e := setupBosses(ctx, t)
	e.publishQuests(ctx, t, coopSet(), e.tenantA, e.tenantB)
	e.publishBosses(ctx, t, []command.ContentItem{bossItem(bossSolo, "math.boss.solo", []int{3}, "party_optional", soloBossEdit)}, e.tenantA, e.tenantB)
	var ids []uuid.UUID
	for _, tc := range []struct{ tenant, school uuid.UUID }{{e.tenantA, e.schoolA}, {e.tenantB, e.schoolB}} {
		a := e.newStudent(ctx, t, tc.tenant, tc.school, "active")
		b := e.newStudent(ctx, t, tc.tenant, tc.school, "active")
		ids = append(ids, a.id)
		enc := e.createSolo(t, a, e.device(a).token, bossSolo).body["encounter_id"].(string)
		replyOK(t, e.bossReady(t, a, enc), "ready")
		q := e.questions(t, a, "p1")
		replyOK(t, e.bossAnswer(t, a, enc, "p1", q[0].req, q[0].inst, answerOK), "answer")
		if _, err := e.startParty(t, gameapi.PartyActivityTypeQuest, "math.g3.coop_count", uuid.New(), uuid.New(), a, a, b); err != nil {
			t.Fatal(err)
		}
	}
	tables := []string{"game.boss_encounters", "game.boss_encounter_members", "game.boss_encounter_contributions", "game.boss_phase_plans",
		"game.boss_answer_results", "game.party_quest_sessions"}
	inTenant := func(tenant *uuid.UUID, table string) (own, other int) {
		tx, err := e.app.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		if tenant != nil {
			if _, err := tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenant.String()); err != nil {
				t.Fatal(err)
			}
		}
		// tenancyguard:ok RLS test: counts the rows of each tenant visible to game_app on purpose
		q := `SELECT count(*) FILTER (WHERE tenant_id = $1), count(*) FILTER (WHERE tenant_id <> $1) FROM ` + table
		if err := tx.QueryRowContext(ctx, q, e.tenantA).Scan(&own, &other); err != nil {
			t.Fatalf("%s: %v", table, err)
		}
		return own, other
	}
	for _, table := range tables {
		if own, other := inTenant(&e.tenantA, table); own == 0 || other != 0 {
			t.Errorf("%s with tenant A: own %d, other %d", table, own, other)
		}
		if own, other := inTenant(nil, table); own+other != 0 {
			t.Errorf("%s without tenant: %d rows", table, own+other)
		}
		if _, err := e.app.ExecContext(ctx, deleteAll(table)); err == nil {
			t.Errorf("game_app may delete from %s", table)
		}
		if _, err := e.platform.ExecContext(ctx, deleteAll(table)); err == nil {
			t.Errorf("game_platform may delete from %s", table)
		}
	}
	if _, err := e.platform.ExecContext(ctx, `UPDATE game.boss_answer_results SET counted = false`); err == nil {
		t.Error("the graded answers are append-only")
	}
	// Online members from the game sessions of the tenant (heartbeat within the timeout).
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	off := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	if r := e.start(t, an, e.device(an), newKey(), nil); r.status != http.StatusCreated {
		t.Fatalf("session: %d %s", r.status, r.raw)
	}
	var online map[uuid.UUID]bool
	err := e.read.Read(ctxOf(t, e, an), func(ctx context.Context) error {
		var err error
		online, err = bosssql.Presence{}.Online(ctx, e.tenantA, []uuid.UUID{an.id, off.id, ids[1]}, e.clock.Now().Add(-45*time.Second))
		return err
	})
	if err != nil || !online[an.id] || online[off.id] || online[ids[1]] {
		t.Fatalf("online from the sessions of tenant A: %v %v", err, online)
	}
}
