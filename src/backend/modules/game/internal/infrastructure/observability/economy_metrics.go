package observability

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
)

// Metrics of the economy (F075 T041, plan "Quan sát"). The wallet mismatches use eduworld_game_ledger_mismatch_total
// {ledger="currency"} of F074.
var (
	currencyDelta = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_currency_delta_total", Help: "Xu posted to the currency ledger by reason code and sign (F075).",
	}, []string{"reason_code", "sign"})
	shopPurchases = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_shop_purchase_total", Help: "Shop purchases by result (ok or the problem code, F075).",
	}, []string{"result"})
	achievementsUnlocked = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "eduworld_game_achievement_unlocked_total", Help: "Achievement tiers unlocked (one per achievement.unlocked, F075).",
	})
	adjustments = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_game_economy_adjustment_total", Help: "Economy adjustments by status reached (F075, four-eyes).",
	}, []string{"status"})
)

// RegisterEconomy registers the F075 metrics.
func RegisterEconomy(r prometheus.Registerer) {
	for _, c := range []prometheus.Collector{currencyDelta, shopPurchases, achievementsUnlocked, adjustments, ledgerMismatch} {
		_ = r.Register(c)
	}
}

// EconomyMetrics implements command.EconomyMetrics.
type EconomyMetrics struct{}

var _ command.EconomyMetrics = EconomyMetrics{}

// CurrencyDelta counts the Xu of one posting.
func (EconomyMetrics) CurrencyDelta(reason string, delta int64) {
	sign, n := "credit", delta
	if delta < 0 {
		sign, n = "debit", -delta
	}
	currencyDelta.WithLabelValues(reason, sign).Add(float64(n))
}

// ShopPurchase counts one purchase attempt by result.
func (EconomyMetrics) ShopPurchase(result string) { shopPurchases.WithLabelValues(result).Inc() }

// LedgerMismatch counts one wallet whose balance differs from Σ currency_ledger.
func (EconomyMetrics) LedgerMismatch() { ledgerMismatch.WithLabelValues("currency").Inc() }

// AchievementUnlocked counts one tier unlocked.
func (EconomyMetrics) AchievementUnlocked() { achievementsUnlocked.Inc() }

// Adjustment counts one adjustment reaching status.
func (EconomyMetrics) Adjustment(status string) { adjustments.WithLabelValues(status).Inc() }
