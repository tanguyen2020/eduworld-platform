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
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/notification"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// Notifications implements write.NotificationStore on social.notifications (hand-written partitioned table, F123) and the
// projections ref.students / ref.player_profiles: explicit SQL in the transaction of ctx, tenant_id and deleted_at in every
// statement (R27), never a DELETE (R21).
type Notifications struct{}

var _ write.NotificationStore = Notifications{}

// NotificationReader implements read.NotificationReader (read transaction of ctx).
type NotificationReader struct{}

var _ read.NotificationReader = NotificationReader{}

const notificationColumns = `notification_id, tenant_id, recipient_id, kind, message_code, params::text, ref_type, ref_id,
	action::text, created_at, read_at`

func uuidList(list []uuid.UUID) string {
	s := make([]string, len(list))
	for i, id := range list {
		s[i] = id.String()
	}
	return "{" + join(s) + "}"
}

// Recipients implements write.NotificationStore.
func (Notifications) Recipients(ctx context.Context, tenant uuid.UUID, students []uuid.UUID) (map[uuid.UUID]notification.Recipient, error) {
	out := map[uuid.UUID]notification.Recipient{}
	if len(students) == 0 {
		return out, nil
	}
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok projection ref.students under the TenantScope (RLS) plus the explicit tenant_id; deleted_at is read as state
	rows, err := t.QueryContext(ctx, `SELECT student_id, tenant_id, school_id, status, deleted_at IS NOT NULL FROM ref.students
		WHERE tenant_id = $1 AND student_id = ANY($2::uuid[])`, tenant, uuidList(students))
	if err != nil {
		return nil, fmt.Errorf("ref students: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var r notification.Recipient
		if err := rows.Scan(&r.ID, &r.TenantID, &r.SchoolID, &r.Status, &r.Deleted); err != nil {
			return nil, fmt.Errorf("ref students: %w", err)
		}
		out[r.ID] = r
	}
	return out, rows.Err()
}

// Nickname implements write.NotificationStore.
func (Notifications) Nickname(ctx context.Context, tenant, student uuid.UUID) (string, error) {
	t, err := tx(ctx)
	if err != nil {
		return "", err
	}
	var nick sql.NullString
	// tenancyguard:ok projection ref.player_profiles under the TenantScope (RLS) plus the explicit tenant_id; ref.* has no deleted_at
	err = t.QueryRowContext(ctx, `SELECT nickname FROM ref.player_profiles WHERE tenant_id = $1 AND student_id = $2`, tenant, student).Scan(&nick)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("ref player profiles: %w", err)
	}
	return nick.String, nil
}

// group is the part of a draft shared by every recipient of one source event.
type group struct {
	tenant, source uuid.UUID
	kind, code     string
	params, action string
	refType        string
	refID          uuid.UUID
	at             time.Time
}

func groupOf(d notification.Draft) (group, error) {
	params, err := json.Marshal(d.Params)
	if err != nil {
		return group{}, err
	}
	if d.Params == nil {
		params = []byte("{}")
	}
	g := group{tenant: d.TenantID, source: d.SourceEventID, kind: string(d.Kind), code: d.MessageCode, params: string(params),
		refType: d.RefType, at: d.CreatedAt.UTC()}
	if d.RefID != nil {
		g.refID = *d.RefID
	}
	if d.Action != nil {
		a, err := json.Marshal(d.Action)
		if err != nil {
			return group{}, err
		}
		g.action = string(a)
	}
	return g, nil
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}

// Insert implements write.NotificationStore: one INSERT … SELECT unnest per group of drafts sharing the source event
// (≤ 500 recipients per page of learning), ON CONFLICT DO NOTHING on (recipient_id, source_event_id, kind, created_at).
func (Notifications) Insert(ctx context.Context, drafts []notification.Draft) ([]notification.Notification, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	type batch struct {
		g                group
		ids, recipients  []string
		schools          []string
		draftOfRecipient map[uuid.UUID]notification.Draft
	}
	var order []group
	batches := map[group]*batch{}
	for _, d := range drafts {
		g, err := groupOf(d)
		if err != nil {
			return nil, err
		}
		b, ok := batches[g]
		if !ok {
			b = &batch{g: g, draftOfRecipient: map[uuid.UUID]notification.Draft{}}
			batches[g] = b
			order = append(order, g)
		}
		school := "NULL"
		if d.SchoolID != nil {
			school = d.SchoolID.String()
		}
		b.ids = append(b.ids, ids.New().String())
		b.recipients = append(b.recipients, d.RecipientID.String())
		b.schools = append(b.schools, school)
		b.draftOfRecipient[d.RecipientID] = d
	}
	var out []notification.Notification
	for _, g := range order {
		b := batches[g]
		got, err := insertGroup(ctx, t, g, b.ids, b.recipients, b.schools, b.draftOfRecipient)
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
	}
	return out, nil
}

