package port

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// RealtimePublisher pushes server messages to the connections of a student through Valkey ew:rt:user:<id>; the
// realtime gateway (F090) delivers them and closes the connection holding the session (REALTIME_ARCHITECTURE
// §3.1). Best effort after commit: the database state is authoritative (a lost push is answered by 409 at the
// next heartbeat).
type RealtimePublisher interface {
	// SessionReplaced: session.session.replaced to the old device, which the gateway closes with 4409.
	SessionReplaced(ctx context.Context, student, session uuid.UUID, newPlatform string, at time.Time) error
	// SessionEnded: session.session.ended (curfew, daily_limit → 4410 with reopen_at; revocations → 4403).
	SessionEnded(ctx context.Context, student, session uuid.UUID, reason string, at time.Time, reopenAt *time.Time) error
	// SessionEndingSoon: session.session.ending_soon (daily_limit) before the job ends the session at endsAt (F014-AC-27).
	SessionEndingSoon(ctx context.Context, student, session uuid.UUID, reason string, endsAt time.Time) error
}

// IPHasher returns the HMAC of a client address (key from the secret store); the raw IP never leaves the
// presentation layer and is never stored or logged (ADR-0031, AGENTS R23). nil when no key is configured.
type IPHasher interface {
	Hash(ip string) []byte
}
