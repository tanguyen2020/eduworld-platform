package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
)

// Messages implements write.MessageStore: the ONLY writer of chat.messages and chat.gameplay_messages (chat-persister,
// R25, F115-AC-21 static test). Rows are idempotent per message_id; a (channel_id, seq) already taken by another message
// is refused by the unique constraint (F115-AC-20) and reported as an orphan (dead letter, metric).
type Messages struct{}

var _ write.MessageStore = Messages{}

const insertMessageSQL = `INSERT INTO chat.messages (message_id, channel_id, seq, tenant_id, school_id, sender_id,
	client_message_id, kind, body, body_display, quick_message_id, sticker_id, gif_id, reply_to_message_id, mentions, revision,
	moderation_state, filter_verdict, created_at, quote, forwarded_from)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15::uuid[], $16, $17, $18::jsonb, $19, $20::jsonb, $21::jsonb)
	ON CONFLICT DO NOTHING`

const insertGameplaySQL = `INSERT INTO chat.gameplay_messages (message_id, channel_id, seq, tenant_id, school_id, sender_id,
	client_message_id, kind, body, body_display, quick_message_id, sticker_id, gif_id, reply_to_message_id, mentions, revision,
	moderation_state, filter_verdict, created_at, created_on, quote, forwarded_from)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15::uuid[], $16, $17, $18::jsonb, $19,
		($19::timestamptz AT TIME ZONE 'UTC')::date, $20::jsonb, $21::jsonb)
	ON CONFLICT DO NOTHING`

