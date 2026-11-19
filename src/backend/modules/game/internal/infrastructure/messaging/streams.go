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
	"student.synced", "user.status_changed", "student.cleaned_up", "student.restored"}

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
