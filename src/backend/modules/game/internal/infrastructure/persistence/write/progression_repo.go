package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// ProgressionRepo implements write.ProgressionStore and write.ProgressionScan on gamification.xp_ledger,
// gamification.xp_daily_caps, gamification.reward_grants, game.level_history and the progression columns of
// game.game_profiles (F074). The ledger, the level history and the grants are only ever inserted (R21).
type ProgressionRepo struct{}

var (
	_ write.ProgressionStore = ProgressionRepo{}
	_ write.ProgressionScan  = ProgressionRepo{}
)

// LockStudent implements write.ProgressionStore.
func (ProgressionRepo) LockStudent(ctx context.Context, student uuid.UUID) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok advisory lock only (no table read)
	if _, err := t.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('game.xp:' || $1::text, 0))`, student); err != nil {
		return fmt.Errorf("progression: lock student: %w", err)
	}
	return nil
}

// Progress implements write.ProgressionStore.
func (ProgressionRepo) Progress(ctx context.Context, tenant, student uuid.UUID) (*write.ProgressState, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var p write.ProgressState
	var cv sql.NullInt64
	var title, year sql.NullString
	// tenancyguard:ok tenant table game.game_profiles: explicit tenant_id + deleted_at IS NULL (RLS as well)
	err = t.QueryRowContext(ctx, `SELECT school_id, level, xp_total, curve_version, title_code, grade_transition_year FROM game.game_profiles
		WHERE tenant_id = $1 AND student_id = $2 AND deleted_at IS NULL FOR UPDATE`, tenant, student).
		Scan(&p.SchoolID, &p.Level, &p.XPTotal, &cv, &title, &year)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("progression: profile: %w", err)
	}
	if cv.Valid {
		v := int(cv.Int64)
		p.CurveVersion = &v
	}
	if title.Valid {
		p.TitleCode = &title.String
	}
	if year.Valid {
		p.GradeYear = &year.String
	}
	return &p, nil
}

// SaveProgress implements write.ProgressionStore.
func (ProgressionRepo) SaveProgress(ctx context.Context, tenant, student uuid.UUID, p write.ProgressState) error {
	// tenancyguard:ok tenant table game.game_profiles: explicit tenant_id + deleted_at IS NULL (RLS as well)
	_, err := exec(ctx, `UPDATE game.game_profiles SET level = $3, xp_total = $4, curve_version = $5, title_code = $6, updated_at = $7
		WHERE tenant_id = $1 AND student_id = $2 AND deleted_at IS NULL`, tenant, student, p.Level, max(0, p.XPTotal),
		p.CurveVersion, p.TitleCode, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("progression: save profile: %w", err)
	}
	return nil
}

// SetGradeYear implements write.ProgressionStore.
func (ProgressionRepo) SetGradeYear(ctx context.Context, tenant, student uuid.UUID, year string) error {
	// tenancyguard:ok tenant table game.game_profiles: explicit tenant_id + deleted_at IS NULL (RLS as well)
	_, err := exec(ctx, `UPDATE game.game_profiles SET grade_transition_year = $3, updated_at = $4
		WHERE tenant_id = $1 AND student_id = $2 AND deleted_at IS NULL`, tenant, student, year, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("progression: grade year: %w", err)
	}
	return nil
}

// LockCaps implements write.ProgressionStore: the counters of the day are created when missing, then locked FOR UPDATE
// so two awards of one student near the cap never both pass (edge case of the spec).
func (ProgressionRepo) LockCaps(ctx context.Context, tenant, student uuid.UUID, day string, buckets []string) (map[string]int, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for _, b := range buckets {
		// tenancyguard:ok insert under the TenantScope (RLS WITH CHECK); the conflict target is the live-row index
		if _, err := t.ExecContext(ctx, `INSERT INTO gamification.xp_daily_caps (xp_daily_cap_id, tenant_id, created_at, updated_at, student_id,
				cap_date, bucket, granted) VALUES ($1, $2, $3, $3, $4, $5::date, $6, 0)
			ON CONFLICT (tenant_id, student_id, cap_date, bucket) WHERE deleted_at IS NULL DO NOTHING`, ids.New(), tenant, now, student, day, b); err != nil {
			return nil, fmt.Errorf("progression: cap counter: %w", err)
		}
	}
	// tenancyguard:ok tenant table gamification.xp_daily_caps: explicit tenant_id + deleted_at IS NULL (RLS as well)
	rows, err := t.QueryContext(ctx, `SELECT bucket, granted FROM gamification.xp_daily_caps
		WHERE tenant_id = $1 AND student_id = $2 AND cap_date = $3::date AND bucket = ANY($4) AND deleted_at IS NULL FOR UPDATE`,
		tenant, student, day, buckets)
	if err != nil {
		return nil, fmt.Errorf("progression: lock caps: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]int{}
	for rows.Next() {
		var b string
		var n int
		if err := rows.Scan(&b, &n); err != nil {
			return nil, err
		}
		out[b] = n
	}
	return out, rows.Err()
}

// AddCap implements write.ProgressionStore.
func (ProgressionRepo) AddCap(ctx context.Context, tenant, student uuid.UUID, day, bucket string, n int) error {
	// tenancyguard:ok tenant table gamification.xp_daily_caps: explicit tenant_id + deleted_at IS NULL (RLS as well)
	_, err := exec(ctx, `UPDATE gamification.xp_daily_caps SET granted = granted + $5, updated_at = $6, version = version + 1
		WHERE tenant_id = $1 AND student_id = $2 AND cap_date = $3::date AND bucket = $4 AND deleted_at IS NULL`,
		tenant, student, day, bucket, n, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("progression: add cap: %w", err)
	}
	return nil
}

// Repeats implements write.ProgressionStore.
func (ProgressionRepo) Repeats(ctx context.Context, tenant, student uuid.UUID, k write.RepeatKey) (int, int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, 0, err
	}
	var in, ever int
	// tenancyguard:ok append-only tenant table gamification.xp_ledger (no deleted_at): explicit tenant_id (RLS as well)
	err = t.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE created_at >= $5), count(*) FROM gamification.xp_ledger
		WHERE tenant_id = $1 AND student_id = $2 AND content_version_id = $3 AND source_code = $4 AND reason_code = 'reward'`,
		tenant, student, k.ContentVersionID, k.SourceCode, k.Since).Scan(&in, &ever)
	if err != nil {
		return 0, 0, fmt.Errorf("progression: repeats: %w", err)
	}
	return in, ever, nil
}