// Insert implements write.MessageStore.
func (Messages) Insert(ctx context.Context, tenant uuid.UUID, es []message.Envelope) (write.PersistResult, error) {
	t, err := tx(ctx)
	if err != nil {
		return write.PersistResult{}, err
	}
	var res write.PersistResult
	chans := map[uuid.UUID]bool{}
	for _, e := range es {
		chans[e.ChannelID] = false
	}
	list := make([]uuid.UUID, 0, len(chans))
	for id := range chans {
		list = append(list, id)
	}
	known, err := knownChannels(ctx, tenant, list)
	if err != nil {
		return res, err
	}
	for id, gameplay := range known {
		chans[id] = gameplay
	}
	implicit, err := implicitChannels(ctx, tenant, list)
	if err != nil {
		return res, err
	}
	// One writer per channel at a time: (channel_id, seq) is unique across partitions only under this lock (the
	// partitioned unique key also holds created_at), in a fixed order so that two replicas never deadlock.
	sort.Slice(list, func(i, j int) bool { return list[i].String() < list[j].String() })
	for _, id := range list {
		if _, ok := known[id]; !ok {
			continue
		}
		if _, err := t.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('chat.seq:' || $1::text, 0))`, id); err != nil {
			return res, fmt.Errorf("chat: persist lock: %w", err)
		}
	}
	lastSeq := map[uuid.UUID]int64{}
	days := map[[2]string]bool{}
	for _, e := range es {
		if _, ok := known[e.ChannelID]; e.TenantID != tenant || !ok {
			res.Orphans = append(res.Orphans, e)
			continue
		}
		verdict, _ := json.Marshal(map[string]any{"decision": e.Moderation.Decision, "category_codes": e.Moderation.CategoryCodes,
			"steps_skipped": e.Moderation.StepsSkipped, "catalog_version": e.Moderation.CatalogVersion,
			"catalog_version_tenant": e.Moderation.CatalogVersionTenant, "params": e.Params})
		if implicit[e.ChannelID] { // World/Area (F120-AC-12): chat.ephemeral_messages, purged after 24 h by F117
			r, err := t.ExecContext(ctx, insertEphemeralSQL, e.MessageID, e.ChannelID, e.TenantID, e.SchoolID, e.SenderID,
				string(e.MsgKind), e.Body(), e.QuickMessageID, string(e.Moderation.State), string(verdict), e.CreatedAt.UTC())
			if err != nil {
				return res, fmt.Errorf("chat: persist ephemeral message: %w", err)
			}
			if n, _ := r.RowsAffected(); n == 0 {
				continue // redelivery
			}
			if err := insertDecision(ctx, EnvelopeDecision(e)); err != nil {
				return res, err
			}
			res.Inserted = append(res.Inserted, e)
			lastSeq[e.ChannelID] = max(lastSeq[e.ChannelID], e.Seq)
			continue
		}
		taken, err := seqTaken(ctx, e, chans[e.ChannelID])
		if err != nil {
			return res, err
		}
		if taken { // (channel_id, seq) already holds another message (lost Valkey counter, F115-AC-20)
			res.Orphans = append(res.Orphans, e)
			continue
		}
		query := insertMessageSQL
		if chans[e.ChannelID] {
			query = insertGameplaySQL
		}
		r, err := t.ExecContext(ctx, query, e.MessageID, e.ChannelID, e.Seq, e.TenantID, e.SchoolID, e.SenderID, e.ClientMessageID,
			string(e.MsgKind), e.Body(), e.BodyDisplay, e.QuickMessageID, e.StickerID, e.GifID, e.ReplyToMessageID,
			idStrings(e.Mentions), max(e.Revision, 1), string(e.Moderation.State), string(verdict), e.CreatedAt.UTC(), jsonOrNil(e.Quote),
			jsonOrNil(e.ForwardedFrom))
		if err != nil {
			return res, fmt.Errorf("chat: persist message: %w", err)
		}
		if n, _ := r.RowsAffected(); n == 0 {
			taken, err := messageExists(ctx, e, chans[e.ChannelID])
			if err != nil {
				return res, err
			}
			if !taken { // seq already used by another message: the unique constraint refused it
				res.Orphans = append(res.Orphans, e)
			}
			continue
		}
		if err := insertDecision(ctx, EnvelopeDecision(e)); err != nil { // decision log of a stored message (F116-AC-09)
			return res, err
		}
		if _, deliver, _, _ := e.Moderation.Decision.Outcome(); deliver && len(e.Mentions) > 0 { // F118-AC-19
			if err := raiseMentions(ctx, t, tenant, e.ChannelID, e.Mentions, e.Seq); err != nil {
				return res, err
			}
		}
		res.Inserted = append(res.Inserted, e)
		lastSeq[e.ChannelID] = max(lastSeq[e.ChannelID], e.Seq)
		if chans[e.ChannelID] {
			days[[2]string{e.CreatedAt.UTC().Format("2006-01-02"), e.ChannelID.String()}] = true
		}
	}
	for id, seq := range lastSeq {
		// tenancyguard:ok chat.channels of the envelope tenant, live rows; last_seq never moves back (GREATEST)
		if _, err := t.ExecContext(ctx, `UPDATE chat.channels SET last_seq = GREATEST(last_seq, $3), updated_at = now()
			WHERE tenant_id = $1 AND channel_id = $2 AND deleted_at IS NULL`, tenant, id, seq); err != nil {
			return res, fmt.Errorf("chat: last_seq: %w", err)
		}
	}
	for k := range days {
		if _, err := t.ExecContext(ctx, `INSERT INTO chat.gameplay_partition_channels (partition_day, channel_id, tenant_id)
			VALUES ($1::date, $2, $3) ON CONFLICT DO NOTHING`, k[0], k[1], tenant); err != nil {
			return res, fmt.Errorf("chat: partition channels: %w", err)
		}
	}
	return res, nil
}

const insertEphemeralSQL = `INSERT INTO chat.ephemeral_messages (message_id, channel_id, tenant_id, school_id, sender_id, kind, body,
	quick_message_id, moderation_state, filter_verdict, created_at, created_on)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, $11, ($11::timestamptz AT TIME ZONE 'UTC')::date)
	ON CONFLICT DO NOTHING`

// implicitChannels returns the World/Area channels of tenant among ids (F120: stored in chat.ephemeral_messages).
func implicitChannels(ctx context.Context, tenant uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok chat.channels of the envelope tenant (TenantScope + RLS), live rows only
	rows, err := t.QueryContext(ctx, `SELECT c.channel_id FROM chat.channels c
		WHERE c.tenant_id = $1 AND c.channel_id = ANY($2::uuid[]) AND c.deleted_at IS NULL AND c.channel_type IN ('world', 'area')`,
		tenant, idStrings(ids))
	if err != nil {
		return nil, fmt.Errorf("chat: persist implicit channels: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[uuid.UUID]bool{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("chat: persist implicit channels: %w", err)
		}
		out[id] = true
	}
	return out, rows.Err()
}

// knownChannels returns the live channels of tenant among ids with their is_gameplay.
func knownChannels(ctx context.Context, tenant uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok chat.channels of the envelope tenant (TenantScope + RLS), live rows only
	rows, err := t.QueryContext(ctx, `SELECT c.channel_id, c.is_gameplay FROM chat.channels c
		WHERE c.tenant_id = $1 AND c.channel_id = ANY($2::uuid[]) AND c.deleted_at IS NULL`, tenant, idStrings(ids))
	if err != nil {
		return nil, fmt.Errorf("chat: persist channels: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[uuid.UUID]bool{}
	for rows.Next() {
		var id uuid.UUID
		var gameplay bool
		if err := rows.Scan(&id, &gameplay); err != nil {
			return nil, fmt.Errorf("chat: persist channels: %w", err)
		}
		out[id] = gameplay
	}
	return out, rows.Err()
}

// seqTaken reports a stored message of another message_id with the (channel_id, seq) of e.
func seqTaken(ctx context.Context, e message.Envelope, gameplay bool) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	var n int
	// tenancyguard:ok uniqueness of (channel_id, seq) in the envelope tenant, removed messages included (seq never reused)
	q := `SELECT count(*) FROM chat.messages m WHERE m.tenant_id = $1 AND m.channel_id = $2 AND m.seq = $3 AND m.message_id <> $4`
	if gameplay {
		// tenancyguard:ok uniqueness of (channel_id, seq) in the envelope tenant, removed messages included (seq never reused)
		q = `SELECT count(*) FROM chat.gameplay_messages m WHERE m.tenant_id = $1 AND m.channel_id = $2 AND m.seq = $3 AND m.message_id <> $4`
	}
	if err := t.QueryRowContext(ctx, q, e.TenantID, e.ChannelID, e.Seq, e.MessageID).Scan(&n); err != nil {
		return false, fmt.Errorf("chat: seq check: %w", err)
	}
	return n > 0, nil
}

// messageExists reports a row with the message_id of e (a redelivery) — false means (channel_id, seq) was taken.
func messageExists(ctx context.Context, e message.Envelope, gameplay bool) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	var n int
	// tenancyguard:ok duplicate check of one message_id of the envelope tenant (any deleted_at: a redelivery stays a duplicate)
	q := `SELECT count(*) FROM chat.messages m WHERE m.tenant_id = $1 AND m.message_id = $2 AND m.created_at = $3`
	if gameplay {
		// tenancyguard:ok duplicate check of one message_id of the envelope tenant (any deleted_at: a redelivery stays a duplicate)
		q = `SELECT count(*) FROM chat.gameplay_messages m WHERE m.tenant_id = $1 AND m.message_id = $2 AND m.created_at = $3`
	}
	if err := t.QueryRowContext(ctx, q, e.TenantID, e.MessageID, e.CreatedAt.UTC()).Scan(&n); err != nil {
		return false, fmt.Errorf("chat: duplicate check: %w", err)
	}
	return n > 0, nil
}

// jsonOrNil encodes v for a jsonb column (nil pointer: NULL).
func jsonOrNil[T any](v *T) any {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return string(b)
}
