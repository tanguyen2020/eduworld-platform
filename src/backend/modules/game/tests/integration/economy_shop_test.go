//go:build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	gamehttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func shopIDs(r resp) []string {
	var out []string
	for _, d := range r.body["data"].([]any) {
		out = append(out, d.(map[string]any)["shop_item_id"].(string))
	}
	return out
}

// listing publishes a test shop item for item (price in Xu) modified by edit.
func (e *env) listing(ctx context.Context, t *testing.T, item, kind string, price int, edit func(m map[string]any)) string {
	t.Helper()
	id := uuid.NewString()
	m := map[string]any{"schema_version": 1, "shop_item_id": id, "owner_scope": "platform", "item_id": item, "item_kind": kind,
		"price": map[string]any{"currency_code": "xu", "amount": price}, "purchase_limit": 1}
	if edit != nil {
		edit(m)
	}
	if _, err := e.publish(ctx, t, []command.ContentItem{{Type: "shop_item", Payload: mustJSON(t, m)}}, "test listing"); err != nil {
		t.Fatal(err)
	}
	return id
}

// F075-AC-06, AC-12: listShopItems shows only published listings on sale now, assigned to the tenant (or every tenant), of
// the grade of the student, with the server price in Xu, the limit, the quantity bought and whether the item is owned;
// a listing of another tenant, another grade, outside its window or of an event that is not running is not shown and
// cannot be bought (409 SHOP_ITEM_UNAVAILABLE, no existence leak).
func TestShopListing(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active") // grade 3
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	past := e.clock.Now().Add(-time.Hour).Format(time.RFC3339)
	gradeSix := e.listing(ctx, t, "emote_cheer_01", "emote", 15, func(m map[string]any) { m["grade_levels"] = []any{6, 7} })
	ended := e.listing(ctx, t, "frame_gold_01", "frame", 15, func(m map[string]any) {
		m["available_from"], m["available_to"] = e.clock.Now().Add(-48*time.Hour).Format(time.RFC3339), past
	})
	event := e.listing(ctx, t, "pet_cat_01", "pet", 15, func(m map[string]any) { m["event_id"] = uuid.NewString() })
	r := e.do(t, http.MethodGet, "/game/shop/items", e.gtoken(an), th(an), nil)
	if got := shopIDs(r); r.status != http.StatusOK || !slices.Equal(got, []string{shopHat, shopFirework, shopWings}) {
		t.Fatalf("shop of A: %d %v %s", r.status, got, r.raw)
	}
	hat := r.body["data"].([]any)[0].(map[string]any)
	if hat["price"] != float64(120) || hat["currency_code"] != "xu" || hat["purchase_limit"] != float64(1) || hat["purchased_count"] != float64(0) ||
		hat["owned"] != false || hat["item_id"] != "hat_star_01" || hat["kind"] != "accessory" || hat["rarity"] != "rare" ||
		hat["name"].(map[string]any)["vi"] != "Mũ ngôi sao" {
		t.Fatalf("hat: %+v", hat)
	}
	if got := shopIDs(e.do(t, http.MethodGet, "/game/shop/items", e.gtoken(dung), th(dung), nil)); !slices.Equal(got, []string{shopHat, shopFirework}) {
		t.Fatalf("shop of B shows the listing of A: %v", got)
	}
	r = e.do(t, http.MethodGet, "/game/shop/items?kind=consumable_cosmetic&limit=1", e.gtoken(an), th(an), nil)
	if got := shopIDs(r); !slices.Equal(got, []string{shopFirework}) || r.body["page"].(map[string]any)["has_more"] != false {
		t.Fatalf("kind filter: %v %s", got, r.raw)
	}
	r = e.do(t, http.MethodGet, "/game/shop/items?limit=2", e.gtoken(an), th(an), nil)
	next, _ := r.body["page"].(map[string]any)["next_cursor"].(string)
	if got := shopIDs(r); len(got) != 2 || next == "" {
		t.Fatalf("page 1: %v", got)
	}
	if got := shopIDs(e.do(t, http.MethodGet, "/game/shop/items?limit=2&cursor="+next, e.gtoken(an), th(an), nil)); !slices.Equal(got, []string{shopWings}) {
		t.Fatalf("page 2: %v", got)
	}
	e.credit(ctx, t, an, 300)
	e.credit(ctx, t, dung, 300)
	for name, c := range map[string]struct {
		s     student
		id    string
		price int
	}{"other grade": {an, gradeSix, 15}, "ended": {an, ended, 15}, "event not running": {an, event, 15}, "other tenant": {dung, shopWings, 300},
		"unknown": {an, uuid.NewString(), 15}} {
		if r := e.buy(t, c.s, c.id, newKey(), c.price, 0); r.status != http.StatusConflict || r.code() != string(problem.ShopItemUnavailable) {
			t.Fatalf("%s: %d %s", name, r.status, r.raw)
		}
	}
	if e.balanceOf(ctx, t, an) != 300 || e.balanceOf(ctx, t, dung) != 300 {
		t.Fatal("an unavailable purchase moved Xu")
	}
}

