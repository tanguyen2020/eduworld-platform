package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/economy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/economyschema"
	gamehttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/http"
)

// moneyWords are words of a real-money purchase, a paid random reward or a trade (AC-ECO-004, INVENTORY_ECONOMY §6.4).
var moneyWords = regexp.MustCompile(`(?i)(payment|checkout|iap|in_app|receipt|vnd|usd|real_money|loot|gacha|lucky_box|mystery|random_reward|` +
	`trade|trading|gift_item|transfer_item)`)

// F075-AC-11, AC-16, AC-17 (TC-ECO-003, SC-003): the merged spec has no operation of module game that buys with real money,
// opens a random box, trades items or adds an item to an inventory from a client; the shop prices are Xu only; every item
// and shop item of the seed passes its JSON Schema (tradable false, Xu price > 0, cosmetic kinds).
func TestNoRealMoneyNoTradeNoClientItems(t *testing.T) {
	doc := spec(t)
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			if svc, _ := op.Extensions["x-service"].(string); svc != "game" {
				continue
			}
			if moneyWords.MatchString(op.OperationID) || moneyWords.MatchString(path) {
				t.Errorf("%s %s (%s) looks like a real-money, random or trade operation", method, path, op.OperationID)
			}
			if strings.HasPrefix(path, "/game/inventory") && method != "GET" {
				t.Errorf("%s %s lets a client change an inventory", method, path)
			}
		}
	}
	shop := doc.Components.Schemas["ShopItem"].Value
	if shop == nil || shop.Properties["price"] == nil || shop.Properties["currency_code"] == nil {
		t.Fatal("ShopItem has no Xu price")
	}
	for name := range shop.Properties {
		if moneyWords.MatchString(name) {
			t.Errorf("ShopItem.%s", name)
		}
	}
	for _, id := range gamehttp.EconomyOperations {
		if moneyWords.MatchString(id) {
			t.Errorf("operation %s", id)
		}
	}
	if slices.ContainsFunc(gamehttp.PlayerOperations, func(id string) bool { return strings.Contains(strings.ToLower(id), "trade") }) {
		t.Error("a trade operation exists")
	}
	root := filepath.Join(repoRoot(t), "src", "backend", "modules", "game", "seed", "economy")
	for file, v := range map[string]interface{ Validate([]byte) error }{"items.v1.json": economyschema.Item{}, "shop-items.v1.json": economyschema.ShopItem{},
		"collections.v1.json": economyschema.Collection{}, "achievements.v1.json": economyschema.Achievement{}} {
		raw, err := os.ReadFile(filepath.Join(root, file)) //nolint:gosec // repository seed path
		if err != nil {
			t.Fatal(err)
		}
		var docs []map[string]any
		if err := json.Unmarshal(raw, &docs); err != nil {
			t.Fatal(err)
		}
		for _, d := range docs {
			d["version"], d["status"] = 1, "published" // server fields of the release core
			d[map[string]string{"items.v1.json": "item_version_id", "shop-items.v1.json": "shop_item_version_id",
				"collections.v1.json": "collection_version_id", "achievements.v1.json": "achievement_version_id"}[file]] = "00000000-0000-0000-0000-000000000000"
			b, _ := json.Marshal(d)
			if err := v.Validate(b); err != nil {
				t.Errorf("%s: %v", file, err)
			}
			if d["tradable"] == true {
				t.Errorf("%s: tradable item", file)
			}
			if p, ok := d["price"].(map[string]any); ok && p["currency_code"] != economy.Xu {
				t.Errorf("%s: price in %v", file, p["currency_code"])
			}
		}
	}
}
