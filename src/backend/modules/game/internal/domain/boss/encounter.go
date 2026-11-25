package boss

import (
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
)

// Encounter statuses (CHALLENGE_BOSS_SYSTEM §3.3). A phase transition and a phase retry are instantaneous in the
// server: the encounter is phase_active on the next phase (or round) at once.
const (
	StatusLobby       = "lobby"
	StatusPhaseActive = "phase_active"
	StatusDefeated    = "defeated"
	StatusEnded       = "ended"
)

// Outcomes (event boss.encounter_completed outcome, AsyncAPI BossEncounterEnded).
const (
	OutcomeDefeated  = "defeated"
	OutcomeTimeUp    = "time_up"
	OutcomeAbandoned = "abandoned"
)

// End reasons (boss_encounters.end_reason).
const (
	EndDefeated      = "defeated"
	EndTotalTime     = "total_time"
	EndRetries       = "retries_exhausted"
	EndPhaseFailed   = "phase_failed"
	EndNoReady       = "no_ready"
	EndBelowMin      = "below_min"
	EndPlanFailure   = "plan_failure"
	EndRecoveryLost  = "recovery_lost"
	EndStudentAbsent = "student_unavailable"
)

// Leave reasons (boss_encounter_members.left_reason).
const (
	LeftByPlayer   = "left"
	LeftDisconnect = "disconnected"
	LeftKicked     = "kicked"
	LeftParty      = "party_left"
	LeftDisbanded  = "disbanded"
)

// Member is a member of an encounter.
type Member struct {
	StudentID      uuid.UUID
	SchoolID       uuid.UUID
	Grade          int
	JoinOrder      int
	Ready          bool
	JoinedAt       time.Time
	DisconnectedAt *time.Time
	LeftAt         *time.Time
	LeftReason     string
	Eligible       *bool
	FirstDefeat    bool
}

// Present reports a member still in the encounter (not left, not past the grace).
func (m Member) Present() bool { return m.LeftAt == nil }

// Contribution is the contribution of a member to one phase (current round).
type Contribution struct {
	StudentID  uuid.UUID
	PhaseID    string
	Correct    int
	Activities int
	Pct        float64
	Eligible   bool
	// Cleared: the phase was cleared while the member was present (it counts for the eligibility of the member).
	Cleared bool
}

// Encounter is the BossEncounter aggregate: one encounter on the pinned boss version. It changes only through its
// methods, from server facts (graded answers, the server clock, membership changes).
type Encounter struct {
	ID, TenantID, SchoolID, BossID, VersionID uuid.UUID
	Mode                                      string
	PartyID, PartyActivityID, LeaderID        *uuid.UUID
	Status                                    string
	PhaseIndex                                int
	PhaseID                                   string
	Retries                                   int
	PlanFailures                              int
	PhasesCleared                             int
	Energy                                    float64
	StateVersion                              int64
	ReadyDeadline                             time.Time
	Deadline, PhaseDeadline                   *time.Time
	Outcome, EndReason                        string
	PeriodKey                                 string
	StartedAt, EndedAt                        *time.Time
	CreatedAt                                 time.Time
	Members                                   []Member
	Contributions                             []Contribution
}

// Start is what a new encounter needs.
type Start struct {
	ID, TenantID, SchoolID, BossID, VersionID uuid.UUID
	Mode                                      string
	PartyID, PartyActivityID, LeaderID        *uuid.UUID
	Members                                   []Member // join order is the order of the slice
	PeriodKey                                 string
	Grace                                     time.Duration // boss_reconnect_grace / reconnect_grace_seconds: deadline of Ready
	At                                        time.Time
}

// New creates an encounter in lobby; members not ready yet; Ready must come before ReadyDeadline (F103-AC-04, AC-12).
func New(s Start) *Encounter {
	e := &Encounter{ID: s.ID, TenantID: s.TenantID, SchoolID: s.SchoolID, BossID: s.BossID, VersionID: s.VersionID, Mode: s.Mode,
		PartyID: s.PartyID, PartyActivityID: s.PartyActivityID, LeaderID: s.LeaderID, Status: StatusLobby, Energy: 1,
		ReadyDeadline: s.At.Add(s.Grace), PeriodKey: s.PeriodKey, CreatedAt: s.At}
	for i, m := range s.Members {
		m.JoinOrder, m.JoinedAt, m.Ready = i+1, s.At, false
		e.Members = append(e.Members, m)
	}
	return e
}

