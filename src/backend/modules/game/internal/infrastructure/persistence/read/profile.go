package read

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/catalogsql"
)

// Profiles implements read.ProfileReadModel on game.game_profiles and game.avatar_presets.
type Profiles struct{}

var _ read.ProfileReadModel = Profiles{}

// Profile implements read.ProfileReadModel; only the columns of GameProfile are read (no school, no class, R16).
func (Profiles) Profile(ctx context.Context, tenant, student uuid.UUID) (*read.Profile, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var p read.Profile
	var avatar, settings []byte
	var nick, hash, area, lang sql.NullString
	var created sql.NullTime
	// tenancyguard:ok TenantScope (RLS) plus the explicit tenant_id; deleted_at filtered
	err = t.QueryRowContext(ctx, `SELECT student_id, nickname, level, xp_total, avatar_config, avatar_config_hash, avatar_created_at,
			last_area_id, settings, preferred_language
		FROM game.game_profiles WHERE tenant_id = $1 AND student_id = $2 AND deleted_at IS NULL`, tenant, student).
		Scan(&p.StudentID, &nick, &p.Level, &p.XPTotal, &avatar, &hash, &created, &area, &settings, &lang)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("game profile: %w", err)
	}
	str := func(n sql.NullString) *string {
		if !n.Valid {
			return nil
		}
		s := n.String
		return &s
	}
	p.Nickname, p.AvatarConfigHash, p.LastAreaID, p.PreferredLanguage = str(nick), str(hash), str(area), str(lang)
	if created.Valid {
		c := created.Time.UTC()
		p.AvatarCreatedAt = &c
	}
	if len(avatar) > 0 && string(avatar) != "null" {
		p.AvatarConfig = avatar
	}
	p.Settings = settings
	return &p, nil
}

// NicknameAllowance implements read.ProfileReadModel.
func (Profiles) NicknameAllowance(ctx context.Context, tenant, student uuid.UUID, since time.Time) (int, *int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, nil, err
	}
	var n int
	var limit sql.NullInt64
	// tenancyguard:ok TenantScope (RLS) plus the explicit tenant_id; game_profiles deleted_at filtered; nickname_changes is
	// append-only (no deleted_at); ref.schools is a projection without deleted_at
	err = t.QueryRowContext(ctx, `SELECT
			(SELECT count(*) FROM game.nickname_changes c WHERE c.tenant_id = $1 AND c.student_id = $2 AND c.reason = 'player_change'
				AND c.changed_at >= $3),
			(SELECT CASE WHEN jsonb_typeof(s.settings_subset->'nickname_change_monthly_limit') = 'number'
				THEN (s.settings_subset->>'nickname_change_monthly_limit')::numeric::bigint END
			 FROM game.game_profiles g JOIN ref.schools s ON s.tenant_id = g.tenant_id AND s.school_id = g.school_id
			 WHERE g.tenant_id = $1 AND g.student_id = $2 AND g.deleted_at IS NULL)`, tenant, student, since.UTC()).Scan(&n, &limit)
	if err != nil {
		return 0, nil, fmt.Errorf("game profile: nickname allowance: %w", err)
	}
	if !limit.Valid {
		return n, nil, nil
	}
	l := int(limit.Int64)
	return n, &l, nil
}

// Presets implements read.ProfileReadModel.
func (Profiles) Presets(ctx context.Context, tenant, student uuid.UUID) ([]read.Preset, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok TenantScope (RLS) plus the explicit tenant_id; deleted_at filtered
	rows, err := t.QueryContext(ctx, `SELECT slot, name, avatar_config, updated_at FROM game.avatar_presets
		WHERE tenant_id = $1 AND student_id = $2 AND deleted_at IS NULL ORDER BY slot`, tenant, student)
	if err != nil {
		return nil, fmt.Errorf("avatar presets: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []read.Preset{}
	for rows.Next() {
		var p read.Preset
		var name sql.NullString
		if err := rows.Scan(&p.Slot, &name, &p.AvatarConfig, &p.UpdatedAt); err != nil {
			return nil, err
		}
		if name.Valid {
			n := name.String
			p.Name = &n
		}
		p.UpdatedAt = p.UpdatedAt.UTC()
		out = append(out, p)
	}
	return out, rows.Err()
}

// TakenNicknames implements read.ProfileReadModel.
func (Profiles) TakenNicknames(ctx context.Context, tenant uuid.UUID, candidates []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(candidates) == 0 {
		return out, nil
	}
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	lower := make([]string, len(candidates))
	for i, c := range candidates {
		lower[i] = strings.ToLower(c)
	}
	// tenancyguard:ok TenantScope (RLS) plus the explicit tenant_id; deleted_at filtered (index game_profiles_nickname_uq)
	rows, err := t.QueryContext(ctx, `SELECT lower(nickname) FROM game.game_profiles
		WHERE tenant_id = $1 AND deleted_at IS NULL AND nickname IS NOT NULL AND lower(nickname) = ANY($2::text[])`, tenant, lower)
	if err != nil {
		return nil, fmt.Errorf("game profile: taken nicknames: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out[n] = true
	}
	return out, rows.Err()
}

// Catalogs implements read.CatalogReadModel.
type Catalogs struct{}

var _ read.CatalogReadModel = Catalogs{}

// Current implements read.CatalogReadModel.
func (Catalogs) Current(ctx context.Context) (*catalog.Catalog, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	return catalogsql.Current(ctx, t, catalog.Code)
}
