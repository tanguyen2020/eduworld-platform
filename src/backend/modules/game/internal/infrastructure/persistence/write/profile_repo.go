package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/profile"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/catalogsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// ProfileRepo implements write.ProfileRepository on game.game_profiles and game.nickname_changes.
type ProfileRepo struct{}

var _ write.ProfileRepository = ProfileRepo{}

func actor(ctx context.Context) any {
	if p, ok := authz.PrincipalFrom(ctx); ok {
		return p.UserID
	}
	return nil
}

const profileColumns = `game_profile_id, student_id, tenant_id, school_id, nickname, level, xp_total, avatar_config,
	avatar_config_hash, avatar_created_at, last_area_id, settings, preferred_language, last_game_login_at, version`

func scanProfile(row interface{ Scan(...any) error }) (*profile.Profile, error) {
	var p profile.Profile
	var nick, hash, area, lang sql.NullString
	var created, login sql.NullTime
	var avatar, settings []byte
	if err := row.Scan(&p.ID, &p.StudentID, &p.TenantID, &p.SchoolID, &nick, &p.Level, &p.XPTotal, &avatar, &hash, &created,
		&area, &settings, &lang, &login, &p.Version); err != nil {
		return nil, err
	}
	str := func(n sql.NullString) *string {
		if !n.Valid {
			return nil
		}
		s := n.String
		return &s
	}
	p.Nickname, p.AvatarHash, p.LastAreaID, p.PreferredLanguage = str(nick), str(hash), str(area), str(lang)
	if created.Valid {
		t := created.Time.UTC()
		p.AvatarCreatedAt = &t
	}
	if login.Valid {
		t := login.Time.UTC()
		p.LastGameLoginAt = &t
	}
	if len(avatar) > 0 && string(avatar) != "null" {
		var c profile.AvatarConfig
		if err := json.Unmarshal(avatar, &c); err != nil {
			return nil, fmt.Errorf("game profile: avatar_config: %w", err)
		}
		p.Avatar = &c
	}
	p.Settings = map[string]any{}
	if len(settings) > 0 {
		if err := json.Unmarshal(settings, &p.Settings); err != nil || p.Settings == nil {
			p.Settings = map[string]any{}
		}
	}
	return &p, nil
}

// Ensure implements write.ProfileRepository: insert when the student has no live profile in the tenant; with login
// set, record last_game_login_at on the live one.
func (ProfileRepo) Ensure(ctx context.Context, p *profile.Profile, login *time.Time) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	if p.ID == uuid.Nil {
		p.ID = ids.New()
	}
	now := time.Now().UTC()
	// tenancyguard:ok insert under the TenantScope (RLS WITH CHECK on tenant_id); the conflict target is the live-row index
	if _, err := t.ExecContext(ctx, `INSERT INTO game.game_profiles (game_profile_id, tenant_id, student_id, school_id, created_at,
			created_by, updated_at, settings, last_game_login_at)
		VALUES ($1, $2, $3, $4, $5, $6, $5, '{}'::jsonb, $7)
		ON CONFLICT (tenant_id, student_id) WHERE deleted_at IS NULL DO NOTHING`,
		p.ID, p.TenantID, p.StudentID, p.SchoolID, now, actor(ctx), login); err != nil {
		return fmt.Errorf("game profile: ensure: %w", err)
	}
	if login == nil {
		return nil
	}
	// tenancyguard:ok TenantScope (RLS) plus the explicit tenant_id; deleted_at filtered
	if _, err := t.ExecContext(ctx, `UPDATE game.game_profiles SET last_game_login_at = $3
		WHERE tenant_id = $1 AND student_id = $2 AND deleted_at IS NULL AND (last_game_login_at IS NULL OR last_game_login_at < $3)`,
		p.TenantID, p.StudentID, login.UTC()); err != nil {
		return fmt.Errorf("game profile: login time: %w", err)
	}
	return nil
}

