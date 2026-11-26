package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/i18ntext"
)

// TranslationState is one row of game.config_translation_states (F082-AC-08).
type TranslationState struct {
	VersionID  uuid.UUID
	ConfigID   uuid.UUID
	ConfigType string
	State      i18ntext.State
	ComputedAt time.Time
}

// LatestVersion is the newest version of a config item with its translation state (nil when not computed yet).
type LatestVersion struct {
	Item    ConfigItem
	Version ConfigVersion
	State   *TranslationState
}

// TranslationJob is one row of game.translation_jobs.
type TranslationJob struct {
	ID             uuid.UUID
	Kind           string // export, import
	Status         string // queued, running, succeeded, failed
	Format         string // xliff, csv
	SourceLanguage string
	TargetLanguage string
	OnlyMissing    bool
	DryRun         bool
	ContentTypes   []string
	ContentIDs     []uuid.UUID
	InputAssetID   *uuid.UUID
	ResultKey      *string
	Report         *i18ntext.Report
	ErrorMessage   *string
	RequestedBy    uuid.UUID
	IdempotencyKey *string
	Attempts       int
	StartedAt      *time.Time
	FinishedAt     *time.Time
	CreatedAt      time.Time
}

// TranslationStore writes the translation tables of module game under the PlatformScope of the transaction
// (role game_platform; soft delete only, R21).
type TranslationStore interface {
	// State returns the translation state of a version (nil when not computed yet).
	State(ctx context.Context, versionID uuid.UUID) (*TranslationState, error)
	// SaveState inserts or replaces the translation state of a version.
	SaveState(ctx context.Context, s TranslationState) error
	// VersionsWithoutState returns console versions that have no translation state yet, oldest first (backfill).
	VersionsWithoutState(ctx context.Context, limit int) ([]ConfigVersion, error)
	// LatestVersions returns the newest version of the items of itemTypes (ids: only these config ids; nil = all), keyset
	// after config_id, at most limit rows.
	LatestVersions(ctx context.Context, itemTypes []string, ids []uuid.UUID, after *uuid.UUID, limit int) ([]LatestVersion, error)
	// InsertJob, JobByIdempotency, ClaimJob (FOR UPDATE SKIP LOCKED: a queued job or a running job whose lease expired)
	// and SaveJob manage game.translation_jobs.
	InsertJob(ctx context.Context, j TranslationJob) error
	JobByIdempotency(ctx context.Context, requestedBy uuid.UUID, key string) (*TranslationJob, error)
	ClaimJob(ctx context.Context, now time.Time, lease time.Duration) (*TranslationJob, error)
	SaveJob(ctx context.Context, j TranslationJob, at time.Time) error
}
