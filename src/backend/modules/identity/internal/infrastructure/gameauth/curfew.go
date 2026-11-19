package gameauth

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/gamelogin"
)

// DefaultPlatformCurfew is platform_curfew (CONFIG_DEFAULTS, [Confirmed] ADR-0078): 21:30–06:30 ICT.
const DefaultPlatformCurfew = "21:30-06:30"

// PlatformCurfew implements port.CurfewSource with the platform curfew only. The curfew window of a school
// (it may only extend the curfew) is read from the school settings of F015, which will replace this source
// by one that unites both; until then every student, every grade and the common tenant get the platform
// value.
type PlatformCurfew struct {
	Curfew gamelogin.Curfew
}

var _ port.CurfewSource = PlatformCurfew{}

// NewPlatformCurfew parses window ("" = DefaultPlatformCurfew) in Asia/Ho_Chi_Minh (fixed UTC+7 when the
// time zone database is missing).
func NewPlatformCurfew(window string) (PlatformCurfew, error) {
	if window == "" {
		window = DefaultPlatformCurfew
	}
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.FixedZone("ICT", 7*3600)
	}
	c, err := gamelogin.ParseCurfew(window, loc)
	return PlatformCurfew{Curfew: c}, err
}

// EffectiveCurfew implements port.CurfewSource.
func (p PlatformCurfew) EffectiveCurfew(context.Context, uuid.UUID, *uuid.UUID) (gamelogin.Curfew, error) {
	return p.Curfew, nil
}
