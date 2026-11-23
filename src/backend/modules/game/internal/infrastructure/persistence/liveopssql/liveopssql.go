// Package liveopssql is the PostgreSQL side of live-ops (F077, DATABASE_DESIGN §2.8c): runs, goals, participation, the
// event point, contribution and season point ledgers (append-only, idempotent per source event), the progress rows, the
// school events, the season projection and the season track progress — on the transaction of ctx (use case, consumer,
// job or read session). Explicit SQL: platform rows (tenant_id NULL) and the rows of the tenant are named in every
// condition with deleted_at IS NULL (AGENTS R27); RLS of the tenant tables applies as well.
package liveopssql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/liveops"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/season"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// Store implements write.LiveOpsStore, read.LiveOpsReadModel, port.LiveOps (limited shop listings) and
// port.ManifestEvents.
type Store struct{}

var (
	_          write.LiveOpsStore    = Store{}
	_          read.LiveOpsReadModel = Store{}
	_          port.LiveOps          = Store{}
	_          port.ManifestEvents   = Store{}
	_          port.LiveOpsReader    = Store{}
	runColumns                       = `r.event_run_id, r.event_id, r.event_version_id, v.version_no, r.tenant_id, r.school_id, r.status, r.starts_at, r.ends_at,
		r.claim_until, r.status_version, r.audience, r.suspended_reason, r.cancelled_at, v.payload`
	runJoin = `game.event_runs r JOIN game.event_versions v ON v.event_version_id = r.event_version_id AND v.deleted_at IS NULL`
)

func tx(ctx context.Context) (*sql.Tx, error) {
	t, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	return t, nil
}

type scanner interface{ Scan(dest ...any) error }

func scanRun(s scanner) (port.EventRunRow, error) {
	var r port.EventRunRow
	var tenant, school uuid.NullUUID
	var audience, payload []byte
	var reason sql.NullString
	var cancelled sql.NullTime
	if err := s.Scan(&r.RunID, &r.EventID, &r.VersionID, &r.VersionNo, &tenant, &school, &r.Status, &r.StartsAt, &r.EndsAt, &r.ClaimUntil,
		&r.StatusVersion, &audience, &reason, &cancelled, &payload); err != nil {
		return r, err
	}
	if tenant.Valid {
		t := tenant.UUID
		r.TenantID = &t
	}
	if school.Valid {
		s := school.UUID
		r.SchoolID = &s
	}
	if reason.Valid {
		r.SuspendedReason = &reason.String
	}
	if cancelled.Valid {
		c := cancelled.Time.UTC()
		r.CancelledAt = &c
	}
	r.StartsAt, r.EndsAt, r.ClaimUntil = r.StartsAt.UTC(), r.EndsAt.UTC(), r.ClaimUntil.UTC()
	if err := json.Unmarshal(audience, &r.Audience); err != nil {
		return r, fmt.Errorf("event run audience: %w", err)
	}
	d, err := liveops.Parse(payload)
	if err != nil {
		return r, fmt.Errorf("event version payload: %w", err)
	}
	r.Definition = d
	return r, nil
}

func runRows(rows *sql.Rows, err error) ([]port.EventRunRow, error) {
	if err != nil {
		return nil, fmt.Errorf("event runs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.EventRunRow
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func oneRun(row *sql.Row) (*port.EventRunRow, error) {
	r, err := scanRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func jsonOf(v any) (string, error) {
	b, err := json.Marshal(v)
	return string(b), err
}

func nullUUID(id *uuid.UUID) uuid.NullUUID {
	if id == nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: *id, Valid: true}
}

// ---------- runs ----------

// Run implements port.LiveOpsReader.
func (Store) Run(ctx context.Context, run uuid.UUID) (*port.EventRunRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok G/T runs: platform rows (tenant_id NULL) and the rows of the session tenant (RLS); deleted_at filtered
	return oneRun(t.QueryRowContext(ctx, `SELECT `+runColumns+` FROM `+runJoin+` WHERE r.event_run_id = $1 AND r.deleted_at IS NULL`, run))
}

// EventRun implements port.LiveOpsReader.
func (Store) EventRun(ctx context.Context, tenant, event uuid.UUID) (*port.EventRunRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok G/T runs: platform rows or explicit tenant_id; deleted_at filtered
	return oneRun(t.QueryRowContext(ctx, `SELECT `+runColumns+` FROM `+runJoin+` WHERE r.event_id = $1 AND r.deleted_at IS NULL
		AND r.status <> 'archived' AND (r.tenant_id IS NULL OR r.tenant_id = $2) ORDER BY r.created_at DESC LIMIT 1`, event, tenant))
}

// EarningRuns implements port.LiveOpsReader.
func (Store) EarningRuns(ctx context.Context, tenant uuid.UUID) ([]port.EventRunRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok G/T runs: platform rows or explicit tenant_id; deleted_at filtered
	return runRows(t.QueryContext(ctx, `SELECT `+runColumns+` FROM `+runJoin+` WHERE r.deleted_at IS NULL AND r.status = 'live'
		AND (r.tenant_id = $1 OR (r.tenant_id IS NULL AND (jsonb_array_length(coalesce(r.audience->'tenant_ids', '[]'::jsonb)) = 0
			OR r.audience->'tenant_ids' @> to_jsonb($1::text))))
		ORDER BY r.starts_at, r.event_run_id`, tenant))
}

// ActiveRun implements write.LiveOpsStore.
func (Store) ActiveRun(ctx context.Context, event uuid.UUID) (*port.EventRunRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok G/T runs of one event (the publishing scope owns it); deleted_at filtered
	return oneRun(t.QueryRowContext(ctx, `SELECT `+runColumns+` FROM `+runJoin+` WHERE r.event_id = $1 AND r.deleted_at IS NULL
		AND r.status IN ('scheduled', 'live', 'suspended')
		ORDER BY r.created_at DESC LIMIT 1 FOR UPDATE OF r`, event))
}

// InsertRun implements write.LiveOpsStore.
func (Store) InsertRun(ctx context.Context, r port.EventRunRow, at time.Time) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	a, err := jsonOf(r.Audience)
	if err != nil {
		return err
	}
	_, err = t.ExecContext(ctx, `INSERT INTO game.event_runs (event_run_id, tenant_id, created_at, updated_at, version, event_id, event_version_id,
			status, starts_at, ends_at, claim_until, status_version, audience, school_id)
		VALUES ($1, $2, $3, $3, 1, $4, $5, 'scheduled', $6, $7, $8, 1, $9::jsonb, $10)`, r.RunID, nullUUID(r.TenantID), at, r.EventID, r.VersionID,
		r.StartsAt, r.EndsAt, r.ClaimUntil, a, nullUUID(r.SchoolID))
	if err != nil {
		return fmt.Errorf("insert event run: %w", err)
	}
	return nil
}

// RepinRun implements write.LiveOpsStore.
func (Store) RepinRun(ctx context.Context, run, version uuid.UUID, starts, ends, claim time.Time, a liveops.Audience, at time.Time) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	aj, err := jsonOf(a)
	if err != nil {
		return err
	}
	// tenancyguard:ok G/T run locked by its id in the transaction of its scope (RLS); deleted_at filtered
	_, err = t.ExecContext(ctx, `UPDATE game.event_runs SET event_version_id = $2, starts_at = $3, ends_at = $4, claim_until = $5,
		audience = $6::jsonb, status_version = status_version + 1, updated_at = $7, version = version + 1
		WHERE event_run_id = $1 AND deleted_at IS NULL`, run, version, starts, ends, claim, aj, at)
	if err != nil {
		return fmt.Errorf("repin event run: %w", err)
	}
	return nil
}

// LockRun implements write.LiveOpsStore.
func (Store) LockRun(ctx context.Context, run uuid.UUID) (*port.EventRunRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok G/T run by id (platform rows or the rows of the session tenant, RLS); deleted_at filtered
	return oneRun(t.QueryRowContext(ctx, `SELECT `+runColumns+` FROM `+runJoin+` WHERE r.event_run_id = $1 AND r.deleted_at IS NULL FOR UPDATE OF r`, run))
}

// SetRunStatus implements write.LiveOpsStore.
func (Store) SetRunStatus(ctx context.Context, run uuid.UUID, statusVersion int, to string, reason *string, by *uuid.UUID, at time.Time) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	var r sql.NullString
	if reason != nil {
		r = sql.NullString{String: *reason, Valid: true}
	}
	// tenancyguard:ok G/T run by id and status version (optimistic lock); deleted_at filtered; RLS limits tenant rows
	res, err := t.ExecContext(ctx, `UPDATE game.event_runs SET status = $3::varchar, status_version = status_version + 1,
			suspended_reason = coalesce($4::varchar, suspended_reason), status_changed_by = $5, updated_at = $6, updated_by = $5, version = version + 1,
			cancelled_at = CASE WHEN $3::varchar = 'cancelled' THEN $6::timestamptz ELSE cancelled_at END,
			archived_at = CASE WHEN $3::varchar = 'archived' THEN $6::timestamptz ELSE archived_at END
		WHERE event_run_id = $1 AND status_version = $2 AND deleted_at IS NULL`,
		run, statusVersion, to, r, nullUUID(by), at)
	if err != nil {
		return false, fmt.Errorf("event run status: %w", err)
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// DueRuns implements write.LiveOpsStore (platform scan, game_platform).
func (Store) DueRuns(ctx context.Context, now time.Time, limit int) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform scan of the scheduler (SystemPlatformScope, every tenant by design); deleted_at filtered
	rows, err := t.QueryContext(ctx, `SELECT event_run_id FROM game.event_runs WHERE deleted_at IS NULL
		AND ((status = 'scheduled' AND starts_at <= $1) OR (status = 'live' AND ends_at <= $1) OR (status = 'ended' AND claim_until <= $1)
			OR status = 'cancelled')
		ORDER BY event_run_id LIMIT $2`, now, limit)
	return ids(rows, err)
}

// PendingRepins implements write.LiveOpsStore (platform scan).
func (Store) PendingRepins(ctx context.Context, now time.Time, limit int) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform scan of the scheduler (every tenant by design); deleted_at filtered on every table
	rows, err := t.QueryContext(ctx, `SELECT r.event_run_id FROM game.event_runs r
		JOIN game.events e ON e.event_id = r.event_id AND e.deleted_at IS NULL
		JOIN game.event_versions v ON v.event_version_id = e.current_version_id AND v.deleted_at IS NULL
		WHERE r.deleted_at IS NULL AND r.status IN ('scheduled', 'live', 'suspended')
			AND e.current_version_id <> r.event_version_id AND coalesce(v.effective_from, v.published_at, v.created_at) <= $1
		ORDER BY r.event_run_id LIMIT $2`, now, limit)
	return ids(rows, err)
}

