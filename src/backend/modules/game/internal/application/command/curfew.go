package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gamesession"
)

// platformWindow returns platform_curfew in force: the value projected from platform_settings.changed (module
// identity, F015) when there is a valid one, otherwise the start-up default def (EW_PLATFORM_CURFEW). A malformed
// projected value cannot come from the consumer (it validates); it is ignored rather than lifting the curfew.
func platformWindow(def gamesession.Window, projected *write.CurfewWindow, loc *time.Location) gamesession.Window {
	if projected == nil {
		return def
	}
	w, err := gamesession.NewWindow(projected.Start, projected.End, loc)
	if err != nil || w.Zero() {
		return def
	}
	return w
}

// effectiveCurfew is the effective curfew of a student: platform ∪ school window (ADR-0078). A malformed school
// window is ignored: the platform curfew still applies.
func effectiveCurfew(platform gamesession.Window, school *write.CurfewWindow, loc *time.Location) gamesession.Curfew {
	c := gamesession.Curfew{platform}
	if school == nil {
		return c
	}
	if sw, err := gamesession.NewWindow(school.Start, school.End, loc); err == nil {
		c = append(c, sw)
	}
	return c
}

// platformCurfew reads platform_curfew in force inside the transaction of ctx.
func (h Sessions) platformCurfew(ctx context.Context) (gamesession.Window, error) {
	p, err := h.Directory.PlatformCurfew(ctx)
	if err != nil {
		return gamesession.Window{}, err
	}
	return platformWindow(h.Config.PlatformCurfew, p, h.Config.CurfewLocation), nil
}

// curfew returns the effective curfew of a student of school (createGameSession).
func (h Sessions) curfew(ctx context.Context, tenant uuid.UUID, school uuid.UUID) (gamesession.Curfew, error) {
	p, err := h.platformCurfew(ctx)
	if err != nil {
		return nil, err
	}
	return h.curfewWith(ctx, p, tenant, school)
}

// curfewWith returns the effective curfew of a student of school for an already read platform window (the job
// curfew-session-closer reads platform_curfew once per page).
func (h Sessions) curfewWith(ctx context.Context, platform gamesession.Window, tenant, school uuid.UUID) (gamesession.Curfew, error) {
	w, err := h.Directory.SchoolCurfew(ctx, tenant, school)
	if err != nil {
		return nil, err
	}
	return effectiveCurfew(platform, w, h.Config.CurfewLocation), nil
}
