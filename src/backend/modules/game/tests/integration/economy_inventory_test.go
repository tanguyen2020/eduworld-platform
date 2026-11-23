//go:build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func itemCodes(r resp) []string {
	var out []string
	for _, d := range r.body["data"].([]any) {
		out = append(out, d.(map[string]any)["item_id"].(string))
	}
	return out
}

// F075-AC-13, AC-14 (TC-ECO-004): item and badge grants post to the item ledger and the inventory in the grant transaction
// (a stackable item adds up) with item.granted; a non-stackable item already owned converts to duplicate_item_xu Xu of
// its rarity (reason duplicate_conversion), shown in the reward; two concurrent grants of one item give one inventory row
// and one conversion.
func TestItemGrantsAndDuplicates(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.rewardRule(ctx, t, ruleQuestSide, map[string]any{"type": "xp", "amount": 80}, map[string]any{"type": "item", "item_id": "pet_cat_01"},
		map[string]any{"type": "badge", "badge_id": "badge_explorer_01"}, map[string]any{"type": "item", "item_id": "firework_01"})
	e.clock.Add(1e9)
	e.ecoDeliver(ctx, t, e.questEvent(an, "side"))
	if e.ownedOf(ctx, t, an, "pet_cat_01") != 1 || e.ownedOf(ctx, t, an, "badge_explorer_01") != 1 || e.ownedOf(ctx, t, an, "firework_01") != 1 {
		t.Fatal("first grant: inventory")
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM gamification.item_ledger WHERE student_id = $1 AND reason_code = 'reward'`, an.id); n != 3 {
		t.Fatalf("item postings %d", n)
	}
	if n := e.events(ctx, t, "item.granted"); n != 3 {
		t.Fatalf("item.granted %d", n)
	}
	e.clock.Add(1e9)
	e.ecoDeliver(ctx, t, e.questEvent(an, "side"))
	if e.ownedOf(ctx, t, an, "pet_cat_01") != 1 || e.ownedOf(ctx, t, an, "badge_explorer_01") != 1 || e.ownedOf(ctx, t, an, "firework_01") != 2 {
		t.Fatal("second grant: one row per non-stackable item, stackable adds up")
	}
	if got := e.balanceOf(ctx, t, an); got != 35 { // uncommon 10 + rare 25
		t.Fatalf("duplicate conversion: %d", got)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM gamification.currency_ledger WHERE student_id = $1 AND reason_code = 'duplicate_conversion'
		AND item_id IN ('pet_cat_01', 'badge_explorer_01')`, an.id); n != 2 {
		t.Fatalf("conversion postings %d", n)
	}
	grants := e.payloads(ctx, t, "reward.granted")
	last := grants[len(grants)-1]["items"].([]any)
	notes := 0
	for _, it := range last {
		if m := it.(map[string]any); m["duplicate_xu"] != nil {
			notes++
		}
	}
	if notes != 2 {
		t.Fatalf("reward does not show the conversion: %+v", last)
	}
	// Two concurrent grants of the same non-stackable item to a new student.
	binh := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.rewardRule(ctx, t, ruleQuestSide, map[string]any{"type": "xp", "amount": 80}, map[string]any{"type": "item", "item_id": "pet_cat_01"})
	e.clock.Add(1e9)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- e.deliver(ctx, t, e.eco().prog, e.questEvent(binh, "side"))
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.inventory_items WHERE student_id = $1 AND item_id = 'pet_cat_01'`, binh.id); n != 1 ||
		e.balanceOf(ctx, t, binh) != 10 {
		t.Fatalf("concurrent grants: %d rows, %d Xu", n, e.balanceOf(ctx, t, binh))
	}
	if seen := e.checkEventSchemas(ctx, t); seen["item.granted"] < 4 || seen["currency.credited"] < 3 {
		t.Fatalf("schemas %v", seen)
	}
}

// F075-AC-15, AC-16, AC-17 (TC-ECO-007): listInventory (filter kind, keyset by code) and listInventoryHistory (newest
// first) answer the student itself; updateAvatar refuses an item not owned (422 ITEM_NOT_OWNED) and an owned item in
// another slot (422 EQUIP_SLOT_INVALID) and accepts an owned item; no route adds an item or trades items.
func TestInventoryAndEquip(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	other := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.rewardRule(ctx, t, ruleQuestSide, map[string]any{"type": "xp", "amount": 80}, map[string]any{"type": "item", "item_id": "pet_cat_01"},
		map[string]any{"type": "item", "item_id": "hat_star_01"}, map[string]any{"type": "badge", "badge_id": "badge_explorer_01"})
	e.clock.Add(1e9)
	e.ecoDeliver(ctx, t, e.questEvent(an, "side"))
	e.clock.Add(1e9)
	e.ecoDeliver(ctx, t, e.questEvent(other, "side"))
	r := e.do(t, http.MethodGet, "/game/inventory", e.gtoken(an), th(an), nil)
	if got := itemCodes(r); r.status != http.StatusOK || !slices.Equal(got, []string{"badge_explorer_01", "hat_star_01", "pet_cat_01"}) {
		t.Fatalf("inventory: %d %v %s", r.status, got, r.raw)
	}
	pet := r.body["data"].([]any)[2].(map[string]any)
	if pet["kind"] != "pet" || pet["equip_slot"] != "pet" || pet["rarity"] != "uncommon" || pet["quantity"] != float64(1) {
		t.Fatalf("pet: %+v", pet)
	}
	if got := itemCodes(e.do(t, http.MethodGet, "/game/inventory?kind=pet", e.gtoken(an), th(an), nil)); !slices.Equal(got, []string{"pet_cat_01"}) {
		t.Fatalf("kind filter: %v", got)
	}
	r = e.do(t, http.MethodGet, "/game/inventory?limit=2", e.gtoken(an), th(an), nil)
	next := r.body["page"].(map[string]any)["next_cursor"].(string)
	if got := itemCodes(e.do(t, http.MethodGet, "/game/inventory?limit=2&cursor="+next, e.gtoken(an), th(an), nil)); !slices.Equal(got, []string{"pet_cat_01"}) {
		t.Fatalf("page 2: %v", got)
	}
	r = e.do(t, http.MethodGet, "/game/inventory/history", e.gtoken(an), map[string]string{"X-Tenant-Id": an.tenant.String(), "Accept-Language": "en-US"}, nil)
	data := r.body["data"].([]any)
	if r.status != http.StatusOK || len(data) != 3 || data[0].(map[string]any)["source_label"] != "Reward" || data[0].(map[string]any)["item_name"] == "" {
		t.Fatalf("history: %d %s", r.status, r.raw)
	}
	for _, banned := range []string{"source_id", "entry_id", "idempotency_key", other.id.String()} {
		if strings.Contains(r.raw, banned) {
			t.Fatalf("history exposes %s", banned)
		}
	}
	if first := e.putAvatar(t, an, avatar()); first.status != http.StatusOK {
		t.Fatalf("setup avatar: %d %s", first.status, first.raw)
	}
	for name, c := range map[string]struct {
		cfg  map[string]any
		code problem.Code
	}{
		"not owned":     {with(avatar(), "accessories", []any{map[string]any{"slot": "back", "item_id": "wings_paper_01"}}), problem.ItemNotOwned},
		"another slot":  {with(avatar(), "accessories", []any{map[string]any{"slot": "back", "item_id": "hat_star_01"}}), problem.EquipSlotInvalid},
		"pet not owned": {with(avatar(), "pet", "pet_cat_02"), problem.ItemNotOwned},
	} {
		if r := e.putAvatar(t, an, c.cfg); r.status != http.StatusUnprocessableEntity || r.code() != string(c.code) {
			t.Fatalf("%s: %d %s", name, r.status, r.raw)
		}
	}
	if r := e.putAvatar(t, an, with(with(avatar(), "accessories", []any{map[string]any{"slot": "head", "item_id": "hat_star_01"}}), "pet",
		"pet_cat_01")); r.status != http.StatusOK {
		t.Fatalf("owned items: %d %s", r.status, r.raw)
	}
	if r := e.putAvatar(t, other, with(avatar(), "pet", "pet_cat_01")); r.status != http.StatusOK {
		t.Fatalf("other owns its pet: %d %s", r.status, r.raw)
	}
}
