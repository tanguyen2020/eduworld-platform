package write

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gameconfig"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// GameConfigRepo implements write.GameConfigStore on the console tables (game.config_items, game.config_versions,
// game.scheduled_publications, game.preview_sessions) and the release pointers of the type tables (F078). Platform
// content: written under PlatformScope only (role game_platform); every query filters deleted_at.
type GameConfigRepo struct{}

var _ write.GameConfigStore = GameConfigRepo{}

const itemCols = `config_id, item_type, item_key, latest_version_no`

func scanItem(row interface{ Scan(...any) error }) (*write.ConfigItem, error) {
	var it write.ConfigItem
	if err := row.Scan(&it.ConfigID, &it.ItemType, &it.ItemKey, &it.LatestVersionNo); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &it, nil
}

// ItemByID implements write.GameConfigStore.
func (GameConfigRepo) ItemByID(ctx context.Context, itemType string, id uuid.UUID) (*write.ConfigItem, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	return scanItem(t.QueryRowContext(ctx, `SELECT `+itemCols+` FROM game.config_items
		WHERE config_id = $1 AND item_type = $2 AND deleted_at IS NULL FOR UPDATE`, id, itemType))
}

// ItemByKey implements write.GameConfigStore.
func (GameConfigRepo) ItemByKey(ctx context.Context, itemType, key string) (*write.ConfigItem, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	return scanItem(t.QueryRowContext(ctx, `SELECT `+itemCols+` FROM game.config_items
		WHERE item_type = $1 AND item_key = $2 AND deleted_at IS NULL FOR UPDATE`, itemType, key))
}

// CreateItem implements write.GameConfigStore.
func (GameConfigRepo) CreateItem(ctx context.Context, it write.ConfigItem, by uuid.UUID, at time.Time) error {
	// tenancyguard:ok platform content (G table, no tenant_id)
	_, err := exec(ctx, `INSERT INTO game.config_items (config_id, created_at, created_by, updated_at, updated_by, item_type, item_key,
		latest_version_no) VALUES ($1, $2, $3, $2, $3, $4, $5, $6)`, it.ConfigID, at, nilUUID(by), it.ItemType, it.ItemKey, it.LatestVersionNo)
	if uniqueViolation(err) {
		return write.ErrConflict
	}
	return err
}

// NextVersionNo implements write.GameConfigStore: above both the console counter and the counter of the type table
// (versions published by the seed through the release core).
func (GameConfigRepo) NextVersionNo(ctx context.Context, it write.ConfigItem) (int, error) {
	ct, err := tableOf(it.ItemType)
	if err != nil {
		return 0, err
	}
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	// tenancyguard:ok platform content (G tables, no tenant_id); table names from a closed map
	err = t.QueryRowContext(ctx, fmt.Sprintf(`UPDATE game.config_items c SET latest_version_no = GREATEST(c.latest_version_no,
			coalesce((SELECT i.latest_version_no FROM %s.%s i WHERE i.%s::text = c.item_key AND i.deleted_at IS NULL), 0)) + 1, version = version + 1
		WHERE c.config_id = $1 AND c.deleted_at IS NULL RETURNING c.latest_version_no`, ct.sch(), ct.identity, ct.idCol), it.ConfigID).Scan(&n)
	return n, err
}

const versionCols = `config_version_id, config_id, item_type, version_no, status, title, payload, payload_sha256, author_id, reviewer_id,
	review_note, change_note, based_on_version_id, submitted_at, reviewed_at, published_at, published_by, updated_at, version`

func scanVersion(row interface{ Scan(...any) error }) (*write.ConfigVersion, error) {
	var v write.ConfigVersion
	var title, note, change sql.NullString
	var payload []byte
	if err := row.Scan(&v.ID, &v.ConfigID, &v.ItemType, &v.VersionNo, &v.Status, &title, &payload, &v.SHA256, &v.AuthorID, &v.ReviewerID,
		&note, &change, &v.BasedOn, &v.SubmittedAt, &v.ReviewedAt, &v.PublishedAt, &v.PublishedBy, &v.UpdatedAt, &v.Revision); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	v.Payload = payload
	v.Title, v.ReviewNote, v.ChangeNote = strPtr(title), strPtr(note), strPtr(change)
	return &v, nil
}

