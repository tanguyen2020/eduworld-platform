package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// Coplayers records who played with whom ("Vừa chơi cùng" of listFriendSuggestions, delta F095 for the client F111)
// from the party.activity_started and match.started events of module competition (consumer chat.recent_coplayers; R29:
// module chat never calls competition, the event carries the participants). It runs in the transaction of the consumer
// (processed_events in the same transaction); the upsert is idempotent. No audit (a projection, not a user action) and
// no event; ids only (R14, R16).
type Coplayers struct {
	UoW   write.UnitOfWork
	Store write.CoplayStore
}

// Record stores the pairs of the participants of tenant for one shared activity.
func (h Coplayers) Record(ctx context.Context, tenant, event uuid.UUID, kind string, at time.Time, ps []social.Coplayer) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.RecordCoplayers")
	defer func() { done(err) }()
	pairs := social.CoplayPairs(tenant, ps)
	if len(pairs) == 0 {
		return 0, nil
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		n, err = h.Store.Touch(ctx, tenant, pairs, kind, at.UTC(), event)
		return err
	})
	return n, err
}
