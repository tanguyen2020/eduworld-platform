// Package ids generates and parses identifiers (F002 T002). Every new primary key and event id is a
// UUIDv7 (time-ordered, DATABASE_DESIGN §1). Idempotency-Key must also be UUIDv7 (ADR-0089 §4).
// Pure package: allowed in the domain layer.
package ids

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// New returns a new UUIDv7. It panics only if the system random source fails.
func New() uuid.UUID { return uuid.Must(uuid.NewV7()) }

// ErrNotV7 is returned by ParseV7 for a valid UUID of another version.
var ErrNotV7 = errors.New("ids: not a UUIDv7")

// ParseV7 parses s and requires version 7.
func ParseV7(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, err
	}
	if id.Version() != 7 {
		return uuid.Nil, ErrNotV7
	}
	return id, nil
}

// Time returns the millisecond timestamp embedded in a UUIDv7.
func Time(id uuid.UUID) (time.Time, bool) {
	if id.Version() != 7 {
		return time.Time{}, false
	}
	sec, nsec := id.Time().UnixTime()
	return time.Unix(sec, nsec), true
}
