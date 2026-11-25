// Package partyquest is the domain of the co-op quests of module game (F103, QUEST_SYSTEM §3.2, §6): the
// PartyQuestSession aggregate — the shared progress of a party on a quest version next to the personal QuestProgress of
// every member (research R4) — with the shared_progress_policy (individual, shared_steps, collective_count), the
// collective steps, the roles and shared clue cards of the interdependent steps, and the membership rules (leave,
// disconnect grace, disband). Learning results are never shared (FR-PTY-013): the session only decides which quest
// steps count for whom. Pure: no I/O, the clock is passed in.
package partyquest

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
)

// Shared progress policies (QUEST_SYSTEM §6 — owner of the enum).
const (
	PolicyIndividual      = "individual"
	PolicySharedSteps     = "shared_steps"
	PolicyCollectiveCount = "collective_count"
)

// Policies are the three values of shared_progress_policy.
var Policies = []string{PolicyIndividual, PolicySharedSteps, PolicyCollectiveCount}

// Party modes (quest.schema.json party_mode).
const (
	PartySolo     = "solo"
	PartyOptional = "party_optional"
	PartyRequired = "party_required"
)

// Assignees (quest.schema.json assignee).
const (
	AssigneeEach = "each_member"
	AssigneeAny  = "any_member"
)

// Role assignment, share mode and completion of an interdependent step.
const (
	AssignAuto      = "auto"
	AssignLeader    = "leader_assigns"
	CompleteAll     = "all_members"
	CompleteMinimum = "min_members"
)

// Session statuses and end reasons (party_quest_sessions.status, end_reason; event quest.party_completed outcome).
const (
	StatusActive    = "active"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusAbandoned = "abandoned"
	EndCompleted    = "completed"
	EndFailed       = "failed"
	EndDisbanded    = "disbanded"
	EndAbandoned    = "abandoned"
)

// Leave reasons of a member of a session.
const (
	LeftParty      = "party_left"
	LeftKicked     = "kicked"
	LeftDisconnect = "disconnected"
)

// Domain errors; the application maps them to problem codes.
var (
	ErrNotMember    = errors.New("partyquest: not a member of the session") // 404
	ErrNotLeader    = errors.New("partyquest: not the leader")              // 403 PARTY_NOT_LEADER
	ErrNotRoleOwner = errors.New("partyquest: role not held")               // 403 PERMISSION_DENIED
	ErrInvalid      = errors.New("partyquest: invalid")                     // 422 VALIDATION_FAILED
	ErrInvalidState = errors.New("partyquest: invalid state")               // 409 / 422
)

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, a...))
}

// Member is a member of a session.
type Member struct {
	StudentID      uuid.UUID  `json:"student_id"`
	Order          int        `json:"order"`
	DisconnectedAt *time.Time `json:"disconnected_at,omitempty"`
	LeftAt         *time.Time `json:"left_at,omitempty"`
	LeftReason     string     `json:"left_reason,omitempty"`
}

// Present reports a member still in the session.
func (m Member) Present() bool { return m.LeftAt == nil }