// Get implements write.ProfileRepository.
func (ProfileRepo) Get(ctx context.Context, tenant, student uuid.UUID) (*profile.Profile, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok TenantScope (RLS) plus the explicit tenant_id; deleted_at filtered
	p, err := scanProfile(t.QueryRowContext(ctx, `SELECT `+profileColumns+` FROM game.game_profiles
		WHERE tenant_id = $1 AND student_id = $2 AND deleted_at IS NULL FOR UPDATE`, tenant, student))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("game profile: get: %w", err)
	}
	return p, nil
}

func nicknameTaken(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && pe.Code == "23505" && pe.ConstraintName == "game_profiles_nickname_uq"
}

// Save implements write.ProfileRepository.
func (ProfileRepo) Save(ctx context.Context, p *profile.Profile) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	var avatar any
	if p.Avatar != nil {
		raw, err := p.Avatar.Canonical()
		if err != nil {
			return err
		}
		avatar = string(raw)
	}
	settings, err := json.Marshal(p.Settings)
	if err != nil {
		return err
	}
	// tenancyguard:ok TenantScope (RLS) plus the explicit tenant_id; deleted_at filtered; optimistic version
	res, err := t.ExecContext(ctx, `UPDATE game.game_profiles SET nickname = $4, avatar_config = $5::jsonb,
			avatar_config_hash = $6, avatar_created_at = $7, settings = $8::jsonb, preferred_language = $9,
			updated_at = now(), updated_by = $10, version = version + 1
		WHERE tenant_id = $1 AND game_profile_id = $2 AND version = $3 AND deleted_at IS NULL`,
		p.TenantID, p.ID, p.Version, p.Nickname, avatar, p.AvatarHash, p.AvatarCreatedAt, string(settings), p.PreferredLanguage,
		actor(ctx))
	if nicknameTaken(err) {
		return write.ErrNicknameTaken
	}
	if err != nil {
		return fmt.Errorf("game profile: save: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		return write.ErrProfileConflict
	}
	p.Version++
	return nil
}

// AppendNicknameChange implements write.ProfileRepository.
func (ProfileRepo) AppendNicknameChange(ctx context.Context, c write.NicknameChange) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok append-only TA insert under the TenantScope (RLS WITH CHECK on tenant_id)
	if _, err := t.ExecContext(ctx, `INSERT INTO game.nickname_changes (nickname_change_id, tenant_id, created_at, created_by,
			student_id, old_nickname, new_nickname, reason, changed_by, changed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $3)`,
		c.ID, c.TenantID, c.ChangedAt, c.ChangedBy, c.StudentID, c.Old, c.New, c.Reason, c.ChangedBy); err != nil {
		return fmt.Errorf("nickname changes: %w", err)
	}
	return nil
}

// PlayerChangesSince implements write.ProfileRepository.
func (ProfileRepo) PlayerChangesSince(ctx context.Context, tenant, student uuid.UUID, since time.Time) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	// tenancyguard:ok append-only TA table (no deleted_at); TenantScope (RLS) plus the explicit tenant_id
	if err := t.QueryRowContext(ctx, `SELECT count(*) FROM game.nickname_changes
		WHERE tenant_id = $1 AND student_id = $2 AND reason = 'player_change' AND changed_at >= $3`,
		tenant, student, since.UTC()).Scan(&n); err != nil {
		return 0, fmt.Errorf("nickname changes: count: %w", err)
	}
	return n, nil
}

// SoftDelete implements write.ProfileRepository (soft-delete only, ADR-0028).
func (ProfileRepo) SoftDelete(ctx context.Context, tenant, student uuid.UUID, reason string, at time.Time) (bool, error) {
	// tenancyguard:ok soft-delete under the TenantScope (RLS) plus the explicit tenant_id; only live rows
	ok, err := exec(ctx, `UPDATE game.game_profiles SET deleted_at = $3, deleted_by = $4, delete_reason = $5, updated_at = $3,
			version = version + 1
		WHERE tenant_id = $1 AND student_id = $2 AND deleted_at IS NULL`, tenant, student, at.UTC(), actor(ctx), reason)
	if err != nil {
		return false, fmt.Errorf("game profile: soft-delete: %w", err)
	}
	// tenancyguard:ok soft-delete under the TenantScope (RLS) plus the explicit tenant_id; only live rows
	if _, err := exec(ctx, `UPDATE game.avatar_presets SET deleted_at = $3, deleted_by = $4, delete_reason = $5, updated_at = $3,
			version = version + 1
		WHERE tenant_id = $1 AND student_id = $2 AND deleted_at IS NULL`, tenant, student, at.UTC(), actor(ctx), reason); err != nil {
		return false, fmt.Errorf("avatar presets: soft-delete: %w", err)
	}
	return ok, nil
}

