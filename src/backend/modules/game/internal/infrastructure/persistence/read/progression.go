package read

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
)

// Progression implements read.ProgressionReadModel (F074): explicit tenant_id on every tenant table (R27).
type Progression struct{}

var _ read.ProgressionReadModel = Progression{}

// Profile implements read.ProgressionReadModel.
func (Progression) Profile(ctx context.Context, tenant, student uuid.UUID) (*read.ProgressionProfile, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var p read.ProgressionProfile
	var cv sql.NullInt64
	// tenancyguard:ok tenant table game.game_profiles: explicit tenant_id + deleted_at IS NULL (RLS as well)
	err = t.QueryRowContext(ctx, `SELECT level, xp_total, curve_version FROM game.game_profiles
		WHERE tenant_id = $1 AND student_id = $2 AND deleted_at IS NULL`, tenant, student).Scan(&p.Level, &p.XPTotal, &cv)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("progression profile: %w", err)
	}
	if cv.Valid {
		v := int(cv.Int64)
		p.CurveVersion = &v
	}
	return &p, nil
}

// DailyGranted implements read.ProgressionReadModel.
func (Progression) DailyGranted(ctx context.Context, tenant, student uuid.UUID, day, bucket string) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	// tenancyguard:ok tenant table gamification.xp_daily_caps: explicit tenant_id + deleted_at IS NULL (RLS as well)
	err = t.QueryRowContext(ctx, `SELECT coalesce(sum(granted), 0) FROM gamification.xp_daily_caps
		WHERE tenant_id = $1 AND student_id = $2 AND cap_date = $3::date AND bucket = $4 AND deleted_at IS NULL`, tenant, student, day, bucket).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("xp daily caps: %w", err)
	}
	return n, nil
}

// XPHistory implements read.ProgressionReadModel.
func (Progression) XPHistory(ctx context.Context, tenant, student uuid.UUID, after *read.XPHistoryCursor, limit int) ([]read.XPHistoryRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var rows *sql.Rows
	if after == nil {
		// tenancyguard:ok append-only tenant table gamification.xp_ledger (no deleted_at): explicit tenant_id (RLS as well)
		rows, err = t.QueryContext(ctx, `SELECT xp_entry_id, amount, source_type, source_code, capped, created_at FROM gamification.xp_ledger
			WHERE tenant_id = $1 AND student_id = $2 ORDER BY created_at DESC, xp_entry_id DESC LIMIT $3`, tenant, student, limit)
	} else {
		// tenancyguard:ok append-only tenant table gamification.xp_ledger (no deleted_at): explicit tenant_id (RLS as well)
		rows, err = t.QueryContext(ctx, `SELECT xp_entry_id, amount, source_type, source_code, capped, created_at FROM gamification.xp_ledger
			WHERE tenant_id = $1 AND student_id = $2 AND (created_at, xp_entry_id) < ($3, $4)
			ORDER BY created_at DESC, xp_entry_id DESC LIMIT $5`, tenant, student, after.At, after.ID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("xp history: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.XPHistoryRow
	for rows.Next() {
		var r read.XPHistoryRow
		if err := rows.Scan(&r.ID, &r.Amount, &r.SourceType, &r.SourceCode, &r.Capped, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.CreatedAt = r.CreatedAt.UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

// Levels implements read.ProgressionReadModel.
func (Progression) Levels(ctx context.Context, curveVersionID uuid.UUID) ([]read.ProgressionLevel, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (GA table game.progression_levels, no tenant_id, immutable rows)
	rows, err := t.QueryContext(ctx, `SELECT level, xp_required, xp_cumulative, title_code, rewards FROM game.progression_levels
		WHERE curve_version_id = $1 ORDER BY level`, curveVersionID)
	if err != nil {
		return nil, fmt.Errorf("progression levels: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.ProgressionLevel
	for rows.Next() {
		var l read.ProgressionLevel
		var req sql.NullInt64
		var raw []byte
		if err := rows.Scan(&l.Level, &req, &l.Cumulative, &l.TitleCode, &raw); err != nil {
			return nil, err
		}
		if req.Valid {
			v := req.Int64
			l.XPRequired = &v
		}
		if err := json.Unmarshal(raw, &l.Rewards); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