// InsertVersion implements write.GameConfigStore.
func (GameConfigRepo) InsertVersion(ctx context.Context, v write.ConfigVersion, by uuid.UUID, at time.Time) error {
	// tenancyguard:ok platform content (G table, no tenant_id)
	_, err := exec(ctx, `INSERT INTO game.config_versions (config_version_id, created_at, created_by, updated_at, updated_by, config_id,
			item_type, version_no, status, title, payload, payload_sha256, author_id, change_note, based_on_version_id)
		VALUES ($1, $2, $3, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10, $11, $12, $13)`, v.ID, at, nilUUID(by), v.ConfigID, v.ItemType,
		v.VersionNo, v.Status, v.Title, string(v.Payload), v.SHA256, v.AuthorID, v.ChangeNote, v.BasedOn)
	if uniqueViolation(err) {
		return write.ErrConflict
	}
	return err
}

// Version implements write.GameConfigStore.
func (GameConfigRepo) Version(ctx context.Context, configID uuid.UUID, no int) (*write.ConfigVersion, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	return scanVersion(t.QueryRowContext(ctx, `SELECT `+versionCols+` FROM game.config_versions
		WHERE config_id = $1 AND version_no = $2 AND deleted_at IS NULL FOR UPDATE`, configID, no))
}

// UpdateDraft implements write.GameConfigStore.
func (GameConfigRepo) UpdateDraft(ctx context.Context, v write.ConfigVersion, expected int64, by uuid.UUID, at time.Time) (bool, error) {
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered; optimistic version
	return exec(ctx, `UPDATE game.config_versions SET title = $3, payload = $4::jsonb, payload_sha256 = $5, status = $6, change_note = $7,
			updated_at = $8, updated_by = $9, version = version + 1
		WHERE config_version_id = $1 AND version = $2 AND status IN ('draft', 'rejected') AND deleted_at IS NULL`,
		v.ID, expected, v.Title, string(v.Payload), v.SHA256, v.Status, v.ChangeNote, at, nilUUID(by))
}

// SetWorkflow implements write.GameConfigStore.
func (GameConfigRepo) SetWorkflow(ctx context.Context, v write.ConfigVersion, expected int64, by uuid.UUID, at time.Time) (bool, error) {
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered; optimistic version
	return exec(ctx, `UPDATE game.config_versions SET status = $3, reviewer_id = $4, review_note = $5, submitted_at = $6, reviewed_at = $7,
			published_at = $8, published_by = $9, updated_at = $10, updated_by = $11, version = version + 1
		WHERE config_version_id = $1 AND version = $2 AND deleted_at IS NULL`,
		v.ID, expected, v.Status, v.ReviewerID, v.ReviewNote, v.SubmittedAt, v.ReviewedAt, v.PublishedAt, v.PublishedBy, at, nilUUID(by))
}

// Pointer implements write.GameConfigStore.
func (GameConfigRepo) Pointer(ctx context.Context, itemType, key string) (write.Pointer, error) {
	ct, err := tableOf(itemType)
	if err != nil {
		return write.Pointer{}, err
	}
	t, err := tx(ctx)
	if err != nil {
		return write.Pointer{}, err
	}
	var cur uuid.NullUUID
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered; table names from a closed map
	err = t.QueryRowContext(ctx, fmt.Sprintf(`SELECT current_version_id FROM %s.%s WHERE %s::text = $1 AND deleted_at IS NULL FOR UPDATE`,
		ct.sch(), ct.identity, ct.idCol), key).Scan(&cur)
	if errors.Is(err, sql.ErrNoRows) {
		return write.Pointer{}, nil
	}
	if err != nil {
		return write.Pointer{}, fmt.Errorf("%s: pointer: %w", ct.identity, err)
	}
	if cur.Valid {
		return write.Pointer{Current: &cur.UUID}, nil
	}
	return write.Pointer{}, nil
}