// insertGroup inserts the drafts of one group in one statement and returns the rows created now (a conflicting row —
// the event was already handled — is not returned).
func insertGroup(ctx context.Context, t *sql.Tx, g group, ids, recipients, schools []string, drafts map[uuid.UUID]notification.Draft) ([]notification.Notification, error) {
	// tenancyguard:ok INSERT into the session tenant (RLS WITH CHECK) with the explicit tenant_id of the event
	rows, err := t.QueryContext(ctx, `INSERT INTO social.notifications (notification_id, tenant_id, recipient_id, school_id, kind,
		message_code, params, ref_type, ref_id, action, source_event_id, created_at)
		SELECT u.id, $1, u.rid, u.sid, $2, $3, $4::jsonb, $5, $6, $7::jsonb, $8, $9
		FROM unnest($10::uuid[], $11::uuid[], $12::uuid[]) AS u(id, rid, sid)
		ON CONFLICT DO NOTHING
		RETURNING notification_id, recipient_id`,
		g.tenant, g.kind, g.code, g.params, nullable(g.refType), nullableID(g.refID), nullable(g.action), g.source, g.at,
		"{"+strings.Join(ids, ",")+"}", "{"+strings.Join(recipients, ",")+"}", "{"+strings.Join(schools, ",")+"}")
	if err != nil {
		return nil, fmt.Errorf("insert notifications: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []notification.Notification
	for rows.Next() {
		var id, rid uuid.UUID
		if err := rows.Scan(&id, &rid); err != nil {
			return nil, fmt.Errorf("insert notifications: %w", err)
		}
		d := drafts[rid]
		out = append(out, notification.Notification{ID: id, TenantID: d.TenantID, RecipientID: rid, Kind: d.Kind,
			MessageCode: d.MessageCode, Params: d.Params, RefType: d.RefType, RefID: d.RefID, Action: d.Action, CreatedAt: d.CreatedAt.UTC()})
	}
	return out, rows.Err()
}

func scanInboxRow(row interface{ Scan(...any) error }) (notification.Notification, error) {
	var n notification.Notification
	var kind, params string
	var refType, action sql.NullString
	var refID uuid.NullUUID
	var readAt sql.NullTime
	if err := row.Scan(&n.ID, &n.TenantID, &n.RecipientID, &kind, &n.MessageCode, &params, &refType, &refID, &action, &n.CreatedAt,
		&readAt); err != nil {
		return n, err
	}
	n.Kind, n.RefType, n.CreatedAt = notification.Kind(kind), refType.String, n.CreatedAt.UTC()
	if err := json.Unmarshal([]byte(params), &n.Params); err != nil {
		return n, fmt.Errorf("notification params: %w", err)
	}
	if refID.Valid {
		id := refID.UUID
		n.RefID = &id
	}
	if action.Valid && action.String != "" {
		var a notification.Action
		if err := json.Unmarshal([]byte(action.String), &a); err != nil {
			return n, fmt.Errorf("notification action: %w", err)
		}
		n.Action = &a
	}
	if readAt.Valid {
		t := readAt.Time.UTC()
		n.ReadAt = &t
	}
	return n, nil
}

func scanInbox(rows *sql.Rows) ([]notification.Notification, error) {
	defer func() { _ = rows.Close() }()
	out := []notification.Notification{}
	for rows.Next() {
		n, err := scanInboxRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// MarkRead implements write.NotificationStore: read_at is set once (read_at IS NULL), only on rows of recipient.
func (Notifications) MarkRead(ctx context.Context, tenant, recipient uuid.UUID, list []uuid.UUID, at time.Time) ([]notification.Notification, int, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, 0, err
	}
	// tenancyguard:ok tenant_id, recipient_id and deleted_at IS NULL in the statement (RLS: session tenant)
	res, err := t.ExecContext(ctx, `UPDATE social.notifications SET read_at = $4
		WHERE tenant_id = $1 AND recipient_id = $2 AND notification_id = ANY($3::uuid[]) AND read_at IS NULL AND deleted_at IS NULL`,
		tenant, recipient, uuidList(list), at.UTC())
	if err != nil {
		return nil, 0, fmt.Errorf("mark notifications read: %w", err)
	}
	changed, _ := res.RowsAffected()
	// tenancyguard:ok tenant_id, recipient_id and deleted_at IS NULL in the statement (RLS: session tenant)
	rows, err := t.QueryContext(ctx, `SELECT `+notificationColumns+` FROM social.notifications
		WHERE tenant_id = $1 AND recipient_id = $2 AND notification_id = ANY($3::uuid[]) AND deleted_at IS NULL`,
		tenant, recipient, uuidList(list))
	if err != nil {
		return nil, 0, fmt.Errorf("read notifications: %w", err)
	}
	out, err := scanInbox(rows)
	return out, int(changed), err
}

// MarkAllRead implements write.NotificationStore.
func (Notifications) MarkAllRead(ctx context.Context, tenant, recipient uuid.UUID, before, at time.Time) (int, error) {
	n, err := exec(ctx, `UPDATE social.notifications SET read_at = $4
		WHERE tenant_id = $1 AND recipient_id = $2 AND read_at IS NULL AND deleted_at IS NULL AND created_at <= $3`,
		tenant, recipient, before.UTC(), at.UTC())
	if err != nil {
		return 0, fmt.Errorf("mark all notifications read: %w", err)
	}
	return int(n), nil
}

// UnreadCount implements write.NotificationStore.
func (Notifications) UnreadCount(ctx context.Context, tenant, recipient uuid.UUID) (int, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	return unreadCount(ctx, t, tenant, recipient)
}

func unreadCount(ctx context.Context, q interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}, tenant, recipient uuid.UUID) (int, error) {
	var n int
	// tenancyguard:ok tenant_id, recipient_id and deleted_at IS NULL in the statement (RLS: session tenant)
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM social.notifications
		WHERE tenant_id = $1 AND recipient_id = $2 AND read_at IS NULL AND deleted_at IS NULL`, tenant, recipient).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return n, nil
}

// Inbox implements read.NotificationReader (newest first, keyset on (created_at, notification_id)).
func (NotificationReader) Inbox(ctx context.Context, tenant, recipient uuid.UUID, unread bool, after *read.After, limit int) ([]notification.Notification, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	var at, id any
	if after != nil {
		at, id = after.At.UTC(), after.ID
	}
	// tenancyguard:ok tenant_id, recipient_id and deleted_at IS NULL in the statement (RLS: session tenant)
	rows, err := t.QueryContext(ctx, `SELECT `+notificationColumns+` FROM social.notifications
		WHERE tenant_id = $1 AND recipient_id = $2 AND deleted_at IS NULL AND (NOT $3::boolean OR read_at IS NULL)
			AND ($4::timestamptz IS NULL OR (created_at, notification_id) < ($4::timestamptz, $5::uuid))
		ORDER BY created_at DESC, notification_id DESC LIMIT $6`, tenant, recipient, unread, at, id, limit)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	return scanInbox(rows)
}

// UnreadCount implements read.NotificationReader.
func (NotificationReader) UnreadCount(ctx context.Context, tenant, recipient uuid.UUID) (int, error) {
	t, err := readTx(ctx)
	if err != nil {
		return 0, err
	}
	return unreadCount(ctx, t, tenant, recipient)
}

// ClubManagers implements write.NotificationStore: the active owner and officers of club (delta F120/F123,
// club_join_request); the RecipientPolicy then keeps the students only (a staff owner is not in ref.students).
func (Notifications) ClubManagers(ctx context.Context, tenant, club uuid.UUID) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok social.club_members with tenant_id + deleted_at IS NULL (active rows only)
	rows, err := t.QueryContext(ctx, `SELECT user_id FROM social.club_members WHERE tenant_id = $1 AND club_id = $2
		AND role IN ('owner', 'officer') AND left_at IS NULL AND deleted_at IS NULL ORDER BY user_id LIMIT 60`, tenant, club)
	if err != nil {
		return nil, fmt.Errorf("club managers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("club managers: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
