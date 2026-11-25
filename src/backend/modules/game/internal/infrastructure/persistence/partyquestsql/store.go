// Package partyquestsql reads and writes the shared progress of the co-op quests (game.party_quest_sessions,
// game.party_quest_contributions — F103 research R4) on the transaction of ctx, and the party quests of the content
// (by code, party_mode ≠ solo). Explicit SQL: tenant tables carry their tenant_id and deleted_at conditions (AGENTS R27,
// RLS as well).
package partyquestsql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/partyquest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/questsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// Store implements write.PartyQuestStore (write transaction only).
type Store struct{}

// Reads implements port.PartyQuestReads (read session or write transaction).
type Reads struct{}

var (
	_ write.PartyQuestStore = Store{}
	_ port.PartyQuestReads  = Reads{}
)

type querier interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
	ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error)
}

func tx(ctx context.Context) (querier, error) {
	t, ok := entx.Tx(ctx)
	if !ok || !entx.InWriteTx(ctx) {
		return nil, entx.ErrNoTransaction
	}
	return t, nil
}

func rq(ctx context.Context) (querier, error) {
	t, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	return t, nil
}

func actor(ctx context.Context) any {
	if p, ok := authz.PrincipalFrom(ctx); ok {
		return p.UserID
	}
	return nil
}

const sessionCols = `party_quest_session_id, tenant_id, school_id, party_id, party_activity_id, leader_id, quest_id, quest_version_id, party_mode,
	shared_progress_policy, party_size_min, status, member_states, shared_step_states, role_assignments, shared_clues, state_version, started_at,
	ended_at, end_reason`

func scanSession(row interface{ Scan(...any) error }) (*partyquest.Session, error) {
	var s partyquest.Session
	var members, steps, roles, clues []byte
	var ended sql.NullTime
	var reason sql.NullString
	if err := row.Scan(&s.ID, &s.TenantID, &s.SchoolID, &s.PartyID, &s.PartyActivityID, &s.LeaderID, &s.QuestID, &s.VersionID, &s.PartyMode,
		&s.Policy, &s.SizeMin, &s.Status, &members, &steps, &roles, &clues, &s.StateVersion, &s.StartedAt, &ended, &reason); err != nil {
		return nil, err
	}
	s.StartedAt = s.StartedAt.UTC()
	if ended.Valid {
		e := ended.Time.UTC()
		s.EndedAt = &e
	}
	s.EndReason = reason.String
	s.Steps, s.Roles = map[string]partyquest.SharedStep{}, map[string]partyquest.Roles{}
	for _, f := range []struct {
		raw []byte
		v   any
	}{{members, &s.Members}, {steps, &s.Steps}, {roles, &s.Roles}, {clues, &s.Clues}} {
		if err := json.Unmarshal(f.raw, f.v); err != nil {
			return nil, fmt.Errorf("party quest session: %w", err)
		}
	}
	return &s, nil
}

func one(ctx context.Context, t querier, query string, args ...any) (*partyquest.Session, error) {
	s, err := scanSession(t.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("party quest session: %w", err)
	}
	return s, nil
}

func memberIDs(s *partyquest.Session) []string {
	out := []string{}
	for _, id := range s.Present() {
		out = append(out, id.String())
	}
	return out
}

func docs(s *partyquest.Session) ([4]string, error) {
	var out [4]string
	members := s.Members
	if members == nil {
		members = []partyquest.Member{}
	}
	clues := s.Clues
	if clues == nil {
		clues = []partyquest.Clue{}
	}
	ids := memberIDs(s)
	for i, v := range []any{ids, members, s.Steps, s.Roles} {
		b, err := json.Marshal(v)
		if err != nil {
			return out, err
		}
		out[i] = string(b)
	}
	b, err := json.Marshal(clues)
	if err != nil {
		return out, err
	}
	// out[0] member ids, [1] member states, [2] shared steps, [3] roles; clues travel separately.
	_ = b
	return out, nil
}

