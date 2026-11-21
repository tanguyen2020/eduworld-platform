package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// Activities implements write.ActivityRepository on multiplayer.party_activities, rooms and session_memberships (F101):
// explicit tenant_id and deleted_at conditions under the TenantScope of the transaction (R27).
type Activities struct{}

var _ write.ActivityRepository = Activities{}

const activityColumns = `party_activity_id, tenant_id, school_id, party_id, activity_type, activity_ref, game_mode, room_id, room_type,
	external_ref, status, min_members, max_members, allow_continue_on_disconnect, started_at, ended_at, end_reason, result_summary`

func scanActivity(s scanner) (*party.Activity, error) {
	var a party.Activity
	var mode, reason sql.NullString
	var ext uuid.NullUUID
	var ended sql.NullTime
	var summary []byte
	if err := s.Scan(&a.ID, &a.TenantID, &a.SchoolID, &a.PartyID, &a.Type, &a.Ref, &mode, &a.RoomID, &a.RoomType, &ext, &a.Status,
		&a.MinMembers, &a.MaxMembers, &a.AllowContinue, &a.StartedAt, &ended, &reason, &summary); err != nil {
		return nil, err
	}
	a.GameMode, a.EndReason, a.StartedAt = mode.String, reason.String, a.StartedAt.UTC()
	if ext.Valid {
		e := ext.UUID
		a.ExternalRef = &e
	}
	if ended.Valid {
		at := ended.Time.UTC()
		a.EndedAt = &at
	}
	if len(summary) > 0 {
		if err := json.Unmarshal(summary, &a.Summary); err != nil {
			return nil, fmt.Errorf("party activity: summary: %w", err)
		}
	}
	return &a, nil
}

func oneActivity(ctx context.Context, query string, args ...any) (*party.Activity, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	a, err := scanActivity(t.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("party activity: %w", err)
	}
	return a, nil
}

// Running implements write.ActivityRepository.
func (Activities) Running(ctx context.Context, tenant, partyID uuid.UUID) (*party.Activity, error) {
	return oneActivity(ctx, `SELECT `+activityColumns+` FROM multiplayer.party_activities WHERE tenant_id = $1 AND party_id = $2
		AND status = 'running' AND deleted_at IS NULL FOR UPDATE`, tenant, partyID)
}

