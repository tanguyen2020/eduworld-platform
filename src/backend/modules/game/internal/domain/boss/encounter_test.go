package boss

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

var t0 = time.Date(2026, 11, 24, 3, 0, 0, 0, time.UTC)

func ptr[T any](v T) *T { return &v }

func act() *ActivityRef {
	return &ActivityRef{ActivityType: "quiz", ActivityID: "0193a5f4-6c1e-7b2a-9d1f-0000000c7601"}
}

// twoPhase is a party boss of grade 3: p1 collective_count 4 correct (min 1 each), p2 all with one requirement.
func twoPhase() Version {
	return Version{SchemaVersion: 1, BossID: uuid.NewString(), Code: "boss.forest.golem", OwnerScope: "platform",
		Name: Text{VI: "Người khổng lồ", EN: "Golem"}, AreaID: "forest", GradeLevels: []int{3}, SubjectID: uuid.NewString(),
		Mode: ModePartyOptional, PartySize: &PartySize{Min: 2, Max: 4}, TotalTimeLimitSeconds: 600, MaxPhaseRetries: ptr(1),
		Phases: []Phase{
			{PhaseID: "p1", Order: 1, EnergyShare: 0.5, TimeLimitSeconds: 120,
				Requirements:   []Requirement{{RequirementID: "r1", Kind: KindActivity, ActivityRef: act()}},
				CompletionRule: CompletionRule{Type: RuleCollectiveCount, CollectiveTarget: 4, MinContribution: 1}},
			{PhaseID: "p2", Order: 2, EnergyShare: 0.5, TimeLimitSeconds: 120,
				Requirements:   []Requirement{{RequirementID: "r1", Kind: KindActivity, ActivityRef: act(), PassRule: &PassRule{MinCorrectRatio: ptr(0.5)}}},
				CompletionRule: CompletionRule{Type: RuleAll}},
		}}
}

func members(ids ...uuid.UUID) []Member {
	out := make([]Member, len(ids))
	for i, id := range ids {
		out[i] = Member{StudentID: id, Grade: 3}
	}
	return out
}

func party(v Version, a, b uuid.UUID) *Encounter {
	return New(Start{ID: uuid.New(), TenantID: uuid.New(), BossID: uuid.New(), VersionID: uuid.New(), Mode: EncounterParty,
		Members: members(a, b), Grace: 2 * time.Minute, At: t0})
}

func round(items, correct map[uuid.UUID]map[string]int) Round {
	return Round{Items: items, Correct: correct}
}

func TestValidate(t *testing.T) {
	if err := twoPhase().Validate(); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(v *Version){
		"energy sum":        func(v *Version) { v.Phases[1].EnergyShare = 0.4 },
		"any_n above reqs":  func(v *Version) { v.Phases[0].CompletionRule = CompletionRule{Type: RuleAnyN, N: 2} },
		"collective no min": func(v *Version) { v.Phases[0].CompletionRule.MinContribution = 0 },
		"minigame kind":     func(v *Version) { v.Phases[0].Requirements[0].Kind = "minigame" },
		"no party size":     func(v *Version) { v.PartySize = nil },
		"dup phase":         func(v *Version) { v.Phases[1].PhaseID = "p1" },
		"grade no activity": func(v *Version) { v.Phases[0].Requirements[0].ActivityRef = nil },
	}
	for name, edit := range cases {
		v := twoPhase()
		edit(&v)
		if err := v.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", name, err)
		}
	}
}

func TestPerGradeActivity(t *testing.T) {
	r := Requirement{RequirementID: "r1", Kind: KindActivity, ActivityRef: act(),
		PerGrade: []GradeActivity{{GradeLevel: 4, ActivityRef: ActivityRef{ActivityType: "quiz", ActivityID: "g4"}}}}
	if a, _ := r.ActivityFor(4); a.ActivityID != "g4" {
		t.Fatalf("grade 4 uses its per_grade activity, got %s", a.ActivityID)
	}
	if a, _ := r.ActivityFor(3); a.ActivityID != act().ActivityID {
		t.Fatalf("grade 3 without per_grade uses the common activity_ref")
	}
	if r.Need(5) != 4 || r.Need(0) < 1000 {
		t.Fatalf("need of 5 items with ratio 0.7 is 4; no items never done")
	}
}