// Live reports an encounter that has not ended.
func (e *Encounter) Live() bool { return e.Status == StatusLobby || e.Status == StatusPhaseActive }

// Member returns the member of a student.
func (e *Encounter) Member(student uuid.UUID) (*Member, bool) {
	for i := range e.Members {
		if e.Members[i].StudentID == student {
			return &e.Members[i], true
		}
	}
	return nil, false
}

// PresentMembers returns the present members by join order.
func (e *Encounter) PresentMembers() []Member {
	var out []Member
	for _, m := range e.Members {
		if m.Present() {
			out = append(out, m)
		}
	}
	slices.SortFunc(out, func(a, b Member) int { return a.JoinOrder - b.JoinOrder })
	return out
}

func (e *Encounter) bump() { e.StateVersion++ }

// Contribution returns the contribution of a student to a phase (zero when none).
func (e *Encounter) Contribution(student uuid.UUID, phaseID string) Contribution {
	for _, c := range e.Contributions {
		if c.StudentID == student && c.PhaseID == phaseID {
			return c
		}
	}
	return Contribution{StudentID: student, PhaseID: phaseID}
}

func (e *Encounter) setContribution(c Contribution) {
	for i := range e.Contributions {
		if e.Contributions[i].StudentID == c.StudentID && e.Contributions[i].PhaseID == c.PhaseID {
			e.Contributions[i] = c
			return
		}
	}
	e.Contributions = append(e.Contributions, c)
}

// Effect is what a transition did, for the events and pushes of the application.
type Effect struct {
	Started       bool   // the first phase started (boss.encounter_started)
	PhaseStarted  bool   // a phase (or round) started: question plans needed
	Retry         bool   // the phase started again with new questions
	PhaseCleared  string // phase id cleared (boss.phase_completed)
	ClearedIndex  int
	Ended         bool // the encounter ended (boss.encounter_completed)
	MembersLeft   []uuid.UUID
	MembersChange bool
}

// SetReady records boss.encounter.ready of a present member in the lobby (F103-AC-04): when every present member is
// ready and at least min are present, the first phase starts.
func (e *Encounter) SetReady(v Version, student uuid.UUID, ready bool, now time.Time) (Effect, error) {
	m, ok := e.Member(student)
	if !ok || !m.Present() {
		return Effect{}, ErrNotMember
	}
	if m.DisconnectedAt != nil {
		m.DisconnectedAt = nil
	}
	if e.Status != StatusLobby {
		if e.Live() {
			return Effect{}, nil // already started: a late ready is a no-op (rejoin from another device)
		}
		return Effect{}, fmt.Errorf("%w: the encounter is %s", ErrInvalidState, e.Status)
	}
	if m.Ready == ready {
		return Effect{}, nil
	}
	m.Ready = ready
	e.bump()
	ef := Effect{MembersChange: true}
	if e.allReady() && len(e.PresentMembers()) >= e.sizeMin(v) {
		e.begin(v, now, &ef)
	}
	return ef, nil
}

func (e *Encounter) sizeMin(v Version) int {
	if e.Mode == EncounterSolo {
		return 1
	}
	return v.SizeMin()
}

func (e *Encounter) allReady() bool {
	ms := e.PresentMembers()
	if len(ms) == 0 {
		return false
	}
	for _, m := range ms {
		if !m.Ready {
			return false
		}
	}
	return true
}

func (e *Encounter) begin(v Version, now time.Time, ef *Effect) {
	dl := now.Add(time.Duration(v.TotalTimeLimitSeconds) * time.Second)
	e.Status, e.StartedAt, e.Deadline = StatusPhaseActive, &now, &dl
	e.PhaseIndex, e.Retries = 0, 0
	e.startPhase(v, now)
	ef.Started, ef.PhaseStarted = true, true
}

// startPhase opens the current phase index (round e.Retries) with its deadline on the server clock, never past the
// total deadline.
func (e *Encounter) startPhase(v Version, now time.Time) {
	p, _ := v.Phase(e.PhaseIndex)
	e.PhaseID = p.PhaseID
	pd := now.Add(time.Duration(p.TimeLimitSeconds) * time.Second)
	if e.Deadline != nil && pd.After(*e.Deadline) {
		pd = *e.Deadline
	}
	e.PhaseDeadline = &pd
	e.Status = StatusPhaseActive
	e.bump()
}

