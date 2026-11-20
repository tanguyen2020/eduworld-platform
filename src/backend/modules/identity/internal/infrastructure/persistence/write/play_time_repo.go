package write

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
)

// PlayTimeRepo implements write.PlayTimeWriter on identity.daily_play_time (OQ-17).
type PlayTimeRepo struct{}

var _ write.PlayTimeWriter = PlayTimeRepo{}

// AddPlayTime implements write.PlayTimeWriter.
func (PlayTimeRepo) AddPlayTime(ctx context.Context, tenant, user uuid.UUID, day settings.PlayDate, played time.Duration) error {
	tx, err := txOf(ctx)
	if err != nil {
		return err
	}
	ms := played.Milliseconds()
	// tenancyguard:ok projection identity.daily_play_time (no soft-delete) under the event TenantScope (RLS) and explicit tenant_id
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity.daily_play_time AS d (tenant_id, user_id, play_date, played_ms, updated_at)
		VALUES ($1, $2, $3::date, least($4::bigint, 86400000), now())
		ON CONFLICT (tenant_id, user_id, play_date) DO UPDATE SET played_ms = least(d.played_ms + $4::bigint, 86400000), updated_at = now()
		WHERE d.tenant_id = $1`, tenant, user, day.String(), ms); err != nil {
		return fmt.Errorf("daily play time: add: %w", err)
	}
	return nil
}
