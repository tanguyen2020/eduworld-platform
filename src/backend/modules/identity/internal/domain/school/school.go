// Package school holds the School aggregate (DATABASE_DESIGN §2.2, ADR-0060): one tenant, one province
// in force, grades 1–9; schools without a province wait in pending_review until a Tenant Admin approves.
package school

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/tenant"
)

// Status of a school (contract School.status).
type Status string

const (
	StatusActive        Status = "active"
	StatusPendingReview Status = "pending_review"
	StatusDisabled      Status = "disabled"
)

// Level of a school.
type Level string

const (
	LevelPrimary        Level = "primary"
	LevelLowerSecondary Level = "lower_secondary"
	LevelMultiLevel     Level = "multi_level"
)

// ErrInvalid is a rule violation on one field (422 VALIDATION_FAILED).
type ErrInvalid struct{ Field, Reason string }

func (e ErrInvalid) Error() string { return "school: " + e.Field + " " + e.Reason }

// Errors of state changes.
var (
	ErrNotPending = errors.New("school: not pending_review") // 422 INVALID_STATE_TRANSITION
	ErrNoChange   = errors.New("school: no change")
	ErrVirtual    = errors.New("school: the virtual school of the common tenant cannot be changed")
)

// Name is the localized name entered by the tenant (vi required, en optional).
type Name struct{ Vi, En string }

// School is the aggregate.
type School struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Code        string
	Name        Name
	CountryCode string
	Province    *catalog.Province
	MoetCode    *string
	Level       Level
	GradeMin    int
	GradeMax    int
	Address     *string
	Status      Status
	IsVirtual   bool
	Version     int64
}

func checkName(n Name) error {
	if l := utf8.RuneCountInString(strings.TrimSpace(n.Vi)); l < 1 || l > 255 {
		return ErrInvalid{Field: "name.vi", Reason: "must be 1–255 characters"}
	}
	if utf8.RuneCountInString(n.En) > 255 {
		return ErrInvalid{Field: "name.en", Reason: "must be at most 255 characters"}
	}
	return nil
}

func checkLevel(l Level) error {
	switch l {
	case LevelPrimary, LevelLowerSecondary, LevelMultiLevel:
		return nil
	}
	return ErrInvalid{Field: "school_level", Reason: "must be primary, lower_secondary or multi_level"}
}

func checkGrades(lo, hi int) error {
	if lo < 1 || lo > 9 || hi < 1 || hi > 9 || lo > hi {
		return ErrInvalid{Field: "grade_min", Reason: "grades must be 1–9 with grade_min ≤ grade_max"}
	}
	return nil
}

// CanAdd enforces the tenant shape (DATABASE_DESIGN §2.2): a school tenant has exactly one school and
// the common tenant only its virtual school (seeded).
func CanAdd(t tenant.Type, existing int) error {
	switch t {
	case tenant.TypeSchool:
		if existing > 0 {
			return ErrInvalid{Field: "tenant_id", Reason: "a school tenant has exactly one school"}
		}
	case tenant.TypeCommon:
		return ErrInvalid{Field: "tenant_id", Reason: "the common tenant only has its virtual school"}
	}
	return nil
}

// New creates an active school chosen by a person: the province must be in force (F010-AC-07).
func New(id, tenantID uuid.UUID, code string, name Name, prov catalog.Province, level Level, gradeMin, gradeMax int,
	moet, address *string) (*School, error) {
	code = strings.TrimSpace(code)
	if l := utf8.RuneCountInString(code); l < 1 || l > 64 {
		return nil, ErrInvalid{Field: "code", Reason: "must be 1–64 characters"}
	}
	if err := checkName(name); err != nil {
		return nil, err
	}
	if err := catalog.RequireActive(prov); err != nil {
		return nil, ErrInvalid{Field: "province_code", Reason: "must be a province in force"}
	}
	if err := checkLevel(level); err != nil {
		return nil, err
	}
	if err := checkGrades(gradeMin, gradeMax); err != nil {
		return nil, err
	}
	p := prov
	return &School{ID: id, TenantID: tenantID, Code: code, Name: name, CountryCode: prov.CountryCode, Province: &p,
		MoetCode: blankNil(moet), Level: level, GradeMin: gradeMin, GradeMax: gradeMax, Address: blankNil(address),
		Status: StatusActive, Version: 1}, nil
}

