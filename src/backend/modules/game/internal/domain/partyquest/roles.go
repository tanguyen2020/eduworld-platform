package partyquest

import (
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
)

// MultiRole reports allow_multi_role of a step (default true).
func MultiRole(st quest.Step) bool { return st.AllowMultiRole == nil || *st.AllowMultiRole }

// roleNames returns the roles of a step in their authored order.
func roleNames(st quest.Step) []string {
	out := make([]string, 0, len(st.Roles))
	for _, r := range st.Roles {
		out = append(out, r.Role)
	}
	return out
}

// RolesAssigned reports roles assigned (or awaited from the leader) for a step.
func (s *Session) RolesAssigned(stepID string) bool {
	r, ok := s.Roles[stepID]
	return ok && (len(r.Assignments) > 0 || r.Pending)
}

// AssignAuto distributes the roles of an interdependent step (F103-AC-20, role_assignment auto): role i goes to the
// present member i in join order; with fewer members than roles and allow_multi_role the remaining roles go round
// the members again (each member holds at least one role). A leader_assigns step is only marked pending.
func (s *Session) AssignAuto(st quest.Step) bool {
	if !s.Active() || s.RolesAssigned(st.ID) {
		return false
	}
	if st.RoleAssignment == AssignLeader && len(s.Present()) > 1 {
		s.Roles[st.ID] = Roles{Assignments: map[string]uuid.UUID{}, Pending: true}
		s.bump()
		return true
	}
	s.Roles[st.ID] = Roles{Assignments: distribute(roleNames(st), s.Present(), MultiRole(st))}
	s.bump()
	return true
}

// distribute gives role i to member i mod n; without multi-role a role beyond the members stays unassigned.
func distribute(roles []string, members []uuid.UUID, multi bool) map[string]uuid.UUID {
	out := map[string]uuid.UUID{}
	if len(members) == 0 {
		return out
	}
	for i, r := range roles {
		if i >= len(members) && !multi {
			break
		}
		out[r] = members[i%len(members)]
	}
	return out
}

// Assignment is one role → student of assignQuestStepRoles.
type Assignment struct {
	Role      string
	StudentID uuid.UUID
}

// AssignByLeader is assignQuestStepRoles (F103-AC-20): only the leader; every role of the step exactly once to a
// present member; one member holds several roles only when allow_multi_role.
func (s *Session) AssignByLeader(st quest.Step, by uuid.UUID, as []Assignment) error {
	if !s.Active() {
		return ErrInvalidState
	}
	if by != s.LeaderID {
		return ErrNotLeader
	}
	roles := roleNames(st)
	if len(as) != len(roles) {
		return invalid("every role of the step is assigned exactly once")
	}
	out := map[string]uuid.UUID{}
	held := map[uuid.UUID]int{}
	for _, a := range as {
		if !slices.Contains(roles, a.Role) {
			return invalid("role %s is not a role of the step", a.Role)
		}
		if _, dup := out[a.Role]; dup {
			return invalid("role %s is assigned twice", a.Role)
		}
		if !s.IsMember(a.StudentID) {
			return invalid("a student of the assignment is not a member of the session")
		}
		out[a.Role] = a.StudentID
		held[a.StudentID]++
	}
	if !MultiRole(st) {
		for _, n := range held {
			if n > 1 {
				return invalid("allow_multi_role is false: one role per member")
			}
		}
	}
	s.Roles[st.ID] = Roles{Assignments: out}
	s.bump()
	return nil
}

// RolesOf returns the roles a student holds on a step, in authored order.
func (s *Session) RolesOf(st quest.Step, student uuid.UUID) []string {
	r := s.Roles[st.ID]
	var out []string
	for _, name := range roleNames(st) {
		if id, ok := r.Assignments[name]; ok && id == student {
			out = append(out, name)
		}
	}
	return out
}

// Reassign moves the roles of a member who left (or is past the grace) to the present members with the fewest roles
// (then join order) when allow_multi_role (F103-AC-24, QUEST_SYSTEM §3.2); the clue cards already shared stay.
func (s *Session) Reassign(st quest.Step, gone uuid.UUID) bool {
	r, ok := s.Roles[st.ID]
	if !ok || !MultiRole(st) {
		return false
	}
	present := s.Present()
	if len(present) == 0 {
		return false
	}
	changed := false
	for _, name := range roleNames(st) {
		if id, ok := r.Assignments[name]; !ok || id != gone {
			continue
		}
		load := map[uuid.UUID]int{}
		for _, id := range r.Assignments {
			load[id]++
		}
		cands := slices.Clone(present)
		cands = slices.DeleteFunc(cands, func(id uuid.UUID) bool { return id == gone })
		if len(cands) == 0 {
			continue
		}
		sort.SliceStable(cands, func(i, j int) bool { return load[cands[i]] < load[cands[j]] })
		r.Assignments[name] = cands[0]
		changed = true
	}
	if changed {
		s.Roles[st.ID] = r
		s.bump()
	}
	return changed
}

// SharedRoles returns the roles of a step whose clue card is on the shared board.
func (s *Session) SharedRoles(stepID string) []string {
	var out []string
	for _, c := range s.Clues {
		if c.StepID == stepID {
			out = append(out, c.Role)
		}
	}
	return out
}

// Shared returns the clue of a role on the board.
func (s *Session) Shared(stepID, role string) (Clue, bool) {
	for _, c := range s.Clues {
		if c.StepID == stepID && c.Role == role {
			return c, true
		}
	}
	return Clue{}, false
}

// ShareClue puts the clue card of a role the student holds on the shared board (F103-AC-22); false when it is there
// already (a repeated call never duplicates the card); ErrNotRoleOwner for a role the student does not hold.
func (s *Session) ShareClue(st quest.Step, role string, student uuid.UUID, now time.Time) (bool, error) {
	if !s.Active() {
		return false, ErrInvalidState
	}
	if !slices.Contains(s.RolesOf(st, student), role) {
		return false, ErrNotRoleOwner
	}
	if _, ok := s.Shared(st.ID, role); ok {
		return false, nil
	}
	s.Clues = append(s.Clues, Clue{StepID: st.ID, Role: role, SharedBy: student, SharedAt: now})
	s.bump()
	return true, nil
}

// Visible returns the roles whose clue a student may read on a step (F103-AC-21): the roles it holds plus the shared
// cards; nothing else.
func (s *Session) Visible(st quest.Step, student uuid.UUID) []string {
	mine := s.RolesOf(st, student)
	var out []string
	for _, name := range roleNames(st) {
		if slices.Contains(mine, name) || slices.Contains(s.SharedRoles(st.ID), name) {
			out = append(out, name)
		}
	}
	return out
}
