// Package persistence implements the repositories and projection readers of module competition with explicit SQL
// on the transaction of ctx (entx.UnitOfWork): tenant and deleted_at conditions are written in every statement
// (AGENTS R27) and RLS checks the tenant again.
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
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/hub"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

func tx(ctx context.Context) (*sql.Tx, error) {
	t, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	return t, nil
}

// Directory implements port.Directory on the ref.* projections.
type Directory struct{}

var _ port.Directory = Directory{}

// Student implements port.Directory.
func (Directory) Student(ctx context.Context, tenant, student uuid.UUID) (*port.StudentRef, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var r port.StudentRef
	var grade sql.NullInt32
	var status string
	var deleted sql.NullTime
	// tenancyguard:ok projection read under the TenantScope of the command; deleted_at mirrors the source (cleanup)
	err = t.QueryRowContext(ctx, `SELECT school_id, grade_level, status, deleted_at FROM ref.students
		WHERE student_id = $1 AND tenant_id = $2`, student, tenant).Scan(&r.SchoolID, &grade, &status, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ref students: %w", err)
	}
	r.GradeLevel = int(grade.Int32)
	r.Active = status == "active" && !deleted.Valid
	return &r, nil
}

type bounds struct {
	Min hub.Vec3 `json:"min"`
	Max hub.Vec3 `json:"max"`
}

// Area implements port.Directory (ref.world_areas is platform content: no tenant).
func (Directory) Area(ctx context.Context, area string) (*hub.Area, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var a hub.Area
	var b, spawns, bands []byte
	var status string
	// tenancyguard:ok ref.world_areas is global platform content (G): no tenant_id, no deleted_at (status mirrors the source)
	err = t.QueryRowContext(ctx, `SELECT area_id, capacity, max_speed_mps::float8, bounds, spawn_points,
		array_to_json(allowed_grade_bands), requires_unlock, status FROM ref.world_areas WHERE area_id = $1`, area).
		Scan(&a.ID, &a.Capacity, &a.MaxSpeedMPS, &b, &spawns, &bands, &a.RequiresUnlock, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ref world_areas: %w", err)
	}
	var bb bounds
	if err := json.Unmarshal(b, &bb); err != nil {
		return nil, fmt.Errorf("ref world_areas %s: bounds: %w", area, err)
	}
	a.Min, a.Max = bb.Min, bb.Max
	if err := json.Unmarshal(spawns, &a.Spawns); err != nil {
		return nil, fmt.Errorf("ref world_areas %s: spawn_points: %w", area, err)
	}
	if err := json.Unmarshal(bands, &a.Bands); err != nil {
		return nil, fmt.Errorf("ref world_areas %s: allowed_grade_bands: %w", area, err)
	}
	a.Active = status == "active"
	return &a, nil
}