// F075-AC-07, AC-08, AC-10 (TC-ECO-001): a valid purchase posts −price (shop_purchase), the item posting, the inventory,
// the purchase row, shop.item_purchased, item.granted, currency.debited and the audit in one transaction and returns
// PurchaseResult; the same Idempotency-Key replays the response and debits once; no key → 428 IDEMPOTENCY_KEY_REQUIRED;
// the key with another body → 409 IDEMPOTENCY_KEY_REUSED (platform convention, spec Clarifications); owned → 409
// ITEM_ALREADY_OWNED; another price → 409 CONFLICT; over the limit → 409 PURCHASE_LIMIT_REACHED.
func TestShopPurchase(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.credit(ctx, t, an, 300)
	key := newKey()
	r := e.buy(t, an, shopHat, key, 120, 0)
	item, _ := r.body["item"].(map[string]any)
	if r.status != http.StatusOK || r.body["balance_after"] != float64(180) || r.body["shop_item_id"] != shopHat || item["item_id"] != "hat_star_01" ||
		item["quantity"] != float64(1) || item["kind"] != "accessory" {
		t.Fatalf("purchase: %d %s", r.status, r.raw)
	}
	again := e.buy(t, an, shopHat, key, 120, 0)
	if again.status != http.StatusOK || again.header.Get("Idempotent-Replayed") != "true" || again.body["balance_after"] != float64(180) {
		t.Fatalf("replay: %d %s", again.status, again.raw)
	}
	if e.balanceOf(ctx, t, an) != 180 || e.ledgerSum(ctx, t, an) != 180 || e.ownedOf(ctx, t, an, "hat_star_01") != 1 {
		t.Fatalf("debited twice: %d", e.balanceOf(ctx, t, an))
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM gamification.currency_ledger WHERE student_id = $1 AND reason_code = 'shop_purchase'
		AND delta = -120`, an.id); n != 1 {
		t.Fatalf("shop postings %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM gamification.item_ledger WHERE student_id = $1 AND reason_code = 'shop_purchase'`, an.id) +
		count(ctx, t, e.platform, `SELECT count(*) FROM gamification.shop_purchases WHERE student_id = $1`, an.id); n != 2 {
		t.Fatalf("item posting + purchase rows %d", n)
	}
	if e.audits(ctx, t, "shop.item_purchased") != 1 || e.events(ctx, t, "shop.item_purchased") != 1 || e.events(ctx, t, "currency.debited") != 1 {
		t.Fatal("events or audit of the purchase")
	}
	if r := e.buy(t, an, shopHat, "", 120, 0); r.status != http.StatusPreconditionRequired || r.code() != string(problem.IdempotencyKeyRequired) {
		t.Fatalf("no key: %d %s", r.status, r.raw)
	}
	if r := e.buy(t, an, shopHat, key, 999, 0); r.status != http.StatusConflict || r.code() != string(problem.IdempotencyKeyReused) {
		t.Fatalf("key reused: %d %s", r.status, r.raw)
	}
	if r := e.buy(t, an, shopHat, newKey(), 120, 0); r.status != http.StatusConflict || r.code() != string(problem.ItemAlreadyOwned) {
		t.Fatalf("owned: %d %s", r.status, r.raw)
	}
	if r := e.buy(t, an, shopFirework, newKey(), 9, 0); r.status != http.StatusConflict || r.code() != string(problem.Conflict) {
		t.Fatalf("price changed: %d %s", r.status, r.raw)
	}
	if r := e.buy(t, an, shopFirework, newKey(), 10, 3); r.status != http.StatusOK || r.body["balance_after"] != float64(150) ||
		r.body["item"].(map[string]any)["quantity"] != float64(3) {
		t.Fatalf("stackable: %d %s", r.status, r.raw)
	}
	if r := e.buy(t, an, shopFirework, newKey(), 10, 18); r.status != http.StatusConflict || r.code() != string(problem.PurchaseLimitReached) {
		t.Fatalf("limit: %d %s", r.status, r.raw)
	}
	r = e.do(t, http.MethodGet, "/game/shop/items?kind=consumable_cosmetic", e.gtoken(an), th(an), nil)
	if fw := r.body["data"].([]any)[0].(map[string]any); fw["purchased_count"] != float64(3) || fw["owned"] != true {
		t.Fatalf("listing after purchase: %+v", fw)
	}
	if seen := e.checkEventSchemas(ctx, t); seen["shop.item_purchased"] != 2 || seen["item.granted"] < 2 || seen["currency.debited"] != 2 {
		t.Fatalf("schemas %v", seen)
	}
}

