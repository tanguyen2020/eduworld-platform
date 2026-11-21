// Package messaging declares the JetStream consumers of module game (EVENT_DESIGN §3.1, ADR-0042): which stream and
// subjects each durable consumer reads. Delivery, idempotency and DLQ are pkg/outbox.Consumer.
package messaging

import "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"

// IdentityStream is the stream of the producer identity (EVT_IDENTITY).
var IdentityStream = natsx.EventStream("identity")

// ProjectionIdentityName is the durable consumer that keeps the ref.* projections of module identity in
// eduworld_game (DATABASE_DESIGN §2.17, F014 T008).
const ProjectionIdentityName = "game.projection_identity"

// ProjectionIdentityEvents are the identity events projected into ref.tenants, ref.schools, ref.students.
var ProjectionIdentityEvents = []string{"tenant.created", "tenant.updated", "school.synced", "school.updated",
	"student.synced", "user.status_changed", "student.cleaned_up", "student.restored",
	"student.sis_link_created", "student.sis_link_superseded"} // ref.students.sis_connection_id (OQ F014-1)

// SessionRevokeName is the durable consumer that ends the game sessions of a revoked, locked or cleaned-up student
// (F014-AC-24, T039).
const SessionRevokeName = "game.session_end_on_revoke"

// SessionRevokeEvents are the events it handles (owners F011/F016).
var SessionRevokeEvents = []string{"user.access_revoked", "user.status_changed", "student.cleaned_up"}

// Subjects returns the exact subjects of identity events (other identity events are never delivered).
func Subjects(events []string) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = natsx.EventSubject("identity", e)
	}
	return out
}

// LearningStream is the stream of the producer learning (EVT_LEARNING).
var LearningStream = natsx.EventStream("learning")

// ManifestLearningName is the durable consumer that merges the released learning items into the tenant content
// manifests (F026 T046, CONTENT_MODEL §5.3).
const ManifestLearningName = "game.manifest_on_learning_content"

// ManifestLearningEvents are the release events of learning content (owner F026).
var ManifestLearningEvents = []string{"content.published", "content.rolled_back", "content.unpublished"}

// LearningSubjects returns the exact subjects of learning events.
func LearningSubjects(events []string) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = natsx.EventSubject("learning", e)
	}
	return out
}

// PlatformSettingsName is the durable consumer that projects the platform settings of module identity that module
// game applies (ref.platform_settings: platform_curfew, OQ F014-2).
const PlatformSettingsName = "game.platform_settings"

// PlatformSettingsEvents are the events it handles (owner F015).
var PlatformSettingsEvents = []string{"platform_settings.changed"}

// GameStream is the stream of module game itself (EVT_GAME): its own release and progress events (F071).
var GameStream = natsx.EventStream("game")

// GameSubjects returns the exact subjects of game events.
func GameSubjects(events []string) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = natsx.EventSubject("game", e)
	}
	return out
}

// ManifestGameName is the durable consumer that rebuilds the content manifest of every tenant when game content is
// released (F071-AC-05, the "manifest builder" consumer of EVENT_DESIGN content.published).
const ManifestGameName = "game.manifest_on_game_content"

// ManifestGameEvents are the release events of game content (owner F071; F078 adds rolled_back/unpublished).
var ManifestGameEvents = []string{"content.published"}

// UnlockOnEventsName is the durable consumer that records the progress facts of a student and evaluates the unlock rules
// of the areas and the story chapters (F071 T031, T040): one consumer so that a quest completing a chapter and the
// unlock that chapter opens are applied in one transaction.
const UnlockOnEventsName = "game.unlock_on_events"

// UnlockOnEventsGameEvents are the game events it handles (owners F072, F074, F075, F077; story.chapter_completed F071).
var UnlockOnEventsGameEvents = []string{"quest.completed", "story.chapter_completed", "level.reached", "achievement.unlocked",
	"event.participation_started"}

// MasteryProjectionName keeps ref.mastery_summary from learning_objective.progressed (owner F031) and re-evaluates the
// objective_progress unlock rules of the student (F071 T011).
const MasteryProjectionName = "game.mastery_projection"

// MasteryProjectionEvents are the learning events it handles.
var MasteryProjectionEvents = []string{"learning_objective.progressed"}
