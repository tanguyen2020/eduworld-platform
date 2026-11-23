//go:build integration

package integration

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func achievementByCode(r resp, code string) map[string]any {
	for _, d := range r.body["data"].([]any) {
		if m := d.(map[string]any); m["code"] == code {
			return m
		}
	}
	return nil
}

// F075-AC-21, AC-22, AC-24 (TC-ECO-005): the achievement-evaluator advances the progress once per event (a redelivered
// event counts nothing again), writes achievement.progressed, unlocks each tier once per (student, achievement, tier) with
// its rewards through the Reward Engine and achievement.unlocked; listAchievements never shows a hidden achievement before
// it is unlocked; a new version never lowers the progress nor unlocks a tier twice.
func TestAchievements(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	r := e.do(t, http.MethodGet, "/game/achievements", e.gtoken(an), th(an), nil)
	if r.status != http.StatusOK || achievementByCode(r, "hidden.first_purchase") != nil || strings.Contains(r.raw, "Người mua sắm") ||
		strings.Contains(r.raw, achHidden) {
		t.Fatalf("hidden achievement leaks: %d %s", r.status, r.raw)
	}
	explorer := achievementByCode(r, "progress.quest_explorer")
	if explorer == nil || explorer["unlocked"] != false || explorer["tier"] != "none" || explorer["progress_current"] != float64(0) ||
		explorer["progress_target"] != float64(3) {
		t.Fatalf("explorer before: %+v", explorer)
	}
	first := e.questEvent(an, "side")
	e.achieve(ctx, t, first)
	e.achieve(ctx, t, first) // redelivery counts nothing
	e.achieve(ctx, t, e.questEvent(an, "main"))
	if n := e.events(ctx, t, "achievement.unlocked"); n != 0 {
		t.Fatalf("unlocked at 2: %d", n)
	}
	e.achieve(ctx, t, e.questEvent(an, "daily"))
	if n := e.events(ctx, t, "achievement.unlocked"); n != 1 || e.balanceOf(ctx, t, an) != 20 {
		t.Fatalf("bronze: %d unlocked, %d Xu", n, e.balanceOf(ctx, t, an))
	}
	if n := e.events(ctx, t, "achievement.progressed"); n != 3 {
		t.Fatalf("achievement.progressed %d", n)
	}
	r = e.do(t, http.MethodGet, "/game/achievements?category=progress", e.gtoken(an), th(an), nil)
	explorer = achievementByCode(r, "progress.quest_explorer")
	rewards, _ := explorer["rewards"].([]any)
	if explorer["unlocked"] != true || explorer["tier"] != "bronze" || explorer["progress_current"] != float64(3) || explorer["progress_target"] != float64(10) ||
		len(rewards) != 1 || rewards[0].(map[string]any)["kind"] != "badge" || explorer["unlocked_at"] == nil {
		t.Fatalf("explorer after bronze: %+v", explorer)
	}
	// AC-24: a new version (bronze 5, silver 6) keeps the progress and never unlocks bronze again.
	if _, err := e.publish(ctx, t, []command.ContentItem{economyItem(t, achExplorer, func(m map[string]any) {
		m["tiers"] = []any{map[string]any{"tier": "bronze", "threshold": 5, "rewards": []any{xuGrant(20)}},
			map[string]any{"tier": "silver", "threshold": 6, "rewards": []any{map[string]any{"type": "badge", "badge_id": "badge_explorer_01"}}}}
	})}, "explorer v2"); err != nil {
		t.Fatal(err)
	}
	e.achieve(ctx, t, e.questEvent(an, "side"))
	if p := count(ctx, t, e.platform, `SELECT progress FROM gamification.student_achievements WHERE student_id = $1 AND achievement_id = $2`, an.id,
		achExplorer); p != 4 || e.events(ctx, t, "achievement.unlocked") != 1 || e.balanceOf(ctx, t, an) != 20 {
		t.Fatalf("v2 at 4: progress %d, unlocked %d, Xu %d", p, e.events(ctx, t, "achievement.unlocked"), e.balanceOf(ctx, t, an))
	}
	e.achieve(ctx, t, e.questEvent(an, "side"))
	e.achieve(ctx, t, e.questEvent(an, "side"))
	if e.events(ctx, t, "achievement.unlocked") != 2 || e.ownedOf(ctx, t, an, "badge_explorer_01") != 1 {
		t.Fatalf("silver: %d unlocked", e.events(ctx, t, "achievement.unlocked"))
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM gamification.reward_grants WHERE student_id = $1 AND trigger_kind = 'achievement'`, an.id); n != 2 {
		t.Fatalf("tier grants %d", n)
	}
	// The hidden achievement: unlocked by the first shop purchase, then listed with its name.
	e.achieve(ctx, t, e.envelope("shop.item_purchased", "game", an.tenant, map[string]any{"purchase_id": uuid.New(), "student_id": an.id,
		"shop_item_id": shopHat, "shop_item_version_id": uuid.New(), "item_id": "hat_star_01", "item_kind": "accessory", "price": 120, "quantity": 1,
		"currency_code": "xu", "balance_after": 0, "currency_entry_id": uuid.New(), "purchased_at": e.clock.Now()}))
	r = e.do(t, http.MethodGet, "/game/achievements", e.gtoken(an), map[string]string{"X-Tenant-Id": an.tenant.String(), "Accept-Language": "vi-VN"}, nil)
	hidden := achievementByCode(r, "hidden.first_purchase")
	if hidden == nil || hidden["unlocked"] != true || hidden["tier"] != "single" || e.ownedOf(ctx, t, an, "badge_shopper_01") != 1 {
		t.Fatalf("hidden after unlock: %s", r.raw)
	}
	if got := len(e.payloads(ctx, t, "achievement.unlocked")); got != 3 {
		t.Fatalf("unlocked events %d", got)
	}
	if seen := e.checkEventSchemas(ctx, t); seen["achievement.unlocked"] != 3 || seen["achievement.progressed"] < 6 {
		t.Fatalf("schemas %v", seen)
	}
}

// F075-AC-23: an achievement version with a trigger outside the catalog or a metric on play time, logins, sessions,
// messages or an official score is refused with 422 VALIDATION_FAILED naming the field; an xp tier reward too (XP only
// from reward rules); nothing is published.
func TestAchievementPublishChecks(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	before := count(ctx, t, e.platform, `SELECT count(*) FROM gamification.achievement_versions`)
	cases := map[string]func(m map[string]any){
		"trigger/event_types/0": func(m map[string]any) { m["trigger"] = map[string]any{"event_types": []any{"student.game_logged_in"}} },
		"progress_metric/sum_field": func(m map[string]any) {
			m["progress_metric"] = map[string]any{"kind": "sum", "sum_field": "duration_ms"}
		},
		"progress_metric/distinct_field": func(m map[string]any) {
			m["progress_metric"] = map[string]any{"kind": "distinct_count", "distinct_field": "login_date"}
		},
		"progress_metric/sum_field ": func(m map[string]any) {
			m["progress_metric"] = map[string]any{"kind": "sum", "sum_field": "message_count"}
		},
		"progress_metric/sum_field  ": func(m map[string]any) { m["progress_metric"] = map[string]any{"kind": "sum", "sum_field": "score"} },
		"tiers/0/rewards/0/type": func(m map[string]any) {
			m["tiers"] = []any{map[string]any{"tier": "single", "threshold": 1, "rewards": []any{map[string]any{"type": "xp", "amount": 50}}}}
		},
	}
	for field, edit := range cases {
		_, err := e.publish(ctx, t, []command.ContentItem{economyItem(t, achExplorer, edit)}, "invalid")
		var pe *problem.Error
		if !errors.As(err, &pe) || pe.Code != problem.ValidationFailed || len(pe.Fields) == 0 {
			t.Fatalf("%s: %v", field, err)
		}
		if want := trimSpace(field); pe.Fields[0].Field != want && pe.Fields[0].Field != "/"+want {
			t.Fatalf("%s: field %q (%v)", field, pe.Fields[0].Field, err)
		}
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM gamification.achievement_versions`); n != before {
		t.Fatalf("refused achievement published: %d → %d", before, n)
	}
}
