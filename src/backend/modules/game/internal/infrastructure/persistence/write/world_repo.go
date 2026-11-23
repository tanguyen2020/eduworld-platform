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
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/world"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// contentTable describes the identity and version tables of a game content type (F071 release core).
type contentTable struct {
	schema               string // "" = game
	identity, idCol      string
	versions, versionCol string
	parent               []string // extra identity columns (npc_id, area_id)
	columns              []string // typed version columns, in order
}

var contentTables = map[string]contentTable{
	// F073: mini-game configs (question_types = result of validateMinigameQuestionSource at publish).
	write.ItemMiniGameConfig: {identity: "minigame_configs", idCol: "minigame_config_id", versions: "minigame_config_versions",
		versionCol: "minigame_config_version_id", parent: []string{"code"}, columns: []string{"code", "minigame_type", "grade_levels", "subject_id",
			"question_types"}},
	write.ItemWorldArea: {identity: "world_areas", idCol: "area_id", versions: "world_area_versions", versionCol: "area_version_id",
		columns: []string{"region_code", "area_class", "subject_code"}},
	write.ItemStoryChapter: {identity: "story_chapters", idCol: "chapter_code", versions: "story_chapter_versions",
		versionCol: "chapter_version_id", columns: []string{"region_code", "order_no", "subject_code"}},
	write.ItemNPCDialogue: {identity: "npc_dialogues", idCol: "dialogue_id", versions: "npc_dialogue_versions",
		versionCol: "dialogue_version_id", parent: []string{"npc_id"}, columns: []string{"npc_id", "grade_levels"}},
	write.ItemLevel: {identity: "level_definitions", idCol: "level_id", versions: "level_versions", versionCol: "level_version_id",
		parent: []string{"area_id"}, columns: []string{"bundle_id", "bundle_url", "bundle_sha256", "size_bytes"}},
	write.ItemQuest: {identity: "quest_definitions", idCol: "quest_id", versions: "quest_versions", versionCol: "quest_version_id",
		parent: []string{"code"}, columns: []string{"code", "quest_type", "grade_levels", "subject_id", "subject_code", "area_id",
			"giver_npc_id", "story_chapter_code", "start_trigger", "objective_ids", "available_from", "available_to"}},
	write.ItemProgressionCurve: {identity: "progression_curves", idCol: "curve_id", versions: "progression_curve_versions",
		versionCol: "curve_version_id", parent: []string{"code"}, columns: []string{"code", "max_level", "level_floor_protection"}},
	write.ItemRewardRule: {schema: "gamification", identity: "reward_rules", idCol: "reward_rule_id", versions: "reward_rule_versions",
		versionCol: "reward_rule_version_id", parent: []string{"code"}, columns: []string{"code", "trigger_event_type", "source_code",
			"effective_from", "active_to"}},
	// F075: items (F070 tables, now with the release columns), shop items, collections, achievements.
	write.ItemGameItem: {identity: "item_definitions", idCol: "item_id", versions: "item_definition_versions", versionCol: "item_version_id",
		columns: []string{"kind", "equip_slot", "rarity", "name", "sources", "asset_refs", "stackable", "max_stack"}},
	write.ItemShopItem: {schema: "gamification", identity: "shop_items", idCol: "shop_item_id", versions: "shop_item_versions",
		versionCol: "shop_item_version_id", columns: []string{"item_id", "item_kind", "price_amount", "currency_code", "purchase_limit",
			"available_from", "available_to", "grade_levels", "featured_order", "event_id", "season_id"}},
	write.ItemCollection: {schema: "gamification", identity: "collections", idCol: "collection_id", versions: "collection_versions",
		versionCol: "collection_version_id", parent: []string{"code"}, columns: []string{"code", "collection_type", "visibility", "item_count"}},
	write.ItemAchievement: {schema: "gamification", identity: "achievements", idCol: "achievement_id", versions: "achievement_versions",
		versionCol: "achievement_version_id", parent: []string{"code"}, columns: []string{"code", "category", "visibility", "trigger_event_types"}},
	// F077: platform events (G/T tables, platform rows only here), school event templates, season tracks.
	write.ItemEvent: {identity: "events", idCol: "event_id", versions: "event_versions", versionCol: "event_version_id",
		parent: []string{"code", "event_kind"}, columns: []string{"event_kind", "start_at", "end_at", "decoration_areas", "effective_from"}},
	write.ItemEventTemplate: {identity: "event_templates", idCol: "template_id", versions: "event_template_versions",
		versionCol: "template_version_id", parent: []string{"code"}, columns: []string{"code", "event_kind", "allowed_params", "max_duration_days"}},
	write.ItemSeasonTrack: {identity: "season_tracks", idCol: "season_track_id", versions: "season_track_versions",
		versionCol: "season_track_version_id", parent: []string{"season_id", "code"}, columns: []string{"season_id"}},
	// F076: solo challenges (platform content, assigned to tenants).
	write.ItemChallenge: {identity: "challenges", idCol: "challenge_id", versions: "challenge_versions", versionCol: "challenge_version_id",
		parent: []string{"code"}, columns: []string{"code", "challenge_type", "mode", "grade_levels", "subject_id", "objective_ids", "area_id",
			"available_from", "available_to"}},
}