// Unlocked implements port.Directory.
func (Directory) Unlocked(ctx context.Context, tenant, student uuid.UUID, area string) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	var ok bool
	// tenancyguard:ok projection read under the TenantScope of the command (ref.* has no deleted_at, rule 2)
	err = t.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM ref.area_unlocks WHERE tenant_id = $1 AND student_id = $2 AND area_id = $3)`,
		tenant, student, area).Scan(&ok)
	return ok, err
}

// Player implements port.Directory: the public profile only (R16).
func (Directory) Player(ctx context.Context, tenant, student uuid.UUID) (*hub.PublicPlayer, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var nick sql.NullString
	var level int
	var avatar []byte
	// tenancyguard:ok projection read under the TenantScope of the command (ref.* has no deleted_at, rule 2)
	err = t.QueryRowContext(ctx, `SELECT nickname, level, avatar_config FROM ref.player_profiles WHERE student_id = $1 AND tenant_id = $2`,
		student, tenant).Scan(&nick, &level, &avatar)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ref player_profiles: %w", err)
	}
	p := &hub.PublicPlayer{UserID: student, Nickname: nick.String, Level: level}
	if len(avatar) > 0 {
		if err := json.Unmarshal(avatar, &p.AvatarConfig); err != nil {
			return nil, fmt.Errorf("ref player_profiles: avatar_config: %w", err)
		}
	}
	return p, nil
}

func idSet(ctx context.Context, query string, args ...any) (map[uuid.UUID]bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := t.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[uuid.UUID]bool{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// Friends implements port.Directory.
func (Directory) Friends(ctx context.Context, tenant, student uuid.UUID) (map[uuid.UUID]bool, error) {
	// tenancyguard:ok projection read under the TenantScope of the command (ref.* has no deleted_at, rule 2)
	return idSet(ctx, `SELECT CASE WHEN user_low_id = $2 THEN user_high_id ELSE user_low_id END FROM ref.friendships
		WHERE tenant_id = $1 AND active AND (user_low_id = $2 OR user_high_id = $2)`, tenant, student)
}

// Blocks implements port.Directory: blocked by the student or blocking the student.
func (Directory) Blocks(ctx context.Context, tenant, student uuid.UUID) (map[uuid.UUID]bool, error) {
	// tenancyguard:ok projection read under the TenantScope of the command (ref.* has no deleted_at, rule 2)
	return idSet(ctx, `SELECT CASE WHEN blocker_id = $2 THEN blocked_id ELSE blocker_id END FROM ref.blocks
		WHERE tenant_id = $1 AND active AND (blocker_id = $2 OR blocked_id = $2)`, tenant, student)
}

// AuditHubMembership implements port.Directory: settings_subset.audit_hub_membership of the tenant, default on
// (CONFIG_DEFAULTS audit_hub_membership, pilot).
func (Directory) AuditHubMembership(ctx context.Context, tenant uuid.UUID) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	var v sql.NullBool
	// tenancyguard:ok ref.tenants is the tenant registry (key = tenant_id, no deleted_at: rule 2)
	err = t.QueryRowContext(ctx, `SELECT (settings_subset->>'audit_hub_membership')::boolean FROM ref.tenants WHERE tenant_id = $1`,
		tenant).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !v.Valid) {
		return true, nil
	}
	if err != nil {
		return true, fmt.Errorf("ref tenants: %w", err)
	}
	return v.Bool, nil
}

// Store implements port.RoomStore on multiplayer.rooms, multiplayer.session_memberships, competition.integrity_signals.
type Store struct{}

var _ port.RoomStore = Store{}

func exec(ctx context.Context, query string, args ...any) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	res, err := t.ExecContext(ctx, query, args...)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// CreateRoom implements port.RoomStore.
func (Store) CreateRoom(ctx context.Context, r port.RoomRecord) error {
	_, err := exec(ctx, `INSERT INTO multiplayer.rooms (room_id, tenant_id, created_at, updated_at, room_type, area_id, school_id,
		grade_band, capacity, status) VALUES ($1, $2, $3, $3, $4, $5, $6, $7, $8, 'open')`,
		r.RoomID, r.Partition.TenantID, r.CreatedAt, hub.RoomTypeHub, r.AreaID, r.Partition.SchoolID, r.Partition.GradeBand, r.Capacity)
	if err != nil {
		return fmt.Errorf("rooms: insert: %w", err)
	}
	return nil
}

// CloseRoom implements port.RoomStore: status closed and closed_at once.
func (Store) CloseRoom(ctx context.Context, tenant, room uuid.UUID, at time.Time) (bool, error) {
	return exec(ctx, `UPDATE multiplayer.rooms SET status = 'closed', closed_at = $3, updated_at = $3, version = version + 1
		WHERE room_id = $1 AND tenant_id = $2 AND status = 'open' AND deleted_at IS NULL`, room, tenant, at)
}

// OpenHubRooms implements port.RoomStore (platform scope of the job: every tenant).
func (Store) OpenHubRooms(ctx context.Context, createdBefore time.Time, limit int) ([]port.RoomRef, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform job close-idle-hub-instances (SystemPlatformScope): open hub rooms of every tenant, ids only
	rows, err := t.QueryContext(ctx, `SELECT tenant_id, room_id FROM multiplayer.rooms
		WHERE room_type = 'hub_instance' AND status = 'open' AND deleted_at IS NULL AND created_at < $1
		ORDER BY created_at LIMIT $2`, createdBefore, limit)
	if err != nil {
		return nil, fmt.Errorf("rooms: open: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.RoomRef
	for rows.Next() {
		var r port.RoomRef
		if err := rows.Scan(&r.TenantID, &r.RoomID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// OpenMembership implements port.RoomStore (one active membership per room and student: unique index).
func (Store) OpenMembership(ctx context.Context, m port.MembershipRecord) error {
	_, err := exec(ctx, `INSERT INTO multiplayer.session_memberships (membership_id, tenant_id, created_at, updated_at, room_id,
		room_type, school_id, student_id, game_session_id, joined_at) VALUES ($1, $2, $3, $3, $4, $5, $6, $7, $8, $3)
		ON CONFLICT (room_id, student_id) WHERE left_at IS NULL AND deleted_at IS NULL DO NOTHING`,
		m.MembershipID, m.TenantID, m.JoinedAt, m.RoomID, hub.RoomTypeHub, m.SchoolID, m.StudentID, m.GameSessionID)
	if err != nil {
		return fmt.Errorf("session_memberships: insert: %w", err)
	}
	return nil
}

// CloseMembership implements port.RoomStore.
func (Store) CloseMembership(ctx context.Context, tenant, room, student uuid.UUID, reason string, at time.Time) (bool, error) {
	return exec(ctx, `UPDATE multiplayer.session_memberships SET left_at = $4, leave_reason = $5, updated_at = $4, version = version + 1
		WHERE room_id = $1 AND tenant_id = $2 AND student_id = $3 AND left_at IS NULL AND deleted_at IS NULL`,
		room, tenant, student, at, reason)
}

// RecordSignal implements port.RoomStore (append-only).
func (Store) RecordSignal(ctx context.Context, s port.SignalRecord) error {
	ev, err := json.Marshal(s.Evidence)
	if err != nil {
		return err
	}
	var school any
	if s.SchoolID != uuid.Nil {
		school = s.SchoolID
	}
	// tenancyguard:ok append-only insert of competition.integrity_signals under the TenantScope (RLS WITH CHECK)
	_, err = exec(ctx, `INSERT INTO competition.integrity_signals (signal_id, tenant_id, school_id, student_id, room_id, signal_type,
		score, evidence, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		s.SignalID, s.TenantID, school, s.StudentID, s.RoomID, s.SignalType, s.Score, string(ev), s.CreatedAt)
	if err != nil {
		return fmt.Errorf("integrity_signals: insert: %w", err)
	}
	return nil
}

// Auditor implements port.Auditor with the outbox audit recorder of the module.
type Auditor struct{ R audit.Recorder }

// Record implements port.Auditor.
func (a Auditor) Record(ctx context.Context, e audit.Event) error { return a.R.Record(ctx, e) }
