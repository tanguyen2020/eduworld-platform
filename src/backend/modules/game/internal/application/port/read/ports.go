// Package read holds the read-context ports of module game (ADR-0087): read models only read, in a READ ONLY
// session (replica allowed), never write, never publish events.
package read

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Session runs fn in a READ ONLY transaction with the scope of ctx (entx.ReadSession).
type Session interface {
	Read(ctx context.Context, fn func(ctx context.Context) error) error
}

// Manifest is the latest content manifest of a tenant (game.content_manifests).
type Manifest struct {
	TenantID    uuid.UUID
	Version     int64
	ETag        string
	Entries     json.RawMessage // JSON object: items, bundles, areas (ADR-0043)
	GeneratedAt time.Time
}

// ManifestReadModel reads game.content_manifests.
type ManifestReadModel interface {
	// Latest returns the latest manifest of tenant, nil when the tenant has no published content yet.
	Latest(ctx context.Context, tenant uuid.UUID) (*Manifest, error)
}

// LoginEvent is one login of the history (never device_id, ip_hash or an IP — F014-AC-11).
type LoginEvent struct {
	LoginEventID  uuid.UUID
	TenantID      uuid.UUID
	StudentID     uuid.UUID
	GameSessionID uuid.UUID
	Platform      string
	DeviceModel   *string
	ClientVersion *string
	LoginAt       time.Time
	LogoutAt      *time.Time
	EndReason     *string
}

// LoginFilter selects login events: tenants (one for a tenant user, the CrossTenantScope ids for System Admin),
// optionally one student and a time range, keyset after (login_at, id).
type LoginFilter struct {
	TenantIDs []uuid.UUID
	StudentID *uuid.UUID
	From, To  *time.Time
	AfterAt   *time.Time
	AfterID   uuid.UUID
	Limit     int
}

// LoginEventReadModel reads game.login_events.
type LoginEventReadModel interface {
	// List returns up to Limit+1 events ordered by login_at DESC, id DESC.
	List(ctx context.Context, f LoginFilter) ([]LoginEvent, error)
}

// StudentRef is the projected school of a student (ref.students).
type StudentRef struct {
	StudentID uuid.UUID
	TenantID  uuid.UUID
	SchoolID  uuid.UUID
}

// StudentReadModel reads ref.students.
type StudentReadModel interface {
	// Student returns the projected student of tenant (nil when unknown in it).
	Student(ctx context.Context, tenant, student uuid.UUID) (*StudentRef, error)
}
