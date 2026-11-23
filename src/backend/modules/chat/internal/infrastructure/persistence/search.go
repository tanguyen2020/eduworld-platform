package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
)

// Search implements read.SearchReader on chat.messages (GIN messages_search_idx over search_tsv = unaccented body_display).
type Search struct{}

var _ read.SearchReader = Search{}

// searchSQL: the persistent channels (club team, direct) the reader is an active member of, messages from its joined_at,
// display content only, no recalled/removed/held/anonymised message, no message of a sender blocked either way.
//
// tenancyguard:ok chat.messages, channel_members, channels, social.blocks of the session tenant with deleted_at filters
const searchSQL = `SELECT ` + messageColumns + `, c.channel_type, p.nickname, coalesce(p.level, 1), p.avatar_config
	FROM chat.messages m
	JOIN chat.channel_members cm ON cm.tenant_id = m.tenant_id AND cm.channel_id = m.channel_id AND cm.user_id = $2
		AND cm.deleted_at IS NULL AND cm.left_at IS NULL AND m.created_at >= cm.joined_at
	JOIN chat.channels c ON c.tenant_id = m.tenant_id AND c.channel_id = m.channel_id AND c.deleted_at IS NULL
		AND NOT c.is_gameplay AND c.channel_type IN ('team', 'direct')
	LEFT JOIN ref.player_profiles p ON p.tenant_id = m.tenant_id AND p.student_id = m.sender_id
	WHERE m.tenant_id = $1 AND m.deleted_at IS NULL AND m.recalled_at IS NULL AND m.anonymized_at IS NULL
		AND m.moderation_state IN ('allowed', 'masked', 'unchecked')
		AND m.search_tsv @@ to_tsquery('simple', chat.f_unaccent(lower($3)))
		AND ($4::uuid IS NULL OR m.channel_id = $4) AND ($5::uuid IS NULL OR m.sender_id = $5)
		AND ($6::timestamptz IS NULL OR m.created_at >= $6) AND ($7::timestamptz IS NULL OR m.created_at < $7)
		AND ($8::timestamptz IS NULL OR (m.created_at, m.message_id) < ($8, $9::uuid))
		AND NOT EXISTS (SELECT 1 FROM social.blocks b WHERE b.tenant_id = m.tenant_id AND b.deleted_at IS NULL
			AND ((b.blocker_id = $2 AND b.blocked_id = m.sender_id) OR (b.blocker_id = m.sender_id AND b.blocked_id = $2)))
	ORDER BY m.created_at DESC, m.message_id DESC LIMIT $10`

// Search implements read.SearchReader.
func (Search) Search(ctx context.Context, q read.SearchQuery) ([]read.SearchHit, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, searchSQL, q.Tenant, q.Reader, q.TSQuery, q.Channel, q.Sender, q.From, q.To, q.AfterAt, q.AfterID,
		q.Limit)
	if err != nil {
		return nil, fmt.Errorf("chat: search: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.SearchHit
	for rows.Next() {
		var h read.SearchHit
		var nick sql.NullString
		var avatar []byte
		m, err := scanMessage(scanTail{rows: rows, tail: []any{&h.ChannelType, &nick, &h.Sender.Level, &avatar}}, q.Tenant, false)
		if err != nil {
			return nil, err
		}
		h.Message = m
		h.Sender.UserID, h.Sender.Nickname = m.Envelope.SenderID, nullString(nick)
		if len(avatar) > 0 && string(avatar) != "null" {
			h.Sender.Avatar = json.RawMessage(avatar)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// scanTail scans the message columns then extra columns of the same row.
type scanTail struct {
	rows *sql.Rows
	tail []any
}

func (s scanTail) Scan(dest ...any) error { return s.rows.Scan(append(dest, s.tail...)...) }