// MovePointer implements write.GameConfigStore (rollback, unpublish; the deferred trigger checks a published target).
func (GameConfigRepo) MovePointer(ctx context.Context, itemType, key string, to *uuid.UUID, by uuid.UUID, at time.Time) error {
	ct, err := tableOf(itemType)
	if err != nil {
		return err
	}
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered; table names from a closed map
	ok, err := exec(ctx, fmt.Sprintf(`UPDATE %s.%s SET current_version_id = $2, updated_at = $3, updated_by = $4, version = version + 1
		WHERE %s::text = $1 AND deleted_at IS NULL`, ct.sch(), ct.identity, ct.idCol), key, to, at, nilUUID(by))
	if err != nil {
		return fmt.Errorf("%s: pointer: %w", ct.identity, err)
	}
	if !ok {
		return write.ErrNotFound
	}
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered — console mirror of the pointer
	_, err = exec(ctx, `UPDATE game.config_items SET current_version_id = $3, updated_at = $4, version = version + 1
		WHERE item_type = $1 AND item_key = $2 AND deleted_at IS NULL`, itemType, key, to, at)
	return err
}

// TypeVersion implements write.GameConfigStore.
func (GameConfigRepo) TypeVersion(ctx context.Context, itemType string, versionID uuid.UUID) (json.RawMessage, error) {
	ct, err := tableOf(itemType)
	if err != nil {
		return nil, err
	}
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var b []byte
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered; table names from a closed map
	err = t.QueryRowContext(ctx, fmt.Sprintf(`SELECT payload FROM %s.%s WHERE %s = $1 AND status IN ('published', 'archived')
		AND deleted_at IS NULL`, ct.sch(), ct.versions, ct.versionCol), versionID).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return b, err
}

// SetTypeStatus implements write.GameConfigStore (status is a mutable column of a published version).
func (GameConfigRepo) SetTypeStatus(ctx context.Context, itemType string, versionID uuid.UUID, status string, at time.Time) error {
	ct, err := tableOf(itemType)
	if err != nil {
		return err
	}
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered; table names from a closed map
	_, err = exec(ctx, fmt.Sprintf(`UPDATE %s.%s SET status = $2, updated_at = $3 WHERE %s = $1 AND deleted_at IS NULL`,
		ct.sch(), ct.versions, ct.versionCol), versionID, status, at)
	return err
}

const scheduleCols = `schedule_id, config_id, item_type, config_version_id, version_no, publish_at, status, failure_code, change_note, reason,
	requested_by, cancelled_by, finished_at, created_at`