// F103-AC-04: the phase starts when every present member is ready; a solo encounter needs one member.
func TestReadyStartsPhase(t *testing.T) {
	v := twoPhase()
	a, b := uuid.New(), uuid.New()
	e := party(v, a, b)
	if ef, err := e.SetReady(v, a, true, t0.Add(time.Second)); err != nil || ef.Started {
		t.Fatalf("one ready of two: not started (%v, %+v)", err, ef)
	}
	ef, err := e.SetReady(v, b, true, t0.Add(2*time.Second))
	if err != nil || !ef.Started || !ef.PhaseStarted || e.Status != StatusPhaseActive || e.PhaseID != "p1" {
		t.Fatalf("all ready: phase p1 starts, got %+v %+v", ef, e)
	}
	if !e.PhaseDeadline.Equal(t0.Add(2*time.Second+120*time.Second)) || !e.Deadline.Equal(t0.Add(2*time.Second+600*time.Second)) {
		t.Fatalf("deadlines on the server clock: %v %v", e.PhaseDeadline, e.Deadline)
	}
	if _, err := e.SetReady(v, uuid.New(), true, t0); !errors.Is(err, ErrNotMember) {
		t.Fatalf("a stranger is not a member: %v", err)
	}
}

// F103-AC-12, AC-04: nobody ready past the grace → abandoned; one of two ready below party_size.min → abandoned;
// both ready late are absent.
func TestReadyTimeout(t *testing.T) {
	v := twoPhase()
	a, b := uuid.New(), uuid.New()
	e := party(v, a, b)
	if ef := e.ReadyTimeout(v, t0.Add(time.Minute)); ef.Ended {
		t.Fatal("before the grace nothing happens")
	}
	ef := e.ReadyTimeout(v, t0.Add(2*time.Minute))
	if !ef.Ended || e.Outcome != OutcomeAbandoned || e.EndReason != EndNoReady || *e.Members[0].Eligible {
		t.Fatalf("nobody came in: abandoned without reward, got %+v", e)
	}
	e = party(v, a, b)
	_, _ = e.SetReady(v, a, true, t0)
	if ef := e.ReadyTimeout(v, t0.Add(2*time.Minute)); !ef.Ended || e.Outcome != OutcomeAbandoned {
		t.Fatalf("one ready below party_size.min 2: abandoned, got %+v", e)
	}
	v.PartySize.Min = 1
	e = party(v, a, b)
	_, _ = e.SetReady(v, a, true, t0)
	if ef := e.ReadyTimeout(v, t0.Add(2*time.Minute)); !ef.Started || e.Status != StatusPhaseActive {
		t.Fatalf("one ready ≥ min 1: starts, the other is absent, got %+v", e)
	}
	if m, _ := e.Member(b); m.Present() || m.LeftReason != LeftDisconnect {
		t.Fatalf("the member not ready left (disconnected): %+v", m)
	}
}

func started(t *testing.T, v Version, a, b uuid.UUID) *Encounter {
	t.Helper()
	e := party(v, a, b)
	_, _ = e.SetReady(v, a, true, t0)
	if _, err := e.SetReady(v, b, true, t0); err != nil {
		t.Fatal(err)
	}
	return e
}

// F103-AC-07: collective_count needs the target and min_contribution of every present member; the energy drops by
// energy_share; the eligibility of the phase is fixed for the present members; the last phase defeats the boss.
func TestCollectiveCountAndDefeat(t *testing.T) {
	v := twoPhase()
	a, b := uuid.New(), uuid.New()
	e := started(t, v, a, b)
	items := map[uuid.UUID]map[string]int{a: {"r1": 5}, b: {"r1": 5}}
	r := round(items, map[uuid.UUID]map[string]int{a: {"r1": 4}})
	if e.Met(v, r) {
		t.Fatal("A alone reached the target but B has 0 < min_contribution 1")
	}
	if p := e.Progress(v, r); p != 1 {
		t.Fatalf("shared progress is the collective ratio, got %v", p)
	}
	r = round(items, map[uuid.UUID]map[string]int{a: {"r1": 3}, b: {"r1": 1}})
	ef := e.Evaluate(v, r, t0.Add(time.Minute))
	if ef.PhaseCleared != "p1" || !ef.PhaseStarted || e.PhaseID != "p2" || e.Energy != 0.5 || e.PhasesCleared != 1 {
		t.Fatalf("p1 cleared, p2 starts, energy 0.5: %+v %+v", ef, e)
	}
	if !e.Contribution(a, "p1").Eligible || !e.Contribution(b, "p1").Eligible {
		t.Fatal("both reached min_contribution in p1")
	}
	// p2 rule all with ratio 0.5 of 2 items: every present member needs 1 correct.
	items2 := map[uuid.UUID]map[string]int{a: {"r1": 2}, b: {"r1": 2}}
	if ef := e.Evaluate(v, round(items2, map[uuid.UUID]map[string]int{a: {"r1": 2}}), t0.Add(2*time.Minute)); ef.Ended {
		t.Fatal("B has not completed r1")
	}
	ef = e.Evaluate(v, round(items2, map[uuid.UUID]map[string]int{a: {"r1": 2}, b: {"r1": 1}}), t0.Add(3*time.Minute))
	if !ef.Ended || e.Status != StatusDefeated || e.Outcome != OutcomeDefeated || e.Energy != 0 || e.Result() != "defeated" {
		t.Fatalf("defeated: %+v %+v", ef, e)
	}
	if !*e.Members[0].Eligible || !*e.Members[1].Eligible {
		t.Fatal("both eligible")
	}
}

