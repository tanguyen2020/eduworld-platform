// Package write holds the write-context ports of module game (ADR-0087): repositories load and save inside
// entx.UnitOfWork; events and audit go to the outbox of the same transaction.
package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gamesession"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// UnitOfWork runs fn in one write transaction (entx.UnitOfWork); InTenantTx uses an explicit scope (jobs).
type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
	InTenantTx(ctx context.Context, s tenancy.Scope, fn func(ctx context.Context) error) error
}

// AfterCommit registers fn to run after the commit of the transaction of ctx (realtime push, best effort).
type AfterCommit func(ctx context.Context, fn func(ctx context.Context)) error

// Event is a domain event written to the outbox. TenantID nil takes the session tenant of the scope.
type Event struct {
	Type         string
	SubjectType  string
	SubjectID    string
	PartitionKey string
	TenantID     *uuid.UUID
	Platform     bool // platform event (envelope tenant_id null): game content releases
	OccurredAt   time.Time
	Payload      any
}

// EventPublisher writes events into the outbox of the transaction.
type EventPublisher interface {
	Publish(ctx context.Context, e Event) error
}

// Auditor records audit events in the transaction (audit.recorded via the outbox, DATABASE_DESIGN §1.3).
type Auditor interface {
	Record(ctx context.Context, e audit.Event) error
}

// SessionRef identifies an active session found by a platform job.
type SessionRef struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	StudentID uuid.UUID
	SchoolID  *uuid.UUID
}

// SessionRepository persists the GameSession aggregate (game.game_sessions). Tenant methods run under the
// TenantScope of the transaction with explicit tenant and deleted_at conditions (AGENTS R27).
type SessionRepository interface {
	// LockStudent serializes the session starts of one student (pg_advisory_xact_lock, DATABASE_DESIGN §3.12).
	LockStudent(ctx context.Context, student uuid.UUID) error
	// Active returns the active session of the student in tenant (locked FOR UPDATE), nil when none.
	Active(ctx context.Context, tenant, student uuid.UUID) (*gamesession.Session, error)
	// ByID returns a session of tenant (locked FOR UPDATE), nil when unknown.
	ByID(ctx context.Context, tenant, id uuid.UUID) (*gamesession.Session, error)
	// Create inserts a new active session (a concurrent active session of the student is 409).
	Create(ctx context.Context, s *gamesession.Session) error
	// Save writes the heartbeat, the end and the replacing session (optimistic version check).
	Save(ctx context.Context, s *gamesession.Session) error
	// ActiveOfStudent returns the active sessions of a student (locked): in tenant, or in every tenant when
	// tenant is nil (platform scope of a platform event).
	ActiveOfStudent(ctx context.Context, student uuid.UUID, tenant *uuid.UUID) ([]*gamesession.Session, error)
	// Stale lists active sessions whose last heartbeat is before before (platform scope, job session-timeout).
	Stale(ctx context.Context, before time.Time, limit int) ([]SessionRef, error)
	// ActiveByPlatform counts the active sessions per client platform (platform scope, gauge eduworld_game_sessions_active).
	ActiveByPlatform(ctx context.Context) (map[string]int, error)
	// PlayedSince returns the time the student played in tenant from since to now: every session (ended or active)
	// clipped to [since, now] (daily play limit, OQ-17, F014-AC-27).
	PlayedSince(ctx context.Context, tenant, student uuid.UUID, since, now time.Time) (time.Duration, error)
	// ActivePage lists active sessions after the session id after (platform scope, job curfew-session-closer).
	ActivePage(ctx context.Context, after uuid.UUID, limit int) ([]SessionRef, error)
}

// LoginEvent is one row of game.login_events (no raw IP, ADR-0031).
type LoginEvent struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	StudentID       uuid.UUID
	GameSessionID   uuid.UUID
	DeviceID        uuid.UUID
	Platform        string
	DeviceModel     *string
	ClientVersion   string
	IPHash          []byte
	LoginAt         time.Time
	SISConnectionID *uuid.UUID
	// GradeLevel is the grade of the student (ref.students.grade_level, nil when unknown); only its band leaves the module
	// (realtime ticket label of eduworld_players_online, MONITORING §3).
	GradeLevel *int
}

// LoginEventRepository appends login events and records their end once.
type LoginEventRepository interface {
	Create(ctx context.Context, e LoginEvent) error
	// Close sets logout_at and end_reason of the login event of session (once; a closed event is left as is).
	Close(ctx context.Context, tenant, session uuid.UUID, at time.Time, reason string) error
}

// Student is the projected state of a student (ref.students).
type Student struct {
	StudentID uuid.UUID
	TenantID  uuid.UUID
	SchoolID  uuid.UUID
	Status    string
	Deleted   bool
	// SISConnectionID is the SIS connection of the student (OQ F014-1), nil when unknown or not synced from a SIS.
	SISConnectionID *uuid.UUID
	// GradeLevel is the grade of the student (ref.students.grade_level, nil when unknown); only its band leaves the module
	// (label of eduworld_players_online in the realtime ticket, MONITORING §3).
	GradeLevel *int
}

// Playable reports a student who may enter the game (active, not cleaned up).
func (s Student) Playable() bool { return s.Status == "active" && !s.Deleted }

// CurfewWindow is a curfew window ({start, end} "HH:MM" ICT): of a school (ref.schools.settings_subset) or of the
// platform (platform_curfew in ref.platform_settings).
type CurfewWindow struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// Directory reads the identity projections inside the write transaction (ref.students, ref.schools,
// ref.platform_settings).
type Directory interface {
	// Student returns the projected student of the transaction tenant, nil when not projected yet.
	Student(ctx context.Context, tenant, student uuid.UUID) (*Student, error)
	// SchoolCurfew returns the curfew window of a school, nil when it has none.
	SchoolCurfew(ctx context.Context, tenant, school uuid.UUID) (*CurfewWindow, error)
	// PlatformCurfew returns platform_curfew projected from platform_settings.changed, nil before the first event
	// (the caller then keeps its start-up default EW_PLATFORM_CURFEW).
	PlatformCurfew(ctx context.Context) (*CurfewWindow, error)
	// DailyPlayLimit returns daily_play_limit_minutes of a school (school.updated settings_subset, OQ-17), nil when
	// off, unknown or the tenant is the common tenant (never limited).
	DailyPlayLimit(ctx context.Context, tenant, school uuid.UUID) (*int, error)
}
