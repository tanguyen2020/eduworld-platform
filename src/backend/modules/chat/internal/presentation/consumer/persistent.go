package consumer

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/club"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// DirectFreezer freezes/reopens the direct channel of a pair (command.Persistent.SetDirectFrozen).
type DirectFreezer interface {
	SetDirectFrozen(ctx context.Context, tenant, a, b, source uuid.UUID, on bool) (bool, error)
}

// Membership is the consumer chat.membership (F120-AC-15, AC-04): user.blocked freezes the direct channel of the pair,
// user.unblocked reopens it when both are still friends. A redelivered event changes nothing (processed_events + the
// status transition).
type Membership struct{ H DirectFreezer }

// MembershipOptions returns its options (EVT_CHAT, events of module chat itself).
func MembershipOptions(h Membership, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "chat", Name: messaging.MembershipName, Stream: messaging.ChatStream,
		Subjects: messaging.Subjects("chat", messaging.MembershipEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c Membership) Handle(ctx context.Context, e outbox.Envelope) error {
	var p struct {
		TenantID  uuid.UUID `json:"tenant_id"`
		BlockerID uuid.UUID `json:"blocker_id"`
		BlockedID uuid.UUID `json:"blocked_id"`
	}
	if err := decode(e, &p); err != nil {
		return outbox.Permanent(err)
	}
	tenant, err := envTenant(e)
	if err != nil {
		return outbox.Permanent(err)
	}
	if p.TenantID != tenant || p.BlockerID == uuid.Nil || p.BlockedID == uuid.Nil || p.BlockerID == p.BlockedID {
		return outbox.Permanent(invalid(e.EventType, "tenant_id/blocker_id/blocked_id"))
	}
	_, err = c.H.SetDirectFrozen(ctx, tenant, p.BlockerID, p.BlockedID, e.EventID, e.EventType == "user.blocked")
	return err
}

// GoalRecorder adds the contribution of one verified event to the club goals (command.Clubs.RecordProgress).
type GoalRecorder interface {
	RecordProgress(ctx context.Context, tenant, student uuid.UUID, metric string, delta int, event uuid.UUID, at time.Time,
		loc *time.Location) (int, error)
}

// ClubGoals is the consumer of the club goals (F120-AC-21): activity.graded (learning), quest.completed and
// world.discovery_found (game) count one unit for the active goals of the clubs of the student — never a score or a
// mastery value is read or stored.
type ClubGoals struct {
	H   GoalRecorder
	Loc *time.Location // week of the goals (policy timezone)
}

// ClubGoalsLearningOptions / ClubGoalsGameOptions return the options of the two durable consumers.
func ClubGoalsLearningOptions(h ClubGoals, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "chat", Name: messaging.ClubGoalsLearningName, Stream: messaging.LearningStream,
		Subjects: messaging.Subjects("learning", messaging.ClubGoalsLearningEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// ClubGoalsGameOptions returns the options of the game consumer.
func ClubGoalsGameOptions(h ClubGoals, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "chat", Name: messaging.ClubGoalsGameName, Stream: messaging.GameStream,
		Subjects: messaging.Subjects("game", messaging.ClubGoalsGameEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

var goalMetric = map[string]string{"activity.graded": club.MetricActivities, "quest.completed": club.MetricQuests,
	"world.discovery_found": club.MetricDiscoveries}

// Handle implements outbox.Handler.
func (c ClubGoals) Handle(ctx context.Context, e outbox.Envelope) error {
	metric, ok := goalMetric[e.EventType]
	if !ok {
		return nil
	}
	var p struct {
		StudentID uuid.UUID `json:"student_id"`
	}
	if err := decode(e, &p); err != nil {
		return outbox.Permanent(err)
	}
	tenant, err := envTenant(e)
	if err != nil {
		return outbox.Permanent(err)
	}
	if p.StudentID == uuid.Nil {
		return outbox.Permanent(invalid(e.EventType, "student_id"))
	}
	_, err = c.H.RecordProgress(ctx, tenant, p.StudentID, metric, 1, e.EventID, e.OccurredAt.UTC(), c.Loc)
	return err
}
