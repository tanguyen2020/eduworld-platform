package party

import "errors"

// Domain errors of the party rules; the application maps them to the contract (PARTY_SYSTEM §2 problem codes).
var (
	ErrNotMember        = errors.New("party: not an active member")      // PARTY_NOT_FOUND
	ErrNotLeader        = errors.New("party: not the leader")            // PARTY_NOT_LEADER
	ErrDisbanded        = errors.New("party: disbanded")                 // PARTY_NOT_FOUND
	ErrFull             = errors.New("party: full")                      // PARTY_FULL
	ErrAlreadyMember    = errors.New("party: already a member")          // idempotent accept/join
	ErrInviteNotAllowed = errors.New("party: invitation not allowed")    // PARTY_INVITE_NOT_ALLOWED (never says why)
	ErrTargetNotMember  = errors.New("party: target is not a member")    // 422
	ErrSelf             = errors.New("party: target is the caller")      // 422
	ErrNotPending       = errors.New("party: invitation is not pending") // 404
	ErrExpired          = errors.New("party: invitation expired")        // PARTY_INVITATION_EXPIRED
	ErrNotInvitee       = errors.New("party: caller is not the invitee") // PERMISSION_DENIED
	ErrCannotCancel     = errors.New("party: caller cannot cancel")      // PERMISSION_DENIED
)

// PolicyError is a refusal by the party policies (PARTY_POLICY_VIOLATION, audited); Rule names the rule for the
// audit and the metric eduworld_party_policy_denied_total{rule} — never sent to the client.
type PolicyError struct{ Rule string }

func (e PolicyError) Error() string { return "party: policy violation: " + e.Rule }

// Policy rules (labels of PolicyError).
const (
	RuleTenant     = "tenant"        // invitee unknown in the session tenant (other tenant, not projected, inactive)
	RuleSchool     = "school"        // other school without cross-school social (ADR-0054)
	RuleGradeBand  = "grade_band"    // other grade band while cross_grade_band is off
	RuleGrade      = "grade"         // other grade level while same_grade_only (always for grades 1–3, ADR-0095)
	RuleFriendOnly = "friend_only"   // party_type friend: friends of the inviter only (PARTY_SYSTEM §4)
	RuleSanction   = "sanction"      // restrict_multiplayer (or account_lock) in force
	RuleKickBlock  = "kick_reinvite" // kicked from this party less than party_kick_reinvite_block ago
	RulePrivate    = "private"       // join of a private party without an accepted, unexpired invitation
	RuleVisibility = "visibility"    // join of a friends party by a non-friend of the leader
)

// ValidationError is a 422 VALIDATION_FAILED on one field.
type ValidationError struct {
	Field   string
	Reason  string // sub-code of the problem field (problem.SubInvalid…)
	Message string
}

func (e ValidationError) Error() string { return "party: invalid " + e.Field + ": " + e.Message }
