package consumer

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// CoplayRecorder is the RecordCoplayers use case (command.Coplayers).
type CoplayRecorder interface {
	Record(ctx context.Context, tenant, event uuid.UUID, kind string, at time.Time, ps []social.Coplayer) (int, error)
}

// Coplayers is the consumer chat.recent_coplayers (delta F095 for the client F111): party.activity_started (members of
// a party activity: context party, or coop for quest/boss/coop_challenge) and match.started (participants: context
// match) of module competition become the "Vừa chơi cùng" rows of social.recent_coplayers. Module chat never calls
// module competition (R29); payloads carry ids only and nothing is logged (R14).
type Coplayers struct{ H CoplayRecorder }

// CoplayersOptions returns its options.
func CoplayersOptions(h Coplayers, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "chat", Name: messaging.RecentCoplayersName, Stream: messaging.CompetitionStream,
		Subjects: messaging.Subjects("competition", messaging.RecentCoplayersEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c Coplayers) Handle(ctx context.Context, e outbox.Envelope) error {
	tenant, err := envTenant(e)
	if err != nil {
		return outbox.Permanent(err)
	}
	switch e.EventType {
	case "party.activity_started":
		var p struct {
			TenantID     uuid.UUID   `json:"tenant_id"`
			ActivityType string      `json:"activity_type"`
			MemberIDs    []uuid.UUID `json:"member_ids"`
			StartedAt    *time.Time  `json:"started_at"`
		}
		if err := decode(e, &p); err != nil {
			return outbox.Permanent(err)
		}
		if p.TenantID != tenant || len(p.MemberIDs) > 8 {
			return outbox.Permanent(invalid(e.EventType, "tenant_id/member_ids"))
		}
		ps := make([]social.Coplayer, len(p.MemberIDs))
		for i, id := range p.MemberIDs {
			ps[i] = social.Coplayer{StudentID: id, TenantID: tenant, Team: -1}
		}
		_, err := c.H.Record(ctx, tenant, e.EventID, social.CoplayContext(p.ActivityType), at(p.StartedAt, e), ps)
		return classify("", e, true, err)
	case "match.started":
		var p struct {
			Participants []participant `json:"participants"`
			StartedAt    *time.Time    `json:"started_at"`
		}
		if err := decode(e, &p); err != nil {
			return outbox.Permanent(err)
		}
		if len(p.Participants) > 40 {
			return outbox.Permanent(invalid(e.EventType, "participants"))
		}
		ps := make([]social.Coplayer, 0, len(p.Participants))
		for _, x := range p.Participants {
			team := -1
			if x.TeamNo != nil {
				team = *x.TeamNo
			}
			ps = append(ps, social.Coplayer{StudentID: x.StudentID, TenantID: x.TenantID, Team: team})
		}
		_, err := c.H.Record(ctx, tenant, e.EventID, social.CoplayMatch, at(p.StartedAt, e), ps)
		return classify("", e, true, err)
	}
	return nil
}
