package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// EventManifestUpdated is the event of a rebuilt tenant manifest (specs/events/payloads/game/content.manifest_updated.v1).
const EventManifestUpdated = "content.manifest_updated"

// ManifestMetrics observes eduworld_content_publish_to_visible_seconds{item_type}; nil = not observed.
type ManifestMetrics interface {
	PublishToVisible(itemType string, d time.Duration)
}

// Manifests merges the learning items released by module learning into the content manifest of each target tenant
// (F026 US6, CONTENT_MODEL §5.3, DATABASE_DESIGN §3.13): every change writes a new manifest_version with a new ETag.
type Manifests struct {
	UoW     write.UnitOfWork
	Store   write.ManifestStore
	Events  write.EventPublisher
	Metrics ManifestMetrics
	Now     func() time.Time
}

// LearningRelease is a release of a learning item (payload of content.published/rolled_back/unpublished).
type LearningRelease struct {
	ItemType         string
	ItemID           uuid.UUID
	VersionID        *uuid.UUID // nil = unpublished
	VersionNo        int
	SnapshotURL      string
	SHA256           string
	ActivityType     string
	MinClientVersion string
	ReleaseID        uuid.UUID
	ReleasedAt       time.Time
	Tenants          []uuid.UUID
}

// ManifestItem is ContentManifestItem.
type ManifestItem struct {
	ItemType         string    `json:"item_type"`
	ItemID           uuid.UUID `json:"item_id"`
	VersionID        uuid.UUID `json:"version_id"`
	VersionNo        int       `json:"version_no"`
	SnapshotURL      string    `json:"snapshot_url,omitempty"`
	SHA256           string    `json:"sha256"`
	ActivityType     string    `json:"activity_type,omitempty"`
	MinClientVersion string    `json:"min_client_version,omitempty"`
}

func (h Manifests) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// Merge applies a learning release to the manifest of every target tenant (F026-AC-23, AC-25, AC-26): the item entry
// is replaced (or removed on unpublish); other entries (game content, bundles) are kept.
func (h Manifests) Merge(ctx context.Context, r LearningRelease) (err error) {
	ctx, done := observability.UseCase(ctx, "game.command.MergeLearningManifestEntries")
	defer func() { done(err) }()
	for _, tenant := range r.Tenants {
		sc, err := authz.EventScope(tenant)
		if err != nil {
			return err
		}
		if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error { return h.mergeOne(ctx, tenant, r) }); err != nil {
			return err
		}
	}
	return nil
}

func (h Manifests) mergeOne(ctx context.Context, tenant uuid.UUID, r LearningRelease) error {
	version, entries, err := h.Store.Latest(ctx, tenant)
	if err != nil {
		return err
	}
	doc := map[string]json.RawMessage{}
	if len(entries) > 0 {
		_ = json.Unmarshal(entries, &doc)
	}
	var items []json.RawMessage
	_ = json.Unmarshal(doc["items"], &items)
	kept := make([]json.RawMessage, 0, len(items)+1)
	for _, raw := range items {
		var it struct {
			ItemType string    `json:"item_type"`
			ItemID   uuid.UUID `json:"item_id"`
		}
		if json.Unmarshal(raw, &it) == nil && it.ItemType == r.ItemType && it.ItemID == r.ItemID {
			continue
		}
		kept = append(kept, raw)
	}
	if r.VersionID != nil {
		b, err := json.Marshal(ManifestItem{ItemType: r.ItemType, ItemID: r.ItemID, VersionID: *r.VersionID, VersionNo: r.VersionNo,
			SnapshotURL: r.SnapshotURL, SHA256: r.SHA256, ActivityType: r.ActivityType, MinClientVersion: r.MinClientVersion})
		if err != nil {
			return err
		}
		kept = append(kept, b)
	}
	if doc["items"], err = json.Marshal(kept); err != nil {
		return err
	}
	if _, ok := doc["bundles"]; !ok {
		doc["bundles"] = json.RawMessage("[]")
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(append(b, []byte(tenant.String())...))
	etag := hex.EncodeToString(sum[:])
	at := h.now()
	next := version + 1
	if err := h.Store.Insert(ctx, write.ManifestRow{TenantID: tenant, Version: next, ETag: etag, Entries: b, GeneratedAt: at,
		CauseReleaseID: &r.ReleaseID}); err != nil {
		return err
	}
	if h.Metrics != nil && !r.ReleasedAt.IsZero() {
		h.Metrics.PublishToVisible(r.ItemType, at.Sub(r.ReleasedAt))
	}
	t := tenant
	return h.Events.Publish(ctx, write.Event{Type: EventManifestUpdated, SubjectType: "content_manifest", SubjectID: tenant.String(), TenantID: &t,
		Payload: map[string]any{"tenant_id": tenant, "manifest_version": next, "etag": etag, "generated_at": at, "cause_release_id": r.ReleaseID}})
}
