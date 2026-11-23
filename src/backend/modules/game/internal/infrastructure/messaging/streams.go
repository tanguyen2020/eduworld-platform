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

// MiniGameActivityGradedName records the result of a mini-game play and publishes minigame.completed on the graded attempts
// of module learning that carry the minigame block (F073-AC-18, DLQ ew.dlq.game.minigame_on_activity_graded).
const MiniGameActivityGradedName = "game.minigame_on_activity_graded"

// QuestActivityGradedName evaluates the pass rule of the quest steps on the graded attempts of module learning (F072
// T031): activity.graded of an attempt linked to a quest step (game.quest_step_attempts).
const QuestActivityGradedName = "game.quest_on_activity_graded"

// QuestActivityGradedEvents are the learning events it handles (owner F030).
var QuestActivityGradedEvents = []string{"activity.graded"}

// QuestDiscoveryName advances the discover steps of the active quests on world.discovery_found (F072 T024).
const QuestDiscoveryName = "game.quest_on_discovery_found"

// QuestDiscoveryEvents are the game events it handles (owner F071).
var QuestDiscoveryEvents = []string{"world.discovery_found"}

// PeriodicQuestsName grants the daily/weekly quests on the first game session of a period (F072 T038).
const PeriodicQuestsName = "game.periodic_quests_on_session_started"

// PeriodicQuestsEvents are the game events it handles (owner F014).
var PeriodicQuestsEvents = []string{"game_session.started"}

// RewardEngineName is the Reward Engine on the learning events (F074 T020, DLQ ew.dlq.game.reward_engine): XP from
// graded activities, completed lessons and mastered objectives.
const RewardEngineName = "game.reward_engine"

// RewardEngineLearningEvents are the learning events it handles (owners F030, F031).
var RewardEngineLearningEvents = []string{"activity.graded", "lesson.completed", "learning_objective.mastered"}

// RewardEngineGameName is the Reward Engine on the events of module game itself (one durable consumer per stream:
// a JetStream consumer reads one stream); minigame.completed since F073.
const RewardEngineGameName = "game.reward_engine_game"

// RewardEngineGameEvents are the game events it handles (owners F071, F072, F073, F076).
var RewardEngineGameEvents = []string{"quest.completed", "world.discovery_found", "world.area_completed", "minigame.completed",
	"challenge.completed"}

// ChallengeProgressName updates the challenge attempts in progress on the graded learning attempts they opened (F076
// T025): score, lives and goal from the result of module learning only (ADR-0016).
const ChallengeProgressName = "game.challenge_progress"

// ChallengeProgressEvents are the learning events it handles (owner F030).
var ChallengeProgressEvents = []string{"activity.graded"}

// GradeTransitionName applies the school-year grade transition of a student (F074 T035, FR-GAM-010).
const GradeTransitionName = "game.grade_transition"

// GradeTransitionEvents are the identity events it handles (owners F010, F013).
var GradeTransitionEvents = []string{"student.synced", "student.placement_changed"}

// CompetitionStream is the stream of module competition (EVT_COMPETITION).
var CompetitionStream = natsx.EventStream("competition")

// CompetitionSubjects returns the exact subjects of competition events.
func CompetitionSubjects(events []string) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = natsx.EventSubject("competition", e)
	}
	return out
}

// AchievementEvaluatorName is the achievement-evaluator on the events of module game (F075 T035): one durable consumer per
// stream (a JetStream consumer reads one stream), same handler.
const AchievementEvaluatorName = "game.achievement_evaluator"

// AchievementEvaluatorGameEvents are the game events of the achievement trigger catalog (owners F070–F076, F075).
var AchievementEvaluatorGameEvents = []string{"avatar.created", "quest.completed", "challenge.completed", "boss.encounter_completed",
	"world.discovery_found", "world.area_completed", "world.area_unlocked", "level.reached", "collection.completed", "shop.item_purchased",
	"event.participation_started", "event.milestone_claimed", "season_track.tier_reached"}

// AchievementEvaluatorLearningName is the achievement-evaluator on the learning events.
const AchievementEvaluatorLearningName = "game.achievement_evaluator_learning"

// AchievementEvaluatorLearningEvents are the learning events of the trigger catalog (owners F030, F031).
var AchievementEvaluatorLearningEvents = []string{"lesson.completed", "exercise.submitted", "learning_objective.mastered"}

// AchievementEvaluatorCompetitionName is the achievement-evaluator on the competition events.
const AchievementEvaluatorCompetitionName = "game.achievement_evaluator_competition"

// AchievementEvaluatorCompetitionEvents are the competition events of the trigger catalog (owners F101, F105).
var AchievementEvaluatorCompetitionEvents = []string{"party.activity_completed", "match.completed"}

// CollectionOnMasteryName grants the knowledge card of an objective at mastery proficient or above (F075 T030).
const CollectionOnMasteryName = "game.collection_on_mastery"

// CollectionOnMasteryEvents are the learning events it handles (owner F031).
var CollectionOnMasteryEvents = []string{"learning_objective.progressed"}

// LiveOpsProgressName is the live-ops progress on the events of module game (F077 T024: event points, community goal
// contributions, season points from verified server events). One durable consumer per stream, same handler.
const LiveOpsProgressName = "game.liveops_progress"

// LiveOpsProgressGameEvents are the game events it handles (owners F071, F072).
var LiveOpsProgressGameEvents = []string{"quest.completed", "world.discovery_found"}

// LiveOpsProgressLearningName is the live-ops progress on the learning events.
const LiveOpsProgressLearningName = "game.liveops_progress_learning"

// LiveOpsProgressLearningEvents are the learning events it handles (owner F030).
var LiveOpsProgressLearningEvents = []string{"activity.graded"}

// LiveOpsProgressCompetitionName is the live-ops progress on the competition events.
const LiveOpsProgressCompetitionName = "game.liveops_progress_competition"

// LiveOpsProgressCompetitionEvents are the competition events it handles (owners F101, F105).
var LiveOpsProgressCompetitionEvents = []string{"party.activity_completed", "match.completed"}

// SeasonProjectionName keeps ref.seasons from the season calendar of module competition and closes the season track
// (F077 T029, platform events).
const SeasonProjectionName = "game.season_projection"

// SeasonProjectionEvents are the competition events it handles (owner F106).
var SeasonProjectionEvents = []string{"season.started", "season.ended"}