func cluesDoc(s *partyquest.Session) (string, error) {
	clues := s.Clues
	if clues == nil {
		clues = []partyquest.Clue{}
	}
	b, err := json.Marshal(clues)
	return string(b), err
}

// Insert implements write.PartyQuestStore.
func (Store) Insert(ctx context.Context, s *partyquest.Session) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	d, err := docs(s)
	if err != nil {
		return false, err
	}
	clues, err := cluesDoc(s)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok tenant table game.party_quest_sessions: explicit tenant_id (RLS insert policy); one per party_activity_id
	res, err := t.ExecContext(ctx, `INSERT INTO game.party_quest_sessions (party_quest_session_id, tenant_id, created_at, created_by, updated_at,
			school_id, party_id, party_activity_id, leader_id, quest_id, quest_version_id, party_mode, shared_progress_policy, party_size_min, status,
			member_ids, member_states, shared_step_states, role_assignments, shared_clues, state_version, started_at)
		VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15::jsonb, $16::jsonb, $17::jsonb, $18::jsonb, $19::jsonb, $20, $3)
		ON CONFLICT DO NOTHING`, s.ID, s.TenantID, s.StartedAt, actor(ctx), s.SchoolID, s.PartyID, s.PartyActivityID, s.LeaderID, s.QuestID,
		s.VersionID, s.PartyMode, s.Policy, s.SizeMin, s.Status, d[0], d[1], d[2], d[3], clues, s.StateVersion)
	if err != nil {
		return false, fmt.Errorf("party quest session: insert: %w", err)
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// Lock implements write.PartyQuestStore.
func (Store) Lock(ctx context.Context, tenant, id uuid.UUID) (*partyquest.Session, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant table game.party_quest_sessions: explicit tenant_id + deleted_at IS NULL (RLS as well)
	return one(ctx, t, `SELECT `+sessionCols+` FROM game.party_quest_sessions WHERE tenant_id = $1 AND party_quest_session_id = $2
		AND deleted_at IS NULL FOR UPDATE`, tenant, id)
}

// ByPartyActivity implements write.PartyQuestStore.
func (Store) ByPartyActivity(ctx context.Context, tenant, partyActivity uuid.UUID) (*partyquest.Session, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant table game.party_quest_sessions: explicit tenant_id + deleted_at IS NULL (RLS as well)
	return one(ctx, t, `SELECT `+sessionCols+` FROM game.party_quest_sessions WHERE tenant_id = $1 AND party_activity_id = $2
		AND deleted_at IS NULL FOR UPDATE`, tenant, partyActivity)
}

func activeFor(ctx context.Context, t querier, tenant, party, questID, student uuid.UUID, lock bool) (*partyquest.Session, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	// tenancyguard:ok tenant table game.party_quest_sessions: explicit tenant_id + deleted_at IS NULL (RLS as well); a present member
	return one(ctx, t, `SELECT `+sessionCols+` FROM game.party_quest_sessions WHERE tenant_id = $1 AND party_id = $2 AND quest_id = $3
		AND status = 'active' AND member_ids ? $4 AND deleted_at IS NULL ORDER BY started_at DESC LIMIT 1`+suffix, tenant, party, questID,
		student.String())
}

// ActiveFor implements write.PartyQuestStore.
func (Store) ActiveFor(ctx context.Context, tenant, party, questID, student uuid.UUID) (*partyquest.Session, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	return activeFor(ctx, t, tenant, party, questID, student, true)
}

// ActiveOfParty implements write.PartyQuestStore.
func (Store) ActiveOfParty(ctx context.Context, tenant, party uuid.UUID) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant table game.party_quest_sessions: explicit tenant_id + deleted_at IS NULL (RLS as well)
	rows, err := t.QueryContext(ctx, `SELECT party_quest_session_id FROM game.party_quest_sessions WHERE tenant_id = $1 AND party_id = $2
		AND status = 'active' AND deleted_at IS NULL ORDER BY party_quest_session_id`, tenant, party)
	if err != nil {
		return nil, fmt.Errorf("party quest sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// Save implements write.PartyQuestStore.
func (Store) Save(ctx context.Context, s *partyquest.Session) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	d, err := docs(s)
	if err != nil {
		return err
	}
	clues, err := cluesDoc(s)
	if err != nil {
		return err
	}
	var ended any
	if s.EndedAt != nil {
		ended = *s.EndedAt
	}
	var reason any
	if s.EndReason != "" {
		reason = s.EndReason
	}
	// tenancyguard:ok tenant table game.party_quest_sessions: explicit tenant_id + deleted_at IS NULL (RLS update policy)
	if _, err := t.ExecContext(ctx, `UPDATE game.party_quest_sessions SET leader_id = $3, status = $4, member_ids = $5::jsonb,
			member_states = $6::jsonb, shared_step_states = $7::jsonb, role_assignments = $8::jsonb, shared_clues = $9::jsonb, state_version = $10,
			ended_at = $11, end_reason = $12, updated_at = now(), updated_by = $13, version = version + 1
		WHERE tenant_id = $1 AND party_quest_session_id = $2 AND deleted_at IS NULL`, s.TenantID, s.ID, s.LeaderID, s.Status, d[0], d[1], d[2],
		d[3], clues, s.StateVersion, ended, reason, actor(ctx)); err != nil {
		return fmt.Errorf("party quest session: save: %w", err)
	}
	return nil
}

func totals(ctx context.Context, t querier, tenant, session uuid.UUID, stepID string) (map[uuid.UUID]int, error) {
	// tenancyguard:ok tenant table game.party_quest_contributions: explicit tenant_id + deleted_at IS NULL (RLS as well)
	rows, err := t.QueryContext(ctx, `SELECT student_id, metric_value FROM game.party_quest_contributions WHERE tenant_id = $1
		AND party_quest_session_id = $2 AND step_id = $3 AND deleted_at IS NULL`, tenant, session, stepID)
	if err != nil {
		return nil, fmt.Errorf("party quest contributions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[uuid.UUID]int{}
	for rows.Next() {
		var id uuid.UUID
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// AddContribution implements write.PartyQuestStore (once per attempt: attempt_ids of the row).
func (Store) AddContribution(ctx context.Context, tenant, session, student uuid.UUID, stepID string, attempt uuid.UUID, metric int,
	minContribution int) (map[uuid.UUID]int, bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, false, err
	}
	var current int
	var raw []byte
	// tenancyguard:ok tenant table game.party_quest_contributions: explicit tenant_id + deleted_at IS NULL (RLS as well)
	err = t.QueryRowContext(ctx, `SELECT metric_value, attempt_ids FROM game.party_quest_contributions WHERE tenant_id = $1
		AND party_quest_session_id = $2 AND student_id = $3 AND step_id = $4 AND deleted_at IS NULL FOR UPDATE`, tenant, session, student, stepID).
		Scan(&current, &raw)
	attempts := []string{}
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return nil, false, fmt.Errorf("party quest contribution: %w", err)
	default:
		if err := json.Unmarshal(raw, &attempts); err != nil {
			return nil, false, err
		}
	}
	if slices.Contains(attempts, attempt.String()) {
		out, err := totals(ctx, t, tenant, session, stepID)
		return out, false, err
	}
	attempts = append(attempts, attempt.String())
	if len(attempts) > 200 {
		attempts = attempts[len(attempts)-200:]
	}
	b, err := json.Marshal(attempts)
	if err != nil {
		return nil, false, err
	}
	next := current + metric
	// tenancyguard:ok tenant table game.party_quest_contributions: explicit tenant_id (RLS insert/update policies); one per (session, student, step)
	if _, err := t.ExecContext(ctx, `INSERT INTO game.party_quest_contributions (party_quest_contribution_id, tenant_id, created_at, created_by,
			updated_at, party_quest_session_id, student_id, step_id, metric_value, min_contribution_met, attempt_ids)
		VALUES ($1, $2, now(), $3, now(), $4, $5, $6, $7, $8, $9::jsonb)
		ON CONFLICT (party_quest_session_id, student_id, step_id) WHERE deleted_at IS NULL DO UPDATE SET metric_value = EXCLUDED.metric_value,
			min_contribution_met = EXCLUDED.min_contribution_met, attempt_ids = EXCLUDED.attempt_ids, updated_at = now(),
			version = game.party_quest_contributions.version + 1`, uuid.New(), tenant, actor(ctx), session, student, stepID, next,
		next >= minContribution, string(b)); err != nil {
		return nil, false, fmt.Errorf("party quest contribution: save: %w", err)
	}
	out, err := totals(ctx, t, tenant, session, stepID)
	return out, true, err
}

// Contributions implements write.PartyQuestStore.
func (Store) Contributions(ctx context.Context, tenant, session uuid.UUID, stepID string) (map[uuid.UUID]int, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	return totals(ctx, t, tenant, session, stepID)
}

func progress(ctx context.Context, t querier, query string, args ...any) (*quest.Progress, error) {
	rows, err := t.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("quest progress: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, rows.Err()
	}
	return questsql.ScanProgress(rows)
}

// ProgressRef implements write.PartyQuestStore (no lock).
func (Store) ProgressRef(ctx context.Context, tenant, progressID uuid.UUID) (*quest.Progress, error) {
	t, err := rq(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant table game.quest_progress: explicit tenant_id + deleted_at IS NULL (RLS as well)
	return progress(ctx, t, `SELECT `+questsql.ProgressColumns+` FROM game.quest_progress WHERE tenant_id = $1 AND quest_progress_id = $2
		AND deleted_at IS NULL`, tenant, progressID)
}

func activeProgress(ctx context.Context, t querier, tenant, student, questID uuid.UUID) (*quest.Progress, error) {
	// tenancyguard:ok tenant table game.quest_progress: explicit tenant_id + deleted_at IS NULL (RLS as well)
	return progress(ctx, t, `SELECT `+questsql.ProgressColumns+` FROM game.quest_progress WHERE tenant_id = $1 AND student_id = $2
		AND quest_id = $3 AND status = 'active' AND deleted_at IS NULL`, tenant, student, questID)
}

// ProgressRefActive implements write.PartyQuestStore (no lock).
func (Store) ProgressRefActive(ctx context.Context, tenant, student, questID uuid.UUID) (*quest.Progress, error) {
	t, err := rq(ctx)
	if err != nil {
		return nil, err
	}
	return activeProgress(ctx, t, tenant, student, questID)
}

// AttemptRef implements write.PartyQuestStore (no lock).
func (Store) AttemptRef(ctx context.Context, tenant, attemptID uuid.UUID) (*write.StepAttempt, error) {
	t, err := rq(ctx)
	if err != nil {
		return nil, err
	}
	var a write.StepAttempt
	// tenancyguard:ok tenant table game.quest_step_attempts: explicit tenant_id + deleted_at IS NULL (RLS as well)
	err = t.QueryRowContext(ctx, `SELECT quest_step_attempt_id, tenant_id, quest_progress_id, step_id, student_id, attempt_id, started_on, status
		FROM game.quest_step_attempts WHERE tenant_id = $1 AND attempt_id = $2 AND deleted_at IS NULL`, tenant, attemptID).
		Scan(&a.ID, &a.TenantID, &a.ProgressID, &a.StepID, &a.StudentID, &a.AttemptID, &a.StartedOn, &a.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("quest step attempt: %w", err)
	}
	return &a, nil
}

// ActiveProgress implements port.PartyQuestReads.
func (Reads) ActiveProgress(ctx context.Context, tenant, student, questID uuid.UUID) (*quest.Progress, error) {
	t, err := rq(ctx)
	if err != nil {
		return nil, err
	}
	return activeProgress(ctx, t, tenant, student, questID)
}

// ActiveSession implements port.PartyQuestReads.
func (Reads) ActiveSession(ctx context.Context, tenant, party, questID, student uuid.UUID) (*partyquest.Session, error) {
	t, err := rq(ctx)
	if err != nil {
		return nil, err
	}
	return activeFor(ctx, t, tenant, party, questID, student, false)
}
