package read

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
)

// GameConfigs implements read.GameConfigReadModel (F078): explicit SQL on the console tables and the release pointer of
// the type table; platform content without tenant_id, every table filtered on deleted_at; lists page in the query
// (keyset + LIMIT n+1, never sliced in memory).
type GameConfigs struct{}

var _ read.GameConfigReadModel = GameConfigs{}

// configSelect is the columns of a version row with the release pointer of its item (config_items.current_version_id, the
// mirror of the type table pointer) and its renders.
//
// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on every table of the select
const configSelect = `SELECT c.config_id, c.item_type, c.item_key, v.config_version_id, v.version_no, v.status, v.title, v.payload, v.author_id,
		v.reviewer_id, v.review_note, v.change_note, b.version_no, v.published_at, v.published_by, v.updated_at, v.version,
		coalesce(c.current_version_id = v.config_version_id, false),
		(SELECT a.sha256 FROM game.media_usages u JOIN game.media_assets a ON a.asset_id = u.asset_id AND a.deleted_at IS NULL AND a.status = 'ready'
			WHERE u.config_version_id = v.config_version_id AND u.usage = 'render_thumbnail' AND u.deleted_at IS NULL ORDER BY u.created_at DESC LIMIT 1),
		(SELECT a.sha256 FROM game.media_usages u JOIN game.media_assets a ON a.asset_id = u.asset_id AND a.deleted_at IS NULL AND a.status = 'ready'
			WHERE u.config_version_id = v.config_version_id AND u.usage = 'render_turntable' AND u.deleted_at IS NULL ORDER BY u.created_at DESC LIMIT 1),
		(SELECT s.publish_at FROM game.scheduled_publications s WHERE s.config_version_id = v.config_version_id AND s.status = 'scheduled'
			AND s.deleted_at IS NULL LIMIT 1),
		(SELECT cv.version_no FROM game.config_versions cv WHERE cv.config_version_id = c.current_version_id AND cv.deleted_at IS NULL)
	FROM game.config_items c
	JOIN game.config_versions v ON v.config_id = c.config_id AND v.deleted_at IS NULL
	LEFT JOIN game.config_versions b ON b.config_version_id = v.based_on_version_id AND b.deleted_at IS NULL`

func scanConfig(row interface{ Scan(...any) error }) (read.ConfigRow, error) {
	var r read.ConfigRow
	var title, note, change, thumb, turn sql.NullString
	var payload []byte
	err := row.Scan(&r.ConfigID, &r.ItemType, &r.ItemKey, &r.VersionID, &r.VersionNo, &r.Status, &title, &payload, &r.AuthorID, &r.ReviewerID,
		&note, &change, &r.BasedOnNo, &r.PublishedAt, &r.PublishedBy, &r.UpdatedAt, &r.Revision, &r.IsCurrent, &thumb, &turn, &r.ScheduledAt,
		&r.CurrentVersion)
	if err != nil {
		return r, err
	}
	r.Payload = payload
	r.Title, r.ReviewNote, r.ChangeNote, r.ThumbnailSHA, r.TurntableSHA = ns(title), ns(note), ns(change), ns(thumb), ns(turn)
	return r, nil
}

func ns(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	v := s.String
	return &v
}

