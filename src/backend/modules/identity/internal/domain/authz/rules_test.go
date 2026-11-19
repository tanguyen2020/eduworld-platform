package authz

import (
	"errors"
	"testing"
)

func TestCheckGrant(t *testing.T) {
	ok := Grant{RoleCode: "school_admin", RoleKind: KindTenant, ScopeType: ScopeTenant, UserType: "staff", UserHasHome: true, ValidityOrder: true}
	if err := CheckGrant(ok); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		g    func(Grant) Grant
		want error
	}{
		"class scope (ADR-0072)":         {func(g Grant) Grant { g.ScopeType, g.HasScopeID = "class", true; return g }, ErrScope},
		"tenant role at platform scope":  {func(g Grant) Grant { g.ScopeType = ScopePlatform; return g }, ErrScope},
		"platform role at tenant scope":  {func(g Grant) Grant { g.RoleKind, g.UserHasHome = KindPlatform, false; return g }, ErrScope},
		"school scope without school id": {func(g Grant) Grant { g.ScopeType = ScopeSchool; return g }, ErrScopeID},
		"tenant scope with an id":        {func(g Grant) Grant { g.HasScopeID = true; return g }, ErrScopeID},
		"student":                        {func(g Grant) Grant { g.UserType = "student"; return g }, ErrStudentRole},
		"tenant role to platform staff":  {func(g Grant) Grant { g.UserHasHome = false; return g }, ErrPlatformStaff},
		"platform role to tenant staff": {func(g Grant) Grant {
			g.RoleKind, g.ScopeType = KindPlatform, ScopePlatform
			return g
		}, ErrTenantStaff},
		"safety_moderator without training": {func(g Grant) Grant {
			g.RoleCode, g.RoleKind, g.ScopeType, g.UserHasHome = "safety_moderator", KindPlatform, ScopePlatform, false
			return g
		}, ErrTrainingMissing},
		"validity": {func(g Grant) Grant { g.ValidityOrder = false; return g }, ErrValidity},
	}
	for name, c := range cases {
		if err := CheckGrant(c.g(ok)); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", name, err, c.want)
		}
	}
	sm := Grant{RoleCode: "safety_moderator", RoleKind: KindPlatform, ScopeType: ScopePlatform, UserType: "staff", TrainingDone: true, ValidityOrder: true}
	if err := CheckGrant(sm); err != nil {
		t.Fatalf("trained safety_moderator: %v", err)
	}
}

func TestExceeds(t *testing.T) {
	if got := Exceeds([]string{"a", "b"}, map[string]bool{"a": true}); len(got) != 1 || got[0] != "b" {
		t.Fatalf("exceeds = %v", got)
	}
}
