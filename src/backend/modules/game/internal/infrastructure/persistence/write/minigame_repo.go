package write

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
)

// MiniGamePlayRepo implements write.MiniGamePlayStore on game.minigame_plays (T, RLS; F073).
type MiniGamePlayRepo struct{}

var _ write.MiniGamePlayStore = MiniGamePlayRepo{}

// LockStudent implements write.MiniGamePlayStore.
func (MiniGamePlayRepo) LockStudent(ctx context.Context, student uuid.UUID) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok advisory lock only (no table read)
	if _, err := t.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('game.minigame:' || $1::text, 0))`, student); err != nil {
		return fmt.Errorf("minigame: lock student: %w", err)
	}
	return nil
}

// InsertPlay implements write.MiniGamePlayStore.
func (MiniGamePlayRepo) InsertPlay(ctx context.Context, p write.MiniGamePlay) (bool, error) {
	// tenancyguard:ok tenant table game.minigame_plays: explicit tenant_id (RLS insert policy); unique attempt among live rows
	ok, err := exec(ctx, `INSERT INTO game.minigame_plays (play_id, tenant_id, created_at, created_by, updated_at, student_id, school_id,
			minigame_config_id, minigame_config_version_id, minigame_type, attempt_id, context_type, context_id, play_date, status, started_at)
		VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $3)
		ON CONFLICT (tenant_id, attempt_id) WHERE deleted_at IS NULL DO NOTHING`,
		p.ID, p.TenantID, p.StartedAt, actor(ctx), p.StudentID, p.SchoolID, p.ConfigID, p.ConfigVersionID, p.MiniGameType, p.AttemptID,
		p.ContextType, p.ContextID, p.PlayDate, p.Status)
	if err != nil {
		return false, fmt.Errorf("minigame plays: insert: %w", err)
	}
	return ok, nil
}

// PlayByAttempt implements write.MiniGamePlayStore (row locked for the result).
func (MiniGamePlayRepo) PlayByAttempt(ctx context.Context, tenant, attempt uuid.UUID) (*write.MiniGamePlay, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var p write.MiniGamePlay
	var school, cid uuid.NullUUID
	var ls, ms sql.NullFloat64
	var gs, cc, ic sql.NullInt64
	var cm, re sql.NullBool
	var done sql.NullTime
	// tenancyguard:ok tenant table game.minigame_plays: tenant_id and deleted_at conditions (RLS read policy)
	err = t.QueryRowContext(ctx, `SELECT play_id, tenant_id, student_id, school_id, minigame_config_id, minigame_config_version_id, minigame_type,
			attempt_id, context_type, context_id, play_date, status, learning_score, max_score, game_score, correct_count, item_count,
			completion_met, rewards_eligible, started_at, completed_at
		FROM game.minigame_plays WHERE tenant_id = $1 AND attempt_id = $2 AND deleted_at IS NULL FOR UPDATE`, tenant, attempt).
		Scan(&p.ID, &p.TenantID, &p.StudentID, &school, &p.ConfigID, &p.ConfigVersionID, &p.MiniGameType, &p.AttemptID, &p.ContextType, &cid,
			&p.PlayDate, &p.Status, &ls, &ms, &gs, &cc, &ic, &cm, &re, &p.StartedAt, &done)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("minigame plays: %w", err)
	}
	if school.Valid {
		p.SchoolID = &school.UUID
	}
	if cid.Valid {
		p.ContextID = &cid.UUID
	}
	if ls.Valid {
		p.LearningScore = &ls.Float64
	}
	if ms.Valid {
		p.MaxScore = &ms.Float64
	}
	for _, x := range []struct {
		n   sql.NullInt64
		dst **int
	}{{gs, &p.GameScore}, {cc, &p.CorrectCount}, {ic, &p.ItemCount}} {
		if x.n.Valid {
			v := int(x.n.Int64)
			*x.dst = &v
		}
	}
	if cm.Valid {
		p.CompletionMet = &cm.Bool
	}
	if re.Valid {
		p.RewardsEligible = &re.Bool
	}
	if done.Valid {
		p.CompletedAt = &done.Time
	}
	return &p, nil
}

// CompletePlay implements write.MiniGamePlayStore.
func (MiniGamePlayRepo) CompletePlay(ctx context.Context, p write.MiniGamePlay) error {
	// tenancyguard:ok tenant table game.minigame_plays: explicit tenant_id (RLS update policy); live rows only
	_, err := exec(ctx, `UPDATE game.minigame_plays SET status = $3, learning_score = $4, max_score = $5, game_score = $6, correct_count = $7,
			item_count = $8, completion_met = $9, rewards_eligible = $10, completed_at = $11, updated_at = $11, version = version + 1
		WHERE tenant_id = $1 AND play_id = $2 AND deleted_at IS NULL`,
		p.TenantID, p.ID, p.Status, p.LearningScore, p.MaxScore, p.GameScore, p.CorrectCount, p.ItemCount, p.CompletionMet, p.RewardsEligible,
		p.CompletedAt)
	if err != nil {
		return fmt.Errorf("minigame plays: complete: %w", err)
	}
	return nil
}

// CountCompletedOn implements write.MiniGamePlayStore.
func (MiniGamePlayRepo) CountCompletedOn(ctx context.Context, tenant, student, config uuid.UUID, day string) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	// tenancyguard:ok tenant table game.minigame_plays: tenant_id and deleted_at conditions (RLS read policy)
	err = t.QueryRowContext(ctx, `SELECT count(*) FROM game.minigame_plays WHERE tenant_id = $1 AND student_id = $2
		AND minigame_config_id = $3 AND play_date = $4 AND status = 'completed' AND deleted_at IS NULL`, tenant, student, config, day).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("minigame plays: count: %w", err)
	}
	return n, nil
}
