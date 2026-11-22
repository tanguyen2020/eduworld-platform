// Package purge is the ONLY code of the platform that deletes rows physically (ADR-0033, AGENTS R21): the job
// gameplay-chat-purge of worker-chat, connected with the database role chat_purger (EW_DB_CHAT_PURGER_DSN). It deletes
// the messages, members and channels of closed gameplay channels (match, party, team bound to a match) past
// purge_after, and world/area messages past ephemeral_chat_retention, after copying every message referenced by a
// report, a case or a case action (and its context) into chat.moderation_evidence. The role and its RLS policies
// (migration 20261231000004) let it delete nothing else: never a club team channel, a direct channel, chat.messages or
// any other table (F117-AC-20). tools/tenancyguard allows DELETE in this package only.
package purge

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// evidenceNamespace derives the id of a purge evidence copy from (case, message): copying again after a failed run
// inserts nothing twice (ON CONFLICT DO NOTHING).
var evidenceNamespace = uuid.MustParse("2b6a3f0e-8c1d-5f4e-9a7b-3c2d1e0f4a5b")

// EvidenceID is the id of the purge copy of message for case.
func EvidenceID(kind string, caseID, message uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(evidenceNamespace, []byte(kind+":"+caseID.String()+":"+message.String()))
}

// Snapshot kinds of moderation_evidence written by the purge (F121 reads target_id and messages).
const (
	SnapshotGameplay  = "gameplay_purge"
	SnapshotEphemeral = "ephemeral_purge"
)

// Store implements port.PurgeStore with explicit SQL in the transaction of ctx (UnitOfWork of the chat_purger pool).
type Store struct{}

var _ port.PurgeStore = Store{}

func tx(ctx context.Context) (*sql.Tx, error) {
	t, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	return t, nil
}

// ModerationReady implements port.PurgeStore (research R3): the moderation tables of F121/F116 exist and the purger
// can read them and write the evidence. Anything missing answers false: nothing is deleted.
func (Store) ModerationReady(ctx context.Context) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	var present bool
	if err := t.QueryRowContext(ctx, `SELECT to_regclass('chat.reports') IS NOT NULL AND to_regclass('chat.moderation_cases') IS NOT NULL
		AND to_regclass('chat.moderation_actions') IS NOT NULL AND to_regclass('chat.sanctions') IS NOT NULL
		AND to_regclass('chat.moderation_evidence') IS NOT NULL`).Scan(&present); err != nil {
		return false, fmt.Errorf("purge: moderation tables: %w", err)
	}
	if !present {
		return false, nil
	}
	var ok bool
	if err := t.QueryRowContext(ctx, `SELECT has_table_privilege('chat.reports', 'SELECT')
		AND has_table_privilege('chat.moderation_cases', 'SELECT') AND has_table_privilege('chat.moderation_actions', 'SELECT')
		AND has_table_privilege('chat.sanctions', 'SELECT') AND has_table_privilege('chat.moderation_evidence', 'INSERT')`).Scan(&ok); err != nil {
		return false, fmt.Errorf("purge: moderation privileges: %w", err)
	}
	return ok, nil
}

