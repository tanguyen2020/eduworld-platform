package consumer

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// PartyTickets cancels the team ticket of a changed party (command.Matchmaking.PartyChanged).
type PartyTickets interface {
	PartyChanged(ctx context.Context, tenant, party uuid.UUID) (bool, error)
}

// MatchmakingParty is the consumer competition.matchmaking_party (F104-AC-15, TC-MM-004): a member who leaves, is
// kicked or joins, or a disband, cancels the waiting team ticket of the party (cancel_reason party_changed, pushed to
// every member) and, when proposed, its proposal (the other side goes back to its queue).
type MatchmakingParty struct{ H PartyTickets }

// MatchmakingPartyOptions returns its options (payloads of specs/events/payloads/competition, owner F100).
func MatchmakingPartyOptions(h MatchmakingParty, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "competition", Name: messaging.MatchmakingPartyName, Stream: messaging.CompetitionStream,
		Subjects: messaging.Subjects("competition", messaging.MatchmakingPartyEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c MatchmakingParty) Handle(ctx context.Context, e outbox.Envelope) error {
	var p struct {
		PartyID  uuid.UUID `json:"party_id"`
		TenantID uuid.UUID `json:"tenant_id"`
	}
	if err := decode(e, &p); err != nil {
		return outbox.Permanent(err)
	}
	tenant, err := envTenant(e)
	if err != nil || p.PartyID == uuid.Nil || (p.TenantID != uuid.Nil && p.TenantID != tenant) {
		return outbox.Permanent(invalid(e.EventType, "party_id and tenant"))
	}
	_, err = c.H.PartyChanged(ctx, tenant, p.PartyID)
	return err
}