// ReadyTimeout runs at ReadyDeadline in the lobby (F103-AC-04, AC-12): members who are not ready count as absent
// (left, disconnected); with ≥ party_size.min ready members the first phase starts, otherwise the encounter is
// abandoned without reward (nobody came in: no_ready).
func (e *Encounter) ReadyTimeout(v Version, now time.Time) Effect {
	ef := Effect{}
	if e.Status != StatusLobby || now.Before(e.ReadyDeadline) {
		return ef
	}
	ready := 0
	for i := range e.Members {
		m := &e.Members[i]
		if !m.Present() {
			continue
		}
		if m.Ready {
			ready++
			continue
		}
		m.LeftAt, m.LeftReason = &now, LeftDisconnect
		ef.MembersLeft = append(ef.MembersLeft, m.StudentID)
		ef.MembersChange = true
	}
	if ready > 0 && ready >= e.sizeMin(v) {
		e.begin(v, now, &ef)
		return ef
	}
	e.end(OutcomeAbandoned, EndNoReady, now)
	ef.Ended = true
	return ef
}

// Round is the progress of the current round of a phase: for every member and requirement, the items of its plan and
// the correct answers counted (loaded from the plans and answer results of the round).
type Round struct {
	Items   map[uuid.UUID]map[string]int
	Correct map[uuid.UUID]map[string]int
}

func (r Round) correctOf(s uuid.UUID) int {
	n := 0
	for _, c := range r.Correct[s] {
		n += c
	}
	return n
}

func (r Round) done(s uuid.UUID, req Requirement) bool {
	return r.Correct[s][req.RequirementID] >= req.Need(r.Items[s][req.RequirementID])
}

// Progress is the shared progress of the phase in [0, 1] (never per member, F103-AC-09).
func (e *Encounter) Progress(v Version, r Round) float64 {
	p, ok := v.Phase(e.PhaseIndex)
	if !ok || e.Status != StatusPhaseActive {
		return 0
	}
	present := e.PresentMembers()
	switch p.CompletionRule.Type {
	case RuleCollectiveCount:
		total := 0
		for _, m := range e.Members {
			total += r.correctOf(m.StudentID)
		}
		return min(1, float64(total)/float64(max(1, p.CompletionRule.CollectiveTarget)))
	case RuleAnyN:
		return min(1, float64(doneReqs(p, present, r))/float64(max(1, p.CompletionRule.N)))
	default:
		all, done := len(present)*len(p.Requirements), 0
		for _, m := range present {
			for _, q := range p.Requirements {
				if r.done(m.StudentID, q) {
					done++
				}
			}
		}
		if all == 0 {
			return 0
		}
		return float64(done) / float64(all)
	}
}

func doneReqs(p Phase, present []Member, r Round) int {
	n := 0
	for _, q := range p.Requirements {
		for _, m := range present {
			if r.done(m.StudentID, q) {
				n++
				break
			}
		}
	}
	return n
}

// Met reports the completion rule of the current phase met (F103-AC-07): all (every present member completed every
// requirement), any_n (n requirements completed by someone) or collective_count (the counted correct answers of the
// members reach collective_target and every present member reached min_contribution).
func (e *Encounter) Met(v Version, r Round) bool {
	p, ok := v.Phase(e.PhaseIndex)
	present := e.PresentMembers()
	if !ok || e.Status != StatusPhaseActive || len(present) == 0 {
		return false
	}
	switch p.CompletionRule.Type {
	case RuleCollectiveCount:
		total := 0
		for _, m := range e.Members {
			total += r.correctOf(m.StudentID)
		}
		if total < p.CompletionRule.CollectiveTarget {
			return false
		}
		for _, m := range present {
			if r.correctOf(m.StudentID) < p.CompletionRule.MinContribution {
				return false
			}
		}
		return true
	case RuleAnyN:
		return doneReqs(p, present, r) >= p.CompletionRule.N
	default:
		for _, m := range present {
			for _, q := range p.Requirements {
				if !r.done(m.StudentID, q) {
					return false
				}
			}
		}
		return true
	}
}

// ContributionMet reports a member reaching min_contribution in the current round (the only per-member signal the
// other members see, F103-AC-09).
func (e *Encounter) ContributionMet(v Version, r Round, student uuid.UUID) bool {
	p, ok := v.Phase(e.PhaseIndex)
	return ok && r.correctOf(student) >= p.CompletionRule.MinContributionOrDefault()
}

