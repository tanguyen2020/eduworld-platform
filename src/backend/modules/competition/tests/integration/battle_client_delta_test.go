//go:build integration

package integration

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/battle"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
)

// pastOutcome inserts a match of k (status) ended ago with k's outcome (what F105 records), for the loss streak of
// delta F109 2026-11-27.
func (b *benv) pastOutcome(k kid, outcome, status string, ago time.Duration) uuid.UUID {
	b.t.Helper()
	id := uuid.New()
	at := b.clock.Now().Add(-ago)
	b.exec(`INSERT INTO competition.matches (match_id, mode, matchmaking_type, subject_id, grade_level, ranked, is_bot, scope_kind,
		ruleset_version_id, tenant_ids, status, started_at, ended_at) VALUES ($1, 'battle_1v1', 'skill', $2, $3, true, false, 'school',
		'01936a00-0000-7000-8000-000000001041', ARRAY[$4]::uuid[], $5, $6, $6)`, id, b.subject, k.grade, k.tenant, status, at)
	b.exec(`INSERT INTO competition.match_participants (participant_id, tenant_id, match_id, student_id, school_id, team_no, outcome)
		VALUES ($1, $2, $3, $4, $5, 0, $6)`, uuid.New(), k.tenant, id, k.id, k.school, outcome)
	return id
}

// recentOutcomes runs RecentOutcomes in the TenantScope of tenant (the transaction of the battle engine).
func (b *benv) recentOutcomes(tenant, student uuid.UUID, limit int) []string {
	b.t.Helper()
	sc, err := authz.EventScope(tenant)
	if err != nil {
		b.t.Fatal(err)
	}
	var out []string
	if err := b.uow.InTenantTx(context.Background(), sc, func(ctx context.Context) error {
		var err error
		out, err = persistence.Battle{}.RecentOutcomes(ctx, student, limit)
		return err
	}); err != nil {
		b.t.Fatal(err)
	}
	return out
}

// Delta F109 2026-11-27 (TC-TEN): the loss streak reads only the completed matches of the session tenant, newest
// first, without no_contest, cancelled matches or soft-deleted rows; a student id seen in tenant B never counts in A.
func TestBattleDeltaRecentOutcomesTenantScope(t *testing.T) {
	b := setupBattle(t)
	k := b.kid(b.tenantA, b.schoolA, 4)
	b.pastOutcome(k, battle.OutcomeWin, "completed", 5*time.Hour)
	b.pastOutcome(k, battle.OutcomeLoss, "completed", 4*time.Hour)
	b.pastOutcome(k, battle.OutcomeNoContest, "completed", 3*time.Hour)
	b.pastOutcome(k, battle.OutcomeLoss, "cancelled", 150*time.Minute)
	gone := b.pastOutcome(k, battle.OutcomeLoss, "completed", 2*time.Hour)
	b.exec(`UPDATE competition.match_participants SET deleted_at = now() WHERE match_id = $1`, gone)
	b.pastOutcome(k, battle.OutcomeForfeit, "completed", time.Hour)
	// the same student id in tenant B (another school) — never visible from tenant A.
	other := kid{id: k.id, tenant: b.tenantB, school: b.schoolB, grade: 4}
	b.pastOutcome(other, battle.OutcomeLoss, "completed", 30*time.Minute)

	if got := b.recentOutcomes(b.tenantA, k.id, 5); !slices.Equal(got, []string{battle.OutcomeForfeit, battle.OutcomeLoss, battle.OutcomeWin}) {
		t.Fatalf("tenant A outcomes %v", got)
	}
	if got := b.recentOutcomes(b.tenantA, k.id, 2); !slices.Equal(got, []string{battle.OutcomeForfeit, battle.OutcomeLoss}) {
		t.Fatalf("limit 2: %v", got)
	}
	if got := b.recentOutcomes(b.tenantB, k.id, 5); !slices.Equal(got, []string{battle.OutcomeLoss}) {
		t.Fatalf("tenant B outcomes %v", got)
	}
	if !battle.LossStreak(b.recentOutcomes(b.tenantA, k.id, 3), 2) || battle.LossStreak(b.recentOutcomes(b.tenantA, k.id, 3), 3) {
		t.Fatal("loss streak of tenant A")
	}
}
