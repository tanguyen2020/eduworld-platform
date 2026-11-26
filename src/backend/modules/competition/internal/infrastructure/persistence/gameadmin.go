package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/gameconfig"
)

// GameConfigStore implements write.GameConfigStore, write.ConfigApplyStore and read.GameConfigReader on
// competition.game_configs, game_config_versions and content_releases (G/GA platform tables, F108): written under the
// platform pool (competition_platform), live rows only, every list paged in SQL (LIMIT n+1).
type GameConfigStore struct{}

var (
	_ write.GameConfigStore  = GameConfigStore{}
	_ write.ConfigApplyStore = GameConfigStore{}
	_ read.GameConfigReader  = GameConfigStore{}
)

const itemCols = `c.config_id, c.config_type, c.code, c.current_version_id, c.latest_version_no`

func scanItem(s interface{ Scan(...any) error }) (port.ConfigItem, error) {
	var it port.ConfigItem
	var cur uuid.NullUUID
	err := s.Scan(&it.ConfigID, &it.Type, &it.Code, &cur, &it.LatestNo)
	if cur.Valid {
		it.Current = &cur.UUID
	}
	return it, err
}

const versionCols = `v.config_version_id, v.config_id, v.version_no, v.status, v.title, v.payload, v.effective_from, v.author_id, v.reviewer_id,
	v.review_note, v.change_note, v.based_on_version_id, v.published_at, v.published_by, v.updated_at, v.version`

func scanVersion(s interface{ Scan(...any) error }) (port.ConfigVersion, error) {
	var v port.ConfigVersion
	var eff, pub sql.NullTime
	var author, reviewer, based, pubBy uuid.NullUUID
	var review, change sql.NullString
	err := s.Scan(&v.ID, &v.ConfigID, &v.No, &v.Status, &v.Title, &v.Payload, &eff, &author, &reviewer, &review, &change, &based, &pub, &pubBy,
		&v.UpdatedAt, &v.Revision)
	if err != nil {
		return v, err
	}
	v.EffectiveFrom, v.PublishedAt = timePtr(eff), timePtr(pub)
	v.AuthorID, v.ReviewerID, v.BasedOn, v.PublishedBy = uuidPtr(author), uuidPtr(reviewer), uuidPtr(based), uuidPtr(pubBy)
	v.ReviewNote, v.ChangeNote = strPtr(review), strPtr(change)
	return v, nil
}

func uuidPtr(u uuid.NullUUID) *uuid.UUID {
	if !u.Valid {
		return nil
	}
	v := u.UUID
	return &v
}

func (GameConfigStore) item(ctx context.Context, where string, args ...any) (*port.ConfigItem, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform configuration (G table game_configs, no tenant), live rows only
	it, err := scanItem(t.QueryRowContext(ctx, `SELECT `+itemCols+` FROM competition.game_configs c WHERE c.deleted_at IS NULL AND `+where, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("game config: %w", err)
	}
	return &it, nil
}

// ItemByID implements write.GameConfigStore.
func (s GameConfigStore) ItemByID(ctx context.Context, configType string, id uuid.UUID) (*port.ConfigItem, error) {
	return s.item(ctx, `c.config_type = $1 AND c.config_id = $2`, configType, id)
}

// Item implements read.GameConfigReader.
func (s GameConfigStore) Item(ctx context.Context, configType string, id uuid.UUID) (*port.ConfigItem, error) {
	return s.ItemByID(ctx, configType, id)
}

// ItemByCode implements write.GameConfigStore.
func (s GameConfigStore) ItemByCode(ctx context.Context, configType, code string) (*port.ConfigItem, error) {
	return s.item(ctx, `c.config_type = $1 AND c.code = $2`, configType, code)
}

// CreateItem implements write.GameConfigStore.
func (GameConfigStore) CreateItem(ctx context.Context, it port.ConfigItem, by uuid.UUID, now time.Time) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok platform configuration (G table game_configs, no tenant)
	_, err = t.ExecContext(ctx, `INSERT INTO competition.game_configs (config_id, created_at, created_by, updated_at, updated_by, version, config_type,
		code, current_version_id, latest_version_no, owner_scope) VALUES ($1, $2, $3, $2, $3, 1, $4, $5, NULL, 0, 'platform')`,
		it.ConfigID, now, by, it.Type, it.Code)
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" {
		return write.ErrConfigConflict
	}
	if err != nil {
		return fmt.Errorf("insert game config: %w", err)
	}
	return nil
}

