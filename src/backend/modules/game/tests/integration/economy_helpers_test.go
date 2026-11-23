//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/economy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/economyschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/economysql"
	readrepo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/worldsql"
	writerepo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	rewardinfra "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/reward"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/seedfiles"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// Seed ids of F075 (src/backend/modules/game/seed/economy).
const (
	shopHat        = "0193a5f4-6c1e-7b2a-9d1f-0000000f7501" // hat_star_01, 120 Xu, every tenant
	shopFirework   = "0193a5f4-6c1e-7b2a-9d1f-0000000f7502" // firework_01, 10 Xu, stackable, limit 20
	shopWings      = "0193a5f4-6c1e-7b2a-9d1f-0000000f7503" // wings_paper_01, 300 Xu, assigned to tenant A in the tests
	cardsColl      = "0193a5f4-6c1e-7b2a-9d1f-0000000f7520"
	cardObjective  = "0193a5f4-6c1e-7b2a-9d1f-0000000f7521" // card_times_table_7
	cardObjective2 = "0193a5f4-6c1e-7b2a-9d1f-0000000f7522"
	achExplorer    = "0193a5f4-6c1e-7b2a-9d1f-0000000f7530" // count quest.completed: bronze 3 (20 Xu), silver 10 (badge)
	achHidden      = "0193a5f4-6c1e-7b2a-9d1f-0000000f7531" // hidden: first shop purchase → badge_shopper_01
)

// economyRules are the publish checks of the F075 content (JSON Schemas, every applier, items, collection items).
func economyRules() *command.EconomyPublishRules {
	return &command.EconomyPublishRules{ItemSchema: economyschema.Item{}, ShopSchema: economyschema.ShopItem{},
		CollectionSchema: economyschema.Collection{}, AchievementSchema: economyschema.Achievement{}, Appliers: rewardinfra.Types(),
		Items: economysql.Content{}, CollectionItems: writerepo.GameContentRepo{}}
}

// defaultEconomyPolicy is the CONFIG_DEFAULTS policy of the economy (duplicate_item_xu, shop_purchase_limit_default).
func defaultEconomyPolicy() command.EconomyPolicy {
	return command.EconomyPolicy{Duplicate: economy.DuplicateTable{"common": 5, "uncommon": 10, "rare": 25, "epic": 50, "legendary": 100},
		PurchaseLimitDefault: 1}
}

type ecoBundle struct {
	eco  *command.Economy
	prog command.Progression
}

var ecoBundles sync.Map // *env → *ecoBundle

// eco returns the economy and the progression of the env wired as module.go does (one registry with every applier).
func (e *env) eco() *ecoBundle {
	if v, ok := ecoBundles.Load(e); ok {
		return v.(*ecoBundle)
	}
	b := &ecoBundle{}
	b.eco = &command.Economy{UoW: e.uow, AfterCommit: entx.AfterCommit, Store: writerepo.EconomyRepo{}, Grants: writerepo.ProgressionRepo{},
		Achievements: writerepo.AchievementRepo{}, Adjustments: writerepo.AdjustmentRepo{}, Scan: writerepo.EconomyRepo{}, Content: economysql.Content{},
		Students: worldsql.Content{}, LiveOps: economysql.NoLiveOps{}, Events: writerepo.Publisher{W: e.writer}, Audit: writerepo.Auditor{R: e.rec},
		Metrics: observability.EconomyMetrics{}, Policy: defaultEconomyPolicy(), Now: e.clock.Now}
	reg := rewardinfra.WithEconomy(b.eco)
	b.eco.Registry = reg
	b.prog = e.progressionUC()
	b.prog.Appliers = reg
	b.eco.XP = b.prog
	v, _ := ecoBundles.LoadOrStore(e, b)
	return v.(*ecoBundle)
}

func (e *env) economyQueries() query.Economy {
	codec, _ := page.NewCodec([]byte(strings.Repeat("c", 32)))
	return query.Economy{Read: e.read, Data: readrepo.Economy{}, Cursors: codec, Now: e.clock.Now}
}

type economyQueryProxy struct{ e *env }

