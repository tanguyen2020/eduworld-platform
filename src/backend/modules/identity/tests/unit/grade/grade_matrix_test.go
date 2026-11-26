// Package grade holds the grade matrix 1–9 of module identity (TEST_STRATEGY §6.10).
package grade

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/placement"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/user"
)

var grades = []int{1, 2, 3, 4, 5, 6, 7, 8, 9}

// TestGradeMatrixPlacement (F010, ADR-0095, FR-ORG-017): every grade 1–9 is a valid placement and 0 / 10 are not; a
// move to the next grade in the same school is a grade move (lên khối, the school-year rollover) whose changed fields
// are exactly grade_level — the field the game consumer game.grade_transition reacts to (F074-AC-25).
func TestGradeMatrixPlacement(t *testing.T) {
	tenant, school := uuid.New(), uuid.New()
	for _, g := range grades {
		p := placement.Placement{TenantID: tenant, SchoolID: school, Grade: g, Active: true}
		if !placement.ValidGrade(g) || p.Validate() != nil || user.CheckGrade(g) != nil {
			t.Errorf("grade %d refused", g)
		}
		if g == 9 {
			continue
		}
		next := p
		next.Grade = g + 1
		kind, err := placement.Move(p, next)
		if err != nil || kind != placement.KindGrade {
			t.Errorf("grade %d → %d: %s %v", g, g+1, kind, err)
		}
		if got := placement.Changed(&p, next); !slices.Equal(got, []string{placement.FieldGrade}) {
			t.Errorf("grade %d → %d changed %v", g, g+1, got)
		}
		if _, err := placement.Move(p, p); !errors.Is(err, placement.ErrUnchanged) {
			t.Errorf("grade %d unchanged: %v", g, err)
		}
	}
	for _, g := range []int{0, 10, -1} {
		if placement.ValidGrade(g) || user.CheckGrade(g) == nil {
			t.Errorf("grade %d accepted", g)
		}
		if _, err := placement.Move(placement.Placement{TenantID: tenant, SchoolID: school, Grade: 9, Active: true},
			placement.Placement{TenantID: tenant, SchoolID: school, Grade: g, Active: true}); !errors.Is(err, placement.ErrInvalid) {
			t.Errorf("move to grade %d: %v", g, err)
		}
	}
}

// TestGradeMatrixTemporaryPassword (ACCOUNT_PROVISIONING §3.4, §6c [Confirmed]): the initial or reset password of a
// student is temporary (changed at first login) for grades 6–9 and not for grades 1–5 (printed login card).
func TestGradeMatrixTemporaryPassword(t *testing.T) {
	for _, g := range grades {
		if user.TemporaryPassword(g) != (g >= 6) {
			t.Errorf("grade %d temporary %v", g, user.TemporaryPassword(g))
		}
	}
}
