package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

func anyTx(ctx context.Context) (*sql.Tx, error) { return readTx(ctx) }

func idStrings(us []uuid.UUID) []string {
	out := make([]string, len(us))
	for i, u := range us {
		out[i] = u.String()
	}
	return out
}

func nullUUID(n uuid.NullUUID) *uuid.UUID {
	if !n.Valid {
		return nil
	}
	v := n.UUID
	return &v
}

func nullTime(n sql.NullTime) *time.Time {
	if !n.Valid {
		return nil
	}
	v := n.Time.UTC()
	return &v
}

func nullString(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	v := n.String
	return &v
}

// ChatReader implements read.ChatReader (explicit SQL, tenant_id and deleted_at on every business table, R27).
type ChatReader struct{}

var _ read.ChatReader = ChatReader{}

func scanChannel(row interface{ Scan(...any) error }, extra ...any) (channel.Channel, error) {
	var c channel.Channel
	var school uuid.NullUUID
	var scope []byte
	var closed sql.NullTime
	dest := append([]any{&c.ID, &c.TenantID, &school, &c.Type, &c.IsGameplay, &scope, &c.Status, &c.LastSeq, &c.OpenedAt, &closed}, extra...)
	if err := row.Scan(dest...); err != nil {
		return c, err
	}
	c.SchoolID, c.ClosedAt, c.OpenedAt = nullUUID(school), nullTime(closed), c.OpenedAt.UTC()
	if len(scope) > 0 {
		_ = json.Unmarshal(scope, &c.ScopeRef)
	}
	return c, nil
}

const channelColumns = `c.channel_id, c.tenant_id, c.school_id, c.channel_type, c.is_gameplay, c.scope_ref, c.status, c.last_seq,
	c.opened_at, c.closed_at`