// Get implements write.ActivityRepository.
func (Activities) Get(ctx context.Context, tenant, id uuid.UUID) (*party.Activity, error) {
	return oneActivity(ctx, `SELECT `+activityColumns+` FROM multiplayer.party_activities WHERE tenant_id = $1 AND party_activity_id = $2
		AND deleted_at IS NULL FOR UPDATE`, tenant, id)
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Create implements write.ActivityRepository.
func (Activities) Create(ctx context.Context, a *party.Activity) error {
	// tenancyguard:ok insert under the TenantScope (RLS WITH CHECK on tenant_id)
	_, err := run(ctx, `INSERT INTO multiplayer.party_activities (party_activity_id, tenant_id, created_at, created_by, updated_at, school_id,
		party_id, activity_type, activity_ref, game_mode, room_id, room_type, external_ref, status, min_members, max_members,
		allow_continue_on_disconnect, started_at) VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8, $9, $10, $11, $12, 'running', $13, $14, $15, $3)`,
		a.ID, a.TenantID, a.StartedAt, actor(ctx), a.SchoolID, a.PartyID, a.Type, a.Ref, nullString(a.GameMode), a.RoomID, a.RoomType,
		a.ExternalRef, a.MinMembers, a.MaxMembers, a.AllowContinue)
	if unique(err, "party_activities_one_running_uq", "party_activities_pkey") {
		return write.ErrActivityRunning
	}
	if err != nil {
		return fmt.Errorf("party activity: create: %w", err)
	}
	if _, err := run(ctx, `UPDATE multiplayer.parties SET current_activity_id = $3, current_game_session_ref = $3
		WHERE tenant_id = $1 AND party_id = $2 AND deleted_at IS NULL`, a.TenantID, a.PartyID, a.ID); err != nil {
		return fmt.Errorf("party activity: current: %w", err)
	}
	return nil
}

// End implements write.ActivityRepository.
func (Activities) End(ctx context.Context, a *party.Activity) error {
	summary, err := json.Marshal(party.CleanSummary(a.Summary))
	if err != nil {
		return err
	}
	if _, err := run(ctx, `UPDATE multiplayer.party_activities SET status = $3, ended_at = $4, end_reason = $5, result_summary = $6::jsonb,
		updated_at = $4, updated_by = $7, version = version + 1 WHERE tenant_id = $1 AND party_activity_id = $2 AND deleted_at IS NULL`,
		a.TenantID, a.ID, a.Status, a.EndedAt, a.EndReason, string(summary), actor(ctx)); err != nil {
		return fmt.Errorf("party activity: end: %w", err)
	}
	if _, err := run(ctx, `UPDATE multiplayer.parties SET current_activity_id = NULL, current_game_session_ref = NULL
		WHERE tenant_id = $1 AND party_id = $2 AND current_activity_id = $3 AND deleted_at IS NULL`, a.TenantID, a.PartyID, a.ID); err != nil {
		return fmt.Errorf("party activity: clear current: %w", err)
	}
	return nil
}

// CreateRoom implements write.ActivityRepository (a retry of an idempotent start finds the room already there).
func (Activities) CreateRoom(ctx context.Context, r write.SessionRoom) error {
	// tenancyguard:ok insert under the TenantScope (RLS WITH CHECK on tenant_id)
	_, err := run(ctx, `INSERT INTO multiplayer.rooms (room_id, tenant_id, created_at, updated_at, room_type, area_id, school_id, grade_band,
		capacity, status) VALUES ($1, $2, $3, $3, $4, $5, $6, $7, $8, 'open') ON CONFLICT (room_id) DO NOTHING`,
		r.RoomID, r.TenantID, r.CreatedAt, r.RoomType, r.AreaID, r.SchoolID, r.GradeBand, r.Capacity)
	if err != nil {
		return fmt.Errorf("rooms: insert party room: %w", err)
	}
	return nil
}

// CloseRoom implements write.ActivityRepository.
func (Activities) CloseRoom(ctx context.Context, tenant, room uuid.UUID, at time.Time) error {
	_, err := run(ctx, `UPDATE multiplayer.rooms SET status = 'closed', closed_at = $3, updated_at = $3, version = version + 1
		WHERE room_id = $1 AND tenant_id = $2 AND status = 'open' AND deleted_at IS NULL`, room, tenant, at)
	return err
}

// OpenMemberships implements write.ActivityRepository.
func (Activities) OpenMemberships(ctx context.Context, a *party.Activity, students []uuid.UUID, sessions map[uuid.UUID]uuid.UUID,
	at time.Time) error {
	for _, s := range students {
		var session any
		if id, ok := sessions[s]; ok {
			session = id
		}
		var encounter any
		if a.RoomType == party.RoomBoss {
			encounter = a.RoomID
		}
		// tenancyguard:ok insert under the TenantScope (RLS WITH CHECK on tenant_id)
		if _, err := run(ctx, `INSERT INTO multiplayer.session_memberships (membership_id, tenant_id, created_at, updated_at, room_id,
			room_type, school_id, student_id, game_session_id, party_id, encounter_id, party_activity_id, joined_at)
			VALUES ($1, $2, $3, $3, $4, $5, $6, $7, $8, $9, $10, $11, $3)`, ids.New(), a.TenantID, at, a.RoomID, a.RoomType, a.SchoolID, s,
			session, a.PartyID, encounter, a.ID); err != nil {
			return fmt.Errorf("session membership: open: %w", err)
		}
	}
	return nil
}

// OpenMembers implements write.ActivityRepository.
func (Activities) OpenMembers(ctx context.Context, tenant, activity uuid.UUID) (map[uuid.UUID]bool, error) {
	return idSet(ctx, `SELECT student_id FROM multiplayer.session_memberships WHERE tenant_id = $1 AND party_activity_id = $2
		AND left_at IS NULL AND deleted_at IS NULL`, tenant, activity)
}

// Participants implements write.ActivityRepository.
func (Activities) Participants(ctx context.Context, tenant, activity uuid.UUID) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, `SELECT DISTINCT ON (student_id) student_id FROM multiplayer.session_memberships
		WHERE tenant_id = $1 AND party_activity_id = $2 AND deleted_at IS NULL ORDER BY student_id`, tenant, activity)
	if err != nil {
		return nil, fmt.Errorf("session membership: participants: %w", err)
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

// CloseMembership implements write.ActivityRepository.
func (Activities) CloseMembership(ctx context.Context, tenant, activity, student uuid.UUID, reason string, at time.Time) (bool, error) {
	n, err := run(ctx, `UPDATE multiplayer.session_memberships SET left_at = $4, leave_reason = $5, updated_at = $4, version = version + 1
		WHERE tenant_id = $1 AND party_activity_id = $2 AND student_id = $3 AND left_at IS NULL AND deleted_at IS NULL`,
		tenant, activity, student, at, reason)
	if err != nil {
		return false, fmt.Errorf("session membership: close: %w", err)
	}
	return n > 0, nil
}

// CloseMemberships implements write.ActivityRepository.
func (Activities) CloseMemberships(ctx context.Context, tenant, activity uuid.UUID, reason string, at time.Time) error {
	_, err := run(ctx, `UPDATE multiplayer.session_memberships SET left_at = $3, leave_reason = $4, updated_at = $3, version = version + 1
		WHERE tenant_id = $1 AND party_activity_id = $2 AND left_at IS NULL AND deleted_at IS NULL`, tenant, activity, at, reason)
	if err != nil {
		return fmt.Errorf("session membership: close all: %w", err)
	}
	return nil
}

// RoomOf implements write.ActivityRepository: the private_instance room of the running activity the student still
// belongs to (open membership), with every open member of that activity.
func (Activities) RoomOf(ctx context.Context, tenant, student uuid.UUID, area string) (*write.PartyRoom, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var r write.PartyRoom
	var band sql.NullString
	var school uuid.NullUUID
	err = t.QueryRowContext(ctx, `SELECT r.room_id, r.room_type, r.area_id, a.party_id, a.party_activity_id, r.school_id, r.grade_band,
		r.capacity FROM multiplayer.session_memberships m
		JOIN multiplayer.party_activities a ON a.party_activity_id = m.party_activity_id AND a.tenant_id = m.tenant_id
			AND a.status = 'running' AND a.deleted_at IS NULL
		JOIN multiplayer.rooms r ON r.room_id = m.room_id AND r.tenant_id = m.tenant_id AND r.status = 'open' AND r.deleted_at IS NULL
		WHERE m.tenant_id = $1 AND m.student_id = $2 AND m.left_at IS NULL AND m.deleted_at IS NULL AND m.room_type = 'private_instance'
			AND ($3 = '' OR r.area_id = $3)
		ORDER BY m.joined_at DESC LIMIT 1`, tenant, student, area).
		Scan(&r.RoomID, &r.RoomType, &r.AreaID, &r.PartyID, &r.ActivityID, &school, &band, &r.Capacity)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("session membership: room of student: %w", err)
	}
	r.SchoolID, r.GradeBand = school.UUID, band.String
	open, err := Activities{}.OpenMembers(ctx, tenant, r.ActivityID)
	if err != nil {
		return nil, err
	}
	for id := range open {
		r.Members = append(r.Members, id)
	}
	return &r, nil
}

