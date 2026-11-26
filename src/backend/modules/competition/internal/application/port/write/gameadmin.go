package write

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/gameconfig"
)

// ErrConfigConflict: a configuration of the type with the code already exists (unique (config_type, code)).
var ErrConfigConflict = errors.New("competition: configuration already exists")

// GameConfigStore is the write side of the competition configurations (F108). Every statement runs in the transaction
// of ctx under the platform pool (competition_platform: the tenant-serving role only reads these tables).
type GameConfigStore interface {
	ItemByID(ctx context.Context, configType string, id uuid.UUID) (*port.ConfigItem, error)
	ItemByCode(ctx context.Context, configType, code string) (*port.ConfigItem, error)
	// CreateItem inserts a configuration (ErrConfigConflict when the code exists).
	CreateItem(ctx context.Context, it port.ConfigItem, by uuid.UUID, now time.Time) error
	// NextVersionNo reserves the next version number of the configuration (latest_version_no + 1, row locked).
	NextVersionNo(ctx context.Context, configID uuid.UUID, by uuid.UUID, now time.Time) (int, error)
	InsertVersion(ctx context.Context, v port.ConfigVersion, by uuid.UUID, now time.Time) error
	Version(ctx context.Context, configID uuid.UUID, no int) (*port.ConfigVersion, error)
	VersionByID(ctx context.Context, configID, id uuid.UUID) (*port.ConfigVersion, error)
	// UpdateDraft / SetWorkflow / Publish change a version when its revision is still rev (false otherwise → 412).
	UpdateDraft(ctx context.Context, v port.ConfigVersion, rev int64, by uuid.UUID, now time.Time) (bool, error)
	SetWorkflow(ctx context.Context, v port.ConfigVersion, rev int64, by uuid.UUID, now time.Time) (bool, error)
	Publish(ctx context.Context, v port.ConfigVersion, rev int64, by uuid.UUID, now time.Time) (bool, error)
	MovePointer(ctx context.Context, configID uuid.UUID, to *uuid.UUID, by uuid.UUID, now time.Time) error
	InsertRelease(ctx context.Context, r port.ConfigRelease) error
	// SeasonStates lists the seasons already in competition.seasons (code, status, period).
	SeasonStates(ctx context.Context) ([]gameconfig.SeasonState, error)
}

// SeasonSync is one season of the seasons configuration in effect, applied to competition.seasons by code.
type SeasonSync struct {
	ID       uuid.UUID
	Code     string
	Name     string
	Theme    *string
	StartsAt time.Time
	EndsAt   time.Time
}

// ConfigApplyStore applies the configurations in effect to the runtime tables (F108-AC-08).
type ConfigApplyStore interface {
	// EffectivePayload returns the version of (type, default) in effect at at by the release timeline (nil when none).
	EffectivePayload(ctx context.Context, configType string, at time.Time) (*uuid.UUID, []byte, error)
	// UpsertSeason inserts a new season (scheduled) or updates an existing one by code: an ended season never changes;
	// an active one only its name, theme and end; a scheduled one everything. Reports whether a row changed.
	UpsertSeason(ctx context.Context, s SeasonSync, version uuid.UUID, now time.Time) (bool, error)
}
