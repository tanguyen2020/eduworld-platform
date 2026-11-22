package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

func run(ctx context.Context, query string, args ...any) (int64, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	res, err := t.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func actor(ctx context.Context) any {
	if p, ok := authz.PrincipalFrom(ctx); ok {
		return p.UserID
	}
	return nil
}

func uuidArray(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

func unique(err error, constraints ...string) bool {
	var pe *pgconn.PgError
	if !errors.As(err, &pe) || pe.Code != "23505" {
		return false
	}
	for _, c := range constraints {
		if pe.ConstraintName == c {
			return true
		}
	}
	return false
}

func times(ctx context.Context, query string, args ...any) ([]time.Time, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("party: times: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []time.Time
	for rows.Next() {
		var at time.Time
		if err := rows.Scan(&at); err != nil {
			return nil, err
		}
		out = append(out, at.UTC())
	}
	return out, rows.Err()
}

func exists(ctx context.Context, query string, args ...any) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	var ok bool
	if err := t.QueryRowContext(ctx, query, args...).Scan(&ok); err != nil {
		return false, fmt.Errorf("party: exists: %w", err)
	}
	return ok, nil
}

// ---------- Directory (projections ref.*) ----------

// PartyDirectory implements write.Directory on the ref.* projections and multiplayer.party_policies.
type PartyDirectory struct{}

var _ write.Directory = PartyDirectory{}

// Tenant implements write.Directory.
func (PartyDirectory) Tenant(ctx context.Context, tenant uuid.UUID) (*write.Tenant, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var out write.Tenant
	// tenancyguard:ok tenant registry ref.tenants (key = tenant_id, no deleted_at: rule 2)
	err = t.QueryRowContext(ctx, `SELECT tenant_type, coalesce((settings_subset->>'cross_school_social_enabled')::boolean, false)
		FROM ref.tenants WHERE tenant_id = $1`, tenant).Scan(&out.Type, &out.CrossSchool)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ref tenants: %w", err)
	}
	return &out, nil
}

// Subject implements write.Directory: school, grade level and band of the student (ref.students — ADR-0095, no
// enrollment) and the multiplayer sanctions in force at now.
func (PartyDirectory) Subject(ctx context.Context, tenant, student uuid.UUID, now time.Time) (*party.Subject, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	s := party.Subject{ID: student}
	var grade sql.NullInt32
	var status string
	var deleted sql.NullTime
	// tenancyguard:ok projection read under the TenantScope of the command; deleted_at mirrors the source (cleanup), ref.sanctions has none (rule 2)
	err = t.QueryRowContext(ctx, `SELECT st.school_id, st.grade_level, st.status, st.deleted_at,
		EXISTS (SELECT 1 FROM ref.sanctions x WHERE x.tenant_id = st.tenant_id AND x.user_id = st.student_id AND x.active
			AND (x.until IS NULL OR x.until > $3))
		FROM ref.students st WHERE st.student_id = $1 AND st.tenant_id = $2`, student, tenant, now).
		Scan(&s.SchoolID, &grade, &status, &deleted, &s.Restricted)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ref students: %w", err)
	}
	if grade.Valid {
		s.Grade = int(grade.Int32)
		s.Band = party.BandOf(s.Grade)
	}
	s.Active = status == "active" && !deleted.Valid
	return &s, nil
}

// Friends implements write.Directory (ref.friendships keeps the pair ordered low < high).
func (PartyDirectory) Friends(ctx context.Context, tenant, a, b uuid.UUID) (bool, error) {
	low, high := a, b
	if strings.Compare(low.String(), high.String()) > 0 {
		low, high = high, low
	}
	// tenancyguard:ok projection read under the TenantScope; ref.* has no deleted_at (rule 2)
	return exists(ctx, `SELECT EXISTS (SELECT 1 FROM ref.friendships WHERE tenant_id = $1 AND user_low_id = $2 AND user_high_id = $3
		AND active)`, tenant, low, high)
}

