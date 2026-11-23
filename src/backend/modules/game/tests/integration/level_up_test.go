//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
)

// F074-AC-13, F074-AC-14, F074-AC-15 (TC-GAM-005): one award crossing two thresholds writes two level_history rows, two
// level.reached (n→n+1, n+1→n+2), the title of the new level and one RewardGrant per rewarded level; reaching the same
// level again after an adjustment gives no reward again.
func TestLevelUpMultipleLevelsAndRewards(t *testing.T) {
	ctx := context.Background()
	e := setupProgression(ctx, t)
	if _, err := e.publish(ctx, t, []command.ContentItem{smallCurve(t)}, "small curve"); err != nil {
		t.Fatal(err)
	}
	uc := e.progressionUC()
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.mustDeliver(ctx, t, uc, e.questEvent(s, "main")) // 150 XP: level 1 → 2 (50) → 3 (150)
	if level, xp := e.xpOf(ctx, t, s); level != 3 || xp != 150 {
		t.Fatalf("level %d xp %d", level, xp)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.level_history WHERE student_id = $1`, s.id); n != 2 {
		t.Fatalf("level_history %d", n)
	}
	reached := e.payloads(ctx, t, "level.reached")
	if len(reached) != 2 || reached[0]["from_level"] != float64(1) || reached[0]["to_level"] != float64(2) ||
		reached[1]["from_level"] != float64(2) || reached[1]["to_level"] != float64(3) || reached[1]["curve_version"] != float64(2) {
		t.Fatalf("level.reached %+v", reached)
	}
	var title string
	if err := e.platform.QueryRowContext(ctx, `SELECT title_code FROM game.game_profiles WHERE student_id = $1`, s.id).Scan(&title); err != nil ||
		title != "beginner" {
		t.Fatalf("title %q %v", title, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM gamification.reward_grants WHERE student_id = $1 AND trigger_kind = 'level_up'`,
		s.id); n != 2 {
		t.Fatalf("level-up grants %d", n)
	}
	if n := e.events(ctx, t, "reward.granted"); n != 2 {
		t.Fatalf("reward.granted %d", n)
	}
	pushes := e.progPush().of("progression.level.reached")
	if len(pushes) != 2 || pushes[1]["to_level"] != 3 || pushes[1]["rewards"] == nil {
		t.Fatalf("level pushes %+v", pushes)
	}
	// An adjustment brings the cached level back to 1; reaching levels 2 and 3 again gives no reward again.
	if _, err := e.platform.ExecContext(ctx, `UPDATE game.game_profiles SET level = 1 WHERE student_id = $1`, s.id); err != nil {
		t.Fatal(err)
	}
	e.mustDeliver(ctx, t, uc, e.lessonEvent(s, uuid.New(), []int{3}, "practice"))
	if level, _ := e.xpOf(ctx, t, s); level != 3 {
		t.Fatalf("level after adjustment %d", level)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM gamification.reward_grants WHERE student_id = $1`, s.id); n != 2 {
		t.Fatalf("grants after reaching again: %d", n)
	}
	if seen := e.checkEventSchemas(ctx, t); seen["level.reached"] != 4 || seen["reward.granted"] != 2 {
		t.Fatalf("schemas %v", seen)
	}
}

// F074-AC-16: at max_level the ledger keeps counting, xp_to_next is null and no level.reached is written.
func TestMaxLevel(t *testing.T) {
	ctx := context.Background()
	e := setupProgression(ctx, t)
	tiny := progressionItem(t, seedCurve, func(m map[string]any) {
		m["max_level"] = 3
		m["formula"] = map[string]any{"kind": "power_linear", "base": 10, "exponent": 1, "linear": 0}
		m["titles"] = []any{map[string]any{"code": "beginner", "from_level": 1, "name": map[string]any{"vi": "Tân binh", "en": "Beginner"}}}
		m["level_rewards"] = []any{}
	})
	if _, err := e.publish(ctx, t, []command.ContentItem{tiny}, "tiny curve"); err != nil {
		t.Fatal(err)
	}
	uc := e.progressionUC()
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.mustDeliver(ctx, t, uc, e.questEvent(s, "main")) // 150 ≫ 30: level 3 = max
	before := e.events(ctx, t, "level.reached")
	e.mustDeliver(ctx, t, uc, e.questEvent(s, "main"))
	if level, xp := e.xpOf(ctx, t, s); level != 3 || xp != 300 {
		t.Fatalf("level %d xp %d", level, xp)
	}
	if n := e.events(ctx, t, "level.reached"); n != before || before != 2 {
		t.Fatalf("level.reached %d → %d", before, n)
	}
	r := e.do(t, "GET", "/game/progression", e.gtoken(s), th(s), nil)
	if r.status != 200 || r.body["xp_to_next"] != nil || r.body["level"] != float64(3) || r.body["max_level"] != float64(3) {
		t.Fatalf("getProgression at max: %d %s", r.status, r.raw)
	}
}
