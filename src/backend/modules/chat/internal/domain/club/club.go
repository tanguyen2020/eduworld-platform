// Package club is the club aggregate of module chat (F120 US4, SOCIAL_SYSTEM §6, ADR-0050): a lasting group of students
// of a school (several schools only in a tenant unit that enabled it, ADR-0054) with a fixed team chat channel. Pure Go:
// no I/O, no grade condition on chat (R15) — the grade 4–9 rule of student creation is a creation rule only.
package club

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Type is club_type (SOCIAL_SYSTEM §6.1; no study_group since ADR-0084/ADR-0095).
type Type string

// Club types.
const (
	SchoolClub      Type = "school_club"
	SubjectClub     Type = "subject_club"
	CompetitionTeam Type = "competition_team"
	InterestGroup   Type = "interest_group"
)

// Types lists the four club types.
var Types = []Type{SchoolClub, SubjectClub, CompetitionTeam, InterestGroup}

// Valid reports one of the four types.
func (t Type) Valid() bool { return slices.Contains(Types, t) }

// StudentCreatable reports the types a student may propose (SOCIAL_SYSTEM §6.1: competition_team, interest_group).
func (t Type) StudentCreatable() bool { return t == CompetitionTeam || t == InterestGroup }

// Statuses of a club.
const (
	StatusPending  = "pending_approval"
	StatusActive   = "active"
	StatusLocked   = "locked"
	StatusArchived = "archived"
)

// Roles of a member.
const (
	RoleOwner   = "owner"
	RoleOfficer = "officer"
	RoleMember  = "member"
)

// Creation is the school policy student_club_creation (SOCIAL_SYSTEM §6.1, RB-511).
type Creation string

// Creation policies.
const (
	CreationDisabled         Creation = "disabled"
	CreationRequiresApproval Creation = "requires_approval"
	CreationAuto             Creation = "auto"
)

// ParseCreation reads the projected value: the three values of SOCIAL_SYSTEM, or the boolean school.updated carries today
// (true → requires_approval, false → disabled — [Proposed], F120 Clarifications 2026-11-23). Anything else: disabled.
func ParseCreation(v any) Creation {
	switch x := v.(type) {
	case string:
		switch c := Creation(x); c {
		case CreationRequiresApproval, CreationAuto:
			return c
		}
	case bool:
		if x {
			return CreationRequiresApproval
		}
	}
	return CreationDisabled
}

// Errors of the aggregate (mapped to the contract by the application).
var (
	ErrType         = errors.New("club: club_type outside the four types")
	ErrName         = errors.New("club: name must be 2..64 characters")
	ErrDisabled     = errors.New("club: student club creation is disabled for the school")
	ErrStudentType  = errors.New("club: students may only create competition_team or interest_group")
	ErrNotApproved  = errors.New("club: not approved")
	ErrNotActive    = errors.New("club: not active")
	ErrFull         = errors.New("club: max_members reached")
	ErrLimitReached = errors.New("club: clubs_per_student_max reached")
	ErrOwnerLeaves  = errors.New("club: the owner cannot leave")
)

// Limits are club_max_members / clubs_per_student_max (CONFIG_DEFAULTS, ADR-0050).
type Limits struct {
	MaxMembers int
	PerStudent int
	InviteTTL  time.Duration
}

// Club is one club.
type Club struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	SchoolID      *uuid.UUID
	Type          Type
	Name          string
	Description   *string
	IconID        *string
	SubjectID     *uuid.UUID
	OwnerID       uuid.UUID
	CreatedByKind string // student | staff
	Status        string
	MaxMembers    int
	MemberCount   int
	ChannelID     *uuid.UUID
	Version       int64
	CreatedAt     time.Time
}

// CheckName validates a name (2..64 characters, trimmed).
func CheckName(name string) (string, error) {
	n := strings.TrimSpace(name)
	if c := utf8.RuneCountInString(n); c < 2 || c > 64 {
		return "", ErrName
	}
	return n, nil
}

// InitialStatus is the status of a new club: a staff club is active at once; a student club follows the school policy
// (disabled → ErrDisabled, requires_approval → pending_approval, auto → active).
func InitialStatus(byStaff bool, t Type, c Creation) (string, error) {
	if !t.Valid() {
		return "", ErrType
	}
	if byStaff {
		return StatusActive, nil
	}
	if !t.StudentCreatable() {
		return "", ErrStudentType
	}
	switch c {
	case CreationAuto:
		return StatusActive, nil
	case CreationRequiresApproval:
		return StatusPending, nil
	}
	return "", ErrDisabled
}

