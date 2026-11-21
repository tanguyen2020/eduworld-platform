// Package safety is the domain of the Platform Safety Team (F121, MODERATION §5–§7): reports, moderation cases (state
// machine of §6.2, merge window, priority of §6.2, SLA of §6.3), sanctions of §7 (defaults, team-lead rule, refusal of
// a message) and school safety notifications (§6.4). Pure rules: no infrastructure, no content of a message.
package safety

import (
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
)

// Case statuses (MODERATION §6.2).
const (
	StatusOpen     = "open"
	StatusInReview = "in_review"
	StatusEscalate = "escalated"
	StatusResolved = "resolved"
	StatusReopened = "reopened"
)

// Severities (MODERATION §6.2).
const (
	P1 = "p1"
	P2 = "p2"
	P3 = "p3"
)

// Case sources.
const (
	SourceReport        = "report"
	SourceFilterAlert   = "filter_alert"
	SourceAsyncAnalysis = "async_analysis"
	SourceSelfHarm      = "self_harm_signal"
)

// Report reasons (MODERATION §5).
const (
	ReasonRudeLanguage      = "rude_language"
	ReasonBullying          = "bullying"
	ReasonAskedPersonalInfo = "asked_personal_info"
	ReasonSpam              = "spam"
	ReasonInappropriate     = "inappropriate_content"
	ReasonCheating          = "cheating"
	ReasonOther             = "other"
)

// Reasons is the closed list of report reasons.
var Reasons = []string{ReasonRudeLanguage, ReasonBullying, ReasonAskedPersonalInfo, ReasonSpam, ReasonInappropriate,
	ReasonCheating, ReasonOther}

// Target types of a report and a case.
const (
	TargetMessage = "message"
	TargetUser    = "user"
)

// Case actions of actOnModerationCase and the system actions of the history (chat.moderation_actions).
const (
	ActionReport           = "report"
	ActionSignal           = "signal"
	ActionAssign           = "assign"
	ActionWarn             = "warn"
	ActionMute             = "mute"
	ActionRestrict         = "restrict"
	ActionRemoveMessage    = "remove_message"
	ActionEscalate         = "escalate"
	ActionDismiss          = "dismiss"
	ActionClose            = "close"
	ActionReopen           = "reopen"
	ActionSystemEscalate   = "system_escalate"
	ActionHold             = "hold"
	ActionReleaseHold      = "release_hold"
	ActionNotifySchool     = "notify_school"
	ActionSchoolAck        = "school_acknowledged"
	ActionSchoolReminder   = "school_reminder"
	ActionSchoolReminderLd = "school_reminder_lead"
)

// ModeratorActions are the actions accepted by actOnModerationCase.
var ModeratorActions = []string{ActionWarn, ActionMute, ActionRestrict, ActionRemoveMessage, ActionEscalate, ActionDismiss,
	ActionClose, ActionReopen}

// Sanction types (MODERATION §7).
const (
	SanctionWarning             = "warning"
	SanctionMute                = "mute"
	SanctionRestrictText        = "restrict_text"
	SanctionRestrictDM          = "restrict_dm"
	SanctionRestrictMultiplayer = "restrict_multiplayer"
	SanctionRestrictCompetition = "restrict_competition"
	SanctionAccountLock         = "account_lock"
)

// SanctionTypes is the closed list of sanction types.
var SanctionTypes = []string{SanctionWarning, SanctionMute, SanctionRestrictText, SanctionRestrictDM,
	SanctionRestrictMultiplayer, SanctionRestrictCompetition, SanctionAccountLock}

// RestrictTypes are the sanction types of the action restrict.
var RestrictTypes = []string{SanctionRestrictText, SanctionRestrictDM, SanctionRestrictMultiplayer,
	SanctionRestrictCompetition, SanctionAccountLock}

// Reason codes of a sanction shown to the student (ui.sanction.<code>): never the internal reason.
const (
	CodeCaseHold   = "case_hold"
	CodeModeration = "moderation"
)

// Contact outcomes and paths of a school notification (MODERATION §6.4).
const (
	OutcomeParentContacted = "parent_contacted"
	OutcomeUnableToContact = "unable_to_contact"
	OutcomeNotNeeded       = "not_needed"
	PathSchool             = "school"
	PathPlatformDirect     = "platform_direct"
)

// Outcomes is the closed list of contact outcomes.
var Outcomes = []string{OutcomeParentContacted, OutcomeUnableToContact, OutcomeNotNeeded}

// Student roles in a school notification.
const (
	RoleAffected = "affected"
	RoleOffender = "offender"
)

// SupportGuideChildProtection is the support guide shown to the moderator of a self-harm case (national child
// protection hotline 111, CHILD_SAFETY §8).
const SupportGuideChildProtection = "child_protection_111"

// Categories of the sensitive-term catalogue that make a case p1 (MODERATION §6.2: grooming, threats, self-harm, asking
// for personal information or a meeting); pii is the PII step of the pipeline.
var p1Categories = []string{"self_harm", "grooming_scam", "pii", "pii_pattern"}

