package gameadmin

import (
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/i18ntext"
)

// ConfigVersionView is GameConfigVersion of the contract; ETag is the header of the response.
type ConfigVersionView struct {
	ConfigType      string         `json:"config_type"`
	ConfigID        uuid.UUID      `json:"config_id"`
	ConfigVersionID uuid.UUID      `json:"config_version_id"`
	Version         int            `json:"version"`
	Status          string         `json:"status"`
	Title           *string        `json:"title,omitempty"`
	Payload         map[string]any `json:"payload"`
	AuthorID        *uuid.UUID     `json:"author_id,omitempty"`
	ReviewerID      *uuid.UUID     `json:"reviewer_id,omitempty"`
	ReviewNote      *string        `json:"review_note,omitempty"`
	ChangeNote      *string        `json:"change_note,omitempty"`
	BasedOnVersion  *int           `json:"based_on_version,omitempty"`
	IsCurrent       bool           `json:"is_current"`
	PublishedAt     *time.Time     `json:"published_at,omitempty"`
	UpdatedAt       time.Time      `json:"updated_at"`
	ThumbnailURL    *string        `json:"thumbnail_url,omitempty"`
	TurntableURL    *string        `json:"turntable_url,omitempty"`
	// RenderStatus is the state of the render_thumbnail of the version (gameconfig.ThumbnailState, F079-AC-16).
	RenderStatus string `json:"render_status,omitempty"`
	// CalendarWarnings is set (possibly empty) only by listGameConfigs events with a from/to window (F079-AC-14).
	CalendarWarnings *[]CalendarWarning `json:"calendar_warnings,omitempty"`
	// TranslationStatus, MissingFields and OutdatedFields are the translation state of the version (F082-AC-08).
	TranslationStatus *string         `json:"translation_status,omitempty"`
	MissingFields     []string        `json:"missing_fields,omitempty"`
	OutdatedFields    []OutdatedField `json:"outdated_fields,omitempty"`
	ETag              string          `json:"-"`
}

// OutdatedField is GameConfigVersion.outdated_fields[].
type OutdatedField struct {
	Pointer             string `json:"pointer"`
	TranslatedInVersion int    `json:"translated_in_version"`
}

// CalendarWarning is GameEventCalendarWarning of the contract.
type CalendarWarning struct {
	Code          string    `json:"code"`
	OtherConfigID uuid.UUID `json:"other_config_id"`
	OtherTitle    *string   `json:"other_title,omitempty"`
	AreaID        string    `json:"area_id,omitempty"`
}

// CalendarWarningAreaOverlap is the code of two events decorating one area at overlapping times (ADMIN_PORTAL §3.15).
const CalendarWarningAreaOverlap = "decoration_area_overlap"

// CalendarWarningsMax is maxItems of calendar_warnings per event.
const CalendarWarningsMax = 20

// ReleaseView is ContentRelease of the contract.
type ReleaseView struct {
	ReleaseID    uuid.UUID   `json:"release_id"`
	ItemType     string      `json:"item_type"`
	ItemID       uuid.UUID   `json:"item_id"`
	Action       string      `json:"action"`
	FromVersion  *int        `json:"from_version,omitempty"`
	ToVersion    *int        `json:"to_version,omitempty"`
	Reason       *string     `json:"reason,omitempty"`
	ScheduledFor *time.Time  `json:"scheduled_for,omitempty"`
	Status       string      `json:"status"`
	FailureCode  *string     `json:"failure_code,omitempty"`
	ReleasedBy   uuid.UUID   `json:"released_by"`
	ReleasedAt   time.Time   `json:"released_at"`
	Usages       []UsageView `json:"usages,omitempty"`
	// Pending: the publication is scheduled (202).
	Pending bool `json:"-"`
}

// UsageView is one entry of usages[].
type UsageView struct {
	Kind     string     `json:"kind"`
	ItemType string     `json:"item_type"`
	ItemID   string     `json:"item_id"`
	TenantID *uuid.UUID `json:"tenant_id,omitempty"`
}

// ETagOf is the entity tag of a version row (its optimistic revision).
func ETagOf(revision int64) string { return `"` + strconv.FormatInt(revision, 10) + `"` }

// TranslationJobView is TranslationJob of the contract (download_url/expires_at are signed by the query, F082).
type TranslationJobView struct {
	JobID          uuid.UUID        `json:"job_id"`
	Kind           string           `json:"kind"`
	Status         string           `json:"status"`
	Format         string           `json:"format"`
	TargetLanguage string           `json:"target_language"`
	DryRun         bool             `json:"dry_run"`
	CreatedAt      time.Time        `json:"created_at"`
	FinishedAt     *time.Time       `json:"finished_at,omitempty"`
	ErrorMessage   *string          `json:"error_message,omitempty"`
	DownloadURL    *string          `json:"download_url,omitempty"`
	ExpiresAt      *time.Time       `json:"expires_at,omitempty"`
	Report         *i18ntext.Report `json:"report,omitempty"`
}

// BucketExports is the private bucket of generated files (ADR-0044); TranslationKeyPrefix the folder of translation files.
const (
	BucketExports        = "ew-exports"
	TranslationKeyPrefix = "translations/"
)
