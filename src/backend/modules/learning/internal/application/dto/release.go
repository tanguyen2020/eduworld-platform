package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ContentReleaseView is schema ContentRelease (F026): a publish, rollback or unpublish, or a scheduled publication.
type ContentReleaseView struct {
	ReleaseID    uuid.UUID  `json:"release_id"`
	ItemType     string     `json:"item_type"`
	ItemID       uuid.UUID  `json:"item_id"`
	Action       string     `json:"action"`
	FromVersion  *int       `json:"from_version,omitempty"`
	ToVersion    *int       `json:"to_version,omitempty"`
	Reason       string     `json:"reason,omitempty"`
	ScheduledFor *time.Time `json:"scheduled_for,omitempty"`
	Status       string     `json:"status,omitempty"`
	FailureCode  string     `json:"failure_code,omitempty"`
	ReleasedBy   uuid.UUID  `json:"released_by"`
	ReleasedAt   time.Time  `json:"released_at"`
	Scheduled    bool       `json:"-"` // 202 rather than 200
}

// VersionSummary is schema ContentVersionSummary.
type VersionSummary struct {
	Version              int        `json:"version"`
	ContentVersionID     uuid.UUID  `json:"content_version_id"`
	Status               string     `json:"status"`
	IsCurrent            bool       `json:"is_current"`
	ChangeNote           *string    `json:"change_note,omitempty"`
	AuthorID             uuid.UUID  `json:"author_id"`
	ReviewerID           *uuid.UUID `json:"reviewer_id,omitempty"`
	PublishedBy          *uuid.UUID `json:"published_by,omitempty"`
	PublishedAt          *time.Time `json:"published_at,omitempty"`
	PublishAt            *time.Time `json:"publish_at,omitempty"`
	ActivePinnedSessions int        `json:"active_pinned_sessions"`
}

// ItemRef is the release state of an item.
type ItemRef struct {
	ID               uuid.UUID
	CurrentVersionID *uuid.UUID
	LatestVersionNo  int
	TenantID         *uuid.UUID
}

// ReleaseRecord is a row of content.content_releases for the history.
type ReleaseRecord struct {
	ID            uuid.UUID
	ItemType      string
	ItemID        uuid.UUID
	Action        string
	FromVersionID *uuid.UUID
	ToVersionID   *uuid.UUID
	Reason        *string
	ScheduledFor  *time.Time
	ReleasedBy    uuid.UUID
	ReleasedAt    time.Time
}

// ContentUsageView is schema ContentUsage (F026-AC-27).
type ContentUsageView struct {
	RefType        string     `json:"ref_type"`
	RefID          uuid.UUID  `json:"ref_id"`
	RefVersion     int        `json:"ref_version,omitempty"`
	RefVersionID   *uuid.UUID `json:"ref_version_id,omitempty"`
	UsedVersion    int        `json:"used_version,omitempty"`
	Title          string     `json:"title,omitempty"`
	PinnedSessions int        `json:"pinned_sessions"`
}

// ContentVersionDetail is schema ContentVersionDetail (getContentVersion).
type ContentVersionDetail struct {
	ContentVersionView
	Content json.RawMessage `json:"content"`
}