// SelfHarmCategory is the catalogue category of the self-harm signal (CHILD_SAFETY §8).
const SelfHarmCategory = "self_harm"

// Parameters (CONFIG_DEFAULTS, owner MODERATION §5–§6.4; registered there, code constants like F116 — no env var).
const (
	// MergeWindow: a report of a target with an open/in_review case created within it joins that case (§6.2).
	MergeWindow = 24 * time.Hour
	// ContextBefore/ContextAfter: the snapshot of a report is ±10 messages of the channel (§5).
	ContextBefore = 10
	ContextAfter  = 10
	// ModeratorContext: a moderator reads ±200 messages of the channel while it exists (§6.1).
	ModeratorContext = 200
	// P1UnclaimedAlert is p1_unclaimed_alert_minutes (§6.3).
	P1UnclaimedAlert = 15 * time.Minute
	// AckSLAUrgent / AckSLAOther are school_notification_ack_sla_hours (24 h p1 or self-harm, 72 h otherwise, §6.4).
	AckSLAUrgent = 24 * time.Hour
	AckSLAOther  = 72 * time.Hour
	// MaxStudents bounds the students of a case and of a notification.
	MaxStudents = 20
	// LeadDuration: restrict_multiplayer longer than this needs the team lead (§7).
	LeadDuration = 7 * 24 * time.Hour
	// AbuseThreshold dismissed reports of one reporter within AbuseWindow lower the weight of the next ones (§5).
	AbuseThreshold = 3
	AbuseWindow    = 30 * 24 * time.Hour
	// AbuseWeight is the weight of a report of such a reporter (never a sanction of the child).
	AbuseWeight = 0.5
)

// SLA is report_sla (MODERATION §6.3): first response and resolution per severity. Working days are counted as calendar
// days (the stricter reading: an earlier deadline, child safety first).
type SLA struct {
	FirstResponse time.Duration
	Resolution    time.Duration
}

var slas = map[string]SLA{
	P1: {FirstResponse: time.Hour, Resolution: 24 * time.Hour},
	P2: {FirstResponse: 24 * time.Hour, Resolution: 3 * 24 * time.Hour},
	P3: {FirstResponse: 3 * 24 * time.Hour, Resolution: 7 * 24 * time.Hour},
}

// SLAOf returns report_sla of severity (p3 for an unknown one).
func SLAOf(severity string) SLA {
	if s, ok := slas[severity]; ok {
		return s
	}
	return slas[P3]
}

// Deadlines returns the first-response and resolution deadlines of a case created at.
func Deadlines(severity string, at time.Time) (firstResponse, resolution time.Time) {
	s := SLAOf(severity)
	return at.Add(s.FirstResponse), at.Add(s.Resolution)
}

// SeverityOfReport maps a report reason to the priority of MODERATION §6.2: asking for personal information → p1;
// bullying and inappropriate content → p2; rude language, spam, cheating, other → p3.
func SeverityOfReport(reason string) string {
	switch reason {
	case ReasonAskedPersonalInfo:
		return P1
	case ReasonBullying, ReasonInappropriate:
		return P2
	}
	return P3
}

// Signal is a pipeline or analysis signal about one message (chat.message_held, async analysis).
type Signal struct {
	Decision string   // hold_for_review, block_and_alert (held messages); empty for async analysis
	Reasons  []string // category codes, pii, bullying
	Async    bool
}

// Classify returns the severity, source and category of the case opened for a signal: self-harm → p1 self_harm_signal;
// block_and_alert, grooming, PII → p1 filter_alert; any other held message → p2; async analysis → p2 (p1 with a p1
// category).
func Classify(s Signal) (severity, source, category string) {
	category = ""
	if len(s.Reasons) > 0 {
		category = s.Reasons[0]
	}
	for _, r := range s.Reasons {
		if r == SelfHarmCategory {
			return P1, SourceSelfHarm, SelfHarmCategory
		}
	}
	source = SourceFilterAlert
	if s.Async {
		source = SourceAsyncAnalysis
	}
	for _, r := range s.Reasons {
		if slices.Contains(p1Categories, r) {
			return P1, source, r
		}
	}
	if s.Decision == "block_and_alert" {
		return P1, source, category
	}
	return P2, source, category
}

// Higher returns the more urgent of two severities.
func Higher(a, b string) string {
	if a == "" {
		return b
	}
	if b != "" && b < a { // "p1" < "p2" < "p3"
		return b
	}
	return a
}

// Domain errors of the case state machine.
var (
	ErrClosed      = errors.New("safety: case is resolved")
	ErrTransition  = errors.New("safety: action not allowed in this status")
	ErrNotAssignee = errors.New("safety: caller is neither the assignee nor a team lead")
	ErrNeedsLead   = errors.New("safety: the team lead is required")
	ErrInvalid     = errors.New("safety: invalid input")
)

// Mergeable reports whether a new report joins an existing case (open or in_review, created within MergeWindow).
func Mergeable(status string, created, now time.Time) bool {
	return (status == StatusOpen || status == StatusInReview) && now.Sub(created) < MergeWindow
}

