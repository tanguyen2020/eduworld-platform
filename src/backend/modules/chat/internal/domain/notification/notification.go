// Package notification is the in-game notification bounded context of module chat (F123, SOCIAL_SYSTEM §8, CHAT_SYSTEM
// §10): which domain event becomes which notification (MessageCatalog), who may receive it (RecipientPolicy) and the read
// state of one notification (read once). No push outside the game exists (FR-NTF-003): a notification is only stored and
// delivered over the realtime connection of the recipient and the inbox API.
package notification

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Kind is the one notification kind of REST (Notification.kind) and WS (notification.item.created) — NotificationKind of
// specs/schemas/common/enums.schema.json (RB-513).
type Kind string

// Kinds of specs/schemas/common/enums.schema.json (same order); F123 creates the ones of the MessageCatalog.
const (
	KindAssignmentNew       Kind = "assignment_new"
	KindAssignmentDue       Kind = "assignment_due"
	KindPartyInvitation     Kind = "party_invitation"
	KindFriendRequest       Kind = "friend_request"
	KindQuestAvailable      Kind = "quest_available"
	KindRewardGranted       Kind = "reward_granted"
	KindTournamentUpdate    Kind = "tournament_update"
	KindModerationNotice    Kind = "moderation_notice"
	KindSystem              Kind = "system"
	KindAssignmentResults   Kind = "assignment_results_released"
	KindLevelReached        Kind = "level_reached"
	KindAchievementUnlocked Kind = "achievement_unlocked"
	// Club kinds (delta F120/F123 for the client F111): deep link open_club {id: club_id}.
	KindClubInvitation  Kind = "club_invitation"
	KindClubJoinRequest Kind = "club_join_request"
	KindClubApproved    Kind = "club_approved"
)

// Kinds lists every value of the enum (CHECK social.notifications.kind).
var Kinds = []Kind{KindAssignmentNew, KindAssignmentDue, KindPartyInvitation, KindFriendRequest, KindQuestAvailable,
	KindRewardGranted, KindTournamentUpdate, KindModerationNotice, KindSystem, KindAssignmentResults, KindLevelReached,
	KindAchievementUnlocked, KindClubInvitation, KindClubJoinRequest, KindClubApproved}

// Source events turned into notifications (EVENT_DESIGN §3.2, consumers chat.notify_*).
const (
	EventFriendRequestSent = "friend_request.sent"
	EventPartyInvitation   = "party.invitation_sent"
	EventAssignmentPublish = "assignment.published"
	EventAssignmentResults = "assignment.results_released"
	EventLevelReached      = "level.reached"
	EventAchievementUnlock = "achievement.unlocked"
	EventReportResolved    = "report.resolved"
	EventClubInvitation    = "club.invitation_sent"
	EventClubJoinRequested = "club.join_requested"
	EventClubApproved      = "club.approved"
)

// ActionOpenClub is the deep link of the club notifications ({kind: open_club, id: club_id}); ActionOpenEvent
// ({kind: open_event, id: event_id}) is reserved: no Release 1.0 notification uses it (delta F120/F123).
const (
	ActionOpenClub  = "open_club"
	ActionOpenEvent = "open_event"
)

// Bounds of the contract (ADR-0041): params ≤ 20 entries of ≤ 255 characters; ≤ 100 ids per mark_read.
const (
	MaxParams      = 20
	MaxParamLen    = 255
	MaxMarkRead    = 100
	maxParamKeyLen = 32
)

// Template is the row of the MessageCatalog of one source event: the kind, the message code (client key
// ui.notification.<code>), the deep link (action kind; "" = none), the type of the referenced resource and the only
// params allowed (F123-AC-05: a nickname, the name of a content, an id — never a real name, school, class, chat content,
// score of another student or sanction detail).
type Template struct {
	Kind        Kind
	MessageCode string
	Action      string
	RefType     string
	Params      []string
}

// MessageCatalog maps each source event to its template (plan F123 "Bảng MessageCatalog"; report.resolved of F121-AC-16).
var MessageCatalog = map[string]Template{
	EventFriendRequestSent: {Kind: KindFriendRequest, MessageCode: "friend.request_received", Action: "open_friend_requests",
		RefType: "friend_request", Params: []string{"nickname"}},
	EventPartyInvitation: {Kind: KindPartyInvitation, MessageCode: "party.invitation_received", Action: "open_party_invitation",
		RefType: "party_invitation", Params: []string{"nickname"}},
	EventAssignmentPublish: {Kind: KindAssignmentNew, MessageCode: "assignment.new", Action: "open_assignment",
		RefType: "assignment", Params: []string{"title"}},
	EventAssignmentResults: {Kind: KindAssignmentResults, MessageCode: "assignment.results_released", Action: "open_assignment_result",
		RefType: "assignment", Params: []string{"title"}},
	EventLevelReached: {Kind: KindLevelReached, MessageCode: "progression.level_reached", Action: "open_profile",
		RefType: "level", Params: []string{"level"}},
	EventAchievementUnlock: {Kind: KindAchievementUnlocked, MessageCode: "achievement.unlocked", Action: "open_achievements",
		RefType: "achievement", Params: []string{"tier"}},
	EventReportResolved: {Kind: KindModerationNotice, MessageCode: "moderation.report_handled", RefType: "report"},
	// Clubs (delta F120/F123): the invitee, the owner and officers of the club, the student owner of an approved club.
	EventClubInvitation: {Kind: KindClubInvitation, MessageCode: "club.invitation_received", Action: ActionOpenClub, RefType: "club",
		Params: []string{"nickname"}},
	EventClubJoinRequested: {Kind: KindClubJoinRequest, MessageCode: "club.join_request_received", Action: ActionOpenClub,
		RefType: "club", Params: []string{"nickname"}},
	EventClubApproved: {Kind: KindClubApproved, MessageCode: "club.approved", Action: ActionOpenClub, RefType: "club"},
}