// RecordCorrect counts one correct answer of a member in the current round (a wrong answer counts nothing and takes
// nothing away, §3.5). The contribution row is the per-phase view of the round.
func (e *Encounter) RecordCorrect(v Version, r Round, student uuid.UUID) {
	c := e.Contribution(student, e.PhaseID)
	c.Correct = r.correctOf(student)
	p, _ := v.Phase(e.PhaseIndex)
	if t := p.CompletionRule.CollectiveTarget; t > 0 {
		c.Pct = min(100, float64(c.Correct)*100/float64(t))
	}
	e.setContribution(c)
	e.bump()
}

// Evaluate clears the current phase when its rule is met (F103-AC-07): the energy drops by energy_share, the present
// members get the eligibility of the phase (min_contribution), the next phase starts — or the boss is defeated.
func (e *Encounter) Evaluate(v Version, r Round, now time.Time) Effect {
	ef := Effect{}
	if !e.Met(v, r) {
		return ef
	}
	p, _ := v.Phase(e.PhaseIndex)
	need := p.CompletionRule.MinContributionOrDefault()
	for _, m := range e.PresentMembers() {
		c := e.Contribution(m.StudentID, p.PhaseID)
		c.Correct = r.correctOf(m.StudentID)
		c.Activities = 0
		for _, q := range p.Requirements {
			if r.done(m.StudentID, q) {
				c.Activities++
			}
		}
		c.Eligible, c.Cleared = c.Correct >= need, true
		e.setContribution(c)
	}
	e.Energy = max(0, e.Energy-p.EnergyShare)
	if e.Energy < 1e-6 {
		e.Energy = 0
	}
	e.PhasesCleared++
	ef.PhaseCleared, ef.ClearedIndex = p.PhaseID, e.PhaseIndex
	if e.PhaseIndex+1 >= len(v.Phases) {
		e.Energy = 0
		e.end(OutcomeDefeated, EndDefeated, now)
		ef.Ended = true
		return ef
	}
	e.PhaseIndex++
	e.Retries = 0
	e.startPhase(v, now)
	ef.PhaseStarted = true
	return ef
}

// Timeout applies the server clock (F103-AC-08): past the total deadline the encounter ends (time_up); past the phase
// deadline the phase starts again with new questions (retry_phase, previous phases kept) until max_phase_retries, or
// the encounter ends (end_encounter, retries exhausted) with the phases cleared so far.
func (e *Encounter) Timeout(v Version, now time.Time) Effect {
	ef := Effect{}
	if e.Status != StatusPhaseActive {
		return ef
	}
	if e.Deadline != nil && !now.Before(*e.Deadline) {
		e.end(OutcomeTimeUp, EndTotalTime, now)
		ef.Ended = true
		return ef
	}
	if e.PhaseDeadline == nil || now.Before(*e.PhaseDeadline) {
		return ef
	}
	p, _ := v.Phase(e.PhaseIndex)
	if p.FailPolicy() == OnFailEnd {
		e.end(OutcomeTimeUp, EndPhaseFailed, now)
		ef.Ended = true
		return ef
	}
	if e.Retries+1 > v.MaxRetries() {
		e.end(OutcomeTimeUp, EndRetries, now)
		ef.Ended = true
		return ef
	}
	e.Retries++
	// The round starts again: the contribution of the phase restarts with the new questions (previous phases kept).
	for _, m := range e.Members {
		c := e.Contribution(m.StudentID, p.PhaseID)
		if c.Correct != 0 || c.Activities != 0 {
			c.Correct, c.Activities, c.Pct = 0, 0, 0
			e.setContribution(c)
		}
	}
	e.startPhase(v, now)
	ef.PhaseStarted, ef.Retry = true, true
	return ef
}

// Leave removes a member (leaveBossEncounter, party.member_left/kicked/disbanded, past the reconnect grace — F103-AC-11,
// AC-12): its contributions and learning records stay. Below party_size.min present members the encounter is
// abandoned without reward; a lobby nobody is left in is abandoned too.
func (e *Encounter) Leave(v Version, student uuid.UUID, reason string, now time.Time) (Effect, error) {
	m, ok := e.Member(student)
	if !ok {
		return Effect{}, ErrNotMember
	}
	if !m.Present() || !e.Live() {
		return Effect{}, nil // idempotent
	}
	m.LeftAt, m.LeftReason = &now, reason
	e.bump()
	ef := Effect{MembersLeft: []uuid.UUID{student}, MembersChange: true}
	present := len(e.PresentMembers())
	switch {
	case present == 0:
		e.end(OutcomeAbandoned, EndBelowMin, now)
		ef.Ended = true
	case e.Status == StatusPhaseActive && present < e.sizeMin(v):
		e.end(OutcomeAbandoned, EndBelowMin, now)
		ef.Ended = true
	case e.Status == StatusLobby && e.allReady() && present >= e.sizeMin(v):
		e.begin(v, now, &ef)
	}
	return ef, nil
}