func scanSchedule(row interface{ Scan(...any) error }) (*write.Schedule, error) {
	var s write.Schedule
	var fc, note, reason sql.NullString
	if err := row.Scan(&s.ID, &s.ConfigID, &s.ItemType, &s.VersionID, &s.VersionNo, &s.PublishAt, &s.Status, &fc, &note, &reason,
		&s.RequestedBy, &s.CancelledBy, &s.FinishedAt, &s.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	s.FailureCode, s.ChangeNote, s.Reason = strPtr(fc), strPtr(note), strPtr(reason)
	return &s, nil
}

// InsertSchedule implements write.GameConfigStore.
func (GameConfigRepo) InsertSchedule(ctx context.Context, s write.Schedule, at time.Time) error {
	// tenancyguard:ok platform content (G table, no tenant_id)
	_, err := exec(ctx, `INSERT INTO game.scheduled_publications (schedule_id, created_at, created_by, updated_at, updated_by, config_id,
			item_type, config_version_id, version_no, publish_at, status, change_note, requested_by)
		VALUES ($1, $2, $3, $2, $3, $4, $5, $6, $7, $8, 'scheduled', $9, $3)`, s.ID, at, s.RequestedBy, s.ConfigID, s.ItemType, s.VersionID,
		s.VersionNo, s.PublishAt, s.ChangeNote)
	if uniqueViolation(err) {
		return write.ErrConflict
	}
	return err
}

// ScheduleByID implements write.GameConfigStore.
func (GameConfigRepo) ScheduleByID(ctx context.Context, configID, id uuid.UUID) (*write.Schedule, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	return scanSchedule(t.QueryRowContext(ctx, `SELECT `+scheduleCols+` FROM game.scheduled_publications
		WHERE schedule_id = $1 AND config_id = $2 AND deleted_at IS NULL FOR UPDATE`, id, configID))
}

// ActiveSchedule implements write.GameConfigStore.
func (GameConfigRepo) ActiveSchedule(ctx context.Context, configID uuid.UUID) (*write.Schedule, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	return scanSchedule(t.QueryRowContext(ctx, `SELECT `+scheduleCols+` FROM game.scheduled_publications
		WHERE config_id = $1 AND status = 'scheduled' AND deleted_at IS NULL FOR UPDATE`, configID))
}

// DueSchedules implements write.GameConfigStore: due publications, locked (SKIP LOCKED: one worker applies each).
func (GameConfigRepo) DueSchedules(ctx context.Context, now time.Time, limit int) ([]write.Schedule, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	rows, err := t.QueryContext(ctx, `SELECT `+scheduleCols+` FROM game.scheduled_publications
		WHERE status = 'scheduled' AND publish_at <= $1 AND deleted_at IS NULL ORDER BY publish_at, schedule_id LIMIT $2
		FOR UPDATE SKIP LOCKED`, now, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []write.Schedule
	for rows.Next() {
		s, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

// FinishSchedule implements write.GameConfigStore.
func (GameConfigRepo) FinishSchedule(ctx context.Context, s write.Schedule, at time.Time) error {
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	ok, err := exec(ctx, `UPDATE game.scheduled_publications SET status = $2, failure_code = $3, reason = $4, cancelled_by = $5,
			finished_at = $6, updated_at = $6, updated_by = $7, version = version + 1
		WHERE schedule_id = $1 AND status = 'scheduled' AND deleted_at IS NULL`, s.ID, s.Status, s.FailureCode, s.Reason, s.CancelledBy, at,
		actor(ctx))
	if err == nil && !ok {
		return write.ErrConflict
	}
	return err
}

// InsertPreview implements write.GameConfigStore.
func (GameConfigRepo) InsertPreview(ctx context.Context, id uuid.UUID, tokenHash, configType string, configID, versionID uuid.UUID, versionNo int,
	by uuid.UUID, at, expires time.Time) error {
	// tenancyguard:ok platform content (G table, no tenant_id)
	_, err := exec(ctx, `INSERT INTO game.preview_sessions (preview_session_id, created_at, created_by, updated_at, token_hash, config_type,
			config_id, config_version_id, version_no, expires_at) VALUES ($1, $2, $3, $2, $4, $5, $6, $7, $8, $9)`,
		id, at, by, tokenHash, configType, configID, versionID, versionNo, expires)
	return err
}

// Usages implements write.GameConfigStore: the current published versions of every game content type whose payload
// mentions the key, and the active tenant assignments of the item.
func (GameConfigRepo) Usages(ctx context.Context, itemType, key string, configID uuid.UUID, limit int) ([]write.Usage, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var out []write.Usage
	needle := `%"` + likeEscape(key) + `"%`
	for _, typ := range gameconfig.Types() {
		ct, err := tableOf(typ.ItemType)
		if err != nil || len(out) >= limit {
			continue
		}
		// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered; table names from a closed map
		ids, err := stringsOf(ctx, t, fmt.Sprintf(`SELECT i.%s::text FROM %s.%s i JOIN %s.%s v ON v.%s = i.current_version_id
			WHERE i.deleted_at IS NULL AND v.deleted_at IS NULL AND v.payload::text LIKE $1 AND NOT ($2 = '%s' AND i.%s::text = $3)
			ORDER BY 1 LIMIT $4`, ct.idCol, ct.sch(), ct.identity, ct.sch(), ct.versions, ct.versionCol, typ.ItemType, ct.idCol),
			needle, itemType, key, limit-len(out))
		if err != nil {
			return nil, fmt.Errorf("usages %s: %w", ct.versions, err)
		}
		for _, id := range ids {
			out = append(out, write.Usage{Kind: "game_content", ItemType: typ.ItemType, ItemID: id})
		}
	}
	if typ, ok := gameconfig.ByItemType(itemType); ok && typ.Assignable != "" && len(out) < limit {
		// tenancyguard:ok infra.tenant_resource_assignments of a platform resource: every tenant it is assigned to (platform console)
		rows, err := t.QueryContext(ctx, `SELECT tenant_id FROM infra.tenant_resource_assignments
			WHERE resource_type = $1 AND resource_id = $2 AND deleted_at IS NULL ORDER BY tenant_id LIMIT $3`, typ.Assignable, configID, limit-len(out))
		if err != nil {
			return nil, err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var tenant uuid.UUID
			if err := rows.Scan(&tenant); err != nil {
				return nil, err
			}
			out = append(out, write.Usage{Kind: "tenant_assignment", ItemType: typ.Assignable, ItemID: configID.String(), TenantID: &tenant})
		}
		return out, rows.Err()
	}
	return out, nil
}

// LatestClientManifests implements write.GameConfigStore.
func (GameConfigRepo) LatestClientManifests(ctx context.Context) ([]json.RawMessage, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform data (G table, no tenant_id); deleted_at filtered
	rows, err := t.QueryContext(ctx, `SELECT DISTINCT ON (platform) manifest FROM game.client_manifests WHERE deleted_at IS NULL
		ORDER BY platform, published_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []json.RawMessage
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// MediaStatuses implements write.GameConfigStore.
func (GameConfigRepo) MediaStatuses(ctx context.Context, assets []uuid.UUID) (map[uuid.UUID]write.MediaRef, error) {
	out := map[uuid.UUID]write.MediaRef{}
	if len(assets) == 0 {
		return out, nil
	}
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform media (G table, no tenant_id); deleted_at filtered
	rows, err := t.QueryContext(ctx, `SELECT asset_id, kind, status FROM game.media_assets WHERE asset_id = ANY($1::uuid[]) AND deleted_at IS NULL`,
		uuidStrings(assets))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id uuid.UUID
		var r write.MediaRef
		if err := rows.Scan(&id, &r.Kind, &r.Status); err != nil {
			return nil, err
		}
		out[id] = r
	}
	return out, rows.Err()
}

// LinkMedia implements write.GameConfigStore.
func (GameConfigRepo) LinkMedia(ctx context.Context, assets []uuid.UUID, itemType string, configID, versionID uuid.UUID, by uuid.UUID, at time.Time) error {
	for _, a := range assets {
		// tenancyguard:ok platform media (G table, no tenant_id)
		if _, err := exec(ctx, `INSERT INTO game.media_usages (usage_id, created_at, created_by, updated_at, asset_id, item_type, config_id,
				config_version_id, usage) VALUES ($1, $2, $3, $2, $4, $5, $6, $7, 'content')
			ON CONFLICT (asset_id, config_version_id, usage) WHERE deleted_at IS NULL DO NOTHING`,
			ids.New(), at, nilUUID(by), a, itemType, configID, versionID); err != nil {
			return err
		}
	}
	return nil
}

// consolePublished keeps the console tables in step with a version published through the release core (the seed, or
// the console itself): the config item exists, its counter covers the version, and the version is published.
func consolePublished(ctx context.Context, t *sql.Tx, v write.ContentVersion) error {
	typ, ok := gameconfig.ByItemType(v.ItemType)
	if !ok {
		return nil // not a console type (level)
	}
	var configID uuid.UUID
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	err := t.QueryRowContext(ctx, `SELECT config_id FROM game.config_items WHERE item_type = $1 AND item_key = $2 AND deleted_at IS NULL`,
		v.ItemType, v.ItemID).Scan(&configID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		configID = ids.New()
		if typ.UUIDKey {
			if configID, err = uuid.Parse(v.ItemID); err != nil {
				return fmt.Errorf("config item %s: %w", v.ItemID, err)
			}
		}
		// tenancyguard:ok platform content (G table, no tenant_id)
		if _, err := t.ExecContext(ctx, `INSERT INTO game.config_items (config_id, created_at, created_by, updated_at, updated_by, item_type,
			item_key, latest_version_no, current_version_id) VALUES ($1, $2, $3, $2, $3, $4, $5, $6, $7)`, configID, v.At, nilUUID(v.By), v.ItemType,
			v.ItemID, v.VersionNo, v.VersionID); err != nil {
			return fmt.Errorf("config item: %w", err)
		}
	case err != nil:
		return err
	default:
		// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
		if _, err := t.ExecContext(ctx, `UPDATE game.config_items SET latest_version_no = GREATEST(latest_version_no, $2), updated_at = $3,
			current_version_id = $4 WHERE config_id = $1`, configID, v.VersionNo, v.At, v.VersionID); err != nil {
			return err
		}
	}
	// tenancyguard:ok platform content (G table, no tenant_id)
	_, err = t.ExecContext(ctx, `INSERT INTO game.config_versions (config_version_id, created_at, created_by, updated_at, updated_by, config_id,
			item_type, version_no, status, payload, payload_sha256, change_note, published_at, published_by)
		VALUES ($1, $2, $3, $2, $3, $4, $5, $6, 'published', $7::jsonb, $8, $9, $2, $3)
		ON CONFLICT (config_version_id) DO UPDATE SET status = 'published', published_at = EXCLUDED.published_at,
			published_by = EXCLUDED.published_by, updated_at = EXCLUDED.updated_at, updated_by = EXCLUDED.updated_by,
			version = game.config_versions.version + 1`,
		v.VersionID, v.At, nilUUID(v.By), configID, v.ItemType, v.VersionNo, string(v.Payload), v.SHA256, nullString(v.Note))
	if err != nil {
		return fmt.Errorf("config version: %w", err)
	}
	return nil
}

func nilUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}

func strPtr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	v := s.String
	return &v
}

func uuidStrings(list []uuid.UUID) []string {
	out := make([]string, len(list))
	for i, u := range list {
		out[i] = u.String()
	}
	return out
}

func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// stringsOf runs a query of one text column.
func stringsOf(ctx context.Context, t *sql.Tx, query string, args ...any) ([]string, error) {
	rows, err := t.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// RenderState implements write.GameConfigStore (same rules as the read side configSelect).
func (GameConfigRepo) RenderState(ctx context.Context, versionID uuid.UUID) (bool, string, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, "", err
	}
	var ready bool
	var job sql.NullString
	// tenancyguard:ok platform media (G tables, no tenant_id); deleted_at filtered on every table
	err = t.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM game.media_usages u JOIN game.media_assets a ON a.asset_id = u.asset_id
				AND a.deleted_at IS NULL AND a.status = 'ready'
			WHERE u.config_version_id = $1 AND u.usage = 'render_thumbnail' AND u.deleted_at IS NULL),
		(SELECT j.status FROM game.render_jobs j WHERE j.config_version_id = $1 AND j.kind = 'render_thumbnail' AND j.deleted_at IS NULL
			ORDER BY CASE j.status WHEN 'queued' THEN 0 WHEN 'failed' THEN 0 WHEN 'dead' THEN 1 ELSE 2 END, j.updated_at DESC, j.render_job_id
			LIMIT 1)`, versionID).Scan(&ready, &job)
	if err != nil {
		return false, "", fmt.Errorf("render state: %w", err)
	}
	return ready, job.String, nil
}
