// Package progressionsql reads the published reward rules and the progression curve of module game (F074, G tables) on
// the transaction of ctx (consumer transaction or read session). Explicit SQL; platform content carries its deleted_at
// condition (AGENTS R27).
package progressionsql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/progression"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/reward"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// Content implements port.RewardRules and port.Curves.
type Content struct{}

var (
	_ port.RewardRules = Content{}
	_ port.Curves      = Content{}
)

func tx(ctx context.Context) (*sql.Tx, error) {
	t, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	return t, nil
}

// RulesFor implements port.RewardRules: per live rule (pointer set), the latest published or archived version in effect
// at at — effective_from ≤ at — unless that version ended (active_to ≤ at). A version published later than the event
// never applies to it, and XP already granted is never revisited (F074-AC-24).
func (Content) RulesFor(ctx context.Context, eventType string, at time.Time) ([]reward.Rule, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered
	rows, err := t.QueryContext(ctx, `SELECT DISTINCT ON (v.reward_rule_id) v.reward_rule_id, v.reward_rule_version_id, v.version_no,
			v.active_to, v.payload
		FROM gamification.reward_rule_versions v
		JOIN gamification.reward_rules r ON r.reward_rule_id = v.reward_rule_id AND r.deleted_at IS NULL AND r.current_version_id IS NOT NULL
		WHERE v.trigger_event_type = $1 AND v.status IN ('published', 'archived') AND v.deleted_at IS NULL AND v.effective_from <= $2
		ORDER BY v.reward_rule_id, v.version_no DESC`, eventType, at.UTC())
	if err != nil {
		return nil, fmt.Errorf("reward rules: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []reward.Rule
	for rows.Next() {
		var id, vid uuid.UUID
		var no int
		var to sql.NullTime
		var raw []byte
		if err := rows.Scan(&id, &vid, &no, &to, &raw); err != nil {
			return nil, err
		}
		if to.Valid && !at.Before(to.Time) {
			continue
		}
		r, err := reward.ParseRule(raw)
		if err != nil {
			return nil, fmt.Errorf("reward rule %s v%d: %w", id, no, err)
		}
		r.RuleID, r.VersionID, r.Version = id, vid, no
		out = append(out, r)
	}
	return out, rows.Err()
}

// Current implements port.Curves.
func (Content) Current(ctx context.Context) (*port.CurveRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var r port.CurveRow
	var raw []byte
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered. Release 1.0 has one curve.
	err = t.QueryRowContext(ctx, `SELECT c.curve_id, v.curve_version_id, v.version_no, v.payload FROM game.progression_curves c
		JOIN game.progression_curve_versions v ON v.curve_version_id = c.current_version_id AND v.deleted_at IS NULL
		WHERE c.deleted_at IS NULL ORDER BY c.created_at LIMIT 1`).Scan(&r.CurveID, &r.VersionID, &r.Version, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("progression curve: %w", err)
	}
	if r.Curve, err = progression.ParseCurve(raw); err != nil {
		return nil, fmt.Errorf("progression curve payload: %w", err)
	}
	return &r, nil
}
