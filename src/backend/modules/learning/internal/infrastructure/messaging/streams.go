// Package messaging declares the JetStream consumers of module learning (EVENT_DESIGN §3.1, ADR-0042): which
// stream and subjects each durable consumer reads. Delivery, idempotency and DLQ are pkg/outbox.Consumer.
package messaging

import "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"

// ProjectionIdentityName is the durable consumer that keeps the ref.* projections of module identity
// (DATABASE_DESIGN §2.17, F020 US1). Its dead letters go to DLQ_LEARNING (ew.dlq.learning.projection_identity).
const ProjectionIdentityName = "learning.projection_identity"

// ProjectionIdentityStream is the stream of the producer (EVT_IDENTITY).
var ProjectionIdentityStream = natsx.EventStream("identity")

// ProjectionIdentityEvents are the identity events projected into ref.* (F020-AC-01, F020-AC-02; school.updated
// also feeds ref.schools, DATABASE_DESIGN §2.17; student.placement_changed keeps the school and grade of ref.students —
// ADR-0095: there is no class.synced and no enrollment.changed any more).
var ProjectionIdentityEvents = []string{"tenant.created", "tenant.updated", "school.synced", "school.updated",
	"student.synced", "student.cleaned_up", "student.restored", "province.updated", "student.placement_changed"}

// ProjectionIdentitySubjects are the filter subjects of the consumer (exact subjects, so other identity events
// are never delivered).
func ProjectionIdentitySubjects() []string {
	out := make([]string, len(ProjectionIdentityEvents))
	for i, e := range ProjectionIdentityEvents {
		out[i] = natsx.EventSubject("identity", e)
	}
	return out
}

// QuestionPoolName is the durable consumer that drops the cached question pools of module learning when a question
// is released or an assignment changes (F022-AC-28, QUESTION_BANK §6).
const QuestionPoolName = "learning.qb_pool_invalidation"

// ReleasePointerName is the durable consumer that keeps the release pointers of Valkey ew:learning:current:* (F026-AC-16);
// it reads the release events of QuestionPoolSubjects.
const ReleasePointerName = "learning.release_pointer_cache"

// QuestionPoolStream is the stream of the content events (EVT_LEARNING).
var QuestionPoolStream = natsx.EventStream("learning")

// QuestionPoolEvents are the release events of learning content (owner F026; item_type question is used).
var QuestionPoolEvents = []string{"content.published", "content.rolled_back", "content.unpublished"}

// QuestionPoolSubjects are the filter subjects of the consumer.
func QuestionPoolSubjects() []string {
	out := make([]string, len(QuestionPoolEvents))
	for i, e := range QuestionPoolEvents {
		out[i] = natsx.EventSubject("learning", e)
	}
	return out
}

// MediaWorkerName is the durable consumer of the media pipeline (F027 T019, profile worker-events): it processes
// media.uploaded of module learning; its dead letters go to DLQ_LEARNING (ew.dlq.learning.media_worker).
const MediaWorkerName = "learning.media_worker"

// MediaWorkerStream is the stream of the learning events (EVT_LEARNING).
var MediaWorkerStream = natsx.EventStream("learning")

// MediaWorkerSubjects are the filter subjects of the media worker.
func MediaWorkerSubjects() []string {
	return []string{natsx.EventSubject("learning", "media.uploaded")}
}

// MasteryName is the durable consumer that applies the evidence of graded attempts to the mastery of
// (student, objective) (F031 US2) and recomputes them after attempt.re_evaluated (F031 US4).
const MasteryName = "learning.mastery_on_graded"

// ProgressDailyName is the durable consumer of the progress_daily counters (F031 US3).
const ProgressDailyName = "learning.progress_daily"

// AttemptStream is the stream of the attempt events of module learning (EVT_LEARNING).
var AttemptStream = natsx.EventStream("learning")

// MasterySubjects are the filter subjects of learning.mastery_on_graded.
func MasterySubjects() []string {
	return []string{natsx.EventSubject("learning", "activity.graded"), natsx.EventSubject("learning", "attempt.re_evaluated")}
}

// ProgressDailySubjects are the filter subjects of learning.progress_daily: the counters of an attempt are taken from its
// activity.graded (activity.submitted of the same attempt carries nothing more) [Proposed].
func ProgressDailySubjects() []string {
	return []string{natsx.EventSubject("learning", "activity.graded"), natsx.EventSubject("learning", "lesson.completed")}
}

// LessonCompletionName is the durable consumer that completes lesson sessions (F031 US6).
const LessonCompletionName = "learning.lesson_completion"

// LessonCompletionSubjects are the filter subjects of learning.lesson_completion.
func LessonCompletionSubjects() []string {
	return []string{natsx.EventSubject("learning", "activity.graded")}
}

// AssignmentSubmissionsName is the durable consumer that keeps the submissions of the school assignments from the attempt
// events (F041-AC-15).
const AssignmentSubmissionsName = "learning.assignment_submissions"

// AssignmentSubmissionsSubjects are the attempt events of module learning an assignment submission follows (exam.graded
// joins with F042).
func AssignmentSubmissionsSubjects() []string {
	out := []string{}
	for _, e := range []string{"activity.started", "activity.submitted", "activity.graded", "attempt.re_evaluated"} {
		out = append(out, natsx.EventSubject("learning", e))
	}
	return out
}

// AssignmentEnrollmentName is the durable consumer that applies the placement changes (school, grade — ADR-0095) and
// account locks of module identity to the open assignments (F041-AC-16).
const AssignmentEnrollmentName = "learning.assignment_enrollment"

// AssignmentEnrollmentSubjects are student.placement_changed, student.synced and user.access_revoked of module identity.
func AssignmentEnrollmentSubjects() []string {
	out := []string{}
	for _, e := range []string{"student.placement_changed", "student.synced", "user.access_revoked"} {
		out = append(out, natsx.EventSubject("identity", e))
	}
	return out
}

// ExamGradeName is the durable consumer that grades the submitted exam attempts (worker exam.grade, F042-AC-19).
const ExamGradeName = "learning.exam_grade"

// ExamGradeSubjects are the filter subjects of learning.exam_grade.
func ExamGradeSubjects() []string { return []string{natsx.EventSubject("learning", "exam.submitted")} }

// ExamQualityName is the durable consumer that computes the quality report of a closed exam session (F044-AC-23).
const ExamQualityName = "learning.exam_quality"

// ExamQualitySubjects are the filter subjects of learning.exam_quality.
func ExamQualitySubjects() []string { return []string{natsx.EventSubject("learning", "exam.closed")} }

// QuestLearningEventName is the durable consumer that records the learning event quest.completed of the quests of module
// game that carry learning objectives (F072-AC-21, EVENT_DESIGN §5.2).
const QuestLearningEventName = "learning.learning_event_on_quest_completed"

// GameStream is the stream of module game (EVT_GAME).
var GameStream = natsx.EventStream("game")

// QuestLearningEventSubjects are the filter subjects of learning.learning_event_on_quest_completed.
func QuestLearningEventSubjects() []string {
	return []string{natsx.EventSubject("game", "quest.completed")}
}

// MinigameConfigProjectionName is the durable consumer that keeps the projection ref.minigame_config_versions of the
// mini-game configurations published by module game (F073-AC-04); it reads the platform events content.published of
// the game stream and keeps item_type minigame_config only.
const MinigameConfigProjectionName = "learning.minigame_config_projection"

// MinigameConfigProjectionSubjects are the filter subjects of learning.minigame_config_projection.
func MinigameConfigProjectionSubjects() []string {
	return []string{natsx.EventSubject("game", "content.published")}
}
