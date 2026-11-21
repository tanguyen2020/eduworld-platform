package party

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Party activity types: the single enum PartyActivityType (specs/schemas/common/enums.schema.json, F101 research R4).
const (
	ActivityExplore           = "explore"
	ActivityQuest             = "quest"
	ActivityBoss              = "boss"
	ActivityLearningChallenge = "learning_challenge"
	ActivityCoopChallenge     = "coop_challenge"
	ActivityTeamBattle        = "team_battle"
)

var activityTypes = map[string]bool{ActivityExplore: true, ActivityQuest: true, ActivityBoss: true, ActivityLearningChallenge: true,
	ActivityCoopChallenge: true, ActivityTeamBattle: true}

// ValidActivityType reports a value of PartyActivityType.
func ValidActivityType(t string) bool { return activityTypes[t] }

// EnabledActivityType reports the activity types of Release M4a (explore, quest, boss); the others come with F104/F105.
func EnabledActivityType(t string) bool {
	return t == ActivityExplore || t == ActivityQuest || t == ActivityBoss
}

// FromGame reports the types whose eligibility and session come from module game (facade game, research R1).
func FromGame(t string) bool { return t == ActivityQuest || t == ActivityBoss }

// Room types of a party session.
const (
	RoomPrivateInstance = "private_instance"
	RoomBoss            = "boss"
)

// Party activity status.
const (
	ActivityRunning = "running"
	ActivityEnded   = "ended"
)

// End reasons of a party activity (PARTY_SYSTEM §6.3; leader_ended has no operation in Release 1.0).
const (
	EndCompleted     = "completed"
	EndTimeout       = "timeout"
	EndAllLeft       = "all_left"
	EndDisbanded     = "disbanded"
	EndServerFailure = "server_failure"
)

var endReasons = map[string]string{EndCompleted: "completed", EndTimeout: "abandoned", EndAllLeft: "abandoned", EndDisbanded: "aborted",
	EndServerFailure: "failed"}

// Outcome is the outcome of party.activity_ended / party.session.ended for an end reason.
func Outcome(reason string) string { return endReasons[reason] }

// Reason codes of a member not eligible for an activity (never the grade or class of the student, R16).
const (
	ReasonGradeNotAllowed     = "grade_not_allowed"
	ReasonGradeBandMismatch   = "grade_band_mismatch"
	ReasonPrerequisiteMissing = "prerequisite_missing"
	ReasonAreaLocked          = "area_locked"
	ReasonAttemptLimit        = "attempt_limit"
	ReasonSanctioned          = "sanctioned"
	ReasonNotOnline           = "not_online"
)

// Errors of the lobby and the party session (mapped to the contract by the application).
var (
	ErrNotOpen          = errors.New("party: not open")                  // INVALID_STATE_TRANSITION
	ErrVersion          = errors.New("party: state_version mismatch")    // VERSION_MISMATCH
	ErrNoSelection      = errors.New("party: no activity selected")      // INVALID_STATE_TRANSITION
	ErrNotReady         = errors.New("party: members not ready")         // PARTY_NOT_READY
	ErrActivityEnded    = errors.New("party: activity already ended")    // idempotent end
	ErrActivityDisabled = errors.New("party: activity type not enabled") // VALIDATION_FAILED
)

// Selection is the activity chosen by the leader in the Party Lobby.
type Selection struct {
	Type     string
	Ref      string
	GameMode string
}

// Check is the eligibility of one member for an activity: a reason code and the requirement of the activity.
type Check struct {
	StudentID   uuid.UUID
	Eligible    bool
	Reason      string
	Requirement string
}

// Eligibility is the result of an eligibility check of an activity for the members of a party.
type Eligibility struct {
	MinMembers    int
	MaxMembers    int
	AllowContinue bool
	AreaID        string
	Members       []Check
}

// Ineligible returns the members that are not eligible.
func (e Eligibility) Ineligible() []Check {
	var out []Check
	for _, c := range e.Members {
		if !c.Eligible {
			out = append(out, c)
		}
	}
	return out
}