// Member implements write.ActivityRepository.
func (Activities) Member(ctx context.Context, tenant, room, student uuid.UUID) (bool, error) {
	return exists(ctx, `SELECT EXISTS (SELECT 1 FROM multiplayer.session_memberships WHERE tenant_id = $1 AND room_id = $2 AND student_id = $3
		AND left_at IS NULL AND deleted_at IS NULL)`, tenant, room, student)
}

// SetGameSession implements write.ActivityRepository (game_session_id of the open membership; a takeover on another
// device replaces it, ADR-0031).
func (Activities) SetGameSession(ctx context.Context, tenant, room, student, session uuid.UUID) error {
	_, err := run(ctx, `UPDATE multiplayer.session_memberships SET game_session_id = $4, updated_at = now(), version = version + 1
		WHERE tenant_id = $1 AND room_id = $2 AND student_id = $3 AND left_at IS NULL AND deleted_at IS NULL
		AND game_session_id IS DISTINCT FROM $4`, tenant, room, student, session)
	return err
}

// ---------- areas (projection ref.world_areas, ref.area_unlocks of module game) ----------

// Areas implements write.AreaDirectory and the area part of read.LobbyReader.
type Areas struct{}

var (
	_ write.AreaDirectory = Areas{}
	_ read.LobbyReader    = LobbyRead{}
)

func scanArea(s scanner) (party.AreaRule, error) {
	var a party.AreaRule
	var bands []byte
	var status string
	if err := s.Scan(&a.ID, &bands, &a.RequiresUnlock, &status); err != nil {
		return a, err
	}
	if err := json.Unmarshal(bands, &a.Bands); err != nil {
		return a, fmt.Errorf("ref world_areas %s: bands: %w", a.ID, err)
	}
	a.Active = status == "active"
	return a, nil
}

