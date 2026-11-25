package bosssql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/boss"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// Store implements write.BossStore on game.boss_* (write transaction only).
type Store struct{}

// Reads implements port.BossReads (read session or write transaction).
type Reads struct{}

var (
	_ write.BossStore = Store{}
	_ port.BossReads  = Reads{}
)

func tx(ctx context.Context) (*sql.Tx, error) {
	t, ok := entx.Tx(ctx)
	if !ok || !entx.InWriteTx(ctx) {
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

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC()
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

const encounterCols = `encounter_id, tenant_id, school_id, boss_id, boss_version_id, mode, party_id, party_activity_id, leader_id, status,
	phase_index, current_phase, phase_retries, plan_failures, phases_cleared, energy_remaining, state_version, ready_deadline_at, deadline_at,
	phase_deadline_at, outcome, end_reason, period_key, started_at, ended_at, created_at`

type scanner interface{ Scan(...any) error }

func scanEncounter(row scanner) (*boss.Encounter, error) {
	var e boss.Encounter
	var party, pa, leader uuid.NullUUID
	var phase, outcome, reason sql.NullString
	var deadline, phaseDeadline, started, ended sql.NullTime
	if err := row.Scan(&e.ID, &e.TenantID, &e.SchoolID, &e.BossID, &e.VersionID, &e.Mode, &party, &pa, &leader, &e.Status, &e.PhaseIndex,
		&phase, &e.Retries, &e.PlanFailures, &e.PhasesCleared, &e.Energy, &e.StateVersion, &e.ReadyDeadline, &deadline, &phaseDeadline,
		&outcome, &reason, &e.PeriodKey, &started, &ended, &e.CreatedAt); err != nil {
		return nil, err
	}
	opt := func(n uuid.NullUUID) *uuid.UUID {
		if !n.Valid {
			return nil
		}
		v := n.UUID
		return &v
	}
	tm := func(n sql.NullTime) *time.Time {
		if !n.Valid {
			return nil
		}
		v := n.Time.UTC()
		return &v
	}
	e.PartyID, e.PartyActivityID, e.LeaderID = opt(party), opt(pa), opt(leader)
	e.PhaseID, e.Outcome, e.EndReason = phase.String, outcome.String, reason.String
	e.ReadyDeadline, e.CreatedAt = e.ReadyDeadline.UTC(), e.CreatedAt.UTC()
	e.Deadline, e.PhaseDeadline, e.StartedAt, e.EndedAt = tm(deadline), tm(phaseDeadline), tm(started), tm(ended)
	return &e, nil
}

// load reads an encounter with its members and contributions (lock: FOR UPDATE of the encounter row).
func load(ctx context.Context, t Querier, tenant, id uuid.UUID, lock bool) (*boss.Encounter, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	// tenancyguard:ok tenant table game.boss_encounters: explicit tenant_id + deleted_at IS NULL (RLS as well)
	e, err := scanEncounter(t.QueryRowContext(ctx, `SELECT `+encounterCols+` FROM game.boss_encounters
		WHERE tenant_id = $1 AND encounter_id = $2 AND deleted_at IS NULL`+suffix, tenant, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("boss encounter: %w", err)
	}
	// tenancyguard:ok tenant table game.boss_encounter_members: explicit tenant_id + deleted_at IS NULL (RLS as well)
	rows, err := t.QueryContext(ctx, `SELECT student_id, school_id, grade_level, join_order, ready, joined_at, disconnected_at, left_at,
			left_reason, eligible, first_defeat
		FROM game.boss_encounter_members WHERE tenant_id = $1 AND encounter_id = $2 AND deleted_at IS NULL ORDER BY join_order`, tenant, id)
	if err != nil {
		return nil, fmt.Errorf("boss encounter members: %w", err)
	}
	for rows.Next() {
		var m boss.Member
		var disc, left sql.NullTime
		var reason sql.NullString
		var el sql.NullBool
		if err := rows.Scan(&m.StudentID, &m.SchoolID, &m.Grade, &m.JoinOrder, &m.Ready, &m.JoinedAt, &disc, &left, &reason, &el,
			&m.FirstDefeat); err != nil {
			_ = rows.Close()
			return nil, err
		}
		m.JoinedAt = m.JoinedAt.UTC()
		if disc.Valid {
			d := disc.Time.UTC()
			m.DisconnectedAt = &d
		}
		if left.Valid {
			l := left.Time.UTC()
			m.LeftAt = &l
		}
		m.LeftReason = reason.String
		if el.Valid {
			b := el.Bool
			m.Eligible = &b
		}
		e.Members = append(e.Members, m)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant table game.boss_encounter_contributions: explicit tenant_id + deleted_at IS NULL (RLS as well)
	rows, err = t.QueryContext(ctx, `SELECT student_id, phase_id, correct_count, activities_completed, contribution_pct, eligible, cleared
		FROM game.boss_encounter_contributions WHERE tenant_id = $1 AND encounter_id = $2 AND deleted_at IS NULL`, tenant, id)
	if err != nil {
		return nil, fmt.Errorf("boss encounter contributions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var c boss.Contribution
		if err := rows.Scan(&c.StudentID, &c.PhaseID, &c.Correct, &c.Activities, &c.Pct, &c.Eligible, &c.Cleared); err != nil {
			return nil, err
		}
		e.Contributions = append(e.Contributions, c)
	}
	return e, rows.Err()
}

// InsertEncounter implements write.BossStore.
func (Store) InsertEncounter(ctx context.Context, e *boss.Encounter) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok tenant table game.boss_encounters: explicit tenant_id (RLS insert policy); one per party_activity_id
	res, err := t.ExecContext(ctx, `INSERT INTO game.boss_encounters (encounter_id, tenant_id, created_at, created_by, updated_at, school_id,
			boss_id, boss_version_id, mode, party_id, party_activity_id, leader_id, status, energy_remaining, state_version, ready_deadline_at,
			period_key)
		VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16) ON CONFLICT DO NOTHING`, e.ID, e.TenantID,
		e.CreatedAt, actor(ctx), e.SchoolID, e.BossID, e.VersionID, e.Mode, e.PartyID, e.PartyActivityID, e.LeaderID, e.Status, e.Energy,
		e.StateVersion, e.ReadyDeadline, e.PeriodKey)
	if err != nil {
		return false, fmt.Errorf("boss encounter: insert: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		return false, err
	}
	for _, m := range e.Members {
		// tenancyguard:ok tenant table game.boss_encounter_members: explicit tenant_id (RLS insert policy)
		if _, err := t.ExecContext(ctx, `INSERT INTO game.boss_encounter_members (boss_encounter_member_id, tenant_id, created_at, created_by,
				updated_at, encounter_id, student_id, school_id, grade_level, join_order, ready, joined_at)
			VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8, $9, $10, $3)`, uuid.New(), e.TenantID, e.CreatedAt, actor(ctx), e.ID, m.StudentID,
			m.SchoolID, m.Grade, m.JoinOrder, m.Ready); err != nil {
			return false, fmt.Errorf("boss encounter member: insert: %w", err)
		}
	}
	return true, nil
}

// Lock implements write.BossStore.
func (Store) Lock(ctx context.Context, tenant, id uuid.UUID) (*boss.Encounter, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	return load(ctx, t, tenant, id, true)
}

// ByPartyActivity implements write.BossStore.
func (Store) ByPartyActivity(ctx context.Context, tenant, partyActivity uuid.UUID) (*boss.Encounter, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var id uuid.UUID
	// tenancyguard:ok tenant table game.boss_encounters: explicit tenant_id + deleted_at IS NULL (RLS as well)
	err = t.QueryRowContext(ctx, `SELECT encounter_id FROM game.boss_encounters WHERE tenant_id = $1 AND party_activity_id = $2
		AND deleted_at IS NULL`, tenant, partyActivity).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("boss encounter by party activity: %w", err)
	}
	return load(ctx, t, tenant, id, true)
}

// Save implements write.BossStore.
func (Store) Save(ctx context.Context, e *boss.Encounter) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok tenant table game.boss_encounters: explicit tenant_id + deleted_at IS NULL (RLS update policy)
	if _, err := t.ExecContext(ctx, `UPDATE game.boss_encounters SET status = $3, phase_index = $4, current_phase = $5, phase_retries = $6,
			plan_failures = $7, phases_cleared = $8, energy_remaining = $9, state_version = $10, deadline_at = $11, phase_deadline_at = $12,
			outcome = $13, end_reason = $14, started_at = $15, ended_at = $16, updated_at = now(), updated_by = $17, version = version + 1
		WHERE tenant_id = $1 AND encounter_id = $2 AND deleted_at IS NULL`, e.TenantID, e.ID, e.Status, e.PhaseIndex, nullStr(e.PhaseID),
		e.Retries, e.PlanFailures, e.PhasesCleared, e.Energy, e.StateVersion, nullTime(e.Deadline), nullTime(e.PhaseDeadline),
		nullStr(e.Outcome), nullStr(e.EndReason), nullTime(e.StartedAt), nullTime(e.EndedAt), actor(ctx)); err != nil {
		return fmt.Errorf("boss encounter: save: %w", err)
	}
	for _, m := range e.Members {
		var el any
		if m.Eligible != nil {
			el = *m.Eligible
		}
		// tenancyguard:ok tenant table game.boss_encounter_members: explicit tenant_id + deleted_at IS NULL (RLS update policy)
		if _, err := t.ExecContext(ctx, `UPDATE game.boss_encounter_members SET ready = $4, disconnected_at = $5, left_at = $6,
				left_reason = $7, eligible = $8, first_defeat = $9, updated_at = now(), version = version + 1
			WHERE tenant_id = $1 AND encounter_id = $2 AND student_id = $3 AND deleted_at IS NULL`, e.TenantID, e.ID, m.StudentID, m.Ready,
			nullTime(m.DisconnectedAt), nullTime(m.LeftAt), nullStr(m.LeftReason), el, m.FirstDefeat); err != nil {
			return fmt.Errorf("boss encounter member: save: %w", err)
		}
	}
	for _, c := range e.Contributions {
		// tenancyguard:ok tenant table game.boss_encounter_contributions: explicit tenant_id (RLS insert/update policies); one per phase
		if _, err := t.ExecContext(ctx, `INSERT INTO game.boss_encounter_contributions (contribution_id, tenant_id, created_at, updated_at,
				encounter_id, student_id, phase_id, correct_count, activities_completed, contribution_pct, eligible, cleared)
			VALUES ($1, $2, now(), now(), $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (encounter_id, student_id, phase_id) WHERE deleted_at IS NULL DO UPDATE SET correct_count = EXCLUDED.correct_count,
				activities_completed = EXCLUDED.activities_completed, contribution_pct = EXCLUDED.contribution_pct, eligible = EXCLUDED.eligible,
				cleared = EXCLUDED.cleared, updated_at = now(), version = game.boss_encounter_contributions.version + 1`, uuid.New(), e.TenantID, e.ID, c.StudentID,
			c.PhaseID, c.Correct, c.Activities, c.Pct, c.Eligible, c.Cleared); err != nil {
			return fmt.Errorf("boss encounter contribution: save: %w", err)
		}
	}
	return nil
}

func ids(ctx context.Context, t Querier, query string, args ...any) ([]uuid.UUID, error) {
	rows, err := t.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
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

// LiveOfStudent implements write.BossStore.
func (Store) LiveOfStudent(ctx context.Context, tenant, student uuid.UUID) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant tables game.boss_encounters/members: explicit tenant_id + deleted_at IS NULL (RLS as well)
	return ids(ctx, t, `SELECT e.encounter_id FROM game.boss_encounters e JOIN game.boss_encounter_members m ON m.encounter_id = e.encounter_id
			AND m.tenant_id = e.tenant_id AND m.deleted_at IS NULL AND m.left_at IS NULL
		WHERE e.tenant_id = $1 AND m.student_id = $2 AND e.status IN ('lobby','phase_active') AND e.deleted_at IS NULL ORDER BY e.encounter_id`,
		tenant, student)
}

// LiveOfParty implements write.BossStore.
func (Store) LiveOfParty(ctx context.Context, tenant, party uuid.UUID) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant table game.boss_encounters: explicit tenant_id + deleted_at IS NULL (RLS as well)
	return ids(ctx, t, `SELECT encounter_id FROM game.boss_encounters WHERE tenant_id = $1 AND party_id = $2
		AND status IN ('lobby','phase_active') AND deleted_at IS NULL ORDER BY encounter_id`, tenant, party)
}

const planCols = `boss_phase_plan_id, tenant_id, encounter_id, phase_id, retry_no, student_id, requirement_id, plan_id, items, expires_at, created_at`

func scanPlans(rows *sql.Rows, err error) ([]port.BossPlanRow, error) {
	if err != nil {
		return nil, fmt.Errorf("boss plans: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.BossPlanRow
	for rows.Next() {
		var p port.BossPlanRow
		var items []byte
		if err := rows.Scan(&p.ID, &p.TenantID, &p.EncounterID, &p.PhaseID, &p.RetryNo, &p.StudentID, &p.RequirementID, &p.PlanID, &items,
			&p.ExpiresAt, &p.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(items, &p.Items); err != nil {
			return nil, fmt.Errorf("boss plan items: %w", err)
		}
		p.ExpiresAt, p.CreatedAt = p.ExpiresAt.UTC(), p.CreatedAt.UTC()
		out = append(out, p)
	}
	return out, rows.Err()
}

func plans(ctx context.Context, t Querier, tenant, encounter uuid.UUID, phaseID string, retry int) ([]port.BossPlanRow, error) {
	// tenancyguard:ok tenant table game.boss_phase_plans: explicit tenant_id + deleted_at IS NULL (RLS as well)
	return scanPlans(t.QueryContext(ctx, `SELECT `+planCols+` FROM game.boss_phase_plans WHERE tenant_id = $1 AND encounter_id = $2
		AND phase_id = $3 AND retry_no = $4 AND deleted_at IS NULL ORDER BY student_id, requirement_id`, tenant, encounter, phaseID, retry))
}

// Plans implements write.BossStore.
func (Store) Plans(ctx context.Context, tenant, encounter uuid.UUID, phaseID string, retry int) ([]port.BossPlanRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	return plans(ctx, t, tenant, encounter, phaseID, retry)
}

// PlanByID implements write.BossStore.
func (Store) PlanByID(ctx context.Context, tenant, planID uuid.UUID) (*port.BossPlanRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant table game.boss_phase_plans: explicit tenant_id + deleted_at IS NULL (RLS as well)
	ps, err := scanPlans(t.QueryContext(ctx, `SELECT `+planCols+` FROM game.boss_phase_plans WHERE tenant_id = $1 AND plan_id = $2
		AND deleted_at IS NULL`, tenant, planID))
	if err != nil || len(ps) == 0 {
		return nil, err
	}
	return &ps[0], nil
}

// InsertPlan implements write.BossStore.
func (Store) InsertPlan(ctx context.Context, p port.BossPlanRow) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	items, err := json.Marshal(p.Items)
	if err != nil {
		return err
	}
	// tenancyguard:ok tenant table game.boss_phase_plans: explicit tenant_id (RLS insert policy); one per round, member, requirement
	if _, err := t.ExecContext(ctx, `INSERT INTO game.boss_phase_plans (boss_phase_plan_id, tenant_id, created_at, created_by, updated_at,
			encounter_id, phase_id, retry_no, student_id, requirement_id, plan_id, item_count, items, expires_at)
		VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8, $9, $10, $11, $12::jsonb, $13) ON CONFLICT DO NOTHING`, p.ID, p.TenantID, p.CreatedAt,
		actor(ctx), p.EncounterID, p.PhaseID, p.RetryNo, p.StudentID, p.RequirementID, p.PlanID, len(p.Items), string(items), p.ExpiresAt); err != nil {
		return fmt.Errorf("boss plan: insert: %w", err)
	}
	return nil
}

// InsertAnswer implements write.BossStore.
func (Store) InsertAnswer(ctx context.Context, a port.BossAnswerRow) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok tenant append-only table game.boss_answer_results: explicit tenant_id (RLS insert policy); once per (plan, index, student)
	res, err := t.ExecContext(ctx, `INSERT INTO game.boss_answer_results (answer_result_id, tenant_id, created_at, created_by, encounter_id,
			student_id, plan_id, item_index, phase_id, retry_no, requirement_id, is_correct, counted, rapid_guess, received_at)
		VALUES ($1, $2, now(), $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14) ON CONFLICT DO NOTHING`, a.ID, a.TenantID, actor(ctx),
		a.EncounterID, a.StudentID, a.PlanID, a.Index, a.PhaseID, a.RetryNo, a.RequirementID, a.IsCorrect, a.Counted, a.RapidGuess, a.ReceivedAt)
	if err != nil {
		return false, fmt.Errorf("boss answer: insert: %w", err)
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// Answer implements write.BossStore.
func (Store) Answer(ctx context.Context, tenant, planID uuid.UUID, index int, student uuid.UUID) (*port.BossAnswerRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var a port.BossAnswerRow
	// tenancyguard:ok tenant append-only table game.boss_answer_results: explicit tenant_id (RLS as well; no soft delete on TA)
	err = t.QueryRowContext(ctx, `SELECT answer_result_id, tenant_id, encounter_id, student_id, plan_id, item_index, phase_id, retry_no,
			requirement_id, is_correct, counted, rapid_guess, received_at
		FROM game.boss_answer_results WHERE tenant_id = $1 AND plan_id = $2 AND item_index = $3 AND student_id = $4`, tenant, planID, index,
		student).Scan(&a.ID, &a.TenantID, &a.EncounterID, &a.StudentID, &a.PlanID, &a.Index, &a.PhaseID, &a.RetryNo, &a.RequirementID,
		&a.IsCorrect, &a.Counted, &a.RapidGuess, &a.ReceivedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("boss answer: %w", err)
	}
	return &a, nil
}

func round(ctx context.Context, t Querier, tenant, encounter uuid.UUID, phaseID string, retry int) (map[uuid.UUID]map[string]int, error) {
	// tenancyguard:ok tenant append-only table game.boss_answer_results: explicit tenant_id (RLS as well; no soft delete on TA)
	rows, err := t.QueryContext(ctx, `SELECT student_id, requirement_id, count(*) FROM game.boss_answer_results
		WHERE tenant_id = $1 AND encounter_id = $2 AND phase_id = $3 AND retry_no = $4 AND counted GROUP BY student_id, requirement_id`,
		tenant, encounter, phaseID, retry)
	if err != nil {
		return nil, fmt.Errorf("boss round: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[uuid.UUID]map[string]int{}
	for rows.Next() {
		var s uuid.UUID
		var req string
		var n int
		if err := rows.Scan(&s, &req, &n); err != nil {
			return nil, err
		}
		if out[s] == nil {
			out[s] = map[string]int{}
		}
		out[s][req] = n
	}
	return out, rows.Err()
}

// Round implements write.BossStore.
func (Store) Round(ctx context.Context, tenant, encounter uuid.UUID, phaseID string, retry int) (map[uuid.UUID]map[string]int, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	return round(ctx, t, tenant, encounter, phaseID, retry)
}

// usage counts the encounters of a student on a boss in a period ("" = never) — except those abandoned by a learning
// failure, which never count — and whether it was ever defeated with the student eligible.
func usage(ctx context.Context, t Querier, tenant, student, bossID uuid.UUID, period string) (port.BossUsage, error) {
	var u port.BossUsage
	// tenancyguard:ok tenant tables game.boss_encounters/members: explicit tenant_id + deleted_at IS NULL (RLS as well)
	err := t.QueryRowContext(ctx, `SELECT
			count(*) FILTER (WHERE $4::text <> '' AND e.period_key = $4 AND COALESCE(e.end_reason, '') <> 'plan_failure'),
			COALESCE(bool_or(e.status = 'defeated' AND m.eligible), false)
		FROM game.boss_encounters e JOIN game.boss_encounter_members m ON m.encounter_id = e.encounter_id AND m.tenant_id = e.tenant_id
			AND m.deleted_at IS NULL
		WHERE e.tenant_id = $1 AND m.student_id = $2 AND e.boss_id = $3 AND e.deleted_at IS NULL`, tenant, student, bossID, period).
		Scan(&u.Encounters, &u.Defeated)
	if err != nil {
		return u, fmt.Errorf("boss usage: %w", err)
	}
	return u, nil
}

// Usage implements write.BossStore.
func (Store) Usage(ctx context.Context, tenant, student, bossID uuid.UUID, period string) (port.BossUsage, error) {
	t, err := tx(ctx)
	if err != nil {
		return port.BossUsage{}, err
	}
	return usage(ctx, t, tenant, student, bossID, period)
}

// Encounter implements port.BossReads.
func (Reads) Encounter(ctx context.Context, tenant, id uuid.UUID) (*boss.Encounter, error) {
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	return load(ctx, t, tenant, id, false)
}

// Plans implements port.BossReads.
func (Reads) Plans(ctx context.Context, tenant, encounter uuid.UUID, phaseID string, retry int) ([]port.BossPlanRow, error) {
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	return plans(ctx, t, tenant, encounter, phaseID, retry)
}

// Round implements port.BossReads.
func (Reads) Round(ctx context.Context, tenant, encounter uuid.UUID, phaseID string, retry int) (map[uuid.UUID]map[string]int, error) {
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	return round(ctx, t, tenant, encounter, phaseID, retry)
}

// Answered implements port.BossReads.
func (Reads) Answered(ctx context.Context, tenant, encounter, student uuid.UUID) (map[uuid.UUID]map[int]bool, error) {
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant append-only table game.boss_answer_results: explicit tenant_id (RLS as well; no soft delete on TA)
	rows, err := t.QueryContext(ctx, `SELECT plan_id, item_index FROM game.boss_answer_results WHERE tenant_id = $1 AND encounter_id = $2
		AND student_id = $3`, tenant, encounter, student)
	if err != nil {
		return nil, fmt.Errorf("boss answered: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[uuid.UUID]map[int]bool{}
	for rows.Next() {
		var p uuid.UUID
		var i int
		if err := rows.Scan(&p, &i); err != nil {
			return nil, err
		}
		if out[p] == nil {
			out[p] = map[int]bool{}
		}
		out[p][i] = true
	}
	return out, rows.Err()
}

// Usage implements port.BossReads.
func (Reads) Usage(ctx context.Context, tenant, student, bossID uuid.UUID, period string) (port.BossUsage, error) {
	t, err := q(ctx)
	if err != nil {
		return port.BossUsage{}, err
	}
	return usage(ctx, t, tenant, student, bossID, period)
}

// Nicknames implements port.BossReads and port.NicknameReader (game profiles of the tenant; never a real name, R16).
func (Reads) Nicknames(ctx context.Context, tenant uuid.UUID, students []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	if len(students) == 0 {
		return out, nil
	}
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]string, len(students))
	for i, s := range students {
		list[i] = s.String()
	}
	// tenancyguard:ok tenant table game.game_profiles: explicit tenant_id + deleted_at IS NULL (RLS as well); nicknames only
	rows, err := t.QueryContext(ctx, `SELECT student_id, COALESCE(nickname, '') FROM game.game_profiles WHERE tenant_id = $1
		AND student_id::text = ANY($2) AND deleted_at IS NULL`, tenant, list)
	if err != nil {
		return nil, fmt.Errorf("nicknames: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id uuid.UUID
		var n string
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}