// ChannelFacts implements read.ChatReader.
func (ChatReader) ChannelFacts(ctx context.Context, tenant, id uuid.UUID) (*read.ChannelFacts, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok chat.channels of the session tenant, live rows only
	ch, err := scanChannel(t.QueryRowContext(ctx, `SELECT `+channelColumns+` FROM chat.channels c
		WHERE c.tenant_id = $1 AND c.channel_id = $2 AND c.deleted_at IS NULL`, tenant, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: channel: %w", err)
	}
	f := &read.ChannelFacts{Channel: ch, Cards: map[uuid.UUID]read.ChatCard{}}
	// tenancyguard:ok chat.channel_members of the session tenant, live rows (left members kept for the history window)
	rows, err := t.QueryContext(ctx, `SELECT m.user_id, m.school_id, m.role, m.joined_at, m.left_at FROM chat.channel_members m
		WHERE m.tenant_id = $1 AND m.channel_id = $2 AND m.deleted_at IS NULL ORDER BY m.joined_at, m.user_id`, tenant, id)
	if err != nil {
		return nil, fmt.Errorf("chat: members: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		m := channel.Member{ChannelID: id, TenantID: tenant}
		var school uuid.NullUUID
		var left sql.NullTime
		if err := rows.Scan(&m.UserID, &school, &m.Role, &m.JoinedAt, &left); err != nil {
			return nil, fmt.Errorf("chat: members: %w", err)
		}
		m.SchoolID, m.LeftAt, m.JoinedAt = nullUUID(school), nullTime(left), m.JoinedAt.UTC()
		f.Members = append(f.Members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	all := make([]uuid.UUID, 0, len(f.Members))
	for _, m := range f.Members {
		all = append(all, m.UserID)
	}
	if len(all) == 0 {
		return f, nil
	}
	// tenancyguard:ok projection ref.player_profiles of the session tenant (public card only, R16; no deleted_at)
	cards, err := t.QueryContext(ctx, `SELECT p.student_id, p.nickname, p.level, p.avatar_config FROM ref.player_profiles p
		WHERE p.tenant_id = $1 AND p.student_id = ANY($2::uuid[])`, tenant, idStrings(all))
	if err != nil {
		return nil, fmt.Errorf("chat: cards: %w", err)
	}
	defer func() { _ = cards.Close() }()
	for cards.Next() {
		var c read.ChatCard
		var nick sql.NullString
		var avatar []byte
		if err := cards.Scan(&c.UserID, &nick, &c.Level, &avatar); err != nil {
			return nil, fmt.Errorf("chat: cards: %w", err)
		}
		c.Nickname = nullString(nick)
		if len(avatar) > 0 && string(avatar) != "null" {
			c.Avatar = json.RawMessage(avatar)
		}
		f.Cards[c.UserID] = c
	}
	if err := cards.Err(); err != nil {
		return nil, err
	}
	// tenancyguard:ok social.blocks of the session tenant, live rows between members only
	blocks, err := t.QueryContext(ctx, `SELECT b.blocker_id, b.blocked_id FROM social.blocks b
		WHERE b.tenant_id = $1 AND b.deleted_at IS NULL AND b.blocker_id = ANY($2::uuid[]) AND b.blocked_id = ANY($2::uuid[])`,
		tenant, idStrings(all))
	if err != nil {
		return nil, fmt.Errorf("chat: blocks: %w", err)
	}
	defer func() { _ = blocks.Close() }()
	for blocks.Next() {
		var b read.Block
		if err := blocks.Scan(&b.Blocker, &b.Blocked); err != nil {
			return nil, fmt.Errorf("chat: blocks: %w", err)
		}
		f.Blocks = append(f.Blocks, b)
	}
	return f, blocks.Err()
}

// PolicyLevels implements read.ChatReader: the platform document (tenant NULL) and the tenant and school documents of
// the session tenant (no class level since ADR-0095; class rows are soft-deleted).
func (ChatReader) PolicyLevels(ctx context.Context, tenant uuid.UUID, school *uuid.UUID) (policy.Levels, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return policy.Levels{}, err
	}
	// tenancyguard:ok chat.chat_policies (T*): the platform row (tenant_id NULL) plus rows of the session tenant, live only
	rows, err := t.QueryContext(ctx, `SELECT p.scope_type, p.rules FROM chat.chat_policies p
		WHERE p.deleted_at IS NULL AND ((p.scope_type = 'platform' AND p.tenant_id IS NULL)
			OR (p.tenant_id = $1 AND (p.scope_type = 'tenant'
				OR (p.scope_type = 'school' AND p.scope_id = $2::uuid))))`, tenant, school)
	if err != nil {
		return policy.Levels{}, fmt.Errorf("chat: policies: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var l policy.Levels
	for rows.Next() {
		var scope string
		var raw []byte
		if err := rows.Scan(&scope, &raw); err != nil {
			return policy.Levels{}, fmt.Errorf("chat: policies: %w", err)
		}
		d, err := policy.Parse(raw)
		if err != nil {
			return policy.Levels{}, fmt.Errorf("chat: policy %s: %w", scope, err)
		}
		switch scope {
		case "platform":
			l.Platform = &d
		case "tenant":
			l.Tenant = &d
		case "school":
			l.School = &d
		}
	}
	return l, rows.Err()
}

// CurfewWindows implements read.ChatReader.
func (ChatReader) CurfewWindows(ctx context.Context, tenant uuid.UUID, school *uuid.UUID) (*read.CurfewWindow, *read.CurfewWindow, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, nil, err
	}
	var platform *read.CurfewWindow
	var raw []byte
	// tenancyguard:ok ref.platform_settings is a G projection (no tenant, no deleted_at)
	err = t.QueryRowContext(ctx, `SELECT s.value FROM ref.platform_settings s WHERE s.key = 'platform_curfew'`).Scan(&raw)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return nil, nil, fmt.Errorf("chat: platform_curfew: %w", err)
	default:
		platform = curfewWindow(raw)
	}
	if school == nil {
		return platform, nil, nil
	}
	raw = nil
	// tenancyguard:ok projection ref.schools of the session tenant (no deleted_at: rule 2)
	err = t.QueryRowContext(ctx, `SELECT sc.settings_subset->'curfew_window' FROM ref.schools sc
		WHERE sc.tenant_id = $1 AND sc.school_id = $2`, tenant, *school).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) || len(raw) == 0 || string(raw) == "null" {
		return platform, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("chat: school curfew: %w", err)
	}
	return platform, curfewWindow(raw), nil
}

// curfewWindow decodes a projected {start, end}; a malformed value is ignored (the platform curfew still applies).
func curfewWindow(raw []byte) *read.CurfewWindow {
	var w read.CurfewWindow
	if json.Unmarshal(raw, &w) != nil || w.Start == "" {
		return nil
	}
	return &w
}

// LastSeq implements read.ChatReader.
func (ChatReader) LastSeq(ctx context.Context, tenant, id uuid.UUID) (int64, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return 0, err
	}
	var n int64
	// tenancyguard:ok chat.channels of the session tenant, live rows only
	err = t.QueryRowContext(ctx, `SELECT c.last_seq FROM chat.channels c WHERE c.tenant_id = $1 AND c.channel_id = $2
		AND c.deleted_at IS NULL`, tenant, id).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("chat: last_seq: %w", err)
	}
	return n, nil
}