// Select records the activity chosen by the leader (F101-AC-03, AC-04): leader only, party open, expected_version equal
// to state_version; state_version grows by one.
func (p *Party) Select(leader uuid.UUID, s Selection, expected int64, now time.Time) error {
	if err := p.RequireLeader(leader); err != nil {
		return err
	}
	if p.Status != StatusOpen {
		return ErrNotOpen
	}
	if expected != p.StateVersion {
		return ErrVersion
	}
	if !ValidActivityType(s.Type) || !EnabledActivityType(s.Type) {
		return ErrActivityDisabled
	}
	sel := s
	p.Selected = &sel
	p.bump(now)
	return nil
}

// Participants is the outcome of the start check: who plays and who stays out (offline members).
type Participants struct {
	Members []uuid.UUID
	Offline []uuid.UUID
}

// CanStart checks steps (1) and (2) of the start flow (PARTY_SYSTEM §6.1, F101-AC-07, AC-09): the caller is the
// leader, the party is open with an activity selected, expected_version matches, every online member (the leader
// included) is Ready and their number is within [min, max]. Offline members do not play and need not be Ready.
func (p *Party) CanStart(leader uuid.UUID, expected int64, online, ready map[uuid.UUID]bool, minMembers, maxMembers int) (Participants, error) {
	var out Participants
	if err := p.RequireLeader(leader); err != nil {
		return out, err
	}
	if p.Status != StatusOpen || p.Selected == nil {
		if p.Status == StatusOpen {
			return out, ErrNoSelection
		}
		return out, ErrNotOpen
	}
	if expected != p.StateVersion {
		return out, ErrVersion
	}
	for _, m := range p.Members {
		if m.StudentID != leader && !online[m.StudentID] {
			out.Offline = append(out.Offline, m.StudentID)
			continue
		}
		if !ready[m.StudentID] {
			return out, ErrNotReady
		}
		out.Members = append(out.Members, m.StudentID)
	}
	if len(out.Members) < minMembers || len(out.Members) > maxMembers {
		return out, ErrNotReady
	}
	return out, nil
}

// StartActivity moves the party into its session (status in_activity, state_version + 1).
func (p *Party) StartActivity(now time.Time) {
	p.Status = StatusInActivity
	p.bump(now)
}

// BackToLobby moves an active party back to its lobby after its session (status open, state_version + 1); a
// disbanded party stays disbanded.
func (p *Party) BackToLobby(now time.Time) {
	if !p.Active() {
		return
	}
	p.Status = StatusOpen
	p.bump(now)
}

// Activity is a party activity (multiplayer.party_activities): one session of a party in one room.
type Activity struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	SchoolID      uuid.UUID
	PartyID       uuid.UUID
	Type          string
	Ref           string
	GameMode      string
	RoomID        uuid.UUID
	RoomType      string
	ExternalRef   *uuid.UUID
	Status        string
	MinMembers    int
	MaxMembers    int
	AllowContinue bool
	StartedAt     time.Time
	EndedAt       *time.Time
	EndReason     string
	Summary       map[string]any
}

// End ends a running activity once (ErrActivityEnded afterwards: consumers and the sweeper are idempotent).
func (a *Activity) End(reason string, summary map[string]any, now time.Time) error {
	if a.Status != ActivityRunning {
		return ErrActivityEnded
	}
	if _, ok := endReasons[reason]; !ok {
		return ValidationError{Field: "end_reason", Reason: "INVALID", Message: "unknown end reason"}
	}
	a.Status = ActivityEnded
	at := now
	a.EndedAt = &at
	a.EndReason = reason
	a.Summary = CleanSummary(summary)
	return nil
}

// summaryKeys are the only keys of a session summary: shared progress, never a score, a result or a mastery of a
// student (FR-PTY-013, F101-AC-13).
var summaryKeys = map[string]bool{"result": true, "outcome": true, "phases_cleared": true, "steps_completed": true}

// CleanSummary keeps the shared progress keys with scalar values (strings ≤ 64 characters, numbers, booleans).
func CleanSummary(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		if !summaryKeys[k] {
			continue
		}
		switch x := v.(type) {
		case string:
			if len(x) <= 64 {
				out[k] = x
			}
		case bool, int, int64, float64:
			out[k] = x
		}
	}
	return out
}
