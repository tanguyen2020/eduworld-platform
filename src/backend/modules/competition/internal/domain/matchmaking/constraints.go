package matchmaking

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// Ticket statuses (MATCHMAKING §4.2).
const (
	StatusQueued    = "queued"
	StatusProposed  = "proposed"
	StatusMatched   = "matched"
	StatusCancelled = "cancelled"
	StatusExpired   = "expired"
)

// Cancel reasons of a ticket (AsyncAPI matchmaking.ticket.updated.cancel_reason).
const (
	CancelUser         = "user"
	CancelPartyChanged = "party_changed"
	CancelTimeout      = "timeout"
	CancelSystem       = "system"
	CancelDeclined     = "declined_proposal"
)

// Scope kinds of a queue and a match (H3, ADR-0054).
const (
	ScopeSchool   = "school"
	ScopeTenant   = "tenant"
	ScopePlatform = "platform"
)

// ValidationError is a 422 on one field of the request.
type ValidationError struct {
	Field, Message string
}

func (e ValidationError) Error() string { return "matchmaking: invalid " + e.Field + ": " + e.Message }

// PolicyError is a 403 POLICY_DENIED: the student (or a member of its party) may not enter this queue.
type PolicyError struct{ Rule string }

func (e PolicyError) Error() string { return "matchmaking: policy denied: " + e.Rule }

// Policy rules (audit reason, never shown to other students).
const (
	RuleGrade       = "grade"            // no grade projected
	RuleModeGrade   = "mode_grade"       // the grade has no such mode (team_battle below grade 4)
	RulePartyGrade  = "party_mixed"      // a ranked party ticket with members of different grades
	RuleRestricted  = "restricted"       // abandon_penalty or restrict_competition in force
	RuleInactive    = "inactive"         // account not active
	RuleHours       = "access_hours"     // outside the allowed hours of the school
	RulePartyType   = "party_type"       // only competition_team parties queue as a team
	RulePartySize   = "party_size"       // party larger than the team, or not filling it without allow_fill
	RuleCrossSchool = "cross_school_off" // a party of several schools in a tenant without cross-school play
)

// Errors of the ready check.
var (
	ErrNotParticipant = errors.New("matchmaking: not a participant")
	ErrProposalClosed = errors.New("matchmaking: proposal closed")
)

// Request is the MatchmakingTicketCreate of the contract.
type Request struct {
	Mode      string
	Type      string
	SubjectID uuid.UUID
	TopicID   *uuid.UUID
	PartyID   *uuid.UUID
	AllowFill bool
}

// CheckRequest validates the request against the configuration before any data is read (422 VALIDATION_FAILED):
// friend (duel only) and tournament (direct proposals only) never enter a queue; the mode must be queueable; the team
// mode pairs with the team type only; topic needs topic_id; party_id only for team tickets.
func (c Config) CheckRequest(r Request) error {
	if !slices.Contains(KnownTypes, r.Type) {
		return ValidationError{Field: "matchmaking_type", Message: "unknown matchmaking type"}
	}
	if r.Type == TypeFriend {
		return ValidationError{Field: "matchmaking_type", Message: "friend matches are duels (createDuel)"}
	}
	if r.Type == TypeTournament {
		return ValidationError{Field: "matchmaking_type", Message: "tournament matches come from the bracket"}
	}
	m, ok := c.Mode(r.Mode)
	if !ok {
		return ValidationError{Field: "mode", Message: "mode has no matchmaking queue"}
	}
	if !slices.Contains(m.Types, r.Type) {
		return ValidationError{Field: "matchmaking_type", Message: "matchmaking type not available for this mode"}
	}
	if r.SubjectID == uuid.Nil {
		return ValidationError{Field: "subject_id", Message: "required"}
	}
	if r.Type == TypeTopic && (r.TopicID == nil || *r.TopicID == uuid.Nil) {
		return ValidationError{Field: "topic_id", Message: "required for topic matchmaking"}
	}
	if r.Type != TypeTopic && r.TopicID != nil {
		return ValidationError{Field: "topic_id", Message: "only for topic matchmaking"}
	}
	if r.PartyID != nil && r.Type != TypeTeam {
		return ValidationError{Field: "party_id", Message: "only for team matchmaking"}
	}
	if r.AllowFill && r.PartyID == nil {
		return ValidationError{Field: "allow_fill", Message: "only for a party ticket"}
	}
	return nil
}

// Player is the server-side view of a student entering a queue: everything comes from the projections and the
// rating reader, never from the request (F104-AC-01).
type Player struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	SchoolID  uuid.UUID
	Grade     int // 0: not projected
	Active    bool
	Restrict  bool // abandon_penalty / restrict_competition / sanction in force
	OffHours  bool // outside the allowed hours of the school
	Rating    Rating
	PartyType string // party tickets: the party type
}

// Eligible checks H5 of one player (account active, not restricted, within the allowed hours).
func Eligible(p Player) error {
	switch {
	case !p.Active:
		return PolicyError{Rule: RuleInactive}
	case p.Restrict:
		return PolicyError{Rule: RuleRestricted}
	case p.OffHours:
		return PolicyError{Rule: RuleHours}
	case p.Grade < 1 || p.Grade > 9:
		return PolicyError{Rule: RuleGrade}
	}
	return nil
}

