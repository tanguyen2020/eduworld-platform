package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// Safety implements write.SafetyStore and read.SafetyReader on the moderation tables of eduworld_chat (F121): explicit
// SQL with tenant_id and deleted_at conditions in every statement (R27). Cross-tenant statements (tenant uuid.Nil,
// work lists of the jobs) run only under tenancy.Bypass on the chat_platform pool; the callers decide when (ADR-0072).
type Safety struct{}

var (
	_ write.SafetyStore = Safety{}
	_ read.SafetyReader = Safety{}
)

type scanner interface{ Scan(...any) error }

const caseCols = `c.case_id, c.tenant_id, c.status, c.severity, c.source, c.category_code, c.channel_id, c.channel_type, c.target_type,
	c.target_id, c.school_ids, c.subject_user_ids, c.assignee_moderator_id, c.assigned_via, c.report_count, c.first_response_due_at,
	c.sla_due_at, c.first_response_at, c.escalated_at, c.hold_sanction_id, c.school_notified_at, c.school_ack_at, c.acknowledged_by,
	c.contact_outcome, c.contact_path, c.resolved_at, c.source_event_id, c.created_at, c.updated_at, c.version,
	(SELECT coalesce(rt.name, rt.code) FROM ref.tenants rt WHERE rt.tenant_id = c.tenant_id),
	(SELECT rt.tenant_type FROM ref.tenants rt WHERE rt.tenant_id = c.tenant_id)` // tenant_name, tenant_type (ref.tenants projection, F122 T064)

func scanCase(sc scanner) (*read.CaseRow, error) {
	var c read.CaseRow
	var category, channelType, via, outcome, tenantName, tenantType sql.NullString
	var channel, assignee, hold, ackBy, event uuid.NullUUID
	var firstAt, escAt, notified, acked, resolved sql.NullTime
	var schools, subjects []byte
	if err := sc.Scan(&c.ID, &c.TenantID, &c.Status, &c.Severity, &c.Source, &category, &channel, &channelType, &c.TargetType, &c.TargetID,
		&schools, &subjects, &assignee, &via, &c.ReportCount, &c.FirstResponseDue, &c.SLADue, &firstAt, &escAt, &hold, &notified, &acked,
		&ackBy, &outcome, &c.ContactPath, &resolved, &event, &c.CreatedAt, &c.UpdatedAt, &c.Version, &tenantName, &tenantType); err != nil {
		return nil, err
	}
	c.Category, c.ChannelID, c.ChannelType, c.Assignee, c.AssignedVia = nullString(category), nullUUID(channel), nullString(channelType),
		nullUUID(assignee), nullString(via)
	c.FirstResponseAt, c.EscalatedAt, c.HoldSanctionID, c.SchoolNotifiedAt, c.SchoolAckAt = nullTime(firstAt), nullTime(escAt),
		nullUUID(hold), nullTime(notified), nullTime(acked)
	c.TenantName, c.TenantType = nullString(tenantName), nullString(tenantType)
	c.AcknowledgedBy, c.ContactOutcome, c.ResolvedAt, c.SourceEventID = nullUUID(ackBy), nullString(outcome), nullTime(resolved), nullUUID(event)
	c.FirstResponseDue, c.SLADue, c.CreatedAt, c.UpdatedAt = c.FirstResponseDue.UTC(), c.SLADue.UTC(), c.CreatedAt.UTC(), c.UpdatedAt.UTC()
	if err := json.Unmarshal(schools, &c.SchoolIDs); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(subjects, &c.Subjects); err != nil {
		return nil, err
	}
	if c.SchoolIDs == nil {
		c.SchoolIDs = []uuid.UUID{}
	}
	if c.Subjects == nil {
		c.Subjects = []uuid.UUID{}
	}
	return &c, nil
}

func oneCase(ctx context.Context, query string, args ...any) (*read.CaseRow, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	c, err := scanCase(t.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: case: %w", err)
	}
	return c, nil
}

func jsonIDs(ids []uuid.UUID) string {
	if ids == nil {
		ids = []uuid.UUID{}
	}
	b, _ := json.Marshal(ids)
	return string(b)
}

// ---------- messages, channels, students ----------

// messageSQL finds a message of the tenant in both message tables (removed messages are soft-deleted: not found).
const messageSQL = `SELECT m.message_id, m.channel_id, coalesce(ch.channel_type, ''), m.seq, m.sender_id, m.school_id, m.created_at
	FROM chat.{table} m LEFT JOIN chat.channels ch ON ch.channel_id = m.channel_id AND ch.tenant_id = m.tenant_id AND ch.deleted_at IS NULL
	WHERE m.tenant_id = $1 AND m.message_id = $2 AND m.deleted_at IS NULL LIMIT 1`

// Message implements write.SafetyStore.
func (Safety) Message(ctx context.Context, tenant, id uuid.UUID) (*write.ReportedMessage, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	for _, table := range []string{"messages", "gameplay_messages"} {
		var m write.ReportedMessage
		var school uuid.NullUUID
		err := t.QueryRowContext(ctx, replaceTable(messageSQL, table), tenant, id).Scan(&m.ID, &m.ChannelID, &m.ChannelType, &m.Seq,
			&m.SenderID, &school, &m.CreatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("chat: reported message: %w", err)
		}
		m.SchoolID, m.CreatedAt = nullUUID(school), m.CreatedAt.UTC()
		return &m, nil
	}
	return nil, nil
}

