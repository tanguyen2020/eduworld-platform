// Package messaging declares the JetStream consumers of module chat (EVENT_DESIGN §3.1, ADR-0042): which stream and
// subjects each durable consumer reads. Delivery, idempotency and DLQ are pkg/outbox.Consumer.
package messaging

import (
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
)

// IdentityStream, GameStream and ChatStream are the streams of the producers identity, game and chat.
var (
	IdentityStream = natsx.EventStream("identity")
	GameStream     = natsx.EventStream("game")
	ChatStream     = natsx.EventStream("chat")
)

// ModerationCasesName opens the cases of the pipeline signals (F121-AC-03): chat.message_held of chat-persister (F116).
const ModerationCasesName = "chat.moderation_cases"

// ModerationCasesEvents are the chat events it consumes.
var ModerationCasesEvents = []string{command.EventMessageHeld}

// ProjectionIdentityName keeps ref.tenants and ref.students of eduworld_chat (F095 T007), ref.schools and
// ref.platform_settings (platform_curfew) for the curfew of the send path (F115 T041).
const ProjectionIdentityName = "chat.projection_identity"

// ProjectionIdentityEvents are the identity events it projects.
var ProjectionIdentityEvents = []string{"tenant.created", "tenant.updated", "student.synced", "user.status_changed",
	"student.cleaned_up", "student.restored", "school.synced", "school.updated", "platform_settings.changed"}

// ProjectionGameName keeps ref.player_profiles (nickname, avatar, level of PlayerCard, F095-AC-11).
const ProjectionGameName = "chat.projection_game"

// ProjectionGameEvents are the game events it projects (owner F070).
var ProjectionGameEvents = []string{"game_profile.updated"}

// Subjects returns the exact subjects of the events of producer.
func Subjects(producer string, events []string) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = natsx.EventSubject(producer, e)
	}
	return out
}

// CompetitionStream is the stream of the producer competition (party and match lifecycle, F100/F101/F104/F105).
var CompetitionStream = natsx.EventStream("competition")

// GameplayChannelsName opens, fills and closes the always-on gameplay channels match/team/party (F117 US1, ADR-0020 §6).
const GameplayChannelsName = "chat.gameplay_channels"

// GameplayChannelsEvents are the competition events it consumes (party.created v1 and v2 accepted).
var GameplayChannelsEvents = []string{"party.created", "party.member_joined", "party.member_left", "party.member_kicked", "party.leader_transferred",
	"party.disbanded", "match.started", "match.completed", "match.aborted"}

// Durable consumers of the in-game notifications (F123 T001, EVENT_DESIGN §3.2): one per producer stream, plus the
// moderation notices of F121 on the chat stream. They run with the other domain consumers (worker events).
const (
	NotifyFriendRequestName   = "chat.notify_friend_request"
	NotifyModerationName      = "chat.notify_moderation"
	NotifyPartyInvitationName = "chat.notify_party_invitation"
	NotifyAssignmentName      = "chat.notify_assignment"
	NotifyProgressionName     = "chat.notify_progression"
)

// NotifierRoute is the stream, producer and events of one notifier consumer.
type NotifierRoute struct {
	Stream   string
	Producer string
	Events   []string
}

// Notifiers maps each notifier consumer to what it reads (owners: F095, F121, F100, F041, F074/F075).
var Notifiers = map[string]NotifierRoute{
	NotifyFriendRequestName:   {Stream: ChatStream, Producer: "chat", Events: []string{"friend_request.sent"}},
	NotifyModerationName:      {Stream: ChatStream, Producer: "chat", Events: []string{"report.resolved"}},
	NotifyPartyInvitationName: {Stream: CompetitionStream, Producer: "competition", Events: []string{"party.invitation_sent"}},
	NotifyAssignmentName:      {Stream: LearningStream, Producer: "learning", Events: []string{"assignment.published", "assignment.results_released"}},
	NotifyProgressionName:     {Stream: GameStream, Producer: "game", Events: []string{"level.reached", "achievement.unlocked"}},
}
