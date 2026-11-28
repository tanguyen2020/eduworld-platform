//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
)

// F075-AC-01 (TC-ECO-002): a currency grant of a reward rule posts a positive Xu entry (reason reward, balance_after) and
// moves the wallet in the transaction of the RewardGrant, with currency.credited; a replayed event adds nothing, so three
// deliveries of which one replay give two postings and balance = Σ ledger.
func TestCurrencyGrantInRewardTransaction(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.rewardRule(ctx, t, ruleQuestSide, map[string]any{"type": "xp", "amount": 80}, xuGrant(20))
	e.clock.Add(1e9)
	first := e.questEvent(an, "side")
	e.ecoDeliver(ctx, t, first)
	e.ecoDeliver(ctx, t, first) // redelivery
	e.clock.Add(1e9)
	e.ecoDeliver(ctx, t, e.questEvent(an, "side"))
	if got := e.balanceOf(ctx, t, an); got != 40 || e.ledgerSum(ctx, t, an) != 40 {
		t.Fatalf("balance %d, ledger %d", got, e.ledgerSum(ctx, t, an))
	}
	rows, err := e.platform.QueryContext(ctx, `SELECT l.delta, l.balance_after, l.reason_code, l.source_type, g.trigger_kind
		FROM gamification.currency_ledger l JOIN gamification.reward_grants g ON g.reward_grant_id = l.source_id
		WHERE l.student_id = $1 ORDER BY l.created_at, l.balance_after`, an.id)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var after []int64
	for rows.Next() {
		var d, b int64
		var reason, src, kind string
		if err := rows.Scan(&d, &b, &reason, &src, &kind); err != nil {
			t.Fatal(err)
		}
		if d != 20 || reason != "reward" || src != "reward_grant" || kind != "event_rule" {
			t.Fatalf("posting %d %s %s %s", d, reason, src, kind)
		}
		after = append(after, b)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(after) != 2 || after[0] != 20 || after[1] != 40 {
		t.Fatalf("balance_after %v", after)
	}
	if n := e.events(ctx, t, "currency.credited"); n != 2 {
		t.Fatalf("currency.credited %d", n)
	}
	if seen := e.checkEventSchemas(ctx, t); seen["currency.credited"] != 2 || seen["reward.granted"] != 2 {
		t.Fatalf("schemas %v", seen)
	}
}

// F075-AC-02: Xu of a day stop at currencies.daily_cap (300, ICT day): the posting that reaches it credits only the rest
// (capped), later ones nothing; repeating one content version diminishes the Xu like the XP (×1, ×0.5, ×0.25).
func TestCurrencyDailyCapAndDiminishing(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.rewardRule(ctx, t, ruleQuestSide, map[string]any{"type": "xp", "amount": 80}, xuGrant(200))
	for i := 0; i < 3; i++ {
		e.clock.Add(1e9)
		e.ecoDeliver(ctx, t, e.questEvent(an, "side"))
	}
	var deltas []int64
	var capped []bool
	func() {
		rows, err := e.platform.QueryContext(ctx, `SELECT delta, capped FROM gamification.currency_ledger WHERE student_id = $1 ORDER BY created_at`, an.id)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var d int64
			var c bool
			_ = rows.Scan(&d, &c)
			deltas, capped = append(deltas, d), append(capped, c)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
	}()
	if len(deltas) != 2 || deltas[0] != 200 || deltas[1] != 100 || capped[0] || !capped[1] || e.balanceOf(ctx, t, an) != 300 {
		t.Fatalf("cap: %v %v balance %d", deltas, capped, e.balanceOf(ctx, t, an))
	}
	r := e.do(t, http.MethodGet, "/game/wallet", e.gtoken(an), th(an), nil)
	if r.status != http.StatusOK || r.body["balance"] != float64(300) || r.body["daily_earn_remaining"] != float64(0) || r.body["currency_code"] != "xu" {
		t.Fatalf("wallet at cap: %d %s", r.status, r.raw)
	}
	// Next ICT day: the cap starts again.
	e.clock.Add(24 * 3600e9)
	e.ecoDeliver(ctx, t, e.questEvent(an, "side"))
	if got := e.balanceOf(ctx, t, an); got != 500 {
		t.Fatalf("next day: %d", got)
	}
	// Diminishing returns per content version (activity.graded of one version three times).
	binh := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.rewardRule(ctx, t, ruleActivity, map[string]any{"type": "xp", "per_correct_answer": 3}, xuGrant(40))
	cv := uuid.New()
	for i := 0; i < 3; i++ {
		e.clock.Add(1e9)
		e.ecoDeliver(ctx, t, e.gradedEvent(binh, cv, 5, 5, nil))
	}
	if got := e.balanceOf(ctx, t, binh); got != 70 { // 40 + 20 + 10
		t.Fatalf("diminishing: %d", got)
	}
}

// F075-AC-03, AC-05 (TC-ECO-002, TC-ECO-008): the balance never goes negative (CHECK) and the ledgers are append-only for
// game_app and game_platform; wallet-reconcile reports a wallet that differs from its ledger (metric, audit) and never
// corrects it; tenant A never reads the wallet or the ledger of tenant B.
func TestWalletReconcileAndAppendOnly(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	e.credit(ctx, t, an, 30)
	e.credit(ctx, t, dung, 30)
	uc := e.eco().eco
	if n, err := uc.ReconcileWallets(ctx, 100); err != nil || n != 0 {
		t.Fatalf("clean run: %d %v", n, err)
	}
	for _, db := range []struct {
		name string
		exec func(q string) error
	}{
		{"game_app", func(q string) error { _, err := e.app.ExecContext(ctx, q); return err }},
		{"game_platform", func(q string) error { _, err := e.platform.ExecContext(ctx, q); return err }},
	} {
		for _, stmt := range []string{`UPDATE gamification.currency_ledger SET delta = 9999`, `DELETE FROM gamification.currency_ledger`,
			`UPDATE gamification.item_ledger SET delta = 9`, `DELETE FROM gamification.item_ledger`, `DELETE FROM gamification.wallets`,
			`UPDATE gamification.shop_purchases SET price_amount = 1`, `DELETE FROM gamification.economy_adjustments`} {
			if err := db.exec(stmt); err == nil || !strings.Contains(err.Error(), "permission denied") {
				t.Fatalf("%s may run %q: %v", db.name, stmt, err)
			}
		}
	}
	if _, err := e.platform.ExecContext(ctx, `UPDATE gamification.wallets SET balance = -1 WHERE student_id = $1`, an.id); err == nil {
		t.Fatal("a negative balance was stored")
	}
	if _, err := e.platform.ExecContext(ctx, `UPDATE gamification.wallets SET balance = 999 WHERE student_id = $1`, an.id); err != nil {
		t.Fatal(err)
	}
	before := testutil.ToFloat64(observability.LedgerMismatches().WithLabelValues("currency"))
	if n, err := uc.ReconcileWallets(ctx, 100); err != nil || n != 1 {
		t.Fatalf("mismatch run: %d %v", n, err)
	}
	if got := testutil.ToFloat64(observability.LedgerMismatches().WithLabelValues("currency")) - before; got != 1 {
		t.Fatalf("metric +%v", got)
	}
	if n := e.audits(ctx, t, "game.currency_ledger.mismatch"); n != 1 {
		t.Fatalf("audit %d", n)
	}
	if got := e.balanceOf(ctx, t, an); got != 999 {
		t.Fatalf("reconcile corrected the wallet: %d", got)
	}
	for _, table := range []string{"gamification.wallets", "gamification.currency_ledger", "gamification.reward_grants"} {
		var n int
		if err := e.inTenant(ctx, t, e.tenantA, func(ctx context.Context) error {
			return txOf(ctx).QueryRowContext(ctx, `SELECT count(*) FROM `+table+` WHERE student_id = $1`, dung.id).Scan(&n)
		}); err != nil || n != 0 {
			t.Fatalf("%s of tenant B visible in A: %d %v", table, n, err)
		}
	}
}

// F075-AC-04: getWallet and listWalletLedger answer for the student itself only: balance, Xu still earnable today, cursor
// history newest first with a friendly label (vi/en) and no technical id.
func TestWalletAPI(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	other := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	r := e.do(t, http.MethodGet, "/game/wallet", e.gtoken(an), th(an), nil)
	if r.status != http.StatusOK || r.body["balance"] != float64(0) || r.body["daily_earn_remaining"] != float64(300) {
		t.Fatalf("empty wallet: %d %s", r.status, r.raw)
	}
	for _, n := range []int{10, 20, 30} {
		e.credit(ctx, t, an, n)
	}
	e.credit(ctx, t, other, 50)
	r = e.do(t, http.MethodGet, "/game/wallet", e.gtoken(an), th(an), nil)
	if r.body["balance"] != float64(60) || r.body["daily_earn_remaining"] != float64(240) {
		t.Fatalf("wallet: %s", r.raw)
	}
	r = e.do(t, http.MethodGet, "/game/wallet/ledger?limit=2", e.gtoken(an), map[string]string{"X-Tenant-Id": an.tenant.String(),
		"Accept-Language": "en-US"}, nil)
	data, _ := r.body["data"].([]any)
	pg, _ := r.body["page"].(map[string]any)
	if r.status != http.StatusOK || len(data) != 2 || pg["has_more"] != true {
		t.Fatalf("page 1: %d %s", r.status, r.raw)
	}
	first := data[0].(map[string]any)
	if first["delta"] != float64(30) || first["balance_after"] != float64(60) || first["source_label"] != "Reward" || first["reason_code"] != "reward" {
		t.Fatalf("newest first: %+v", first)
	}
	for _, banned := range []string{"source_id", "entry_id", "idempotency_key", "student_id", "tenant_id"} {
		if strings.Contains(r.raw, banned) {
			t.Fatalf("ledger exposes %s: %s", banned, r.raw)
		}
	}
	r = e.do(t, http.MethodGet, "/game/wallet/ledger?limit=2&cursor="+pg["next_cursor"].(string), e.gtoken(an), th(an), nil)
	data, _ = r.body["data"].([]any)
	if r.status != http.StatusOK || len(data) != 1 || data[0].(map[string]any)["delta"] != float64(10) ||
		data[0].(map[string]any)["source_label"] != "Phần thưởng" || r.body["page"].(map[string]any)["has_more"] != false {
		t.Fatalf("page 2: %d %s", r.status, r.raw)
	}
	for _, q := range []string{"limit=0", "limit=101", "cursor=not-a-cursor"} {
		if r := e.do(t, http.MethodGet, "/game/wallet/ledger?"+q, e.gtoken(an), th(an), nil); r.status != http.StatusUnprocessableEntity {
			t.Fatalf("%s: %d %s", q, r.status, r.raw)
		}
	}
}
