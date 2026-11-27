package social

import "github.com/google/uuid"

// Contexts of a "Vừa chơi cùng" row (FriendSuggestion.context_kind, delta F095): party (party activity), coop (co-op
// quest, boss, co-op challenge of a party), match (a match of module competition). club is reserved: no source yet.
const (
	CoplayParty = "party"
	CoplayMatch = "match"
	CoplayClub  = "club"
	CoplayCoop  = "coop"
)

// CoplayMaxAllPairs bounds a match recorded as all pairs: a larger match keeps the pairs of one team only (playing in
// the same big room is not playing together).
const CoplayMaxAllPairs = 8

// CoplayContext maps a PartyActivityType (specs/schemas/common/enums.schema.json) to the context of the row.
func CoplayContext(activityType string) string {
	switch activityType {
	case "quest", "boss", "coop_challenge":
		return CoplayCoop
	}
	return CoplayParty
}

// Coplayer is one participant of a shared activity: the student, its tenant and its team (-1: no team).
type Coplayer struct {
	StudentID uuid.UUID
	TenantID  uuid.UUID
	Team      int
}

// CoplayPair is one directed row (Student played with Other).
type CoplayPair struct{ Student, Other uuid.UUID }

// CoplayPairs returns the directed pairs of the participants of tenant: both directions of every two distinct students
// of that tenant (a participant of another tenant never pairs — cross-tenant play exists only in platform events);
// above CoplayMaxAllPairs participants only the pairs of the same team. Duplicates are removed.
func CoplayPairs(tenant uuid.UUID, ps []Coplayer) []CoplayPair {
	var in []Coplayer
	seen := map[uuid.UUID]bool{}
	for _, p := range ps {
		if p.StudentID == uuid.Nil || p.TenantID != tenant || seen[p.StudentID] {
			continue
		}
		seen[p.StudentID] = true
		in = append(in, p)
	}
	teamOnly := len(ps) > CoplayMaxAllPairs
	var out []CoplayPair
	for _, a := range in {
		for _, b := range in {
			if a.StudentID == b.StudentID || (teamOnly && (a.Team < 0 || a.Team != b.Team)) {
				continue
			}
			out = append(out, CoplayPair{Student: a.StudentID, Other: b.StudentID})
		}
	}
	return out
}