// NextVersionNo implements write.GameConfigStore (the row update serializes concurrent drafts of one configuration).
func (GameConfigStore) NextVersionNo(ctx context.Context, configID uuid.UUID, by uuid.UUID, now time.Time) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var no int
	// tenancyguard:ok platform configuration (G table game_configs, no tenant), live rows only
	err = t.QueryRowContext(ctx, `UPDATE competition.game_configs SET latest_version_no = latest_version_no + 1, updated_at = $2, updated_by = $3,
		version = version + 1 WHERE config_id = $1 AND deleted_at IS NULL RETURNING latest_version_no`, configID, now, by).Scan(&no)
	if err != nil {
		return 0, fmt.Errorf("next version: %w", err)
	}
	return no, nil
}

// InsertVersion implements write.GameConfigStore.
func (GameConfigStore) InsertVersion(ctx context.Context, v port.ConfigVersion, by uuid.UUID, now time.Time) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok platform configuration (G table game_config_versions, no tenant)
	_, err = t.ExecContext(ctx, `INSERT INTO competition.game_config_versions (config_version_id, created_at, created_by, updated_at, updated_by,
		version, config_id, version_no, status, title, payload, author_id, change_note, based_on_version_id)
		VALUES ($1, $2, $3, $2, $3, 1, $4, $5, $6, $7, $8, $9, $10, $11)`,
		v.ID, now, by, v.ConfigID, v.No, v.Status, v.Title, v.Payload, v.AuthorID, v.ChangeNote, v.BasedOn)
	if err != nil {
		return fmt.Errorf("insert game config version: %w", err)
	}
	return nil
}

