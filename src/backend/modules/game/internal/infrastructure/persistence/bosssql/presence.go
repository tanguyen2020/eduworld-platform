package bosssql

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
)

// Presence implements port.OnlineMembers on game.game_sessions: a student is online while it has an active game session
// whose last heartbeat (sent by the realtime gateway for a live connection, F090) is after since. A reconnection from
// another device opens a new active session: the student is online again (F103-AC-11, ADR-0031).
type Presence struct{}

var _ port.OnlineMembers = Presence{}

// Online implements port.OnlineMembers.
func (Presence) Online(ctx context.Context, tenant uuid.UUID, students []uuid.UUID, since time.Time) (map[uuid.UUID]bool, error) {
	out := map[uuid.UUID]bool{}
	if len(students) == 0 {
		return out, nil
	}
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]string, len(students))
	for i, s := range students {
		list[i] = s.String()
	}
	// tenancyguard:ok tenant table game.game_sessions: explicit tenant_id + deleted_at IS NULL (RLS as well)
	rows, err := t.QueryContext(ctx, `SELECT DISTINCT student_id FROM game.game_sessions WHERE tenant_id = $1 AND student_id::text = ANY($2)
		AND ended_at IS NULL AND last_heartbeat_at >= $3 AND deleted_at IS NULL`, tenant, list, since)
	if err != nil {
		return nil, fmt.Errorf("online students: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}
