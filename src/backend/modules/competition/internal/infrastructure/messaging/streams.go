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
	"student.cleaned_up", "student.restored"}

// HubAccessRevokedName removes a revoked student from the hub at once (F091 edge case, no grace).
const HubAccessRevokedName = "competition.hub_access_revoked"

// HubAccessRevokedEvents are the identity events it handles (owner F016).
var HubAccessRevokedEvents = []string{"user.access_revoked"}

// ProjectionGameName keeps ref.player_profiles and pushes appearance changes to the hub (F091-AC-19).
const ProjectionGameName = "competition.projection_game"

// ProjectionGameEvents are the game events it projects (owner F070). content.published (item_type = world_area) and
// world.area_unlocked join this consumer when F071 defines their payload (F091 T005: not created here).
var ProjectionGameEvents = []string{"game_profile.updated"}

// Subjects returns the exact subjects of the events of producer.
func Subjects(producer string, events []string) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = natsx.EventSubject(producer, e)
	}
	return out
}