func (GameConfigStore) version(ctx context.Context, where string, args ...any) (*port.ConfigVersion, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform configuration (G table game_config_versions, no tenant), live rows only
	v, err := scanVersion(t.QueryRowContext(ctx, `SELECT `+versionCols+` FROM competition.game_config_versions v WHERE v.deleted_at IS NULL AND `+where,
		args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("game config version: %w", err)
	}
	return &v, nil
}

// Version implements write.GameConfigStore and read.GameConfigReader.
func (s GameConfigStore) Version(ctx context.Context, configID uuid.UUID, no int) (*port.ConfigVersion, error) {
	return s.version(ctx, `v.config_id = $1 AND v.version_no = $2`, configID, no)
}

// VersionByID implements write.GameConfigStore.
func (s GameConfigStore) VersionByID(ctx context.Context, configID, id uuid.UUID) (*port.ConfigVersion, error) {
	return s.version(ctx, `v.config_id = $1 AND v.config_version_id = $2`, configID, id)
}

func (GameConfigStore) update(ctx context.Context, q string, args ...any) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok platform configuration (G table game_config_versions, no tenant), live rows only
	res, err := t.ExecContext(ctx, q, args...)
	if err != nil {
		return false, fmt.Errorf("update game config version: %w", err)
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// UpdateDraft implements write.GameConfigStore.
func (s GameConfigStore) UpdateDraft(ctx context.Context, v port.ConfigVersion, rev int64, by uuid.UUID, now time.Time) (bool, error) {
	// tenancyguard:ok platform configuration (G table game_config_versions, no tenant), live rows only
	return s.update(ctx, `UPDATE competition.game_config_versions SET payload = $3, title = $4, status = $5, updated_at = $6, updated_by = $7,
		version = version + 1 WHERE config_version_id = $1 AND version = $2 AND deleted_at IS NULL AND status IN ('draft','rejected')`,
		v.ID, rev, v.Payload, v.Title, v.Status, now, by)
}

// SetWorkflow implements write.GameConfigStore.
func (s GameConfigStore) SetWorkflow(ctx context.Context, v port.ConfigVersion, rev int64, by uuid.UUID, now time.Time) (bool, error) {
	// tenancyguard:ok platform configuration (G table game_config_versions, no tenant), live rows only
	return s.update(ctx, `UPDATE competition.game_config_versions SET status = $3, reviewer_id = $4, review_note = $5, updated_at = $6,
		updated_by = $7, version = version + 1 WHERE config_version_id = $1 AND version = $2 AND deleted_at IS NULL`,
		v.ID, rev, v.Status, v.ReviewerID, v.ReviewNote, now, by)
}

// Publish implements write.GameConfigStore (approved → published: the trigger freezes the version from now on, R11).
func (s GameConfigStore) Publish(ctx context.Context, v port.ConfigVersion, rev int64, by uuid.UUID, now time.Time) (bool, error) {
	// tenancyguard:ok platform configuration (G table game_config_versions, no tenant), live rows only
	return s.update(ctx, `UPDATE competition.game_config_versions SET status = 'published', effective_from = $3, published_at = $4,
		published_by = $5, change_note = COALESCE($6, change_note), updated_at = $4, updated_by = $5, version = version + 1
		WHERE config_version_id = $1 AND version = $2 AND deleted_at IS NULL AND status = 'approved'`,
		v.ID, rev, v.EffectiveFrom, now, by, v.ChangeNote)
}

// MovePointer implements write.GameConfigStore.
func (GameConfigStore) MovePointer(ctx context.Context, configID uuid.UUID, to *uuid.UUID, by uuid.UUID, now time.Time) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok platform configuration (G table game_configs, no tenant), live rows only
	_, err = t.ExecContext(ctx, `UPDATE competition.game_configs SET current_version_id = $2, updated_at = $3, updated_by = $4, version = version + 1
		WHERE config_id = $1 AND deleted_at IS NULL`, configID, to, now, by)
	if err != nil {
		return fmt.Errorf("move pointer: %w", err)
	}
	return nil
}

// InsertRelease implements write.GameConfigStore (append-only; created_at is the insertion time ordering the timeline).
func (GameConfigStore) InsertRelease(ctx context.Context, r port.ConfigRelease) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok platform configuration (GA table content_releases, append-only, no tenant)
	_, err = t.ExecContext(ctx, `INSERT INTO competition.content_releases (release_id, created_at, created_by, config_id, action, from_version_id,
		to_version_id, reason, released_by, released_at, effective_from) VALUES ($1, clock_timestamp(), $2, $3, $4, $5, $6, $7, $2, $8, $9)`,
		r.ID, r.By, r.ConfigID, r.Action, r.From, r.To, r.Reason, r.At, r.EffectiveFrom)
	if err != nil {
		return fmt.Errorf("insert release: %w", err)
	}
	return nil
}

