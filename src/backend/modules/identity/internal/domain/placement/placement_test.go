package placement

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
)

func TestMoveKinds(t *testing.T) {
	tenantA, tenantB := uuid.New(), uuid.New()
	school1, school2 := uuid.New(), uuid.New()
	cur := Placement{TenantID: tenantA, SchoolID: school1, Grade: 4, Active: true}
	cases := []struct {
		name   string
		target Placement
		kind   Kind
		err    error
	}{
		{"grade (lên khối)", Placement{tenantA, school1, 5, true}, KindGrade, nil},
		{"school same tenant (F010-AC-14)", Placement{tenantA, school2, 4, true}, KindSchool, nil},
		{"other tenant (F010-AC-15)", Placement{tenantB, uuid.New(), 4, true}, KindTenant, nil},
		{"same school and grade", Placement{tenantA, school1, 4, true}, "", ErrUnchanged},
		{"grade out of range", Placement{tenantA, school1, 10, true}, "", ErrInvalid},
		{"no school", Placement{tenantA, uuid.Nil, 4, true}, "", ErrInvalid},
	}
	for _, c := range cases {
		kind, err := Move(cur, c.target)
		if kind != c.kind || !errors.Is(err, c.err) {
			t.Errorf("%s: kind %q err %v, want %q %v", c.name, kind, err, c.kind, c.err)
		}
	}
	inactive := cur
	inactive.Active = false
	if _, err := Move(inactive, Placement{tenantA, school2, 4, true}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("a student that is not effective cannot be moved: %v", err)
	}
}

func TestChangedFields(t *testing.T) {
	a := Placement{TenantID: uuid.New(), SchoolID: uuid.New(), Grade: 3, Active: true}
	if got := Changed(nil, a); len(got) != 4 {
		t.Fatalf("first placement changes every field: %v", got)
	}
	if got := Changed(&a, a); len(got) != 0 {
		t.Fatalf("no change: %v", got)
	}
	b := a
	b.Grade, b.Active = 4, false
	if got := Changed(&a, b); !slices.Equal(got, []string{FieldGrade, FieldStatus}) {
		t.Fatalf("grade and status: %v", got)
	}
	if b.Status() != StatusInactive || a.Status() != StatusActive {
		t.Fatal("status")
	}
}