func ids(rows *sql.Rows, err error) ([]uuid.UUID, error) {
	if err != nil {
		return nil, fmt.Errorf("live-ops scan: %w", err)
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

// CurrentEventVersion implements port.LiveOpsReader.
func (Store) CurrentEventVersion(ctx context.Context, event uuid.UUID) (*port.CurrentVersion, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var cv port.CurrentVersion
	var payload []byte
	var eff, pub sql.NullTime
	// tenancyguard:ok G/T event by id (platform rows or the session tenant, RLS); deleted_at filtered
	err = t.QueryRowContext(ctx, `SELECT v.event_version_id, v.version_no, v.payload, v.effective_from, v.published_at FROM game.events e
		JOIN game.event_versions v ON v.event_version_id = e.current_version_id AND v.deleted_at IS NULL
		WHERE e.event_id = $1 AND e.deleted_at IS NULL`, event).
		Scan(&cv.VersionID, &cv.VersionNo, &payload, &eff, &pub)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("current event version: %w", err)
	}
	if cv.Definition, err = liveops.Parse(payload); err != nil {
		return nil, err
	}
	if eff.Valid {
		e := eff.Time.UTC()
		cv.EffectiveFrom = &e
	}
	if pub.Valid {
		p := pub.Time.UTC()
		cv.PublishedAt = &p
	}
	return &cv, nil
}

// ---------- goals ----------

const goalColumns = `g.goal_id, g.event_run_id, g.tenant_id, g.goal_code, g.metric, g.scope, g.target, g.per_student_daily_cap,
	g.show_grade_contributions, g.milestones, g.metric_filter`

func scanGoal(row *sql.Row) (*port.GoalRow, error) {
	var g port.GoalRow
	var tenant uuid.NullUUID
	var ms, filter []byte
	err := row.Scan(&g.GoalID, &g.RunID, &tenant, &g.Code, &g.Metric, &g.Scope, &g.Target, &g.DailyCap, &g.ShowGrades, &ms, &filter)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("community goal: %w", err)
	}
	if tenant.Valid {
		t := tenant.UUID
		g.TenantID = &t
	}
	if err := json.Unmarshal(ms, &g.Milestones); err != nil {
		return nil, err
	}
	if len(filter) > 0 && string(filter) != "null" {
		var f liveops.MetricFilter
		if err := json.Unmarshal(filter, &f); err == nil && (len(f.SubjectIDs) > 0 || len(f.ObjectiveIDs) > 0) {
			g.Filter = &f
		}
	}
	return &g, nil
}

// RunGoal implements port.LiveOpsReader.
func (Store) RunGoal(ctx context.Context, run uuid.UUID) (*port.GoalRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok G/T goals: platform rows or the session tenant (RLS); deleted_at filtered
	return scanGoal(t.QueryRowContext(ctx, `SELECT `+goalColumns+` FROM game.community_goals g WHERE g.event_run_id = $1
		AND g.deleted_at IS NULL LIMIT 1`, run))
}

// Goal implements port.LiveOpsReader.
func (Store) Goal(ctx context.Context, goal uuid.UUID) (*port.GoalRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok G/T goals: platform rows or the session tenant (RLS); deleted_at filtered
	return scanGoal(t.QueryRowContext(ctx, `SELECT `+goalColumns+` FROM game.community_goals g WHERE g.goal_id = $1
		AND g.deleted_at IS NULL`, goal))
}

// UpsertGoal implements write.LiveOpsStore.
func (Store) UpsertGoal(ctx context.Context, g port.GoalRow, at time.Time) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	ms, err := jsonOf(g.Milestones)
	if err != nil {
		return err
	}
	filter := "{}"
	if g.Filter != nil {
		if filter, err = jsonOf(g.Filter); err != nil {
			return err
		}
	}
	// tenancyguard:ok G/T goal of the run being published (its scope owns it); upsert on the primary key
	_, err = t.ExecContext(ctx, `INSERT INTO game.community_goals (goal_id, tenant_id, created_at, updated_at, version, event_run_id, goal_code, metric,
			scope, target, per_student_daily_cap, show_grade_contributions, milestones, metric_filter)
		VALUES ($1, $2, $3, $3, 1, $4, $5, $6, $7, $8, $9, $10, $11::jsonb, $12::jsonb)
		ON CONFLICT (goal_id) DO UPDATE SET metric = EXCLUDED.metric, scope = EXCLUDED.scope, target = EXCLUDED.target,
			per_student_daily_cap = EXCLUDED.per_student_daily_cap, show_grade_contributions = EXCLUDED.show_grade_contributions,
			milestones = EXCLUDED.milestones, metric_filter = EXCLUDED.metric_filter, updated_at = EXCLUDED.updated_at,
			version = game.community_goals.version + 1`, g.GoalID, nullUUID(g.TenantID), at, g.RunID, g.Code, g.Metric, g.Scope, g.Target, g.DailyCap,
		g.ShowGrades, ms, filter)
	if err != nil {
		return fmt.Errorf("community goal: %w", err)
	}
	return nil
}

// ---------- participation and event points ----------