func replaceTable(q, table string) string {
	out := []byte{}
	for i := 0; i < len(q); i++ {
		if i+7 <= len(q) && q[i:i+7] == "{table}" {
			out = append(out, table...)
			i += 6
			continue
		}
		out = append(out, q[i])
	}
	return string(out)
}

// Member implements write.SafetyStore.
func (Safety) Member(ctx context.Context, tenant, channel, user uuid.UUID, at time.Time) (bool, error) {
	n, err := count(ctx, `SELECT count(*) FROM chat.channel_members cm WHERE cm.tenant_id = $1 AND cm.channel_id = $2 AND cm.user_id = $3
		AND cm.deleted_at IS NULL AND cm.joined_at <= $4`, tenant, channel, user, at)
	return n > 0, err
}

// ChannelExists implements read.SafetyReader.
func (s Safety) ChannelExists(ctx context.Context, tenant, channel uuid.UUID) (string, bool, error) {
	return s.channelType(ctx, tenant, channel)
}

// channelType returns the type of a live channel (ok false: no live channel).
func (Safety) channelType(ctx context.Context, tenant, channel uuid.UUID) (string, bool, error) {
	t, err := readTx(ctx)
	if err != nil {
		return "", false, err
	}
	var typ string
	err = t.QueryRowContext(ctx, `SELECT ch.channel_type FROM chat.channels ch WHERE ch.tenant_id = $1 AND ch.channel_id = $2
		AND ch.deleted_at IS NULL`, tenant, channel).Scan(&typ)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("chat: channel type: %w", err)
	}
	return typ, true, nil
}

// ChannelType implements write.SafetyStore.
func (s Safety) ChannelType(ctx context.Context, tenant, channel uuid.UUID) (string, error) {
	typ, _, err := s.channelType(ctx, tenant, channel)
	return typ, err
}

// windowSQL reads ±n messages of one channel around seq of one message table (original content: body; the evidence of a
// case, never served to a player). seq 0: the newest messages.
const windowSQL = `(SELECT m.message_id, m.channel_id, m.seq, m.sender_id, m.school_id, m.kind, m.body, m.body_display, m.quick_message_id,
		m.revision, m.edited_at, m.moderation_state, m.recalled_at, m.created_at
	FROM chat.{table} m WHERE m.tenant_id = $1 AND m.channel_id = $2 AND ($3::bigint = 0 OR m.seq <= $3) AND m.deleted_at IS NULL
	ORDER BY m.seq DESC LIMIT $4)
	UNION ALL
	(SELECT m.message_id, m.channel_id, m.seq, m.sender_id, m.school_id, m.kind, m.body, m.body_display, m.quick_message_id,
		m.revision, m.edited_at, m.moderation_state, m.recalled_at, m.created_at
	FROM chat.{table} m WHERE m.tenant_id = $1 AND m.channel_id = $2 AND $3::bigint > 0 AND m.seq > $3 AND m.deleted_at IS NULL
	ORDER BY m.seq ASC LIMIT $5)`

// Window implements write.SafetyStore: before+1 messages up to seq (the message itself) and after messages after it.
func (s Safety) Window(ctx context.Context, tenant, channel uuid.UUID, seq int64, before, after int) ([]read.CaseMessage, error) {
	return s.ChannelWindow(ctx, tenant, channel, seq, before, after)
}

// ChannelWindow implements read.SafetyReader.
func (s Safety) ChannelWindow(ctx context.Context, tenant, channel uuid.UUID, seq int64, before, after int) ([]read.CaseMessage, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	typ, _, err := s.channelType(ctx, tenant, channel)
	if err != nil {
		return nil, err
	}
	var out []read.CaseMessage
	for _, table := range []string{"messages", "gameplay_messages"} {
		ms, err := windowOf(ctx, t, table, tenant, channel, seq, before, after)
		if err != nil {
			return nil, err
		}
		for i := range ms {
			ms[i].ChannelType = typ
		}
		out = append(out, ms...)
	}
	sortBySeq(out)
	if out == nil {
		out = []read.CaseMessage{}
	}
	return out, nil
}