// MyChannels implements read.ChatReader.
func (ChatReader) MyChannels(ctx context.Context, tenant, user uuid.UUID, limit int) ([]read.MyChannel, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok chat.channel_members joined with chat.channels, both of the session tenant, live rows only
	rows, err := t.QueryContext(ctx, `SELECT `+channelColumns+`, m.school_id, m.last_read_seq, m.joined_at
		FROM chat.channel_members m
		JOIN chat.channels c ON c.channel_id = m.channel_id AND c.tenant_id = m.tenant_id AND c.deleted_at IS NULL
		WHERE m.tenant_id = $1 AND m.user_id = $2 AND m.deleted_at IS NULL AND m.left_at IS NULL
		ORDER BY c.updated_at DESC, c.channel_id LIMIT $3`, tenant, user, limit)
	if err != nil {
		return nil, fmt.Errorf("chat: my channels: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.MyChannel
	for rows.Next() {
		var r read.MyChannel
		var school uuid.NullUUID
		ch, err := scanChannel(rows, &school, &r.MyReadSeq, &r.JoinedAt)
		if err != nil {
			return nil, fmt.Errorf("chat: my channels: %w", err)
		}
		r.Channel, r.SchoolID, r.JoinedAt = ch, nullUUID(school), r.JoinedAt.UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

// messagesSQL reads one channel of chat.messages or chat.gameplay_messages ({table}, {dir}): tenant, deleted_at, from
// joined_at, held only for the reader, removed never.
const messagesSQL = `SELECT m.message_id, m.seq, m.school_id, m.sender_id, m.client_message_id, m.kind, m.body_display,
	m.quick_message_id, m.sticker_id, m.gif_id, m.reply_to_message_id, array_to_string(m.mentions, ','), m.revision,
	m.moderation_state, m.created_at, m.recalled_at IS NOT NULL, m.filter_verdict->'params'
	FROM chat.{table} m
	WHERE m.tenant_id = $1 AND m.channel_id = $2 AND m.deleted_at IS NULL AND m.created_at >= $3
		AND m.moderation_state <> 'removed' AND (m.moderation_state <> 'held' OR m.sender_id = $4)
		AND ($5::bigint IS NULL OR m.seq < $5) AND ($6::bigint IS NULL OR m.seq > $6)
	ORDER BY m.seq {dir} LIMIT $7`

// Messages implements read.ChatReader.
func (ChatReader) Messages(ctx context.Context, q read.MessageQuery) ([]read.StoredMessage, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	table, dir := "messages", "DESC"
	if q.Gameplay {
		table = "gameplay_messages"
	}
	if q.After != nil {
		dir = "ASC"
	}
	query := strings.NewReplacer("{table}", table, "{dir}", dir).Replace(messagesSQL)
	rows, err := t.QueryContext(ctx, query, q.Tenant, q.Channel, q.From, q.Reader, q.Before, q.After, q.Limit)
	if err != nil {
		return nil, fmt.Errorf("chat: messages: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.StoredMessage
	for rows.Next() {
		e := message.Envelope{V: 1, Kind: message.EnvelopeKind, TenantID: q.Tenant, ChannelID: q.Channel, IsGameplay: q.Gameplay}
		var school, cmid, sticker, gif, reply uuid.NullUUID
		var display, quick sql.NullString
		var mentions, kind, state string
		var recalled bool
		var params []byte
		if err := rows.Scan(&e.MessageID, &e.Seq, &school, &e.SenderID, &cmid, &kind, &display, &quick, &sticker, &gif, &reply,
			&mentions, &e.Revision, &state, &e.CreatedAt, &recalled, &params); err != nil {
			return nil, fmt.Errorf("chat: messages: %w", err)
		}
		e.SchoolID, e.ClientMessageID, e.StickerID, e.GifID, e.ReplyToMessageID = nullUUID(school), nullUUID(cmid), nullUUID(sticker),
			nullUUID(gif), nullUUID(reply)
		e.MsgKind, e.Moderation.State, e.CreatedAt = message.Kind(kind), message.State(state), e.CreatedAt.UTC()
		e.BodyDisplay, e.QuickMessageID = nullString(display), nullString(quick)
		if len(params) > 0 && string(params) != "null" { // quick message parameters (F116-AC-28)
			_ = json.Unmarshal(params, &e.Params)
		}
		e.Mentions = []uuid.UUID{}
		for _, s := range strings.Split(mentions, ",") {
			if u, err := uuid.Parse(s); err == nil {
				e.Mentions = append(e.Mentions, u)
			}
		}
		out = append(out, read.StoredMessage{Envelope: e, Recalled: recalled})
	}
	return out, rows.Err()
}

// Channels implements write.ChannelRepository on chat.channels / chat.channel_members.
type Channels struct{}

var _ write.ChannelRepository = Channels{}

// CreateChannel implements write.ChannelRepository.
func (Channels) CreateChannel(ctx context.Context, c channel.Channel) (bool, error) {
	var scope any
	if len(c.ScopeRef) > 0 {
		b, err := json.Marshal(c.ScopeRef)
		if err != nil {
			return false, err
		}
		scope = string(b)
	}
	n, err := exec(ctx, `INSERT INTO chat.channels (channel_id, tenant_id, school_id, channel_type, is_gameplay, scope_ref, status,
		opened_at, created_by) VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9) ON CONFLICT (channel_id) DO NOTHING`,
		c.ID, c.TenantID, c.SchoolID, string(c.Type), c.IsGameplay, scope, c.Status, c.OpenedAt, actor(ctx))
	if err != nil {
		return false, fmt.Errorf("chat: create channel: %w", err)
	}
	return n == 1, nil
}

// Channel implements write.ChannelRepository.
func (Channels) Channel(ctx context.Context, tenant, id uuid.UUID) (*channel.Channel, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok chat.channels of the session tenant, live rows only
	c, err := scanChannel(t.QueryRowContext(ctx, `SELECT `+channelColumns+` FROM chat.channels c
		WHERE c.tenant_id = $1 AND c.channel_id = $2 AND c.deleted_at IS NULL`, tenant, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: channel: %w", err)
	}
	return &c, nil
}

// StudentSchool implements write.ChannelRepository.
func (Channels) StudentSchool(ctx context.Context, tenant, student uuid.UUID) (uuid.UUID, bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return uuid.Nil, false, err
	}
	var school uuid.UUID
	// tenancyguard:ok projection ref.students of the session tenant, cleaned-up students excluded
	err = t.QueryRowContext(ctx, `SELECT st.school_id FROM ref.students st WHERE st.tenant_id = $1 AND st.student_id = $2
		AND st.deleted_at IS NULL AND st.status = 'active'`, tenant, student).Scan(&school)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("chat: student: %w", err)
	}
	return school, true, nil
}

// JoinMember implements write.ChannelRepository: a new member row, or a member who left joins again (joined_at moves:
// the history window restarts, CHAT_SYSTEM §8.3).
func (Channels) JoinMember(ctx context.Context, m channel.Member) (bool, error) {
	n, err := exec(ctx, `INSERT INTO chat.channel_members AS cm (channel_member_id, tenant_id, channel_id, user_id, school_id, role,
		joined_at, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (channel_id, user_id) WHERE deleted_at IS NULL DO UPDATE SET left_at = NULL, joined_at = EXCLUDED.joined_at,
			school_id = EXCLUDED.school_id, updated_at = now(), version = cm.version + 1
		WHERE cm.left_at IS NOT NULL AND cm.tenant_id = EXCLUDED.tenant_id`,
		ids.New(), m.TenantID, m.ChannelID, m.UserID, m.SchoolID, m.Role, m.JoinedAt, actor(ctx))
	if err != nil {
		return false, fmt.Errorf("chat: join member: %w", err)
	}
	return n == 1, nil
}

// LeaveMember implements write.ChannelRepository.
func (Channels) LeaveMember(ctx context.Context, tenant, id, user uuid.UUID, at time.Time) (bool, error) {
	// tenancyguard:ok chat.channel_members of the session tenant, live active rows only
	n, err := exec(ctx, `UPDATE chat.channel_members SET left_at = $4, updated_at = now(), version = version + 1
		WHERE tenant_id = $1 AND channel_id = $2 AND user_id = $3 AND deleted_at IS NULL AND left_at IS NULL`, tenant, id, user, at)
	if err != nil {
		return false, fmt.Errorf("chat: leave member: %w", err)
	}
	return n == 1, nil
}
