package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
)

// Features implements read.FeatureReader (F118): explicit SQL, tenant_id and deleted_at on every tenant table (R27); the
// sticker/GIF library is a G table set of the platform (no tenant), seen by a tenant through
// infra.tenant_resource_assignments (sticker packs, ADR-0029).
type Features struct{}

var _ read.FeatureReader = Features{}

// MessageByID implements read.FeatureReader (both message tables; a message lives in exactly one).
func (Features) MessageByID(ctx context.Context, tenant, id uuid.UUID) (*read.StoredMessage, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	for _, gameplay := range []bool{false, true} {
		table := "messages"
		if gameplay {
			table = "gameplay_messages"
		}
		// tenancyguard:ok one message of the session tenant, live rows only (removed ones are read for their state)
		row := t.QueryRowContext(ctx, `SELECT `+messageColumns+` FROM chat.`+table+` m
			WHERE m.tenant_id = $1 AND m.message_id = $2 AND m.deleted_at IS NULL LIMIT 1`, tenant, id)
		m, err := scanMessage(row, tenant, gameplay)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			if errors.Is(errors.Unwrap(err), sql.ErrNoRows) {
				continue
			}
			return nil, err
		}
		return &m, nil
	}
	return nil, nil
}

// MessagesByID implements read.FeatureReader.
func (Features) MessagesByID(ctx context.Context, tenant, channel uuid.UUID, gameplay bool, ids []uuid.UUID) (map[uuid.UUID]read.StoredMessage, error) {
	out := map[uuid.UUID]read.StoredMessage{}
	if len(ids) == 0 {
		return out, nil
	}
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	table := "messages"
	if gameplay {
		table = "gameplay_messages"
	}
	// tenancyguard:ok messages of one channel of the session tenant, live rows only
	rows, err := t.QueryContext(ctx, `SELECT `+messageColumns+` FROM chat.`+table+` m
		WHERE m.tenant_id = $1 AND m.channel_id = $2 AND m.message_id = ANY($3::uuid[]) AND m.deleted_at IS NULL`,
		tenant, channel, idStrings(ids))
	if err != nil {
		return nil, fmt.Errorf("chat: messages by id: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		m, err := scanMessage(rows, tenant, gameplay)
		if err != nil {
			return nil, err
		}
		out[m.Envelope.MessageID] = m
	}
	return out, rows.Err()
}

// Pins implements read.FeatureReader.
func (Features) Pins(ctx context.Context, tenant, channel uuid.UUID) ([]read.PinRow, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok live pins of one channel of the session tenant
	rows, err := t.QueryContext(ctx, `SELECT p.message_id, p.pinned_by, p.pinned_at FROM chat.pinned_messages p
		WHERE p.tenant_id = $1 AND p.channel_id = $2 AND p.deleted_at IS NULL ORDER BY p.pinned_at DESC LIMIT 10`, tenant, channel)
	if err != nil {
		return nil, fmt.Errorf("chat: pins: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.PinRow
	for rows.Next() {
		var p read.PinRow
		if err := rows.Scan(&p.MessageID, &p.PinnedBy, &p.PinnedAt); err != nil {
			return nil, fmt.Errorf("chat: pins: %w", err)
		}
		p.PinnedAt = p.PinnedAt.UTC()
		out = append(out, p)
	}
	return out, rows.Err()
}

// Reactions implements read.FeatureReader.
func (Features) Reactions(ctx context.Context, tenant uuid.UUID, ids []uuid.UUID) ([]read.ReactionRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok live reactions of messages of the session tenant
	rows, err := t.QueryContext(ctx, `SELECT r.message_id, r.user_id, r.emoji_code FROM chat.message_reactions r
		WHERE r.tenant_id = $1 AND r.message_id = ANY($2::uuid[]) AND r.deleted_at IS NULL ORDER BY r.created_at`, tenant, idStrings(ids))
	if err != nil {
		return nil, fmt.Errorf("chat: reactions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.ReactionRow
	for rows.Next() {
		var r read.ReactionRow
		if err := rows.Scan(&r.MessageID, &r.UserID, &r.Emoji); err != nil {
			return nil, fmt.Errorf("chat: reactions: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Cursors implements read.FeatureReader.
func (Features) Cursors(ctx context.Context, tenant, channel uuid.UUID) ([]read.MemberCursor, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok active members of one channel of the session tenant, live rows only
	rows, err := t.QueryContext(ctx, `SELECT m.user_id, m.last_delivered_seq, m.last_read_seq, m.unread_mentions FROM chat.channel_members m
		WHERE m.tenant_id = $1 AND m.channel_id = $2 AND m.deleted_at IS NULL AND m.left_at IS NULL`, tenant, channel)
	if err != nil {
		return nil, fmt.Errorf("chat: cursors: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.MemberCursor
	for rows.Next() {
		var c read.MemberCursor
		if err := rows.Scan(&c.UserID, &c.Delivered, &c.Read, &c.Mentions); err != nil {
			return nil, fmt.Errorf("chat: cursors: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// StickerPacks implements read.FeatureReader: published, live packs assigned to the tenant (active assignment).
func (Features) StickerPacks(ctx context.Context, tenant, student uuid.UUID) ([]read.StickerPackRow, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform packs (G) assigned to the session tenant (assignment tenant_id), live rows only
	rows, err := t.QueryContext(ctx, `SELECT p.pack_id, p.code, p.name_vi, p.name_en, p.unlock_rule IS NOT NULL,
			EXISTS (SELECT 1 FROM ref.sticker_unlocks u WHERE u.tenant_id = $1 AND u.student_id = $2 AND u.pack_id = p.pack_id),
			p.unlock_hint_vi, p.unlock_hint_en
		FROM chat.sticker_packs p
		WHERE p.status = 'published' AND p.deleted_at IS NULL AND EXISTS (SELECT 1 FROM infra.tenant_resource_assignments a
			WHERE a.tenant_id = $1 AND a.resource_type = 'sticker_pack' AND a.resource_id = p.pack_id AND a.deleted_at IS NULL)
		ORDER BY p.code`, tenant, student)
	if err != nil {
		return nil, fmt.Errorf("chat: sticker packs: %w", err)
	}
	out, err := scanPacks(rows)
	if err != nil {
		return nil, err
	}
	for i := range out {
		if out[i].Stickers, err = stickersOf(ctx, t, out[i].PackID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func scanPacks(rows *sql.Rows) ([]read.StickerPackRow, error) {
	defer func() { _ = rows.Close() }()
	var out []read.StickerPackRow
	for rows.Next() {
		var p read.StickerPackRow
		var nameEN, hintVI, hintEN sql.NullString
		if err := rows.Scan(&p.PackID, &p.Code, &p.NameVI, &nameEN, &p.Locked, &p.Unlocked, &hintVI, &hintEN); err != nil {
			return nil, fmt.Errorf("chat: sticker packs: %w", err)
		}
		p.NameEN, p.HintVI, p.HintEN = nullString(nameEN), nullString(hintVI), nullString(hintEN)
		p.Unlocked = !p.Locked || p.Unlocked
		out = append(out, p)
	}
	return out, rows.Err()
}

func stickersOf(ctx context.Context, t *sql.Tx, pack uuid.UUID) ([]read.Sticker, error) {
	// tenancyguard:ok stickers of a platform pack (G table), live rows only
	rows, err := t.QueryContext(ctx, `SELECT s.sticker_id, s.asset_key, s.alt_text_vi, s.alt_text_en FROM chat.stickers s
		WHERE s.pack_id = $1 AND s.deleted_at IS NULL ORDER BY s.sort_order, s.sticker_id`, pack)
	if err != nil {
		return nil, fmt.Errorf("chat: stickers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.Sticker
	for rows.Next() {
		var s read.Sticker
		var en sql.NullString
		if err := rows.Scan(&s.StickerID, &s.AssetKey, &s.AltVI, &en); err != nil {
			return nil, fmt.Errorf("chat: stickers: %w", err)
		}
		s.AltEN = nullString(en)
		out = append(out, s)
	}
	return out, rows.Err()
}

// StickerAvailable implements read.FeatureReader (F118-AC-12).
func (Features) StickerAvailable(ctx context.Context, tenant, student, sticker uuid.UUID) (bool, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return false, err
	}
	var ok bool
	// tenancyguard:ok platform sticker (G) of a pack assigned to the session tenant; unlock of the student of the tenant
	err = t.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM chat.stickers s JOIN chat.sticker_packs p ON p.pack_id = s.pack_id
		WHERE s.sticker_id = $3 AND s.deleted_at IS NULL AND p.deleted_at IS NULL AND p.status = 'published'
			AND EXISTS (SELECT 1 FROM infra.tenant_resource_assignments a WHERE a.tenant_id = $1 AND a.resource_type = 'sticker_pack'
				AND a.resource_id = p.pack_id AND a.deleted_at IS NULL)
			AND (p.unlock_rule IS NULL OR EXISTS (SELECT 1 FROM ref.sticker_unlocks u WHERE u.tenant_id = $1 AND u.student_id = $2
				AND u.pack_id = p.pack_id)))`, tenant, student, sticker).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("chat: sticker available: %w", err)
	}
	return ok, nil
}

// Gifs implements read.FeatureReader.
func (Features) Gifs(ctx context.Context, tag string, after *read.GifCursor, limit int) ([]read.GifRow, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	var at any
	var id any
	if after != nil {
		at, id = after.CreatedAt, after.GifID
	}
	// tenancyguard:ok platform GIF library (G table, no tenant), published live rows only
	rows, err := t.QueryContext(ctx, `SELECT g.gif_id, g.asset_key, g.preview_key, g.width, g.height, g.created_at,
			coalesce((SELECT array_to_string(array_agg(DISTINCT x.tag ORDER BY x.tag), ',') FROM chat.gif_tags x
				WHERE x.gif_id = g.gif_id AND x.deleted_at IS NULL), '')
		FROM chat.gif_assets g
		WHERE g.status = 'published' AND g.deleted_at IS NULL
			AND ($1 = '' OR EXISTS (SELECT 1 FROM chat.gif_tags tg WHERE tg.gif_id = g.gif_id AND tg.tag = $1 AND tg.deleted_at IS NULL))
			AND ($2::timestamptz IS NULL OR (g.created_at, g.gif_id) < ($2::timestamptz, $3::uuid))
		ORDER BY g.created_at DESC, g.gif_id DESC LIMIT $4`, tag, at, id, limit)
	if err != nil {
		return nil, fmt.Errorf("chat: gifs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.GifRow
	for rows.Next() {
		var g read.GifRow
		var tags string
		if err := rows.Scan(&g.GifID, &g.AssetKey, &g.PreviewKey, &g.Width, &g.Height, &g.CreatedAt, &tags); err != nil {
			return nil, fmt.Errorf("chat: gifs: %w", err)
		}
		g.CreatedAt = g.CreatedAt.UTC()
		g.Tags = splitTags(tags)
		out = append(out, g)
	}
	return out, rows.Err()
}

func splitTags(s string) []string {
	out := []string{}
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	return out
}

// GifAvailable implements read.FeatureReader (F118-AC-13).
func (Features) GifAvailable(ctx context.Context, gif uuid.UUID) (bool, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return false, err
	}
	var ok bool
	// tenancyguard:ok platform GIF library (G table, no tenant), published live rows only
	if err := t.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM chat.gif_assets g WHERE g.gif_id = $1 AND g.status = 'published'
		AND g.deleted_at IS NULL)`, gif).Scan(&ok); err != nil {
		return false, fmt.Errorf("chat: gif available: %w", err)
	}
	return ok, nil
}
