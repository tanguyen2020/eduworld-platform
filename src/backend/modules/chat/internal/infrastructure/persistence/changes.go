package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
)

// Changes implements write.ChangeStore: chat-persister applies the message changes of the durable stream (F118, R25)
// by revision (EVENT_DESIGN §4.10): an edit or recall older than the stored revision changes nothing, but the version
// an edit replaced is always kept in chat.message_revisions (evidence, ADR-0034), whatever the order of arrival.
type Changes struct{}

var _ write.ChangeStore = Changes{}

type row struct {
	revision  int
	body      sql.NullString
	display   sql.NullString
	mentions  string
	decision  sql.NullString
	state     string
	recalled  bool
	senderID  uuid.UUID
	seq       int64
	mentionsL []uuid.UUID
}

func table(gameplay bool) string {
	if gameplay {
		return "gameplay_messages"
	}
	return "messages"
}

func mentionList(s string) []uuid.UUID {
	out := []uuid.UUID{}
	for _, p := range splitTags(s) {
		if u, err := uuid.Parse(p); err == nil {
			out = append(out, u)
		}
	}
	return out
}

// Apply implements write.ChangeStore.
func (Changes) Apply(ctx context.Context, c message.Change) (write.ChangeOutcome, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	tb := table(c.IsGameplay)
	var r row
	// tenancyguard:ok the message of the change in the session tenant, locked while the change is applied
	err = t.QueryRowContext(ctx, `SELECT m.revision, m.body, m.body_display, array_to_string(m.mentions, ','),
			m.filter_verdict->>'decision', m.moderation_state, m.recalled_at IS NOT NULL, m.sender_id, m.seq
		FROM chat.`+tb+` m WHERE m.tenant_id = $1 AND m.channel_id = $2 AND m.message_id = $3 AND m.deleted_at IS NULL
		LIMIT 1 FOR UPDATE`, c.TenantID, c.ChannelID, c.MessageID).
		Scan(&r.revision, &r.body, &r.display, &r.mentions, &r.decision, &r.state, &r.recalled, &r.senderID, &r.seq)
	if errors.Is(err, sql.ErrNoRows) {
		return write.ChangeMissing, nil
	}
	if err != nil {
		return 0, fmt.Errorf("chat: change target: %w", err)
	}
	r.mentionsL = mentionList(r.mentions)
	switch c.Op {
	case message.OpEdit:
		return applyEdit(ctx, t, tb, c, r)
	case message.OpEditHeld:
		verdict, _ := json.Marshal(verdictOf(c.Moderation))
		body := c.BodyOriginal
		if body == nil {
			body = c.BodyDisplay
		}
		if err := insertRevision(ctx, t, c, c.Revision, "held", body, c.BodyDisplay, c.Mentions, string(c.Moderation.Decision), verdict); err != nil {
			return 0, err
		}
		return write.ChangeApplied, nil
	case message.OpRecall:
		if r.recalled || c.Revision <= r.revision {
			return write.ChangeStale, nil
		}
		// tenancyguard:ok the locked message of the session tenant: soft recall, the body stays as evidence (ADR-0028)
		if _, err := t.ExecContext(ctx, `UPDATE chat.`+tb+` SET recalled_at = $4, recalled_by = $5, revision = $6
			WHERE tenant_id = $1 AND channel_id = $2 AND message_id = $3 AND deleted_at IS NULL`,
			c.TenantID, c.ChannelID, c.MessageID, c.At, c.ActorID, c.Revision); err != nil {
			return 0, fmt.Errorf("chat: recall: %w", err)
		}
		// tenancyguard:ok live pin of the recalled message of the session tenant (soft delete)
		if _, err := t.ExecContext(ctx, `UPDATE chat.pinned_messages SET deleted_at = $4, deleted_by = $5
			WHERE tenant_id = $1 AND channel_id = $2 AND message_id = $3 AND deleted_at IS NULL`,
			c.TenantID, c.ChannelID, c.MessageID, c.At, c.ActorID); err != nil {
			return 0, fmt.Errorf("chat: recall unpin: %w", err)
		}
		return write.ChangeApplied, nil
	case message.OpReactionSet:
		// tenancyguard:ok reaction row of the session tenant; an older change never overrides a newer one
		if _, err := t.ExecContext(ctx, `INSERT INTO chat.message_reactions AS r (tenant_id, channel_id, message_id, user_id, emoji_code,
				created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $6)
			ON CONFLICT (message_id, user_id, emoji_code) DO UPDATE SET deleted_at = NULL, updated_at = EXCLUDED.updated_at
			WHERE r.updated_at <= EXCLUDED.updated_at AND r.tenant_id = EXCLUDED.tenant_id`,
			c.TenantID, c.ChannelID, c.MessageID, c.ActorID, c.Emoji, c.At); err != nil {
			return 0, fmt.Errorf("chat: reaction: %w", err)
		}
		return write.ChangeApplied, nil
	case message.OpReactionUnset:
		// tenancyguard:ok reaction row of the session tenant (soft delete); an older change never overrides a newer one
		if _, err := t.ExecContext(ctx, `UPDATE chat.message_reactions SET deleted_at = $5, updated_at = $5
			WHERE tenant_id = $1 AND message_id = $2 AND user_id = $3 AND emoji_code = $4 AND deleted_at IS NULL AND updated_at <= $5`,
			c.TenantID, c.MessageID, c.ActorID, c.Emoji, c.At); err != nil {
			return 0, fmt.Errorf("chat: reaction: %w", err)
		}
		return write.ChangeApplied, nil
	case message.OpPin:
		if r.recalled {
			return write.ChangeStale, nil
		}
		if _, err := t.ExecContext(ctx, `SAVEPOINT chat_pin`); err != nil {
			return 0, err
		}
		// tenancyguard:ok pin row of the session tenant (re-pinning clears the soft delete)
		_, err := t.ExecContext(ctx, `INSERT INTO chat.pinned_messages AS p (tenant_id, channel_id, message_id, pinned_by, pinned_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (channel_id, message_id) DO UPDATE SET deleted_at = NULL, deleted_by = NULL, pinned_by = EXCLUDED.pinned_by,
				pinned_at = EXCLUDED.pinned_at WHERE p.deleted_at IS NOT NULL AND p.tenant_id = EXCLUDED.tenant_id`,
			c.TenantID, c.ChannelID, c.MessageID, c.ActorID, c.At)
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23514" { // more than 3 live pins (database ceiling): nothing pinned
			if _, rerr := t.ExecContext(ctx, `ROLLBACK TO SAVEPOINT chat_pin`); rerr != nil {
				return 0, rerr
			}
			return write.ChangeStale, nil
		}
		if err != nil {
			return 0, fmt.Errorf("chat: pin: %w", err)
		}
		return write.ChangeApplied, nil
	case message.OpUnpin:
		// tenancyguard:ok live pin of the session tenant (soft delete)
		if _, err := t.ExecContext(ctx, `UPDATE chat.pinned_messages SET deleted_at = $4, deleted_by = $5
			WHERE tenant_id = $1 AND channel_id = $2 AND message_id = $3 AND deleted_at IS NULL`,
			c.TenantID, c.ChannelID, c.MessageID, c.At, c.ActorID); err != nil {
			return 0, fmt.Errorf("chat: unpin: %w", err)
		}
		return write.ChangeApplied, nil
	}
	return 0, message.ErrChange
}