// Usable reports a club members can be added to (active); pending → ErrNotApproved, locked/archived → ErrNotActive.
func (c Club) Usable() error {
	switch c.Status {
	case StatusActive:
		return nil
	case StatusPending:
		return ErrNotApproved
	}
	return ErrNotActive
}

// CanAdd checks the member cap before adding one member (in the transaction, club row locked).
func (c Club) CanAdd() error {
	if err := c.Usable(); err != nil {
		return err
	}
	if c.MemberCount >= c.MaxMembers {
		return ErrFull
	}
	return nil
}

// Manager reports a role that handles the join requests and invitations of a club (owner, officer — SOCIAL_SYSTEM §6.2).
func Manager(role string) bool { return role == RoleOwner || role == RoleOfficer }

// Editor reports the role that edits the club, names officers and creates goals and events: the owner only
// (SOCIAL_SYSTEM §6.2 — an officer approves requests and invites; delta F112-1, 2026-11-28).
func Editor(role string) bool { return role == RoleOwner }

// Kinds of a club event (ClubEventCreate.event_kind).
const (
	EventPractice   = "practice_session"
	EventCoopQuest  = "coop_quest"
	EventBossRaid   = "boss_raid"
	EventTournament = "internal_tournament"
	EventOther      = "other"
)

// Event kind errors: ErrEventKind for a value outside the contract, ErrEventKindUnavailable for internal_tournament (or a
// tournament template) from the game — students have no operation that lists the internal tournament templates
// (delta F112-3, 2026-11-28).
var (
	ErrEventKind            = errors.New("club: event_kind is not a club event kind")
	ErrEventKindUnavailable = errors.New("club: internal_tournament is not available from the game")
)

// StudentEventKind checks the kind of an event a student creates ("" = other) and returns it.
func StudentEventKind(kind string, template *uuid.UUID) (string, error) {
	if kind == "" {
		kind = EventOther
	}
	switch kind {
	case EventPractice, EventCoopQuest, EventBossRaid, EventOther:
		if template != nil {
			return "", ErrEventKindUnavailable
		}
		return kind, nil
	case EventTournament:
		return "", ErrEventKindUnavailable
	}
	return "", ErrEventKind
}

// WeekStart is the Monday (policy day in loc) of the week of t: the week of a club goal.
func WeekStart(t time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	l := t.In(loc)
	d := (int(l.Weekday()) + 6) % 7
	return time.Date(l.Year(), l.Month(), l.Day()-d, 0, 0, 0, 0, time.UTC)
}

// Metrics of a club goal (counted from verified events, never a score or mastery value — F120-AC-21).
const (
	MetricActivities  = "activities_completed"
	MetricQuests      = "quests_completed"
	MetricMinutes     = "minutes_learning"
	MetricBoss        = "boss_defeated"
	MetricDiscoveries = "discoveries_found"
)

// Metrics lists the metrics of the contract (ClubGoalCreate.metric).
var Metrics = []string{MetricActivities, MetricQuests, MetricMinutes, MetricBoss, MetricDiscoveries}

// AcceptedMetrics are the goal metrics fed by a verified event today, in contract order (ClubGoalAcceptedMetric, delta
// F112-4; F120-AC-21): minutes_learning waits for activity.graded.duration_seconds (owner F031) and boss_defeated for
// boss.encounter_completed (owner F103) [Proposed].
var AcceptedMetrics = []string{MetricActivities, MetricQuests, MetricDiscoveries}

// Metric errors: ErrMetric outside the contract, ErrMetricUnavailable for a contract metric without a source event yet.
var (
	ErrMetric            = errors.New("club: metric is not a club goal metric")
	ErrMetricUnavailable = errors.New("club: metric not counted from a verified event yet")
)

// CheckMetric checks the metric of a new goal.
func CheckMetric(m string) error {
	switch {
	case slices.Contains(AcceptedMetrics, m):
		return nil
	case slices.Contains(Metrics, m):
		return ErrMetricUnavailable
	}
	return ErrMetric
}
