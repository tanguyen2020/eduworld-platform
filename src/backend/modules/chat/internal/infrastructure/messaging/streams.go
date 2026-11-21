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
