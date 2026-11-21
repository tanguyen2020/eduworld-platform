//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// T007 — RLS and soft-delete only on competition.match_participants, duels, competition_restrictions and ref.schools
// (AGENTS R8, R21, R27, DATABASE_DESIGN §4): the app role sees the rows of its session tenant only, nothing without a
// tenant, writes no row of another tenant, may neither DELETE nor TRUNCATE; a soft-delete (UPDATE deleted_at) works
// under RLS and the _live views hide it.
func TestMatchmakingRLS(t *testing.T) {
	m := setupMM(t)
	ctx := context.Background()
	a := m.kid(m.tenantA, m.schoolA, m.classA, 3)
	b := m.kid(m.tenantB, m.schoolB, uuid.Nil, 3)
	b2 := m.kid(m.tenantB, m.schoolB, uuid.Nil, 3)
	now := m.clock.Now()
	match := uuid.New()
	m.exec(`INSERT INTO competition.matches (match_id, mode, matchmaking_type, subject_id, grade_level, ranked, scope_kind, ruleset_version_id, tenant_ids)
		VALUES ($1, 'battle_1v1', 'skill', $2, 3, true, 'school', '01936a00-0000-7000-8000-000000001041', ARRAY[$3]::uuid[])`, match, m.subject, m.tenantB)
	m.exec(`INSERT INTO competition.match_participants (participant_id, tenant_id, match_id, student_id, school_id, team_no)
		VALUES ($1, $2, $3, $4, $5, 0)`, uuid.New(), m.tenantB, match, b.id, m.schoolB)
	duel := uuid.New()
	m.exec(`INSERT INTO competition.duels (duel_id, tenant_id, school_id, challenger_id, opponent_id, subject_id, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, duel, m.tenantB, m.schoolB, b.id, b2.id, m.subject, now.Add(time.Minute))
	m.restrict(b, "abandon_penalty")
	m.exec(`INSERT INTO ref.schools (school_id, tenant_id, status, settings_subset, source_version) VALUES ($1, $2, 'active', '{}', 1)`,
		m.schoolB, m.tenantB)
	_ = a
	tables := []string{"competition.match_participants", "competition.duels", "competition.competition_restrictions", "ref.schools"}
	count := func(tenant, table string) int {
		t.Helper()
		tx, err := m.app.BeginTx(ctx, nil)
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
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM `+table+` WHERE tenant_id = $1`, m.tenantB).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	for _, table := range tables {
		if n := count(m.tenantA.String(), table); n != 0 {
			t.Fatalf("%s: tenant A reads %d rows of B", table, n)
		}
		if n := count("", table); n != 0 {
			t.Fatalf("%s: no scope reads %d rows", table, n)
		}
		if n := count(m.tenantB.String(), table); n != 1 {
			t.Fatalf("%s: tenant B reads %d of its rows", table, n)
		}
	}
	exec := func(tenant, q string, args ...any) error {
		tx, err := m.app.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		_, _ = tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenant)
		if _, err := tx.ExecContext(ctx, q, args...); err != nil {
			return err
		}
		return tx.Commit()
	}
	for _, q := range []string{stmt("DEL", "ETE FROM competition.duels"), stmt("TRUN", "CATE competition.match_participants"),
		stmt("DEL", "ETE FROM competition.matches"), stmt("DEL", "ETE FROM competition.competition_restrictions")} {
		if err := exec(m.tenantB.String(), q); err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := exec(m.tenantA.String(), `INSERT INTO competition.duels (duel_id, tenant_id, school_id, challenger_id, opponent_id, subject_id,
		expires_at) VALUES ($1, $2, $3, $4, $5, $6, now())`, uuid.New(), m.tenantB, m.schoolB, b2.id, b.id, m.subject); err == nil {
		t.Fatal("tenant A wrote a duel of tenant B")
	}
	if err := exec(m.tenantB.String(), `UPDATE competition.duels SET deleted_at = now(), deleted_by = $2, delete_reason = 'test'
		WHERE duel_id = $1`, duel, b.id); err != nil {
		t.Fatalf("soft-delete under RLS: %v", err)
	}
	if n := m.count(`SELECT count(*) FROM competition.duels_live WHERE duel_id = $1`, duel); n != 0 {
		t.Fatal("_live view shows a soft-deleted duel")
	}
}