func scanParticipation(s scanner) (port.ParticipationRow, error) {
	var p port.ParticipationRow
	var reached, claimed, granted []byte
	if err := s.Scan(&p.ID, &p.TenantID, &p.RunID, &p.StudentID, &p.SchoolID, &p.JoinedAt, &p.Points, &reached, &claimed, &granted); err != nil {
		return p, err
	}
	p.JoinedAt = p.JoinedAt.UTC()
	_ = json.Unmarshal(reached, &p.Reached)
	_ = json.Unmarshal(claimed, &p.Claimed)
	_ = json.Unmarshal(granted, &p.Granted)
	for _, s := range []*[]string{&p.Reached, &p.Claimed, &p.Granted} {
		if *s == nil {
			*s = []string{}
		}
	}
	return p, nil
}

const participationColumns = `participation_id, tenant_id, event_run_id, student_id, school_id, joined_at, event_points, reached_milestones,
	claimed_milestones, auto_granted_milestones`

func (Store) participation(ctx context.Context, tenant, run, student uuid.UUID, lock bool) (*port.ParticipationRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	q := `SELECT ` + participationColumns + ` FROM game.event_participation WHERE tenant_id = $1 AND event_run_id = $2 AND student_id = $3
		AND deleted_at IS NULL`
	if lock {
		q += ` FOR UPDATE`
	}
	p, err := scanParticipation(t.QueryRowContext(ctx, q, tenant, run, student))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("event participation: %w", err)
	}
	return &p, nil
}

// Participation implements port.LiveOpsReader.
func (s Store) Participation(ctx context.Context, tenant, run, student uuid.UUID) (*port.ParticipationRow, error) {
	return s.participation(ctx, tenant, run, student, false)
}

// LockParticipation implements write.LiveOpsStore.
func (s Store) LockParticipation(ctx context.Context, tenant, run, student uuid.UUID) (*port.ParticipationRow, error) {
	return s.participation(ctx, tenant, run, student, true)
}

// InsertParticipation implements write.LiveOpsStore.
func (Store) InsertParticipation(ctx context.Context, p port.ParticipationRow) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	res, err := t.ExecContext(ctx, `INSERT INTO game.event_participation (participation_id, tenant_id, created_at, updated_at, version, event_run_id,
			student_id, school_id, joined_at, event_points, reached_milestones, claimed_milestones, auto_granted_milestones)
		VALUES ($1, $2, $3, $3, 1, $4, $5, $6, $3, 0, '[]', '[]', '[]')
		ON CONFLICT (event_run_id, student_id) WHERE deleted_at IS NULL DO NOTHING`, p.ID, p.TenantID, p.JoinedAt, p.RunID, p.StudentID, p.SchoolID)
	if err != nil {
		return false, fmt.Errorf("insert event participation: %w", err)
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// SaveParticipation implements write.LiveOpsStore.
func (Store) SaveParticipation(ctx context.Context, p port.ParticipationRow, at time.Time) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	reached, _ := jsonOf(nonNil(p.Reached))
	claimed, _ := jsonOf(nonNil(p.Claimed))
	granted, _ := jsonOf(nonNil(p.Granted))
	_, err = t.ExecContext(ctx, `UPDATE game.event_participation SET event_points = $3, reached_milestones = $4::jsonb, claimed_milestones = $5::jsonb,
			auto_granted_milestones = $6::jsonb, updated_at = $7, version = version + 1
		WHERE participation_id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, p.ID, p.TenantID, p.Points, reached, claimed, granted, at)
	if err != nil {
		return fmt.Errorf("save event participation: %w", err)
	}
	return nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// Participants implements write.LiveOpsStore.
func (Store) Participants(ctx context.Context, tenant, run, after uuid.UUID, limit int) ([]port.ParticipationRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, `SELECT `+participationColumns+` FROM game.event_participation WHERE tenant_id = $1 AND event_run_id = $2
		AND deleted_at IS NULL AND participation_id > $3 ORDER BY participation_id LIMIT $4`, tenant, run, after, limit)
	if err != nil {
		return nil, fmt.Errorf("event participants: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.ParticipationRow
	for rows.Next() {
		p, err := scanParticipation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ParticipantTenants implements write.LiveOpsStore (platform scan).
func (Store) ParticipantTenants(ctx context.Context, run uuid.UUID) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform scan (SystemPlatformScope): the tenants of one run, then each is handled in its tenant
	rows, err := t.QueryContext(ctx, `SELECT DISTINCT tenant_id FROM game.event_participation WHERE event_run_id = $1 AND deleted_at IS NULL
		ORDER BY tenant_id`, run)
	return ids(rows, err)
}

// LockStudent implements write.LiveOpsStore.
func (Store) LockStudent(ctx context.Context, student uuid.UUID) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok advisory lock only (no table read)
	_, err = t.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('game.liveops:' || $1::text, 0))`, student)
	return err
}

// LockTenant implements write.LiveOpsStore.
func (Store) LockTenant(ctx context.Context, tenant uuid.UUID) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok advisory lock only (no table read)
	_, err = t.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('game.school_events:' || $1::text, 0))`, tenant)
	return err
}

func insertPoints(ctx context.Context, table, refCol string, e write.PointEntry) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok append-only ledger of the tenant (TA): insert with explicit tenant_id; table name from a closed set
	res, err := t.ExecContext(ctx, fmt.Sprintf(`INSERT INTO game.%s (entry_id, tenant_id, created_at, %s, student_id, delta, source_code,
			source_event_id, capped, earned_on) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (%s, source_event_id, student_id) DO NOTHING`, table, refCol, refCol), e.ID, e.TenantID, e.At, e.Ref, e.StudentID, e.Delta,
		e.SourceCode, e.SourceEventID, e.Capped, e.Day)
	if err != nil {
		return false, fmt.Errorf("%s: %w", table, err)
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// InsertEventPoints implements write.LiveOpsStore.
func (Store) InsertEventPoints(ctx context.Context, e write.PointEntry) (bool, error) {
	return insertPoints(ctx, "event_point_ledger", "event_run_id", e)
}

// InsertSeasonPoints implements write.LiveOpsStore.
func (Store) InsertSeasonPoints(ctx context.Context, e write.PointEntry) (bool, error) {
	return insertPoints(ctx, "season_point_ledger", "season_id", e)
}

func sumOn(ctx context.Context, q string, args ...any) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	if err := t.QueryRowContext(ctx, q, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("daily sum: %w", err)
	}
	return n, nil
}

// EventPointsOn implements port.LiveOpsReader.
func (Store) EventPointsOn(ctx context.Context, tenant, run, student uuid.UUID, day time.Time) (int, error) {
	// tenancyguard:ok append-only ledger (TA, no deleted_at): explicit tenant_id
	return sumOn(ctx, `SELECT coalesce(sum(delta), 0) FROM game.event_point_ledger WHERE tenant_id = $1 AND event_run_id = $2
		AND student_id = $3 AND earned_on = $4`, tenant, run, student, day)
}

// SeasonPointsOn implements port.LiveOpsReader.
func (Store) SeasonPointsOn(ctx context.Context, tenant, season, student uuid.UUID, day time.Time) (int, error) {
	// tenancyguard:ok append-only ledger (TA, no deleted_at): explicit tenant_id
	return sumOn(ctx, `SELECT coalesce(sum(delta), 0) FROM game.season_point_ledger WHERE tenant_id = $1 AND season_id = $2
		AND student_id = $3 AND earned_on = $4`, tenant, season, student, day)
}

// ContributedOn implements port.LiveOpsReader.
func (Store) ContributedOn(ctx context.Context, tenant, goal, student uuid.UUID, day time.Time) (int, error) {
	// tenancyguard:ok append-only contributions (TA, no deleted_at): explicit tenant_id
	return sumOn(ctx, `SELECT coalesce(sum(amount), 0) FROM game.community_goal_contributions WHERE tenant_id = $1 AND goal_id = $2
		AND student_id = $3 AND contributed_on = $4`, tenant, goal, student, day)
}

// Contributed implements port.LiveOpsReader.
func (Store) Contributed(ctx context.Context, tenant, goal, student uuid.UUID) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	var ok bool
	// tenancyguard:ok append-only contributions (TA, no deleted_at): explicit tenant_id
	err = t.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM game.community_goal_contributions WHERE tenant_id = $1 AND goal_id = $2
		AND student_id = $3)`, tenant, goal, student).Scan(&ok)
	return ok, err
}