func windowOf(ctx context.Context, t *sql.Tx, table string, tenant, channel uuid.UUID, seq int64, before, after int) ([]read.CaseMessage, error) {
	rows, err := t.QueryContext(ctx, replaceTable(windowSQL, table), tenant, channel, seq, before+1, after)
	if err != nil {
		return nil, fmt.Errorf("chat: case window: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.CaseMessage
	for rows.Next() {
		var m read.CaseMessage
		var school uuid.NullUUID
		var body, display, quick sql.NullString
		var edited, recalled sql.NullTime
		if err := rows.Scan(&m.MessageID, &m.ChannelID, &m.Seq, &m.SenderID, &school, &m.Kind, &body, &display, &quick, &m.Revision,
			&edited, &m.ModerationState, &recalled, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("chat: case window: %w", err)
		}
		m.SenderSchoolID, m.Body, m.BodyDisplay, m.QuickMessageID = nullUUID(school), nullString(body), nullString(display), nullString(quick)
		m.EditedAt, m.RecalledAt, m.CreatedAt = nullTime(edited), nullTime(recalled), m.CreatedAt.UTC()
		out = append(out, m)
	}
	return out, rows.Err()
}

func sortBySeq(ms []read.CaseMessage) {
	for i := 1; i < len(ms); i++ {
		for j := i; j > 0 && ms[j].Seq < ms[j-1].Seq; j-- {
			ms[j], ms[j-1] = ms[j-1], ms[j]
		}
	}
}

// Student implements write.SafetyStore (ref.students of the tenant).
func (Safety) Student(ctx context.Context, tenant, id uuid.UUID) (*write.SafetyStudent, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	var s write.SafetyStudent
	var deleted sql.NullTime
	err = t.QueryRowContext(ctx, `SELECT st.student_id, st.school_id, st.status, st.deleted_at FROM ref.students st
		WHERE st.tenant_id = $1 AND st.student_id = $2`, tenant, id).Scan(&s.ID, &s.SchoolID, &s.Status, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: student: %w", err)
	}
	s.Deleted = deleted.Valid
	return &s, nil
}

// TenantType implements write.SafetyStore (ref.tenants: school, unit, common).
func (Safety) TenantType(ctx context.Context, tenant uuid.UUID) (string, error) {
	t, err := readTx(ctx)
	if err != nil {
		return "", err
	}
	var typ string
	// tenancyguard:ok ref.tenants is the tenant registry projection (tenant_id is its key, no soft-delete)
	err = t.QueryRowContext(ctx, `SELECT tn.tenant_type FROM ref.tenants tn WHERE tn.tenant_id = $1`, tenant).Scan(&typ)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("chat: tenant type: %w", err)
	}
	return typ, nil
}

// ---------- cases ----------

// OpenCase implements write.SafetyStore.
func (Safety) OpenCase(ctx context.Context, tenant uuid.UUID, targetType string, target uuid.UUID, since time.Time) (*read.CaseRow, error) {
	return oneCase(ctx, `SELECT `+caseCols+` FROM chat.moderation_cases c WHERE c.tenant_id = $1 AND c.target_type = $2 AND c.target_id = $3
		AND c.status IN ('open','in_review') AND c.created_at > $4 AND c.deleted_at IS NULL ORDER BY c.created_at DESC LIMIT 1 FOR UPDATE`,
		tenant, targetType, target, since)
}

// CaseByEvent implements write.SafetyStore.
func (Safety) CaseByEvent(ctx context.Context, tenant, event uuid.UUID) (*read.CaseRow, error) {
	return oneCase(ctx, `SELECT `+caseCols+` FROM chat.moderation_cases c WHERE c.tenant_id = $1 AND c.source_event_id = $2
		AND c.deleted_at IS NULL`, tenant, event)
}

// CaseTenant implements write.SafetyStore (cross-tenant: only under tenancy.Bypass(moderation_case)).
func (Safety) CaseTenant(ctx context.Context, id uuid.UUID) (*uuid.UUID, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	var tenant uuid.UUID
	// tenancyguard:ok cross-tenant lookup of the tenant of one case under tenancy.Bypass(moderation_case), deleted_at IS NULL
	err = t.QueryRowContext(ctx, `SELECT c.tenant_id FROM chat.moderation_cases c WHERE c.case_id = $1 AND c.deleted_at IS NULL`, id).Scan(&tenant)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: case tenant: %w", err)
	}
	return &tenant, nil
}

// LockCase implements write.SafetyStore.
func (Safety) LockCase(ctx context.Context, tenant, id uuid.UUID) (*read.CaseRow, error) {
	return oneCase(ctx, `SELECT `+caseCols+` FROM chat.moderation_cases c WHERE c.tenant_id = $1 AND c.case_id = $2 AND c.deleted_at IS NULL
		FOR UPDATE`, tenant, id)
}

// Case implements read.SafetyReader (cross-tenant under tenancy.Bypass(moderation_case)).
func (Safety) Case(ctx context.Context, id uuid.UUID) (*read.CaseRow, error) {
	// tenancyguard:ok one case of any tenant under tenancy.Bypass(moderation_case) (ADR-0072); deleted_at IS NULL
	return oneCase(ctx, `SELECT `+caseCols+` FROM chat.moderation_cases c WHERE c.case_id = $1 AND c.deleted_at IS NULL`, id)
}

// InsertCase implements write.SafetyStore.
func (Safety) InsertCase(ctx context.Context, c read.CaseRow) error {
	_, err := exec(ctx, `INSERT INTO chat.moderation_cases (case_id, tenant_id, created_at, updated_at, version, school_ids, status, severity,
		source, category_code, channel_id, channel_type, target_type, target_id, subject_user_ids, assignee_moderator_id, assigned_via,
		report_count, first_response_due_at, sla_due_at, hold_sanction_id, contact_path, source_event_id)
		VALUES ($1, $2, $3, $3, $4, $5::jsonb, $6, $7, $8, $9, $10, $11, $12, $13, $14::jsonb, $15, $16, $17, $18, $19, $20, $21, $22)`,
		c.ID, c.TenantID, c.CreatedAt, c.Version, jsonIDs(c.SchoolIDs), c.Status, c.Severity, c.Source, c.Category, c.ChannelID, c.ChannelType,
		c.TargetType, c.TargetID, jsonIDs(c.Subjects), c.Assignee, c.AssignedVia, c.ReportCount, c.FirstResponseDue, c.SLADue, c.HoldSanctionID,
		c.ContactPath, c.SourceEventID)
	if err != nil {
		return fmt.Errorf("chat: insert case: %w", err)
	}
	return nil
}