// InsertEntry implements write.ProgressionStore.
func (ProgressionRepo) InsertEntry(ctx context.Context, e write.XpEntry) (bool, error) {
	// tenancyguard:ok append-only insert under the TenantScope (RLS WITH CHECK); both unique keys make a replay a no-op
	ok, err := exec(ctx, `INSERT INTO gamification.xp_ledger (xp_entry_id, tenant_id, created_at, created_by, student_id, school_id, amount,
			reason_code, source_type, source_code, source_id, content_version_id, rule_id, rule_version, capped, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16) ON CONFLICT DO NOTHING`,
		e.ID, e.TenantID, e.At, actor(ctx), e.StudentID, e.SchoolID, e.Amount, e.ReasonCode, e.SourceType, e.SourceCode, e.SourceID,
		e.ContentVersionID, e.RuleID, e.RuleVersion, e.Capped, e.IdempotencyKey)
	if err != nil {
		return false, fmt.Errorf("progression: ledger: %w", err)
	}
	return ok, nil
}

// LedgerTotal implements write.ProgressionStore.
func (ProgressionRepo) LedgerTotal(ctx context.Context, tenant, student uuid.UUID) (int64, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var n int64
	// tenancyguard:ok append-only tenant table gamification.xp_ledger (no deleted_at): explicit tenant_id (RLS as well)
	if err := t.QueryRowContext(ctx, `SELECT coalesce(sum(amount), 0) FROM gamification.xp_ledger WHERE tenant_id = $1 AND student_id = $2`,
		tenant, student).Scan(&n); err != nil {
		return 0, fmt.Errorf("progression: ledger total: %w", err)
	}
	return n, nil
}