func verdictOf(m *message.Moderation) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return map[string]any{"decision": m.Decision, "category_codes": m.CategoryCodes, "steps_skipped": m.StepsSkipped,
		"catalog_version": m.CatalogVersion, "catalog_version_tenant": m.CatalogVersionTenant}
}

func applyEdit(ctx context.Context, t *sql.Tx, tb string, c message.Change, r row) (write.ChangeOutcome, error) {
	// Evidence first: the replaced version, from the stored row when it is that version, else from the change.
	p := c.Previous
	if p.Revision == r.revision {
		dec := "allow"
		if r.decision.Valid && r.decision.String != "" {
			dec = r.decision.String
		}
		if err := insertRevision(ctx, t, c, r.revision, "previous", nullString(r.body), nullString(r.display), r.mentionsL, dec, nil); err != nil {
			return 0, err
		}
	} else {
		body := p.BodyOriginal
		if body == nil {
			body = p.BodyDisplay
		}
		dec := string(p.Decision)
		if dec == "" {
			dec = "allow"
		}
		if err := insertRevision(ctx, t, c, p.Revision, "previous", body, p.BodyDisplay, p.Mentions, dec, nil); err != nil {
			return 0, err
		}
	}
	if r.recalled || c.Revision <= r.revision {
		return write.ChangeStale, nil
	}
	body := c.BodyOriginal
	if body == nil {
		body = c.BodyDisplay
	}
	verdict, _ := json.Marshal(verdictOf(c.Moderation))
	// tenancyguard:ok the locked message of the session tenant: the newer revision of an edit
	if _, err := t.ExecContext(ctx, `UPDATE chat.`+tb+` SET body = $4, body_display = $5, mentions = $6::uuid[], revision = $7,
			edit_count = edit_count + 1, edited_at = $8, moderation_state = $9, filter_verdict = $10::jsonb
		WHERE tenant_id = $1 AND channel_id = $2 AND message_id = $3 AND deleted_at IS NULL`,
		c.TenantID, c.ChannelID, c.MessageID, body, c.BodyDisplay, idStrings(c.Mentions), c.Revision, c.At,
		string(c.Moderation.State), string(verdict)); err != nil {
		return 0, fmt.Errorf("chat: edit: %w", err)
	}
	// New mentions of the edit raise the counter of members who have not read past the message.
	if added := newIDs(c.Mentions, r.mentionsL); len(added) > 0 {
		if err := raiseMentions(ctx, t, c.TenantID, c.ChannelID, added, r.seq); err != nil {
			return 0, err
		}
	}
	return write.ChangeApplied, nil
}

