// Package port declares the ports of the application layer of module competition (ADR-0087): repositories and
// readers run on the transaction of ctx (entx.UnitOfWork); the room host, the registry and the realtime publisher
// are adapters of infrastructure.
package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/hub"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// UnitOfWork runs fn in one transaction with the scope s (entx.UnitOfWork).
type UnitOfWork interface {
	InTenantTx(ctx context.Context, s tenancy.Scope, fn func(ctx context.Context) error) error
}

// StudentRef is the projected identity data of a student (ref.students).
type StudentRef struct {
	SchoolID   uuid.UUID
	GradeLevel int
	Active     bool
}

// Directory reads the projections of module competition (ref.*) in the transaction of ctx: data of other modules
// come from events only, never from a synchronous call on the hub path (R29, R30, F091 research R2).
type Directory interface {
	Student(ctx context.Context, tenant, student uuid.UUID) (*StudentRef, error)
	Area(ctx context.Context, area string) (*hub.Area, error)
	Unlocked(ctx context.Context, tenant, student uuid.UUID, area string) (bool, error)
	Player(ctx context.Context, tenant, student uuid.UUID) (*hub.PublicPlayer, error)
	// Friends returns the active friends of student; Blocks the students blocked by or blocking student.
	Friends(ctx context.Context, tenant, student uuid.UUID) (map[uuid.UUID]bool, error)
	Blocks(ctx context.Context, tenant, student uuid.UUID) (map[uuid.UUID]bool, error)
	// AuditHubMembership is audit_hub_membership of the tenant (CONFIG_DEFAULTS, default on).
	AuditHubMembership(ctx context.Context, tenant uuid.UUID) (bool, error)
}

// RoomRecord is a row of multiplayer.rooms.
type RoomRecord struct {
	RoomID    uuid.UUID
	AreaID    string
	Partition hub.Partition
	Capacity  int
	CreatedAt time.Time
}

// MembershipRecord is a row of multiplayer.session_memberships.
type MembershipRecord struct {
	MembershipID  uuid.UUID
	TenantID      uuid.UUID
	RoomID        uuid.UUID
	SchoolID      uuid.UUID
	StudentID     uuid.UUID
	GameSessionID uuid.UUID
	JoinedAt      time.Time
}

// SignalRecord is a row of competition.integrity_signals (no PII: ids and counters).
type SignalRecord struct {
	SignalID   uuid.UUID
	TenantID   uuid.UUID
	SchoolID   uuid.UUID
	StudentID  uuid.UUID
	RoomID     uuid.UUID
	SignalType string
	Score      float64
	Evidence   map[string]any
	CreatedAt  time.Time
}

// RoomRef identifies an open room of any tenant (job close-idle-hub-instances).
type RoomRef struct {
	TenantID uuid.UUID
	RoomID   uuid.UUID
}

// RoomStore writes the summary records of rooms (in the transaction of ctx).
type RoomStore interface {
	CreateRoom(ctx context.Context, r RoomRecord) error
	CloseRoom(ctx context.Context, tenant, room uuid.UUID, at time.Time) (bool, error)
	OpenHubRooms(ctx context.Context, createdBefore time.Time, limit int) ([]RoomRef, error)
	OpenMembership(ctx context.Context, m MembershipRecord) error
	CloseMembership(ctx context.Context, tenant, room, student uuid.UUID, reason string, at time.Time) (bool, error)
	RecordSignal(ctx context.Context, s SignalRecord) error
}

// RoomMeta is the live description of a hub instance in the registry.
type RoomMeta struct {
	RoomID    string
	AreaID    string
	Partition hub.Partition
	Capacity  int
}

// InstanceInfo is an open instance of a partition with its seated students.
type InstanceInfo struct {
	RoomID   string
	Capacity int
	Members  []uuid.UUID
}

// UserRoom is the hub instance a student holds a seat in.
type UserRoom struct {
	RoomID string
	AreaID string
}