// ---------- community goal ----------

// InsertContribution implements write.LiveOpsStore.
func (Store) InsertContribution(ctx context.Context, c write.Contribution) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	var grade sql.NullInt64
	if c.Grade != nil {
		grade = sql.NullInt64{Int64: int64(*c.Grade), Valid: true}
	}
	res, err := t.ExecContext(ctx, `INSERT INTO game.community_goal_contributions (contribution_id, tenant_id, created_at, goal_id, scope_id, student_id,
			school_id, grade_level, amount, source_event_id, contributed_on) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (goal_id, source_event_id, student_id) DO NOTHING`, c.ID, c.TenantID, c.At, c.GoalID, c.ScopeID, c.StudentID, c.SchoolID, grade,
		c.Amount, c.SourceEventID, c.Day)
	if err != nil {
		return false, fmt.Errorf("community goal contribution: %w", err)
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func scanProgress(s scanner, p *port.ProgressRow) error {
	var reached []byte
	if err := s.Scan(&p.Total, &reached); err != nil {
		return err
	}
	_ = json.Unmarshal(reached, &p.Reached)
	if p.Reached == nil {
		p.Reached = []int{}
	}
	return nil
}

// AddProgress implements write.LiveOpsStore.
func (Store) AddProgress(ctx context.Context, tenant, goal, scope uuid.UUID, amount int64, at time.Time) (port.ProgressRow, error) {
	p := port.ProgressRow{TenantID: tenant, GoalID: goal, ScopeID: scope}
	t, err := tx(ctx)
	if err != nil {
		return p, err
	}
	err = scanProgress(t.QueryRowContext(ctx, `INSERT INTO game.community_goal_progress (progress_id, tenant_id, created_at, updated_at, version, goal_id,
			scope_id, total, milestones_reached) VALUES (gen_random_uuid(), $1, $2, $2, 1, $3, $4, $5, '[]')
		ON CONFLICT (goal_id, scope_id) WHERE deleted_at IS NULL DO UPDATE SET total = game.community_goal_progress.total + EXCLUDED.total,
			updated_at = EXCLUDED.updated_at, version = game.community_goal_progress.version + 1
		RETURNING total, milestones_reached`, tenant, at, goal, scope, amount), &p)
	if err != nil {
		return p, fmt.Errorf("community goal progress: %w", err)
	}
	return p, nil
}

// Progress implements port.LiveOpsReader.
func (Store) Progress(ctx context.Context, tenant, goal, scope uuid.UUID) (port.ProgressRow, error) {
	p := port.ProgressRow{TenantID: tenant, GoalID: goal, ScopeID: scope, Reached: []int{}}
	t, err := tx(ctx)
	if err != nil {
		return p, err
	}
	err = scanProgress(t.QueryRowContext(ctx, `SELECT total, milestones_reached FROM game.community_goal_progress WHERE tenant_id = $1
		AND goal_id = $2 AND scope_id = $3 AND deleted_at IS NULL`, tenant, goal, scope), &p)
	if errors.Is(err, sql.ErrNoRows) {
		return p, nil
	}
	return p, err
}

// SetReached implements write.LiveOpsStore.
func (Store) SetReached(ctx context.Context, tenant, goal, scope uuid.UUID, reached []int, at time.Time) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	r, err := jsonOf(reached)
	if err != nil {
		return err
	}
	_, err = t.ExecContext(ctx, `INSERT INTO game.community_goal_progress (progress_id, tenant_id, created_at, updated_at, version, goal_id, scope_id,
			total, milestones_reached) VALUES (gen_random_uuid(), $1, $5, $5, 1, $2, $3, 0, $4::jsonb)
		ON CONFLICT (goal_id, scope_id) WHERE deleted_at IS NULL DO UPDATE SET milestones_reached = EXCLUDED.milestones_reached,
			updated_at = EXCLUDED.updated_at, version = game.community_goal_progress.version + 1`, tenant, goal, scope, r, at)
	if err != nil {
		return fmt.Errorf("community goal milestones: %w", err)
	}
	return nil
}

// SetTotal implements write.LiveOpsStore.
func (Store) SetTotal(ctx context.Context, tenant, goal, scope uuid.UUID, total int64, at time.Time) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	_, err = t.ExecContext(ctx, `UPDATE game.community_goal_progress SET total = $4, recomputed_at = $5, updated_at = $5, version = version + 1
		WHERE tenant_id = $1 AND goal_id = $2 AND scope_id = $3 AND deleted_at IS NULL`, tenant, goal, scope, total, at)
	if err != nil {
		return fmt.Errorf("community goal total: %w", err)
	}
	return nil
}

// PlatformTotal implements port.LiveOpsReader: the SECURITY DEFINER sum over every tenant (a count, never a student).
func (Store) PlatformTotal(ctx context.Context, goal uuid.UUID) (int64, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var n int64
	// tenancyguard:ok SECURITY DEFINER function game.community_goal_platform_total (sum of the progress rows of one goal)
	if err := t.QueryRowContext(ctx, `SELECT game.community_goal_platform_total($1)`, goal).Scan(&n); err != nil {
		return 0, fmt.Errorf("platform goal total: %w", err)
	}
	return n, nil
}