// Action is the in-game deep link {kind, id?} (NotificationAction of the contract).
type Action struct {
	Kind string     `json:"kind"`
	ID   *uuid.UUID `json:"id,omitempty"`
}

// ErrUnknownSource: the event has no template.
var ErrUnknownSource = errors.New("notification: no template for this event")

// ErrParam: a param is not allowed by the template or breaks the bounds.
var ErrParam = errors.New("notification: param not allowed")

// Draft is a notification to create for one recipient.
type Draft struct {
	TenantID      uuid.UUID
	RecipientID   uuid.UUID
	SchoolID      *uuid.UUID
	Kind          Kind
	MessageCode   string
	Params        map[string]string
	RefType       string
	RefID         *uuid.UUID
	Action        *Action
	SourceEventID uuid.UUID
	CreatedAt     time.Time
}

// Source is a source event reduced to what a notification needs (decoded and validated by the consumer).
type Source struct {
	EventID    uuid.UUID
	EventType  string
	TenantID   uuid.UUID
	OccurredAt time.Time
	RefID      *uuid.UUID
	Params     map[string]string // candidate params; Build keeps the allowed ones only
}

// Build returns the draft of src for recipient (school of the recipient projection): the template of the event, params
// filtered by the whitelist and sanitized, created_at = the occurred_at of the source (deterministic: a redelivered event
// maps to the same unique row, F123-AC-03).
func Build(src Source, recipient uuid.UUID, school *uuid.UUID) (Draft, error) {
	t, ok := MessageCatalog[src.EventType]
	if !ok {
		return Draft{}, fmt.Errorf("%w: %s", ErrUnknownSource, src.EventType)
	}
	params, err := Params(t, src.Params)
	if err != nil {
		return Draft{}, err
	}
	d := Draft{TenantID: src.TenantID, RecipientID: recipient, SchoolID: school, Kind: t.Kind, MessageCode: t.MessageCode,
		Params: params, RefType: t.RefType, RefID: src.RefID, SourceEventID: src.EventID, CreatedAt: src.OccurredAt.UTC()}
	if t.Action != "" {
		d.Action = &Action{Kind: t.Action, ID: src.RefID}
	}
	return d, nil
}

// Params keeps the params the template allows (empty values are dropped) and sanitizes them; a key outside the
// whitelist is an error (a producer adapter must never pass more than the template needs, R16).
func Params(t Template, in map[string]string) (map[string]string, error) {
	out := map[string]string{}
	for _, k := range slices.Sorted(maps.Keys(in)) {
		if !slices.Contains(t.Params, k) || len(k) > maxParamKeyLen {
			return nil, fmt.Errorf("%w: %s", ErrParam, k)
		}
		if v := Sanitize(in[k]); v != "" {
			out[k] = v
		}
	}
	if len(out) > MaxParams {
		return nil, fmt.Errorf("%w: more than %d params", ErrParam, MaxParams)
	}
	return out, nil
}

// Sanitize removes control characters and invalid UTF-8 and trims a param value to MaxParamLen characters.
func Sanitize(s string) string {
	s = strings.ToValidUTF8(s, "")
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > MaxParamLen {
		s = string([]rune(s)[:MaxParamLen])
	}
	return s
}

// Recipient is the projected student who would receive a notification (ref.students of eduworld_chat).
type Recipient struct {
	ID       uuid.UUID
	TenantID uuid.UUID
	SchoolID uuid.UUID
	Status   string
	Deleted  bool
}

// Eligible is the RecipientPolicy (F123-AC-06): only an active, not removed student of the tenant of the event receives
// it; an unknown student, a student of another tenant or a locked/deactivated/cleaned-up one receives nothing.
func Eligible(r *Recipient, tenant uuid.UUID) bool {
	return r != nil && tenant != uuid.Nil && r.TenantID == tenant && r.Status == "active" && !r.Deleted
}

// Notification is one stored notification of one recipient.
type Notification struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	RecipientID uuid.UUID
	Kind        Kind
	MessageCode string
	Params      map[string]string
	RefType     string
	RefID       *uuid.UUID
	Action      *Action
	CreatedAt   time.Time
	ReadAt      *time.Time
}

// Read reports a notification already read.
func (n Notification) Read() bool { return n.ReadAt != nil }

// MarkRead sets read_at once (F123-AC-09): a second call changes nothing and reports false.
func (n *Notification) MarkRead(at time.Time) bool {
	if n.ReadAt != nil {
		return false
	}
	t := at.UTC()
	n.ReadAt = &t
	return true
}

// MarkReadIDs bounds and deduplicates the ids of notification.item.mark_read (1..100, AsyncAPI maxItems).
func MarkReadIDs(ids []uuid.UUID) ([]uuid.UUID, error) {
	if len(ids) == 0 || len(ids) > MaxMarkRead {
		return nil, fmt.Errorf("notification: 1..%d ids", MaxMarkRead)
	}
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id != uuid.Nil && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out, nil
}
