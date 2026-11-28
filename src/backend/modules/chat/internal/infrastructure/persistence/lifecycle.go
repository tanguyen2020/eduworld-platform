package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// Lifecycle implements write.LifecycleStore (F115 T042) on chat.channel_members, chat.messages, chat.moderation_cases,
// chat.sanctions, social.blocks, social.mutes and ref.player_profiles: soft-delete and UPDATE only, never DELETE (R21);
// explicit tenant conditions under the TenantScope of the saga step (R8, R27).
type Lifecycle struct{}

var _ write.LifecycleStore = Lifecycle{}

// openCaseStatuses are the case statuses not resolved yet (moderation_cases_status_chk).
const openCaseStatuses = `('open','in_review','escalated','reopened')`

// OpenCaseOf implements write.LifecycleStore.
func (Lifecycle) OpenCaseOf(ctx context.Context, tenant, user uuid.UUID) (bool, error) {
	// tenancyguard:ok chat.moderation_cases of the session tenant, live rows only
	n, err := count(ctx, `SELECT count(*) FROM chat.moderation_cases mc WHERE mc.tenant_id = $1 AND mc.deleted_at IS NULL
		AND mc.status IN `+openCaseStatuses+`
		AND (mc.subject_user_ids @> jsonb_build_array($2::text) OR (mc.target_type = 'user' AND mc.target_id = $2::uuid))`,
		tenant, user.String())
	if err != nil {
		return false, fmt.Errorf("chat: open case of user: %w", err)
	}
	return n > 0, nil
}

// idList runs a query (or UPDATE ... RETURNING) of one uuid column in the transaction of ctx.
func idList(ctx context.Context, query string, args ...any) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// EndMemberships implements write.LifecycleStore.
func (Lifecycle) EndMemberships(ctx context.Context, tenant, user uuid.UUID, reason string, at time.Time) ([]uuid.UUID, error) {
	// tenancyguard:ok chat.channel_members of the session tenant, live active rows only (soft-delete, R21)
	out, err := idList(ctx, `UPDATE chat.channel_members SET deleted_at = $3, deleted_by = $4, delete_reason = $5, updated_at = $3,
		updated_by = $4, version = version + 1
		WHERE tenant_id = $1 AND user_id = $2 AND deleted_at IS NULL AND left_at IS NULL RETURNING channel_id`,
		tenant, user, at, actor(ctx), reason)
	if err != nil {
		return nil, fmt.Errorf("chat: end memberships: %w", err)
	}
	return out, nil
}

// RestoreMemberships implements write.LifecycleStore.
func (Lifecycle) RestoreMemberships(ctx context.Context, tenant, user uuid.UUID, reason string, at time.Time) ([]uuid.UUID, error) {
	// tenancyguard:ok chat.channel_members + chat.channels of the session tenant; the rows soft-deleted by the step are
	// read on purpose (restore of ADR-0032), a live row of the pair always wins (channel_members_pair_uq)
	out, err := idList(ctx, `UPDATE chat.channel_members cm SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL,
		updated_at = $3, updated_by = $5, version = cm.version + 1
		FROM chat.channels c
		WHERE cm.channel_member_id IN (SELECT DISTINCT ON (x.channel_id) x.channel_member_id FROM chat.channel_members x
				WHERE x.tenant_id = $1 AND x.user_id = $2 AND x.deleted_at IS NOT NULL AND x.delete_reason = $4
				ORDER BY x.channel_id, x.deleted_at DESC)
			AND cm.tenant_id = $1 AND c.tenant_id = cm.tenant_id AND c.channel_id = cm.channel_id
			AND c.deleted_at IS NULL AND NOT c.is_gameplay AND c.status <> 'closed'
			AND NOT EXISTS (SELECT 1 FROM chat.channel_members l WHERE l.tenant_id = $1 AND l.channel_id = cm.channel_id
				AND l.user_id = cm.user_id AND l.deleted_at IS NULL)
		RETURNING cm.channel_id`, tenant, user, at, reason, actor(ctx))
	if err != nil {
		return nil, fmt.Errorf("chat: restore memberships: %w", err)
	}
	return out, nil
}

// movableMember is an active membership of the source of a merge in a live persistent channel (locked FOR UPDATE).
type movableMember struct {
	id, channel        uuid.UUID
	school             uuid.NullUUID
	role, notification string
	joined             time.Time
	delivered, readSeq int64
	receipts           bool
}

