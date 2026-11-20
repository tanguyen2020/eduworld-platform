package read

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// PlayTimeModel implements read.PlayTimeReader on identity.daily_play_time (OQ-17) in a READ ONLY session.
type PlayTimeModel struct {
	Session *entx.ReadSession
}

var _ read.PlayTimeReader = PlayTimeModel{}

// PlayedOn implements read.PlayTimeReader.
func (m PlayTimeModel) PlayedOn(ctx context.Context, tenant, user uuid.UUID, day settings.PlayDate) (time.Duration, error) {
	var ms int64
	// tenancyguard:ok projection identity.daily_play_time (no soft-delete) under the TenantScope (RLS) and explicit tenant_id
	_, err := SettingsModel(m).one(ctx, `SELECT played_ms FROM identity.daily_play_time WHERE tenant_id = $1 AND user_id = $2
		AND play_date = $3::date`, []any{tenant, user, day.String()}, &ms)
	return time.Duration(ms) * time.Millisecond, err
}
