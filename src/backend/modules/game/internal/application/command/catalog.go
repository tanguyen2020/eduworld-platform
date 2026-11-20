package command

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/profile"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// CatalogPublish publishes the character catalog and the item definitions (platform content, ADR-0043) under a
// PlatformScope: the dev/pilot seed (cmd/seed, F070 T008) and later the Game Operations Console (F078). A version
// identical to the current one is not published again, so the seed can run any number of times.
type CatalogPublish struct {
	UoW   write.UnitOfWork
	Store write.CatalogPublisher
	Audit write.Auditor
	Now   func() time.Time
}

// PublishResult counts what changed.
type PublishResult struct {
	CatalogPublished bool
	ItemsPublished   int
}

// Publish checks payload (character-catalog.schema.json rules the server relies on) and the items (every default
// item and emote of the catalog needs a definition), then publishes them in one transaction.
func (h CatalogPublish) Publish(ctx context.Context, payload []byte, items []catalog.ItemDefinition, note string) (res PublishResult, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.PublishCharacterCatalog")
	defer func() { done(err) }()
	if s, ok := tenancy.ScopeFrom(ctx); !ok || !s.PlatformOnly() {
		return res, problem.New(problem.PermissionDenied, "platform content is published under a platform scope")
	}
	p, err := catalog.Parse(payload)
	if err != nil {
		return res, problem.Invalid("payload", problem.SubInvalid, err.Error())
	}
	known := map[string]catalog.ItemDefinition{}
	for _, it := range items {
		if !profile.ValidCode(it.ItemID) || it.Name.VI == "" || it.Name.EN == "" || len(it.Sources) > 20 {
			return res, problem.Invalid("items", problem.SubInvalid, "item "+it.ItemID+" needs a code, names vi and en, ≤ 20 sources")
		}
		known[it.ItemID] = it
	}
	for _, id := range append(append([]string{}, p.DefaultItemIDs...), p.DefaultEmoteIDs...) {
		if _, ok := known[id]; !ok {
			return res, problem.Invalid("items", problem.SubInvalid, fmt.Sprintf("default item %s has no definition", id))
		}
	}
	for _, id := range p.DefaultEmoteIDs {
		if known[id].EquipSlot != profile.SlotEmote {
			return res, problem.Invalid("items", problem.SubInvalid, fmt.Sprintf("default emote %s is not an emote", id))
		}
	}
	canonical, err := json.Marshal(p)
	if err != nil {
		return res, err
	}
	sha, err := catalog.Sha256(p)
	if err != nil {
		return res, err
	}
	now := time.Now().UTC()
	if h.Now != nil {
		now = h.Now().UTC()
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		for _, it := range items {
			changed, err := h.Store.PublishItem(ctx, it, now)
			if err != nil {
				return err
			}
			if changed {
				res.ItemsPublished++
			}
		}
		changed, err := h.Store.PublishCatalog(ctx, catalog.Code, canonical, sha, note, now)
		if err != nil {
			return err
		}
		res.CatalogPublished = changed
		if !changed && res.ItemsPublished == 0 {
			return nil
		}
		return h.Audit.Record(ctx, audit.Event{Action: "character_catalog.published", TargetType: "character_catalog", TargetID: catalog.Code,
			Result: audit.ResultSuccess, Metadata: map[string]any{"payload_sha256": sha, "catalog_published": changed,
				"items_published": res.ItemsPublished}})
	})
	return res, err
}
