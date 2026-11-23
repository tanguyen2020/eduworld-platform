package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/achievement"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/collection"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/economy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/progression"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// EconomyPublishRules are the checks and the side table of the F075 content types on the release core (T023, T030, T035,
// F075-AC-11, AC-23): the JSON Schemas of item, shop-item, collection and achievement, the domain rules (no real money, no
// tradable item, cosmetic kinds only, never play time / logins / messages / official scores), the grant appliers of the
// rewards, the published items the shop items and collections reference, and the items of a published collection
// version (gamification.collection_items).
type EconomyPublishRules struct {
	ItemSchema        port.PayloadValidator
	ShopSchema        port.PayloadValidator
	CollectionSchema  port.PayloadValidator
	AchievementSchema port.PayloadValidator
	Appliers          write.GrantApplierRegistry
	Items             port.EconomyContent
	CollectionItems   write.CollectionItemStore
}

// economyVersionField is the server id field of each F075 item type (set by the release core, never authored).
var economyVersionField = map[string]string{write.ItemGameItem: "item_version_id", write.ItemShopItem: "shop_item_version_id",
	write.ItemCollection: "collection_version_id", write.ItemAchievement: "achievement_version_id"}

func economyItem(t string) bool { _, ok := economyVersionField[t]; return ok }

func economyFieldProblem(err error) error {
	var ef *economy.FieldError
	if errors.As(err, &ef) {
		return problem.Invalid(ef.Field, problem.SubInvalid, ef.Reason)
	}
	var cf *collection.FieldError
	if errors.As(err, &cf) {
		return problem.Invalid(cf.Field, problem.SubInvalid, cf.Reason)
	}
	var af *achievement.FieldError
	if errors.As(err, &af) {
		return problem.Invalid(af.Field, problem.SubInvalid, af.Reason)
	}
	return problem.Invalid("payload", problem.SubInvalid, err.Error())
}

