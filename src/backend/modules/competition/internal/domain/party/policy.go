package party

import (
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/hub"
)

// Subject is what the policies know of a student (projections ref.students, ref.sanctions): never sent to another
// student (R16). School and grade level come from the student itself (ADR-0095: no class, no enrollment).
type Subject struct {
	ID         uuid.UUID
	SchoolID   uuid.UUID
	Grade      int    // grade level 1–9; 0 when unknown
	Band       string // grade band (g1_3…); "" when the grade is unknown
	Active     bool   // projected, active, not cleaned up
	Restricted bool   // restrict_multiplayer (or account_lock) in force
}

// BandOf is the grade band of a grade level (g1_3, g4_5, g6_7, g8_9).
func BandOf(grade int) string {
	b, _ := hub.BandOf(grade)
	return b
}

// Rules are the effective party policies (PARTY_SYSTEM §8 R7): platform defaults tightened by the school
// party_policies row; a lower level only tightens.
type Rules struct {
	AllowedTypes   []string
	SameGradeOnly  bool // every member in the grade level of the reference student (always for grades 1–3)
	CrossGradeBand bool
	MaxMembersCap  int
}

// PlatformRules are the platform party_policies of a grade band [Confirmed ADR-0095 D2, D7]: grades 1–3 create
// friend/quest parties whose members are all in the same grade level (R12; another school only inside the tenant under
// ADR-0054); study_party and custom from grade 6 (PARTY_SYSTEM §4); competition_team from grade 4; one grade band per
// party; max_members_cap = capacity (platform 8).
func PlatformRules(band string, capacity int) Rules {
	r := Rules{CrossGradeBand: false, MaxMembersCap: capacity}
	switch band {
	case hub.BandG1to3:
		r.AllowedTypes = []string{TypeFriend, TypeQuest}
		r.SameGradeOnly = true
	case "g4_5":
		r.AllowedTypes = []string{TypeFriend, TypeQuest, TypeCompetitionTeam}
	default:
		r.AllowedTypes = []string{TypeFriend, TypeStudyParty, TypeQuest, TypeCompetitionTeam, TypeCustom}
	}
	return r
}

// Override is the rules object of a party_policies row (nil: not set at that level). A legacy cross_class key is
// ignored (ADR-0095).
type Override struct {
	AllowedTypes   []string `json:"allowed_party_types,omitempty"`
	SameGradeOnly  *bool    `json:"same_grade_only,omitempty"`
	CrossGradeBand *bool    `json:"cross_grade_band,omitempty"`
	MaxMembersCap  *int     `json:"max_members_cap,omitempty"`
}

// Tighten applies a lower level: allowed types intersect, cross_grade_band only turns off, same_grade_only only turns
// on, the cap only decreases.
func (r Rules) Tighten(o Override) Rules {
	out := r
	if o.AllowedTypes != nil {
		out.AllowedTypes = nil
		for _, t := range r.AllowedTypes {
			if slices.Contains(o.AllowedTypes, t) {
				out.AllowedTypes = append(out.AllowedTypes, t)
			}
		}
	}
	if o.SameGradeOnly != nil && *o.SameGradeOnly {
		out.SameGradeOnly = true
	}
	if o.CrossGradeBand != nil && !*o.CrossGradeBand {
		out.CrossGradeBand = false
	}
	if o.MaxMembersCap != nil && *o.MaxMembersCap < out.MaxMembersCap {
		out.MaxMembersCap = max(*o.MaxMembersCap, MinMembers)
	}
	return out
}

// Allows reports a party type allowed by the rules.
func (r Rules) Allows(typ string) bool { return slices.Contains(r.AllowedTypes, typ) }

// Scope is the tenant side of the policies: cross-school only in a unit tenant with cross_school_social_enabled
// (ADR-0054).
type Scope struct{ CrossSchool bool }

// CheckCreate checks createParty (F100-AC-04, AC-25): a restricted student creates nothing; the type must be allowed
// for the grade band and max_members within the cap.
func CheckCreate(r Rules, creator Subject, typ string, maxMembers int) error {
	if creator.Restricted {
		return PolicyError{Rule: RuleSanction}
	}
	if !ValidType(typ) || !r.Allows(typ) {
		return ValidationError{Field: "party_type", Reason: "INVALID", Message: "not allowed for this grade"}
	}
	if maxMembers > r.MaxMembersCap {
		return ValidationError{Field: "max_members", Reason: "INVALID", Message: "above the limit of the school policy"}
	}
	return nil
}