func newIDs(a, b []uuid.UUID) []uuid.UUID {
	seen := map[uuid.UUID]bool{}
	for _, x := range b {
		seen[x] = true
	}
	var out []uuid.UUID
	for _, x := range a {
		if !seen[x] {
			out = append(out, x)
		}
	}
	return out
}

// raiseMentions adds one unread mention to each active member among users whose read cursor is before seq (F118-AC-19).
func raiseMentions(ctx context.Context, t *sql.Tx, tenant, channel uuid.UUID, users []uuid.UUID, seq int64) error {
	// tenancyguard:ok active members of one channel of the session tenant, live rows only
	if _, err := t.ExecContext(ctx, `UPDATE chat.channel_members SET unread_mentions = unread_mentions + 1, updated_at = now()
		WHERE tenant_id = $1 AND channel_id = $2 AND user_id = ANY($3::uuid[]) AND deleted_at IS NULL AND left_at IS NULL
			AND last_read_seq < $4`, tenant, channel, idStrings(users), seq); err != nil {
		return fmt.Errorf("chat: mentions: %w", err)
	}
	return nil
}

func insertRevision(ctx context.Context, t *sql.Tx, c message.Change, revision int, state string, body, display *string,
	mentions []uuid.UUID, decision string, verdict []byte) error {
	var v any
	if len(verdict) > 0 {
		v = string(verdict)
	}
	if mentions == nil {
		mentions = []uuid.UUID{}
	}
	// tenancyguard:ok evidence row of the session tenant (append-only; a redelivered change inserts nothing)
	if _, err := t.ExecContext(ctx, `INSERT INTO chat.message_revisions (tenant_id, channel_id, message_id, revision, state, body,
			body_display, mentions, decision, filter_verdict, edited_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::uuid[], $9, $10::jsonb, $11)
		ON CONFLICT DO NOTHING`, c.TenantID, c.ChannelID, c.MessageID, max(revision, 1), state, body, display, idStrings(mentions),
		decision, v, c.At); err != nil {
		return fmt.Errorf("chat: revision: %w", err)
	}
	return nil
}