// parseEconomy reads an authored item, shop item, collection or achievement: JSON Schema of the stored form (placeholders
// for the server fields), then the domain checks; 422 VALIDATION_FAILED names the first failing field.
func parseEconomy(it ContentItem, rules *EconomyPublishRules) (parsedItem, error) {
	p := parsedItem{ContentItem: it, parent: map[string]any{}, columns: map[string]any{}}
	if rules == nil || rules.ItemSchema == nil || rules.ShopSchema == nil || rules.CollectionSchema == nil || rules.AchievementSchema == nil ||
		rules.Appliers == nil {
		return p, problem.New(problem.Internal, "economy publishing is not configured")
	}
	if err := json.Unmarshal(it.Payload, &p.doc); err != nil || p.doc == nil {
		return p, problem.Invalid("payload", problem.SubRequired, "a JSON object is required")
	}
	vf := economyVersionField[it.Type]
	for _, f := range []string{vf, "version", "status"} {
		delete(p.doc, f)
	}
	stored := map[string]any{}
	for k, v := range p.doc {
		stored[k] = v
	}
	stored[vf], stored["version"], stored["status"] = placeholderVersionID, 1, "published"
	raw, err := json.Marshal(stored)
	if err != nil {
		return p, err
	}
	switch it.Type {
	case write.ItemGameItem:
		if err := rules.ItemSchema.Validate(raw); err != nil {
			return p, schemaProblem(err)
		}
		item, err := economy.ParseItem(raw)
		if err == nil {
			err = item.Validate()
		}
		if err != nil {
			return p, economyFieldProblem(err)
		}
		p.item, p.id = &item, item.ItemID
		assets := item.AssetRefs
		if assets == nil {
			assets = map[string]any{}
		}
		p.columns = map[string]any{"kind": item.Kind, "equip_slot": item.EquipSlot, "rarity": item.Rarity, "name": item.Name,
			"sources": nonNil(item.Sources), "asset_refs": assets, "stackable": item.Stackable, "max_stack": item.MaxStack}
	case write.ItemShopItem:
		if err := rules.ShopSchema.Validate(raw); err != nil {
			return p, schemaProblem(err)
		}
		s, err := economy.ParseShopItem(raw)
		if err != nil {
			return p, economyFieldProblem(err)
		}
		p.shop, p.id = &s, s.ShopItemID.String()
		grades := s.Grades
		if grades == nil {
			grades = []int{}
		}
		p.columns = map[string]any{"item_id": s.ItemID, "item_kind": s.ItemKind, "price_amount": s.Price, "currency_code": s.Currency,
			"purchase_limit": s.PurchaseLimit, "available_from": s.From, "available_to": s.To, "grade_levels": grades,
			"featured_order": s.FeaturedOrder, "event_id": s.EventID, "season_id": s.SeasonID}
	case write.ItemCollection:
		if err := rules.CollectionSchema.Validate(raw); err != nil {
			return p, schemaProblem(err)
		}
		c, err := collection.Parse(raw)
		if err == nil {
			err = c.Validate(rules.Appliers.Has)
		}
		if err != nil {
			return p, economyFieldProblem(err)
		}
		p.coll, p.id = &c, c.CollectionID.String()
		p.parent = map[string]any{"code": c.Code}
		p.columns = map[string]any{"code": c.Code, "collection_type": c.Type, "visibility": c.Visibility, "item_count": len(c.Items)}
	case write.ItemAchievement:
		if err := rules.AchievementSchema.Validate(raw); err != nil {
			return p, schemaProblem(err)
		}
		d, err := achievement.Parse(raw)
		if err == nil {
			err = d.Validate(rules.Appliers.Has)
		}
		if err != nil {
			return p, economyFieldProblem(err)
		}
		p.ach, p.id = &d, d.AchievementID.String()
		p.parent = map[string]any{"code": d.Code}
		p.columns = map[string]any{"code": d.Code, "category": d.Category, "visibility": d.Visibility, "trigger_event_types": d.Trigger.EventTypes}
	}
	return p, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// checkEconomy checks the items a batch references against the batch and the published items: a shop item sells a
// published item of its kind under the shop rules; a collection lists published items; rewards name published items.
func (h PublishGameContent) checkEconomy(ctx context.Context, items []parsedItem) error {
	var codes []string
	batch := map[string]economy.Item{}
	for _, p := range items {
		switch {
		case p.item != nil:
			batch[p.item.ItemID] = *p.item
		case p.shop != nil:
			codes = append(codes, p.shop.ItemID)
		case p.coll != nil:
			codes = append(codes, p.coll.ItemIDs()...)
			codes = append(codes, rewardItems(p.coll.Rewards)...)
		case p.ach != nil:
			for _, t := range p.ach.Tiers {
				codes = append(codes, rewardItems(t.Rewards)...)
			}
		case p.rule != nil:
			codes = append(codes, rewardItems(grantMaps(p.rule.Grants))...)
		case p.curve != nil:
			for _, lr := range p.curve.LevelRewards {
				codes = append(codes, rewardItems(levelRewards(lr.Rewards))...)
			}
		}
	}
	if len(codes) == 0 {
		return nil
	}
	if h.Economy == nil || h.Economy.Items == nil {
		return problem.New(problem.Internal, "economy publishing is not configured")
	}
	published, err := h.Economy.Items.Items(ctx, codes)
	if err != nil {
		return err
	}
	lookup := func(code string) *economy.Item {
		if it, ok := batch[code]; ok {
			return &it
		}
		if d, ok := published[code]; ok {
			return &economy.Item{ItemID: d.ItemID, Kind: d.Kind, Rarity: d.Rarity, EquipSlot: d.EquipSlot, Stackable: d.Stackable, MaxStack: d.MaxStack}
		}
		return nil
	}
	for _, p := range items {
		switch {
		case p.shop != nil:
			if err := p.shop.Validate(lookup(p.shop.ItemID)); err != nil {
				return economyFieldProblem(err)
			}
		case p.coll != nil:
			for i, code := range p.coll.ItemIDs() {
				if lookup(code) == nil {
					return problem.New(problem.ItemRefInvalid, fmt.Sprintf("collection %s: items/%d is the unknown item %s", p.coll.Code, i, code))
				}
			}
			if err := checkRewardItems(p.coll.Rewards, lookup); err != nil {
				return err
			}
		case p.ach != nil:
			for _, t := range p.ach.Tiers {
				if err := checkRewardItems(t.Rewards, lookup); err != nil {
					return err
				}
			}
		case p.rule != nil:
			if err := checkRewardItems(grantMaps(p.rule.Grants), lookup); err != nil {
				return err
			}
		case p.curve != nil:
			for _, lr := range p.curve.LevelRewards {
				if err := checkRewardItems(levelRewards(lr.Rewards), lookup); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func levelRewards(rs []progression.Reward) []map[string]any {
	out := make([]map[string]any, 0, len(rs))
	for _, r := range rs {
		out = append(out, map[string]any(r))
	}
	return out
}

var rewardItemFields = map[string]string{"item": "item_id", "badge": "badge_id", "collection_item": "collection_item_id"}

func rewardItems(rs []map[string]any) []string {
	var out []string
	for _, r := range rs {
		t, _ := r["type"].(string)
		if f, ok := rewardItemFields[t]; ok {
			if c, _ := r[f].(string); c != "" {
				out = append(out, c)
			}
		}
	}
	return out
}

func checkRewardItems(rs []map[string]any, lookup func(string) *economy.Item) error {
	for _, r := range rs {
		t, _ := r["type"].(string)
		f, ok := rewardItemFields[t]
		if !ok {
			continue
		}
		code, _ := r[f].(string)
		it := lookup(code)
		if it == nil {
			return problem.New(problem.ItemRefInvalid, fmt.Sprintf("reward names the unknown item %s", code))
		}
		if t == "badge" && it.Kind != "badge" {
			return problem.Invalid("badge_id", problem.SubInvalid, fmt.Sprintf("item %s is not a badge", code))
		}
	}
	return nil
}

// economyStored adds the server fields to an authored F075 payload.
func economyStored(p parsedItem, vid uuid.UUID, version int) ([]byte, error) {
	doc := map[string]any{}
	for k, v := range p.doc {
		doc[k] = v
	}
	doc[economyVersionField[p.Type]], doc["version"], doc["status"] = vid.String(), version, "published"
	return json.Marshal(doc)
}

// insertCollectionItems writes the items of a published collection version (after its version row, same transaction).
func (h PublishGameContent) insertCollectionItems(ctx context.Context, p parsedItem, vid uuid.UUID) error {
	if h.Economy == nil || h.Economy.CollectionItems == nil {
		return problem.New(problem.Internal, "collection item store is not configured")
	}
	rows := make([]write.CollectionItemRow, 0, len(p.coll.Items))
	for i, it := range p.coll.Items {
		rows = append(rows, write.CollectionItemRow{ItemID: it.ItemID, Position: i, Hint: map[string]string{"vi": it.SourceHint.VI, "en": it.SourceHint.EN},
			ObjectiveID: it.ObjectiveID})
	}
	return h.Economy.CollectionItems.InsertCollectionItems(ctx, p.coll.CollectionID, vid, rows)
}
