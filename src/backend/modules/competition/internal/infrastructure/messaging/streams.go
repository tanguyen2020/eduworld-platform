// Package messaging declares the JetStream consumers of module competition (EVENT_DESIGN §3.1, ADR-0042): which
// stream and subjects each durable consumer reads. Delivery, idempotency and DLQ are pkg/outbox.Consumer.
package messaging

import "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"

// IdentityStream and GameStream are the streams of the producers identity and game.
var (
	IdentityStream = natsx.EventStream("identity")
	GameStream     = natsx.EventStream("game")
)

// ProjectionIdentityName keeps ref.tenants and ref.students of eduworld_competition (F091 T009).
const ProjectionIdentityName = "competition.projection_identity"

// ProjectionIdentityEvents are the identity events it projects.
var ProjectionIdentityEvents = []string{"tenant.created", "tenant.updated", "student.synced", "user.status_changed",
	"student.cleaned_up", "student.restored", "enrollment.changed"}

// HubAccessRevokedName removes a revoked student from the hub at once (F091 edge case, no grace).
const HubAccessRevokedName = "competition.hub_access_revoked"

// HubAccessRevokedEvents are the identity events it handles (owner F016).
var HubAccessRevokedEvents = []string{"user.access_revoked"}

// ProjectionGameName keeps ref.player_profiles and pushes appearance changes to the hub (F091-AC-19).
const ProjectionGameName = "competition.projection_game"

// ProjectionGameEvents are the game events it projects (owner F070). content.published (item_type = world_area) and
// world.area_unlocked join this consumer when F071 defines their payload (F091 T005: not created here).
var ProjectionGameEvents = []string{"game_profile.updated"}

// ChatStream is the stream of the producer chat.
var ChatStream = natsx.EventStream("chat")

// ProjectionSocialName keeps ref.friendships and ref.blocks from the relation events of module chat (owner F095, spec
// F091 A-1): "join a friend", friends first and block avoidance in the hub (F091), party and matchmaking (F100, F104).
const ProjectionSocialName = "competition.projection_social"

// ProjectionSocialEvents are the chat events it projects.
var ProjectionSocialEvents = []string{"friend.added", "friend.removed", "user.blocked", "user.unblocked"}

// Subjects returns the exact subjects of the events of producer.
func Subjects(producer string, events []string) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = natsx.EventSubject(producer, e)
	}
	return out
}

// PartySafetyChatName applies the safety events of module chat to parties (F100-AC-25): user.blocked cancels the
// pending invitations between the two students; sanction.applied/lifted keep ref.sanctions and a multiplayer sanction
// takes the student out of its party.
const PartySafetyChatName = "competition.party_safety_chat"

// PartySafetyChatEvents are the chat events it handles (owners F095, F121).
var PartySafetyChatEvents = []string{"user.blocked", "sanction.applied", "sanction.lifted"}

// PartySafetyIdentityName takes out of their party the students who changed school, were deactivated, locked or
// cleaned up (F100 edge cases).
const PartySafetyIdentityName = "competition.party_safety_identity"

// PartySafetyIdentityEvents are the identity events it handles (owners F010, F016).
var PartySafetyIdentityEvents = []string{"student.synced", "user.status_changed", "student.cleaned_up"}