// Areas implements write.AreaDirectory.
func (Areas) Areas(ctx context.Context, limit int) ([]party.AreaRule, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok ref.world_areas is global platform content (G): no tenant_id, no deleted_at (status mirrors the source)
	rows, err := t.QueryContext(ctx, `SELECT area_id, array_to_json(allowed_grade_bands), requires_unlock, status FROM ref.world_areas
		WHERE status = 'active' ORDER BY area_id LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("ref world_areas: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []party.AreaRule
	for rows.Next() {
		a, err := scanArea(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Area implements write.AreaDirectory.
func (Areas) Area(ctx context.Context, id string) (*party.AreaRule, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok ref.world_areas is global platform content (G): no tenant_id, no deleted_at (status mirrors the source)
	a, err := scanArea(t.QueryRowContext(ctx, `SELECT area_id, array_to_json(allowed_grade_bands), requires_unlock, status
		FROM ref.world_areas WHERE area_id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// Unlocked implements write.AreaDirectory.
func (Areas) Unlocked(ctx context.Context, tenant uuid.UUID, students []uuid.UUID, area string) (map[uuid.UUID]bool, error) {
	// tenancyguard:ok projection read under the TenantScope (ref.* has no deleted_at, rule 2)
	return idSet(ctx, `SELECT student_id FROM ref.area_unlocks WHERE tenant_id = $1 AND student_id = ANY($2::uuid[]) AND area_id = $3`,
		tenant, uuidArray(students), area)
}

// UnlockedAreas implements write.AreaDirectory.
func (Areas) UnlockedAreas(ctx context.Context, tenant uuid.UUID, students []uuid.UUID) (map[uuid.UUID]map[string]bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok projection read under the TenantScope (ref.* has no deleted_at, rule 2)
	rows, err := t.QueryContext(ctx, `SELECT student_id, area_id FROM ref.area_unlocks WHERE tenant_id = $1 AND student_id = ANY($2::uuid[])`,
		tenant, uuidArray(students))
	if err != nil {
		return nil, fmt.Errorf("ref area_unlocks: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[uuid.UUID]map[string]bool{}
	for rows.Next() {
		var id uuid.UUID
		var area string
		if err := rows.Scan(&id, &area); err != nil {
			return nil, err
		}
		if out[id] == nil {
			out[id] = map[string]bool{}
		}
		out[id][area] = true
	}
	return out, rows.Err()
}

// LobbyRead implements read.LobbyReader (entx.ReadSession).
type LobbyRead struct{}

// Areas implements read.LobbyReader.
func (LobbyRead) Areas(ctx context.Context, limit int) ([]party.AreaRule, error) {
	return Areas{}.Areas(ctx, limit)
}

// UnlockedAreas implements read.LobbyReader.
func (LobbyRead) UnlockedAreas(ctx context.Context, tenant uuid.UUID, students []uuid.UUID) (map[uuid.UUID]map[string]bool, error) {
	return Areas{}.UnlockedAreas(ctx, tenant, students)
}

// Subjects implements read.LobbyReader.
func (LobbyRead) Subjects(ctx context.Context, tenant uuid.UUID, students []uuid.UUID, now time.Time) ([]party.Subject, error) {
	out := make([]party.Subject, 0, len(students))
	for _, id := range students {
		s, err := PartyDirectory{}.Subject(ctx, tenant, id, now)
		if err != nil {
			return nil, err
		}
		if s == nil {
			s = &party.Subject{ID: id}
		}
		out = append(out, *s)
	}
	return out, nil
}

// RunningRoom implements read.LobbyReader.
func (LobbyRead) RunningRoom(ctx context.Context, tenant, partyID uuid.UUID) (*uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var id uuid.UUID
	err = t.QueryRowContext(ctx, `SELECT room_id FROM multiplayer.party_activities WHERE tenant_id = $1 AND party_id = $2
		AND status = 'running' AND deleted_at IS NULL`, tenant, partyID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("party activity: running room: %w", err)
	}
	return &id, nil
}

// Sweeps implements write.SweepRepository (platform scope of the job party-presence-sweeper).
type Sweeps struct{}

var _ write.SweepRepository = Sweeps{}

// ActiveParties implements write.SweepRepository: ids only, keyset by party_id.
func (Sweeps) ActiveParties(ctx context.Context, after uuid.UUID, limit int) ([]port.PartyRef, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform job party-presence-sweeper (SystemPlatformScope): active parties of every tenant, ids only
	rows, err := t.QueryContext(ctx, `SELECT tenant_id, party_id FROM multiplayer.parties WHERE status IN ('open','in_activity')
		AND deleted_at IS NULL AND party_id > $1 ORDER BY party_id LIMIT $2`, after, limit)
	if err != nil {
		return nil, fmt.Errorf("parties: active: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.PartyRef
	for rows.Next() {
		var r port.PartyRef
		if err := rows.Scan(&r.TenantID, &r.PartyID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