// SeasonStates implements write.GameConfigStore.
func (GameConfigStore) SeasonStates(ctx context.Context) ([]gameconfig.SeasonState, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform seasons (G table seasons, no tenant), live rows only
	rows, err := t.QueryContext(ctx, `SELECT code, status, starts_at, ends_at FROM competition.seasons WHERE deleted_at IS NULL ORDER BY starts_at`)
	if err != nil {
		return nil, fmt.Errorf("seasons: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []gameconfig.SeasonState
	for rows.Next() {
		var s gameconfig.SeasonState
		if err := rows.Scan(&s.Code, &s.Status, &s.StartsAt, &s.EndsAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// effectiveSQL selects the version in effect of (config_type, code) at $3 by the release timeline (ADR-0043): the latest
// release by insertion whose effective time — content_releases.effective_from, else the effective_from of its version
// (seed releases) — is not after $3; an unpublish release has no version (nothing in effect).
// tenancyguard:ok platform configuration (G/GA tables content_releases/game_configs/game_config_versions, no tenant), live rows only
const effectiveSQL = `SELECT r.to_version_id, v.payload FROM competition.content_releases r
	JOIN competition.game_configs c ON c.config_id = r.config_id AND c.deleted_at IS NULL
	LEFT JOIN competition.game_config_versions v ON v.config_version_id = r.to_version_id AND v.deleted_at IS NULL
	WHERE c.config_type = $1 AND c.code = $2 AND COALESCE(r.effective_from, v.effective_from, '-infinity'::timestamptz) <= $3
	ORDER BY r.created_at DESC, r.release_id DESC LIMIT 1`

// effective returns the version in effect of (type, default) at at (nil when none or withdrawn).
func effective(ctx context.Context, typ string, at time.Time) (*uuid.UUID, []byte, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, nil, err
	}
	var id uuid.NullUUID
	var payload []byte
	// tenancyguard:ok platform configuration (G/GA tables content_releases/game_configs/game_config_versions, no tenant), live rows only
	err = t.QueryRowContext(ctx, effectiveSQL, typ, gameconfig.DefaultCode, at).Scan(&id, &payload)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !id.Valid) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("%s config in effect: %w", typ, err)
	}
	return &id.UUID, payload, nil
}

// EffectivePayload implements write.ConfigApplyStore.
func (GameConfigStore) EffectivePayload(ctx context.Context, configType string, at time.Time) (*uuid.UUID, []byte, error) {
	return effective(ctx, configType, at)
}

// UpsertSeason implements write.ConfigApplyStore (SystemPlatformScope, competition_platform).
func (GameConfigStore) UpsertSeason(ctx context.Context, s write.SeasonSync, version uuid.UUID, now time.Time) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	// tenancyguard:ok platform seasons (G table seasons, no tenant), live rows only
	res, err := t.ExecContext(ctx, `UPDATE competition.seasons SET
		name = $2, theme_code = $3,
		starts_at = CASE WHEN status = 'scheduled' THEN $4 ELSE starts_at END,
		ends_at = $5, config_version_id = $6, updated_at = $7, version = version + 1
		WHERE code = $1 AND deleted_at IS NULL AND status <> 'ended'
		  AND (name, COALESCE(theme_code, ''), starts_at, ends_at) IS DISTINCT FROM
		      ($2, COALESCE($3, ''), CASE WHEN status = 'scheduled' THEN $4 ELSE starts_at END, $5)`,
		s.Code, s.Name, s.Theme, s.StartsAt, s.EndsAt, version, now)
	if err != nil {
		return false, fmt.Errorf("update season: %w", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return true, nil
	}
	// tenancyguard:ok platform seasons (G table seasons, no tenant)
	res, err = t.ExecContext(ctx, `INSERT INTO competition.seasons (season_id, created_at, updated_at, version, code, name, theme_code, starts_at,
		ends_at, status, rules, config_version_id) SELECT $1::uuid, $7::timestamptz, $7::timestamptz, 1, $2::text, $3::text, $4::text, $5::timestamptz, $6::timestamptz, 'scheduled', '{}'::jsonb, $8::uuid
		WHERE NOT EXISTS (SELECT 1 FROM competition.seasons WHERE code = $2::text AND deleted_at IS NULL)
		  AND NOT EXISTS (SELECT 1 FROM competition.seasons WHERE season_id = $1::uuid)`,
		s.ID, s.Code, s.Name, s.Theme, s.StartsAt, s.EndsAt, now, version)
	if err != nil {
		return false, fmt.Errorf("insert season: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Configs implements read.GameConfigReader: the newest version of each configuration, keyset by config_id.
func (GameConfigStore) Configs(ctx context.Context, configType, status string, after *uuid.UUID, limit int) ([]read.ConfigRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform configuration (G tables game_configs/game_config_versions, no tenant), live rows only
	rows, err := t.QueryContext(ctx, `SELECT `+itemCols+`, `+versionCols+` FROM competition.game_configs c
		JOIN competition.game_config_versions v ON v.config_id = c.config_id AND v.version_no = c.latest_version_no AND v.deleted_at IS NULL
		WHERE c.config_type = $1 AND c.deleted_at IS NULL AND ($2 = '' OR v.status = $2) AND ($3::uuid IS NULL OR c.config_id > $3)
		ORDER BY c.config_id LIMIT $4`, configType, status, after, limit+1)
	if err != nil {
		return nil, fmt.Errorf("list game configs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.ConfigRow
	for rows.Next() {
		var r read.ConfigRow
		var cur, author, reviewer, based, pubBy uuid.NullUUID
		var eff, pub sql.NullTime
		var review, change sql.NullString
		v := &r.Version
		if err := rows.Scan(&r.Item.ConfigID, &r.Item.Type, &r.Item.Code, &cur, &r.Item.LatestNo,
			&v.ID, &v.ConfigID, &v.No, &v.Status, &v.Title, &v.Payload, &eff, &author, &reviewer, &review, &change, &based, &pub, &pubBy,
			&v.UpdatedAt, &v.Revision); err != nil {
			return nil, err
		}
		r.Item.Current = uuidPtr(cur)
		v.EffectiveFrom, v.PublishedAt = timePtr(eff), timePtr(pub)
		v.AuthorID, v.ReviewerID, v.BasedOn, v.PublishedBy = uuidPtr(author), uuidPtr(reviewer), uuidPtr(based), uuidPtr(pubBy)
		v.ReviewNote, v.ChangeNote = strPtr(review), strPtr(change)
		out = append(out, r)
	}
	return out, rows.Err()
}

// Versions implements read.GameConfigReader (newest first, keyset by version number).
func (GameConfigStore) Versions(ctx context.Context, configID uuid.UUID, before, limit int) ([]port.ConfigVersion, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform configuration (G table game_config_versions, no tenant), live rows only
	rows, err := t.QueryContext(ctx, `SELECT `+versionCols+` FROM competition.game_config_versions v
		WHERE v.config_id = $1 AND v.deleted_at IS NULL AND ($2 = 0 OR v.version_no < $2) ORDER BY v.version_no DESC LIMIT $3`,
		configID, before, limit+1)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.ConfigVersion
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// VersionNos implements read.GameConfigReader.
func (GameConfigStore) VersionNos(ctx context.Context, configID uuid.UUID) (map[uuid.UUID]int, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform configuration (G table game_config_versions, no tenant), live rows only; one config's versions
	rows, err := t.QueryContext(ctx, `SELECT config_version_id, version_no FROM competition.game_config_versions
		WHERE config_id = $1 AND deleted_at IS NULL`, configID)
	if err != nil {
		return nil, fmt.Errorf("version numbers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[uuid.UUID]int{}
	for rows.Next() {
		var id uuid.UUID
		var no int
		if err := rows.Scan(&id, &no); err != nil {
			return nil, err
		}
		out[id] = no
	}
	return out, rows.Err()
}

// Releases implements read.GameConfigReader (newest first by insertion; keyset (created_at, release_id) of the cursor
// release).
func (GameConfigStore) Releases(ctx context.Context, configID uuid.UUID, after *uuid.UUID, limit int) ([]port.ConfigRelease, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform configuration (GA table content_releases, append-only, no tenant)
	rows, err := t.QueryContext(ctx, `SELECT r.release_id, r.config_id, r.action, r.from_version_id, r.to_version_id, r.reason, r.released_by,
		r.released_at, COALESCE(r.effective_from, v.effective_from, r.released_at)
		FROM competition.content_releases r
		LEFT JOIN competition.game_config_versions v ON v.config_version_id = r.to_version_id
		WHERE r.config_id = $1 AND ($2::uuid IS NULL OR (r.created_at, r.release_id) <
			(SELECT a.created_at, a.release_id FROM competition.content_releases a WHERE a.release_id = $2))
		ORDER BY r.created_at DESC, r.release_id DESC LIMIT $3`, configID, after, limit+1)
	if err != nil {
		return nil, fmt.Errorf("list releases: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.ConfigRelease
	for rows.Next() {
		var r port.ConfigRelease
		var from, to, by uuid.NullUUID
		var reason sql.NullString
		if err := rows.Scan(&r.ID, &r.ConfigID, &r.Action, &from, &to, &reason, &by, &r.At, &r.EffectiveFrom); err != nil {
			return nil, err
		}
		r.From, r.To, r.Reason = uuidPtr(from), uuidPtr(to), strPtr(reason)
		if by.Valid {
			r.By = by.UUID
		}
		r.At, r.EffectiveFrom = r.At.UTC(), r.EffectiveFrom.UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}