// sch is the schema of the tables of a content type.
func (c contentTable) sch() string {
	if c.schema == "" {
		return "game"
	}
	return c.schema
}

// jsonColumns are the typed version columns stored as jsonb.
var jsonColumns = map[string]bool{"grade_levels": true, "objective_ids": true, "name": true, "sources": true, "asset_refs": true,
	"trigger_event_types": true, "question_types": true, "decoration_areas": true, "allowed_params": true}

// GameContentRepo implements write.GameContentStore (platform content, written under PlatformScope only).
type GameContentRepo struct{}

var _ write.GameContentStore = GameContentRepo{}

func tableOf(itemType string) (contentTable, error) {
	t, ok := contentTables[itemType]
	if !ok {
		return t, fmt.Errorf("game content: unknown item type %q", itemType)
	}
	return t, nil
}

// Head implements write.GameContentStore.
func (GameContentRepo) Head(ctx context.Context, itemType, itemID string, parent map[string]any, at time.Time) (write.ContentHead, error) {
	var h write.ContentHead
	ct, err := tableOf(itemType)
	if err != nil {
		return h, err
	}
	t, err := tx(ctx)
	if err != nil {
		return h, err
	}
	cols, vals := "", []any{itemID, at, actor(ctx)}
	for _, c := range ct.parent {
		cols += ", " + c
		vals = append(vals, parent[c])
	}
	ph := ""
	for i := range ct.parent {
		ph += fmt.Sprintf(", $%d", i+4)
	}
	// tenancyguard:ok platform content (G table, no tenant_id); written under PlatformScope only; table names from a closed map
	if _, err := t.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s.%s (%s, created_at, created_by, updated_at%s) VALUES ($1, $2, $3, $2%s)
		ON CONFLICT (%s) DO NOTHING`, ct.sch(), ct.identity, ct.idCol, cols, ph, ct.idCol), vals...); err != nil {
		return h, fmt.Errorf("%s: %w", ct.identity, err)
	}
	var current uuid.NullUUID
	var sha sql.NullString
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered; table names from a closed map
	err = t.QueryRowContext(ctx, fmt.Sprintf(`SELECT i.latest_version_no + 1, i.current_version_id, v.payload_sha256 FROM %s.%s i
		LEFT JOIN %s.%s v ON v.%s = i.current_version_id AND v.deleted_at IS NULL
		WHERE i.%s::text = $1 AND i.deleted_at IS NULL FOR UPDATE OF i`, ct.sch(), ct.identity, ct.sch(), ct.versions, ct.versionCol, ct.idCol), itemID).
		Scan(&h.NextVersionNo, &current, &sha)
	if err != nil {
		return h, fmt.Errorf("%s: head: %w", ct.identity, err)
	}
	if current.Valid {
		h.CurrentVersionID = &current.UUID
	}
	h.CurrentSHA256 = sha.String
	return h, nil
}

// Publish implements write.GameContentStore.
func (GameContentRepo) Publish(ctx context.Context, v write.ContentVersion) error {
	ct, err := tableOf(v.ItemType)
	if err != nil {
		return err
	}
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	cols := ""
	vals := []any{v.VersionID, v.At, v.By, v.ItemID, v.VersionNo, string(v.Payload), v.SHA256, nullString(v.Note), nullString(v.SnapshotKey),
		nullString(v.SnapshotSHA256)}
	ph := ""
	for i, c := range ct.columns {
		cols += ", " + c
		val := v.Columns[c]
		if jsonColumns[c] {
			b, err := json.Marshal(val)
			if err != nil {
				return err
			}
			val = string(b)
			ph += fmt.Sprintf(", $%d::jsonb", len(vals)+1)
		} else {
			ph += fmt.Sprintf(", $%d", len(vals)+1)
		}
		_ = i
		vals = append(vals, val)
	}
	// tenancyguard:ok platform content (G table, no tenant_id); written under PlatformScope only; table names from a closed map
	if _, err := t.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s.%s (%s, created_at, created_by, updated_at, %s, version_no, status, payload,
			payload_sha256, change_note, snapshot_key, snapshot_sha256, published_at, published_by%s)
		VALUES ($1, $2, $3, $2, $4, $5, 'published', $6::jsonb, $7, $8, $9, $10, $2, $3%s)`, ct.sch(), ct.versions, ct.versionCol, ct.idCol, cols, ph),
		vals...); err != nil {
		return fmt.Errorf("%s: insert: %w", ct.versions, err)
	}
	// tenancyguard:ok platform content (G table, no tenant_id); the previous published version is archived, never changed otherwise
	if _, err := t.ExecContext(ctx, fmt.Sprintf(`UPDATE %s.%s SET status = 'archived', updated_at = $3
		WHERE %s::text = $1 AND %s <> $2 AND status = 'published' AND deleted_at IS NULL`, ct.sch(), ct.versions, ct.idCol, ct.versionCol),
		v.ItemID, v.VersionID, v.At); err != nil {
		return fmt.Errorf("%s: archive: %w", ct.versions, err)
	}
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered; table names from a closed map
	if _, err := t.ExecContext(ctx, fmt.Sprintf(`UPDATE %s.%s SET current_version_id = $2, latest_version_no = $3, updated_at = $4,
		updated_by = $5, version = version + 1 WHERE %s::text = $1 AND deleted_at IS NULL`, ct.sch(), ct.identity, ct.idCol),
		v.ItemID, v.VersionID, v.VersionNo, v.At, v.By); err != nil {
		return fmt.Errorf("%s: pointer: %w", ct.identity, err)
	}
	return nil
}

