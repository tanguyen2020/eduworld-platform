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
		(SELECT cv.version_no FROM game.config_versions cv WHERE cv.config_version_id = c.current_version_id AND cv.deleted_at IS NULL),
		(SELECT j.status FROM game.render_jobs j WHERE j.config_version_id = v.config_version_id AND j.kind = 'render_thumbnail'
			AND j.deleted_at IS NULL ORDER BY CASE j.status WHEN 'queued' THEN 0 WHEN 'failed' THEN 0 WHEN 'dead' THEN 1 ELSE 2 END,
			j.updated_at DESC, j.render_job_id LIMIT 1),
		ts.translation_status, ts.missing_pointers, ts.outdated_pointers
	FROM game.config_items c
	JOIN game.config_versions v ON v.config_id = c.config_id AND v.deleted_at IS NULL
	LEFT JOIN game.config_versions b ON b.config_version_id = v.based_on_version_id AND b.deleted_at IS NULL
	LEFT JOIN game.config_translation_states ts ON ts.config_version_id = v.config_version_id AND ts.deleted_at IS NULL`

func scanConfig(row interface{ Scan(...any) error }) (read.ConfigRow, error) {
	var r read.ConfigRow
	var title, note, change, thumb, turn, job, tstatus sql.NullString
	var payload, missing, outdated []byte
	err := row.Scan(&r.ConfigID, &r.ItemType, &r.ItemKey, &r.VersionID, &r.VersionNo, &r.Status, &title, &payload, &r.AuthorID, &r.ReviewerID,
		&note, &change, &r.BasedOnNo, &r.PublishedAt, &r.PublishedBy, &r.UpdatedAt, &r.Revision, &r.IsCurrent, &thumb, &turn, &r.ScheduledAt,
		&r.CurrentVersion, &job, &tstatus, &missing, &outdated)
	if err != nil {
		return r, err
	}
	r.TranslationStatus = ns(tstatus)
	if len(missing) > 0 {
		if err := json.Unmarshal(missing, &r.MissingFields); err != nil {
			return r, fmt.Errorf("missing_pointers: %w", err)
		}
	}
	if len(outdated) > 0 {
		if err := json.Unmarshal(outdated, &r.OutdatedFields); err != nil {
			return r, fmt.Errorf("outdated_pointers: %w", err)
		}
	}
	r.Payload = payload
	r.Title, r.ReviewNote, r.ChangeNote, r.ThumbnailSHA, r.TurntableSHA = ns(title), ns(note), ns(change), ns(thumb), ns(turn)
	r.RenderJob = ns(job)
	return r, nil
}

// scheduleAt is the instant of payload.schedule.<field> of the version v (event.schema.json date-time, checked by the
// type schema when the draft is saved); a value that does not look like a date-time is NULL instead of a cast error,
// and NULL never matches a window.
func scheduleAt(field string) string {
	return `CASE WHEN (v.payload #>> '{schedule,` + field + `}') ~ '^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}'
		THEN (v.payload #>> '{schedule,` + field + `}')::timestamptz END`
}

// decorationAreas is components.decoration.areas of the version v as a JSON array ('[]' when absent or not an array).
const decorationAreas = `CASE WHEN jsonb_typeof(v.payload #> '{components,decoration,areas}') = 'array'
	THEN v.payload #> '{components,decoration,areas}' ELSE '[]'::jsonb END`

// listWindow is listGameConfigs_game with a calendar window (F079-AC-14): the newest version of each item (or its newest
// version of status) first, then the window — schedule [start_at, end_at) intersecting [$5, $6), the half-open form of
// liveops.CalendarWindow.Overlaps — then the keyset page; the rows are read with configSelect.
//
// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on every table; keyset on config_id
var listWindow = `WITH newest AS (
		SELECT DISTINCT ON (c.config_id) c.config_id, v.config_version_id, ` + scheduleAt("start_at") + ` AS s, ` + scheduleAt("end_at") + ` AS e
		FROM game.config_items c JOIN game.config_versions v ON v.config_id = c.config_id AND v.deleted_at IS NULL
		WHERE c.item_type = $1 AND c.deleted_at IS NULL AND ($2 = '' OR v.status = $2) AND ($3::uuid IS NULL OR c.config_id > $3)
		ORDER BY c.config_id, v.version_no DESC),
	page AS (SELECT config_version_id FROM newest WHERE s < $6 AND e > $5 ORDER BY config_id LIMIT $4)
	` + configSelect + `
	WHERE v.config_version_id IN (SELECT config_version_id FROM page) AND c.deleted_at IS NULL ORDER BY c.config_id`

// listTranslation is listGameConfigs_game with translation_status (F082-AC-09): the newest version of each item (or its
// newest version of status), kept when the translation state of that version equals $5; keyset page on config_id.
//
// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on every table; keyset on config_id
var listTranslation = `WITH newest AS (
		SELECT DISTINCT ON (c.config_id) c.config_id, v.config_version_id
		FROM game.config_items c JOIN game.config_versions v ON v.config_id = c.config_id AND v.deleted_at IS NULL
		WHERE c.item_type = $1 AND c.deleted_at IS NULL AND ($2 = '' OR v.status = $2) AND ($3::uuid IS NULL OR c.config_id > $3)
		ORDER BY c.config_id, v.version_no DESC),
	page AS (SELECT n.config_version_id FROM newest n JOIN game.config_translation_states s ON s.config_version_id = n.config_version_id
		AND s.deleted_at IS NULL WHERE s.translation_status = $5 ORDER BY n.config_id LIMIT $4)
	` + configSelect + `
	WHERE v.config_version_id IN (SELECT config_version_id FROM page) AND c.deleted_at IS NULL ORDER BY c.config_id`

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
	var rows *sql.Rows
	if f.TranslationStatus != "" {
		rows, err = t.QueryContext(ctx, listTranslation, f.ItemType, f.Status, f.After, f.Limit, f.TranslationStatus)
	} else if f.WindowStart != nil && f.WindowEnd != nil {
		rows, err = t.QueryContext(ctx, listWindow, f.ItemType, f.Status, f.After, f.Limit, *f.WindowStart, *f.WindowEnd)
	} else {
		// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on every table; keyset on config_id
		rows, err = t.QueryContext(ctx, `SELECT DISTINCT ON (c.config_id) `+configSelect[len("SELECT "):]+`
			WHERE c.item_type = $1 AND c.deleted_at IS NULL AND ($2 = '' OR v.status = $2) AND ($3::uuid IS NULL OR c.config_id > $3)
			ORDER BY c.config_id, v.version_no DESC LIMIT $4`, f.ItemType, f.Status, f.After, f.Limit)
	}
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

// CalendarWarnings implements read.GameConfigReadModel: the listed event versions ($1) against the newest version of
// every other event of the window [$2, $3) — schedules overlapping (half-open) and components.decoration.areas sharing an
// area — numbered per listed event and cut at perItem ($4) in the query.
func (GameConfigs) CalendarWarnings(ctx context.Context, versionIDs []uuid.UUID, start, end time.Time, perItem int) ([]read.CalendarWarningRow, error) {
	if len(versionIDs) == 0 {
		return []read.CalendarWarningRow{}, nil
	}
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(versionIDs))
	for i, id := range versionIDs {
		ids[i] = id.String()
	}
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on every table
	rows, err := t.QueryContext(ctx, `WITH listed AS (
			SELECT v.config_id, `+decorationAreas+` AS areas, `+scheduleAt("start_at")+` AS s, `+scheduleAt("end_at")+` AS e
			FROM game.config_versions v WHERE v.config_version_id = ANY($1::uuid[]) AND v.item_type = 'event' AND v.deleted_at IS NULL),
		newest AS (
			SELECT DISTINCT ON (c.config_id) c.config_id, left(coalesce(v.title, v.payload #>> '{name,vi}'), 255) AS title,
				`+decorationAreas+` AS areas, `+scheduleAt("start_at")+` AS s, `+scheduleAt("end_at")+` AS e
			FROM game.config_items c JOIN game.config_versions v ON v.config_id = c.config_id AND v.deleted_at IS NULL
			WHERE c.item_type = 'event' AND c.deleted_at IS NULL
			ORDER BY c.config_id, v.version_no DESC),
		win AS (SELECT * FROM newest WHERE s < $3 AND e > $2 AND jsonb_array_length(areas) > 0),
		pairs AS (
			SELECT a.config_id AS id, b.config_id AS other_id, b.title AS other_title, ar.area,
				row_number() OVER (PARTITION BY a.config_id ORDER BY b.config_id, ar.area) AS n
			FROM listed a
			JOIN win b ON b.config_id <> a.config_id AND b.s < a.e AND a.s < b.e
			CROSS JOIN LATERAL (SELECT x FROM jsonb_array_elements_text(a.areas) x
				INTERSECT SELECT y FROM jsonb_array_elements_text(b.areas) y) ar(area))
		SELECT id, other_id, other_title, area FROM pairs WHERE n <= $4 ORDER BY id, n`, ids, start, end, perItem)
	if err != nil {
		return nil, fmt.Errorf("calendar warnings: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []read.CalendarWarningRow{}
	for rows.Next() {
		var r read.CalendarWarningRow
		var title sql.NullString
		if err := rows.Scan(&r.ConfigID, &r.OtherID, &title, &r.AreaID); err != nil {
			return nil, err
		}
		r.OtherTitle = ns(title)
		out = append(out, r)
	}
	return out, rows.Err()
}

// ClientManifests implements read.GameConfigReadModel (same rows as write.GameConfigStore.LatestClientManifests).
func (GameConfigs) ClientManifests(ctx context.Context) ([]read.ClientManifestRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform data (G table, no tenant_id); deleted_at filtered
	rows, err := t.QueryContext(ctx, `SELECT DISTINCT ON (platform) platform, client_version, published_at, manifest FROM game.client_manifests
		WHERE deleted_at IS NULL ORDER BY platform, published_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("client manifests: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []read.ClientManifestRow{}
	for rows.Next() {
		var r read.ClientManifestRow
		var b []byte
		if err := rows.Scan(&r.Platform, &r.ClientVersion, &r.PublishedAt, &b); err != nil {
			return nil, err
		}
		r.Manifest, r.PublishedAt = b, r.PublishedAt.UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

// AreaNames implements read.GameConfigReadModel: the name of the current release of each world area (item key = area_id).
func (GameConfigs) AreaNames(ctx context.Context, ids []string) (map[string]map[string]string, error) {
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on both tables
	return names(ctx, `SELECT c.item_key, v.payload -> 'name' FROM game.config_items c
		JOIN game.config_versions v ON v.config_version_id = c.current_version_id AND v.deleted_at IS NULL
		WHERE c.item_type = 'world_area' AND c.deleted_at IS NULL AND c.item_key = ANY($1::text[]) ORDER BY c.item_key`, ids)
}

// NPCNames implements read.GameConfigReadModel: npc_name of the current release of a dialogue of each NPC (the first
// dialogue by config_id when several).
func (GameConfigs) NPCNames(ctx context.Context, ids []string) (map[string]map[string]string, error) {
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on both tables
	return names(ctx, `SELECT DISTINCT ON (v.payload ->> 'npc_id') v.payload ->> 'npc_id', v.payload -> 'npc_name' FROM game.config_items c
		JOIN game.config_versions v ON v.config_version_id = c.current_version_id AND v.deleted_at IS NULL
		WHERE c.item_type = 'npc_dialogue' AND c.deleted_at IS NULL AND v.payload ->> 'npc_id' = ANY($1::text[]) AND v.payload ? 'npc_name'
		ORDER BY v.payload ->> 'npc_id', c.config_id`, ids)
}

func names(ctx context.Context, q string, ids []string) (map[string]map[string]string, error) {
	out := map[string]map[string]string{}
	if len(ids) == 0 {
		return out, nil
	}
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, q, ids)
	if err != nil {
		return nil, fmt.Errorf("manifest names: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id string
		var b []byte
		if err := rows.Scan(&id, &b); err != nil {
			return nil, err
		}
		var name map[string]string
		if json.Unmarshal(b, &name) == nil && name != nil {
			out[id] = name
		}
	}
	return out, rows.Err()
}

// ConfigCounts implements read.GameConfigReadModel: GROUP BY on the newest version of each item (deleted_at filtered).
func (GameConfigs) ConfigCounts(ctx context.Context, itemTypes []string) ([]read.ConfigCountRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on both tables
	rows, err := t.QueryContext(ctx, `SELECT x.item_type, x.status, count(*) FROM (
			SELECT DISTINCT ON (c.config_id) c.item_type, v.status
			FROM game.config_items c JOIN game.config_versions v ON v.config_id = c.config_id AND v.deleted_at IS NULL
			WHERE c.deleted_at IS NULL AND c.item_type = ANY($1::text[])
			ORDER BY c.config_id, v.version_no DESC) x
		GROUP BY x.item_type, x.status ORDER BY x.item_type, x.status`, itemTypes)
	if err != nil {
		return nil, fmt.Errorf("game config counts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []read.ConfigCountRow{}
	for rows.Next() {
		var r read.ConfigCountRow
		if err := rows.Scan(&r.ItemType, &r.Status, &r.Count); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