// Restore implements write.ProfileRepository: the newest profile soft-deleted with reason comes back unless the student
// already has a live one; its nickname is cleared when another live profile took it meanwhile.
func (ProfileRepo) Restore(ctx context.Context, tenant, student uuid.UUID, reason string) (bool, error) {
	// tenancyguard:ok restore of a soft-deleted row (deleted_at IS NOT NULL on purpose) under the TenantScope (RLS) plus tenant_id
	ok, err := exec(ctx, `UPDATE game.game_profiles g SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL,
			updated_at = now(), version = g.version + 1,
			nickname = CASE WHEN EXISTS (SELECT 1 FROM game.game_profiles o WHERE o.tenant_id = g.tenant_id
				AND o.deleted_at IS NULL AND lower(o.nickname) = lower(g.nickname)) THEN NULL ELSE g.nickname END
		WHERE g.game_profile_id = (SELECT p.game_profile_id FROM game.game_profiles p
			WHERE p.tenant_id = $1 AND p.student_id = $2 AND p.deleted_at IS NOT NULL AND p.delete_reason = $3
			ORDER BY p.deleted_at DESC LIMIT 1)
		AND NOT EXISTS (SELECT 1 FROM game.game_profiles l WHERE l.tenant_id = $1 AND l.student_id = $2 AND l.deleted_at IS NULL)`,
		tenant, student, reason)
	if err != nil {
		return false, fmt.Errorf("game profile: restore: %w", err)
	}
	if !ok {
		return false, nil
	}
	// tenancyguard:ok restore of soft-deleted rows (deleted_at IS NOT NULL on purpose) under the TenantScope (RLS) plus tenant_id
	if _, err := exec(ctx, `UPDATE game.avatar_presets a SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL,
			updated_at = now(), version = a.version + 1
		WHERE a.tenant_id = $1 AND a.student_id = $2 AND a.deleted_at IS NOT NULL AND a.delete_reason = $3
			AND NOT EXISTS (SELECT 1 FROM game.avatar_presets l WHERE l.tenant_id = $1 AND l.student_id = $2 AND l.slot = a.slot
				AND l.deleted_at IS NULL)`, tenant, student, reason); err != nil {
		return false, fmt.Errorf("avatar presets: restore: %w", err)
	}
	return true, nil
}

// Anonymize implements write.ProfileRepository.
func (ProfileRepo) Anonymize(ctx context.Context, tenant, student uuid.UUID, anon string) (bool, error) {
	// Personal data is overwritten wherever it is, live or soft-deleted (ADR-0028 §8).
	// tenancyguard:ok anonymization of every profile row of the student under the TenantScope (RLS) plus tenant_id
	ok, err := exec(ctx, `UPDATE game.game_profiles SET nickname = CASE WHEN nickname IS NULL THEN NULL ELSE $3 END,
			avatar_config = NULL, avatar_config_hash = NULL, settings = '{}'::jsonb, preferred_language = NULL, last_area_id = NULL,
			updated_at = now(), version = version + 1
		WHERE tenant_id = $1 AND student_id = $2 AND ((nickname IS NOT NULL AND nickname <> $3) OR avatar_config IS NOT NULL)`,
		tenant, student, anon)
	if err != nil {
		return false, fmt.Errorf("game profile: anonymize: %w", err)
	}
	// tenancyguard:ok anonymization of every saved outfit of the student under the TenantScope (RLS) plus tenant_id
	if _, err := exec(ctx, `UPDATE game.avatar_presets SET name = NULL, updated_at = now(), version = version + 1
		WHERE tenant_id = $1 AND student_id = $2 AND name IS NOT NULL`, tenant, student); err != nil {
		return false, fmt.Errorf("avatar presets: anonymize: %w", err)
	}
	return ok, nil
}