// List implements read.GameConfigReadModel.
func (GameConfigs) List(ctx context.Context, f read.ConfigFilter) ([]read.ConfigRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on every table; keyset on config_id
	rows, err := t.QueryContext(ctx, `SELECT DISTINCT ON (c.config_id) `+configSelect[len("SELECT "):]+`
		WHERE c.item_type = $1 AND c.deleted_at IS NULL AND ($2 = '' OR v.status = $2) AND ($3::uuid IS NULL OR c.config_id > $3)
		ORDER BY c.config_id, v.version_no DESC LIMIT $4`, f.ItemType, f.Status, f.After, f.Limit)
	if err != nil {
		return nil, fmt.Errorf("game configs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []read.ConfigRow{}
	for rows.Next() {
		r, err := scanConfig(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Version implements read.GameConfigReadModel.
func (GameConfigs) Version(ctx context.Context, itemType string, configID uuid.UUID, no int) (*read.ConfigRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on every table
	r, err := scanConfig(t.QueryRowContext(ctx, configSelect+`
		WHERE c.config_id = $1 AND c.item_type = $2 AND c.deleted_at IS NULL AND v.version_no = $3`, configID, itemType, no))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("game config version: %w", err)
	}
	return &r, nil
}

// Versions implements read.GameConfigReadModel.
func (GameConfigs) Versions(ctx context.Context, itemType string, configID uuid.UUID, before *int, limit int) ([]read.ConfigRow, *int, bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	var exists bool
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	err = t.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM game.config_items WHERE config_id = $1 AND item_type = $2 AND deleted_at IS NULL)`,
		configID, itemType).Scan(&exists)
	if err != nil || !exists {
		return nil, nil, false, err
	}
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on every table; keyset on version_no
	rows, err := t.QueryContext(ctx, configSelect+`
		WHERE c.config_id = $1 AND c.deleted_at IS NULL AND ($2::int IS NULL OR v.version_no < $2) ORDER BY v.version_no DESC LIMIT $3`,
		configID, before, limit)
	if err != nil {
		return nil, nil, true, fmt.Errorf("game config versions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []read.ConfigRow{}
	var current *int
	for rows.Next() {
		r, err := scanConfig(rows)
		if err != nil {
			return nil, nil, true, err
		}
		current = r.CurrentVersion
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, true, err
	}
	if len(out) == 0 {
		// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered
		_ = t.QueryRowContext(ctx, `SELECT cv.version_no FROM game.config_items c JOIN game.config_versions cv
			ON cv.config_version_id = c.current_version_id AND cv.deleted_at IS NULL WHERE c.config_id = $1`, configID).Scan(&current)
	}
	return out, current, true, nil
}

// Releases implements read.GameConfigReadModel.
func (GameConfigs) Releases(ctx context.Context, itemType string, configID uuid.UUID, beforeAt *time.Time, beforeID uuid.UUID, limit int) ([]read.ReleaseRow, bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, false, err
	}
	var key string
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	err = t.QueryRowContext(ctx, `SELECT item_key FROM game.config_items WHERE config_id = $1 AND item_type = $2 AND deleted_at IS NULL`,
		configID, itemType).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	// Release history: game.content_releases (GA, append-only, no deleted_at) and game.scheduled_publications (G, deleted_at
	// filtered); keyset on (at, id).
	// tenancyguard:ok platform release history (no tenant_id; append-only content_releases has no deleted_at)
	rows, err := t.QueryContext(ctx, `SELECT id, action, from_no, to_no, reason, scheduled_for, status, failure_code, by_user, at FROM (
			SELECT r.release_id AS id, r.action, fv.version_no AS from_no, tv.version_no AS to_no, r.reason, s.publish_at AS scheduled_for,
				'applied'::varchar AS status, NULL::varchar AS failure_code, r.released_by AS by_user, r.released_at AS at
			FROM game.content_releases r
			LEFT JOIN game.config_versions fv ON fv.config_version_id = r.from_version_id AND fv.deleted_at IS NULL
			LEFT JOIN game.config_versions tv ON tv.config_version_id = r.to_version_id AND tv.deleted_at IS NULL
			LEFT JOIN game.scheduled_publications s ON s.schedule_id = r.release_id AND s.deleted_at IS NULL
			WHERE r.item_type = $1 AND r.item_id = $2
			UNION ALL
			SELECT s.schedule_id, 'publish', NULL, s.version_no, s.reason, s.publish_at, s.status, s.failure_code,
				coalesce(s.cancelled_by, s.requested_by), coalesce(s.finished_at, s.created_at)
			FROM game.scheduled_publications s WHERE s.config_id = $3 AND s.status <> 'applied' AND s.deleted_at IS NULL
		) x WHERE ($4::timestamptz IS NULL OR (x.at, x.id) < ($4, $5)) ORDER BY x.at DESC, x.id DESC LIMIT $6`,
		itemType, key, configID, beforeAt, beforeID, limit)
	if err != nil {
		return nil, true, fmt.Errorf("game config releases: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []read.ReleaseRow{}
	for rows.Next() {
		var r read.ReleaseRow
		var reason, fc sql.NullString
		if err := rows.Scan(&r.ID, &r.Action, &r.FromNo, &r.ToNo, &reason, &r.ScheduledFor, &r.Status, &fc, &r.By, &r.At); err != nil {
			return nil, true, err
		}
		r.Reason, r.FailureCode = ns(reason), ns(fc)
		out = append(out, r)
	}
	return out, true, rows.Err()
}

// Preview implements read.GameConfigReadModel.
func (GameConfigs) Preview(ctx context.Context, tokenHash string) (*read.PreviewRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var r read.PreviewRow
	// tenancyguard:ok platform content (G table, no tenant_id); deleted_at filtered
	err = t.QueryRowContext(ctx, `SELECT preview_session_id, config_type, config_id, config_version_id, version_no, created_by, expires_at
		FROM game.preview_sessions WHERE token_hash = $1 AND deleted_at IS NULL`, tokenHash).
		Scan(&r.ID, &r.ConfigType, &r.ConfigID, &r.VersionID, &r.VersionNo, &r.CreatedBy, &r.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &r, err
}

// MediaAsset implements read.GameConfigReadModel.
func (GameConfigs) MediaAsset(ctx context.Context, id uuid.UUID) (*read.MediaAssetRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var r read.MediaAssetRow
	var reject, license, alt sql.NullString
	var variants []byte
	// tenancyguard:ok platform media (G table, no tenant_id); deleted_at filtered
	err = t.QueryRowContext(ctx, `SELECT asset_id, kind, mime_type, original_filename, size_bytes, sha256, bucket, status, reject_reason,
			variants, width, height, license, alt_text, created_at FROM game.media_assets WHERE asset_id = $1 AND deleted_at IS NULL`, id).
		Scan(&r.ID, &r.Kind, &r.MIME, &r.Filename, &r.Size, &r.SHA256, &r.Bucket, &r.Status, &reject, &variants, &r.Width, &r.Height, &license,
			&alt, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.Reject, r.License, r.AltText = ns(reject), ns(license), ns(alt)
	if len(variants) > 0 {
		_ = json.Unmarshal(variants, &r.Variants)
	}
	// tenancyguard:ok platform media (G tables, no tenant_id); deleted_at filtered on both tables
	rows, err := t.QueryContext(ctx, `SELECT u.item_type, u.config_id, v.version_no FROM game.media_usages u
		JOIN game.config_versions v ON v.config_version_id = u.config_version_id AND v.deleted_at IS NULL
		WHERE u.asset_id = $1 AND u.deleted_at IS NULL ORDER BY u.created_at, u.usage_id LIMIT 500`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var m read.MediaUse
		if err := rows.Scan(&m.ItemType, &m.ConfigID, &m.Version); err != nil {
			return nil, err
		}
		r.UsedIn = append(r.UsedIn, m)
	}
	return &r, rows.Err()
}

// Assigned implements read.GameConfigReadModel: the published quests and mini-game configs with an active assignment to
// tenant (explicit tenant condition, R8), keyset after (item_type, config_id).
func (GameConfigs) Assigned(ctx context.Context, tenant uuid.UUID, afterType string, afterID *uuid.UUID, limit int) ([]read.AssignedActivity, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok explicit tenant condition on infra.tenant_resource_assignments (deleted_at filtered) + platform content
	// (G tables, deleted_at filtered); keyset on (item_type, config_id)
	rows, err := t.QueryContext(ctx, `SELECT x.item_type, x.config_id, x.version_id, x.version_no, x.payload, x.published_at, x.thumb FROM (
			SELECT 'minigame_config' AS item_type, c.config_id, v.config_version_id AS version_id, v.version_no, v.payload, v.published_at,
				(SELECT a2.sha256 FROM game.media_usages u JOIN game.media_assets a2 ON a2.asset_id = u.asset_id AND a2.status = 'ready'
					AND a2.deleted_at IS NULL WHERE u.config_version_id = v.config_version_id AND u.usage = 'render_thumbnail'
					AND u.deleted_at IS NULL LIMIT 1) AS thumb
			FROM infra.tenant_resource_assignments a
			JOIN game.minigame_configs d ON d.minigame_config_id = a.resource_id AND d.deleted_at IS NULL AND d.current_version_id IS NOT NULL
			JOIN game.config_items c ON c.item_type = 'minigame_config' AND c.item_key = d.minigame_config_id::text AND c.deleted_at IS NULL
			JOIN game.config_versions v ON v.config_version_id = d.current_version_id AND v.deleted_at IS NULL
			WHERE a.tenant_id = $1 AND a.resource_type = 'minigame_config' AND a.deleted_at IS NULL
			UNION ALL
			SELECT 'quest', c.config_id, v.config_version_id, v.version_no, v.payload, v.published_at,
				(SELECT a2.sha256 FROM game.media_usages u JOIN game.media_assets a2 ON a2.asset_id = u.asset_id AND a2.status = 'ready'
					AND a2.deleted_at IS NULL WHERE u.config_version_id = v.config_version_id AND u.usage = 'render_thumbnail'
					AND u.deleted_at IS NULL LIMIT 1)
			FROM infra.tenant_resource_assignments a
			JOIN game.quest_definitions d ON d.quest_id = a.resource_id AND d.deleted_at IS NULL AND d.current_version_id IS NOT NULL
			JOIN game.config_items c ON c.item_type = 'quest' AND c.item_key = d.quest_id::text AND c.deleted_at IS NULL
			JOIN game.config_versions v ON v.config_version_id = d.current_version_id AND v.deleted_at IS NULL
			WHERE a.tenant_id = $1 AND a.resource_type = 'quest' AND a.deleted_at IS NULL
		) x WHERE ($3::uuid IS NULL OR (x.item_type, x.config_id) > ($2, $3)) ORDER BY x.item_type, x.config_id LIMIT $4`,
		tenant, afterType, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("assigned activities: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []read.AssignedActivity{}
	for rows.Next() {
		var a read.AssignedActivity
		var payload []byte
		var thumb sql.NullString
		if err := rows.Scan(&a.ItemType, &a.ConfigID, &a.VersionID, &a.VersionNo, &payload, &a.PublishedAt, &thumb); err != nil {
			return nil, err
		}
		a.Payload, a.ThumbnailSHA = payload, ns(thumb)
		out = append(out, a)
	}
	return out, rows.Err()
}
