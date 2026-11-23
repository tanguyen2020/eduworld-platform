//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
)

// F074-AC-21, F074-AC-22 (TC-GAM-006, TC-GAM-005): publishing a curve version writes its level table; a raised curve
// keeps the level of a level-12 student (floor protection) with xp_to_next from the new thresholds; a lowered curve
// makes curve-rollout reach every new level in order with its rewards, and a rerun changes nothing.
func TestCurveRollout(t *testing.T) {
	ctx := context.Background()
	e := setupProgression(ctx, t)
	uc := e.progressionUC()
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.mustDeliver(ctx, t, uc, e.questEvent(s, "main")) // creates the profile on curve v1
	// Level 12 by the v1 table (cumulative 14 533 XP): set as an earlier history would have.
	if _, err := e.platform.ExecContext(ctx, `UPDATE game.game_profiles SET level = 12, xp_total = 14600 WHERE student_id = $1`, s.id); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.progression_levels`); n != 60 {
		t.Fatalf("level table v1: %d rows", n)
	}
	raised := progressionItem(t, seedCurve, func(m map[string]any) {
		m["formula"] = map[string]any{"kind": "power_linear", "base": 200, "exponent": 1.35, "linear": 20}
	})
	if _, err := e.publish(ctx, t, []command.ContentItem{raised}, "curve v2 raised"); err != nil {
		t.Fatal(err)
	}
	if n, err := uc.RolloutCurve(ctx, 100); err != nil || n != 2 { // the student of A and the earlier test profile? only profiles behind v2
		if err != nil || n < 1 {
			t.Fatalf("rollout v2: %d %v", n, err)
		}
	}
	if level, _ := e.xpOf(ctx, t, s); level != 12 {
		t.Fatalf("raised curve lowered the level: %d", level)
	}
	r := e.do(t, "GET", "/game/progression", e.gtoken(s), th(s), nil)
	if r.status != 200 || r.body["level"] != float64(12) || r.body["curve_version"] != float64(2) {
		t.Fatalf("after v2: %d %s", r.status, r.raw)
	}
	var cum13 float64
	if err := e.platform.QueryRowContext(ctx, `SELECT l.xp_cumulative FROM game.progression_levels l JOIN game.progression_curve_versions v
		ON v.curve_version_id = l.curve_version_id WHERE v.version_no = 2 AND l.level = 13`).Scan(&cum13); err != nil {
		t.Fatal(err)
	}
	if r.body["xp_to_next"] != cum13-14600 || r.body["xp_in_level"] != float64(0) {
		t.Fatalf("xp_to_next on v2: %v (want %v) in_level %v", r.body["xp_to_next"], cum13-14600, r.body["xp_in_level"])
	}
	reachedBefore := e.events(ctx, t, "level.reached")
	// v3 lowers the curve: 14 600 XP now reach level 13 and 14 (10/20/… per level): rollout reaches each in order.
	lowered := progressionItem(t, seedCurve, func(m map[string]any) {
		m["formula"] = map[string]any{"kind": "power_linear", "base": 100, "exponent": 1.2, "linear": 0}
		m["level_rewards"] = []any{map[string]any{"level": 13, "rewards": []any{map[string]any{"type": "title", "title_code": "lucky_13"}}}}
	})
	if _, err := e.publish(ctx, t, []command.ContentItem{lowered}, "curve v3 lowered"); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.RolloutCurve(ctx, 100); err != nil {
		t.Fatal(err)
	}
	level, _ := e.xpOf(ctx, t, s)
	if level <= 13 {
		t.Fatalf("lowered curve level %d", level)
	}
	reached := e.payloads(ctx, t, "level.reached")[reachedBefore:]
	if len(reached) != level-12 {
		t.Fatalf("level.reached %d for 12 → %d", len(reached), level)
	}
	for i, p := range reached {
		if p["from_level"] != float64(12+i) || p["to_level"] != float64(13+i) || p["trigger_xp_entry_id"] != nil || p["curve_version"] != float64(3) {
			t.Fatalf("reached %d: %+v", i, p)
		}
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM gamification.reward_grants WHERE student_id = $1 AND level = 13`, s.id); n != 1 {
		t.Fatalf("level 13 reward %d", n)
	}
	// A rerun of the job finds every profile on v3: nothing more.
	if n, err := uc.RolloutCurve(ctx, 100); err != nil || n != 0 {
		t.Fatalf("rerun: %d %v", n, err)
	}
	if n := e.events(ctx, t, "level.reached"); n != reachedBefore+len(reached) {
		t.Fatalf("rerun wrote level.reached: %d", n)
	}
}
