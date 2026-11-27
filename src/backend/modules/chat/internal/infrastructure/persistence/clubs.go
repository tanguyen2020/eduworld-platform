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
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/club"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// Clubs implements write.ClubStore and read.ClubReader on social.clubs and its side tables (F120 US4): explicit SQL with
// tenant_id and deleted_at on top of RLS (R27); nothing is deleted (R21).
type Clubs struct{}

var _ write.ClubStore = Clubs{}

const clubColumns = `cl.club_id, cl.tenant_id, cl.school_id, cl.club_type, cl.name, cl.description, cl.icon_id, cl.subject_id, cl.owner_id,
	cl.created_by_kind, cl.status, cl.max_members, cl.member_count, cl.chat_channel_id, cl.version, cl.created_at`

func scanClub(row interface{ Scan(...any) error }, extra ...any) (club.Club, error) {
	var c club.Club
	var school, subject, ch uuid.NullUUID
	var desc, icon sql.NullString
	var typ string
	dest := append([]any{&c.ID, &c.TenantID, &school, &typ, &c.Name, &desc, &icon, &subject, &c.OwnerID, &c.CreatedByKind, &c.Status,
		&c.MaxMembers, &c.MemberCount, &ch, &c.Version, &c.CreatedAt}, extra...)
	if err := row.Scan(dest...); err != nil {
		return c, err
	}
	c.Type, c.SchoolID, c.SubjectID, c.ChannelID = club.Type(typ), nullUUID(school), nullUUID(subject), nullUUID(ch)
	c.Description, c.IconID, c.CreatedAt = nullString(desc), nullString(icon), c.CreatedAt.UTC()
	return c, nil
}

// SchoolCreation implements write.ClubStore.
func (Clubs) SchoolCreation(ctx context.Context, tenant, school uuid.UUID) (club.Creation, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return club.CreationDisabled, err
	}
	var raw []byte
	// tenancyguard:ok projection ref.schools of the session tenant (no deleted_at: rule 2)
	err = t.QueryRowContext(ctx, `SELECT sc.settings_subset->'student_club_creation' FROM ref.schools sc
		WHERE sc.tenant_id = $1 AND sc.school_id = $2 AND sc.status <> 'disabled'`, tenant, school).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) || len(raw) == 0 {
		return club.CreationDisabled, nil
	}
	if err != nil {
		return club.CreationDisabled, fmt.Errorf("chat: school club policy: %w", err)
	}
	var v any
	_ = json.Unmarshal(raw, &v)
	return club.ParseCreation(v), nil
}

// CreateClub implements write.ClubStore.
func (Clubs) CreateClub(ctx context.Context, c club.Club) error {
	_, err := exec(ctx, `INSERT INTO social.clubs (club_id, tenant_id, school_id, club_type, name, description, icon_id, subject_id, owner_id,
		created_by_kind, status, max_members, member_count, chat_channel_id, created_at, created_by, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $9, $15, $9)`,
		c.ID, c.TenantID, c.SchoolID, string(c.Type), c.Name, c.Description, c.IconID, c.SubjectID, c.OwnerID, c.CreatedByKind, c.Status,
		c.MaxMembers, c.MemberCount, c.ChannelID, c.CreatedAt)
	if err != nil {
		return fmt.Errorf("chat: create club: %w", err)
	}
	return nil
}

