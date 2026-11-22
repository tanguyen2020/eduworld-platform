package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/chatasset"
)

// Assets implements write.AssetRepository and the reads of the Game Studio library: platform G tables (no tenant, no
// RLS) written by chat_platform under PlatformScope; soft delete only (R21); a published version never changes (trigger
// chat.f_asset_version_immutable, R31).
type Assets struct{}

var _ write.AssetRepository = Assets{}

const assetVersionColumns = `v.version_id, v.config_type, v.config_id, v.version, v.status, coalesce(v.change_note, ''), v.payload,
	v.created_by, v.reviewed_by, v.reviewed_at, v.published_by, v.published_at, v.updated_at`

func scanAssetVersion(r interface{ Scan(dest ...any) error }) (port.AssetVersion, error) {
	var v port.AssetVersion
	var typ, status string
	var reviewer, publisher uuid.NullUUID
	var reviewed, published sql.NullTime
	if err := r.Scan(&v.VersionID, &typ, &v.ConfigID, &v.Version, &status, &v.Title, &v.Payload, &v.AuthorID, &reviewer, &reviewed,
		&publisher, &published, &v.UpdatedAt); err != nil {
		return v, err
	}
	v.Type, v.Status = chatasset.Type(typ), chatasset.Status(status)
	v.ReviewerID, v.PublishedBy = nullUUID(reviewer), nullUUID(publisher)
	v.ReviewedAt, v.PublishedAt, v.UpdatedAt = nullTime(reviewed), nullTime(published), v.UpdatedAt.UTC()
	return v, nil
}

// CreateItem implements write.AssetRepository.
func (Assets) CreateItem(ctx context.Context, v port.AssetVersion, pack *chatasset.Pack) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	if v.Type == chatasset.TypeStickerPacks {
		// tenancyguard:ok platform library (G table, tenant_id always NULL)
		if _, err := t.ExecContext(ctx, `INSERT INTO chat.sticker_packs (pack_id, code, name_vi, name_en, status, created_by)
			VALUES ($1, $2, $3, NULLIF($4, ''), 'draft', $5)`, v.ConfigID, pack.Code, pack.NameVI, pack.NameEN, v.AuthorID); err != nil {
			return fmt.Errorf("chat assets: create pack: %w", err)
		}
	} else {
		// tenancyguard:ok platform library (G table, tenant_id always NULL)
		if _, err := t.ExecContext(ctx, `INSERT INTO chat.gif_assets (gif_id, status, created_by) VALUES ($1, 'draft', $2)`,
			v.ConfigID, v.AuthorID); err != nil {
			return fmt.Errorf("chat assets: create gif: %w", err)
		}
	}
	return Assets{}.InsertVersion(ctx, v)
}

