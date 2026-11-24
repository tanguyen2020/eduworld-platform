package gameadmin

import (
	"strconv"
	"time"

	"github.com/google/uuid"
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
	ETag            string         `json:"-"`
}

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
