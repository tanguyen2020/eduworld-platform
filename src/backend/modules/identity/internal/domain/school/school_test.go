package school

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/tenant"
)

var (
	hanoi  = catalog.Province{ID: uuid.New(), Code: "vn.ha-noi", CountryCode: "VN", Status: catalog.StatusActive}
	oldHaG = catalog.Province{ID: uuid.New(), Code: "vn.old.ha-giang", CountryCode: "VN", Status: catalog.StatusInactive}
)

func field(err error) string {
	var inv ErrInvalid
	if errors.As(err, &inv) {
		return inv.Field
	}
	return ""
}

// F010-AC-07: a person cannot choose a former province.
func TestNewRequiresActiveProvince(t *testing.T) {
	s, err := New(uuid.New(), uuid.New(), "th-a", Name{Vi: "TH A"}, hanoi, LevelPrimary, 1, 5, nil, nil)
	if err != nil || s.Status != StatusActive || s.Province.Code != "vn.ha-noi" {
		t.Fatalf("new = %+v %v", s, err)
	}
	if _, err := New(uuid.New(), uuid.New(), "th-b", Name{Vi: "TH B"}, oldHaG, LevelPrimary, 1, 5, nil, nil); field(err) != "province_code" {
		t.Fatalf("inactive province: %v", err)
	}
	for name, c := range map[string]struct {
		code   string
		n      Name
		level  Level
		lo, hi int
		want   string
	}{
		"empty code":   {"", Name{Vi: "x"}, LevelPrimary, 1, 5, "code"},
		"empty name":   {"c", Name{Vi: " "}, LevelPrimary, 1, 5, "name.vi"},
		"bad level":    {"c", Name{Vi: "x"}, Level("upper"), 1, 5, "school_level"},
		"grades order": {"c", Name{Vi: "x"}, LevelPrimary, 6, 5, "grade_min"},
		"grade 10":     {"c", Name{Vi: "x"}, LevelPrimary, 1, 10, "grade_min"},
	} {
		if _, err := New(uuid.New(), uuid.New(), c.code, c.n, hanoi, c.level, c.lo, c.hi, nil, nil); field(err) != c.want {
			t.Errorf("%s: %v, want field %s", name, err, c.want)
		}
	}
}

// F010-AC-08 / AC-09: import without province waits; approval needs a province in force.
func TestImportedAndApprove(t *testing.T) {
	s, err := NewImported(uuid.New(), uuid.New(), "th-c", Name{Vi: "TH C"}, "VN", nil, LevelPrimary, 1, 5, nil, nil)
	if err != nil || s.Status != StatusPendingReview || s.Province != nil {
		t.Fatalf("imported = %+v %v", s, err)
	}
	if field(s.Approve()) != "province_code" {
		t.Fatal("approval without province accepted")
	}
	active := StatusActive
	if field(s.Apply(Patch{Status: &active})) != "status" {
		t.Fatal("activating a school without province via patch accepted")
	}
	if err := s.Apply(Patch{Province: &hanoi}); err != nil {
		t.Fatal(err)
	}
	if err := s.Approve(); err != nil || s.Status != StatusActive {
		t.Fatalf("approve: %v %s", err, s.Status)
	}
	if !errors.Is(s.Approve(), ErrNotPending) {
		t.Fatal("approving an active school must fail")
	}
}

func TestApply(t *testing.T) {
	s, _ := New(uuid.New(), uuid.New(), "th-a", Name{Vi: "TH A"}, hanoi, LevelPrimary, 1, 5, nil, nil)
	if !errors.Is(s.Apply(Patch{Name: &Name{Vi: "TH A"}}), ErrNoChange) {
		t.Fatal("same name must be no change")
	}
	addr := "  12 Phố Huế "
	ap := &addr
	if err := s.Apply(Patch{Address: &ap}); err != nil || *s.Address != "12 Phố Huế" {
		t.Fatalf("address: %v", err)
	}
	var none *string
	if err := s.Apply(Patch{Address: &none}); err != nil || s.Address != nil {
		t.Fatalf("clear address: %v", err)
	}
	nine := 9
	if err := s.Apply(Patch{GradeMax: &nine}); err != nil || s.GradeMax != 9 {
		t.Fatalf("grade max: %v", err)
	}
	if field(s.Apply(Patch{Province: &oldHaG})) != "province_code" {
		t.Fatal("former province accepted on update")
	}
	disabled := StatusDisabled
	if err := s.Apply(Patch{Status: &disabled}); err != nil || s.Status != StatusDisabled {
		t.Fatalf("disable: %v", err)
	}
	v := &School{IsVirtual: true}
	if !errors.Is(v.Apply(Patch{Name: &Name{Vi: "x"}}), ErrVirtual) {
		t.Fatal("virtual school must not change")
	}
}

func TestCanAdd(t *testing.T) {
	if err := CanAdd(tenant.TypeSchool, 0); err != nil {
		t.Fatal(err)
	}
	if field(CanAdd(tenant.TypeSchool, 1)) != "tenant_id" {
		t.Fatal("second school in a school tenant accepted")
	}
	if field(CanAdd(tenant.TypeCommon, 1)) != "tenant_id" {
		t.Fatal("school in the common tenant accepted")
	}
	if err := CanAdd(tenant.TypeUnit, 7); err != nil {
		t.Fatal(err)
	}
}

func TestResolveForImport(t *testing.T) {
	r, err := catalog.ResolveForImport(oldHaG, &hanoi)
	if err != nil || !r.Outdated || r.Province.Code != "vn.ha-noi" {
		t.Fatalf("resolve former = %+v %v", r, err)
	}
	if r, err := catalog.ResolveForImport(hanoi, nil); err != nil || r.Outdated {
		t.Fatalf("resolve active = %+v %v", r, err)
	}
	if _, err := catalog.ResolveForImport(oldHaG, nil); !errors.Is(err, catalog.ErrNoSuccessor) {
		t.Fatalf("no successor = %v", err)
	}
}