func (p economyQueryProxy) GetWallet(ctx context.Context) (query.WalletView, error) {
	return p.e.economyQueries().GetWallet(ctx)
}
func (p economyQueryProxy) ListWalletLedger(ctx context.Context, limit int, cursor, lang string) (query.Page[query.LedgerView], error) {
	return p.e.economyQueries().ListWalletLedger(ctx, limit, cursor, lang)
}
func (p economyQueryProxy) ListShopItems(ctx context.Context, kind string, limit int, cursor string) (query.Page[query.ShopItemView], error) {
	return p.e.economyQueries().ListShopItems(ctx, kind, limit, cursor)
}
func (p economyQueryProxy) ListInventory(ctx context.Context, kind string, limit int, cursor string) (query.Page[query.InventoryView], error) {
	return p.e.economyQueries().ListInventory(ctx, kind, limit, cursor)
}
func (p economyQueryProxy) ListInventoryHistory(ctx context.Context, limit int, cursor, lang string) (query.Page[query.ItemHistoryView], error) {
	return p.e.economyQueries().ListInventoryHistory(ctx, limit, cursor, lang)
}
func (p economyQueryProxy) ListCollections(ctx context.Context, limit int, cursor string) (query.Page[query.CollectionView], error) {
	return p.e.economyQueries().ListCollections(ctx, limit, cursor)
}
func (p economyQueryProxy) GetCollection(ctx context.Context, id uuid.UUID, lang string) (query.CollectionDetailView, error) {
	return p.e.economyQueries().GetCollection(ctx, id, lang)
}
func (p economyQueryProxy) ListAchievements(ctx context.Context, category string, limit int, cursor string) (query.Page[query.AchievementView], error) {
	return p.e.economyQueries().ListAchievements(ctx, category, limit, cursor)
}
func (p economyQueryProxy) ListAdjustments(ctx context.Context, in query.AdjustmentsInput) (query.Page[read.AdjustmentRecord], error) {
	return p.e.economyQueries().ListAdjustments(ctx, in)
}

type economyCommandProxy struct{ e *env }

func (p economyCommandProxy) Purchase(ctx context.Context, id uuid.UUID, qty, price int) (command.PurchaseResult, error) {
	return p.e.eco().eco.Purchase(ctx, id, qty, price)
}
func (p economyCommandProxy) CreateAdjustment(ctx context.Context, in command.AdjustmentInput) (command.AdjustmentView, error) {
	return p.e.eco().eco.CreateAdjustment(ctx, in)
}
func (p economyCommandProxy) ApproveAdjustment(ctx context.Context, id uuid.UUID) (command.AdjustmentView, error) {
	return p.e.eco().eco.ApproveAdjustment(ctx, id)
}
func (p economyCommandProxy) RejectAdjustment(ctx context.Context, id uuid.UUID, reason string) (command.AdjustmentView, error) {
	return p.e.eco().eco.RejectAdjustment(ctx, id, reason)
}

// economySeed returns the authored F075 seed (items, shop items, collection, achievements).
func economySeed(t *testing.T) []command.ContentItem {
	t.Helper()
	items, err := seedfiles.Economy(filepath.Join(testkit.RepoRoot(t), "src", "backend", "modules", "game", "seed"))
	if err != nil {
		t.Fatal(err)
	}
	return items
}

// setupEconomy is the environment with the F074 and F075 seeds published and the wings listing assigned to tenant A.
func setupEconomy(ctx context.Context, t *testing.T) *env {
	t.Helper()
	e := setupProgression(ctx, t)
	if _, err := e.publish(ctx, t, economySeed(t), "seed economy v1"); err != nil {
		t.Fatal(err)
	}
	sc, err := authz.SystemPlatformScope("game-seed")
	if err != nil {
		t.Fatal(err)
	}
	if _, skipped, err := (command.AssignSeedQuests{UoW: e.uow, Assignments: writerepo.QuestRepo{}, Audit: writerepo.Auditor{R: e.rec},
		ItemType: "shop_item"}).Assign(tenancy.WithScope(ctx, sc), []command.SeedAssignment{{QuestID: uuid.MustParse(shopWings),
		Tenants: []string{"t-a"}}}); err != nil || len(skipped) > 0 {
		t.Fatalf("assign wings: %v %v", skipped, err)
	}
	return e
}