// PresetRepo implements write.PresetRepository on game.avatar_presets.
type PresetRepo struct{}

var _ write.PresetRepository = PresetRepo{}

// Upsert implements write.PresetRepository.
func (PresetRepo) Upsert(ctx context.Context, tenant, student uuid.UUID, slot int, name *string, cfg []byte, at time.Time) error {
	// tenancyguard:ok upsert under the TenantScope (RLS) on the live-row index (student_id, slot); tenant_id kept by the WHERE
	if _, err := exec(ctx, `INSERT INTO game.avatar_presets AS a (avatar_preset_id, tenant_id, created_at, created_by, updated_at,
			student_id, slot, name, avatar_config)
		VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8::jsonb)
		ON CONFLICT (student_id, slot) WHERE deleted_at IS NULL DO UPDATE SET name = EXCLUDED.name,
			avatar_config = EXCLUDED.avatar_config, updated_at = EXCLUDED.updated_at, updated_by = EXCLUDED.created_by,
			version = a.version + 1
		WHERE a.tenant_id = EXCLUDED.tenant_id`,
		ids.New(), tenant, at.UTC(), actor(ctx), student, slot, name, string(cfg)); err != nil {
		return fmt.Errorf("avatar presets: upsert: %w", err)
	}
	return nil
}

// MoveStudent implements write.PresetRepository.
func (PresetRepo) MoveStudent(ctx context.Context, tenant, from, into uuid.UUID, reason string, at time.Time) error {
	// tenancyguard:ok soft-delete of the source outfits whose slot the target already uses; TenantScope (RLS) plus tenant_id
	if _, err := exec(ctx, `UPDATE game.avatar_presets a SET deleted_at = $4, deleted_by = $5, delete_reason = $6, updated_at = $4,
			version = a.version + 1
		WHERE a.tenant_id = $1 AND a.student_id = $2 AND a.deleted_at IS NULL AND EXISTS (SELECT 1 FROM game.avatar_presets t
			WHERE t.tenant_id = $1 AND t.student_id = $3 AND t.slot = a.slot AND t.deleted_at IS NULL)`,
		tenant, from, into, at.UTC(), actor(ctx), reason); err != nil {
		return fmt.Errorf("avatar presets: merge: %w", err)
	}
	// student_id is immutable in ent; the move rewrites the reference as the merge saga requires (ADR-0028 keeps the row).
	// tenancyguard:ok merge of the remaining live outfits to the target student; TenantScope (RLS) plus tenant_id
	if _, err := exec(ctx, `UPDATE game.avatar_presets SET student_id = $3, updated_at = $4, updated_by = $5, version = version + 1
		WHERE tenant_id = $1 AND student_id = $2 AND deleted_at IS NULL`, tenant, from, into, at.UTC(), actor(ctx)); err != nil {
		return fmt.Errorf("avatar presets: merge: %w", err)
	}
	return nil
}

// OwnershipRepo implements write.OwnershipReader on game.inventory_items.
type OwnershipRepo struct{}

var _ write.OwnershipReader = OwnershipRepo{}

