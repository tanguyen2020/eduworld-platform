// Package catalog holds the platform catalog rules (ADR-0060): provinces with a validity period and a
// succession from former units to the current one.
package catalog

import (
	"errors"

	"github.com/google/uuid"
)

// Province status.
const (
	StatusActive   = "active"
	StatusInactive = "inactive"
)

// Province is a catalog unit.
type Province struct {
	ID          uuid.UUID
	Code        string
	CountryCode string
	Status      string
	Name        map[string]string
}

// Active reports whether the unit is in force.
func (p Province) Active() bool { return p.Status == StatusActive }

// Errors of province resolution.
var (
	ErrProvinceInactive = errors.New("catalog: province is not active")
	ErrNoSuccessor      = errors.New("catalog: former province has no active successor")
)

// Resolution is the effective province of an imported code (F010-AC-08).
type Resolution struct {
	Province Province
	Outdated bool // the code was a former unit mapped to its successor (warning province_outdated)
}

// ResolveForImport maps a province to the unit in force: an active unit is kept; a former unit is
// replaced by its successor (which must be active). successor is nil when no succession exists.
func ResolveForImport(p Province, successor *Province) (Resolution, error) {
	if p.Active() {
		return Resolution{Province: p}, nil
	}
	if successor == nil || !successor.Active() {
		return Resolution{}, ErrNoSuccessor
	}
	return Resolution{Province: *successor, Outdated: true}, nil
}

// RequireActive is the rule of school create/update by a person (F010-AC-07): only units in force.
func RequireActive(p Province) error {
	if !p.Active() {
		return ErrProvinceInactive
	}
	return nil
}
