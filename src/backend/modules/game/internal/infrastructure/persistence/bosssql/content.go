// Package bosssql reads the published bosses (G tables), their tenant assignment (infra.tenant_resource_assignments),
// and reads and writes the boss encounters, members, contributions, plans and answers (T tables) on the transaction of
// ctx — F103. Explicit SQL: the tenant tables carry their tenant_id and deleted_at conditions (AGENTS R27, RLS as
// well), platform content its deleted_at condition.
package bosssql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/boss"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// Querier is the part of *sql.Tx the readers need.
type Querier interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
	ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error)
}

func q(ctx context.Context) (Querier, error) {
	t, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	return t, nil
}

// Content implements port.BossContent on the transaction of ctx (read session or write transaction).
type Content struct{}

var _ port.BossContent = Content{}

const versionCols = `v.boss_id, v.boss_version_id, v.version_no, v.payload`

func scanVersions(rows *sql.Rows) ([]port.BossRow, error) {
	defer func() { _ = rows.Close() }()
	var out []port.BossRow
	for rows.Next() {
		var r port.BossRow
		var raw []byte
		if err := rows.Scan(&r.BossID, &r.VersionID, &r.VersionNo, &raw); err != nil {
			return nil, err
		}
		v, err := boss.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("boss payload: %w", err)
		}
		r.Version = v
		out = append(out, r)
	}
	return out, rows.Err()
}

func one(rows []port.BossRow, err error) (*port.BossRow, error) {
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

// AvailableBosses implements port.BossContent: the keyset page (boss_id) of the current versions assigned to the tenant
// and serving the grade; LIMIT in the database.
func (Content) AvailableBosses(ctx context.Context, f port.BossFilter) ([]port.BossRow, error) {
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform bosses (G tables) joined with the active assignments of the session tenant ($1, explicit; infra without RLS)
	rows, err := t.QueryContext(ctx, `SELECT `+versionCols+` FROM game.boss_definitions d
		JOIN game.boss_versions v ON v.boss_version_id = d.current_version_id
		JOIN infra.tenant_resource_assignments a ON a.resource_type = 'boss' AND a.resource_id = d.boss_id AND a.tenant_id = $1
			AND a.deleted_at IS NULL
		WHERE d.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published' AND v.grade_levels @> jsonb_build_array($2::int)
			AND ($3::text = '' OR v.area_id = $3) AND ($4::uuid IS NULL OR d.boss_id > $4)
		ORDER BY d.boss_id LIMIT $5`, f.Tenant, f.Grade, f.AreaID, f.After, f.Limit)
	if err != nil {
		return nil, fmt.Errorf("available bosses: %w", err)
	}
	return scanVersions(rows)
}

// PartyBosses implements port.BossContent.
func (Content) PartyBosses(ctx context.Context, tenant uuid.UUID, areaID string, limit int) ([]port.BossRow, error) {
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform bosses (G tables) joined with the active assignments of the session tenant ($1, explicit; infra without RLS)
	rows, err := t.QueryContext(ctx, `SELECT `+versionCols+` FROM game.boss_definitions d
		JOIN game.boss_versions v ON v.boss_version_id = d.current_version_id
		JOIN infra.tenant_resource_assignments a ON a.resource_type = 'boss' AND a.resource_id = d.boss_id AND a.tenant_id = $1
			AND a.deleted_at IS NULL
		WHERE d.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published' AND v.mode <> 'solo'
			AND ($2::text = '' OR v.area_id = $2)
		ORDER BY v.code LIMIT $3`, tenant, areaID, limit)
	if err != nil {
		return nil, fmt.Errorf("party bosses: %w", err)
	}
	return scanVersions(rows)
}

// BossAssigned implements port.BossContent.
func (Content) BossAssigned(ctx context.Context, tenant, bossID uuid.UUID) (bool, error) {
	t, err := q(ctx)
	if err != nil {
		return false, err
	}
	var ok bool
	// tenancyguard:ok infra.tenant_resource_assignments (no RLS, ADR-0089 §4): explicit tenant_id + deleted_at IS NULL
	err = t.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM infra.tenant_resource_assignments
		WHERE resource_type = 'boss' AND resource_id = $2 AND tenant_id = $1 AND deleted_at IS NULL)`, tenant, bossID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("boss assignment: %w", err)
	}
	return ok, nil
}

// CurrentBoss implements port.BossContent.
func (Content) CurrentBoss(ctx context.Context, bossID uuid.UUID) (*port.BossRow, error) {
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on both tables
	rows, err := t.QueryContext(ctx, `SELECT `+versionCols+` FROM game.boss_definitions d
		JOIN game.boss_versions v ON v.boss_version_id = d.current_version_id
		WHERE d.boss_id = $1 AND d.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published'`, bossID)
	if err != nil {
		return nil, fmt.Errorf("boss: %w", err)
	}
	return one(scanVersions(rows))
}

// BossByCode implements port.BossContent.
func (Content) BossByCode(ctx context.Context, code string) (*port.BossRow, error) {
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on both tables
	rows, err := t.QueryContext(ctx, `SELECT `+versionCols+` FROM game.boss_definitions d
		JOIN game.boss_versions v ON v.boss_version_id = d.current_version_id
		WHERE d.code = $1 AND d.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published'`, code)
	if err != nil {
		return nil, fmt.Errorf("boss by code: %w", err)
	}
	return one(scanVersions(rows))
}

// BossVersion implements port.BossContent (a pinned version whatever its status: never deleted, ADR-0043).
func (Content) BossVersion(ctx context.Context, versionID uuid.UUID) (*port.BossRow, error) {
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	rows, err := t.QueryContext(ctx, `SELECT `+versionCols+` FROM game.boss_versions v WHERE v.boss_version_id = $1 AND v.deleted_at IS NULL`,
		versionID)
	if err != nil {
		return nil, fmt.Errorf("boss version: %w", err)
	}
	return one(scanVersions(rows))
}

// KnownBosses implements port.BossContent.
func (Content) KnownBosses(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	out := map[uuid.UUID]bool{}
	if len(ids) == 0 {
		return out, nil
	}
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]string, len(ids))
	for i, id := range ids {
		list[i] = id.String()
	}
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered; published pointer only
	rows, err := t.QueryContext(ctx, `SELECT boss_id FROM game.boss_definitions WHERE boss_id::text = ANY($1)
		AND current_version_id IS NOT NULL AND deleted_at IS NULL`, list)
	if err != nil {
		return nil, fmt.Errorf("known bosses: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}