// Release implements write.GameContentStore.
func (GameContentRepo) Release(ctx context.Context, r write.ContentRelease) error {
	// tenancyguard:ok platform release history game.content_releases (GA: no tenant_id, append-only)
	_, err := exec(ctx, `INSERT INTO game.content_releases (release_id, created_at, created_by, item_type, item_id, action, from_version_id,
			to_version_id, reason, released_by, released_at, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $3, $2, $10)`, r.ReleaseID, r.At, r.By, r.ItemType, r.ItemID, r.Action, r.From, r.To,
		nullString(r.Reason), r.IdempotencyKey)
	if err != nil {
		return fmt.Errorf("content releases: %w", err)
	}
	return nil
}

// WorldRepo implements write.WorldStore on the tenant tables of the world progress.
type WorldRepo struct{}

var _ write.WorldStore = WorldRepo{}

// LockStudent implements write.WorldStore.
func (WorldRepo) LockStudent(ctx context.Context, student uuid.UUID) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok advisory lock only (no table read)
	_, err = t.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('game.world:' || $1::text, 0))`, student)
	if err != nil {
		return fmt.Errorf("world: lock student: %w", err)
	}
	return nil
}

// RecordFact implements write.WorldStore.
func (WorldRepo) RecordFact(ctx context.Context, tenant, student uuid.UUID, kind, ref string, value int, at time.Time) (bool, error) {
	// tenancyguard:ok tenant table game.world_unlock_facts: explicit tenant_id (RLS insert/update policies); live rows only
	ok, err := exec(ctx, `INSERT INTO game.world_unlock_facts AS f (fact_id, tenant_id, created_at, created_by, updated_at, student_id,
			fact_kind, fact_ref, fact_value, occurred_at)
		VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8, $3)
		ON CONFLICT (tenant_id, student_id, fact_kind, fact_ref) WHERE deleted_at IS NULL
		DO UPDATE SET fact_value = EXCLUDED.fact_value, occurred_at = EXCLUDED.occurred_at, updated_at = EXCLUDED.updated_at,
			version = f.version + 1
		WHERE f.fact_value < EXCLUDED.fact_value`, ids.New(), tenant, at, actor(ctx), student, kind, ref, value)
	if err != nil {
		return false, fmt.Errorf("world facts: %w", err)
	}
	return ok, nil
}

// InsertUnlock implements write.WorldStore.
func (WorldRepo) InsertUnlock(ctx context.Context, tenant, student uuid.UUID, areaID string, ruleVersion int, by string, at time.Time) (bool, error) {
	// tenancyguard:ok tenant table game.world_area_unlocks: explicit tenant_id (RLS insert policy); unique among live rows
	ok, err := exec(ctx, `INSERT INTO game.world_area_unlocks (area_unlock_id, tenant_id, created_at, created_by, updated_at, student_id,
			area_id, rule_version, unlocked_by, unlocked_at)
		VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8, $3)
		ON CONFLICT (tenant_id, student_id, area_id) WHERE deleted_at IS NULL DO NOTHING`,
		ids.New(), tenant, at, actor(ctx), student, areaID, ruleVersion, by)
	if err != nil {
		return false, fmt.Errorf("area unlocks: %w", err)
	}
	return ok, nil
}

// SaveStory implements write.WorldStore.
func (WorldRepo) SaveStory(ctx context.Context, tenant, student uuid.UUID, chapter string, versionID uuid.UUID, p world.Progress, done int,
	completedAt *time.Time, at time.Time) error {
	quests, err := json.Marshal(p.CompletedQuests)
	if err != nil {
		return err
	}
	// tenancyguard:ok tenant table game.story_progress: explicit tenant_id (RLS insert/update policies); live rows only
	_, err = exec(ctx, `INSERT INTO game.story_progress AS s (story_progress_id, tenant_id, created_at, created_by, updated_at, student_id,
			chapter_code, chapter_version_id, status, main_quests_completed, completed_quest_ids, completed_at)
		VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8, $9, $10::jsonb, $11)
		ON CONFLICT (tenant_id, student_id, chapter_code) WHERE deleted_at IS NULL
		DO UPDATE SET chapter_version_id = EXCLUDED.chapter_version_id, status = EXCLUDED.status,
			main_quests_completed = EXCLUDED.main_quests_completed, completed_quest_ids = EXCLUDED.completed_quest_ids,
			completed_at = EXCLUDED.completed_at, updated_at = EXCLUDED.updated_at, version = s.version + 1`,
		ids.New(), tenant, at, actor(ctx), student, chapter, versionID, p.Status, done, string(quests), completedAt)
	if err != nil {
		return fmt.Errorf("story progress: %w", err)
	}
	return nil
}

// SaveExploration implements write.WorldStore.
func (WorldRepo) SaveExploration(ctx context.Context, tenant, student uuid.UUID, areaID string, e world.Exploration, pct float64,
	completedAt *time.Time, at time.Time) error {
	d, err := json.Marshal(nonNil(e.Discoveries))
	if err != nil {
		return err
	}
	c, err := json.Marshal(nonNil(e.Collectibles))
	if err != nil {
		return err
	}
	// tenancyguard:ok tenant table game.exploration_progress: explicit tenant_id (RLS insert/update policies); live rows only
	_, err = exec(ctx, `INSERT INTO game.exploration_progress AS x (exploration_id, tenant_id, created_at, created_by, updated_at, student_id,
			area_id, discoveries, collectibles, completion_pct, completed_at)
		VALUES ($1, $2, $3, $4, $3, $5, $6, $7::jsonb, $8::jsonb, $9, $10)
		ON CONFLICT (tenant_id, student_id, area_id) WHERE deleted_at IS NULL
		DO UPDATE SET discoveries = EXCLUDED.discoveries, collectibles = EXCLUDED.collectibles, completion_pct = EXCLUDED.completion_pct,
			completed_at = coalesce(x.completed_at, EXCLUDED.completed_at), updated_at = EXCLUDED.updated_at, version = x.version + 1`,
		ids.New(), tenant, at, actor(ctx), student, areaID, string(d), string(c), pct, completedAt)
	if err != nil {
		return fmt.Errorf("exploration progress: %w", err)
	}
	return nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// SetLastArea implements write.WorldStore.
func (WorldRepo) SetLastArea(ctx context.Context, tenant, student uuid.UUID, areaID string, at time.Time) error {
	// tenancyguard:ok tenant table game.game_profiles: explicit tenant_id + deleted_at IS NULL (RLS update policy)
	_, err := exec(ctx, `UPDATE game.game_profiles SET last_area_id = $3, updated_at = $4, version = version + 1
		WHERE tenant_id = $1 AND student_id = $2 AND deleted_at IS NULL AND last_area_id IS DISTINCT FROM $3`, tenant, student, areaID, at)
	if err != nil {
		return fmt.Errorf("game profile: last area: %w", err)
	}
	return nil
}

const levelSessionCols = `level_session_id, tenant_id, student_id, game_session_id, level_id, level_version_id, level_version_no, area_id,
	manifest_version, content_versions, status, started_at, ended_at`

func scanLevelSession(row *sql.Row) (*write.LevelSession, error) {
	var s write.LevelSession
	var pins []byte
	var ended sql.NullTime
	err := row.Scan(&s.ID, &s.TenantID, &s.StudentID, &s.GameSessionID, &s.LevelID, &s.LevelVersionID, &s.LevelVersionNo, &s.AreaID,
		&s.ManifestVersion, &pins, &s.Status, &s.StartedAt, &ended)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("level session: %w", err)
	}
	if err := json.Unmarshal(pins, &s.ContentVersions); err != nil {
		return nil, fmt.Errorf("level session pins: %w", err)
	}
	s.StartedAt = s.StartedAt.UTC()
	if ended.Valid {
		at := ended.Time.UTC()
		s.EndedAt = &at
	}
	return &s, nil
}

// ActiveLevelSession implements write.WorldStore.
func (WorldRepo) ActiveLevelSession(ctx context.Context, tenant, student uuid.UUID) (*write.LevelSession, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant table game.level_sessions: explicit tenant_id + deleted_at IS NULL (RLS as well)
	return scanLevelSession(t.QueryRowContext(ctx, `SELECT `+levelSessionCols+` FROM game.level_sessions
		WHERE tenant_id = $1 AND student_id = $2 AND status = 'active' AND deleted_at IS NULL FOR UPDATE`, tenant, student))
}

// LevelSession implements write.WorldStore.
func (WorldRepo) LevelSession(ctx context.Context, tenant, id uuid.UUID) (*write.LevelSession, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant table game.level_sessions: explicit tenant_id + deleted_at IS NULL (RLS as well)
	return scanLevelSession(t.QueryRowContext(ctx, `SELECT `+levelSessionCols+` FROM game.level_sessions
		WHERE tenant_id = $1 AND level_session_id = $2 AND deleted_at IS NULL FOR UPDATE`, tenant, id))
}

// CreateLevelSession implements write.WorldStore.
func (WorldRepo) CreateLevelSession(ctx context.Context, s write.LevelSession) error {
	pins, err := json.Marshal(s.ContentVersions)
	if err != nil {
		return err
	}
	// tenancyguard:ok tenant table game.level_sessions: explicit tenant_id (RLS insert policy)
	_, err = exec(ctx, `INSERT INTO game.level_sessions (level_session_id, tenant_id, created_at, created_by, updated_at, student_id,
			game_session_id, level_id, level_version_id, level_version_no, area_id, manifest_version, content_versions, status, started_at)
		VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8, $9, $10, $11, $12::jsonb, 'active', $3)`,
		s.ID, s.TenantID, s.StartedAt, actor(ctx), s.StudentID, s.GameSessionID, s.LevelID, s.LevelVersionID, s.LevelVersionNo, s.AreaID,
		s.ManifestVersion, string(pins))
	if err != nil {
		return fmt.Errorf("level session: insert: %w", err)
	}
	return nil
}

// EndLevelSession implements write.WorldStore.
func (WorldRepo) EndLevelSession(ctx context.Context, tenant, id uuid.UUID, status string, at time.Time) error {
	// tenancyguard:ok tenant table game.level_sessions: explicit tenant_id + deleted_at IS NULL (RLS update policy)
	_, err := exec(ctx, `UPDATE game.level_sessions SET status = $3, ended_at = $4, updated_at = $4, version = version + 1
		WHERE tenant_id = $1 AND level_session_id = $2 AND status = 'active' AND deleted_at IS NULL`, tenant, id, status, at)
	if err != nil {
		return fmt.Errorf("level session: end: %w", err)
	}
	return nil
}

var _ write.LevelTableStore = GameContentRepo{}

// InsertLevels implements write.LevelTableStore: the level table of a curve version, in the publish transaction.
func (GameContentRepo) InsertLevels(ctx context.Context, curveVersionID uuid.UUID, levels []write.LevelRow) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, l := range levels {
		rewards := l.Rewards
		if rewards == nil {
			rewards = []map[string]any{}
		}
		raw, err := json.Marshal(rewards)
		if err != nil {
			return err
		}
		// tenancyguard:ok platform content (GA table game.progression_levels, no tenant_id, append-only); PlatformScope only
		if _, err := t.ExecContext(ctx, `INSERT INTO game.progression_levels (progression_level_id, created_at, created_by, curve_version_id, level,
				xp_required, xp_cumulative, title_code, rewards) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb)`,
			ids.New(), now, actor(ctx), curveVersionID, l.Level, l.XPRequired, l.Cumulative, l.TitleCode, string(raw)); err != nil {
			return fmt.Errorf("progression levels: %w", err)
		}
	}
	return nil
}