// Presence applies the online state of the present members (F103-AC-11): a member offline is held for the grace
// (disconnected_at), one back online is restored, one offline past the grace leaves (disconnected) with its
// contribution kept — which may abandon the encounter below party_size.min.
func (e *Encounter) Presence(v Version, online map[uuid.UUID]bool, grace time.Duration, now time.Time) Effect {
	ef := Effect{}
	if !e.Live() {
		return ef
	}
	var expired []uuid.UUID
	for i := range e.Members {
		m := &e.Members[i]
		if !m.Present() {
			continue
		}
		switch {
		case online[m.StudentID] && m.DisconnectedAt != nil:
			m.DisconnectedAt = nil
			ef.MembersChange = true
		case !online[m.StudentID] && m.DisconnectedAt == nil:
			at := now
			m.DisconnectedAt = &at
			ef.MembersChange = true
		case !online[m.StudentID] && now.Sub(*m.DisconnectedAt) >= grace:
			expired = append(expired, m.StudentID)
		}
	}
	if ef.MembersChange {
		e.bump()
	}
	for _, s := range expired {
		le, _ := e.Leave(v, s, LeftDisconnect, now)
		ef.MembersLeft = append(ef.MembersLeft, le.MembersLeft...)
		ef.MembersChange = true
		ef.Ended = ef.Ended || le.Ended
		ef.Started = ef.Started || le.Started
		ef.PhaseStarted = ef.PhaseStarted || le.PhaseStarted
		if ef.Ended {
			break
		}
	}
	return ef
}

// PlanFailed counts a failed creation of the question plans of a round (edge case: retried idempotently; past
// maxFailures the encounter is abandoned without reward and without counting an attempt of the students).
func (e *Encounter) PlanFailed(maxFailures int, now time.Time) Effect {
	if !e.Live() {
		return Effect{}
	}
	e.PlanFailures++
	e.bump()
	if e.PlanFailures > maxFailures {
		e.end(OutcomeAbandoned, EndPlanFailure, now)
		return Effect{Ended: true}
	}
	return Effect{}
}

// Abandon ends a live encounter without reward (recovery impossible, student unavailable).
func (e *Encounter) Abandon(reason string, now time.Time) Effect {
	if !e.Live() {
		return Effect{}
	}
	e.end(OutcomeAbandoned, reason, now)
	return Effect{Ended: true}
}

// end fixes the result (F103-AC-10): eligible = the boss is defeated, the member took part in at least one cleared
// phase and reached min_contribution in every cleared phase it took part in.
func (e *Encounter) end(outcome, reason string, now time.Time) {
	switch outcome {
	case OutcomeDefeated:
		e.Status = StatusDefeated
	default:
		e.Status = StatusEnded
	}
	e.Outcome, e.EndReason, e.EndedAt, e.PhaseDeadline = outcome, reason, &now, nil
	for i := range e.Members {
		m := &e.Members[i]
		ok, took := outcome == OutcomeDefeated, false
		for _, c := range e.Contributions {
			if c.StudentID != m.StudentID || !c.Cleared {
				continue
			}
			took = true
			ok = ok && c.Eligible
		}
		el := ok && took
		m.Eligible = &el
	}
	e.bump()
}

// Result is defeated or ended (event boss.encounter_completed result).
func (e *Encounter) Result() string {
	if e.Outcome == OutcomeDefeated {
		return "defeated"
	}
	return "ended"
}

// NextDue is the next server time the room of the encounter must be checked: the ready deadline in the lobby, the
// phase (or total) deadline while a phase runs, never later than every (presence checks of the party).
func (e *Encounter) NextDue(every time.Duration, now time.Time) time.Time {
	due := now.Add(every)
	pick := func(t *time.Time) {
		if t != nil && t.Before(due) {
			due = *t
		}
	}
	switch e.Status {
	case StatusLobby:
		pick(&e.ReadyDeadline)
	case StatusPhaseActive:
		pick(e.PhaseDeadline)
		pick(e.Deadline)
	}
	return due
}