// SharedStep is the shared state of one step of the session.
type SharedStep struct {
	Status      string     `json:"status"` // completed when shared for the party
	CompletedBy *uuid.UUID `json:"completed_by,omitempty"`
	Collective  int        `json:"collective_progress,omitempty"` // sum of the metric (collective_target)
	Target      int        `json:"target,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// Roles is the role state of one interdependent step.
type Roles struct {
	Assignments map[string]uuid.UUID `json:"assignments"`
	Pending     bool                 `json:"pending,omitempty"` // leader_assigns: waiting for the leader
}

// Clue is a clue card on the shared board (never its content).
type Clue struct {
	StepID   string    `json:"step_id"`
	Role     string    `json:"role"`
	SharedBy uuid.UUID `json:"shared_by"`
	SharedAt time.Time `json:"shared_at"`
}

// Session is the PartyQuestSession aggregate.
type Session struct {
	ID, TenantID, SchoolID, PartyID, PartyActivityID, LeaderID uuid.UUID
	QuestID, VersionID                                         uuid.UUID
	PartyMode, Policy                                          string
	SizeMin                                                    int
	Status                                                     string
	Members                                                    []Member
	Steps                                                      map[string]SharedStep
	Roles                                                      map[string]Roles
	Clues                                                      []Clue
	StateVersion                                               int64
	StartedAt                                                  time.Time
	EndedAt                                                    *time.Time
	EndReason                                                  string
}

// Start is what a new session needs.
type Start struct {
	ID, TenantID, SchoolID, PartyID, PartyActivityID, LeaderID uuid.UUID
	Version                                                    quest.Version
	QuestID, VersionID                                         uuid.UUID
	Members                                                    []uuid.UUID // join order
	At                                                         time.Time
}

// New starts an active session for the members in join order.
func New(s Start) *Session {
	policy := s.Version.SharedProgressPolicy
	if policy == "" {
		policy = PolicyIndividual
	}
	out := &Session{ID: s.ID, TenantID: s.TenantID, SchoolID: s.SchoolID, PartyID: s.PartyID, PartyActivityID: s.PartyActivityID,
		LeaderID: s.LeaderID, QuestID: s.QuestID, VersionID: s.VersionID, PartyMode: s.Version.PartyMode, Policy: policy, SizeMin: 1,
		Status: StatusActive, Steps: map[string]SharedStep{}, Roles: map[string]Roles{}, StartedAt: s.At}
	if s.Version.PartySize != nil && s.Version.PartySize.Min > 0 {
		out.SizeMin = s.Version.PartySize.Min
	}
	for i, m := range s.Members {
		out.Members = append(out.Members, Member{StudentID: m, Order: i + 1})
	}
	return out
}

// Active reports an active session.
func (s *Session) Active() bool { return s.Status == StatusActive }

func (s *Session) bump() { s.StateVersion++ }

// Member returns the member of a student.
func (s *Session) Member(student uuid.UUID) (*Member, bool) {
	for i := range s.Members {
		if s.Members[i].StudentID == student {
			return &s.Members[i], true
		}
	}
	return nil, false
}

// IsMember reports a present member.
func (s *Session) IsMember(student uuid.UUID) bool {
	m, ok := s.Member(student)
	return ok && m.Present()
}

// Present returns the present members by join order.
func (s *Session) Present() []uuid.UUID {
	ms := slices.Clone(s.Members)
	sort.SliceStable(ms, func(i, j int) bool { return ms[i].Order < ms[j].Order })
	var out []uuid.UUID
	for _, m := range ms {
		if m.Present() {
			out = append(out, m.StudentID)
		}
	}
	return out
}

// frameTypes are the steps a member may complete for the party (QUEST_SYSTEM §6: talk_npc/go_to/collect, and discover).
var frameTypes = []string{quest.StepTalkNPC, quest.StepGoTo, quest.StepCollect, quest.StepDiscover}

// SharesStep reports a frame step completed by one member for the party (F103-AC-14): policy shared_steps or
// collective_count, and assignee any_member (or absent — any_member is the default of a frame step in a shared policy).
// A learning step is never shared.
func (s *Session) SharesStep(st quest.Step) bool {
	if s.Policy == PolicyIndividual || !slices.Contains(frameTypes, st.Type) {
		return false
	}
	return st.Assignee == "" || st.Assignee == AssigneeAny
}

// CollectiveStep reports a step whose completion is the collective target of the party (policy collective_count and a
// collective_target), never the pass rule of one member (F103-AC-15).
func (s *Session) CollectiveStep(st quest.Step) bool {
	return s.Policy == PolicyCollectiveCount && st.CollectiveTarget != nil
}

// MarkShared records a frame step completed by student for the party; false when it was shared before (replays and
// the propagated completions of the other members change nothing).
func (s *Session) MarkShared(stepID string, by uuid.UUID, now time.Time) bool {
	if !s.Active() {
		return false
	}
	if st, ok := s.Steps[stepID]; ok && st.Status == quest.StepCompleted {
		return false
	}
	b := by
	s.Steps[stepID] = SharedStep{Status: quest.StepCompleted, CompletedBy: &b, CompletedAt: &now}
	s.bump()
	return true
}

// Collective records the total of a collective step (sum of the members' metric) and reports the target reached now.
func (s *Session) Collective(st quest.Step, total int, now time.Time) bool {
	if st.CollectiveTarget == nil {
		return false
	}
	cur := s.Steps[st.ID]
	was := cur.Status == quest.StepCompleted
	cur.Collective, cur.Target = total, st.CollectiveTarget.Count
	if total >= st.CollectiveTarget.Count && !was {
		cur.Status, cur.CompletedAt = quest.StepCompleted, &now
	}
	s.Steps[st.ID] = cur
	s.bump()
	return cur.Status == quest.StepCompleted && !was
}

// CollectiveDone reports the collective target of a step reached.
func (s *Session) CollectiveDone(stepID string) bool {
	return s.Steps[stepID].Status == quest.StepCompleted
}

// CollectiveProgress is the shared progress of a collective step in [0, 1].
func (s *Session) CollectiveProgress(st quest.Step) float64 {
	if st.CollectiveTarget == nil || st.CollectiveTarget.Count <= 0 {
		return 0
	}
	return min(1, float64(s.Steps[st.ID].Collective)/float64(st.CollectiveTarget.Count))
}

// InterdependentDone reports the completion of an interdependent step for the party (F103-AC-23): all_members (every
// present member passed) or min_members (at least min_members_count members passed).
func (s *Session) InterdependentDone(st quest.Step, passed map[uuid.UUID]bool) bool {
	n := 0
	for _, id := range s.Present() {
		if passed[id] {
			n++
		}
	}
	if st.Completion == CompleteMinimum {
		need := max(1, st.MinMembersCount)
		return n >= min(need, max(1, len(s.Present())))
	}
	return n > 0 && n == len(s.Present())
}

// End closes the session once (quest.party_completed exactly once, F103-AC-17).
func (s *Session) End(status, reason string, now time.Time) bool {
	if !s.Active() {
		return false
	}
	s.Status, s.EndReason, s.EndedAt = status, reason, &now
	s.bump()
	return true
}

// Leave removes a member (party.member_left/kicked, past coop_quest_reconnect_grace — F103-AC-19): its progress and
// the shared steps already completed stay; its roles move to the present members when allow_multi_role.
func (s *Session) Leave(v quest.Version, student uuid.UUID, reason string, now time.Time) bool {
	m, ok := s.Member(student)
	if !ok || !m.Present() || !s.Active() {
		return false
	}
	m.LeftAt, m.LeftReason = &now, reason
	for stepID := range s.Roles {
		if st, ok := v.Step(stepID); ok {
			s.Reassign(st, student)
		}
	}
	if s.LeaderID == student {
		if p := s.Present(); len(p) > 0 {
			s.LeaderID = p[0]
		}
	}
	s.bump()
	return true
}

// Continues reports whether the session goes on after a member left (QUEST_SYSTEM §6): party_mode ≠ party_required, or
// at least party_size.min members remain.
func (s *Session) Continues() bool {
	n := len(s.Present())
	if n == 0 {
		return false
	}
	return s.PartyMode != PartyRequired || n >= s.SizeMin
}

// Presence applies the online state of the present members with the grace (F103-AC-19, AC-24): returns the members
// who left now (offline past the grace).
func (s *Session) Presence(v quest.Version, online map[uuid.UUID]bool, grace time.Duration, now time.Time) []uuid.UUID {
	if !s.Active() {
		return nil
	}
	var expired []uuid.UUID
	for i := range s.Members {
		m := &s.Members[i]
		if !m.Present() {
			continue
		}
		switch {
		case online[m.StudentID] && m.DisconnectedAt != nil:
			m.DisconnectedAt = nil
			s.bump()
		case !online[m.StudentID] && m.DisconnectedAt == nil:
			at := now
			m.DisconnectedAt = &at
			s.bump()
		case !online[m.StudentID] && now.Sub(*m.DisconnectedAt) >= grace:
			expired = append(expired, m.StudentID)
		}
	}
	for _, id := range expired {
		s.Leave(v, id, LeftDisconnect, now)
	}
	return expired
}