// NewImported creates a school from an import (F010-AC-08): the province is the resolved unit in force;
// without a province the school waits in pending_review.
func NewImported(id, tenantID uuid.UUID, code string, name Name, countryCode string, prov *catalog.Province, level Level,
	gradeMin, gradeMax int, moet, address *string) (*School, error) {
	if prov != nil {
		return New(id, tenantID, code, name, *prov, level, gradeMin, gradeMax, moet, address)
	}
	s, err := New(id, tenantID, code, name, catalog.Province{CountryCode: countryCode, Status: catalog.StatusActive}, level,
		gradeMin, gradeMax, moet, address)
	if err != nil {
		return nil, err
	}
	s.Province, s.Status = nil, StatusPendingReview
	return s, nil
}

func blankNil(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	v := strings.TrimSpace(*s)
	return &v
}

// Patch is a merge-patch (SchoolUpdateRequest / AdminSchoolUpdate). Province is the resolved province of
// province_code; Status is accepted only from System Admin (AdminSchoolUpdate).
type Patch struct {
	Name     *Name
	Province *catalog.Province
	MoetCode **string // set to a nil pointer to clear
	Level    *Level
	GradeMin *int
	GradeMax *int
	Address  **string
	Status   *Status
}

// Apply applies p (ErrNoChange when nothing changes).
func (s *School) Apply(p Patch) error {
	if s.IsVirtual {
		return ErrVirtual
	}
	changed := false
	if p.Name != nil {
		if err := checkName(*p.Name); err != nil {
			return err
		}
		if *p.Name != s.Name {
			s.Name, changed = *p.Name, true
		}
	}
	if p.Province != nil {
		if err := catalog.RequireActive(*p.Province); err != nil {
			return ErrInvalid{Field: "province_code", Reason: "must be a province in force"}
		}
		if s.Province == nil || s.Province.ID != p.Province.ID {
			v := *p.Province
			s.Province, s.CountryCode, changed = &v, v.CountryCode, true
		}
	}
	if p.MoetCode != nil {
		if v := blankNil(*p.MoetCode); !eq(v, s.MoetCode) {
			s.MoetCode, changed = v, true
		}
	}
	if p.Level != nil {
		if err := checkLevel(*p.Level); err != nil {
			return err
		}
		if *p.Level != s.Level {
			s.Level, changed = *p.Level, true
		}
	}
	lo, hi := s.GradeMin, s.GradeMax
	if p.GradeMin != nil {
		lo = *p.GradeMin
	}
	if p.GradeMax != nil {
		hi = *p.GradeMax
	}
	if lo != s.GradeMin || hi != s.GradeMax {
		if err := checkGrades(lo, hi); err != nil {
			return err
		}
		s.GradeMin, s.GradeMax, changed = lo, hi, true
	}
	if p.Address != nil {
		if v := blankNil(*p.Address); !eq(v, s.Address) {
			s.Address, changed = v, true
		}
	}
	if p.Status != nil {
		if *p.Status != StatusActive && *p.Status != StatusDisabled {
			return ErrInvalid{Field: "status", Reason: "must be active or disabled"}
		}
		if *p.Status == StatusActive && s.Province == nil {
			return ErrInvalid{Field: "status", Reason: "a school without province must be approved first"}
		}
		if *p.Status != s.Status {
			s.Status, changed = *p.Status, true
		}
	}
	if !changed {
		return ErrNoChange
	}
	return nil
}

// Approve moves a pending_review school to active (F010-AC-09); its province must be in force.
func (s *School) Approve() error {
	if s.Status != StatusPendingReview {
		return ErrNotPending
	}
	if s.Province == nil || !s.Province.Active() {
		return ErrInvalid{Field: "province_code", Reason: "choose a province in force before approving"}
	}
	s.Status = StatusActive
	return nil
}

func eq(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