// economyItem returns a copy of an economy seed item (by its id field value) modified by edit.
func economyItem(t *testing.T, id string, edit func(m map[string]any)) command.ContentItem {
	t.Helper()
	for _, it := range economySeed(t) {
		var m map[string]any
		_ = json.Unmarshal(it.Payload, &m)
		for _, k := range []string{"item_id", "shop_item_id", "collection_id", "achievement_id"} {
			if m[k] == id {
				if edit != nil {
					edit(m)
				}
				b, _ := json.Marshal(m)
				return command.ContentItem{Type: it.Type, Payload: b}
			}
		}
	}
	t.Fatalf("no economy seed item %s", id)
	return command.ContentItem{}
}

// rewardRule is the seed rule ruleID re-published with grants (a new version, effective now).
func (e *env) rewardRule(ctx context.Context, t *testing.T, ruleID string, grants ...map[string]any) {
	t.Helper()
	gs := make([]any, 0, len(grants))
	for _, g := range grants {
		gs = append(gs, g)
	}
	if _, err := e.publish(ctx, t, []command.ContentItem{progressionItem(t, ruleID, func(m map[string]any) { m["grants"] = gs })}, "f075 rule"); err != nil {
		t.Fatal(err)
	}
}

func xuGrant(n int) map[string]any {
	return map[string]any{"type": "currency", "currency_code": "xu", "amount": n}
}

// ecoDeliver hands an event to the Reward Engine with the economy appliers (consumer transaction of the envelope tenant).
func (e *env) ecoDeliver(ctx context.Context, t *testing.T, env outbox.Envelope) {
	t.Helper()
	e.mustDeliver(ctx, t, e.eco().prog, env)
}

// inTenant runs fn in a transaction of tenant (as a consumer would).
func (e *env) inTenant(ctx context.Context, t *testing.T, tenant uuid.UUID, fn func(ctx context.Context) error) error {
	t.Helper()
	return e.uow.InTenantTx(ctx, scopeOf(t, tenant), fn)
}

// balanceOf is the wallet balance of s (0 without a wallet; read RLS-free as the platform role).
func (e *env) balanceOf(ctx context.Context, t *testing.T, s student) int64 {
	t.Helper()
	var n int64
	err := e.platform.QueryRowContext(ctx, `SELECT coalesce((SELECT balance FROM gamification.wallets WHERE student_id = $1 AND currency_code = 'xu'
		AND deleted_at IS NULL), 0)`, s.id).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// ledgerSum is Σ currency_ledger.delta of s.
func (e *env) ledgerSum(ctx context.Context, t *testing.T, s student) int64 {
	t.Helper()
	var n int64
	if err := e.platform.QueryRowContext(ctx, `SELECT coalesce(sum(delta), 0) FROM gamification.currency_ledger WHERE student_id = $1`, s.id).
		Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// ownedOf is the quantity of item s owns (0 when none).
func (e *env) ownedOf(ctx context.Context, t *testing.T, s student, item string) int {
	t.Helper()
	return count(ctx, t, e.platform, `SELECT coalesce((SELECT quantity FROM game.inventory_items WHERE student_id = $1 AND item_id = $2
		AND deleted_at IS NULL), 0)`, s.id, item)
}

// credit gives s amount Xu through a published rule (quest side completed, a currency grant) — the real path of a reward.
func (e *env) credit(ctx context.Context, t *testing.T, s student, amount int) {
	t.Helper()
	e.rewardRule(ctx, t, ruleQuestSide, map[string]any{"type": "xp", "amount": 80}, xuGrant(amount))
	e.clock.Add(1e9)
	e.ecoDeliver(ctx, t, e.questEvent(s, "side"))
}

// buy calls purchaseShopItem.
func (e *env) buy(t *testing.T, s student, shopItem, key string, price, qty int) resp {
	t.Helper()
	h := th(s)
	if key != "" {
		h["Idempotency-Key"] = key
	}
	body := map[string]any{"expected_price": price}
	if qty > 0 {
		body["quantity"] = qty
	}
	return e.do(t, http.MethodPost, "/game/shop/items/"+shopItem+":purchase", e.gtoken(s), h, body)
}

// achievementConsumer hands an event to the achievement-evaluator in a transaction of the envelope tenant.
func (e *env) achieve(ctx context.Context, t *testing.T, env outbox.Envelope) {
	t.Helper()
	c := consumer.AchievementConsumer{Achievements: e.eco().eco}
	if err := e.inTenant(ctx, t, *env.TenantID, func(ctx context.Context) error { return c.Handle(ctx, env) }); err != nil {
		t.Fatalf("%s: %v", env.EventType, err)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
