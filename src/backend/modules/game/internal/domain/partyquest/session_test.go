package partyquest

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
)

var t0 = time.Date(2026, 11, 24, 3, 0, 0, 0, time.UTC)

func roles(n int) []quest.Role {
	names := []string{"measurer", "map_reader", "scout", "scribe"}
	out := make([]quest.Role, n)
	for i := range out {
		out[i] = quest.Role{Role: names[i], Title: quest.Text{VI: names[i], EN: names[i]},
			Clue: quest.ClueRef{Kind: "learning_material", ID: uuid.NewString()}}
	}
	return out
}

func version(policy, mode string) quest.Version {
	return quest.Version{PartyMode: mode, PartySize: &quest.PartySize{Min: 2, Max: 4}, SharedProgressPolicy: policy,
		Steps: []quest.Step{
			{ID: "walk", Order: 1, Type: quest.StepGoTo},
			{ID: "solo_walk", Order: 2, Type: quest.StepGoTo, Assignee: AssigneeEach},
			{ID: "learn", Order: 3, Type: quest.StepActivity},
			{ID: "count", Order: 4, Type: quest.StepActivity, CollectiveTarget: &quest.CollectiveTarget{Metric: "correct_answers", Count: 20, MinContribution: 5}},
			{ID: "puzzle", Order: 5, Type: quest.StepInterdep, Roles: roles(2)},
		}}
}

func session(v quest.Version, ms ...uuid.UUID) *Session {
	return New(Start{ID: uuid.New(), PartyID: uuid.New(), PartyActivityID: uuid.New(), LeaderID: ms[0], Version: v, Members: ms, At: t0})
}

func step(v quest.Version, id string) quest.Step {
	s, _ := v.Step(id)
	return s
}

// F103-AC-14: shared_steps/collective_count share the frame steps of any_member (default) — never each_member or a
// learning step; individual shares nothing; a shared step is marked once.
func TestSharesStep(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	for _, policy := range []string{PolicySharedSteps, PolicyCollectiveCount} {
		v := version(policy, PartyOptional)
		s := session(v, a, b)
		if !s.SharesStep(step(v, "walk")) || s.SharesStep(step(v, "solo_walk")) || s.SharesStep(step(v, "learn")) {
			t.Fatalf("%s: frame any_member shared, each_member and learning not", policy)
		}
		if !s.MarkShared("walk", a, t0) || s.MarkShared("walk", b, t0) {
			t.Fatalf("%s: marked once", policy)
		}
	}
	v := version(PolicyIndividual, PartyOptional)
	if session(v, a, b).SharesStep(step(v, "walk")) {
		t.Fatal("individual shares nothing")
	}
}

// F103-AC-15: the collective step completes once when the total reaches count; only collective_count uses it.
func TestCollective(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	v := version(PolicyCollectiveCount, PartyOptional)
	s := session(v, a, b)
	st := step(v, "count")
	if !s.CollectiveStep(st) || s.CollectiveStep(step(v, "learn")) {
		t.Fatal("collective step is the one with collective_target")
	}
	if s.Collective(st, 18, t0) || s.CollectiveProgress(st) != 0.9 {
		t.Fatal("18 of 20: not done, progress 0.9")
	}
	if !s.Collective(st, 20, t0) || !s.CollectiveDone("count") {
		t.Fatal("20 of 20: completes now")
	}
	if s.Collective(st, 23, t0) {
		t.Fatal("completes once")
	}
	vs := version(PolicySharedSteps, PartyOptional)
	if session(vs, a, b).CollectiveStep(step(vs, "count")) {
		t.Fatal("shared_steps never uses collective targets")
	}
}

// F103-AC-23: all_members needs every present member to pass; min_members the count.
func TestInterdependentCompletion(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	v := version(PolicySharedSteps, PartyOptional)
	s := session(v, a, b, c)
	st := step(v, "puzzle")
	if s.InterdependentDone(st, map[uuid.UUID]bool{a: true, b: true}) {
		t.Fatal("all_members: C has not passed")
	}
	if !s.InterdependentDone(st, map[uuid.UUID]bool{a: true, b: true, c: true}) {
		t.Fatal("all passed")
	}
	st.Completion, st.MinMembersCount = CompleteMinimum, 2
	if !s.InterdependentDone(st, map[uuid.UUID]bool{a: true, c: true}) || s.InterdependentDone(st, map[uuid.UUID]bool{a: true}) {
		t.Fatal("min_members 2")
	}
}