// F103-AC-07: any_n counts requirements completed by someone; all needs every present member.
func TestAnyN(t *testing.T) {
	v := twoPhase()
	v.Phases[0] = Phase{PhaseID: "p1", Order: 1, EnergyShare: 0.5, TimeLimitSeconds: 60,
		Requirements: []Requirement{{RequirementID: "r1", Kind: KindActivity, ActivityRef: act()},
			{RequirementID: "r2", Kind: KindActivity, ActivityRef: act()}, {RequirementID: "r3", Kind: KindActivity, ActivityRef: act()}},
		CompletionRule: CompletionRule{Type: RuleAnyN, N: 2}}
	a, b := uuid.New(), uuid.New()
	e := started(t, v, a, b)
	items := map[uuid.UUID]map[string]int{a: {"r1": 2, "r2": 2, "r3": 2}, b: {"r1": 2, "r2": 2, "r3": 2}}
	if e.Met(v, round(items, map[uuid.UUID]map[string]int{a: {"r1": 2}, b: {"r1": 2}})) {
		t.Fatal("one distinct requirement only")
	}
	if !e.Met(v, round(items, map[uuid.UUID]map[string]int{a: {"r1": 2}, b: {"r3": 2}})) {
		t.Fatal("two distinct requirements done by someone")
	}
}

// F103-AC-08: the phase deadline restarts the phase with new questions (previous phases kept) until max_phase_retries;
// then the encounter ends with the phases cleared; end_encounter ends at once; the total deadline ends time_up.
func TestTimeoutRetry(t *testing.T) {
	v := twoPhase()
	a, b := uuid.New(), uuid.New()
	e := started(t, v, a, b)
	items := map[uuid.UUID]map[string]int{a: {"r1": 5}, b: {"r1": 5}}
	e.Evaluate(v, round(items, map[uuid.UUID]map[string]int{a: {"r1": 3}, b: {"r1": 1}}), t0.Add(time.Minute))
	if ef := e.Timeout(v, t0.Add(2*time.Minute)); ef.Retry {
		t.Fatal("before the p2 deadline nothing")
	}
	ef := e.Timeout(v, t0.Add(3*time.Minute+time.Second))
	if !ef.Retry || !ef.PhaseStarted || e.Retries != 1 || e.PhaseID != "p2" || e.Energy != 0.5 || e.PhasesCleared != 1 {
		t.Fatalf("p2 retried with p1 kept: %+v %+v", ef, e)
	}
	ef = e.Timeout(v, e.PhaseDeadline.Add(time.Second))
	if !ef.Ended || e.Outcome != OutcomeTimeUp || e.EndReason != EndRetries || e.PhasesCleared != 1 {
		t.Fatalf("past max_phase_retries 1: ended with 1 phase cleared, got %+v", e)
	}
	if *e.Members[0].Eligible {
		t.Fatal("not defeated: nobody is eligible")
	}
	v.Phases[0].OnFail = OnFailEnd
	e = started(t, v, a, b)
	if ef := e.Timeout(v, t0.Add(121*time.Second)); !ef.Ended || e.EndReason != EndPhaseFailed {
		t.Fatalf("end_encounter: %+v", e)
	}
	v.Phases[0].OnFail = ""
	e = started(t, v, a, b)
	if ef := e.Timeout(v, t0.Add(601*time.Second)); !ef.Ended || e.EndReason != EndTotalTime || e.Outcome != OutcomeTimeUp {
		t.Fatalf("total deadline: %+v", e)
	}
}

