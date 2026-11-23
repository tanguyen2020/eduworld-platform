package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/economy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Defaults of CONFIG_DEFAULTS for the economy (owner INVENTORY_ECONOMY §4–§6). xu_daily_cap is gamification.currencies
// .daily_cap (the currency row, seeded at 300 by the migration f075_economy); badges_displayed_max is the
// badge_showcase_1..3 slots of CHARACTER_SYSTEM; trading_enabled is off (no route exists); real_money_purchase: none.
const (
	DefaultDuplicateItemXu         = "common=5,uncommon=10,rare=25,epic=50,legendary=100" // duplicate_item_xu
	DefaultShopPurchaseLimit       = 1                                                    // shop_purchase_limit_default
	WalletReconcileInterval        = 24 * time.Hour                                       // nightly wallet-reconcile
	EconomyJobBatch                = 500
	maxDuplicateXu, maxPurchaseLim = 100000, 999
)

// EconomyFrom reads the F075 policy (T002): EW_GAME_DUPLICATE_ITEM_XU (rarity=xu comma list) and
// EW_GAME_SHOP_PURCHASE_LIMIT_DEFAULT with the CONFIG_DEFAULTS values [Proposed: the platform settings pipeline of module
// identity carries no economy key yet — same pattern as the progression config of F074].
func EconomyFrom(c *config.Config) (command.EconomyPolicy, error) {
	d, err := ParseDuplicateTable(c.StringOr("EW_GAME_DUPLICATE_ITEM_XU", DefaultDuplicateItemXu))
	if err != nil {
		return command.EconomyPolicy{}, err
	}
	p := command.EconomyPolicy{Duplicate: d, PurchaseLimitDefault: c.Int("EW_GAME_SHOP_PURCHASE_LIMIT_DEFAULT", DefaultShopPurchaseLimit)}
	if p.PurchaseLimitDefault < 1 || p.PurchaseLimitDefault > maxPurchaseLim {
		return p, errors.New("game config: EW_GAME_SHOP_PURCHASE_LIMIT_DEFAULT must be 1..999")
	}
	return p, nil
}

// ParseDuplicateTable reads "common=5,uncommon=10,…" (every rarity, 1..100000 Xu).
func ParseDuplicateTable(s string) (economy.DuplicateTable, error) {
	d := economy.DuplicateTable{}
	for _, part := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if !ok || err != nil || n < 1 || n > maxDuplicateXu {
			return nil, fmt.Errorf("game config: EW_GAME_DUPLICATE_ITEM_XU entry %q must be rarity=1..100000", part)
		}
		d[strings.TrimSpace(k)] = n
	}
	if err := d.Validate(); err != nil {
		return nil, fmt.Errorf("game config: EW_GAME_DUPLICATE_ITEM_XU: %w", err)
	}
	return d, nil
}
