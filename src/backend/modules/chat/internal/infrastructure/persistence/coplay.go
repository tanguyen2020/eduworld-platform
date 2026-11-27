package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
)

// Coplay implements write.CoplayStore on social.recent_coplayers (delta F095) in the transaction of ctx.
type Coplay struct{}

var _ write.CoplayStore = Coplay{}

// Touch implements write.CoplayStore: one statement for every pair; a row keeps its newest time (a late, older event
// changes nothing).
func (Coplay) Touch(ctx context.Context, tenant uuid.UUID, pairs []social.CoplayPair, kind string, at time.Time, event uuid.UUID) (int, error) {
	if len(pairs) == 0 {
		return 0, nil
	}
	users, others := make([]string, len(pairs)), make([]string, len(pairs))
	for i, p := range pairs {
		users[i], others[i] = p.Student.String(), p.Other.String()
	}
	// tenancyguard:ok social.recent_coplayers upsert of the event tenant (RLS on tenant_id); the conflict target is the live row of the pair
	n, err := exec(ctx, `INSERT INTO social.recent_coplayers AS c (tenant_id, user_id, other_id, context_kind, last_played_at, source_event_id)
		SELECT $1, u::uuid, o::uuid, $4, $5, $6 FROM unnest($2::text[], $3::text[]) AS t(u, o)
		ON CONFLICT (tenant_id, user_id, other_id) DO UPDATE SET context_kind = EXCLUDED.context_kind,
			last_played_at = EXCLUDED.last_played_at, source_event_id = EXCLUDED.source_event_id, updated_at = now()
		WHERE c.last_played_at < EXCLUDED.last_played_at AND c.tenant_id = $1 AND c.deleted_at IS NULL`,
		tenant, users, others, kind, at, event)
	if err != nil {
		return 0, fmt.Errorf("recent coplayers: %w", err)
	}
	return int(n), nil
}