// UpdateCase implements write.SafetyStore.
func (Safety) UpdateCase(ctx context.Context, c read.CaseRow, actor *uuid.UUID) error {
	n, err := exec(ctx, `UPDATE chat.moderation_cases SET updated_at = now(), updated_by = $3, version = $4, school_ids = $5::jsonb,
		status = $6, severity = $7, category_code = $8, subject_user_ids = $9::jsonb, assignee_moderator_id = $10, assigned_via = $11,
		report_count = $12, first_response_due_at = $13, sla_due_at = $14, first_response_at = $15, escalated_at = $16,
		hold_sanction_id = $17, school_notified_at = $18, school_ack_at = $19, acknowledged_by = $20, contact_outcome = $21,
		contact_path = $22, resolved_at = $23
		WHERE tenant_id = $1 AND case_id = $2 AND version = $4 - 1 AND deleted_at IS NULL`,
		c.TenantID, c.ID, actor, c.Version, jsonIDs(c.SchoolIDs), c.Status, c.Severity, c.Category, jsonIDs(c.Subjects), c.Assignee,
		c.AssignedVia, c.ReportCount, c.FirstResponseDue, c.SLADue, c.FirstResponseAt, c.EscalatedAt, c.HoldSanctionID, c.SchoolNotifiedAt,
		c.SchoolAckAt, c.AcknowledgedBy, c.ContactOutcome, c.ContactPath, c.ResolvedAt)
	if err != nil {
		return fmt.Errorf("chat: update case: %w", err)
	}
	if n == 0 {
		return write.ErrStale
	}
	return nil
}

// ---------- reports, actions, evidence ----------

// InsertReport implements write.SafetyStore.
func (Safety) InsertReport(ctx context.Context, r write.NewReport) (uuid.UUID, bool, error) {
	var context_ any
	if len(r.Context) > 0 {
		b, _ := json.Marshal(r.Context)
		context_ = string(b)
	}
	n, err := exec(ctx, `INSERT INTO chat.reports (report_id, tenant_id, created_at, updated_at, school_id, reporter_id, target_type,
		target_id, target_user_id, reason, note, context, case_id, weight, status, evidence_id)
		VALUES ($1, $2, $3, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb, $12, $13, 'open', $14)
		ON CONFLICT (reporter_id, target_type, target_id, case_id) WHERE deleted_at IS NULL DO NOTHING`,
		r.ID, r.TenantID, r.CreatedAt, r.SchoolID, r.ReporterID, r.TargetType, r.TargetID, r.TargetUserID, r.Reason, r.Note, context_,
		r.CaseID, r.Weight, r.EvidenceID)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("chat: insert report: %w", err)
	}
	if n == 1 {
		return r.ID, true, nil
	}
	t, err := readTx(ctx)
	if err != nil {
		return uuid.Nil, false, err
	}
	var id uuid.UUID
	if err := t.QueryRowContext(ctx, `SELECT r.report_id FROM chat.reports r WHERE r.tenant_id = $1 AND r.reporter_id = $2
		AND r.target_type = $3 AND r.target_id = $4 AND r.case_id = $5 AND r.deleted_at IS NULL`, r.TenantID, r.ReporterID, r.TargetType,
		r.TargetID, r.CaseID).Scan(&id); err != nil {
		return uuid.Nil, false, fmt.Errorf("chat: existing report: %w", err)
	}
	return id, false, nil
}

// DismissedReports implements write.SafetyStore.
func (Safety) DismissedReports(ctx context.Context, tenant, reporter uuid.UUID, since time.Time) (int, error) {
	return count(ctx, `SELECT count(*) FROM chat.reports r WHERE r.tenant_id = $1 AND r.reporter_id = $2 AND r.status = 'dismissed'
		AND r.created_at >= $3 AND r.deleted_at IS NULL`, tenant, reporter, since)
}

// SetReportStatus implements write.SafetyStore.
func (Safety) SetReportStatus(ctx context.Context, tenant, caseID uuid.UUID, status string) error {
	_, err := exec(ctx, `UPDATE chat.reports SET status = $3, updated_at = now(), version = version + 1
		WHERE tenant_id = $1 AND case_id = $2 AND deleted_at IS NULL AND status <> $3`, tenant, caseID, status)
	if err != nil {
		return fmt.Errorf("chat: report status: %w", err)
	}
	return nil
}

// CaseReports implements write.SafetyStore and read.SafetyReader.
func (Safety) CaseReports(ctx context.Context, tenant, caseID uuid.UUID) ([]read.ReportRow, error) {
	return caseReports(ctx, tenant, caseID, 100)
}