// F103-AC-20: auto gives role i to member i in join order; fewer members than roles with allow_multi_role go round;
// solo holds every role (AC-24); leader_assigns is pending, only the leader assigns, every role once.
func TestRoles(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	v := version(PolicySharedSteps, PartyOptional)
	v.Steps[4].Roles = roles(3)
	s := session(v, a, b)
	st := step(v, "puzzle")
	if !s.AssignAuto(st) || s.AssignAuto(st) {
		t.Fatal("assigned once")
	}
	if got := s.RolesOf(st, a); !slices.Equal(got, []string{"measurer", "scout"}) {
		t.Fatalf("A holds roles 1 and 3, got %v", got)
	}
	if got := s.RolesOf(st, b); !slices.Equal(got, []string{"map_reader"}) {
		t.Fatalf("B holds role 2, got %v", got)
	}
	solo := session(v, a)
	solo.AssignAuto(st)
	if len(solo.RolesOf(st, a)) != 3 {
		t.Fatal("solo holds every role")
	}
	st.RoleAssignment = AssignLeader
	s = session(v, a, b, c)
	s.AssignAuto(st)
	if !s.Roles["puzzle"].Pending || len(s.RolesOf(st, a)) != 0 {
		t.Fatal("leader_assigns waits for the leader")
	}
	as := []Assignment{{"measurer", c}, {"map_reader", b}, {"scout", a}}
	if err := s.AssignByLeader(st, b, as); !errors.Is(err, ErrNotLeader) {
		t.Fatalf("B is not the leader: %v", err)
	}
	if err := s.AssignByLeader(st, a, as[:2]); !errors.Is(err, ErrInvalid) {
		t.Fatal("every role once")
	}
	if err := s.AssignByLeader(st, a, []Assignment{{"measurer", a}, {"map_reader", uuid.New()}, {"scout", b}}); !errors.Is(err, ErrInvalid) {
		t.Fatal("a stranger cannot hold a role")
	}
	no := false
	st.AllowMultiRole = &no
	if err := s.AssignByLeader(st, a, []Assignment{{"measurer", a}, {"map_reader", a}, {"scout", b}}); !errors.Is(err, ErrInvalid) {
		t.Fatal("allow_multi_role false: one role per member")
	}
	st.AllowMultiRole = nil
	if err := s.AssignByLeader(st, a, as); err != nil || s.RolesOf(st, c)[0] != "measurer" || s.Roles["puzzle"].Pending {
		t.Fatalf("leader assigns: %v", err)
	}
}

// F103-AC-21, AC-22: a member sees its roles and the shared cards only; sharing needs the role; once.
func TestClues(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	v := version(PolicySharedSteps, PartyOptional)
	s := session(v, a, b)
	st := step(v, "puzzle")
	s.AssignAuto(st)
	if got := s.Visible(st, b); !slices.Equal(got, []string{"map_reader"}) {
		t.Fatalf("B sees only its card, got %v", got)
	}
	if _, err := s.ShareClue(st, "measurer", b, t0); !errors.Is(err, ErrNotRoleOwner) {
		t.Fatal("B cannot share A's card")
	}
	if ok, err := s.ShareClue(st, "measurer", a, t0); !ok || err != nil {
		t.Fatal("A shares its card")
	}
	if ok, _ := s.ShareClue(st, "measurer", a, t0); ok || len(s.Clues) != 1 {
		t.Fatal("no duplicate card")
	}
	if got := s.Visible(st, b); !slices.Equal(got, []string{"measurer", "map_reader"}) {
		t.Fatalf("B sees the shared card too, got %v", got)
	}
}

// F103-AC-24, AC-19: a member past the grace leaves; its roles move to the present member (allow_multi_role), shared
// cards stay; the session continues for party_optional, and for party_required only with ≥ party_size.min.
func TestReassignAndContinue(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	v := version(PolicySharedSteps, PartyOptional)
	s := session(v, a, b)
	st := step(v, "puzzle")
	s.AssignAuto(st)
	_, _ = s.ShareClue(st, "map_reader", b, t0)
	grace := 2 * time.Minute
	if left := s.Presence(v, map[uuid.UUID]bool{a: true}, grace, t0); len(left) != 0 {
		t.Fatal("B offline is held for the grace")
	}
	left := s.Presence(v, map[uuid.UUID]bool{a: true}, grace, t0.Add(grace))
	if len(left) != 1 || left[0] != b {
		t.Fatalf("B past the grace leaves, got %v", left)
	}
	if got := s.RolesOf(st, a); len(got) != 2 {
		t.Fatalf("A holds both roles now, got %v", got)
	}
	if len(s.SharedRoles("puzzle")) != 1 || !s.Continues() {
		t.Fatal("shared card kept; party_optional continues")
	}
	vr := version(PolicySharedSteps, PartyRequired)
	r := session(vr, a, b)
	r.Leave(vr, b, LeftParty, t0)
	if r.Continues() {
		t.Fatal("party_required below party_size.min stops")
	}
	if !r.End(StatusAbandoned, EndAbandoned, t0) || r.End(StatusAbandoned, EndAbandoned, t0) {
		t.Fatal("ends once")
	}
}
