package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
)

// CoplayStore keeps social.recent_coplayers ("Vừa chơi cùng" of listFriendSuggestions, delta F095) inside the
// transaction of the consumer.
type CoplayStore interface {
	// Touch upserts the pairs of tenant with context kind at at (an older time never overwrites a newer one); event is the
	// source event id. It returns the number of rows written.
	Touch(ctx context.Context, tenant uuid.UUID, pairs []social.CoplayPair, kind string, at time.Time, event uuid.UUID) (int, error)
}
