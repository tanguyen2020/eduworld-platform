// Package tournament holds the rules of the tournaments of module competition (COMPETITION_SYSTEM §10, F107): the
// lifecycle, the formats and their minimum entries, the entry and node states, the timers, the advancement of a bracket
// after a result, a no-show, a forfeit or an override, the group standings and the final ranks. Pure Go, no I/O.
package tournament

import (
	"errors"
	"slices"
	"time"
)

// Tournament statuses (COMPETITION_SYSTEM §10.3).
const (
	StatusDraft              = "draft"
	StatusPublished          = "published"
	StatusRegistrationOpen   = "registration_open"
	StatusRegistrationClosed = "registration_closed"
	StatusSeeding            = "seeding"
	StatusScheduled          = "scheduled"
	StatusInProgress         = "in_progress"
	StatusCompleted          = "completed"
	StatusRewarded           = "rewarded"
	StatusArchived           = "archived"
	StatusCancelled          = "cancelled"
)

// Statuses lists the tournament statuses.
var Statuses = []string{StatusDraft, StatusPublished, StatusRegistrationOpen, StatusRegistrationClosed, StatusSeeding, StatusScheduled,
	StatusInProgress, StatusCompleted, StatusRewarded, StatusArchived, StatusCancelled}

// transitions is the lifecycle of §10.3 (F107-AC-07): forward steps by command or job; cancel from draft, published,
// registration_open and scheduled only.
var transitions = map[string][]string{
	StatusDraft:              {StatusPublished, StatusCancelled},
	StatusPublished:          {StatusRegistrationOpen, StatusCancelled},
	StatusRegistrationOpen:   {StatusRegistrationClosed, StatusCancelled},
	StatusRegistrationClosed: {StatusSeeding},
	StatusSeeding:            {StatusScheduled},
	StatusScheduled:          {StatusInProgress, StatusCancelled},
	StatusInProgress:         {StatusCompleted},
	StatusCompleted:          {StatusRewarded},
	StatusRewarded:           {StatusArchived},
}

// ErrInvalidTransition: the move is not in the lifecycle (422 INVALID_STATE_TRANSITION).
var ErrInvalidTransition = errors.New("tournament: invalid state transition")

// CanMove reports whether a tournament may go from → to.
func CanMove(from, to string) bool { return slices.Contains(transitions[from], to) }

// Move returns ErrInvalidTransition unless from → to is in the lifecycle.
func Move(from, to string) error {
	if !CanMove(from, to) {
		return ErrInvalidTransition
	}
	return nil
}

// Cancellable reports whether a tournament in status s may be cancelled (draft, published, registration_open, scheduled).
func Cancellable(s string) bool { return CanMove(s, StatusCancelled) }

// Formats (§10.1, §10.5): Team, School and Grade Tournament are the participant type and the scope of these four.
const (
	FormatSingleElim = "single_elim"
	FormatDoubleElim = "double_elim"
	FormatRoundRobin = "round_robin"
	FormatGroups     = "groups_then_single_elim"
)

// Formats lists the bracket formats.
var Formats = []string{FormatSingleElim, FormatDoubleElim, FormatRoundRobin, FormatGroups}

// MinEntries is the smallest number of entries of a format (§10.4): single and double elimination 4, round robin 3,
// groups 8 (two groups of 4).
func MinEntries(format string) int {
	switch format {
	case FormatRoundRobin:
		return 3
	case FormatGroups:
		return 8
	default:
		return 4
	}
}

// Participant types and owner scopes.
const (
	ParticipantIndividual = "individual"
	ParticipantTeam       = "team"
	ScopeSchool           = "school"
	ScopeTenant           = "tenant"
	ScopePlatform         = "platform"
)

// Entry statuses.
const (
	EntryRegistered   = "registered"
	EntryConfirmed    = "confirmed"
	EntryCheckedIn    = "checked_in"
	EntryWithdrawn    = "withdrawn"
	EntryDisqualified = "disqualified"
	EntryEliminated   = "eliminated"
	EntryChampion     = "champion"
)

// ActiveEntry reports an entry still in the tournament.
func ActiveEntry(s string) bool {
	return s == EntryRegistered || s == EntryConfirmed || s == EntryCheckedIn || s == EntryEliminated || s == EntryChampion
}

// Locked reports whether the bracket of a tournament in status s is locked (entries can no longer withdraw).
func Locked(s string) bool {
	switch s {
	case StatusInProgress, StatusCompleted, StatusRewarded, StatusArchived, StatusCancelled:
		return true
	}
	return false
}

// Timers is tournament_timers (CONFIG_DEFAULTS, COMPETITION_SYSTEM §10.4, §10.7): check-in window before starts_at,
// no-show limit after the match time, complaint window after a result, the delay of a reschedule after a server failure
// and the number of reschedules before the organizer decides.
type Timers struct {
	CheckIn       time.Duration
	NoShow        time.Duration
	Complaint     time.Duration
	Reschedule    time.Duration
	RescheduleMax int
}

// Valid reports usable timers.
func (t Timers) Valid() bool {
	return t.CheckIn > 0 && t.NoShow > 0 && t.Complaint > 0 && t.Reschedule > 0 && t.RescheduleMax >= 0 && t.RescheduleMax <= 2
}

// Notice minutes of tournament.match_scheduled (§10.6) and their bit in bracket_nodes.notices.
var Notices = []struct {
	Minutes int
	Bit     int
}{{10, 1}, {1, 2}}