// Assign returns the status after an assignment (MODERATION §6.2): open/reopened → in_review; escalated → in_review
// (lead only); in_review stays (reassignment by the lead). resolved → ErrClosed.
func Assign(status string, self, lead bool) (string, error) {
	switch status {
	case StatusResolved:
		return "", ErrClosed
	case StatusEscalate:
		if !lead {
			return "", ErrNeedsLead
		}
	case StatusInReview:
		if !lead {
			return "", ErrNeedsLead
		}
	}
	if !self && !lead {
		return "", ErrNeedsLead
	}
	return StatusInReview, nil
}

// Act returns the status after a moderator action (MODERATION §6.2). Sanction actions (warn, mute, restrict,
// remove_message) keep the status; escalate: in_review → escalated; close/dismiss: in_review/escalated → resolved;
// reopen: resolved → reopened. A resolved case accepts only reopen (ErrClosed); open/reopened cases must be assigned
// first (ErrTransition).
func Act(status, action string) (string, error) {
	if status == StatusResolved {
		if action == ActionReopen {
			return StatusReopened, nil
		}
		return "", ErrClosed
	}
	if action == ActionReopen {
		return "", ErrTransition
	}
	if status == StatusOpen || status == StatusReopened {
		return "", ErrTransition
	}
	switch action {
	case ActionEscalate:
		if status != StatusInReview {
			return "", ErrTransition
		}
		return StatusEscalate, nil
	case ActionClose, ActionDismiss:
		return StatusResolved, nil
	case ActionWarn, ActionMute, ActionRestrict, ActionRemoveMessage:
		return status, nil
	}
	return "", ErrInvalid
}

// CanRead reports whether a moderator may read the content of a case: its assignee or a team lead (§6.1).
func CanRead(assignee *uuid.UUID, caller uuid.UUID, lead bool) bool {
	return lead || (assignee != nil && *assignee == caller)
}

// DefaultDuration returns the default duration of a sanction type (MODERATION §7): mute 24 h (custom for the Safety
// Team; the conservative default when omitted), restrict_text 7 days, restrict_dm 30 days, restrict_multiplayer
// 7 days; warning has no duration; restrict_competition and account_lock last until revoked unless a duration is given.
func DefaultDuration(typ string) (time.Duration, bool) {
	switch typ {
	case SanctionMute:
		return 24 * time.Hour, true
	case SanctionRestrictText, SanctionRestrictMultiplayer:
		return 7 * 24 * time.Hour, true
	case SanctionRestrictDM:
		return 30 * 24 * time.Hour, true
	}
	return 0, false
}

// RequiresLead reports a sanction that needs the team lead or a System Admin (§7): account_lock, and
// restrict_multiplayer longer than 7 days (or without end).
func RequiresLead(typ string, d time.Duration, hasEnd bool) bool {
	switch typ {
	case SanctionAccountLock:
		return true
	case SanctionRestrictMultiplayer:
		return !hasEnd || d > LeadDuration
	}
	return false
}

// AckDue returns ack_due_at of a school notification sent at (§6.4): 24 h for p1 or a self-harm category, 72 h else.
func AckDue(severity, category string, at time.Time) time.Time {
	if severity == P1 || category == SelfHarmCategory {
		return at.Add(AckSLAUrgent)
	}
	return at.Add(AckSLAOther)
}

// SupportGuide returns the support guide of a case category ("" when none).
func SupportGuide(category string) string {
	if category == SelfHarmCategory {
		return SupportGuideChildProtection
	}
	return ""
}

// Refusal of a message by an active sanction: the problem code and whether it applies (F121-AC-14).
type Refusal string

// Refusals of the send path.
const (
	RefuseNone   Refusal = ""
	RefuseMuted  Refusal = "CHAT_MUTED"
	RefuseKind   Refusal = "CHAT_KIND_NOT_ALLOWED"
	RefuseDirect Refusal = "CHAT_NOT_ALLOWED"
)

// Refuse returns the refusal of a message of kind in a channel of channelType by a sanction (scope all, or channel
// matching): mute refuses every message; restrict_text allows quick messages only; restrict_dm refuses direct
// channels. Other types do not restrict chat (multiplayer, competition: competition; account_lock: identity).
func Refuse(typ string, scopeMatches bool, kind, channelType string) Refusal {
	if !scopeMatches {
		return RefuseNone
	}
	switch typ {
	case SanctionMute:
		return RefuseMuted
	case SanctionRestrictText:
		if kind != "quick" {
			return RefuseKind
		}
	case SanctionRestrictDM:
		if channelType == "direct" {
			return RefuseDirect
		}
	}
	return RefuseNone
}

// Holds reports a sanction reason code that restricts new invitations and friend requests too (§6.3: the temporary hold
// of a p1 case and the block_and_alert hold of the pipeline).
func Holds(reasonCode string) bool {
	return reasonCode == CodeCaseHold || reasonCode == "block_and_alert_hold"
}
