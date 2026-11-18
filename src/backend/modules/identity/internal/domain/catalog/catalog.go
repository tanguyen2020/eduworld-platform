// Package catalog holds the platform catalog rules (ADR-0060): provinces with a validity period and a
// succession from former units to the current one.
package catalog

import (
	"errors"
	"strings"
	"time"

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

// ErrInvalid is a rule violation on one catalog field (422 VALIDATION_FAILED).
type ErrInvalid struct{ Field, Reason string }

func (e ErrInvalid) Error() string { return "catalog: " + e.Field + " " + e.Reason }

// Country is a catalog country (ISO 3166-1 alpha-2).
type Country struct {
	Code          string
	Name          map[string]string
	DefaultLocale string
	Status        string
	Version       int64
}

// ProvinceRecord is the full catalog province (create/update by System Admin).
type ProvinceRecord struct {
	Province
	OfficialCode  *string
	ShortName     map[string]string
	ProvinceType  string
	EffectiveFrom *time.Time
	EffectiveTo   *time.Time
	Version       int64
}

// Succession links a former unit to its successor.
type Succession struct {
	OldID, NewID  uuid.UUID
	EffectiveDate time.Time
	Kind          string
}

func checkI18n(field string, m map[string]string) error {
	if strings.TrimSpace(m["vi"]) == "" || strings.TrimSpace(m["en"]) == "" {
		return ErrInvalid{Field: field, Reason: "needs vi and en"}
	}
	return nil
}

// Validate checks a country.
func (c Country) Validate() error {
	if len(c.Code) != 2 || strings.ToUpper(c.Code) != c.Code {
		return ErrInvalid{Field: "country_code", Reason: "must be ISO 3166-1 alpha-2"}
	}
	if err := checkI18n("name", c.Name); err != nil {
		return err
	}
	if c.Status != StatusActive && c.Status != StatusInactive {
		return ErrInvalid{Field: "status", Reason: "must be active or inactive"}
	}
	return nil
}

// Validate checks a province: names, type, and inactive ⇔ effective_to set (ADR-0060).
func (p ProvinceRecord) Validate() error {
	if err := checkI18n("name", p.Name); err != nil {
		return err
	}
	if err := checkI18n("short_name", p.ShortName); err != nil {
		return err
	}
	if p.ProvinceType != "municipality" && p.ProvinceType != "province" {
		return ErrInvalid{Field: "province_type", Reason: "must be municipality or province"}
	}
	if p.Status != StatusActive && p.Status != StatusInactive {
		return ErrInvalid{Field: "status", Reason: "must be active or inactive"}
	}
	if (p.Status == StatusInactive) != (p.EffectiveTo != nil) {
		return ErrInvalid{Field: "effective_to", Reason: "set exactly when the unit is inactive"}
	}
	if p.EffectiveFrom != nil && p.EffectiveTo != nil && p.EffectiveTo.Before(*p.EffectiveFrom) {
		return ErrInvalid{Field: "effective_to", Reason: "before effective_from"}
	}
	return nil
}

// NewSuccession checks a succession: distinct units, successor in force.
func NewSuccession(old, next Province, on time.Time, kind string) (Succession, error) {
	switch kind {
	case "merged", "unchanged", "renamed", "split":
	default:
		return Succession{}, ErrInvalid{Field: "kind", Reason: "must be merged, unchanged, renamed or split"}
	}
	if old.ID == next.ID {
		return Succession{}, ErrInvalid{Field: "new_province_id", Reason: "must differ from the former unit"}
	}
	if !next.Active() {
		return Succession{}, ErrInvalid{Field: "new_province_id", Reason: "the successor must be in force"}
	}
	return Succession{OldID: old.ID, NewID: next.ID, EffectiveDate: on, Kind: kind}, nil
}