// Club implements write.ClubStore.
func (Clubs) Club(ctx context.Context, tenant, id uuid.UUID, lock bool) (*club.Club, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	q := `SELECT ` + clubColumns + ` FROM social.clubs cl WHERE cl.tenant_id = $1 AND cl.club_id = $2 AND cl.deleted_at IS NULL`
	if lock {
		q += ` FOR UPDATE`
	}
	// tenancyguard:ok social.clubs of the session tenant, live rows only
	c, err := scanClub(t.QueryRowContext(ctx, q, tenant, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: club: %w", err)
	}
	return &c, nil
}

// SaveClub implements write.ClubStore.
func (Clubs) SaveClub(ctx context.Context, c club.Club, actor uuid.UUID, now time.Time) error {
	// tenancyguard:ok social.clubs of the session tenant, live rows only
	_, err := exec(ctx, `UPDATE social.clubs SET status = $3::varchar, name = $4, description = $5, icon_id = $6, member_count = $7,
		chat_channel_id = $8, updated_at = $9, updated_by = $10, version = version + 1,
		approved_by = CASE WHEN status = 'pending_approval' AND $3::varchar = 'active' THEN $10 ELSE approved_by END,
		approved_at = CASE WHEN status = 'pending_approval' AND $3::varchar = 'active' THEN $9 ELSE approved_at END,
		archived_at = CASE WHEN $3::varchar = 'archived' AND archived_at IS NULL THEN $9 ELSE archived_at END
		WHERE tenant_id = $1 AND club_id = $2 AND deleted_at IS NULL`,
		c.TenantID, c.ID, c.Status, c.Name, c.Description, c.IconID, c.MemberCount, c.ChannelID, now, actor)
	if err != nil {
		return fmt.Errorf("chat: save club: %w", err)
	}
	return nil
}

// Member implements write.ClubStore.
func (Clubs) Member(ctx context.Context, tenant, clubID, user uuid.UUID) (*write.ClubMember, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	m := write.ClubMember{ClubID: clubID, UserID: user}
	var school uuid.NullUUID
	// tenancyguard:ok social.club_members of the session tenant, live active rows only
	err = t.QueryRowContext(ctx, `SELECT cm.school_id, cm.role, cm.joined_at FROM social.club_members cm WHERE cm.tenant_id = $1
		AND cm.club_id = $2 AND cm.user_id = $3 AND cm.left_at IS NULL AND cm.deleted_at IS NULL`, tenant, clubID, user).
		Scan(&school, &m.Role, &m.JoinedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: club member: %w", err)
	}
	m.SchoolID, m.JoinedAt = nullUUID(school), m.JoinedAt.UTC()
	return &m, nil
}

// AddMember implements write.ClubStore.
func (Clubs) AddMember(ctx context.Context, tenant uuid.UUID, m write.ClubMember, source uuid.UUID) error {
	_, err := exec(ctx, `INSERT INTO social.club_members (club_member_id, tenant_id, club_id, user_id, school_id, role, joined_at,
		source_event_id, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $4)`,
		ids.New(), tenant, m.ClubID, m.UserID, m.SchoolID, m.Role, m.JoinedAt, source)
	if err != nil {
		return fmt.Errorf("chat: add club member: %w", err)
	}
	return nil
}

// EndMember implements write.ClubStore.
func (Clubs) EndMember(ctx context.Context, tenant, clubID, user uuid.UUID, at time.Time, reason string) (bool, error) {
	// tenancyguard:ok social.club_members of the session tenant, live active rows only
	n, err := exec(ctx, `UPDATE social.club_members SET left_at = $4, left_reason = $5, updated_at = $4
		WHERE tenant_id = $1 AND club_id = $2 AND user_id = $3 AND left_at IS NULL AND deleted_at IS NULL`, tenant, clubID, user, at, reason)
	if err != nil {
		return false, fmt.Errorf("chat: end club member: %w", err)
	}
	return n == 1, nil
}

// EndAll implements write.ClubStore.
func (Clubs) EndAll(ctx context.Context, tenant, clubID uuid.UUID, at time.Time, reason string) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok social.club_members of the session tenant, live active rows only
	rows, err := t.QueryContext(ctx, `UPDATE social.club_members SET left_at = $3, left_reason = $4, updated_at = $3
		WHERE tenant_id = $1 AND club_id = $2 AND left_at IS NULL AND deleted_at IS NULL RETURNING user_id`, tenant, clubID, at, reason)
	if err != nil {
		return nil, fmt.Errorf("chat: end club members: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []uuid.UUID
	for rows.Next() {
		var u uuid.UUID
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// SetRole implements write.ClubStore.
func (Clubs) SetRole(ctx context.Context, tenant, clubID, user uuid.UUID, role string) (bool, error) {
	// tenancyguard:ok social.club_members of the session tenant, live active rows only
	n, err := exec(ctx, `UPDATE social.club_members SET role = $4, updated_at = now() WHERE tenant_id = $1 AND club_id = $2
		AND user_id = $3 AND left_at IS NULL AND deleted_at IS NULL`, tenant, clubID, user, role)
	if err != nil {
		return false, fmt.Errorf("chat: club role: %w", err)
	}
	return n == 1, nil
}

// CountClubs implements write.ClubStore.
func (Clubs) CountClubs(ctx context.Context, tenant, user uuid.UUID) (int, error) {
	// tenancyguard:ok social.club_members joined with social.clubs of the session tenant, live rows only
	return count(ctx, `SELECT count(*) FROM social.club_members cm JOIN social.clubs cl ON cl.club_id = cm.club_id
		AND cl.tenant_id = cm.tenant_id AND cl.deleted_at IS NULL AND cl.status <> 'archived'
		WHERE cm.tenant_id = $1 AND cm.user_id = $2 AND cm.left_at IS NULL AND cm.deleted_at IS NULL`, tenant, user)
}

// CreateRequest implements write.ClubStore.
func (Clubs) CreateRequest(ctx context.Context, r write.ClubRequest) (bool, error) {
	n, err := exec(ctx, `INSERT INTO social.club_join_requests (request_id, tenant_id, club_id, requester_id, status, created_at, created_by,
		updated_at) VALUES ($1, $2, $3, $4, 'pending', $5, $4, $5) ON CONFLICT DO NOTHING`, r.ID, r.TenantID, r.ClubID, r.RequesterID, r.CreatedAt)
	if err != nil {
		return false, fmt.Errorf("chat: club request: %w", err)
	}
	return n == 1, nil
}

func scanClubRequest(row interface{ Scan(...any) error }) (*write.ClubRequest, error) {
	var r write.ClubRequest
	if err := row.Scan(&r.ID, &r.TenantID, &r.ClubID, &r.RequesterID, &r.Status, &r.CreatedAt); err != nil {
		return nil, err
	}
	r.CreatedAt = r.CreatedAt.UTC()
	return &r, nil
}

// Request implements write.ClubStore.
func (Clubs) Request(ctx context.Context, tenant, id uuid.UUID) (*write.ClubRequest, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok social.club_join_requests of the session tenant, live rows only
	r, err := scanClubRequest(t.QueryRowContext(ctx, `SELECT r.request_id, r.tenant_id, r.club_id, r.requester_id, r.status, r.created_at
		FROM social.club_join_requests r WHERE r.tenant_id = $1 AND r.request_id = $2 AND r.deleted_at IS NULL FOR UPDATE`, tenant, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: club request: %w", err)
	}
	return r, nil
}

// PendingRequest implements write.ClubStore.
func (Clubs) PendingRequest(ctx context.Context, tenant, clubID, requester uuid.UUID) (*write.ClubRequest, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok social.club_join_requests of the session tenant, live pending rows only
	r, err := scanClubRequest(t.QueryRowContext(ctx, `SELECT r.request_id, r.tenant_id, r.club_id, r.requester_id, r.status, r.created_at
		FROM social.club_join_requests r WHERE r.tenant_id = $1 AND r.club_id = $2 AND r.requester_id = $3 AND r.status = 'pending'
		AND r.deleted_at IS NULL`, tenant, clubID, requester))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: club request: %w", err)
	}
	return r, nil
}

// DecideRequest implements write.ClubStore.
func (Clubs) DecideRequest(ctx context.Context, tenant, id uuid.UUID, status string, by uuid.UUID, at time.Time) (bool, error) {
	// tenancyguard:ok social.club_join_requests of the session tenant, live pending rows only
	n, err := exec(ctx, `UPDATE social.club_join_requests SET status = $3, decided_by = $4, decided_at = $5, updated_at = $5, updated_by = $4
		WHERE tenant_id = $1 AND request_id = $2 AND status = 'pending' AND deleted_at IS NULL`, tenant, id, status, by, at)
	if err != nil {
		return false, fmt.Errorf("chat: decide club request: %w", err)
	}
	return n == 1, nil
}

func scanInvitation(row interface{ Scan(...any) error }) (*write.ClubInvitation, error) {
	var v write.ClubInvitation
	if err := row.Scan(&v.ID, &v.TenantID, &v.ClubID, &v.InviterID, &v.InviteeID, &v.Status, &v.ExpiresAt); err != nil {
		return nil, err
	}
	v.ExpiresAt = v.ExpiresAt.UTC()
	return &v, nil
}

const invitationColumns = `i.invitation_id, i.tenant_id, i.club_id, i.inviter_id, i.invitee_id, i.status, i.expires_at`

// CreateInvitation implements write.ClubStore.
func (Clubs) CreateInvitation(ctx context.Context, inv write.ClubInvitation) (*write.ClubInvitation, bool, error) {
	n, err := exec(ctx, `INSERT INTO social.club_invitations (invitation_id, tenant_id, club_id, inviter_id, invitee_id, status, expires_at,
		created_by) VALUES ($1, $2, $3, $4, $5, 'pending', $6, $4) ON CONFLICT DO NOTHING`, inv.ID, inv.TenantID, inv.ClubID, inv.InviterID,
		inv.InviteeID, inv.ExpiresAt)
	if err != nil {
		return nil, false, fmt.Errorf("chat: club invitation: %w", err)
	}
	if n == 1 {
		inv.Status = "pending"
		return &inv, true, nil
	}
	t, err := tx(ctx)
	if err != nil {
		return nil, false, err
	}
	// tenancyguard:ok social.club_invitations of the session tenant, live pending rows only
	existing, err := scanInvitation(t.QueryRowContext(ctx, `SELECT `+invitationColumns+` FROM social.club_invitations i
		WHERE i.tenant_id = $1 AND i.club_id = $2 AND i.invitee_id = $3 AND i.status = 'pending' AND i.deleted_at IS NULL`,
		inv.TenantID, inv.ClubID, inv.InviteeID))
	if err != nil {
		return nil, false, fmt.Errorf("chat: club invitation: %w", err)
	}
	return existing, false, nil
}

// Invitation implements write.ClubStore.
func (Clubs) Invitation(ctx context.Context, tenant, id uuid.UUID) (*write.ClubInvitation, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok social.club_invitations of the session tenant, live rows only
	v, err := scanInvitation(t.QueryRowContext(ctx, `SELECT `+invitationColumns+` FROM social.club_invitations i
		WHERE i.tenant_id = $1 AND i.invitation_id = $2 AND i.deleted_at IS NULL FOR UPDATE`, tenant, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: club invitation: %w", err)
	}
	return v, nil
}

// DecideInvitation implements write.ClubStore.
func (Clubs) DecideInvitation(ctx context.Context, tenant, id uuid.UUID, status string, at time.Time) (bool, error) {
	// tenancyguard:ok social.club_invitations of the session tenant, live pending rows only
	n, err := exec(ctx, `UPDATE social.club_invitations SET status = $3, decided_at = $4, updated_at = $4
		WHERE tenant_id = $1 AND invitation_id = $2 AND status = 'pending' AND deleted_at IS NULL`, tenant, id, status, at)
	if err != nil {
		return false, fmt.Errorf("chat: decide club invitation: %w", err)
	}
	return n == 1, nil
}

// CreateGoal implements write.ClubStore.
func (Clubs) CreateGoal(ctx context.Context, g write.ClubGoal, actor uuid.UUID) error {
	_, err := exec(ctx, `INSERT INTO social.club_goals (goal_id, tenant_id, club_id, title, metric, subject_id, target, progress, week_start,
		status, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7, 0, $8::date, 'active', $9)`, g.ID, g.TenantID, g.ClubID, g.Title, g.Metric,
		g.SubjectID, g.Target, g.WeekStart.Format("2006-01-02"), actor)
	if err != nil {
		return fmt.Errorf("chat: club goal: %w", err)
	}
	return nil
}

// CreateEvent implements write.ClubStore.
func (Clubs) CreateEvent(ctx context.Context, e write.ClubEvent, actor uuid.UUID) error {
	_, err := exec(ctx, `INSERT INTO social.club_events (event_id, tenant_id, club_id, title, description, event_kind, tournament_template_id,
		starts_at, ends_at, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`, e.ID, e.TenantID, e.ClubID, e.Title, e.Description,
		e.Kind, e.TournamentTemplateID, e.StartsAt, e.EndsAt, actor)
	if err != nil {
		return fmt.Errorf("chat: club event: %w", err)
	}
	return nil
}

const goalColumns = `g.goal_id, g.tenant_id, g.club_id, g.title, g.metric, g.subject_id, g.target, g.progress, g.week_start, g.status`

func scanGoal(row interface{ Scan(...any) error }) (write.ClubGoal, error) {
	var g write.ClubGoal
	var title sql.NullString
	var subject uuid.NullUUID
	if err := row.Scan(&g.ID, &g.TenantID, &g.ClubID, &title, &g.Metric, &subject, &g.Target, &g.Progress, &g.WeekStart, &g.Status); err != nil {
		return g, err
	}
	g.Title, g.SubjectID = nullString(title), nullUUID(subject)
	return g, nil
}

// GoalsOf implements write.ClubStore.
func (Clubs) GoalsOf(ctx context.Context, tenant, user uuid.UUID, metric string, week time.Time) ([]write.ClubGoal, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok social.club_goals/clubs/club_members of the session tenant, live rows only
	rows, err := t.QueryContext(ctx, `SELECT `+goalColumns+` FROM social.club_goals g
		JOIN social.clubs cl ON cl.club_id = g.club_id AND cl.tenant_id = g.tenant_id AND cl.deleted_at IS NULL AND cl.status = 'active'
		JOIN social.club_members cm ON cm.club_id = g.club_id AND cm.tenant_id = g.tenant_id AND cm.user_id = $2
			AND cm.left_at IS NULL AND cm.deleted_at IS NULL
		WHERE g.tenant_id = $1 AND g.metric = $3 AND g.week_start = $4::date AND g.status = 'active' AND g.deleted_at IS NULL
		ORDER BY g.goal_id FOR UPDATE OF g`, tenant, user, metric, week.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("chat: club goals: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.ClubGoal
	for rows.Next() {
		g, err := scanGoal(rows)
		if err != nil {
			return nil, fmt.Errorf("chat: club goals: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// Contribute implements write.ClubStore: one event counts once per goal (last_event_id + processed_events of the consumer).
func (Clubs) Contribute(ctx context.Context, tenant, goal, user uuid.UUID, delta int, event uuid.UUID) (bool, error) {
	// tenancyguard:ok social.club_goal_progress of the session tenant (counter table, no deleted_at)
	n, err := exec(ctx, `INSERT INTO social.club_goal_progress AS p (tenant_id, goal_id, user_id, contribution, last_event_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (goal_id, user_id) DO UPDATE SET contribution = p.contribution + EXCLUDED.contribution,
			last_event_id = EXCLUDED.last_event_id, updated_at = now()
		WHERE p.last_event_id IS DISTINCT FROM EXCLUDED.last_event_id AND p.tenant_id = EXCLUDED.tenant_id`, tenant, goal, user, delta, event)
	if err != nil {
		return false, fmt.Errorf("chat: club contribution: %w", err)
	}
	return n == 1, nil
}

// AdvanceGoal implements write.ClubStore.
func (Clubs) AdvanceGoal(ctx context.Context, tenant, goal uuid.UUID, delta int, at time.Time) (write.ClubGoal, error) {
	t, err := tx(ctx)
	if err != nil {
		return write.ClubGoal{}, err
	}
	// tenancyguard:ok social.club_goals of the session tenant, live active rows only
	g, err := scanGoal(t.QueryRowContext(ctx, `UPDATE social.club_goals g SET progress = least(g.progress + $3::int, 1000000000),
		status = CASE WHEN g.progress + $3::int >= g.target THEN 'completed' ELSE g.status END,
		completed_at = CASE WHEN g.progress + $3 >= g.target THEN $4 ELSE g.completed_at END, updated_at = $4
		WHERE g.tenant_id = $1 AND g.goal_id = $2 AND g.status = 'active' AND g.deleted_at IS NULL RETURNING `+goalColumns,
		tenant, goal, delta, at))
	if err != nil {
		return write.ClubGoal{}, fmt.Errorf("chat: advance club goal: %w", err)
	}
	return g, nil
}

// ---------- read side ----------

// ClubReads implements read.ClubReader.
type ClubReads struct{}

var _ read.ClubReader = ClubReads{}

// Clubs implements read.ClubReader.
func (ClubReads) Clubs(ctx context.Context, f read.ClubFilter) ([]read.ClubRow, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	var afterAt *time.Time
	var afterID *uuid.UUID
	if f.After != nil {
		afterAt, afterID = &f.After.At, &f.After.ID
	}
	var typ *string
	if f.Type != nil {
		s := string(*f.Type)
		typ = &s
	}
	var schools any
	if f.Schools != nil {
		schools = idStrings(f.Schools)
	}
	var statuses any
	if len(f.Status) > 0 {
		statuses = f.Status
	}
	// tenancyguard:ok social.clubs (+ club_members of the viewer) of the session tenant, live rows only
	rows, err := t.QueryContext(ctx, `SELECT `+clubColumns+`, coalesce(me.role, '') FROM social.clubs cl
		LEFT JOIN social.club_members me ON me.club_id = cl.club_id AND me.tenant_id = cl.tenant_id AND me.user_id = $2
			AND me.left_at IS NULL AND me.deleted_at IS NULL
		WHERE cl.tenant_id = $1 AND cl.deleted_at IS NULL
			AND ($3::uuid IS NULL OR EXISTS (SELECT 1 FROM social.club_members m WHERE m.club_id = cl.club_id AND m.tenant_id = cl.tenant_id
				AND m.user_id = $3 AND m.left_at IS NULL AND m.deleted_at IS NULL))
			AND ($4::uuid[] IS NULL OR cl.school_id = ANY($4::uuid[]))
			AND ($5::text[] IS NULL OR cl.status = ANY($5::text[]))
			AND ($6::text IS NULL OR cl.club_type = $6)
			AND ($7::timestamptz IS NULL OR (cl.created_at, cl.club_id) < ($7, $8::uuid))
		ORDER BY cl.created_at DESC, cl.club_id DESC LIMIT $9`, f.Tenant, f.Viewer, f.Member, schools, statuses, typ, afterAt, afterID, f.Limit)
	if err != nil {
		return nil, fmt.Errorf("chat: clubs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.ClubRow
	for rows.Next() {
		var r read.ClubRow
		c, err := scanClub(rows, &r.MyRole)
		if err != nil {
			return nil, fmt.Errorf("chat: clubs: %w", err)
		}
		r.Club = c
		out = append(out, r)
	}
	return out, rows.Err()
}

// Club implements read.ClubReader.
func (ClubReads) Club(ctx context.Context, tenant, id uuid.UUID, viewer *uuid.UUID) (*read.ClubRow, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	var r read.ClubRow
	// tenancyguard:ok social.clubs (+ club_members of the viewer) of the session tenant, live rows only
	c, err := scanClub(t.QueryRowContext(ctx, `SELECT `+clubColumns+`, coalesce(me.role, '') FROM social.clubs cl
		LEFT JOIN social.club_members me ON me.club_id = cl.club_id AND me.tenant_id = cl.tenant_id AND me.user_id = $3
			AND me.left_at IS NULL AND me.deleted_at IS NULL
		WHERE cl.tenant_id = $1 AND cl.club_id = $2 AND cl.deleted_at IS NULL`, tenant, id, viewer), &r.MyRole)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: club: %w", err)
	}
	r.Club = c
	return &r, nil
}

func keyset(after *read.ClubCursor) (*time.Time, *uuid.UUID) {
	if after == nil {
		return nil, nil
	}
	return &after.At, &after.ID
}

// Members implements read.ClubReader (public card only, R16).
func (ClubReads) Members(ctx context.Context, tenant, clubID uuid.UUID, after *read.ClubCursor, limit int) ([]read.ClubMemberRow, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	at, id := keyset(after)
	// tenancyguard:ok social.club_members + ref.player_profiles of the session tenant, live active rows only
	rows, err := t.QueryContext(ctx, `SELECT cm.user_id, p.nickname, p.avatar_thumb_key, cm.role, cm.joined_at FROM social.club_members cm
		LEFT JOIN ref.player_profiles p ON p.tenant_id = cm.tenant_id AND p.student_id = cm.user_id
		WHERE cm.tenant_id = $1 AND cm.club_id = $2 AND cm.left_at IS NULL AND cm.deleted_at IS NULL
			AND ($3::timestamptz IS NULL OR (cm.joined_at, cm.user_id) > ($3, $4::uuid))
		ORDER BY cm.joined_at, cm.user_id LIMIT $5`, tenant, clubID, at, id, limit)
	if err != nil {
		return nil, fmt.Errorf("chat: club members: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.ClubMemberRow
	for rows.Next() {
		var m read.ClubMemberRow
		var nick, thumb sql.NullString
		if err := rows.Scan(&m.UserID, &nick, &thumb, &m.Role, &m.JoinedAt); err != nil {
			return nil, fmt.Errorf("chat: club members: %w", err)
		}
		m.Nickname, m.Avatar, m.JoinedAt = nullString(nick), nullString(thumb), m.JoinedAt.UTC()
		out = append(out, m)
	}
	return out, rows.Err()
}

// Requests implements read.ClubReader.
func (ClubReads) Requests(ctx context.Context, tenant, clubID uuid.UUID, after *read.ClubCursor, limit int) ([]read.ClubRequestRow, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	at, id := keyset(after)
	// tenancyguard:ok social.club_join_requests + ref.player_profiles of the session tenant, live pending rows only
	rows, err := t.QueryContext(ctx, `SELECT r.request_id, r.club_id, r.requester_id, p.nickname, r.status, r.created_at
		FROM social.club_join_requests r LEFT JOIN ref.player_profiles p ON p.tenant_id = r.tenant_id AND p.student_id = r.requester_id
		WHERE r.tenant_id = $1 AND r.club_id = $2 AND r.status = 'pending' AND r.deleted_at IS NULL
			AND ($3::timestamptz IS NULL OR (r.created_at, r.request_id) > ($3, $4::uuid))
		ORDER BY r.created_at, r.request_id LIMIT $5`, tenant, clubID, at, id, limit)
	if err != nil {
		return nil, fmt.Errorf("chat: club requests: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.ClubRequestRow
	for rows.Next() {
		var r read.ClubRequestRow
		var nick sql.NullString
		if err := rows.Scan(&r.ID, &r.ClubID, &r.RequesterID, &nick, &r.Status, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("chat: club requests: %w", err)
		}
		r.Nickname, r.CreatedAt = nullString(nick), r.CreatedAt.UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

// Goals implements read.ClubReader (at most 20 goals, newest week first).
func (ClubReads) Goals(ctx context.Context, tenant, clubID, viewer uuid.UUID) ([]read.ClubGoalRow, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok social.club_goals + club_goal_progress of the session tenant, live rows only
	rows, err := t.QueryContext(ctx, `SELECT g.goal_id, g.club_id, g.title, g.metric, g.subject_id, g.target, g.progress,
		coalesce(pr.contribution, 0), g.week_start, g.status FROM social.club_goals g
		LEFT JOIN social.club_goal_progress pr ON pr.goal_id = g.goal_id AND pr.tenant_id = g.tenant_id AND pr.user_id = $3
		WHERE g.tenant_id = $1 AND g.club_id = $2 AND g.deleted_at IS NULL ORDER BY g.week_start DESC, g.goal_id LIMIT 20`,
		tenant, clubID, viewer)
	if err != nil {
		return nil, fmt.Errorf("chat: club goals: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.ClubGoalRow
	for rows.Next() {
		var g read.ClubGoalRow
		var title sql.NullString
		var subject uuid.NullUUID
		if err := rows.Scan(&g.ID, &g.ClubID, &title, &g.Metric, &subject, &g.Target, &g.Progress, &g.MyContribution, &g.WeekStart,
			&g.Status); err != nil {
			return nil, fmt.Errorf("chat: club goals: %w", err)
		}
		g.Title, g.SubjectID = nullString(title), nullUUID(subject)
		out = append(out, g)
	}
	return out, rows.Err()
}

// Events implements read.ClubReader.
func (ClubReads) Events(ctx context.Context, tenant, clubID uuid.UUID, after *read.ClubCursor, limit int) ([]read.ClubEventRow, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	at, id := keyset(after)
	// tenancyguard:ok social.club_events of the session tenant, live rows only
	rows, err := t.QueryContext(ctx, `SELECT e.event_id, e.club_id, e.title, e.description, e.event_kind, e.starts_at, e.ends_at, e.created_at
		FROM social.club_events e WHERE e.tenant_id = $1 AND e.club_id = $2 AND e.deleted_at IS NULL
			AND ($3::timestamptz IS NULL OR (e.starts_at, e.event_id) > ($3, $4::uuid))
		ORDER BY e.starts_at, e.event_id LIMIT $5`, tenant, clubID, at, id, limit)
	if err != nil {
		return nil, fmt.Errorf("chat: club events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.ClubEventRow
	for rows.Next() {
		var e read.ClubEventRow
		var desc sql.NullString
		var ends sql.NullTime
		if err := rows.Scan(&e.ID, &e.ClubID, &e.Title, &desc, &e.Kind, &e.StartsAt, &ends, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("chat: club events: %w", err)
		}
		e.Description, e.EndsAt, e.StartsAt = nullString(desc), nullTime(ends), e.StartsAt.UTC()
		out = append(out, e)
	}
	return out, rows.Err()
}

// Invitations implements read.ClubReader (listMyClubInvitations, delta F120): the invitations of invitee only, newest
// first, with the club name and the nickname of the inviter (R16).
func (ClubReads) Invitations(ctx context.Context, tenant, invitee uuid.UUID, status string, now time.Time, after *read.ClubCursor,
	limit int) ([]read.ClubInvitationRow, error) {
	t, err := anyTx(ctx)
	if err != nil {
		return nil, err
	}
	cond := `i.status = $3::varchar`
	switch status {
	case "pending":
		cond = `$3::varchar = 'pending' AND i.status = 'pending' AND i.expires_at > $4 AND c.status = 'active'`
	case "expired":
		cond = `$3::varchar = 'expired' AND (i.status = 'expired' OR (i.status = 'pending' AND i.expires_at <= $4))`
	}
	at, id := keyset(after)
	// tenancyguard:ok social.club_invitations + social.clubs with tenant_id + deleted_at IS NULL; projection ref.player_profiles of the session tenant (no deleted_at)
	rows, err := t.QueryContext(ctx, `SELECT i.invitation_id, i.club_id, c.name, i.inviter_id, p.nickname, i.invitee_id,
			CASE WHEN i.status = 'pending' AND i.expires_at <= $4 THEN 'expired' ELSE i.status END, i.expires_at, i.created_at
		FROM social.club_invitations i
		JOIN social.clubs c ON c.club_id = i.club_id AND c.tenant_id = i.tenant_id AND c.deleted_at IS NULL
		LEFT JOIN ref.player_profiles p ON p.tenant_id = i.tenant_id AND p.student_id = i.inviter_id
		WHERE i.tenant_id = $1 AND i.invitee_id = $2 AND i.deleted_at IS NULL AND `+cond+`
			AND ($5::timestamptz IS NULL OR (i.created_at, i.invitation_id) < ($5, $6::uuid))
		ORDER BY i.created_at DESC, i.invitation_id DESC LIMIT $7`, tenant, invitee, status, now, at, id, limit)
	if err != nil {
		return nil, fmt.Errorf("chat: club invitations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.ClubInvitationRow
	for rows.Next() {
		var r read.ClubInvitationRow
		var name, nick sql.NullString
		if err := rows.Scan(&r.ID, &r.ClubID, &name, &r.InviterID, &nick, &r.InviteeID, &r.Status, &r.ExpiresAt, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("chat: club invitations: %w", err)
		}
		r.ClubName, r.InviterNickname = name.String, nullString(nick)
		r.ExpiresAt, r.CreatedAt = r.ExpiresAt.UTC(), r.CreatedAt.UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}