func movableMembers(ctx context.Context, tenant, from uuid.UUID) ([]movableMember, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok chat.channel_members + chat.channels + social.clubs of the session tenant, live rows only
	rows, err := t.QueryContext(ctx, `SELECT m.channel_member_id, m.channel_id, m.school_id, m.role, m.joined_at, m.last_delivered_seq,
			m.last_read_seq, m.show_read_receipts, m.notification_level
		FROM chat.channel_members m
		JOIN chat.channels c ON c.tenant_id = m.tenant_id AND c.channel_id = m.channel_id AND c.deleted_at IS NULL
			AND NOT c.is_gameplay AND c.status <> 'closed'
		WHERE m.tenant_id = $1 AND m.user_id = $2 AND m.deleted_at IS NULL AND m.left_at IS NULL
			AND NOT EXISTS (SELECT 1 FROM social.clubs cl WHERE cl.tenant_id = $1 AND cl.chat_channel_id = m.channel_id AND cl.deleted_at IS NULL)
		ORDER BY m.channel_id FOR UPDATE OF m`, tenant, from)
	if err != nil {
		return nil, fmt.Errorf("chat: memberships to move: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var list []movableMember
	for rows.Next() {
		var m movableMember
		if err := rows.Scan(&m.id, &m.channel, &m.school, &m.role, &m.joined, &m.delivered, &m.readSeq, &m.receipts, &m.notification); err != nil {
			return nil, fmt.Errorf("chat: memberships to move: %w", err)
		}
		list = append(list, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chat: memberships to move: %w", err)
	}
	return list, nil
}

// MoveMemberships implements write.LifecycleStore. The fixed team channels of the clubs follow social.club_members
// (F120: a club is joined through its own flow) and the gameplay channels follow the events of their owner module
// (party.member_joined via merge, F100 T042): both are left as they are.
func (Lifecycle) MoveMemberships(ctx context.Context, tenant, from, into uuid.UUID, at time.Time) ([]uuid.UUID, error) {
	list, err := movableMembers(ctx, tenant, from)
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, 0, len(list))
	for _, m := range list {
		// The target takes the place of the source unless already an active member (its own role and cursors win); a
		// target that had left comes back with the earliest joined_at (history visible since the source joined).
		// tenancyguard:ok insert under the TenantScope (RLS WITH CHECK on tenant_id); ref.students of the session tenant
		if _, err := exec(ctx, `INSERT INTO chat.channel_members AS cm (channel_member_id, tenant_id, created_at, created_by, updated_at,
				channel_id, user_id, school_id, role, joined_at, last_delivered_seq, last_read_seq, show_read_receipts, notification_level)
			VALUES ($1, $2, $3, $4, $3, $5, $6, coalesce((SELECT st.school_id FROM ref.students st WHERE st.tenant_id = $2
				AND st.student_id = $6 AND st.deleted_at IS NULL), $7), $8, $9, $10, $11, $12, $13)
			ON CONFLICT (channel_id, user_id) WHERE deleted_at IS NULL DO UPDATE SET left_at = NULL,
				joined_at = LEAST(cm.joined_at, EXCLUDED.joined_at), role = EXCLUDED.role,
				last_delivered_seq = GREATEST(cm.last_delivered_seq, EXCLUDED.last_delivered_seq),
				last_read_seq = GREATEST(cm.last_read_seq, EXCLUDED.last_read_seq), updated_at = EXCLUDED.updated_at, version = cm.version + 1
			WHERE cm.left_at IS NOT NULL AND cm.tenant_id = EXCLUDED.tenant_id`,
			ids.New(), tenant, at, actor(ctx), m.channel, into, m.school, m.role, m.joined, m.delivered, m.readSeq, m.receipts,
			m.notification); err != nil {
			return nil, fmt.Errorf("chat: move membership: %w", err)
		}
		// tenancyguard:ok chat.channel_members of the session tenant, the locked live row of the source (soft-delete, R21)
		if _, err := exec(ctx, `UPDATE chat.channel_members SET deleted_at = $3, deleted_by = $4, delete_reason = $5, updated_at = $3,
			updated_by = $4, version = version + 1 WHERE tenant_id = $1 AND channel_member_id = $2 AND deleted_at IS NULL`,
			tenant, m.id, at, actor(ctx), write.MemberDeletedMerge); err != nil {
			return nil, fmt.Errorf("chat: end merged membership: %w", err)
		}
		out = append(out, m.channel)
	}
	return out, nil
}

// LiveChannels implements write.LifecycleStore.
func (Lifecycle) LiveChannels(ctx context.Context, tenant, user uuid.UUID) ([]uuid.UUID, error) {
	// tenancyguard:ok chat.channel_members of the session tenant, live active rows only
	out, err := idList(ctx, `SELECT m.channel_id FROM chat.channel_members m WHERE m.tenant_id = $1 AND m.user_id = $2
		AND m.deleted_at IS NULL AND m.left_at IS NULL ORDER BY m.channel_id`, tenant, user)
	if err != nil {
		return nil, fmt.Errorf("chat: live channels: %w", err)
	}
	return out, nil
}

// AuthoredMessages implements write.LifecycleStore.
func (Lifecycle) AuthoredMessages(ctx context.Context, tenant, user uuid.UUID, limit int) ([]uuid.UUID, error) {
	// tenancyguard:ok chat.messages of the session tenant, deleted ones too on purpose (anonymisation is data minimisation)
	out, err := idList(ctx, `SELECT m.message_id FROM chat.messages m WHERE m.tenant_id = $1 AND m.sender_id = $2
		AND m.anonymized_at IS NULL ORDER BY m.created_at, m.message_id LIMIT $3`, tenant, user, limit)
	if err != nil {
		return nil, fmt.Errorf("chat: authored messages: %w", err)
	}
	return out, nil
}

// CopyEvidence implements write.LifecycleStore (the evidence copies of persistent-chat-retention, F120-AC-25).
func (Lifecycle) CopyEvidence(ctx context.Context, tenant uuid.UUID, messages []uuid.UUID, contextN int, at time.Time) (int, error) {
	return copyEvidence(ctx, tenant, messages, contextN, at, "user-anonymize")
}

// AnonymizeMessages implements write.LifecycleStore.
func (Lifecycle) AnonymizeMessages(ctx context.Context, tenant uuid.UUID, messages []uuid.UUID, at time.Time) (int, error) {
	n, err := Retention{}.Anonymize(ctx, tenant, messages, at)
	if err != nil || n == 0 {
		return n, err
	}
	// The quote of a reply carries a part of the quoted message: cleared with it.
	if err := clearReplyQuotes(ctx, tenant, messages); err != nil {
		return 0, err
	}
	return n, nil
}

// AnonymizeCard implements write.LifecycleStore.
func (Lifecycle) AnonymizeCard(ctx context.Context, tenant, user uuid.UUID) (bool, error) {
	// tenancyguard:ok projection ref.player_profiles of the session tenant (no deleted_at); source_version kept on purpose
	return changed(ctx, `UPDATE ref.player_profiles SET nickname = NULL, avatar_config = NULL, avatar_thumb_key = NULL, title = NULL
		WHERE tenant_id = $1 AND student_id = $2
			AND (nickname IS NOT NULL OR avatar_config IS NOT NULL OR avatar_thumb_key IS NOT NULL OR title IS NOT NULL)`, tenant, user)
}

// Relations implements write.LifecycleStore.
func (Lifecycle) Relations(ctx context.Context, tenant, user uuid.UUID) ([]write.Relation, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok social.blocks + social.mutes of the session tenant, live rows only
	rows, err := t.QueryContext(ctx, `SELECT 'block', b.blocker_id, b.blocked_id FROM social.blocks b WHERE b.tenant_id = $1
			AND b.deleted_at IS NULL AND (b.blocker_id = $2 OR b.blocked_id = $2)
		UNION ALL SELECT 'mute', m.muter_id, m.muted_id FROM social.mutes m WHERE m.tenant_id = $1
			AND m.deleted_at IS NULL AND (m.muter_id = $2 OR m.muted_id = $2)
		ORDER BY 1, 2, 3`, tenant, user)
	if err != nil {
		return nil, fmt.Errorf("chat: relations of user: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []write.Relation{}
	for rows.Next() {
		var r write.Relation
		if err := rows.Scan(&r.Kind, &r.From, &r.To); err != nil {
			return nil, fmt.Errorf("chat: relations of user: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ActiveSanctions implements write.LifecycleStore.
func (Lifecycle) ActiveSanctions(ctx context.Context, tenant, user uuid.UUID, at time.Time) ([]read.SanctionDetail, error) {
	// tenancyguard:ok chat.sanctions of the session tenant, live rows only
	return sanctions(ctx, `SELECT `+sanctionCols+` FROM chat.sanctions s WHERE s.tenant_id = $1 AND s.user_id = $2
		AND s.deleted_at IS NULL AND s.revoked_at IS NULL AND s.lifted_at IS NULL AND (s.ends_at IS NULL OR s.ends_at > $3)
		ORDER BY s.created_at, s.sanction_id`, tenant, user, at)
}

// SanctionExists implements write.LifecycleStore.
func (Lifecycle) SanctionExists(ctx context.Context, tenant, id uuid.UUID) (bool, error) {
	// tenancyguard:ok chat.sanctions of the session tenant (any row: idempotency of a copy)
	n, err := count(ctx, `SELECT count(*) FROM chat.sanctions s WHERE s.tenant_id = $1 AND s.sanction_id = $2`, tenant, id)
	if err != nil {
		return false, fmt.Errorf("chat: sanction exists: %w", err)
	}
	return n > 0, nil
}
