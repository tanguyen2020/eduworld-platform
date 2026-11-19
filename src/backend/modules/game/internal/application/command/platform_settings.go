package command

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gamesession"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// ErrInvalidPlatformCurfew: a platform_curfew value that is not two different "HH:MM" bounds; retrying cannot fix it.
var ErrInvalidPlatformCurfew = errors.New("platform settings: platform_curfew must be two different HH:MM bounds")

// PlatformSettings applies the platform settings of module identity that module game needs (platform_settings.changed,
// owner F015) to ref.platform_settings: today only platform_curfew (OQ F014-2). It runs inside the transaction the
// consumer game.platform_settings opened (SystemPlatformScope) and is the only writer of ref.platform_settings.
type PlatformSettings struct {
	Refs write.PlatformSettingWriter
}

// ApplyPlatformCurfew stores w as platform_curfew when sourceVersion is newer than the stored one (the envelope time:
// a late or redelivered older event never overrides a newer value). It returns whether the row changed.
func (h PlatformSettings) ApplyPlatformCurfew(ctx context.Context, w write.CurfewWindow, sourceVersion int64) (applied bool, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.ApplyPlatformCurfew")
	defer func() { done(err) }()
	win, werr := gamesession.NewWindow(w.Start, w.End, time.UTC)
	if werr != nil || win.Zero() {
		return false, ErrInvalidPlatformCurfew
	}
	raw, err := json.Marshal(write.CurfewWindow{Start: w.Start, End: w.End})
	if err != nil {
		return false, err
	}
	return h.Refs.UpsertPlatformSetting(ctx, write.RefPlatformSetting{Key: write.KeyPlatformCurfew, Value: raw, SourceVersion: sourceVersion})
}
