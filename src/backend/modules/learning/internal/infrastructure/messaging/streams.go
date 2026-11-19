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