// Blocked implements write.Directory.
func (PartyDirectory) Blocked(ctx context.Context, tenant, student uuid.UUID, others []uuid.UUID) (bool, error) {
	if len(others) == 0 {
		return false, nil
	}
	// tenancyguard:ok projection read under the TenantScope; ref.* has no deleted_at (rule 2)
	return exists(ctx, `SELECT EXISTS (SELECT 1 FROM ref.blocks WHERE tenant_id = $1 AND active
		AND ((blocker_id = $2 AND blocked_id = ANY($3::uuid[])) OR (blocked_id = $2 AND blocker_id = ANY($3::uuid[]))))`,
		tenant, student, uuidArray(others))
}

// Overrides implements write.Directory.
func (PartyDirectory) Overrides(ctx context.Context, tenant, school uuid.UUID) ([]party.Override, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, `SELECT rules FROM multiplayer.party_policies WHERE tenant_id = $1 AND deleted_at IS NULL
		AND scope_type = 'school' AND scope_id = $2`, tenant, school)
	if err != nil {
		return nil, fmt.Errorf("party policies: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []party.Override
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var o party.Override
		if err := json.Unmarshal(raw, &o); err != nil {
			return nil, fmt.Errorf("party policies: rules: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// Cards implements write.Directory and read.PartyReader: public data only (R16).
func (PartyDirectory) Cards(ctx context.Context, tenant uuid.UUID, students []uuid.UUID) (map[uuid.UUID]port.Card, error) {
	out := map[uuid.UUID]port.Card{}
	if len(students) == 0 {
		return out, nil
	}
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok projection read under the TenantScope; ref.* has no deleted_at (rule 2)
	rows, err := t.QueryContext(ctx, `SELECT student_id, nickname, level, avatar_config FROM ref.player_profiles
		WHERE tenant_id = $1 AND student_id = ANY($2::uuid[])`, tenant, uuidArray(students))
	if err != nil {
		return nil, fmt.Errorf("ref player profiles: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var c port.Card
		var nick sql.NullString
		var avatar []byte
		if err := rows.Scan(&c.UserID, &nick, &c.Level, &avatar); err != nil {
			return nil, err
		}
		if nick.Valid {
			n := nick.String
			c.Nickname = &n
		}
		if len(avatar) > 0 {
			c.Avatar = json.RawMessage(avatar)
		}
		out[c.UserID] = c
	}
	return out, rows.Err()
}

// ---------- Parties, members, history ----------

// Parties implements write.PartyRepository and read.PartyReader on multiplayer.* (explicit tenant_id and deleted_at
// conditions, RLS checks the tenant again; never a physical delete — ADR-0028).
type Parties struct{ PartyDirectory }

var (
	_ write.PartyRepository = Parties{}
	_ read.PartyReader      = Parties{}
)

// LockStudents implements write.PartyRepository: transaction advisory locks per student, in a fixed order (taken
// before any party row lock).
func (Parties) LockStudents(ctx context.Context, students ...uuid.UUID) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	keys := uuidArray(students)
	sort.Strings(keys)
	for i, k := range keys {
		if i > 0 && keys[i-1] == k {
			continue
		}
		if _, err := t.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('competition.party:' || $1, 0))`, k); err != nil {
			return fmt.Errorf("party: lock: %w", err)
		}
	}
	return nil
}

// ActivePartyOf implements write.PartyRepository.
func (Parties) ActivePartyOf(ctx context.Context, tenant, student uuid.UUID) (uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	var id uuid.UUID
	err = t.QueryRowContext(ctx, `SELECT party_id FROM multiplayer.party_members WHERE tenant_id = $1 AND student_id = $2
		AND left_at IS NULL AND deleted_at IS NULL LIMIT 1`, tenant, student).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, nil
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("party: active of student: %w", err)
	}
	return id, nil
}

// Create implements write.PartyRepository.
func (r Parties) Create(ctx context.Context, p *party.Party) error {
	if _, err := run(ctx, `INSERT INTO multiplayer.parties (party_id, tenant_id, created_at, created_by, updated_at, school_id, name,
		leader_id, party_type, visibility, invitation_policy, max_members, grade_band, status, state_version)
		VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		p.ID, p.TenantID, p.CreatedAt, actor(ctx), p.SchoolID, p.Name, p.LeaderID, p.Type, p.Visibility, p.InvitationPolicy,
		p.MaxMembers, p.GradeBand, p.Status, p.StateVersion); err != nil {
		return fmt.Errorf("party: create: %w", err)
	}
	return r.AddMember(ctx, p, p.Members[0])
}

const partyColumns = `party_id, tenant_id, school_id, name, leader_id, party_type, visibility, invitation_policy, max_members, grade_band,
	status, state_version, created_at, updated_at, disbanded_at, selected_activity_type, selected_activity_ref, selected_game_mode`

// Load implements write.PartyRepository.
func (Parties) Load(ctx context.Context, tenant, id uuid.UUID, lock bool) (*party.Party, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	q := `SELECT ` + partyColumns + ` FROM multiplayer.parties WHERE tenant_id = $1 AND party_id = $2 AND deleted_at IS NULL`
	if lock {
		q += ` FOR UPDATE`
	}
	var p party.Party
	var disbanded sql.NullTime
	var selType, selRef, selMode sql.NullString
	err = t.QueryRowContext(ctx, q, tenant, id).Scan(&p.ID, &p.TenantID, &p.SchoolID, &p.Name, &p.LeaderID, &p.Type, &p.Visibility,
		&p.InvitationPolicy, &p.MaxMembers, &p.GradeBand, &p.Status, &p.StateVersion, &p.CreatedAt, &p.UpdatedAt, &disbanded,
		&selType, &selRef, &selMode)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("party: load: %w", err)
	}
	p.Legacy()
	p.CreatedAt, p.UpdatedAt = p.CreatedAt.UTC(), p.UpdatedAt.UTC()
	if disbanded.Valid {
		at := disbanded.Time.UTC()
		p.DisbandedAt = &at
	}
	if selType.Valid {
		p.Selected = &party.Selection{Type: selType.String, Ref: selRef.String, GameMode: selMode.String}
	}
	rows, err := t.QueryContext(ctx, `SELECT student_id, school_id, role, joined_at, invitation_id FROM multiplayer.party_members
		WHERE tenant_id = $1 AND party_id = $2 AND left_at IS NULL AND deleted_at IS NULL ORDER BY joined_at, membership_id`, tenant, id)
	if err != nil {
		return nil, fmt.Errorf("party: members: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var m party.Member
		var inv uuid.NullUUID
		if err := rows.Scan(&m.StudentID, &m.SchoolID, &m.Role, &m.JoinedAt, &inv); err != nil {
			return nil, err
		}
		m.JoinedAt = m.JoinedAt.UTC()
		if inv.Valid {
			i := inv.UUID
			m.InvitationID = &i
		}
		p.Members = append(p.Members, m)
	}
	return &p, rows.Err()
}

// Save implements write.PartyRepository.
func (Parties) Save(ctx context.Context, p *party.Party) error {
	var selType, selRef, selMode any
	if p.Selected != nil {
		selType, selRef, selMode = p.Selected.Type, p.Selected.Ref, nullString(p.Selected.GameMode)
	}
	_, err := run(ctx, `UPDATE multiplayer.parties SET name = $3, leader_id = $4, visibility = $5, invitation_policy = $6, max_members = $7,
		status = $8, state_version = $9, disbanded_at = $10, updated_at = $11, updated_by = $12, version = version + 1,
		selected_activity_type = $13, selected_activity_ref = $14, selected_game_mode = $15
		WHERE tenant_id = $1 AND party_id = $2 AND deleted_at IS NULL`, p.TenantID, p.ID, p.Name, p.LeaderID, p.Visibility,
		p.InvitationPolicy, p.MaxMembers, p.Status, p.StateVersion, p.DisbandedAt, p.UpdatedAt, actor(ctx), selType, selRef, selMode)
	if err != nil {
		return fmt.Errorf("party: save: %w", err)
	}
	return nil
}

// AddMember implements write.PartyRepository.
func (Parties) AddMember(ctx context.Context, p *party.Party, m party.Member) error {
	// tenancyguard:ok insert under the TenantScope (RLS WITH CHECK on tenant_id)
	_, err := run(ctx, `INSERT INTO multiplayer.party_members (membership_id, tenant_id, created_at, created_by, updated_at, party_id,
		student_id, school_id, role, invitation_id, joined_at) VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8, $9, $3)`,
		ids.New(), p.TenantID, m.JoinedAt, actor(ctx), p.ID, m.StudentID, m.SchoolID, m.Role, m.InvitationID)
	if unique(err, "party_members_one_active_party_uq", "party_members_active_uq") {
		return write.ErrAlreadyInParty
	}
	if err != nil {
		return fmt.Errorf("party: add member: %w", err)
	}
	return nil
}

// EndMember implements write.PartyRepository.
func (Parties) EndMember(ctx context.Context, tenant, partyID, student uuid.UUID, reason string, at time.Time) error {
	_, err := run(ctx, `UPDATE multiplayer.party_members SET left_at = $4, left_reason = $5, updated_at = $4, updated_by = $6,
		version = version + 1 WHERE tenant_id = $1 AND party_id = $2 AND student_id = $3 AND left_at IS NULL AND deleted_at IS NULL`,
		tenant, partyID, student, at, reason, actor(ctx))
	if err != nil {
		return fmt.Errorf("party: end member: %w", err)
	}
	return nil
}

// SetRoles implements write.PartyRepository: demote, then promote (the one-leader index holds at every statement).
func (Parties) SetRoles(ctx context.Context, p *party.Party) error {
	if _, err := run(ctx, `UPDATE multiplayer.party_members SET role = 'member', updated_at = now(), version = version + 1
		WHERE tenant_id = $1 AND party_id = $2 AND left_at IS NULL AND deleted_at IS NULL AND role = 'leader' AND student_id <> $3`,
		p.TenantID, p.ID, p.LeaderID); err != nil {
		return fmt.Errorf("party: demote: %w", err)
	}
	if _, err := run(ctx, `UPDATE multiplayer.party_members SET role = 'leader', updated_at = now(), version = version + 1
		WHERE tenant_id = $1 AND party_id = $2 AND left_at IS NULL AND deleted_at IS NULL AND student_id = $3 AND role <> 'leader'`,
		p.TenantID, p.ID, p.LeaderID); err != nil {
		return fmt.Errorf("party: promote: %w", err)
	}
	return nil
}

// AppendHistory implements write.PartyRepository (append-only, ids and enums only).
func (Parties) AppendHistory(ctx context.Context, h write.History) error {
	data, err := json.Marshal(h.Data)
	if err != nil {
		return fmt.Errorf("party: history data: %w", err)
	}
	// tenancyguard:ok append-only insert under the TenantScope (RLS WITH CHECK on tenant_id)
	if _, err := run(ctx, `INSERT INTO multiplayer.party_histories (history_id, tenant_id, school_id, party_id, event_type, actor_id,
		data, occurred_at) VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8)`, ids.New(), h.TenantID, h.SchoolID, h.PartyID, h.EventType,
		h.ActorID, string(data), h.At); err != nil {
		return fmt.Errorf("party: history: %w", err)
	}
	return nil
}

// KickedSince implements write.PartyRepository.
func (Parties) KickedSince(ctx context.Context, tenant, partyID, student uuid.UUID, since time.Time) (bool, error) {
	return exists(ctx, `SELECT EXISTS (SELECT 1 FROM multiplayer.party_members WHERE tenant_id = $1 AND party_id = $2 AND student_id = $3
		AND left_reason = 'kicked' AND left_at > $4 AND deleted_at IS NULL)`, tenant, partyID, student, since)
}

// HistorySince implements write.PartyRepository.
func (Parties) HistorySince(ctx context.Context, tenant, actorID uuid.UUID, types []string, since time.Time) ([]time.Time, error) {
	// tenancyguard:ok append-only history under the TenantScope (TA table: no deleted_at)
	return times(ctx, `SELECT occurred_at FROM multiplayer.party_histories WHERE tenant_id = $1 AND actor_id = $2
		AND event_type = ANY($3::varchar[]) AND occurred_at > $4 ORDER BY occurred_at`, tenant, actorID, types, since)
}

// MembershipsOf implements write.PartyRepository.
func (Parties) MembershipsOf(ctx context.Context, tenant, student uuid.UUID) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, `SELECT party_id FROM multiplayer.party_members WHERE tenant_id = $1 AND student_id = $2
		AND left_at IS NULL AND deleted_at IS NULL ORDER BY joined_at`, tenant, student)
	if err != nil {
		return nil, fmt.Errorf("party: memberships: %w", err)
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

// RenameLed implements write.PartyRepository.
func (Parties) RenameLed(ctx context.Context, tenant, user uuid.UUID, name string) (int, error) {
	n, err := run(ctx, `UPDATE multiplayer.parties SET name = $3, updated_at = now(), version = version + 1
		WHERE tenant_id = $1 AND (leader_id = $2 OR created_by = $2) AND name <> $3 AND deleted_at IS NULL`, tenant, user, name)
	if err != nil {
		return 0, fmt.Errorf("party: rename: %w", err)
	}
	return int(n), nil
}

// ---------- Invitations ----------

const invitationColumns = `invitation_id, tenant_id, school_id, party_id, inviter_id, invitee_id, status, created_at, expires_at,
	responded_at, end_reason`

type scanner interface{ Scan(dest ...any) error }

func scanInvitation(s scanner, extra ...any) (party.Invitation, error) {
	var i party.Invitation
	var responded sql.NullTime
	var reason sql.NullString
	dest := append([]any{&i.ID, &i.TenantID, &i.SchoolID, &i.PartyID, &i.InviterID, &i.InviteeID, &i.Status, &i.CreatedAt, &i.ExpiresAt,
		&responded, &reason}, extra...)
	if err := s.Scan(dest...); err != nil {
		return i, err
	}
	i.CreatedAt, i.ExpiresAt = i.CreatedAt.UTC(), i.ExpiresAt.UTC()
	if responded.Valid {
		at := responded.Time.UTC()
		i.RespondedAt = &at
	}
	i.EndReason = reason.String
	return i, nil
}

// CreateInvitation implements write.PartyRepository.
func (Parties) CreateInvitation(ctx context.Context, i *party.Invitation) error {
	// tenancyguard:ok insert under the TenantScope (RLS WITH CHECK on tenant_id)
	_, err := run(ctx, `INSERT INTO multiplayer.party_invitations (invitation_id, tenant_id, created_at, created_by, updated_at, party_id,
		school_id, inviter_id, invitee_id, status, expires_at) VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8, 'pending', $9)`,
		i.ID, i.TenantID, i.CreatedAt, actor(ctx), i.PartyID, i.SchoolID, i.InviterID, i.InviteeID, i.ExpiresAt)
	if unique(err, "party_invitations_pending_uq") {
		return write.ErrPendingExists
	}
	if err != nil {
		return fmt.Errorf("party: create invitation: %w", err)
	}
	return nil
}

// Invitation implements write.PartyRepository.
func (Parties) Invitation(ctx context.Context, tenant, id uuid.UUID, lock bool) (*party.Invitation, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	q := `SELECT ` + invitationColumns + ` FROM multiplayer.party_invitations WHERE tenant_id = $1 AND invitation_id = $2
		AND deleted_at IS NULL`
	if lock {
		q += ` FOR UPDATE`
	}
	i, err := scanInvitation(t.QueryRowContext(ctx, q, tenant, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("party: invitation: %w", err)
	}
	return &i, nil
}

// SaveInvitation implements write.PartyRepository.
func (Parties) SaveInvitation(ctx context.Context, i *party.Invitation) error {
	var reason any
	if i.EndReason != "" {
		reason = i.EndReason
	}
	_, err := run(ctx, `UPDATE multiplayer.party_invitations SET status = $3, responded_at = $4, end_reason = $5, updated_at = now(),
		updated_by = $6, version = version + 1 WHERE tenant_id = $1 AND invitation_id = $2 AND deleted_at IS NULL`,
		i.TenantID, i.ID, i.Status, i.RespondedAt, reason, actor(ctx))
	if err != nil {
		return fmt.Errorf("party: save invitation: %w", err)
	}
	return nil
}

// Pending implements write.PartyRepository.
func (Parties) Pending(ctx context.Context, tenant uuid.UUID, f write.PendingFilter) ([]party.Invitation, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	q := `SELECT ` + invitationColumns + ` FROM multiplayer.party_invitations WHERE tenant_id = $1 AND status = 'pending'
		AND deleted_at IS NULL`
	args := []any{tenant}
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	if f.PartyID != nil {
		q += ` AND party_id = ` + arg(*f.PartyID)
	}
	if f.InviterID != nil {
		q += ` AND inviter_id = ` + arg(*f.InviterID)
	}
	if f.InviteeID != nil {
		q += ` AND invitee_id = ` + arg(*f.InviteeID)
	}
	if f.Student != nil {
		s := arg(*f.Student)
		q += ` AND (inviter_id = ` + s + ` OR invitee_id = ` + s + `)`
	}
	if f.Between != nil {
		a, b := arg(f.Between[0]), arg(f.Between[1])
		q += ` AND ((inviter_id = ` + a + ` AND invitee_id = ` + b + `) OR (inviter_id = ` + b + ` AND invitee_id = ` + a + `))`
	}
	if f.ExceptParty != nil {
		q += ` AND party_id <> ` + arg(*f.ExceptParty)
	}
	if f.Expired != nil {
		q += ` AND expires_at <= ` + arg(*f.Expired)
	}
	q += ` ORDER BY created_at, invitation_id FOR UPDATE`
	rows, err := t.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("party: pending: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []party.Invitation
	for rows.Next() {
		i, err := scanInvitation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// InvitesSince implements write.PartyRepository (every invitation sent counts toward party_invite_rate).
func (Parties) InvitesSince(ctx context.Context, tenant, inviter uuid.UUID, since time.Time) ([]time.Time, error) {
	return times(ctx, `SELECT created_at FROM multiplayer.party_invitations WHERE tenant_id = $1 AND inviter_id = $2 AND created_at > $3
		AND deleted_at IS NULL ORDER BY created_at`, tenant, inviter, since)
}

// RejectionsSince implements write.PartyRepository.
func (Parties) RejectionsSince(ctx context.Context, tenant, inviter, invitee uuid.UUID, since time.Time) ([]time.Time, error) {
	return times(ctx, `SELECT responded_at FROM multiplayer.party_invitations WHERE tenant_id = $1 AND inviter_id = $2 AND invitee_id = $3
		AND status = 'rejected' AND responded_at > $4 AND deleted_at IS NULL ORDER BY responded_at`, tenant, inviter, invitee, since)
}

// AcceptedInvitation implements write.PartyRepository.
func (Parties) AcceptedInvitation(ctx context.Context, tenant, partyID, student uuid.UUID, now time.Time) (bool, error) {
	return exists(ctx, `SELECT EXISTS (SELECT 1 FROM multiplayer.party_invitations WHERE tenant_id = $1 AND party_id = $2
		AND invitee_id = $3 AND status = 'accepted' AND expires_at > $4 AND deleted_at IS NULL)`, tenant, partyID, student, now)
}

// ---------- Read side (read.PartyReader, entx.ReadSession) ----------

// ActiveParty implements read.PartyReader.
func (r Parties) ActiveParty(ctx context.Context, tenant, student uuid.UUID) (*party.Party, error) {
	id, err := r.ActivePartyOf(ctx, tenant, student)
	if err != nil || id == uuid.Nil {
		return nil, err
	}
	return r.Load(ctx, tenant, id, false)
}

// Party implements read.PartyReader.
func (r Parties) Party(ctx context.Context, tenant, id uuid.UUID) (*party.Party, error) {
	return r.Load(ctx, tenant, id, false)
}

// Invitations implements read.PartyReader.
func (Parties) Invitations(ctx context.Context, tenant, invitee uuid.UUID, status string, now time.Time, limit int) ([]read.Invitation, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, `SELECT i.invitation_id, i.tenant_id, i.school_id, i.party_id, i.inviter_id, i.invitee_id, i.status,
		i.created_at, i.expires_at, i.responded_at, i.end_reason, p.name
		FROM multiplayer.party_invitations i JOIN multiplayer.parties p ON p.party_id = i.party_id AND p.tenant_id = i.tenant_id
		WHERE i.tenant_id = $1 AND i.invitee_id = $2 AND i.deleted_at IS NULL AND p.deleted_at IS NULL
		AND ($3 = '' OR i.status = $3) AND ($3 <> 'pending' OR i.expires_at > $4)
		ORDER BY i.created_at DESC, i.invitation_id LIMIT $5`, tenant, invitee, status, now, limit)
	if err != nil {
		return nil, fmt.Errorf("party: invitations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.Invitation
	for rows.Next() {
		var name string
		i, err := scanInvitation(rows, &name)
		if err != nil {
			return nil, err
		}
		out = append(out, read.Invitation{Invitation: i, PartyName: name})
	}
	return out, rows.Err()
}

// ---------- Platform job, sanctions, outbox ----------

// Expiry implements write.ExpiryRepository (SystemPlatformScope: every tenant).
type Expiry struct{}

var _ write.ExpiryRepository = Expiry{}

// ExpiredPending implements write.ExpiryRepository.
func (Expiry) ExpiredPending(ctx context.Context, now time.Time, limit int) ([]write.InvitationRef, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform job party-invitation-expiry (SystemPlatformScope): pending invitations of every tenant, ids only
	rows, err := t.QueryContext(ctx, `SELECT tenant_id, invitation_id FROM multiplayer.party_invitations
		WHERE status = 'pending' AND expires_at <= $1 AND deleted_at IS NULL ORDER BY expires_at LIMIT $2`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("party: expired: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []write.InvitationRef
	for rows.Next() {
		var r write.InvitationRef
		if err := rows.Scan(&r.TenantID, &r.InvitationID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Sanctions implements write.SanctionWriter on ref.sanctions.
type Sanctions struct{}

var _ write.SanctionWriter = Sanctions{}

// Sanction implements write.SanctionWriter (newer source_version only; a sanction never moves to another tenant).
func (Sanctions) Sanction(ctx context.Context, tenant, sanction, user uuid.UUID, typ string, until *time.Time, active bool,
	version int64) (bool, error) {
	// tenancyguard:ok projection upsert under the event TenantScope (RLS on tenant_id); ref.* has no deleted_at (rule 2)
	n, err := run(ctx, `INSERT INTO ref.sanctions AS s (sanction_id, tenant_id, user_id, sanction_type, until, active, source_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (sanction_id) DO UPDATE SET sanction_type = $4, until = $5, active = $6, source_version = $7
		WHERE s.source_version < $7 AND s.tenant_id = $2 AND s.user_id = $3`, sanction, tenant, user, typ, until, active, version)
	if err != nil {
		return false, fmt.Errorf("ref sanctions: %w", err)
	}
	return n > 0, nil
}

// OwnerTenant implements write.SanctionWriter.
func (Sanctions) OwnerTenant(ctx context.Context, student uuid.UUID) (*uuid.UUID, error) {
	return Refs{}.OwnerTenant(ctx, "students", student)
}

// Publisher implements write.EventPublisher with the outbox writer of the module.
type Publisher struct{ W *outbox.Writer }

var _ write.EventPublisher = Publisher{}

// Publish writes e into the outbox of the transaction.
func (p Publisher) Publish(ctx context.Context, e write.Event) error {
	_, err := p.W.Publish(ctx, outbox.Message{Type: e.Type, Subject: outbox.Subject{Type: e.SubjectType, ID: e.SubjectID},
		PartitionKey: e.PartitionKey, TenantID: e.TenantID, OccurredAt: e.OccurredAt, Payload: e.Payload, SchemaVersion: e.SchemaVersion,
		Platform: e.Platform})
	return err
}
