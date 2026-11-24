package consumer

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// TournamentMatchHandler is what the consumer needs of command.Tournaments.
type TournamentMatchHandler interface {
	AdvanceOnMatch(ctx context.Context, tenant uuid.UUID, m command.MatchOutcome) (int, error)
	MatchAborted(ctx context.Context, tenant, matchID uuid.UUID) (int, error)
}

// TournamentMatch is the consumer competition.tournament_match: match.completed advances the bracket node of a
// tournament match (idempotent by match id and processed_events, F107-AC-17); match.aborted reschedules it (F107-AC-19).
// Matches outside a tournament change nothing.
type TournamentMatch struct{ H TournamentMatchHandler }

// TournamentMatchOptions returns its options.
func TournamentMatchOptions(h TournamentMatch, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "competition", Name: messaging.TournamentMatchName, Stream: messaging.CompetitionStream,
		Subjects: messaging.Subjects("competition", messaging.TournamentMatchEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c TournamentMatch) Handle(ctx context.Context, e outbox.Envelope) error {
	tenant, err := envTenant(e)
	if err != nil {
		return outbox.Permanent(err)
	}
	switch e.EventType {
	case "match.completed":
		m, err := DecodeMatchCompleted(e)
		if err != nil {
			return outbox.Permanent(err)
		}
		if m.TournamentID == nil && m.Type != "tournament" {
			return nil
		}
		_, err = c.H.AdvanceOnMatch(ctx, tenant, command.MatchOutcome{MatchID: m.MatchID, Status: m.Status, Lines: m.Results, EndedAt: m.EndedAt})
		return err
	case "match.aborted":
		var p struct {
			MatchID uuid.UUID `json:"match_id"`
		}
		if err := decode(e, &p); err != nil || p.MatchID == uuid.Nil {
			return outbox.Permanent(invalid(e.EventType, "match_id"))
		}
		_, err := c.H.MatchAborted(ctx, tenant, p.MatchID)
		return err
	}
	return nil
}
