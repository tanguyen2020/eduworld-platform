package consumer

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// ActivityEnder ends a party activity in the transaction of the consumer (command.Parties.EndPartyActivity).
type ActivityEnder interface {
	EndPartyActivity(ctx context.Context, tenant, activityID uuid.UUID, reason string, summary map[string]any) (bool, error)
}

// PartyActivityEnd is the consumer competition.party_activity_end (F101-AC-12, AC-13, TC-SVC-003): the end of the
// encounter or of the shared quest session of a party activity ends it once (processed_events); events of a solo
// encounter (no party_activity_id) are ignored. Only the shared outcome is kept: contributions, scores and results of
// the students never enter the party activity (FR-PTY-013).
type PartyActivityEnd struct{ H ActivityEnder }

// PartyActivityEndOptions returns its options (payload read per EVENT_DESIGN §4; schemas owned by F103).
func PartyActivityEndOptions(h PartyActivityEnd, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "competition", Name: messaging.PartyActivityEndName, Stream: messaging.GameStream,
		Subjects: messaging.Subjects("game", messaging.PartyActivityEndEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c PartyActivityEnd) Handle(ctx context.Context, e outbox.Envelope) error {
	var p struct {
		PartyActivityID *uuid.UUID `json:"party_activity_id"`
		Result          *string    `json:"result"`
		Outcome         *string    `json:"outcome"`
	}
	if err := decode(e, &p); err != nil {
		return outbox.Permanent(err)
	}
	if p.PartyActivityID == nil || *p.PartyActivityID == uuid.Nil {
		return nil // solo encounter / quest outside a party session
	}
	if !maxLen(p.Result, 64) || !maxLen(p.Outcome, 64) {
		return outbox.Permanent(invalid(e.EventType, "result/outcome"))
	}
	tenant, err := envTenant(e)
	if err != nil {
		return outbox.Permanent(err)
	}
	summary := map[string]any{}
	reason := party.EndCompleted
	switch e.EventType {
	case "boss.encounter_completed":
		if p.Result != nil {
			summary["result"] = *p.Result
		}
	case "quest.party_completed":
		if p.Outcome != nil {
			summary["outcome"] = *p.Outcome
			switch *p.Outcome {
			case "abandoned":
				reason = party.EndAllLeft
			case "failed":
				reason = party.EndTimeout // the shared quest failed (time or condition): the session ends without completion
			}
		}
	default:
		return nil
	}
	_, err = c.H.EndPartyActivity(ctx, tenant, *p.PartyActivityID, reason, summary)
	return classify("", e, false, err)
}