// Owned implements write.OwnershipReader.
func (OwnershipRepo) Owned(ctx context.Context, tenant, student uuid.UUID, codes []string) (map[string]bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok TenantScope (RLS) plus the explicit tenant_id; deleted_at filtered
	rows, err := t.QueryContext(ctx, `SELECT item_id FROM game.inventory_items
		WHERE tenant_id = $1 AND student_id = $2 AND item_id = ANY($3::varchar[]) AND quantity > 0 AND deleted_at IS NULL`,
		tenant, student, codes)
	if err != nil {
		return nil, fmt.Errorf("inventory: owned: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// NicknameChangeLimit implements write.SchoolPolicy (settings_subset.nickname_change_monthly_limit of school.updated;
// a school may only tighten the platform value).
func (Directory) NicknameChangeLimit(ctx context.Context, tenant, school uuid.UUID) (*int, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var limit sql.NullInt64
	// tenancyguard:ok projection ref.schools (RLS + explicit tenant_id); projections have no deleted_at
	err = t.QueryRowContext(ctx, `SELECT CASE WHEN jsonb_typeof(settings_subset->'nickname_change_monthly_limit') = 'number'
			THEN (settings_subset->>'nickname_change_monthly_limit')::numeric::bigint END
		FROM ref.schools WHERE tenant_id = $1 AND school_id = $2`, tenant, school).Scan(&limit)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !limit.Valid) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ref schools: nickname limit: %w", err)
	}
	n := int(limit.Int64)
	return &n, nil
}

var _ write.SchoolPolicy = Directory{}

// CatalogRepo implements write.CatalogReader and write.CatalogPublisher on the platform content tables.
type CatalogRepo struct{}

var (
	_ write.CatalogReader    = CatalogRepo{}
	_ write.CatalogPublisher = CatalogRepo{}
)

// Current implements write.CatalogReader.
func (CatalogRepo) Current(ctx context.Context) (*catalog.Catalog, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	return catalogsql.Current(ctx, t, catalog.Code)
}

// Items implements write.CatalogReader.
func (CatalogRepo) Items(ctx context.Context, codes []string) (map[string]profile.Item, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on both tables
	rows, err := t.QueryContext(ctx, `SELECT d.item_id, v.kind, v.equip_slot FROM game.item_definitions d
		JOIN game.item_definition_versions v ON v.item_version_id = d.current_version_id
		WHERE d.item_id = ANY($1::varchar[]) AND d.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published'`, codes)
	if err != nil {
		return nil, fmt.Errorf("item definitions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]profile.Item{}
	for rows.Next() {
		var id string
		var it profile.Item
		if err := rows.Scan(&id, &it.Kind, &it.EquipSlot); err != nil {
			return nil, err
		}
		out[id] = it
	}
	return out, rows.Err()
}

// PublishCatalog implements write.CatalogPublisher.
func (CatalogRepo) PublishCatalog(ctx context.Context, code string, payload []byte, sha, note string, at time.Time) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok platform content (G table, no tenant_id); written under PlatformScope only
	if _, err := t.ExecContext(ctx, `INSERT INTO game.character_catalogs (catalog_id, created_at, created_by, updated_at, code)
		VALUES ($1, $2, $3, $2, $4) ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING`, ids.New(), at, actor(ctx), code); err != nil {
		return false, fmt.Errorf("character catalog: %w", err)
	}
	var id uuid.UUID
	var next int
	var current sql.NullString
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered
	if err := t.QueryRowContext(ctx, `SELECT c.catalog_id, c.latest_version_no + 1, v.payload_sha256 FROM game.character_catalogs c
		LEFT JOIN game.character_catalog_versions v ON v.catalog_version_id = c.current_version_id AND v.deleted_at IS NULL
		WHERE c.code = $1 AND c.deleted_at IS NULL FOR UPDATE OF c`, code).Scan(&id, &next, &current); err != nil {
		return false, fmt.Errorf("character catalog: %w", err)
	}
	if current.Valid && current.String == sha {
		return false, nil
	}
	vid := ids.New()
	// tenancyguard:ok platform content (G table, no tenant_id); written under PlatformScope only
	if _, err := t.ExecContext(ctx, `INSERT INTO game.character_catalog_versions (catalog_version_id, created_at, created_by,
			updated_at, catalog_id, version_no, status, payload, payload_sha256, change_note, published_at)
		VALUES ($1, $2, $3, $2, $4, $5, 'published', $6::jsonb, $7, $8, $2)`,
		vid, at, actor(ctx), id, next, string(payload), sha, nullString(note)); err != nil {
		return false, fmt.Errorf("character catalog: version: %w", err)
	}
	// tenancyguard:ok platform content (G tables, no tenant_id); the previous version is archived, never changed otherwise
	if _, err := t.ExecContext(ctx, `UPDATE game.character_catalog_versions SET status = 'archived', updated_at = $3
		WHERE catalog_id = $1 AND catalog_version_id <> $2 AND status = 'published' AND deleted_at IS NULL`, id, vid, at); err != nil {
		return false, fmt.Errorf("character catalog: archive: %w", err)
	}
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	if _, err := t.ExecContext(ctx, `UPDATE game.character_catalogs SET current_version_id = $2, latest_version_no = $3,
			updated_at = $4, version = version + 1 WHERE catalog_id = $1 AND deleted_at IS NULL`, id, vid, next, at); err != nil {
		return false, fmt.Errorf("character catalog: pointer: %w", err)
	}
	return true, nil
}

// PublishItem implements write.CatalogPublisher.
func (CatalogRepo) PublishItem(ctx context.Context, def catalog.ItemDefinition, at time.Time) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	name, err := json.Marshal(def.Name)
	if err != nil {
		return false, err
	}
	sources, err := json.Marshal(def.Sources)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok platform content (G table, no tenant_id); written under PlatformScope only
	if _, err := t.ExecContext(ctx, `INSERT INTO game.item_definitions (item_id, created_at, created_by, updated_at)
		VALUES ($1, $2, $3, $2) ON CONFLICT (item_id) DO NOTHING`, def.ItemID, at, actor(ctx)); err != nil {
		return false, fmt.Errorf("item definitions: %w", err)
	}
	var next int
	var same sql.NullBool
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered
	if err := t.QueryRowContext(ctx, `SELECT d.latest_version_no + 1, (v.kind = $2 AND v.equip_slot = $3 AND v.rarity = $4
			AND v.name = $5::jsonb AND v.sources = $6::jsonb)
		FROM game.item_definitions d LEFT JOIN game.item_definition_versions v ON v.item_version_id = d.current_version_id
		WHERE d.item_id = $1 AND d.deleted_at IS NULL FOR UPDATE OF d`, def.ItemID, def.Kind, def.EquipSlot, def.Rarity, string(name),
		string(sources)).Scan(&next, &same); err != nil {
		return false, fmt.Errorf("item definitions: %w", err)
	}
	if same.Valid && same.Bool {
		return false, nil
	}
	vid := ids.New()
	// tenancyguard:ok platform content (G table, no tenant_id); written under PlatformScope only
	if _, err := t.ExecContext(ctx, `INSERT INTO game.item_definition_versions (item_version_id, created_at, created_by, updated_at,
			item_id, version_no, kind, equip_slot, rarity, name, sources, status)
		VALUES ($1, $2, $3, $2, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb, 'published')`,
		vid, at, actor(ctx), def.ItemID, next, def.Kind, def.EquipSlot, def.Rarity, string(name), string(sources)); err != nil {
		return false, fmt.Errorf("item definitions: version: %w", err)
	}
	// tenancyguard:ok platform content (G tables, no tenant_id); the previous version is archived
	if _, err := t.ExecContext(ctx, `UPDATE game.item_definition_versions SET status = 'archived', updated_at = $3
		WHERE item_id = $1 AND item_version_id <> $2 AND status = 'published' AND deleted_at IS NULL`, def.ItemID, vid, at); err != nil {
		return false, fmt.Errorf("item definitions: archive: %w", err)
	}
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	if _, err := t.ExecContext(ctx, `UPDATE game.item_definitions SET current_version_id = $2, latest_version_no = $3, updated_at = $4,
			version = version + 1 WHERE item_id = $1 AND deleted_at IS NULL`, def.ItemID, vid, next, at); err != nil {
		return false, fmt.Errorf("item definitions: pointer: %w", err)
	}
	return true, nil
}

// Reassign implements write.ProfileRepository (mergeUserData: the target keeps the source profile when it has none).
func (ProfileRepo) Reassign(ctx context.Context, tenant, from, into uuid.UUID) (bool, error) {
	// tenancyguard:ok merge of the live profile to the target student; TenantScope (RLS) plus tenant_id; deleted_at filtered
	ok, err := exec(ctx, `UPDATE game.game_profiles SET student_id = $3, updated_at = now(), updated_by = $4, version = version + 1
		WHERE tenant_id = $1 AND student_id = $2 AND deleted_at IS NULL
			AND NOT EXISTS (SELECT 1 FROM game.game_profiles t WHERE t.tenant_id = $1 AND t.student_id = $3 AND t.deleted_at IS NULL)`,
		tenant, from, into, actor(ctx))
	if err != nil {
		return false, fmt.Errorf("game profile: reassign: %w", err)
	}
	return ok, nil
}