// CheckPlayers checks H1 and H5 of the players of a ticket (403 POLICY_DENIED) and the subject/topic of their grade
// (422): the grade must have the mode; a party ticket needs a competition_team party of one grade, at most the team
// size, exactly the team size without allow_fill.
func (c Config) CheckPlayers(r Request, ps []Player) error {
	m, _ := c.Mode(r.Mode)
	if len(ps) == 0 {
		return PolicyError{Rule: RuleGrade}
	}
	for _, p := range ps {
		if err := Eligible(p); err != nil {
			return err
		}
		if p.Grade < m.MinGrade {
			return PolicyError{Rule: RuleModeGrade}
		}
	}
	if r.PartyID != nil {
		if ps[0].PartyType != "competition_team" {
			return PolicyError{Rule: RulePartyType}
		}
		for _, p := range ps[1:] {
			if p.Grade != ps[0].Grade {
				return PolicyError{Rule: RulePartyGrade}
			}
		}
		if len(ps) < m.MinTeamSize || len(ps) > m.MaxTeamSize || (!r.AllowFill && len(ps) != c.TeamSize(r, len(ps))) {
			return PolicyError{Rule: RulePartySize}
		}
	}
	subjectOK, topicOK := c.subjectAllowed(ps[0].Grade, r.SubjectID, r.TopicID)
	if !subjectOK {
		return ValidationError{Field: "subject_id", Message: "subject not available for this grade"}
	}
	if !topicOK {
		return ValidationError{Field: "topic_id", Message: "topic not available for this grade"}
	}
	return nil
}

// TeamSize is the team size of a team ticket: the party itself without allow_fill, else the configured size (never
// smaller than the party); 1 for the other modes.
func (c Config) TeamSize(r Request, members int) int {
	if r.Mode != ModeTeamBattle {
		return 1
	}
	m, _ := c.Mode(r.Mode)
	if r.PartyID != nil && !r.AllowFill {
		return members
	}
	return max(m.TeamSize, members)
}

// GradeKey is the grade part of a queue key (H1): g<n> for grades 1–5 and for grade matchmaking (exact grade at every
// level), b6_7 / b8_9 for grades 6–9. Grades 1–5 are never pooled with another grade.
func GradeKey(typ string, grade int) string {
	switch {
	case typ == TypeGrade || grade <= 5:
		return "g" + strconv.Itoa(grade)
	case grade <= 7:
		return "b6_7"
	default:
		return "b8_9"
	}
}

// Scope is the matching scope of a queue (H3): one school, the whole tenant (unit with cross-school play) or the
// platform (direct proposals of platform events only).
type Scope struct {
	Kind string
	ID   uuid.UUID // school or tenant; nil for platform
}

func (s Scope) String() string {
	switch s.Kind {
	case ScopeSchool:
		return "s:" + s.ID.String()
	case ScopeTenant:
		return "t:" + s.ID.String()
	}
	return "p"
}

// QueueKey partitions the queues by every hard constraint (MATCHMAKING §5): mode and type (H4), scope (H3), grade
// key (H1), subject and topic (H2, H7: the language follows the subject) and, for teams, the team size.
type QueueKey struct {
	Mode     string
	Type     string
	Scope    Scope
	GradeKey string
	Subject  uuid.UUID
	Topic    *uuid.UUID
	TeamSize int
}

func (k QueueKey) String() string {
	s := fmt.Sprintf("%s:%s:%s:%s:%s", k.Mode, k.Type, k.Scope, k.GradeKey, k.Subject)
	if k.Topic != nil {
		s += ":" + k.Topic.String()
	}
	if k.Mode == ModeTeamBattle {
		s += ":n" + strconv.Itoa(k.TeamSize)
	}
	return s
}

// GradeCompatible is H1 between two tickets of one queue: same grade, or in grades 6–9 ±Levels after both waited
// AfterS (grade_window_g6_9). Grade matchmaking and grades 1–5 always need the same grade.
func (c Config) GradeCompatible(typ string, ga, gb int, waitA, waitB time.Duration) bool {
	if ga == gb {
		return true
	}
	if typ == TypeGrade || ga <= 5 || gb <= 5 || GradeKey(typ, ga) != GradeKey(typ, gb) {
		return false
	}
	after := time.Duration(c.GradeWindow.AfterS) * time.Second
	return int(math.Abs(float64(ga-gb))) <= c.GradeWindow.Levels && waitA >= after && waitB >= after
}

// TeamRating is the rating of a party ticket (MATCHMAKING §8): μ_team = Σμ/n, RD_team = sqrt(ΣRD²)/n.
func TeamRating(rs []Rating) Rating {
	if len(rs) == 0 {
		return Rating{}
	}
	var mu, rd2 float64
	for _, r := range rs {
		mu += r.Mu
		rd2 += r.RD * r.RD
	}
	n := float64(len(rs))
	return Rating{Mu: mu / n, RD: math.Sqrt(rd2) / n}
}