// F103-AC-10: a member present at a cleared phase below min_contribution is not eligible; a member who left after
// meeting it in the phases it took part in stays eligible.
func TestEligibility(t *testing.T) {
	v := twoPhase()
	v.PartySize.Min = 1
	v.Phases[0].CompletionRule = CompletionRule{Type: RuleCollectiveCount, CollectiveTarget: 3, MinContribution: 1}
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	e := New(Start{ID: uuid.New(), Mode: EncounterParty, Members: members(a, b, c), Grace: time.Minute, At: t0})
	for _, s := range []uuid.UUID{a, b, c} {
		_, _ = e.SetReady(v, s, true, t0)
	}
	items := map[uuid.UUID]map[string]int{a: {"r1": 5}, b: {"r1": 5}, c: {"r1": 5}}
	e.Evaluate(v, round(items, map[uuid.UUID]map[string]int{a: {"r1": 1}, b: {"r1": 1}, c: {"r1": 1}}), t0.Add(time.Second))
	if _, err := e.Leave(v, c, LeftByPlayer, t0.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	items2 := map[uuid.UUID]map[string]int{a: {"r1": 2}, b: {"r1": 2}}
	e.Evaluate(v, round(items2, map[uuid.UUID]map[string]int{a: {"r1": 1}, b: {"r1": 1}}), t0.Add(3*time.Second))
	if e.Status != StatusDefeated {
		t.Fatalf("defeated: %+v", e)
	}
	for _, s := range []uuid.UUID{a, b, c} {
		if m, _ := e.Member(s); !*m.Eligible {
			t.Fatalf("%v eligible (met min in every phase it took part in)", s)
		}
	}
}

// F103-AC-11, AC-12: an offline member is held for the grace; back online it is restored; past the grace it leaves
// with its contribution and the encounter goes on while ≥ party_size.min remain, else abandoned. Leave is idempotent.
func TestPresenceAndLeave(t *testing.T) {
	v := twoPhase()
	a, b := uuid.New(), uuid.New()
	e := started(t, v, a, b)
	grace := 2 * time.Minute
	e.Presence(v, map[uuid.UUID]bool{a: true}, grace, t0.Add(10*time.Second))
	if m, _ := e.Member(b); m.DisconnectedAt == nil || !m.Present() {
		t.Fatal("B offline is held")
	}
	e.Presence(v, map[uuid.UUID]bool{a: true, b: true}, grace, t0.Add(30*time.Second))
	if m, _ := e.Member(b); m.DisconnectedAt != nil {
		t.Fatal("B back online (another device) is restored")
	}
	e.Presence(v, map[uuid.UUID]bool{a: true}, grace, t0.Add(40*time.Second))
	ef := e.Presence(v, map[uuid.UUID]bool{a: true}, grace, t0.Add(40*time.Second+grace))
	if !ef.Ended || e.Outcome != OutcomeAbandoned || e.EndReason != EndBelowMin {
		t.Fatalf("B past the grace leaves; 1 < min 2: abandoned, got %+v", e)
	}
	e = started(t, v, a, b)
	if ef, _ := e.Leave(v, a, LeftByPlayer, t0.Add(time.Second)); !ef.Ended {
		t.Fatal("A leaves: 1 < min 2 → abandoned")
	}
	if ef, err := e.Leave(v, a, LeftByPlayer, t0.Add(2*time.Second)); err != nil || ef.Ended {
		t.Fatal("leave again is a no-op")
	}
}

// Edge case: learning failures at the start of a round are retried; past the limit the encounter is abandoned.
func TestPlanFailures(t *testing.T) {
	v := twoPhase()
	a, b := uuid.New(), uuid.New()
	e := started(t, v, a, b)
	for i := 0; i < 2; i++ {
		if ef := e.PlanFailed(2, t0); ef.Ended {
			t.Fatalf("failure %d is retried", i+1)
		}
	}
	if ef := e.PlanFailed(2, t0); !ef.Ended || e.EndReason != EndPlanFailure || e.Outcome != OutcomeAbandoned {
		t.Fatalf("third failure abandons: %+v", e)
	}
}

func TestNextDue(t *testing.T) {
	v := twoPhase()
	a, b := uuid.New(), uuid.New()
	e := party(v, a, b)
	if d := e.NextDue(5*time.Second, t0); !d.Equal(t0.Add(5 * time.Second)) {
		t.Fatalf("lobby checks every 5 s before the ready deadline: %v", d)
	}
	if d := e.NextDue(time.Hour, t0); !d.Equal(e.ReadyDeadline) {
		t.Fatalf("lobby due is the ready deadline: %v", d)
	}
}
