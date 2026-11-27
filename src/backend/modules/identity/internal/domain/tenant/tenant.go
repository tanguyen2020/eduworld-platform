// Package tenant holds the Tenant aggregate (ADR-0005, ADR-0026, KEYCLOAK_SSO §2.1): the unit of data
// isolation and login, mapped to exactly one Keycloak Organization.
package tenant

import (
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Type of tenant.
type Type string

const (
	TypeSchool Type = "school" // independent school: exactly one School
	TypeUnit   Type = "unit"   // unit of several schools (e.g. district office)
	TypeCommon Type = "common" // the platform common tenant (only one, ADR-0063)
)

// Status of a tenant.
type Status string

const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
)

// Fixed identifiers of the common tenant, seeded by migration 20261119000002_f010_seed.sql
// (scripts/db/gen-identity-seed.mjs). A student of the common tenant belongs to the virtual school with its grade
// (ADR-0095: no virtual class).
var (
	CommonTenantID       = uuid.MustParse("0193a000-0000-7000-8000-000000000001")
	CommonSchoolID       = uuid.MustParse("0193a000-0000-7000-8000-000000000002")
	CommonAcademicYearID = uuid.MustParse("0193a000-0000-7000-8000-000000000003")
)

// ErrInvalid is a rule violation on one field (mapped to 422 VALIDATION_FAILED).
type ErrInvalid struct{ Field, Reason string }

func (e ErrInvalid) Error() string { return "tenant: " + e.Field + " " + e.Reason }

// ErrNoChange: an update that changes nothing.
var ErrNoChange = errors.New("tenant: no change")

var aliasRe = regexp.MustCompile(`^[a-z0-9-]{3,63}$`)

// Tenant is the aggregate.
type Tenant struct {
	ID            uuid.UUID
	Code          string
	Name          string
	Type          Type
	Status        Status
	OrgAlias      string
	OrgID         *uuid.UUID
	DefaultLocale string
	IsQA          bool // QA tenant (synthetic monitoring, F131): excluded from analytics (F134)
	Version       int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ParseType validates a tenant type.
func ParseType(s string) (Type, error) {
	switch Type(s) {
	case TypeSchool, TypeUnit, TypeCommon:
		return Type(s), nil
	}
	return "", ErrInvalid{Field: "tenant_type", Reason: "must be school, unit or common"}
}

// ParseStatus validates a tenant status.
func ParseStatus(s string) (Status, error) {
	switch Status(s) {
	case StatusActive, StatusDisabled:
		return Status(s), nil
	}
	return "", ErrInvalid{Field: "status", Reason: "must be active or disabled"}
}

func checkName(name string) error {
	if n := utf8.RuneCountInString(strings.TrimSpace(name)); n < 1 || n > 255 {
		return ErrInvalid{Field: "name", Reason: "must be 1–255 characters"}
	}
	return nil
}

// New creates an active tenant. The common tenant exists once (seeded); uniqueness of code, alias and
// the single common tenant are enforced again by unique indexes (409 CONFLICT).
func New(id uuid.UUID, code, name string, typ Type, alias, locale string) (*Tenant, error) {
	code = strings.TrimSpace(code)
	if n := utf8.RuneCountInString(code); n < 2 || n > 64 {
		return nil, ErrInvalid{Field: "code", Reason: "must be 2–64 characters"}
	}
	if err := checkName(name); err != nil {
		return nil, err
	}
	if _, err := ParseType(string(typ)); err != nil {
		return nil, err
	}
	if !aliasRe.MatchString(alias) {
		return nil, ErrInvalid{Field: "keycloak_org_alias", Reason: "must match ^[a-z0-9-]{3,63}$"}
	}
	if locale == "" {
		locale = "vi-VN"
	}
	return &Tenant{ID: id, Code: code, Name: strings.TrimSpace(name), Type: typ, Status: StatusActive,
		OrgAlias: alias, DefaultLocale: locale, Version: 1}, nil
}

// Patch is a merge-patch of the mutable fields (TenantUpdate); nil = unchanged.
type Patch struct {
	Name   *string
	Status *Status
	IsQA   *bool
}

// Change describes what an applied patch changed (for tenant.updated and the Organization sync).
type Change struct {
	NameChanged   bool
	StatusChanged bool
	QAChanged     bool
}

// Apply applies p. It returns ErrNoChange when nothing changes.
func (t *Tenant) Apply(p Patch) (Change, error) {
	var c Change
	if p.Name != nil {
		if err := checkName(*p.Name); err != nil {
			return c, err
		}
		if n := strings.TrimSpace(*p.Name); n != t.Name {
			t.Name, c.NameChanged = n, true
		}
	}
	if p.Status != nil {
		if _, err := ParseStatus(string(*p.Status)); err != nil {
			return c, err
		}
		if *p.Status != t.Status {
			t.Status, c.StatusChanged = *p.Status, true
		}
	}
	if p.IsQA != nil && *p.IsQA != t.IsQA {
		if err := t.SetQA(*p.IsQA); err != nil {
			return c, err
		}
		c.QAChanged = true
	}
	if !c.NameChanged && !c.StatusChanged && !c.QAChanged {
		return c, ErrNoChange
	}
	return c, nil
}

// SetQA flags or unflags the tenant as a QA tenant (F134-AC-07, TEST_STRATEGY §4). The common tenant can never be
// a QA tenant (CHECK tenants_qa_chk): its students are real.
func (t *Tenant) SetQA(qa bool) error {
	if qa && t.Type == TypeCommon {
		return ErrInvalid{Field: "is_qa", Reason: "the common tenant cannot be a QA tenant"}
	}
	t.IsQA = qa
	return nil
}

// OrgAttributes are the Keycloak Organization attributes of the tenant (KEYCLOAK_SSO §2.1).
func (t *Tenant) OrgAttributes() map[string][]string {
	return map[string][]string{"tenant_id": {t.ID.String()}, "tenant_type": {string(t.Type)}, "display_name": {t.Name}}
}
