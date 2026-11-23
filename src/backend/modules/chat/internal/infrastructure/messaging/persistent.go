package messaging

import "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"

// LearningStream is the stream of the producer learning (activity.graded, club goals of F120).
var LearningStream = natsx.EventStream("learning")

// MembershipName keeps the direct channels of blocked pairs frozen (F120-AC-15): user.blocked / user.unblocked of module
// chat itself (social), at least once, idempotent through processed_events.
const MembershipName = "chat.membership"

// MembershipEvents are the chat events it consumes (owner F095).
var MembershipEvents = []string{"user.blocked", "user.unblocked"}

// ClubGoalsLearningName and ClubGoalsGameName count the club goals from verified events (F120-AC-21). No class or study
// group event is consumed (ADR-0095); boss.encounter_completed has no schema yet (owner F103) [Proposed].
const (
	ClubGoalsLearningName = "chat.club_goals_learning"
	ClubGoalsGameName     = "chat.club_goals_game"
)

// ClubGoalsLearningEvents / ClubGoalsGameEvents are the events of the club goal consumers.
var (
	ClubGoalsLearningEvents = []string{"activity.graded"}
	ClubGoalsGameEvents     = []string{"quest.completed", "world.discovery_found"}
)