// F075-AC-09 (TC-ECO-001, SC-001 in small): not enough Xu → 409 INSUFFICIENT_BALANCE, balance and ledger unchanged; two
// concurrent purchases of which only one is affordable give exactly one success — 50 pairs, never a negative balance or a
// double debit.
func TestShopPurchaseRace(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	other := e.listing(ctx, t, "frame_gold_01", "frame", 100, nil)
	lone := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.credit(ctx, t, lone, 50)
	if r := e.buy(t, lone, shopHat, newKey(), 120, 0); r.status != http.StatusConflict || r.code() != string(problem.InsufficientBalance) {
		t.Fatalf("insufficient: %d %s", r.status, r.raw)
	}
	if e.balanceOf(ctx, t, lone) != 50 || count(ctx, t, e.platform, `SELECT count(*) FROM gamification.currency_ledger WHERE student_id = $1`, lone.id) != 1 {
		t.Fatal("a refused purchase posted")
	}
	const pairs = 50
	students := make([]student, pairs)
	for i := range students {
		students[i] = e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
		e.credit(ctx, t, students[i], 150)
	}
	var wg sync.WaitGroup
	results := make(chan int, 2*pairs)
	for _, s := range students {
		for _, l := range []struct {
			id    string
			price int
		}{{shopHat, 120}, {other, 100}} {
			wg.Add(1)
			go func(s student, id string, price int) {
				defer wg.Done()
				results <- e.buy(t, s, id, newKey(), price, 0).status
			}(s, l.id, l.price)
		}
	}
	wg.Wait()
	close(results)
	ok, refused := 0, 0
	for st := range results {
		switch st {
		case http.StatusOK:
			ok++
		case http.StatusConflict:
			refused++
		default:
			t.Fatalf("status %d", st)
		}
	}
	if ok != pairs || refused != pairs {
		t.Fatalf("%d ok, %d refused (want %d each)", ok, refused, pairs)
	}
	for _, s := range students {
		b := e.balanceOf(ctx, t, s)
		if (b != 30 && b != 50) || b != e.ledgerSum(ctx, t, s) {
			t.Fatalf("balance %d ledger %d", b, e.ledgerSum(ctx, t, s))
		}
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM gamification.wallets WHERE balance < 0`); n != 0 {
		t.Fatalf("negative balances %d", n)
	}
}

// F075-AC-12 (TC-AUTH-014): every economy player route is a guarded player route — staff tokens of the 5 roles → 403
// GAME_ACCESS_STUDENT_ONLY (TestPlayerRoutesStaffDenied iterates them too), X-Tenant-Id of another tenant → 403
// TENANT_MISMATCH; no route reads another student.
func TestEconomyPlayerAuthz(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	for _, id := range gamehttp.EconomyOperations {
		if !slices.Contains(gamehttp.PlayerOperations, id) || gamehttp.Operations[id].Guard == 0 {
			t.Fatalf("%s is not a guarded player operation", id)
		}
	}
	authztest.Cover(t, "getWallet", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "listWalletLedger", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "listShopItems", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "purchaseShopItem", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "listInventory", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "listInventoryHistory", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "listCollections", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "getCollection", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "listAchievements", authztest.RoleDenied, authztest.CrossTenant)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	staff := map[string]string{
		"school_admin":     e.staff(e.tenantA, "school_admin"),
		"academic_manager": e.staff(e.tenantA, "academic_manager"),
		"tenant_admin":     e.staff(e.tenantA, "tenant_admin"),
		"system_admin":     e.platformStaff("system_admin"),
	}
	for _, id := range gamehttp.EconomyOperations {
		o := gamehttp.Operations[id]
		path := pathOf(o.Pattern)
		var body any
		if id == "purchaseShopItem" {
			body = map[string]any{"expected_price": 120}
		}
		for role, sub := range staff {
			r := e.do(t, o.Method, path, e.idp.Token(sub, "eduworld-portal", nowWall()), map[string]string{"X-Tenant-Id": e.tenantA.String(),
				"Idempotency-Key": newKey()}, body)
			if r.status != http.StatusForbidden || r.code() != string(problem.GameAccessStudentOnly) {
				t.Fatalf("%s by %s: %d %s", id, role, r.status, r.raw)
			}
		}
		r := e.do(t, o.Method, path, e.gtoken(an), map[string]string{"X-Tenant-Id": e.tenantB.String(), "Idempotency-Key": newKey()}, body)
		if r.status != http.StatusForbidden || r.code() != string(problem.TenantMismatch) {
			t.Fatalf("%s with the tenant of B: %d %s", id, r.status, r.raw)
		}
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM gamification.wallets`) + count(ctx, t, e.platform,
		`SELECT count(*) FROM gamification.shop_purchases`); n != 0 {
		t.Fatalf("rows written: %d", n)
	}
}

func pathOf(pattern string) string {
	return strings.NewReplacer("{shop_item_id}", shopHat, "{collection_id}", cardsColl).Replace(pattern)
}
