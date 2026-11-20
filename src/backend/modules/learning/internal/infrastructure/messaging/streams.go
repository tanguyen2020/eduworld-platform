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
// also feeds ref.schools, DATABASE_DESIGN §2.17).
var ProjectionIdentityEvents = []string{"tenant.created", "tenant.updated", "school.synced", "school.updated",
	"class.synced", "student.synced", "student.cleaned_up", "student.restored", "enrollment.changed", "province.updated"}

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
	return []string{natsx.EventSubject("learning", "activity.graded")}
}