// pair checks school, grade band and grade level between the reference student (inviter, leader) and another student
// of the same tenant (the Subject lookup is tenant-scoped: another tenant is RuleTenant before pair — R8, ADR-0005).
func pair(r Rules, sc Scope, band string, ref, other Subject) error {
	if ref.SchoolID != other.SchoolID && !sc.CrossSchool {
		return PolicyError{Rule: RuleSchool}
	}
	if other.Band != band && !r.CrossGradeBand {
		return PolicyError{Rule: RuleGradeBand}
	}
	if r.SameGradeOnly && (ref.Grade == 0 || other.Grade != ref.Grade) {
		return PolicyError{Rule: RuleGrade}
	}
	return nil
}

// InviteCheck is the evaluation of inviteToParty (F100-AC-08, AC-09) and its re-check at accept (AC-11).
type InviteCheck struct {
	Party   *Party
	Inviter Subject
	Invitee Subject
	Friends bool // inviter and invitee are friends (ref.friendships)
	Blocked bool // a block between the invitee and any active member, either direction
	Kicked  bool // the invitee was kicked from this party within party_kick_reinvite_block
	Rules   Rules
	Scope   Scope
}

// Evaluate returns nil, ErrInviteNotAllowed (never says why) or a PolicyError.
func (c InviteCheck) Evaluate() error {
	if c.Invitee.ID == c.Inviter.ID {
		return ErrSelf
	}
	if err := c.Party.CanInvite(c.Inviter.ID); err != nil {
		return err
	}
	if !c.Invitee.Active {
		return PolicyError{Rule: RuleTenant}
	}
	if c.Blocked {
		return ErrInviteNotAllowed
	}
	if c.Invitee.Restricted || c.Inviter.Restricted {
		return PolicyError{Rule: RuleSanction}
	}
	if err := pair(c.Rules, c.Scope, c.Party.GradeBand, c.Inviter, c.Invitee); err != nil {
		return err
	}
	if c.Party.Type == TypeFriend && !c.Friends {
		return PolicyError{Rule: RuleFriendOnly}
	}
	if c.Kicked {
		return PolicyError{Rule: RuleKickBlock}
	}
	return nil
}

// AcceptCheck re-checks an invitation at accept time against the current state (F100-AC-11): the inviter as the
// reference while still a member, else the leader.
func (c InviteCheck) AcceptCheck() error {
	if !c.Party.Active() {
		return ErrDisbanded
	}
	if c.Blocked {
		return ErrInviteNotAllowed
	}
	if c.Invitee.Restricted {
		return PolicyError{Rule: RuleSanction}
	}
	if err := pair(c.Rules, c.Scope, c.Party.GradeBand, c.Inviter, c.Invitee); err != nil {
		return err
	}
	if c.Party.Type == TypeFriend && !c.Friends {
		return PolicyError{Rule: RuleFriendOnly}
	}
	if c.Kicked {
		return PolicyError{Rule: RuleKickBlock}
	}
	return nil
}

// JoinCheck is the evaluation of joinParty (F100-AC-16).
type JoinCheck struct {
	Party          *Party
	Leader         Subject
	Joiner         Subject
	FriendOfLeader bool
	Blocked        bool
	Kicked         bool
	Invited        bool // an accepted invitation of this party, not expired
	Rules          Rules
	Scope          Scope
}

// Evaluate returns nil, ErrAlreadyMember (idempotent join), ErrDisbanded, ErrInviteNotAllowed or a PolicyError.
func (c JoinCheck) Evaluate() error {
	if !c.Party.Active() {
		return ErrDisbanded
	}
	if _, ok := c.Party.Member(c.Joiner.ID); ok {
		return ErrAlreadyMember
	}
	if c.Joiner.Restricted {
		return PolicyError{Rule: RuleSanction}
	}
	if c.Blocked {
		return ErrInviteNotAllowed
	}
	switch c.Party.Visibility {
	case VisibilityPrivate:
		if !c.Invited {
			return PolicyError{Rule: RulePrivate}
		}
	case VisibilityFriends:
		if !c.FriendOfLeader {
			return PolicyError{Rule: RuleVisibility}
		}
	default: // no other circle (the class visibility is gone, ADR-0095)
		return PolicyError{Rule: RuleVisibility}
	}
	if err := pair(c.Rules, c.Scope, c.Party.GradeBand, c.Leader, c.Joiner); err != nil {
		return err
	}
	if c.Kicked {
		return PolicyError{Rule: RuleKickBlock}
	}
	return nil
}
