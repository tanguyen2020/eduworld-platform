package dto

import (
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/media"
)

// MediaVariantView is one variant of MediaAsset.
type MediaVariantView struct {
	Variant   string     `json:"variant"`
	URL       string     `json:"url"`
	Key       string     `json:"key"`
	MIME      string     `json:"mime_type,omitempty"`
	SizeBytes int64      `json:"size_bytes"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// MediaUsageView is one entry of MediaAsset.used_in.
type MediaUsageView struct {
	ItemType string    `json:"item_type"`
	ItemID   uuid.UUID `json:"item_id"`
	Version  int       `json:"version"`
}

// MediaView is MediaAsset.
type MediaView struct {
	AssetID          uuid.UUID          `json:"asset_id"`
	TenantID         *uuid.UUID         `json:"tenant_id,omitempty"`
	OwnerScope       string             `json:"owner_scope"`
	Kind             string             `json:"kind"`
	OriginalFilename string             `json:"original_filename"`
	MIME             string             `json:"mime_type"`
	SizeBytes        int64              `json:"size_bytes"`
	Width            *int               `json:"width,omitempty"`
	Height           *int               `json:"height,omitempty"`
	DurationMS       *int               `json:"duration_ms,omitempty"`
	SHA256           string             `json:"sha256"`
	Status           string             `json:"status"`
	RejectReason     *string            `json:"reject_reason,omitempty"`
	AltText          *string            `json:"alt_text,omitempty"`
	License          string             `json:"license"`
	Bucket           string             `json:"bucket"`
	ExamMaterial     bool               `json:"is_exam_material"`
	Variants         []MediaVariantView `json:"variants"`
	UsedIn           []MediaUsageView   `json:"used_in,omitempty"`
	CreatedAt        time.Time          `json:"created_at"`
}

// URLFunc returns the CDN URL of an object (signed with its expiry when private).
type URLFunc func(bucket, key string) (string, *time.Time)

// MediaViewOf maps an asset; urls builds the variant URLs (nil = no URL).
func MediaViewOf(a media.Asset, urls URLFunc) MediaView {
	v := MediaView{AssetID: a.ID, TenantID: a.Owner.TenantID, OwnerScope: string(a.Owner.Scope), Kind: string(a.Kind), OriginalFilename: a.OriginalFilename,
		MIME: a.MIME, SizeBytes: a.Size, Width: a.Width, Height: a.Height, DurationMS: a.DurationMS, SHA256: a.SHA256, Status: string(a.Status),
		RejectReason: a.RejectReason, AltText: a.AltText, License: a.License, Bucket: a.Bucket, ExamMaterial: a.ExamMaterial,
		Variants: []MediaVariantView{}, CreatedAt: a.CreatedAt}
	names := make([]string, 0, len(a.Variants))
	for n := range a.Variants {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		x := a.Variants[n]
		mv := MediaVariantView{Variant: n, Key: x.Key, MIME: x.MIME, SizeBytes: x.Bytes}
		if urls != nil {
			mv.URL, mv.ExpiresAt = urls(a.Bucket, x.Key)
		}
		v.Variants = append(v.Variants, mv)
	}
	return v
}
