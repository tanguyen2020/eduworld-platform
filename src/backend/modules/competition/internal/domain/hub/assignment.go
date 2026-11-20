package hub

// Candidate is an open instance of the joining student's partition and area.
type Candidate struct {
	RoomID   string
	Members  int  // members holding a seat (held seats included)
	Capacity int  // effective capacity of the instance
	Friends  int  // online friends of the joining student in the instance
	Blocked  bool // someone in the instance blocked the student or was blocked by them
}

// Choice is the outcome of the assignment policy: an existing instance, or a new one when RoomID is empty.
type Choice struct {
	RoomID string
}

// New reports that a new instance must be created.
func (c Choice) New() bool { return c.RoomID == "" }

// crowdLimit is the "most crowded below 90 %" rule (MULTIPLAYER_GAMEPLAY §6, F091-AC-03).
const crowdLimit = 0.9

// Choose applies MULTIPLAYER_GAMEPLAY §6 to the candidates of one partition (party rules belong to F100/F101):
//  1. the instance with the most online friends that still has a seat;
//  2. otherwise the most crowded instance below 90 % of its capacity;
//  3. otherwise a new instance.
//
// An instance with a blocked/blocking student is avoided: creating a new instance is always another choice in a hub,
// so such an instance is never chosen (soft constraint of SOCIAL_SYSTEM §5 met whenever possible). Candidates must
// all belong to the student's partition; the room host checks it again (F091-AC-02).
func Choose(cands []Candidate) Choice {
	best := -1
	for i, c := range cands {
		if c.Blocked || c.Members >= c.Capacity || c.Friends == 0 {
			continue
		}
		if best < 0 || c.Friends > cands[best].Friends || (c.Friends == cands[best].Friends && c.Members > cands[best].Members) {
			best = i
		}
	}
	if best >= 0 {
		return Choice{RoomID: cands[best].RoomID}
	}
	for i, c := range cands {
		if c.Blocked || c.Members >= c.Capacity || float64(c.Members) >= crowdLimit*float64(c.Capacity) {
			continue
		}
		if best < 0 || c.Members > cands[best].Members {
			best = i
		}
	}
	if best >= 0 {
		return Choice{RoomID: cands[best].RoomID}
	}
	return Choice{}
}

// FollowOutcome is the decision for "Tham gia bạn" (SOCIAL_SYSTEM §4.2, F091-AC-05).
type FollowOutcome int

// Follow outcomes.
const (
	FollowOK FollowOutcome = iota
	FollowDenied
	FollowFull
)

// Follow decides whether a student may join the instance of a friend. Every refusal other than a full instance is
// FollowDenied, whatever the reason, so the answer never reveals where the friend is.
func Follow(friends, blocked, samePartition, inPublicHub, hasSeat bool) FollowOutcome {
	switch {
	case !friends || blocked || !samePartition || !inPublicHub:
		return FollowDenied
	case !hasSeat:
		return FollowFull
	}
	return FollowOK
}
