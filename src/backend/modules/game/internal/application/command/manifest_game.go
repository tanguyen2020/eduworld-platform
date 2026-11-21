package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/world"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// gameItemTypes are the manifest item types owned by the game release core (replaced on every game rebuild).
var gameItemTypes = []string{write.ItemWorldArea, write.ItemStoryChapter, write.ItemNPCDialogue, write.ItemLevel}

// GameManifest rebuilds the game part of the content manifest of every active tenant when game content is released
// (F071-AC-05, ADR-0043 §6): items of the game types, areas and bundles are replaced by the published content; the
// learning entries are kept. Each rebuild writes a new manifest_version with a new ETag and content.manifest_updated.
type GameManifest struct {
	UoW     write.UnitOfWork
	Store   write.ManifestStore
	Tenants write.TenantLister
	Content port.WorldContent
	Events  write.EventPublisher
	Metrics ManifestMetrics
	// BuildSeconds observes eduworld_game_manifest_build_seconds; nil = not observed.
	BuildSeconds func(seconds float64)
	Now          func() time.Time
}

type gameEntries struct {
	items   []json.RawMessage
	areas   []map[string]any
	bundles []map[string]any
}

// Rebuild runs in the platform transaction of a game content.published event.
func (h GameManifest) Rebuild(ctx context.Context, cause uuid.UUID, itemType string, releasedAt time.Time) (err error) {
	ctx, done := observability.UseCase(ctx, "game.command.RebuildGameManifest")
	defer func() { done(err) }()
	start := time.Now()
	defer func() {
		if err == nil && h.BuildSeconds != nil {
			h.BuildSeconds(time.Since(start).Seconds())
		}
	}()
	tenants, err := h.Tenants.ActiveTenants(ctx)
	if err != nil {
		return err
	}
	entries, err := h.entries(ctx)
	if err != nil {
		return err
	}
	for _, tenant := range tenants {
		sc, err := authz.EventScope(tenant)
		if err != nil {
			return err
		}
		if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
			return h.rebuildOne(ctx, tenant, entries, cause, itemType, releasedAt)
		}); err != nil {
			return err
		}
	}
	return nil
}

func manifestItem(itemType string, id uuid.UUID, code string, versionID uuid.UUID, versionNo int, sha string) (json.RawMessage, error) {
	m := map[string]any{"item_type": itemType, "item_id": id, "version_id": versionID, "version_no": versionNo, "sha256": sha}
	if code != "" {
		m["item_code"] = code
	}
	return json.Marshal(m)
}

func (h GameManifest) entries(ctx context.Context) (gameEntries, error) {
	var e gameEntries
	areas, err := h.Content.CurrentAreas(ctx)
	if err != nil {
		return e, err
	}
	for _, a := range areas {
		it, err := manifestItem(write.ItemWorldArea, world.ContentItemID(write.ItemWorldArea, a.Area.AreaID), a.Area.AreaID, a.VersionID, a.VersionNo, a.SHA256)
		if err != nil {
			return e, err
		}
		e.items = append(e.items, it)
		e.areas = append(e.areas, map[string]any{"area_id": a.Area.AreaID, "scene_ref": a.Area.SceneRef, "bundle_id": a.Area.BundleID})
	}
	chapters, err := h.Content.CurrentChapters(ctx)
	if err != nil {
		return e, err
	}
	for _, c := range chapters {
		it, err := manifestItem(write.ItemStoryChapter, world.ContentItemID(write.ItemStoryChapter, c.Chapter.Code), c.Chapter.Code, c.VersionID, c.VersionNo, c.SHA256)
		if err != nil {
			return e, err
		}
		e.items = append(e.items, it)
	}
	dialogues, err := h.Content.CurrentDialogues(ctx, "")
	if err != nil {
		return e, err
	}
	for _, d := range dialogues {
		it, err := manifestItem(write.ItemNPCDialogue, d.DialogueID, d.NPCID, d.VersionID, d.VersionNo, d.SHA256)
		if err != nil {
			return e, err
		}
		e.items = append(e.items, it)
	}
	levels, err := h.Content.CurrentLevels(ctx)
	if err != nil {
		return e, err
	}
	for _, l := range levels {
		it, err := manifestItem(write.ItemLevel, l.LevelID, l.Level.AreaID, l.VersionID, l.VersionNo, l.SHA256)
		if err != nil {
			return e, err
		}
		e.items = append(e.items, it)
		b := map[string]any{"bundle_id": l.Level.BundleID, "version": l.VersionNo, "url": l.Level.BundleURL, "sha256": l.Level.BundleSHA256,
			"size_bytes": l.Level.SizeBytes, "platforms": l.Level.Platforms}
		if l.Level.RequiresClient != "" {
			b["requires_client"] = l.Level.RequiresClient
		}
		e.bundles = append(e.bundles, b)
	}
	return e, nil
}

func (h GameManifest) rebuildOne(ctx context.Context, tenant uuid.UUID, g gameEntries, cause uuid.UUID, itemType string, releasedAt time.Time) error {
	version, raw, err := h.Store.Latest(ctx, tenant)
	if err != nil {
		return err
	}
	doc := map[string]json.RawMessage{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &doc)
	}
	var items []json.RawMessage
	_ = json.Unmarshal(doc["items"], &items)
	kept := make([]json.RawMessage, 0, len(items)+len(g.items))
	for _, it := range items {
		var head struct {
			ItemType string `json:"item_type"`
		}
		if json.Unmarshal(it, &head) == nil && slices.Contains(gameItemTypes, head.ItemType) {
			continue
		}
		kept = append(kept, it)
	}
	kept = append(kept, g.items...)
	var errs [3]error
	doc["items"], errs[0] = json.Marshal(kept)
	doc["areas"], errs[1] = json.Marshal(nonNilMaps(g.areas))
	doc["bundles"], errs[2] = json.Marshal(nonNilMaps(g.bundles))
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	next := version + 1
	sum := sha256.Sum256(append(b, []byte(fmt.Sprintf("%s:%d", tenant, next))...))
	etag := hex.EncodeToString(sum[:])
	at := time.Now().UTC()
	if h.Now != nil {
		at = h.Now().UTC()
	}
	if err := h.Store.Insert(ctx, write.ManifestRow{TenantID: tenant, Version: next, ETag: etag, Entries: b, GeneratedAt: at, CauseReleaseID: &cause}); err != nil {
		return err
	}
	if h.Metrics != nil && !releasedAt.IsZero() {
		h.Metrics.PublishToVisible(itemType, at.Sub(releasedAt))
	}
	t := tenant
	return h.Events.Publish(ctx, write.Event{Type: EventManifestUpdated, SubjectType: "content_manifest", SubjectID: tenant.String(), TenantID: &t,
		Payload: map[string]any{"tenant_id": tenant, "manifest_version": next, "etag": etag, "generated_at": at, "cause_release_id": cause}})
}

func nonNilMaps(m []map[string]any) []map[string]any {
	if m == nil {
		return []map[string]any{}
	}
	return m
}