func caseReports(ctx context.Context, tenant, caseID uuid.UUID, limit int) ([]read.ReportRow, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, `SELECT r.report_id, r.reporter_id, r.school_id, r.reason, r.note, r.status, r.created_at
		FROM chat.reports r WHERE r.tenant_id = $1 AND r.case_id = $2 AND r.deleted_at IS NULL ORDER BY r.created_at, r.report_id LIMIT $3`,
		tenant, caseID, limit)
	if err != nil {
		return nil, fmt.Errorf("chat: case reports: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []read.ReportRow{}
	for rows.Next() {
		var r read.ReportRow
		var school uuid.NullUUID
		var note sql.NullString
		if err := rows.Scan(&r.ID, &r.ReporterID, &school, &r.Reason, &note, &r.Status, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("chat: case reports: %w", err)
		}
		r.SchoolID, r.Note, r.CreatedAt = nullUUID(school), nullString(note), r.CreatedAt.UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

// InsertAction implements write.SafetyStore.
func (Safety) InsertAction(ctx context.Context, a write.NewAction) error {
	_, err := exec(ctx, `INSERT INTO chat.moderation_actions (action_id, tenant_id, created_at, created_by, case_id, action, actor_id,
		actor_kind, note, sanction_id, message_id) VALUES ($10, $1, $2, $3, $4, $5, $3, $6, $7, $8, $9)`,
		a.TenantID, a.At, a.ActorID, a.CaseID, a.Action, a.ActorKind, a.Note, a.SanctionID, a.MessageID, ids.New())
	if err != nil {
		return fmt.Errorf("chat: case action: %w", err)
	}
	return nil
}

// CaseActions implements write.SafetyStore.
func (Safety) CaseActions(ctx context.Context, tenant, caseID uuid.UUID) ([]read.ActionRow, error) {
	return caseActions(ctx, tenant, caseID, 100)
}

func caseActions(ctx context.Context, tenant, caseID uuid.UUID, limit int) ([]read.ActionRow, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok append-only history (no deleted_at): tenant_id and case_id conditions
	rows, err := t.QueryContext(ctx, `SELECT a.action_id, a.action, a.actor_id, a.actor_kind, a.note, a.sanction_id, a.message_id, a.created_at
		FROM chat.moderation_actions a WHERE a.tenant_id = $1 AND a.case_id = $2 ORDER BY a.created_at, a.action_id LIMIT $3`,
		tenant, caseID, limit)
	if err != nil {
		return nil, fmt.Errorf("chat: case actions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []read.ActionRow{}
	for rows.Next() {
		var a read.ActionRow
		var actor, sanction, msg uuid.NullUUID
		var note sql.NullString
		if err := rows.Scan(&a.ID, &a.Action, &actor, &a.ActorKind, &note, &sanction, &msg, &a.At); err != nil {
			return nil, fmt.Errorf("chat: case actions: %w", err)
		}
		a.ActorID, a.Note, a.SanctionID, a.MessageID, a.At = nullUUID(actor), nullString(note), nullUUID(sanction), nullUUID(msg), a.At.UTC()
		out = append(out, a)
	}
	return out, rows.Err()
}

// InsertEvidence implements write.SafetyStore.
func (Safety) InsertEvidence(ctx context.Context, e write.NewEvidence) error {
	_, err := exec(ctx, `INSERT INTO chat.moderation_evidence (evidence_id, tenant_id, created_at, case_id, report_id, school_id, channel_id,
		snapshot, sha256) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9)`,
		e.ID, e.TenantID, e.At, e.CaseID, e.ReportID, e.SchoolID, e.ChannelID, string(e.Snapshot), e.SHA256)
	if err != nil {
		return fmt.Errorf("chat: insert evidence: %w", err)
	}
	return nil
}

// CaseEvidence implements write.SafetyStore and read.SafetyReader.
func (Safety) CaseEvidence(ctx context.Context, tenant, caseID uuid.UUID) ([]read.EvidenceRow, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok immutable evidence (no deleted_at, ADR-0033): tenant_id and case_id conditions
	rows, err := t.QueryContext(ctx, `SELECT e.evidence_id, e.case_id, e.channel_id, e.snapshot, e.created_at FROM chat.moderation_evidence e
		WHERE e.tenant_id = $1 AND e.case_id = $2 ORDER BY e.created_at, e.evidence_id LIMIT 50`, tenant, caseID)
	if err != nil {
		return nil, fmt.Errorf("chat: case evidence: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []read.EvidenceRow{}
	for rows.Next() {
		var e read.EvidenceRow
		var ch uuid.NullUUID
		var snap []byte
		if err := rows.Scan(&e.ID, &e.CaseID, &ch, &snap, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("chat: case evidence: %w", err)
		}
		e.ChannelID, e.Snapshot, e.CreatedAt = nullUUID(ch), json.RawMessage(snap), e.CreatedAt.UTC()
		out = append(out, e)
	}
	return out, rows.Err()
}

// RemoveMessage implements write.SafetyStore: moderation_state removed + soft-delete; body stays as evidence (ADR-0034).
func (Safety) RemoveMessage(ctx context.Context, tenant, id, by uuid.UUID, at time.Time) (bool, error) {
	total := int64(0)
	for _, table := range []string{"messages", "gameplay_messages"} {
		n, err := exec(ctx, replaceTable(`UPDATE chat.{table} SET moderation_state = 'removed', deleted_at = $3, deleted_by = $4
			WHERE tenant_id = $1 AND message_id = $2 AND deleted_at IS NULL`, table), tenant, id, at, by)
		if err != nil {
			return false, fmt.Errorf("chat: remove message: %w", err)
		}
		total += n
	}
	return total > 0, nil
}

// ---------- sanctions ----------

const sanctionCols = `s.sanction_id, s.tenant_id, s.school_id, s.user_id, s.case_id, s.type, s.scope, s.channel_id, s.reason,
	coalesce(s.reason_code, s.reason), s.issued_by_kind, s.issued_by, s.starts_at, s.ends_at, s.revoked_at, s.revoked_by, s.revoke_reason,
	s.lifted_at, s.created_at, s.version`

func scanSanction(sc scanner) (*read.SanctionDetail, error) {
	var s read.SanctionDetail
	var school, caseID, channel, issuer, revoker uuid.NullUUID
	var ends, revoked, lifted sql.NullTime
	var revokeReason sql.NullString
	if err := sc.Scan(&s.ID, &s.TenantID, &school, &s.UserID, &caseID, &s.Type, &s.Scope, &channel, &s.Reason, &s.ReasonCode,
		&s.IssuedByKind, &issuer, &s.StartsAt, &ends, &revoked, &revoker, &revokeReason, &lifted, &s.CreatedAt, &s.Version); err != nil {
		return nil, err
	}
	s.SchoolID, s.CaseID, s.ChannelID, s.IssuedBy, s.RevokedBy = nullUUID(school), nullUUID(caseID), nullUUID(channel), nullUUID(issuer),
		nullUUID(revoker)
	s.EndsAt, s.RevokedAt, s.LiftedAt, s.RevokeReason = nullTime(ends), nullTime(revoked), nullTime(lifted), nullString(revokeReason)
	s.StartsAt, s.CreatedAt = s.StartsAt.UTC(), s.CreatedAt.UTC()
	return &s, nil
}

func sanctions(ctx context.Context, query string, args ...any) ([]read.SanctionDetail, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("chat: sanctions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []read.SanctionDetail{}
	for rows.Next() {
		s, err := scanSanction(rows)
		if err != nil {
			return nil, fmt.Errorf("chat: sanctions: %w", err)
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

// InsertSanction implements write.SafetyStore.
func (Safety) InsertSanction(ctx context.Context, s write.SafetySanction) error {
	_, err := exec(ctx, `INSERT INTO chat.sanctions (sanction_id, tenant_id, created_at, created_by, updated_at, school_id, user_id, type,
		scope, channel_id, reason, case_id, issued_by, starts_at, ends_at, reason_code, issued_by_kind)
		VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8, $9, $10, $11, $4, $3, $12, $13, $14)`,
		s.ID, s.TenantID, s.StartsAt, s.IssuedBy, s.SchoolID, s.UserID, s.Type, s.Scope, s.ChannelID, s.Reason, s.CaseID, s.EndsAt,
		s.ReasonCode, s.IssuedByKind)
	if err != nil {
		return fmt.Errorf("chat: insert sanction: %w", err)
	}
	return nil
}

// ActiveHold implements write.SafetyStore.
func (Safety) ActiveHold(ctx context.Context, tenant, user uuid.UUID) (*read.SanctionDetail, error) {
	list, err := sanctions(ctx, `SELECT `+sanctionCols+` FROM chat.sanctions s WHERE s.tenant_id = $1 AND s.user_id = $2
		AND coalesce(s.reason_code, s.reason) IN ('case_hold','block_and_alert_hold') AND s.ends_at IS NULL AND s.revoked_at IS NULL
		AND s.lifted_at IS NULL AND s.deleted_at IS NULL ORDER BY s.created_at LIMIT 1 FOR UPDATE`, tenant, user)
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return &list[0], nil
}

// Held implements command.HoldChecker (friend requests of a held student, F121-AC-04).
func (Safety) Held(ctx context.Context, tenant, user uuid.UUID) (bool, error) {
	n, err := count(ctx, `SELECT count(*) FROM chat.sanctions s WHERE s.tenant_id = $1 AND s.user_id = $2
		AND coalesce(s.reason_code, s.reason) IN ('case_hold','block_and_alert_hold') AND s.revoked_at IS NULL AND s.lifted_at IS NULL
		AND (s.ends_at IS NULL OR s.ends_at > now()) AND s.deleted_at IS NULL`, tenant, user)
	return n > 0, err
}

// LinkSanction implements write.SafetyStore.
func (Safety) LinkSanction(ctx context.Context, tenant, sanction, caseID uuid.UUID) error {
	_, err := exec(ctx, `UPDATE chat.sanctions SET case_id = $3, updated_at = now(), version = version + 1
		WHERE tenant_id = $1 AND sanction_id = $2 AND case_id IS NULL AND deleted_at IS NULL`, tenant, sanction, caseID)
	if err != nil {
		return fmt.Errorf("chat: link sanction: %w", err)
	}
	return nil
}

// LockSanction implements write.SafetyStore.
func (Safety) LockSanction(ctx context.Context, tenant, id uuid.UUID) (*read.SanctionDetail, error) {
	list, err := sanctions(ctx, `SELECT `+sanctionCols+` FROM chat.sanctions s WHERE s.tenant_id = $1 AND s.sanction_id = $2
		AND s.deleted_at IS NULL FOR UPDATE`, tenant, id)
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return &list[0], nil
}

// Revoke implements write.SafetyStore.
func (Safety) Revoke(ctx context.Context, tenant, id uuid.UUID, by *uuid.UUID, reason string, at time.Time) error {
	_, err := exec(ctx, `UPDATE chat.sanctions SET revoked_at = $3, revoked_by = $4, revoke_reason = $5, lifted_at = $3, updated_at = now(),
		updated_by = $4, version = version + 1 WHERE tenant_id = $1 AND sanction_id = $2 AND revoked_at IS NULL AND deleted_at IS NULL`,
		tenant, id, at, by, reason)
	if err != nil {
		return fmt.Errorf("chat: revoke sanction: %w", err)
	}
	return nil
}

// MarkLifted implements write.SafetyStore.
func (Safety) MarkLifted(ctx context.Context, tenant, id uuid.UUID, at time.Time) error {
	_, err := exec(ctx, `UPDATE chat.sanctions SET lifted_at = $3, updated_at = now(), version = version + 1
		WHERE tenant_id = $1 AND sanction_id = $2 AND lifted_at IS NULL AND deleted_at IS NULL`, tenant, id, at)
	if err != nil {
		return fmt.Errorf("chat: lift sanction: %w", err)
	}
	return nil
}

// CaseSanctions implements write.SafetyStore.
func (Safety) CaseSanctions(ctx context.Context, tenant, caseID uuid.UUID) ([]read.SanctionDetail, error) {
	return sanctions(ctx, `SELECT `+sanctionCols+` FROM chat.sanctions s WHERE s.tenant_id = $1 AND s.case_id = $2 AND s.deleted_at IS NULL
		ORDER BY s.created_at, s.sanction_id`, tenant, caseID)
}

// ---------- school notifications ----------

const notificationCols = `n.notification_id, n.tenant_id, n.case_id, n.school_id, n.severity, n.category_code, n.students,
	n.measures_taken, n.summary, n.school_notified_at, n.ack_due_at, n.reminder_count, n.last_reminded_at, n.school_ack_at,
	n.acknowledged_by, n.contact_outcome, n.ack_note, n.created_at, n.version`

func scanNotification(sc scanner) (*read.NotificationRow, error) {
	var n read.NotificationRow
	var category, measures, outcome, note sql.NullString
	var reminded, acked sql.NullTime
	var ackBy uuid.NullUUID
	var students []byte
	if err := sc.Scan(&n.ID, &n.TenantID, &n.CaseID, &n.SchoolID, &n.Severity, &category, &students, &measures, &n.Summary,
		&n.SchoolNotifiedAt, &n.AckDueAt, &n.ReminderCount, &reminded, &acked, &ackBy, &outcome, &note, &n.CreatedAt, &n.Version); err != nil {
		return nil, err
	}
	n.Category, n.MeasuresTaken, n.ContactOutcome, n.AckNote = nullString(category), nullString(measures), nullString(outcome), nullString(note)
	n.LastRemindedAt, n.SchoolAckAt, n.AcknowledgedBy = nullTime(reminded), nullTime(acked), nullUUID(ackBy)
	n.SchoolNotifiedAt, n.AckDueAt, n.CreatedAt = n.SchoolNotifiedAt.UTC(), n.AckDueAt.UTC(), n.CreatedAt.UTC()
	if err := json.Unmarshal(students, &n.Students); err != nil {
		return nil, err
	}
	return &n, nil
}

func notifications(ctx context.Context, query string, args ...any) ([]read.NotificationRow, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("chat: school notifications: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []read.NotificationRow{}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, fmt.Errorf("chat: school notifications: %w", err)
		}
		out = append(out, *n)
	}
	return out, rows.Err()
}

// InsertNotification implements write.SafetyStore.
func (Safety) InsertNotification(ctx context.Context, n read.NotificationRow, actor uuid.UUID) error {
	students, _ := json.Marshal(n.Students)
	_, err := exec(ctx, `INSERT INTO chat.school_safety_notifications (notification_id, tenant_id, created_at, created_by, updated_at,
		case_id, school_id, severity, category_code, students, measures_taken, summary, school_notified_at, ack_due_at)
		VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8, $9::jsonb, $10, $11, $3, $12)`,
		n.ID, n.TenantID, n.SchoolNotifiedAt, actor, n.CaseID, n.SchoolID, n.Severity, n.Category, string(students), n.MeasuresTaken,
		n.Summary, n.AckDueAt)
	if err != nil {
		return fmt.Errorf("chat: insert school notification: %w", err)
	}
	return nil
}

// LockNotification implements write.SafetyStore.
func (Safety) LockNotification(ctx context.Context, tenant, id uuid.UUID) (*read.NotificationRow, error) {
	list, err := notifications(ctx, `SELECT `+notificationCols+` FROM chat.school_safety_notifications n WHERE n.tenant_id = $1
		AND n.notification_id = $2 AND n.deleted_at IS NULL FOR UPDATE`, tenant, id)
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return &list[0], nil
}

// UpdateNotification implements write.SafetyStore.
func (Safety) UpdateNotification(ctx context.Context, n read.NotificationRow, actor *uuid.UUID) error {
	c, err := exec(ctx, `UPDATE chat.school_safety_notifications SET updated_at = now(), updated_by = $3, version = $4, reminder_count = $5,
		last_reminded_at = $6, school_ack_at = $7, acknowledged_by = $8, contact_outcome = $9, ack_note = $10
		WHERE tenant_id = $1 AND notification_id = $2 AND version = $4 - 1 AND deleted_at IS NULL`,
		n.TenantID, n.ID, actor, n.Version, n.ReminderCount, n.LastRemindedAt, n.SchoolAckAt, n.AcknowledgedBy, n.ContactOutcome, n.AckNote)
	if err != nil {
		return fmt.Errorf("chat: update school notification: %w", err)
	}
	if c == 0 {
		return write.ErrStale
	}
	return nil
}

// PendingNotification implements write.SafetyStore.
func (Safety) PendingNotification(ctx context.Context, tenant, caseID, school uuid.UUID) (*read.NotificationRow, error) {
	list, err := notifications(ctx, `SELECT `+notificationCols+` FROM chat.school_safety_notifications n WHERE n.tenant_id = $1
		AND n.case_id = $2 AND n.school_id = $3 AND n.school_ack_at IS NULL AND n.deleted_at IS NULL LIMIT 1`, tenant, caseID, school)
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return &list[0], nil
}

// CaseNotifications implements read.SafetyReader.
func (Safety) CaseNotifications(ctx context.Context, tenant, caseID uuid.UUID) ([]read.NotificationRow, error) {
	return notifications(ctx, `SELECT `+notificationCols+` FROM chat.school_safety_notifications n WHERE n.tenant_id = $1 AND n.case_id = $2
		AND n.deleted_at IS NULL ORDER BY n.created_at, n.notification_id LIMIT 20`, tenant, caseID)
}

// ---------- work lists of the platform jobs (tenancy.Bypass(platform_job)) ----------

// OverdueCases implements write.SafetyStore.
func (Safety) OverdueCases(ctx context.Context, now time.Time, limit int) ([]read.CaseRow, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok cross-tenant work list of moderation-sla-escalator under tenancy.Bypass(platform_job); deleted_at IS NULL
	rows, err := t.QueryContext(ctx, `SELECT `+caseCols+` FROM chat.moderation_cases c WHERE c.deleted_at IS NULL
		AND c.status IN ('open','in_review','reopened') AND c.escalated_at IS NULL
		AND ((c.first_response_at IS NULL AND c.first_response_due_at <= $1) OR c.sla_due_at <= $1)
		ORDER BY c.first_response_due_at LIMIT $2`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("chat: overdue cases: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.CaseRow
	for rows.Next() {
		c, err := scanCase(rows)
		if err != nil {
			return nil, fmt.Errorf("chat: overdue cases: %w", err)
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// DueNotifications implements write.SafetyStore.
func (Safety) DueNotifications(ctx context.Context, now time.Time, limit int) ([]read.NotificationRow, error) {
	// tenancyguard:ok cross-tenant work list of school-notification-reminder under tenancy.Bypass(platform_job); deleted_at IS NULL
	return notifications(ctx, `SELECT `+notificationCols+` FROM chat.school_safety_notifications n WHERE n.deleted_at IS NULL
		AND n.school_ack_at IS NULL AND ((n.reminder_count = 0 AND n.ack_due_at <= $1)
			OR (n.reminder_count = 1 AND n.ack_due_at + (n.ack_due_at - n.school_notified_at) <= $1))
		ORDER BY n.ack_due_at LIMIT $2`, now, limit)
}

// ExpiredSanctions implements write.SafetyStore.
func (Safety) ExpiredSanctions(ctx context.Context, now time.Time, limit int) ([]read.SanctionDetail, error) {
	// tenancyguard:ok cross-tenant work list of sanction-expirer under tenancy.Bypass(platform_job); deleted_at IS NULL
	return sanctions(ctx, `SELECT `+sanctionCols+` FROM chat.sanctions s WHERE s.deleted_at IS NULL AND s.lifted_at IS NULL
		AND s.revoked_at IS NULL AND s.ends_at IS NOT NULL AND s.ends_at <= $1 ORDER BY s.ends_at LIMIT $2`, now, limit)
}

// StaleHolds implements write.SafetyStore.
func (Safety) StaleHolds(ctx context.Context, limit int) ([]read.SanctionDetail, error) {
	// tenancyguard:ok cross-tenant safety net of sanction-expirer under tenancy.Bypass(platform_job); deleted_at IS NULL
	return sanctions(ctx, `SELECT `+sanctionCols+` FROM chat.sanctions s JOIN chat.moderation_cases c ON c.case_id = s.case_id
		AND c.tenant_id = s.tenant_id AND c.deleted_at IS NULL
		WHERE s.deleted_at IS NULL AND s.lifted_at IS NULL AND s.revoked_at IS NULL
		AND coalesce(s.reason_code, s.reason) IN ('case_hold','block_and_alert_hold') AND c.status = 'resolved'
		ORDER BY s.created_at LIMIT $1`, limit)
}
