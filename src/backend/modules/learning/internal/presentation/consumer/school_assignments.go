package consumer

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// AssignmentTracker applies the attempt events and the enrollment changes to the school assignments (command.Assignments).
type AssignmentTracker interface {
	ApplyAttemptEvent(ctx context.Context, e command.AttemptEvent) error
	SyncEnrollment(ctx context.Context, e command.EnrollmentChange) error
	AccessRevoked(ctx context.Context, studentID uuid.UUID) error
}

// AssignmentSubmissions is the consumer learning.assignment_submissions (F041-AC-15): activity.started/submitted/graded
// and attempt.re_evaluated of an attempt of an assignment update the submission of its student, once per event
// (processed_events; the submission is recomputed from the attempts).
type AssignmentSubmissions struct {
	Tracker AssignmentTracker
}

// AssignmentSubmissionsOptions returns the outbox consumer options (attempt events of module learning).
func AssignmentSubmissionsOptions(h AssignmentSubmissions, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "learning", Name: messaging.AssignmentSubmissionsName, Stream: messaging.AttemptStream,
		Subjects: messaging.AssignmentSubmissionsSubjects(), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c AssignmentSubmissions) Handle(ctx context.Context, e outbox.Envelope) error {
	var p struct {
		AttemptID    uuid.UUID  `json:"attempt_id"`
		StudentID    uuid.UUID  `json:"student_id"`
		ContextType  string     `json:"context_type"`
		ContextID    *uuid.UUID `json:"context_id"`
		AssignmentID *uuid.UUID `json:"assignment_id"`
	}
	if err := json.Unmarshal(e.Payload, &p); err != nil || p.AttemptID == uuid.Nil || e.TenantID == nil {
		return invalid(e.EventType, "payload has no attempt")
	}
	ev := command.AttemptEvent{Type: e.EventType, AttemptID: p.AttemptID, StudentID: p.StudentID, AssignmentID: p.AssignmentID}
	if ev.AssignmentID == nil && p.ContextType == "assignment" {
		ev.AssignmentID = p.ContextID
	}
	if ev.AssignmentID == nil && e.EventType != "attempt.re_evaluated" {
		return nil // not an attempt of an assignment
	}
	return c.Tracker.ApplyAttemptEvent(ctx, ev)
}

// AssignmentEnrollment is the consumer learning.assignment_enrollment (F041-AC-16): a student joining a class is added to
// the open assignments of the class; a locked account is excused before the due date.
type AssignmentEnrollment struct {
	Tracker AssignmentTracker
}

// AssignmentEnrollmentOptions returns the outbox consumer options (tenant events of module identity).
func AssignmentEnrollmentOptions(h AssignmentEnrollment, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "learning", Name: messaging.AssignmentEnrollmentName, Stream: natsx.EventStream("identity"),
		Subjects: messaging.AssignmentEnrollmentSubjects(), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c AssignmentEnrollment) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.TenantID == nil {
		return nil // a platform event: no assignment
	}
	if e.EventType == "user.access_revoked" {
		var p struct {
			UserID uuid.UUID `json:"user_id"`
		}
		if err := json.Unmarshal(e.Payload, &p); err != nil || p.UserID == uuid.Nil {
			return invalid(e.EventType, "payload has no user")
		}
		return c.Tracker.AccessRevoked(ctx, p.UserID)
	}
	var p struct {
		StudentID uuid.UUID `json:"student_id"`
		ClassID   uuid.UUID `json:"class_id"`
		Op        string    `json:"op"`
		Status    string    `json:"status"`
	}
	if err := json.Unmarshal(e.Payload, &p); err != nil || p.StudentID == uuid.Nil || p.ClassID == uuid.Nil {
		return invalid(e.EventType, "payload is not an enrollment")
	}
	return c.Tracker.SyncEnrollment(ctx, command.EnrollmentChange{StudentID: p.StudentID, ClassID: p.ClassID, Op: p.Op, Status: p.Status})
}
