package persistence

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
)

// Retention implements port.RetentionStore (F120 US6) on chat.messages, chat.message_revisions, chat.moderation_evidence
// and chat.persistent_retention_runs: UPDATE only (anonymisation), never DELETE (R21, ADR-0070).
type Retention struct{}

var _ port.RetentionStore = Retention{}

// retentionNamespace derives the idempotent evidence ids of the retention copies (one per case and message).
var retentionNamespace = uuid.MustParse("3a7e2c41-9b0d-5f6e-8a1c-2d4b6e8f0a13")

// SnapshotPersistentRetention is the snapshot kind of moderation_evidence written by persistent-chat-retention.
const SnapshotPersistentRetention = "persistent_retention"

// Tenants implements port.RetentionStore.
func (Retention) Tenants(ctx context.Context) ([]uuid.UUID, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok ref.tenants is not tenant-scoped (projection of identity, no deleted_at)
	rows, err := t.QueryContext(ctx, `SELECT tn.tenant_id FROM ref.tenants tn ORDER BY tn.tenant_id`)
	if err != nil {
		return nil, fmt.Errorf("chat: retention tenants: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// PlatformDays implements port.RetentionStore.
func (Retention) PlatformDays(ctx context.Context) (*int, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	var raw []byte
	// tenancyguard:ok ref.platform_settings is a G projection (no tenant, no deleted_at)
	err = t.QueryRowContext(ctx, `SELECT s.value FROM ref.platform_settings s WHERE s.key = 'chat_retention'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: chat_retention: %w", err)
	}
	var v struct {
		Days *int `json:"persistent_days"`
	}
	_ = json.Unmarshal(raw, &v) // a malformed projection falls back to the configured default
	if v.Days == nil || *v.Days < 1 {
		return nil, nil
	}
	return v.Days, nil
}

// Expired implements port.RetentionStore.
func (Retention) Expired(ctx context.Context, tenant uuid.UUID, days, limit int, now time.Time) ([]port.RetentionMessage, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// Persistent channels (club team, direct), every message past retention not anonymised yet — deleted ones too:
	// anonymisation is data minimisation.
	// tenancyguard:ok chat.messages + chat.channels of the session tenant, deleted rows included on purpose (retention)
	rows, err := t.QueryContext(ctx, `SELECT m.message_id, m.channel_id, m.created_at FROM chat.messages m
		JOIN chat.channels c ON c.tenant_id = m.tenant_id AND c.channel_id = m.channel_id
			AND NOT c.is_gameplay AND c.channel_type IN ('team', 'direct')
		WHERE m.tenant_id = $1 AND m.anonymized_at IS NULL
			AND m.created_at < $2::timestamptz - make_interval(days => coalesce(c.retention_days, $3)::int)
		ORDER BY m.created_at, m.message_id LIMIT $4`, tenant, now, days, limit)
	if err != nil {
		return nil, fmt.Errorf("chat: retention expired: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.RetentionMessage
	for rows.Next() {
		var m port.RetentionMessage
		if err := rows.Scan(&m.MessageID, &m.ChannelID, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// retentionRefsSQL lists the (message, case, report) references of the moderation into the given messages: reports and
// cases targeting a message and case actions naming one, open or closed (evidence is never lost).
//
// tenancyguard:ok moderation sources of the session tenant (tenant_id; deleted_at not filtered on purpose: evidence)
const retentionRefsSQL = `SELECT DISTINCT ON (x.case_id, x.message_id) x.message_id, x.case_id, x.report_id FROM (
		SELECT r.target_id AS message_id, r.case_id, r.report_id FROM chat.reports r WHERE r.tenant_id = $1 AND r.target_type = 'message'
			AND r.case_id IS NOT NULL
		UNION ALL SELECT mc.target_id, mc.case_id, NULL::uuid FROM chat.moderation_cases mc WHERE mc.tenant_id = $1 AND mc.target_type = 'message'
		UNION ALL SELECT a.message_id, a.case_id, NULL::uuid FROM chat.moderation_actions a WHERE a.tenant_id = $1 AND a.message_id IS NOT NULL
	) x WHERE x.message_id = ANY($2::uuid[])
	ORDER BY x.case_id, x.message_id, x.report_id NULLS LAST`

const persistentColumns = `m.message_id, m.channel_id, m.seq, m.sender_id, m.school_id, m.kind, m.body, m.body_display, m.quick_message_id,
	m.revision, m.edited_at, m.moderation_state, m.recalled_at, m.created_at`

// CopyEvidence implements port.RetentionStore.
func (Retention) CopyEvidence(ctx context.Context, tenant uuid.UUID, messages []uuid.UUID, contextN int, at time.Time) (int, error) {
	return copyEvidence(ctx, tenant, messages, contextN, at, "persistent-chat-retention")
}

// copyEvidence copies the referenced messages (±contextN) into chat.moderation_evidence before an anonymisation; source
// names the caller in the snapshot context. The evidence id is one per case and message whatever the source, so the
// retention job and the anonymisation of a user (F115 T042) never copy the same message twice.
func copyEvidence(ctx context.Context, tenant uuid.UUID, messages []uuid.UUID, contextN int, at time.Time, source string) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	refs, err := retentionRefs(ctx, t, tenant, messages)
	if err != nil {
		return 0, err
	}
	copied := 0
	for _, r := range refs {
		ms, err := retentionWindow(ctx, t, tenant, r.message, contextN)
		if err != nil {
			return copied, err
		}
		var target *read.CaseMessage
		for i := range ms {
			if ms[i].MessageID == r.message {
				target = &ms[i]
			}
		}
		if target == nil {
			return copied, fmt.Errorf("chat: referenced message %s not found", r.message)
		}
		ctxMap := map[string]string{"case_id": r.caseID.String(), "source": source}
		if r.report != nil {
			ctxMap["report_id"] = r.report.String()
		}
		ch := target.ChannelID
		snap := map[string]any{"kind": SnapshotPersistentRetention, "target_id": r.message, "channel_id": ch, "context": ctxMap,
			"messages": ms, "taken_at": at.UTC()}
		b, err := json.Marshal(snap)
		if err != nil {
			return copied, err
		}
		sum := sha256.Sum256(b)
		id := uuid.NewSHA1(retentionNamespace, []byte(SnapshotPersistentRetention+":"+r.caseID.String()+":"+r.message.String()))
		res, err := t.ExecContext(ctx, `INSERT INTO chat.moderation_evidence (evidence_id, tenant_id, created_at, case_id, report_id,
			school_id, channel_id, snapshot, sha256) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9) ON CONFLICT (evidence_id) DO NOTHING`,
			id, tenant, at.UTC(), r.caseID, r.report, target.SenderSchoolID, ch, string(b), hex.EncodeToString(sum[:]))
		if err != nil {
			return copied, fmt.Errorf("chat: retention evidence: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 1 {
			copied++
		}
	}
	return copied, nil
}

type retentionRef struct {
	message, caseID uuid.UUID
	report          *uuid.UUID
}

func retentionRefs(ctx context.Context, t *sql.Tx, tenant uuid.UUID, messages []uuid.UUID) ([]retentionRef, error) {
	rows, err := t.QueryContext(ctx, retentionRefsSQL, tenant, idStrings(messages))
	if err != nil {
		return nil, fmt.Errorf("chat: retention references: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []retentionRef
	for rows.Next() {
		var r retentionRef
		var report uuid.NullUUID
		if err := rows.Scan(&r.message, &r.caseID, &report); err != nil {
			return nil, err
		}
		r.report = nullUUID(report)
		out = append(out, r)
	}
	return out, rows.Err()
}

// retentionWindow returns the referenced message and ±n messages around it, in seq order (original content: evidence).
func retentionWindow(ctx context.Context, t *sql.Tx, tenant, message uuid.UUID, n int) ([]read.CaseMessage, error) {
	// tenancyguard:ok chat.messages of the session tenant around the referenced message (deleted ones too: evidence)
	rows, err := t.QueryContext(ctx, `(SELECT `+persistentColumns+` FROM chat.messages m
			WHERE m.tenant_id = $1 AND m.channel_id = (SELECT x.channel_id FROM chat.messages x WHERE x.tenant_id = $1 AND x.message_id = $2 LIMIT 1)
			AND m.seq <= (SELECT x.seq FROM chat.messages x WHERE x.tenant_id = $1 AND x.message_id = $2 LIMIT 1)
			ORDER BY m.seq DESC LIMIT $3 + 1)
		UNION ALL
		(SELECT `+persistentColumns+` FROM chat.messages m
			WHERE m.tenant_id = $1 AND m.channel_id = (SELECT x.channel_id FROM chat.messages x WHERE x.tenant_id = $1 AND x.message_id = $2 LIMIT 1)
			AND m.seq > (SELECT x.seq FROM chat.messages x WHERE x.tenant_id = $1 AND x.message_id = $2 LIMIT 1)
			ORDER BY m.seq ASC LIMIT $3)`, tenant, message, n)
	if err != nil {
		return nil, fmt.Errorf("chat: retention window: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var ms []read.CaseMessage
	for rows.Next() {
		var m read.CaseMessage
		var sc uuid.NullUUID
		var body, display, quick sql.NullString
		var edited, recalled sql.NullTime
		if err := rows.Scan(&m.MessageID, &m.ChannelID, &m.Seq, &m.SenderID, &sc, &m.Kind, &body, &display, &quick, &m.Revision,
			&edited, &m.ModerationState, &recalled, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.SenderSchoolID = nullUUID(sc)
		m.Body, m.BodyDisplay, m.QuickMessageID = nullString(body), nullString(display), nullString(quick)
		m.EditedAt, m.RecalledAt, m.CreatedAt = nullTime(edited), nullTime(recalled), m.CreatedAt.UTC()
		ms = append(ms, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].Seq < ms[j].Seq })
	return ms, nil
}

// Anonymize implements port.RetentionStore.
func (Retention) Anonymize(ctx context.Context, tenant uuid.UUID, messages []uuid.UUID, at time.Time) (int, error) {
	if len(messages) == 0 {
		return 0, nil
	}
	// tenancyguard:ok chat.messages of the session tenant (anonymisation, never DELETE — R21)
	n, err := exec(ctx, `UPDATE chat.messages SET body = NULL, body_display = NULL, quote = NULL, anonymized_at = $3
		WHERE tenant_id = $1 AND message_id = ANY($2::uuid[]) AND anonymized_at IS NULL`, tenant, idStrings(messages), at)
	if err != nil {
		return 0, fmt.Errorf("chat: anonymize messages: %w", err)
	}
	// tenancyguard:ok chat.message_revisions of the session tenant (body/body_display only: chat.f_revisions_guard)
	if _, err := exec(ctx, `UPDATE chat.message_revisions SET body = NULL, body_display = NULL
		WHERE tenant_id = $1 AND message_id = ANY($2::uuid[]) AND (body IS NOT NULL OR body_display IS NOT NULL)`,
		tenant, idStrings(messages)); err != nil {
		return 0, fmt.Errorf("chat: anonymize revisions: %w", err)
	}
	return int(n), nil
}

// RecordRun implements port.RetentionStore.
func (Retention) RecordRun(ctx context.Context, r port.RetentionRun) error {
	var code any
	if r.ErrorCode != "" {
		code = r.ErrorCode
	}
	_, err := exec(ctx, `INSERT INTO chat.persistent_retention_runs (run_id, tenant_id, started_at, finished_at, channels_scanned,
		messages_anonymized, evidence_copied, status, error_code) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		r.ID, r.TenantID, r.StartedAt, r.FinishedAt, r.Channels, r.Anonymized, r.EvidenceCopied, r.Status, code)
	if err != nil {
		return fmt.Errorf("chat: retention run: %w", err)
	}
	return nil
}

// clearReplyQuotes clears the quote of the replies to anonymised messages (the quote carries a part of the quoted text;
// used by the anonymisation of a user, F115 T042). UPDATE only, the reply itself is kept.
func clearReplyQuotes(ctx context.Context, tenant uuid.UUID, messages []uuid.UUID) error {
	// tenancyguard:ok chat.messages of the session tenant (anonymisation, never DELETE — R21)
	if _, err := exec(ctx, `UPDATE chat.messages SET quote = NULL WHERE tenant_id = $1 AND reply_to_message_id = ANY($2::uuid[])
		AND quote IS NOT NULL`, tenant, idStrings(messages)); err != nil {
		return fmt.Errorf("chat: anonymize quotes: %w", err)
	}
	return nil
}
