package party

import (
	"slices"
	"strings"

	"github.com/google/uuid"
)

// AreaRule is what the explore eligibility knows of a world area (projection ref.world_areas of module game).
type AreaRule struct {
	ID             string
	Bands          []string
	RequiresUnlock bool
	Active         bool
}

// ExploreEligibility checks an explore activity (a private_instance of the area) for the members of a party: account
// active and no multiplayer sanction, grade band admitted by the area, area unlocked when it requires it. The reason
// carries the requirement of the area, never the grade of the student (R16). min_members 1, max_members the
// private_instance capacity; the party continues when a member disconnects.
func ExploreEligibility(area AreaRule, members []Subject, unlocked map[uuid.UUID]bool, capacity int) Eligibility {
	e := Eligibility{MinMembers: 1, MaxMembers: max(min(capacity, MaxMembers), 1), AllowContinue: true, AreaID: area.ID}
	for _, s := range members {
		c := Check{StudentID: s.ID, Eligible: true}
		switch {
		case !s.Active || s.Restricted:
			c.Eligible, c.Reason = false, ReasonSanctioned
		case s.Band == "" || !slices.Contains(area.Bands, s.Band):
			c.Eligible, c.Reason, c.Requirement = false, ReasonGradeBandMismatch, "grade_bands:"+strings.Join(area.Bands, ",")
		case area.RequiresUnlock && !unlocked[s.ID]:
			c.Eligible, c.Reason, c.Requirement = false, ReasonAreaLocked, "area:"+area.ID
		}
		e.Members = append(e.Members, c)
	}
	return e
}