// Registry is the Valkey registry of live hub instances (VALKEY_DESIGN §3.1, owner competition · room).
type Registry interface {
	Instances(ctx context.Context, area string, p hub.Partition) ([]InstanceInfo, error)
	Meta(ctx context.Context, room string) (*RoomMeta, error)
	UserRoom(ctx context.Context, user uuid.UUID) (*UserRoom, error)
	// Live reports a room with a lease holder or seated members.
	Live(ctx context.Context, room string) (bool, error)
}

// JoinRequest seats a student in a room.
type JoinRequest struct {
	Seat  hub.Seat
	Spawn string
}

// PlayerState is one other player in hub.instance.assigned.
type PlayerState struct {
	EID    int              `json:"eid"`
	Player hub.PublicPlayer `json:"player"`
	State  hub.Entity       `json:"state"`
}

// JoinResult is the seat given by the room host.
type JoinResult struct {
	SelfEID  int           `json:"self_eid"`
	Spawn    hub.Vec3      `json:"spawn"`
	Players  []PlayerState `json:"players"`
	Rejoined bool          `json:"rejoined"`
}

// EmoteResult is the decision of the room host for an emote.
type EmoteResult struct {
	Owned bool          `json:"owned"`
	Wait  time.Duration `json:"wait"`
}

// Rooms is the room host: every command runs on the instance holding the lease of the room (in process or over NATS
// ew.cmd.competition.room.<room_id>.<op>, REALTIME_ARCHITECTURE §5.1). Errors: hub.ErrFull, hub.ErrNotMember,
// hub.ErrWrongPartition, ErrRoomGone.
type Rooms interface {
	Open(ctx context.Context, m RoomMeta) error
	Join(ctx context.Context, room string, r JoinRequest) (*JoinResult, error)
	Leave(ctx context.Context, room string, user uuid.UUID, reason string) error
	Emote(ctx context.Context, room string, tenant, user uuid.UUID, emote string) (*EmoteResult, error)
	Appearance(ctx context.Context, room string, user uuid.UUID, p hub.PublicPlayer) error
}

// Presence tells the gateway the area/room of a student (control presence.update, F091-AC-23) and pushes messages to
// the connections of a student (ew:rt:user:<id>).
type Presence interface {
	Update(ctx context.Context, user uuid.UUID, area, room string) error
	ToUser(ctx context.Context, user, session uuid.UUID, typ string, payload any) error
}

// Auditor records audit events (outbox audit.recorded in the transaction of ctx).
type Auditor interface {
	Record(ctx context.Context, e audit.Event) error
}

// HubMetrics observes the hub (T032).
type HubMetrics interface {
	AssignDuration(d time.Duration)
	JoinFriend(result string)
	Violation(reason string)
}

// RoomEvents is what the room host reports to the application (implemented by command.Hub).
type RoomEvents interface {
	MemberRemoved(ctx context.Context, m RoomMeta, user, school uuid.UUID, reason string) error
	RoomClosed(ctx context.Context, m RoomMeta) error
	Violation(ctx context.Context, m RoomMeta, user, school uuid.UUID, reason string, count int, sample hub.Vec3) error
}

// ProjectionWriter writes the ref.* projections (consumers only, DATABASE_DESIGN §2.17).
type ProjectionWriter interface {
	OwnerTenant(ctx context.Context, table string, id uuid.UUID) (*uuid.UUID, error)
	Tenant(ctx context.Context, tenant uuid.UUID, code *string, typ, status string, isQA *bool, settings []byte, version int64) (bool, error)
	Student(ctx context.Context, student, tenant, school uuid.UUID, grade *int, status *string, version int64) (bool, error)
	StudentStatus(ctx context.Context, student uuid.UUID, tenant *uuid.UUID, status string) (bool, error)
	StudentDeleted(ctx context.Context, student, tenant uuid.UUID, deleted bool, at time.Time) (bool, error)
	PlayerProfile(ctx context.Context, student, tenant uuid.UUID, nickname *string, level int, avatar []byte, version int64) (bool, error)
}