// CodeTaken implements write.AssetRepository.
func (Assets) CodeTaken(ctx context.Context, code string, id uuid.UUID) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	var taken bool
	// tenancyguard:ok platform library (G table), live rows only
	if err := t.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM chat.sticker_packs WHERE code = $1 AND pack_id <> $2 AND deleted_at IS NULL)`,
		code, id).Scan(&taken); err != nil {
		return false, fmt.Errorf("chat assets: code: %w", err)
	}
	return taken, nil
}

// Version implements write.AssetRepository.
func (Assets) Version(ctx context.Context, typ chatasset.Type, id uuid.UUID, version int) (*port.AssetVersion, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform library versions (G table, append-only once published)
	v, err := scanAssetVersion(t.QueryRowContext(ctx, `SELECT `+assetVersionColumns+` FROM chat.chat_asset_versions v
		WHERE v.config_type = $1 AND v.config_id = $2 AND v.version = $3 FOR UPDATE`, string(typ), id, version))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat assets: version: %w", err)
	}
	return &v, nil
}

// Latest implements write.AssetRepository.
func (Assets) Latest(ctx context.Context, typ chatasset.Type, id uuid.UUID) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	// tenancyguard:ok platform library versions (G table)
	if err := t.QueryRowContext(ctx, `SELECT coalesce(max(v.version), 0) FROM chat.chat_asset_versions v WHERE v.config_type = $1
		AND v.config_id = $2`, string(typ), id).Scan(&n); err != nil {
		return 0, fmt.Errorf("chat assets: latest: %w", err)
	}
	return n, nil
}

// Published implements write.AssetRepository.
func (Assets) Published(ctx context.Context, typ chatasset.Type, id uuid.UUID) (*port.AssetVersion, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform library versions (G table)
	v, err := scanAssetVersion(t.QueryRowContext(ctx, `SELECT `+assetVersionColumns+` FROM chat.chat_asset_versions v
		WHERE v.config_type = $1 AND v.config_id = $2 AND v.status = 'published' ORDER BY v.version DESC LIMIT 1 FOR UPDATE`, string(typ), id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat assets: published: %w", err)
	}
	return &v, nil
}

// InsertVersion implements write.AssetRepository.
func (Assets) InsertVersion(ctx context.Context, v port.AssetVersion) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok platform library versions (G table)
	if _, err := t.ExecContext(ctx, `INSERT INTO chat.chat_asset_versions (version_id, config_type, config_id, version, status, payload,
		change_note, created_by, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6::jsonb, NULLIF($7, ''), $8, $9, $9)`,
		v.VersionID, string(v.Type), v.ConfigID, v.Version, string(v.Status), string(v.Payload), v.Title, v.AuthorID, v.UpdatedAt); err != nil {
		return fmt.Errorf("chat assets: insert version: %w", err)
	}
	return nil
}

// SaveVersion implements write.AssetRepository.
func (Assets) SaveVersion(ctx context.Context, v port.AssetVersion) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok platform library versions (G table); the trigger refuses any change of a published version
	if _, err := t.ExecContext(ctx, `UPDATE chat.chat_asset_versions SET status = $2, payload = $3::jsonb, change_note = NULLIF($4, ''),
		created_by = $5, reviewed_by = $6, reviewed_at = $7, published_by = $8, published_at = $9, updated_at = $10 WHERE version_id = $1`,
		v.VersionID, string(v.Status), string(v.Payload), v.Title, v.AuthorID, v.ReviewerID, v.ReviewedAt, v.PublishedBy, v.PublishedAt,
		v.UpdatedAt); err != nil {
		return fmt.Errorf("chat assets: save version: %w", err)
	}
	return nil
}

// Publish implements write.AssetRepository.
func (Assets) Publish(ctx context.Context, v port.AssetVersion, prev *port.AssetVersion, r port.AssetRelease) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	if prev != nil && prev.VersionID != v.VersionID {
		// tenancyguard:ok platform library versions (G table): published → archived is the only change allowed
		if _, err := t.ExecContext(ctx, `UPDATE chat.chat_asset_versions SET status = 'archived', updated_at = $2 WHERE version_id = $1`,
			prev.VersionID, r.At); err != nil {
			return fmt.Errorf("chat assets: archive previous: %w", err)
		}
	}
	if err := (Assets{}).SaveVersion(ctx, v); err != nil {
		return err
	}
	switch v.Type {
	case chatasset.TypeStickerPacks:
		if err := publishPack(ctx, t, v, r.At); err != nil {
			return err
		}
	default:
		if err := publishGif(ctx, t, v, r.At); err != nil {
			return err
		}
	}
	var from, to any
	if r.FromVersion != nil {
		from = *r.FromVersion
	}
	if r.ToVersion != nil {
		to = *r.ToVersion
	}
	// tenancyguard:ok platform library releases (G table, append-only)
	if _, err := t.ExecContext(ctx, `INSERT INTO chat.chat_asset_releases (release_id, config_type, config_id, action, from_version_id,
		to_version_id, reason, actor_id, created_at) VALUES ($1, $2::varchar, $3, $4,
			(SELECT version_id FROM chat.chat_asset_versions WHERE config_type = $2::varchar AND config_id = $3 AND version = $5::int),
			(SELECT version_id FROM chat.chat_asset_versions WHERE config_type = $2::varchar AND config_id = $3 AND version = $6::int), NULLIF($7::varchar, ''), $8, $9)`,
		r.ReleaseID, string(r.Type), r.ConfigID, r.Action, from, to, r.Reason, r.ActorID, r.At); err != nil {
		return fmt.Errorf("chat assets: release: %w", err)
	}
	return nil
}

func publishPack(ctx context.Context, t *sql.Tx, v port.AssetVersion, at time.Time) error {
	var p chatasset.Pack
	if err := json.Unmarshal(v.Payload, &p); err != nil {
		return err
	}
	var rule any
	if p.UnlockRule != nil {
		b, _ := json.Marshal(p.UnlockRule)
		rule = string(b)
	}
	// tenancyguard:ok platform library (G table): the pack takes the state of the published version
	if _, err := t.ExecContext(ctx, `UPDATE chat.sticker_packs SET code = $2, name_vi = $3, name_en = NULLIF($4, ''), unlock_rule = $5::jsonb,
		unlock_hint_vi = NULLIF($6, ''), unlock_hint_en = NULLIF($7, ''), status = 'published', current_version_id = $8, updated_at = $9
		WHERE pack_id = $1 AND deleted_at IS NULL`, v.ConfigID, p.Code, p.NameVI, p.NameEN, rule, p.HintVI, p.HintEN, v.VersionID, at); err != nil {
		return fmt.Errorf("chat assets: publish pack: %w", err)
	}
	keep := make([]string, 0, len(p.Stickers))
	for _, s := range p.Stickers {
		keep = append(keep, s.StickerID.String())
		// tenancyguard:ok platform library (G table); a sticker of a published version is immutable (new ids for new files)
		if _, err := t.ExecContext(ctx, `INSERT INTO chat.stickers (sticker_id, pack_id, asset_key, content_type, size_bytes, alt_text_vi,
			alt_text_en, sort_order) VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8)
			ON CONFLICT (sticker_id) DO UPDATE SET alt_text_vi = EXCLUDED.alt_text_vi, alt_text_en = EXCLUDED.alt_text_en,
				sort_order = EXCLUDED.sort_order, deleted_at = NULL, deleted_by = NULL
			WHERE chat.stickers.asset_key = EXCLUDED.asset_key AND chat.stickers.pack_id = EXCLUDED.pack_id`,
			s.StickerID, v.ConfigID, s.AssetKey, s.ContentType, s.SizeBytes, s.AltVI, s.AltEN, s.SortOrder); err != nil {
			return fmt.Errorf("chat assets: sticker: %w", err)
		}
	}
	// tenancyguard:ok platform library (G table): stickers left out of the version are soft-deleted (old messages keep the id)
	if _, err := t.ExecContext(ctx, `UPDATE chat.stickers SET deleted_at = $3, deleted_by = $4 WHERE pack_id = $1
		AND NOT (sticker_id = ANY($2::uuid[])) AND deleted_at IS NULL`, v.ConfigID, keep, at, v.PublishedBy); err != nil {
		return fmt.Errorf("chat assets: stickers: %w", err)
	}
	return nil
}

func publishGif(ctx context.Context, t *sql.Tx, v port.AssetVersion, at time.Time) error {
	var g chatasset.Gif
	if err := json.Unmarshal(v.Payload, &g); err != nil {
		return err
	}
	// tenancyguard:ok platform library (G table): the GIF takes the state of the published version
	if _, err := t.ExecContext(ctx, `UPDATE chat.gif_assets SET asset_key = $2, preview_key = $3, size_bytes = $4, width = $5, height = $6,
		status = 'published', current_version_id = $7, created_by = $8, reviewed_by = $9, reviewed_at = $10, updated_at = $11
		WHERE gif_id = $1 AND deleted_at IS NULL`, v.ConfigID, g.AssetKey, g.PreviewKey, g.SizeBytes, g.Width, g.Height, v.VersionID,
		v.AuthorID, v.ReviewerID, v.ReviewedAt, at); err != nil {
		return fmt.Errorf("chat assets: publish gif: %w", err)
	}
	type tag struct{ t, l string }
	var tags []tag
	for _, x := range g.TagsVI {
		tags = append(tags, tag{x, "vi"})
	}
	for _, x := range g.TagsEN {
		tags = append(tags, tag{x, "en"})
	}
	// tenancyguard:ok platform library tags (G table): soft delete, then the tags of the version are live again
	if _, err := t.ExecContext(ctx, `UPDATE chat.gif_tags SET deleted_at = $2 WHERE gif_id = $1 AND deleted_at IS NULL`, v.ConfigID, at); err != nil {
		return fmt.Errorf("chat assets: tags: %w", err)
	}
	for _, x := range tags {
		// tenancyguard:ok platform library tags (G table)
		if _, err := t.ExecContext(ctx, `INSERT INTO chat.gif_tags (gif_id, tag, locale) VALUES ($1, $2, $3)
			ON CONFLICT (gif_id, tag, locale) DO UPDATE SET deleted_at = NULL`, v.ConfigID, x.t, x.l); err != nil {
			return fmt.Errorf("chat assets: tag: %w", err)
		}
	}
	return nil
}

// ListVersions returns the versions of the library, newest first: of one type, optionally of one item or status.
func (Assets) ListVersions(ctx context.Context, typ chatasset.Type, id *uuid.UUID, status string, limit int) ([]port.AssetVersion, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	var item any
	if id != nil {
		item = *id
	}
	// tenancyguard:ok platform library versions (G table)
	rows, err := t.QueryContext(ctx, `SELECT `+assetVersionColumns+` FROM chat.chat_asset_versions v
		WHERE v.config_type = $1 AND ($2::uuid IS NULL OR v.config_id = $2) AND ($3 = '' OR v.status = $3)
		ORDER BY v.updated_at DESC, v.version DESC LIMIT $4`, string(typ), item, status, limit)
	if err != nil {
		return nil, fmt.Errorf("chat assets: list: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.AssetVersion
	for rows.Next() {
		v, err := scanAssetVersion(rows)
		if err != nil {
			return nil, fmt.Errorf("chat assets: list: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// GetVersion returns one version (read side), nil when unknown.
func (Assets) GetVersion(ctx context.Context, typ chatasset.Type, id uuid.UUID, version int) (*port.AssetVersion, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform library versions (G table)
	v, err := scanAssetVersion(t.QueryRowContext(ctx, `SELECT `+assetVersionColumns+` FROM chat.chat_asset_versions v
		WHERE v.config_type = $1 AND v.config_id = $2 AND v.version = $3`, string(typ), id, version))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat assets: version: %w", err)
	}
	return &v, nil
}