// Expired implements port.PurgeStore: across tenants, through the purger policy (closed gameplay channels only).
func (Store) Expired(ctx context.Context, now time.Time, limit int) ([]port.ExpiredChannel, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// Soft-deleted gameplay channels are purged too (deleted_at not filtered on purpose).
	// tenancyguard:ok cross-tenant list of the purger (RLS purger_gameplay_only: closed gameplay channels only)
	rows, err := t.QueryContext(ctx, `SELECT c.channel_id, c.tenant_id, c.channel_type, c.purge_after FROM chat.channels c
		WHERE c.is_gameplay AND c.status = 'closed' AND c.purge_after IS NOT NULL AND c.purge_after < $1
		ORDER BY c.purge_after, c.channel_id LIMIT $2`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("purge: expired channels: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.ExpiredChannel
	for rows.Next() {
		var c port.ExpiredChannel
		if err := rows.Scan(&c.ID, &c.TenantID, &c.ChannelType, &c.PurgeAfter); err != nil {
			return nil, fmt.Errorf("purge: expired channels: %w", err)
		}
		c.PurgeAfter = c.PurgeAfter.UTC()
		out = append(out, c)
	}
	return out, rows.Err()
}

// Holds implements port.PurgeStore (F117-AC-18).
func (Store) Holds(ctx context.Context, tenant, channel uuid.UUID) (port.Holds, error) {
	t, err := tx(ctx)
	if err != nil {
		return port.Holds{}, err
	}
	var h port.Holds
	// tenancyguard:ok reports/cases of the session tenant, live rows; gameplay_messages of the tenant (removed ones too)
	err = t.QueryRowContext(ctx, `SELECT
		(SELECT count(*) FROM chat.reports r WHERE r.tenant_id = $1 AND r.deleted_at IS NULL AND r.status = 'open'
			AND ((r.target_type = 'message' AND EXISTS (SELECT 1 FROM chat.gameplay_messages m WHERE m.tenant_id = $1
				AND m.channel_id = $2 AND m.message_id = r.target_id)) OR r.context->>'channel_id' = $2::text)),
		(SELECT count(*) FROM chat.moderation_cases mc WHERE mc.tenant_id = $1 AND mc.deleted_at IS NULL
			AND mc.status IN ('open','in_review','escalated','reopened')
			AND (mc.channel_id = $2 OR (mc.target_type = 'message' AND EXISTS (SELECT 1 FROM chat.gameplay_messages m
				WHERE m.tenant_id = $1 AND m.channel_id = $2 AND m.message_id = mc.target_id))))`, tenant, channel).
		Scan(&h.OpenReports, &h.OpenCases)
	if err != nil {
		return h, fmt.Errorf("purge: holds: %w", err)
	}
	return h, nil
}

// refsSQL lists the (message, case, report) references of the moderation into the messages of {table} selected by
// {where}: reports and cases targeting a message and case actions naming one (sanctions act through their case).
// Every reference counts, open or closed, soft-deleted or not (evidence is never lost).
//
// tenancyguard:ok moderation sources of the session tenant (tenant_id; deleted_at not filtered on purpose: evidence)
const refsSQL = `SELECT DISTINCT ON (x.case_id, x.message_id) x.message_id, x.case_id, x.report_id FROM (
		SELECT r.target_id AS message_id, r.case_id, r.report_id FROM chat.reports r WHERE r.tenant_id = $1 AND r.target_type = 'message'
		UNION ALL SELECT mc.target_id, mc.case_id, NULL::uuid FROM chat.moderation_cases mc WHERE mc.tenant_id = $1 AND mc.target_type = 'message'
		UNION ALL SELECT a.message_id, a.case_id, NULL::uuid FROM chat.moderation_actions a WHERE a.tenant_id = $1 AND a.message_id IS NOT NULL
	) x JOIN chat.{table} m ON m.message_id = x.message_id AND m.tenant_id = $1 AND {where}
	ORDER BY x.case_id, x.message_id, x.report_id NULLS LAST`

type ref struct {
	message, caseID uuid.UUID
	report          *uuid.UUID
}

func refs(ctx context.Context, t *sql.Tx, table, where string, args ...any) ([]ref, error) {
	q := replace(refsSQL, map[string]string{"{table}": table, "{where}": where})
	// tenancyguard:ok moderation sources of the session tenant (tenant_id; deleted_at not filtered on purpose: evidence)
	rows, err := t.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("purge: references: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []ref
	for rows.Next() {
		var r ref
		var report uuid.NullUUID
		if err := rows.Scan(&r.message, &r.caseID, &report); err != nil {
			return nil, fmt.Errorf("purge: references: %w", err)
		}
		if report.Valid {
			id := report.UUID
			r.report = &id
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func replace(s string, kv map[string]string) string {
	pairs := make([]string, 0, 2*len(kv))
	for k, v := range kv {
		pairs = append(pairs, k, v)
	}
	return strings.NewReplacer(pairs...).Replace(s)
}

// message is one message copied into an evidence snapshot (original content, revision state).
type message struct {
	read.CaseMessage
	SchoolID       *uuid.UUID
	CatalogVersion *string
}

const gameplayColumns = `m.message_id, m.channel_id, m.seq, m.sender_id, m.school_id, m.kind, m.body, m.body_display,
	m.quick_message_id, m.revision, m.edited_at, m.moderation_state, m.recalled_at, m.created_at, m.filter_verdict->>'catalog_version'`

func scanMessages(rows *sql.Rows, channelType string) ([]message, error) {
	defer func() { _ = rows.Close() }()
	var out []message
	for rows.Next() {
		var m message
		var school uuid.NullUUID
		var body, display, quick, catalog sql.NullString
		var edited, recalled sql.NullTime
		if err := rows.Scan(&m.MessageID, &m.ChannelID, &m.Seq, &m.SenderID, &school, &m.Kind, &body, &display, &quick, &m.Revision,
			&edited, &m.ModerationState, &recalled, &m.CreatedAt, &catalog); err != nil {
			return nil, fmt.Errorf("purge: messages: %w", err)
		}
		m.ChannelType = channelType
		if school.Valid {
			s := school.UUID
			m.SenderSchoolID, m.SchoolID = &s, &s
		}
		m.Body, m.BodyDisplay, m.QuickMessageID = str(body), str(display), str(quick)
		m.EditedAt, m.RecalledAt, m.CreatedAt = tm(edited), tm(recalled), m.CreatedAt.UTC()
		m.CatalogVersion = str(catalog)
		out = append(out, m)
	}
	return out, rows.Err()
}

func str(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	v := n.String
	return &v
}

func tm(n sql.NullTime) *time.Time {
	if !n.Valid {
		return nil
	}
	v := n.Time.UTC()
	return &v
}

// snapshot is moderation_evidence.snapshot of a purge copy (the shape of F121: target_id + messages).
type snapshot struct {
	Kind      string             `json:"kind"`
	TargetID  uuid.UUID          `json:"target_id"`
	ChannelID *uuid.UUID         `json:"channel_id,omitempty"`
	Context   map[string]string  `json:"context,omitempty"`
	Messages  []read.CaseMessage `json:"messages"`
	TakenAt   time.Time          `json:"taken_at"`
}

func insertEvidence(ctx context.Context, t *sql.Tx, tenant uuid.UUID, r ref, target message, channel *uuid.UUID, s snapshot, kind string) (bool, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return false, err
	}
	sum := sha256.Sum256(b)
	res, err := t.ExecContext(ctx, `INSERT INTO chat.moderation_evidence (evidence_id, tenant_id, created_at, case_id, report_id,
		school_id, channel_id, snapshot, sha256) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9) ON CONFLICT (evidence_id) DO NOTHING`,
		EvidenceID(kind, r.caseID, r.message), tenant, s.TakenAt, r.caseID, r.report, target.SchoolID, channel, string(b),
		hex.EncodeToString(sum[:]))
	if err != nil {
		return false, fmt.Errorf("purge: insert evidence: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

func plain(ms []message) []read.CaseMessage {
	out := make([]read.CaseMessage, len(ms))
	for i, m := range ms {
		out[i] = m.CaseMessage
	}
	return out
}

// PurgeChannel implements port.PurgeStore (F117-AC-16, AC-17).
func (Store) PurgeChannel(ctx context.Context, c port.ExpiredChannel, contextN, batch int, at time.Time) (port.ChannelPurge, error) {
	var out port.ChannelPurge
	t, err := tx(ctx)
	if err != nil {
		return out, err
	}
	var scope []byte
	// tenancyguard:ok the closed gameplay channel of the session tenant (purger policy); locked for the purge
	if err := t.QueryRowContext(ctx, `SELECT c.scope_ref FROM chat.channels c WHERE c.tenant_id = $1 AND c.channel_id = $2
		AND c.is_gameplay AND c.status = 'closed' FOR UPDATE`, c.TenantID, c.ID).
		Scan(&scope); err != nil {
		return out, fmt.Errorf("purge: channel: %w", err)
	}
	scopeRef := map[string]string{}
	if len(scope) > 0 {
		_ = json.Unmarshal(scope, &scopeRef)
	}
	// 1. Rollup of the daily statistics (kept for analytics, no content).
	// tenancyguard:ok gameplay messages and reports of the session tenant (every row counted, deleted_at not filtered)
	rows, err := t.QueryContext(ctx, `SELECT m.created_on, count(*),
			count(*) FILTER (WHERE m.moderation_state = 'masked'),
			count(*) FILTER (WHERE m.moderation_state IN ('held','removed')),
			count(*) FILTER (WHERE EXISTS (SELECT 1 FROM chat.reports r WHERE r.tenant_id = $1 AND r.target_type = 'message'
				AND r.target_id = m.message_id))
		FROM chat.gameplay_messages m WHERE m.tenant_id = $1 AND m.channel_id = $2 GROUP BY m.created_on ORDER BY m.created_on`,
		c.TenantID, c.ID)
	if err != nil {
		return out, fmt.Errorf("purge: rollup: %w", err)
	}
	if out.Stats, err = dayStats(rows, c.ChannelType); err != nil {
		return out, err
	}
	for _, s := range out.Stats {
		// tenancyguard:ok aggregate table of the session tenant (no deleted_at: rollups are never deleted)
		if _, err := t.ExecContext(ctx, `INSERT INTO chat.gameplay_chat_daily_stats AS s (tenant_id, day, channel_type, channels, messages,
			masked, blocked, reported) VALUES ($1, $2::date, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (tenant_id, day, channel_type) DO UPDATE SET channels = s.channels + EXCLUDED.channels,
				messages = s.messages + EXCLUDED.messages, masked = s.masked + EXCLUDED.masked, blocked = s.blocked + EXCLUDED.blocked,
				reported = s.reported + EXCLUDED.reported, updated_at = now()`,
			c.TenantID, s.Day.Format("2006-01-02"), s.ChannelType, s.Channels, s.Messages, s.Masked, s.Blocked, s.Reported); err != nil {
			return out, fmt.Errorf("purge: rollup: %w", err)
		}
	}
	// 2. Evidence: every referenced message with ±contextN messages, before any delete.
	rs, err := refs(ctx, t, "gameplay_messages", "m.channel_id = $2", c.TenantID, c.ID)
	if err != nil {
		return out, err
	}
	for _, r := range rs {
		target, window, err := gameplayWindow(ctx, t, c, r.message, contextN)
		if err != nil {
			return out, err
		}
		if err := withRevisions(ctx, t, c.TenantID, window); err != nil {
			return out, err
		}
		ctxMap := map[string]string{"channel_type": c.ChannelType, "case_id": r.caseID.String(), "source": "gameplay-chat-purge"}
		for _, k := range []string{"match_id", "party_id", "team_no"} {
			if v := scopeRef[k]; v != "" {
				ctxMap[k] = v
			}
		}
		if r.report != nil {
			ctxMap["report_id"] = r.report.String()
		}
		if target.CatalogVersion != nil {
			ctxMap["catalog_version"] = *target.CatalogVersion
		}
		id := c.ID
		inserted, err := insertEvidence(ctx, t, c.TenantID, r, target, &id, snapshot{Kind: SnapshotGameplay, TargetID: r.message, ChannelID: &id,
			Context: ctxMap, Messages: plain(window), TakenAt: at.UTC()}, SnapshotGameplay)
		if err != nil {
			return out, err
		}
		if inserted {
			out.Evidence++
		}
	}
	// 3. Delete the side tables of the channel (F118: revisions, reactions, pins — their evidence is in the snapshots of
	// step 2), then the messages in batches, then members, then the channel; mark the daily partitions.
	for _, tb := range []string{"message_revisions", "message_reactions", "pinned_messages"} {
		// tenancyguard:ok physical delete of the purge (ADR-0033): side rows of the closed gameplay channel of the tenant
		if _, err := t.ExecContext(ctx, `DELETE FROM chat.`+tb+` WHERE tenant_id = $1 AND channel_id = $2`, c.TenantID, c.ID); err != nil {
			return out, fmt.Errorf("purge: delete %s: %w", tb, err)
		}
	}
	for {
		// tenancyguard:ok physical delete of the purge (ADR-0033): messages of the closed gameplay channel of the tenant
		res, err := t.ExecContext(ctx, `DELETE FROM chat.gameplay_messages WHERE (message_id, created_on) IN (
			SELECT m.message_id, m.created_on FROM chat.gameplay_messages m WHERE m.tenant_id = $1 AND m.channel_id = $2 LIMIT $3)`,
			c.TenantID, c.ID, batch)
		if err != nil {
			return out, fmt.Errorf("purge: delete messages: %w", err)
		}
		n, _ := res.RowsAffected()
		out.Messages += n
		if n < int64(batch) {
			break
		}
	}
	// tenancyguard:ok partition index of the purge (no deleted_at): the channel is purged in every day it wrote
	if _, err := t.ExecContext(ctx, `UPDATE chat.gameplay_partition_channels SET purged_at = $3
		WHERE tenant_id = $1 AND channel_id = $2 AND purged_at IS NULL`, c.TenantID, c.ID, at); err != nil {
		return out, fmt.Errorf("purge: partition channels: %w", err)
	}
	// tenancyguard:ok physical delete of the purge (ADR-0033): members of the closed gameplay channel of the tenant
	if _, err := t.ExecContext(ctx, `DELETE FROM chat.channel_members WHERE tenant_id = $1 AND channel_id = $2`, c.TenantID, c.ID); err != nil {
		return out, fmt.Errorf("purge: delete members: %w", err)
	}
	// tenancyguard:ok physical delete of the purge (ADR-0033): the closed gameplay channel itself
	res, err := t.ExecContext(ctx, `DELETE FROM chat.channels WHERE tenant_id = $1 AND channel_id = $2 AND is_gameplay
		AND status = 'closed'`, c.TenantID, c.ID)
	if err != nil {
		return out, fmt.Errorf("purge: delete channel: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return out, errors.New("purge: channel not deleted (not a closed gameplay channel of the tenant)")
	}
	return out, nil
}

func gameplayWindow(ctx context.Context, t *sql.Tx, c port.ExpiredChannel, target uuid.UUID, n int) (message, []message, error) {
	// tenancyguard:ok gameplay messages of the session tenant (removed and recalled ones too: original content of evidence)
	rows, err := t.QueryContext(ctx, `SELECT `+gameplayColumns+` FROM chat.gameplay_messages m
		WHERE m.tenant_id = $1 AND m.channel_id = $2 AND m.seq BETWEEN
			(SELECT t.seq FROM chat.gameplay_messages t WHERE t.tenant_id = $1 AND t.channel_id = $2 AND t.message_id = $3 LIMIT 1) - $4
			AND (SELECT t.seq FROM chat.gameplay_messages t WHERE t.tenant_id = $1 AND t.channel_id = $2 AND t.message_id = $3 LIMIT 1) + $4
		ORDER BY m.seq`, c.TenantID, c.ID, target, n)
	if err != nil {
		return message{}, nil, fmt.Errorf("purge: window: %w", err)
	}
	ms, err := scanMessages(rows, c.ChannelType)
	if err != nil {
		return message{}, nil, err
	}
	for _, m := range ms {
		if m.MessageID == target {
			return m, ms, nil
		}
	}
	return message{}, nil, fmt.Errorf("purge: referenced message %s not found in its channel", target)
}

var partitionRe = regexp.MustCompile(`^(gameplay|ephemeral)_messages_p([0-9]{8})$`)

func partitions(ctx context.Context, t *sql.Tx, table string) ([]time.Time, error) {
	rows, err := t.QueryContext(ctx, `SELECT c.relname FROM pg_catalog.pg_inherits i
		JOIN pg_catalog.pg_class c ON c.oid = i.inhrelid JOIN pg_catalog.pg_class p ON p.oid = i.inhparent
		JOIN pg_catalog.pg_namespace n ON n.oid = p.relnamespace WHERE n.nspname = 'chat' AND p.relname = $1 ORDER BY c.relname`, table)
	if err != nil {
		return nil, fmt.Errorf("purge: partitions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []time.Time
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		m := partitionRe.FindStringSubmatch(name)
		if m == nil {
			continue // the default partition
		}
		d, err := time.Parse("20060102", m[2])
		if err == nil {
			out = append(out, d)
		}
	}
	return out, rows.Err()
}

// DropGameplayPartitions implements port.PurgeStore: a day partition goes only when every channel with messages that day
// was purged (gameplay_partition_channels) and it holds no row (checked again by chat.drop_gameplay_partition).
func (Store) DropGameplayPartitions(ctx context.Context) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	days, err := partitions(ctx, t, "gameplay_messages")
	if err != nil {
		return 0, err
	}
	dropped := 0
	for _, d := range days {
		var pending bool
		// tenancyguard:ok partition index across tenants (purger policy, no deleted_at): any channel of the day not purged
		if err := t.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM chat.gameplay_partition_channels p
			WHERE p.partition_day = $1::date AND p.purged_at IS NULL)`, d.Format("2006-01-02")).Scan(&pending); err != nil {
			return dropped, fmt.Errorf("purge: partition day: %w", err)
		}
		if pending {
			continue
		}
		var action string
		if err := t.QueryRowContext(ctx, `SELECT chat.drop_gameplay_partition($1::date)`, d.Format("2006-01-02")).Scan(&action); err != nil {
			return dropped, fmt.Errorf("purge: drop partition: %w", err)
		}
		if action == "dropped" {
			dropped++
		}
	}
	return dropped, nil
}

// EphemeralTenants implements port.PurgeStore.
func (Store) EphemeralTenants(ctx context.Context, cutoff time.Time) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok cross-tenant list of the purger (policy purger_expired_only), deleted_at not filtered on purpose
	rows, err := t.QueryContext(ctx, `SELECT DISTINCT e.tenant_id FROM chat.ephemeral_messages e WHERE e.created_at < $1`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("purge: ephemeral tenants: %w", err)
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

const ephemeralColumns = `m.message_id, m.channel_id, 0::bigint, m.sender_id, m.school_id, m.kind, m.body, m.body, m.quick_message_id,
	1, NULL::timestamptz, m.moderation_state, NULL::timestamptz, m.created_at, m.filter_verdict->>'catalog_version'`

// CopyEphemeralEvidence implements port.PurgeStore (F117-AC-21).
func (Store) CopyEphemeralEvidence(ctx context.Context, tenant uuid.UUID, cutoff time.Time, contextN int, at time.Time) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	rs, err := refs(ctx, t, "ephemeral_messages", "m.created_at < $2", tenant, cutoff)
	if err != nil {
		return 0, err
	}
	copied := 0
	for _, r := range rs {
		// tenancyguard:ok world/area messages of the session tenant around the referenced one (deleted ones too: evidence)
		rows, err := t.QueryContext(ctx, `(SELECT `+ephemeralColumns+` FROM chat.ephemeral_messages m
				WHERE m.tenant_id = $1 AND m.channel_id = (SELECT x.channel_id FROM chat.ephemeral_messages x WHERE x.tenant_id = $1
					AND x.message_id = $2 LIMIT 1)
				AND m.created_at <= (SELECT x.created_at FROM chat.ephemeral_messages x WHERE x.tenant_id = $1 AND x.message_id = $2 LIMIT 1)
				ORDER BY m.created_at DESC LIMIT $3 + 1)
			UNION ALL
			(SELECT `+ephemeralColumns+` FROM chat.ephemeral_messages m
				WHERE m.tenant_id = $1 AND m.channel_id = (SELECT x.channel_id FROM chat.ephemeral_messages x WHERE x.tenant_id = $1
					AND x.message_id = $2 LIMIT 1)
				AND m.created_at > (SELECT x.created_at FROM chat.ephemeral_messages x WHERE x.tenant_id = $1 AND x.message_id = $2 LIMIT 1)
				ORDER BY m.created_at ASC LIMIT $3)`, tenant, r.message, contextN)
		if err != nil {
			return copied, fmt.Errorf("purge: ephemeral window: %w", err)
		}
		ms, err := scanMessages(rows, "")
		if err != nil {
			return copied, err
		}
		sortByTime(ms)
		var target *message
		for i := range ms {
			if ms[i].MessageID == r.message {
				target = &ms[i]
			}
		}
		if target == nil {
			return copied, fmt.Errorf("purge: referenced ephemeral message %s not found", r.message)
		}
		ctxMap := map[string]string{"case_id": r.caseID.String(), "source": "gameplay-chat-purge"}
		if r.report != nil {
			ctxMap["report_id"] = r.report.String()
		}
		if target.CatalogVersion != nil {
			ctxMap["catalog_version"] = *target.CatalogVersion
		}
		ch := target.ChannelID
		inserted, err := insertEvidence(ctx, t, tenant, r, *target, &ch, snapshot{Kind: SnapshotEphemeral, TargetID: r.message,
			ChannelID: &ch, Context: ctxMap, Messages: plain(ms), TakenAt: at.UTC()}, SnapshotEphemeral)
		if err != nil {
			return copied, err
		}
		if inserted {
			copied++
		}
	}
	return copied, nil
}

func sortByTime(ms []message) {
	for i := 1; i < len(ms); i++ {
		for j := i; j > 0 && ms[j].CreatedAt.Before(ms[j-1].CreatedAt); j-- {
			ms[j], ms[j-1] = ms[j-1], ms[j]
		}
	}
	for i := range ms {
		ms[i].Seq = int64(i + 1) // position in the snapshot (ephemeral messages have no seq)
	}
}

// DropEphemeralPartitions implements port.PurgeStore.
func (Store) DropEphemeralPartitions(ctx context.Context, cutoff time.Time) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	days, err := partitions(ctx, t, "ephemeral_messages")
	if err != nil {
		return 0, err
	}
	dropped := 0
	for _, d := range days {
		if d.AddDate(0, 0, 1).After(cutoff) {
			continue
		}
		var action string
		if err := t.QueryRowContext(ctx, `SELECT chat.drop_ephemeral_partition($1::date, $2)`, d.Format("2006-01-02"), cutoff).
			Scan(&action); err != nil {
			return dropped, fmt.Errorf("purge: drop ephemeral partition: %w", err)
		}
		if action == "dropped" {
			dropped++
		}
	}
	return dropped, nil
}

// DeleteEphemeral implements port.PurgeStore.
func (Store) DeleteEphemeral(ctx context.Context, tenant uuid.UUID, cutoff time.Time, batch int) (int64, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var total int64
	for {
		// tenancyguard:ok physical delete of the purge (ADR-0033): world/area messages of the tenant past ephemeral_chat_retention
		res, err := t.ExecContext(ctx, `DELETE FROM chat.ephemeral_messages WHERE (message_id, created_on) IN (
			SELECT m.message_id, m.created_on FROM chat.ephemeral_messages m WHERE m.tenant_id = $1 AND m.created_at < $2 LIMIT $3)`,
			tenant, cutoff, batch)
		if err != nil {
			return total, fmt.Errorf("purge: delete ephemeral: %w", err)
		}
		n, _ := res.RowsAffected()
		total += n
		if n < int64(batch) {
			return total, nil
		}
	}
}

// RecordRun implements port.PurgeStore.
func (Store) RecordRun(ctx context.Context, r port.PurgeRun) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok append-only run record (no deleted_at), tenant of the session or the run-level row
	if _, err := t.ExecContext(ctx, `INSERT INTO chat.gameplay_purge_runs (row_id, run_id, tenant_id, kind, started_at, finished_at,
		channels_purged, messages_purged, partitions_dropped, deferred_open_case, evidence_copied, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`, ids.New(), r.RunID, r.TenantID, r.Kind, r.StartedAt, r.FinishedAt,
		r.ChannelsPurged, r.MessagesPurged, r.PartitionsDropped, r.DeferredOpenCase, r.EvidenceCopied, r.Status); err != nil {
		return fmt.Errorf("purge: run record: %w", err)
	}
	return nil
}

// EphemeralRetention implements port.PurgeStore.
func (Store) EphemeralRetention(ctx context.Context) (*time.Duration, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var raw sql.NullString
	// tenancyguard:ok ref.platform_settings is a G projection (no tenant, no deleted_at)
	err = t.QueryRowContext(ctx, `SELECT ps.value->>'ephemeral_retention_hours' FROM ref.platform_settings ps WHERE ps.key = 'chat_retention'`).
		Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !raw.Valid) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("purge: ephemeral retention: %w", err)
	}
	h, convErr := strconv.Atoi(raw.String)
	if convErr != nil || h < 1 || h > 168 {
		return nil, nil //nolint:nilerr // malformed projected value: the configured default applies
	}
	d := time.Duration(h) * time.Hour
	return &d, nil
}

// EnsurePartitions implements port.PurgeStore (job chat-partition-maintainer).
func (Store) EnsurePartitions(ctx context.Context, day time.Time, days int) (int, map[string]int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, nil, err
	}
	var created int
	if err := t.QueryRowContext(ctx, `SELECT chat.ensure_message_partitions($1::date, $2)`, day.Format("2006-01-02"), days).
		Scan(&created); err != nil {
		return 0, nil, fmt.Errorf("purge: ensure partitions: %w", err)
	}
	ahead := map[string]int{}
	for _, table := range []string{"gameplay_messages", "ephemeral_messages"} {
		var n int
		if err := t.QueryRowContext(ctx, `SELECT chat.message_partition_days_ahead($1, $2::date)`, table, day.Format("2006-01-02")).
			Scan(&n); err != nil {
			return created, ahead, fmt.Errorf("purge: partition days ahead: %w", err)
		}
		ahead[table] = n
	}
	return created, ahead, nil
}

func dayStats(rows *sql.Rows, channelType string) ([]port.DayStats, error) {
	defer func() { _ = rows.Close() }()
	var out []port.DayStats
	for rows.Next() {
		s := port.DayStats{ChannelType: channelType, Channels: 1}
		if err := rows.Scan(&s.Day, &s.Messages, &s.Masked, &s.Blocked, &s.Reported); err != nil {
			return nil, fmt.Errorf("purge: rollup: %w", err)
		}
		s.Day = s.Day.UTC()
		out = append(out, s)
	}
	return out, rows.Err()
}

// withRevisions attaches to each message of an evidence window its rows of chat.message_revisions (earlier versions and
// held edit proposals, F118-AC-06): the purge copies them before deleting them.
func withRevisions(ctx context.Context, t *sql.Tx, tenant uuid.UUID, ms []message) error {
	for i := range ms {
		rv, err := revisionsOf(ctx, t, tenant, ms[i].MessageID)
		if err != nil {
			return err
		}
		ms[i].Revisions = rv
	}
	return nil
}

func revisionsOf(ctx context.Context, t *sql.Tx, tenant, msg uuid.UUID) ([]read.CaseRevision, error) {
	// tenancyguard:ok evidence rows of one message of the session tenant (append-only table, no deleted_at)
	rows, err := t.QueryContext(ctx, `SELECT r.revision, r.state, r.body, r.body_display, r.decision, r.edited_at
		FROM chat.message_revisions r WHERE r.tenant_id = $1 AND r.message_id = $2 ORDER BY r.revision, r.state`, tenant, msg)
	if err != nil {
		return nil, fmt.Errorf("purge: revisions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.CaseRevision
	for rows.Next() {
		var rv read.CaseRevision
		var body, display sql.NullString
		if err := rows.Scan(&rv.Revision, &rv.State, &body, &display, &rv.Decision, &rv.EditedAt); err != nil {
			return nil, fmt.Errorf("purge: revisions: %w", err)
		}
		rv.Body, rv.BodyDisplay, rv.EditedAt = str(body), str(display), rv.EditedAt.UTC()
		out = append(out, rv)
	}
	return out, rows.Err()
}