// Contributors implements write.LiveOpsStore.
func (Store) Contributors(ctx context.Context, tenant, goal, scope, after uuid.UUID, limit int) ([]write.Contributor, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok append-only contributions (TA, no deleted_at): explicit tenant_id
	rows, err := t.QueryContext(ctx, `SELECT DISTINCT ON (student_id) student_id, school_id FROM game.community_goal_contributions
		WHERE tenant_id = $1 AND goal_id = $2 AND scope_id = $3 AND student_id > $4 ORDER BY student_id LIMIT $5`, tenant, goal, scope, after, limit)
	if err != nil {
		return nil, fmt.Errorf("goal contributors: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.Contributor
	for rows.Next() {
		var c write.Contributor
		if err := rows.Scan(&c.StudentID, &c.SchoolID); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GoalScopes implements write.LiveOpsStore (platform scan of goal-recompute).
func (Store) GoalScopes(ctx context.Context, after *write.GoalScope, limit int) ([]write.GoalScope, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var ag, as, at uuid.UUID
	if after != nil {
		ag, as, at = after.GoalID, after.ScopeID, after.TenantID
	}
	// tenancyguard:ok platform scan (SystemPlatformScope, every tenant by design); deleted_at filtered; contributions TA
	rows, err := t.QueryContext(ctx, `SELECT p.tenant_id, p.goal_id, p.scope_id,
			coalesce((SELECT sum(c.amount) FROM game.community_goal_contributions c WHERE c.tenant_id = p.tenant_id AND c.goal_id = p.goal_id
				AND c.scope_id = p.scope_id), 0), p.total, p.milestones_reached
		FROM game.community_goal_progress p
		JOIN game.community_goals g ON g.goal_id = p.goal_id AND g.deleted_at IS NULL
		JOIN game.event_runs r ON r.event_run_id = g.event_run_id AND r.deleted_at IS NULL AND r.status <> 'archived'
		WHERE p.deleted_at IS NULL AND (p.goal_id, p.scope_id, p.tenant_id) > ($1, $2, $3)
		ORDER BY p.goal_id, p.scope_id, p.tenant_id LIMIT $4`, ag, as, at, limit)
	if err != nil {
		return nil, fmt.Errorf("goal scopes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.GoalScope
	for rows.Next() {
		var s write.GoalScope
		var reached []byte
		if err := rows.Scan(&s.TenantID, &s.GoalID, &s.ScopeID, &s.Total, &s.Cached, &reached); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(reached, &s.Reached)
		out = append(out, s)
	}
	return out, rows.Err()
}

// GradeContributions implements port.LiveOpsReader.
func (Store) GradeContributions(ctx context.Context, tenant, goal, school uuid.UUID) ([]port.GradeContribution, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok append-only contributions (TA, no deleted_at): explicit tenant_id; aggregates only, no student id
	rows, err := t.QueryContext(ctx, `SELECT grade_level, count(DISTINCT student_id), sum(amount) FROM game.community_goal_contributions
		WHERE tenant_id = $1 AND goal_id = $2 AND school_id = $3 AND grade_level IS NOT NULL GROUP BY grade_level ORDER BY grade_level`,
		tenant, goal, school)
	if err != nil {
		return nil, fmt.Errorf("grade contributions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.GradeContribution
	for rows.Next() {
		var g port.GradeContribution
		if err := rows.Scan(&g.Grade, &g.Contributors, &g.Progress); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// ---------- templates and school events ----------

// Template implements port.LiveOpsReader.
func (Store) Template(ctx context.Context, tenant, template uuid.UUID) (*port.TemplateRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var r port.TemplateRow
	var payload []byte
	// tenancyguard:ok platform content (G tables, deleted_at filtered) + infra.tenant_resource_assignments (explicit tenant_id)
	err = t.QueryRowContext(ctx, `SELECT t.template_id, v.template_version_id, v.version_no, v.payload FROM game.event_templates t
		JOIN game.event_template_versions v ON v.template_version_id = t.current_version_id AND v.deleted_at IS NULL AND v.status = 'published'
		WHERE t.template_id = $2 AND t.deleted_at IS NULL AND EXISTS (SELECT 1 FROM infra.tenant_resource_assignments a
			WHERE a.resource_type = 'event_template' AND a.resource_id = t.template_id AND a.tenant_id = $1 AND a.deleted_at IS NULL)`,
		tenant, template).Scan(&r.TemplateID, &r.VersionID, &r.VersionNo, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("event template: %w", err)
	}
	if r.Template, err = liveops.ParseTemplate(payload); err != nil {
		return nil, err
	}
	return &r, nil
}

// Templates implements read.LiveOpsReadModel.
func (Store) Templates(ctx context.Context, tenant uuid.UUID, afterCode string, limit int) ([]port.TemplateRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G tables, deleted_at filtered) + infra.tenant_resource_assignments (explicit tenant_id)
	rows, err := t.QueryContext(ctx, `SELECT t.template_id, v.template_version_id, v.version_no, v.payload FROM game.event_templates t
		JOIN game.event_template_versions v ON v.template_version_id = t.current_version_id AND v.deleted_at IS NULL AND v.status = 'published'
		WHERE t.deleted_at IS NULL AND v.code > $2 AND EXISTS (SELECT 1 FROM infra.tenant_resource_assignments a
			WHERE a.resource_type = 'event_template' AND a.resource_id = t.template_id AND a.tenant_id = $1 AND a.deleted_at IS NULL)
		ORDER BY v.code LIMIT $3`, tenant, afterCode, limit)
	if err != nil {
		return nil, fmt.Errorf("event templates: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.TemplateRow
	for rows.Next() {
		var r port.TemplateRow
		var payload []byte
		if err := rows.Scan(&r.TemplateID, &r.VersionID, &r.VersionNo, &payload); err != nil {
			return nil, err
		}
		if r.Template, err = liveops.ParseTemplate(payload); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// KnownSchools implements port.LiveOpsReader.
func (Store) KnownSchools(ctx context.Context, tenant uuid.UUID, schools []uuid.UUID) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	// tenancyguard:ok projection ref.schools (RLS + explicit tenant_id; the projection has no deleted_at)
	err = t.QueryRowContext(ctx, `SELECT count(DISTINCT school_id) FROM ref.schools WHERE tenant_id = $1 AND school_id = ANY($2::uuid[])`,
		tenant, schools).Scan(&n)
	return n, err
}

// ActiveTenantEvents implements write.LiveOpsStore.
func (Store) ActiveTenantEvents(ctx context.Context, tenant uuid.UUID) (int, error) {
	return sumOn(ctx, `SELECT count(*) FROM game.event_runs WHERE tenant_id = $1 AND deleted_at IS NULL
		AND status IN ('scheduled', 'live', 'suspended')`, tenant)
}

// InsertTenantEvent implements write.LiveOpsStore.
func (Store) InsertTenantEvent(ctx context.Context, tenant, event uuid.UUID, code, kind string, template uuid.UUID, school *uuid.UUID,
	params map[string]any, by uuid.UUID, at time.Time) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	pj, err := jsonOf(params)
	if err != nil {
		return err
	}
	_, err = t.ExecContext(ctx, `INSERT INTO game.events (event_id, tenant_id, created_at, created_by, updated_at, updated_by, version, owner_scope, code,
			event_kind, template_id, school_id, school_params, latest_version_no)
		VALUES ($1, $2, $3, $4, $3, $4, 1, 'tenant', $5, $6, $7, $8, $9::jsonb, 0)`, event, tenant, at, by, code, kind, template, nullUUID(school), pj)
	if err != nil {
		return fmt.Errorf("insert school event: %w", err)
	}
	return nil
}

// InsertTenantVersion implements write.LiveOpsStore.
func (Store) InsertTenantVersion(ctx context.Context, v write.EventVersionRow, params map[string]any) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	pj, err := jsonOf(params)
	if err != nil {
		return err
	}
	if _, err := t.ExecContext(ctx, `INSERT INTO game.event_versions (event_version_id, tenant_id, created_at, created_by, updated_at, version,
			event_id, version_no, status, payload, payload_sha256, published_at, published_by, event_kind, start_at, end_at, effective_from,
			decoration_areas)
		VALUES ($1, $2, $3, $4, $3, 1, $5, $6, 'published', $7::jsonb, $8, $3, $4, $9, $10, $11, $12, '[]')`, v.VersionID, v.TenantID, v.At, v.By,
		v.EventID, v.VersionNo, string(v.Payload), v.SHA256, v.Definition.Kind, v.Definition.Schedule.StartAt, v.Definition.Schedule.EndAt,
		v.EffectiveFrom); err != nil {
		return fmt.Errorf("insert school event version: %w", err)
	}
	if _, err := t.ExecContext(ctx, `UPDATE game.event_versions SET status = 'archived', updated_at = $4 WHERE tenant_id = $1 AND event_id = $2
		AND event_version_id <> $3 AND status = 'published' AND deleted_at IS NULL`, v.TenantID, v.EventID, v.VersionID, v.At); err != nil {
		return fmt.Errorf("archive school event version: %w", err)
	}
	if _, err := t.ExecContext(ctx, `UPDATE game.events SET current_version_id = $3, latest_version_no = $4, school_params = $5::jsonb,
			updated_at = $6, updated_by = $7, version = version + 1 WHERE tenant_id = $1 AND event_id = $2 AND deleted_at IS NULL`,
		v.TenantID, v.EventID, v.VersionID, v.VersionNo, pj, v.At, v.By); err != nil {
		return fmt.Errorf("school event pointer: %w", err)
	}
	return nil
}

const schoolEventColumns = `e.event_id, e.tenant_id, e.school_id, e.template_id, e.version, e.school_params, e.created_by, e.created_at,
	(SELECT r.event_run_id FROM game.event_runs r WHERE r.event_id = e.event_id AND r.tenant_id = e.tenant_id AND r.deleted_at IS NULL
		ORDER BY r.created_at DESC LIMIT 1)`

func (s Store) withRun(ctx context.Context, r *port.SchoolEventRow, run uuid.NullUUID) error {
	if !run.Valid {
		return nil
	}
	rr, err := s.Run(ctx, run.UUID)
	if err != nil {
		return err
	}
	r.Run = rr
	return nil
}

func (s Store) schoolEvent(ctx context.Context, tenant, event uuid.UUID, lock bool) (*port.SchoolEventRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	q := `SELECT ` + schoolEventColumns + ` FROM game.events e WHERE e.tenant_id = $1 AND e.event_id = $2 AND e.owner_scope = 'tenant'
		AND e.deleted_at IS NULL`
	if lock {
		q += ` FOR UPDATE OF e`
	}
	var run uuid.NullUUID
	row := t.QueryRowContext(ctx, q, tenant, event)
	var r port.SchoolEventRow
	var school, by uuid.NullUUID
	var params []byte
	err = row.Scan(&r.EventID, &r.TenantID, &school, &r.TemplateID, &r.Version, &params, &by, &r.CreatedAt, &run)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("school event: %w", err)
	}
	if school.Valid {
		v := school.UUID
		r.SchoolID = &v
	}
	if by.Valid {
		v := by.UUID
		r.CreatedBy = &v
	}
	r.CreatedAt = r.CreatedAt.UTC()
	_ = json.Unmarshal(params, &r.Params)
	if err := s.withRun(ctx, &r, run); err != nil {
		return nil, err
	}
	return &r, nil
}

// LockSchoolEvent implements write.LiveOpsStore.
func (s Store) LockSchoolEvent(ctx context.Context, tenant, event uuid.UUID) (*port.SchoolEventRow, error) {
	return s.schoolEvent(ctx, tenant, event, true)
}

// SchoolEvent implements read.LiveOpsReadModel.
func (s Store) SchoolEvent(ctx context.Context, tenant, event uuid.UUID) (*port.SchoolEventRow, error) {
	return s.schoolEvent(ctx, tenant, event, false)
}

// schoolStatus is the SchoolEvent.status of a run (suspended shows as live, archived as ended or cancelled).
const schoolStatus = `CASE WHEN r.status IN ('live', 'suspended') THEN 'live' WHEN r.status = 'archived' AND r.cancelled_at IS NOT NULL THEN 'cancelled'
	WHEN r.status = 'archived' THEN 'ended' ELSE r.status END`

// SchoolEvents implements read.LiveOpsReadModel: newest first, keyset on (created_at, event_id), school-limited staff
// see the events of their schools only.
func (s Store) SchoolEvents(ctx context.Context, f read.SchoolEventsFilter) ([]port.SchoolEventRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	before, beforeID := time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC), uuid.Max
	if f.After != nil {
		before, beforeID = f.After.StartsAt, f.After.ID
	}
	all := f.Schools == nil
	schools := f.Schools
	if schools == nil {
		schools = []uuid.UUID{}
	}
	rows, err := t.QueryContext(ctx, `SELECT `+schoolEventColumns+` FROM game.events e
		LEFT JOIN LATERAL (SELECT r.status, r.cancelled_at FROM game.event_runs r WHERE r.event_id = e.event_id AND r.tenant_id = e.tenant_id
			AND r.deleted_at IS NULL ORDER BY r.created_at DESC LIMIT 1) r ON true
		WHERE e.tenant_id = $1 AND e.owner_scope = 'tenant' AND e.deleted_at IS NULL AND ($2 OR e.school_id = ANY($3::uuid[]))
			AND ($4 = '' OR `+schoolStatus+` = $4) AND (e.created_at, e.event_id) < ($5, $6)
		ORDER BY e.created_at DESC, e.event_id DESC LIMIT $7`, f.Tenant, all, schools, f.Status, before, beforeID, f.Limit)
	if err != nil {
		return nil, fmt.Errorf("school events: %w", err)
	}
	items, err := scanSchoolEvents(rows)
	if err != nil {
		return nil, err
	}
	out := make([]port.SchoolEventRow, 0, len(items))
	for _, it := range items {
		r := it.row
		if err := s.withRun(ctx, &r, it.run); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// ---------- player lists ----------

// StudentRuns implements read.LiveOpsReadModel (F077-AC-07): the audience (tenant, school, grade) and the status window
// are SQL conditions; keyset on (starts_at, event_run_id).
func (Store) StudentRuns(ctx context.Context, f read.StudentRunsFilter) ([]port.EventRunRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	afterAt, afterID := time.Time{}, uuid.Nil
	if f.After != nil {
		afterAt, afterID = f.After.StartsAt, f.After.ID
	}
	// tenancyguard:ok G/T runs: platform rows or explicit tenant_id; deleted_at filtered
	return runRows(t.QueryContext(ctx, `SELECT `+runColumns+` FROM `+runJoin+` WHERE r.deleted_at IS NULL
		AND (r.tenant_id = $1 OR (r.tenant_id IS NULL AND (jsonb_array_length(coalesce(r.audience->'tenant_ids', '[]'::jsonb)) = 0
			OR r.audience->'tenant_ids' @> to_jsonb($1::text))))
		AND (jsonb_array_length(coalesce(r.audience->'school_ids', '[]'::jsonb)) = 0 OR r.audience->'school_ids' @> to_jsonb($2::text))
		AND r.audience->'grade_levels' @> to_jsonb($3::int)
		AND (($4 AND (r.status IN ('live', 'suspended') OR (r.status = 'ended' AND r.claim_until > $5)))
			OR ($6 AND r.status = 'scheduled' AND r.starts_at <= $7))
		AND (r.starts_at, r.event_run_id) > ($8, $9)
		ORDER BY r.starts_at, r.event_run_id LIMIT $10`, f.Tenant, f.School, f.Grade, f.Live, f.Now, f.Soon, f.Now.Add(f.Upcoming), afterAt, afterID,
		f.Limit))
}

// ManifestEvents implements port.ManifestEvents (F077-AC-02, AC-07): the live runs of the tenant's audience with their
// components and audience (the manifest query keeps the entries of the student's school and grade).
func (Store) ManifestEvents(ctx context.Context, tenant uuid.UUID) ([]map[string]any, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok G/T runs: platform rows or explicit tenant_id; deleted_at filtered
	runs, err := runRows(t.QueryContext(ctx, `SELECT `+runColumns+` FROM `+runJoin+` WHERE r.deleted_at IS NULL AND r.status = 'live'
		AND (r.tenant_id = $1 OR (r.tenant_id IS NULL AND (jsonb_array_length(coalesce(r.audience->'tenant_ids', '[]'::jsonb)) = 0
			OR r.audience->'tenant_ids' @> to_jsonb($1::text))))
		ORDER BY r.starts_at, r.event_run_id`, tenant))
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(runs))
	for _, r := range runs {
		c := r.Definition.Components
		comp := map[string]any{"quest_ids": nonNilIDs(c.QuestIDs), "challenge_ids": nonNilIDs(c.ChallengeIDs), "boss_ids": nonNilIDs(c.BossIDs),
			"shop_item_ids": nonNilIDs(c.ShopItemIDs)}
		if c.CollectionID != nil {
			comp["collection_id"] = c.CollectionID
		}
		if c.Decoration != nil {
			comp["decoration"] = c.Decoration
		}
		out = append(out, map[string]any{"item_type": "event", "item_id": r.EventID, "item_code": r.Definition.Code, "version_id": r.VersionID,
			"version_no": r.VersionNo, "event_run_id": r.RunID, "audience": map[string]any{"school_ids": nonNilIDs(r.Audience.SchoolIDs),
				"grade_levels": r.Audience.GradeLevels}, "components": comp})
	}
	return out, nil
}

func nonNilIDs(s []uuid.UUID) []uuid.UUID {
	if s == nil {
		return []uuid.UUID{}
	}
	return s
}

// Running implements port.LiveOps (the limited shop listings of F075, INVENTORY_ECONOMY §5.1): the listing is sold while
// its event has a live run of the tenant's audience and its season is started and holds at.
func (Store) Running(ctx context.Context, tenant uuid.UUID, event, season *uuid.UUID, at time.Time) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	var ok bool
	// tenancyguard:ok G/T runs (platform rows or explicit tenant_id, deleted_at filtered) + projection ref.seasons (platform)
	err = t.QueryRowContext(ctx, `SELECT ($2::uuid IS NULL OR EXISTS (SELECT 1 FROM game.event_runs r WHERE r.event_id = $2 AND r.status = 'live'
			AND r.deleted_at IS NULL AND (r.tenant_id = $1 OR (r.tenant_id IS NULL AND (jsonb_array_length(coalesce(r.audience->'tenant_ids', '[]'::jsonb)) = 0
				OR r.audience->'tenant_ids' @> to_jsonb($1::text))))))
		AND ($3::uuid IS NULL OR EXISTS (SELECT 1 FROM ref.seasons s WHERE s.season_id = $3 AND s.status = 'started' AND s.starts_at <= $4
			AND s.ends_at > $4))`, tenant, nullUUID(event), nullUUID(season), at).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("live-ops running: %w", err)
	}
	return ok, nil
}

// ---------- seasons ----------

const seasonColumns = `season_id, code, name, theme_code, starts_at, ends_at, ended_at, status, source_version`

func scanSeason(s scanner) (*port.SeasonRow, error) {
	var r port.SeasonRow
	var theme sql.NullString
	var ended sql.NullTime
	err := s.Scan(&r.SeasonID, &r.Code, &r.Name, &theme, &r.StartsAt, &r.EndsAt, &ended, &r.Status, &r.SourceVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("season: %w", err)
	}
	if theme.Valid {
		r.ThemeCode = &theme.String
	}
	if ended.Valid {
		e := ended.Time.UTC()
		r.EndedAt = &e
	}
	r.StartsAt, r.EndsAt = r.StartsAt.UTC(), r.EndsAt.UTC()
	return &r, nil
}

// Season implements port.LiveOpsReader.
func (Store) Season(ctx context.Context, id uuid.UUID) (*port.SeasonRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform projection ref.seasons (no tenant_id, no deleted_at)
	return scanSeason(t.QueryRowContext(ctx, `SELECT `+seasonColumns+` FROM ref.seasons WHERE season_id = $1`, id))
}

// CurrentSeason implements port.LiveOpsReader.
func (Store) CurrentSeason(ctx context.Context, at time.Time) (*port.SeasonRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform projection ref.seasons (no tenant_id, no deleted_at)
	return scanSeason(t.QueryRowContext(ctx, `SELECT `+seasonColumns+` FROM ref.seasons WHERE status = 'started' AND starts_at <= $1
		AND ends_at > $1 ORDER BY starts_at DESC LIMIT 1`, at))
}

// UpsertSeason implements write.LiveOpsStore (source_version guard: an older event never overwrites a newer one).
func (Store) UpsertSeason(ctx context.Context, s port.SeasonRow) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	var theme sql.NullString
	if s.ThemeCode != nil {
		theme = sql.NullString{String: *s.ThemeCode, Valid: true}
	}
	var ended sql.NullTime
	if s.EndedAt != nil {
		ended = sql.NullTime{Time: *s.EndedAt, Valid: true}
	}
	// tenancyguard:ok platform projection ref.seasons (no tenant_id), written only by game.season_projection
	res, err := t.ExecContext(ctx, `INSERT INTO ref.seasons (season_id, code, name, theme_code, starts_at, ends_at, ended_at, status, source_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (season_id) DO UPDATE SET code = EXCLUDED.code, name = EXCLUDED.name, theme_code = EXCLUDED.theme_code,
			starts_at = EXCLUDED.starts_at, ends_at = EXCLUDED.ends_at, ended_at = EXCLUDED.ended_at, status = EXCLUDED.status,
			source_version = EXCLUDED.source_version
		WHERE ref.seasons.source_version < EXCLUDED.source_version`, s.SeasonID, s.Code, s.Name, theme, s.StartsAt, s.EndsAt, ended, s.Status,
		s.SourceVersion)
	if err != nil {
		return false, fmt.Errorf("season projection: %w", err)
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// Track implements port.LiveOpsReader.
func (Store) Track(ctx context.Context, seasonID uuid.UUID) (*port.TrackRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var r port.TrackRow
	var payload []byte
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered
	err = t.QueryRowContext(ctx, `SELECT s.season_track_id, v.season_track_version_id, v.season_id, v.payload FROM game.season_tracks s
		JOIN game.season_track_versions v ON v.season_track_version_id = s.current_version_id AND v.deleted_at IS NULL AND v.status = 'published'
		WHERE s.season_id = $1 AND s.deleted_at IS NULL`, seasonID).Scan(&r.TrackID, &r.VersionID, &r.SeasonID, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("season track: %w", err)
	}
	if r.Track, err = season.Parse(payload); err != nil {
		return nil, err
	}
	return &r, nil
}

const seasonProgressColumns = `progress_id, tenant_id, season_id, student_id, school_id, points, tier, claimed_tiers, auto_granted_tiers, auto_granted_at`

func scanSeasonProgress(s scanner) (port.SeasonProgressRow, error) {
	var p port.SeasonProgressRow
	var claimed, granted []byte
	var at sql.NullTime
	if err := s.Scan(&p.ID, &p.TenantID, &p.SeasonID, &p.StudentID, &p.SchoolID, &p.Points, &p.Tier, &claimed, &granted, &at); err != nil {
		return p, err
	}
	_ = json.Unmarshal(claimed, &p.Claimed)
	_ = json.Unmarshal(granted, &p.Granted)
	if p.Claimed == nil {
		p.Claimed = []int{}
	}
	if p.Granted == nil {
		p.Granted = []int{}
	}
	if at.Valid {
		v := at.Time.UTC()
		p.AutoGrantedAt = &v
	}
	return p, nil
}

func (Store) seasonProgress(ctx context.Context, tenant, seasonID, student uuid.UUID, lock bool) (*port.SeasonProgressRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	q := `SELECT ` + seasonProgressColumns + ` FROM game.season_track_progress WHERE tenant_id = $1 AND season_id = $2 AND student_id = $3
		AND deleted_at IS NULL`
	if lock {
		q += ` FOR UPDATE`
	}
	p, err := scanSeasonProgress(t.QueryRowContext(ctx, q, tenant, seasonID, student))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("season track progress: %w", err)
	}
	return &p, nil
}

// SeasonProgress implements port.LiveOpsReader.
func (s Store) SeasonProgress(ctx context.Context, tenant, seasonID, student uuid.UUID) (*port.SeasonProgressRow, error) {
	return s.seasonProgress(ctx, tenant, seasonID, student, false)
}

// LockSeasonProgress implements write.LiveOpsStore.
func (s Store) LockSeasonProgress(ctx context.Context, tenant, seasonID, student uuid.UUID) (*port.SeasonProgressRow, error) {
	return s.seasonProgress(ctx, tenant, seasonID, student, true)
}

// SaveSeasonProgress implements write.LiveOpsStore.
func (Store) SaveSeasonProgress(ctx context.Context, p port.SeasonProgressRow, at time.Time) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	claimed, _ := jsonOf(nonNilInts(p.Claimed))
	granted, _ := jsonOf(nonNilInts(p.Granted))
	var auto sql.NullTime
	if p.AutoGrantedAt != nil {
		auto = sql.NullTime{Time: *p.AutoGrantedAt, Valid: true}
	}
	_, err = t.ExecContext(ctx, `INSERT INTO game.season_track_progress (progress_id, tenant_id, created_at, updated_at, version, season_id, student_id,
			school_id, points, tier, claimed_tiers, auto_granted_tiers, auto_granted_at)
		VALUES ($1, $2, $3, $3, 1, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb, $11)
		ON CONFLICT (season_id, student_id) WHERE deleted_at IS NULL DO UPDATE SET points = EXCLUDED.points, tier = EXCLUDED.tier,
			claimed_tiers = EXCLUDED.claimed_tiers, auto_granted_tiers = EXCLUDED.auto_granted_tiers, auto_granted_at = EXCLUDED.auto_granted_at,
			updated_at = EXCLUDED.updated_at, version = game.season_track_progress.version + 1`, p.ID, p.TenantID, at, p.SeasonID, p.StudentID,
		p.SchoolID, p.Points, p.Tier, claimed, granted, auto)
	if err != nil {
		return fmt.Errorf("season track progress: %w", err)
	}
	return nil
}

func nonNilInts(s []int) []int {
	if s == nil {
		return []int{}
	}
	return s
}

// SeasonTenants implements write.LiveOpsStore (platform scan).
func (Store) SeasonTenants(ctx context.Context, seasonID uuid.UUID) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform scan (platform consumer of season.ended): the tenants of one season, each handled in its tenant
	rows, err := t.QueryContext(ctx, `SELECT DISTINCT tenant_id FROM game.season_track_progress WHERE season_id = $1 AND deleted_at IS NULL
		ORDER BY tenant_id`, seasonID)
	return ids(rows, err)
}

// SeasonProgressPage implements write.LiveOpsStore.
func (Store) SeasonProgressPage(ctx context.Context, tenant, seasonID, after uuid.UUID, limit int) ([]port.SeasonProgressRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, `SELECT `+seasonProgressColumns+` FROM game.season_track_progress WHERE tenant_id = $1 AND season_id = $2
		AND deleted_at IS NULL AND progress_id > $3 ORDER BY progress_id LIMIT $4`, tenant, seasonID, after, limit)
	if err != nil {
		return nil, fmt.Errorf("season track progress page: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.SeasonProgressRow
	for rows.Next() {
		p, err := scanSeasonProgress(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SeasonHistory implements read.LiveOpsReadModel (ended seasons, newest first).
func (Store) SeasonHistory(ctx context.Context, tenant, student uuid.UUID, after *read.SeasonCursor, limit int) ([]read.SeasonHistoryRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	before, beforeID := time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC), uuid.Max
	if after != nil {
		before, beforeID = after.EndedAt, after.ID
	}
	// tenancyguard:ok tenant progress (explicit tenant_id + deleted_at) joined to the platform projection ref.seasons
	rows, err := t.QueryContext(ctx, `SELECT p.progress_id, p.tenant_id, p.season_id, p.student_id, p.school_id, p.points, p.tier, p.claimed_tiers,
			p.auto_granted_tiers, p.auto_granted_at, s.name, coalesce(s.ended_at, s.ends_at)
		FROM game.season_track_progress p JOIN ref.seasons s ON s.season_id = p.season_id AND s.status = 'ended'
		WHERE p.tenant_id = $1 AND p.student_id = $2 AND p.deleted_at IS NULL
			AND (coalesce(s.ended_at, s.ends_at), p.season_id) < ($3, $4)
		ORDER BY coalesce(s.ended_at, s.ends_at) DESC, p.season_id DESC LIMIT $5`, tenant, student, before, beforeID, limit)
	if err != nil {
		return nil, fmt.Errorf("season history: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.SeasonHistoryRow
	for rows.Next() {
		var h read.SeasonHistoryRow
		var claimed, granted []byte
		var at sql.NullTime
		if err := rows.Scan(&h.ID, &h.TenantID, &h.SeasonID, &h.StudentID, &h.SchoolID, &h.Points, &h.Tier, &claimed, &granted, &at, &h.SeasonName,
			&h.EndedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(claimed, &h.Claimed)
		_ = json.Unmarshal(granted, &h.Granted)
		h.EndedAt = h.EndedAt.UTC()
		out = append(out, h)
	}
	return out, rows.Err()
}

// ---------- checks, pushes, stats ----------

// QuestRecurrence implements port.LiveOpsReader.
func (Store) QuestRecurrence(ctx context.Context, quests []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	if len(quests) == 0 {
		return out, nil
	}
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered
	rows, err := t.QueryContext(ctx, `SELECT d.quest_id, coalesce(nullif(v.payload->'availability'->>'recurrence', 'none'),
			CASE v.quest_type WHEN 'daily' THEN 'daily' WHEN 'weekly' THEN 'weekly' ELSE 'none' END)
		FROM game.quest_definitions d JOIN game.quest_versions v ON v.quest_version_id = d.current_version_id AND v.deleted_at IS NULL
		WHERE d.quest_id = ANY($1::uuid[]) AND d.deleted_at IS NULL`, quests)
	if err != nil {
		return nil, fmt.Errorf("quest recurrence: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id uuid.UUID
		var r string
		if err := rows.Scan(&id, &r); err != nil {
			return nil, err
		}
		out[id] = r
	}
	return out, rows.Err()
}

// OverlappingDecorations implements port.LiveOpsReader.
func (Store) OverlappingDecorations(ctx context.Context, event uuid.UUID, areas []string, from, to time.Time) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform events (G rows, tenant_id NULL); deleted_at filtered
	rows, err := t.QueryContext(ctx, `SELECT e.event_id FROM game.events e
		JOIN game.event_versions v ON v.event_version_id = e.current_version_id AND v.deleted_at IS NULL
		WHERE e.deleted_at IS NULL AND e.tenant_id IS NULL AND e.event_id <> $1 AND v.decoration_areas ?| $2::text[]
			AND v.start_at < $4 AND v.end_at > $3
			AND EXISTS (SELECT 1 FROM game.event_runs r WHERE r.event_id = e.event_id AND r.deleted_at IS NULL
				AND r.status IN ('scheduled', 'live', 'suspended'))`, event, areas, from, to)
	return ids(rows, err)
}

// OnlineStudents implements write.LiveOpsStore.
func (Store) OnlineStudents(ctx context.Context, tenant uuid.UUID, schools []uuid.UUID, grades []int, after uuid.UUID, limit int) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	if schools == nil {
		schools = []uuid.UUID{}
	}
	if grades == nil {
		grades = []int{}
	}
	// tenancyguard:ok game sessions (explicit tenant_id + deleted_at) joined to the projection ref.students (RLS + tenant_id)
	rows, err := t.QueryContext(ctx, `SELECT DISTINCT s.student_id FROM game.game_sessions s
		JOIN ref.students st ON st.student_id = s.student_id AND st.tenant_id = s.tenant_id AND st.deleted_at IS NULL
		WHERE s.tenant_id = $1 AND s.ended_at IS NULL AND s.deleted_at IS NULL
			AND (cardinality($2::uuid[]) = 0 OR st.school_id = ANY($2::uuid[])) AND (cardinality($3::int[]) = 0 OR st.grade_level = ANY($3::int[]))
			AND s.student_id > $4
		ORDER BY s.student_id LIMIT $5`, tenant, schools, grades, after, limit)
	return ids(rows, err)
}

// RunStats implements port.LiveOpsReader (platform scope: every tenant, counts only).
func (Store) RunStats(ctx context.Context, run uuid.UUID) (port.RunStats, error) {
	st := port.RunStats{MilestoneClaims: map[string]int{}}
	t, err := tx(ctx)
	if err != nil {
		return st, err
	}
	// tenancyguard:ok platform read (getEventRunStats, game.liveops.read): aggregates over every tenant, no student id
	if err := t.QueryRowContext(ctx, `SELECT count(*), coalesce(sum(jsonb_array_length(claimed_milestones) + jsonb_array_length(auto_granted_milestones)), 0)
		FROM game.event_participation WHERE event_run_id = $1 AND deleted_at IS NULL`, run).Scan(&st.Participants, &st.RewardsGranted); err != nil {
		return st, fmt.Errorf("run stats: %w", err)
	}
	// tenancyguard:ok platform read (aggregates over every tenant)
	rows, err := t.QueryContext(ctx, `SELECT m, count(*) FROM game.event_participation p,
		jsonb_array_elements_text(p.claimed_milestones || p.auto_granted_milestones) m
		WHERE p.event_run_id = $1 AND p.deleted_at IS NULL GROUP BY m ORDER BY m`, run)
	if err != nil {
		return st, fmt.Errorf("run milestone stats: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var m string
		var n int
		if err := rows.Scan(&m, &n); err != nil {
			return st, err
		}
		st.MilestoneClaims[m] = n
	}
	return st, rows.Err()
}

type schoolEventItem struct {
	row port.SchoolEventRow
	run uuid.NullUUID
}

// scanSchoolEvents reads a page of school events (the runs are loaded after the rows are closed).
func scanSchoolEvents(rows *sql.Rows) ([]schoolEventItem, error) {
	defer func() { _ = rows.Close() }()
	var items []schoolEventItem
	for rows.Next() {
		var it schoolEventItem
		var school, by uuid.NullUUID
		var params []byte
		if err := rows.Scan(&it.row.EventID, &it.row.TenantID, &school, &it.row.TemplateID, &it.row.Version, &params, &by, &it.row.CreatedAt,
			&it.run); err != nil {
			return nil, err
		}
		if school.Valid {
			v := school.UUID
			it.row.SchoolID = &v
		}
		if by.Valid {
			v := by.UUID
			it.row.CreatedBy = &v
		}
		it.row.CreatedAt = it.row.CreatedAt.UTC()
		_ = json.Unmarshal(params, &it.row.Params)
		items = append(items, it)
	}
	return items, rows.Err()
}