// InsertLevel implements write.ProgressionStore.
func (ProgressionRepo) InsertLevel(ctx context.Context, l write.LevelReached) error {
	// tenancyguard:ok append-only insert under the TenantScope (RLS WITH CHECK)
	_, err := exec(ctx, `INSERT INTO game.level_history (level_history_id, tenant_id, created_at, created_by, student_id, from_level, to_level,
			curve_id, curve_version, trigger_xp_entry_id, reached_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $3)`,
		l.ID, l.TenantID, l.At, actor(ctx), l.StudentID, l.FromLevel, l.ToLevel, l.CurveID, l.CurveVersion, l.TriggerEntry)
	if err != nil {
		return fmt.Errorf("progression: level history: %w", err)
	}
	return nil
}

// InsertGrant implements write.ProgressionStore.
func (ProgressionRepo) InsertGrant(ctx context.Context, g write.RewardGrant) (bool, error) {
	grants := g.Grants
	if grants == nil {
		grants = []map[string]any{}
	}
	raw, err := json.Marshal(grants)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok append-only insert under the TenantScope (RLS WITH CHECK); the idempotency key makes a repeat a no-op
	ok, err := exec(ctx, `INSERT INTO gamification.reward_grants (reward_grant_id, tenant_id, created_at, created_by, student_id, school_id,
			rule_version_id, trigger_event_id, trigger_kind, level, curve_id, grants, idempotency_key, granted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb, $13, $3) ON CONFLICT DO NOTHING`,
		g.ID, g.TenantID, g.At, actor(ctx), g.StudentID, g.SchoolID, g.RuleVersionID, g.TriggerEventID, g.TriggerKind, g.Level, g.CurveID,
		string(raw), g.IdempotencyKey)
	if err != nil {
		return false, fmt.Errorf("progression: reward grant: %w", err)
	}
	return ok, nil
}

// BehindCurve implements write.ProgressionScan.
func (ProgressionRepo) BehindCurve(ctx context.Context, version, limit int) ([]write.ProfileRef, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform job scope: live profiles of every tenant (each is then handled in its tenant); deleted_at filtered
	rows, err := t.QueryContext(ctx, `SELECT tenant_id, student_id FROM game.game_profiles
		WHERE deleted_at IS NULL AND curve_version IS DISTINCT FROM $1 ORDER BY tenant_id, student_id LIMIT $2`, version, limit)
	if err != nil {
		return nil, fmt.Errorf("progression: behind curve: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.ProfileRef
	for rows.Next() {
		var r write.ProfileRef
		if err := rows.Scan(&r.TenantID, &r.StudentID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Mismatched implements write.ProgressionScan.
func (ProgressionRepo) Mismatched(ctx context.Context, after *write.ProfileRef, limit int) ([]write.ProfileMismatch, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	at, as := uuid.Nil, uuid.Nil
	if after != nil {
		at, as = after.TenantID, after.StudentID
	}
	// tenancyguard:ok platform job scope: compares every live profile with its ledger (read only); deleted_at filtered
	rows, err := t.QueryContext(ctx, `SELECT p.tenant_id, p.student_id, p.xp_total, coalesce(l.total, 0) FROM game.game_profiles p
		LEFT JOIN LATERAL (SELECT sum(amount) AS total FROM gamification.xp_ledger x WHERE x.tenant_id = p.tenant_id AND x.student_id = p.student_id) l
			ON true
		WHERE p.deleted_at IS NULL AND (p.tenant_id, p.student_id) > ($1, $2) AND p.xp_total <> coalesce(l.total, 0)
		ORDER BY p.tenant_id, p.student_id LIMIT $3`, at, as, limit)
	if err != nil {
		return nil, fmt.Errorf("progression: mismatched: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.ProfileMismatch
	for rows.Next() {
		var m write.ProfileMismatch
		if err := rows.Scan(&m.TenantID, &m.StudentID, &m.Cached, &m.Ledger); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
