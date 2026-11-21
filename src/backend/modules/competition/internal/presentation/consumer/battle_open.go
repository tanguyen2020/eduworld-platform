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

// RoomOpener opens the room of a match on a core-api replica (battlehost.Host.Open).
type RoomOpener interface {
	Open(ctx context.Context, matchID, tenant uuid.UUID) error
}

// BattleOpen is the consumer competition.battle_open (F105-AC-01, F104 hand-off): match.created → the room host opens
// the match room (question plan pinned before the countdown). A platform match of several tenants (F107 only) carries
// no tenant in its envelope: it is refused as permanent (DLQ) until F107 brings the platform question pool.
type BattleOpen struct{ R RoomOpener }

// BattleOpenOptions returns its options (payload match.created.v1, owner F104).
func BattleOpenOptions(h BattleOpen, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "competition", Name: messaging.BattleOpenName, Stream: messaging.CompetitionStream,
		Subjects: messaging.Subjects("competition", messaging.BattleOpenEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler: an error redelivers (no core-api replica answered).
func (c BattleOpen) Handle(ctx context.Context, e outbox.Envelope) error {
	var p struct {
		MatchID uuid.UUID `json:"match_id"`
	}
	if err := decode(e, &p); err != nil {
		return outbox.Permanent(err)
	}
	tenant, err := envTenant(e)
	if err != nil || p.MatchID == uuid.Nil {
		return outbox.Permanent(invalid(e.EventType, "match_id and tenant"))
	}
	return c.R.Open(ctx, p.MatchID, tenant)
}
