//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F074-AC-23: a curve or a reward rule outside its JSON Schema, with thresholds that decrease, a cap ≤ 0, a source code
// outside the catalog, a trigger outside the list or a reward type without applier is refused with 422
// VALIDATION_FAILED naming the field; a second curve is 409 CONFLICT (Release 1.0: one curve); nothing is published.
func TestProgressionPublishChecks(t *testing.T) {
	ctx := context.Background()
	e := setupProgression(ctx, t)
	cases := map[string]command.ContentItem{
		"max_level": progressionItem(t, seedCurve, func(m map[string]any) { m["max_level"] = 500 }),
		"level_overrides": progressionItem(t, seedCurve, func(m map[string]any) {
			m["level_overrides"] = []any{map[string]any{"level": 5, "xp_to_next": 10}}
		}),
		"level_rewards/0/rewards/0/type": progressionItem(t, seedCurve, func(m map[string]any) {
			m["level_rewards"] = []any{map[string]any{"level": 5, "rewards": []any{map[string]any{"type": "currency", "currency_code": "xu", "amount": 50}}}}
		}),
		"caps/xp_per_day": progressionItem(t, ruleLesson, func(m map[string]any) { m["caps"] = map[string]any{"xp_per_day": 0} }),
		"source_code":     progressionItem(t, ruleLesson, func(m map[string]any) { m["source_code"] = "login.daily" }),
		"trigger/event_type": progressionItem(t, ruleLesson, func(m map[string]any) {
			m["trigger"] = map[string]any{"event_type": "game_session.started"}
		}),
		"grants/0/type": progressionItem(t, ruleLesson, func(m map[string]any) {
			m["grants"] = []any{map[string]any{"type": "item", "item_id": "hat_red"}}
		}),
		"trigger/event_type ": progressionItem(t, ruleLesson, func(m map[string]any) {
			m["trigger"] = map[string]any{"event_type": "match.completed"} // in the schema, not a Reward Engine trigger of F074
		}),
	}
	versions := count(ctx, t, e.platform, `SELECT count(*) FROM game.progression_curve_versions`) +
		count(ctx, t, e.platform, `SELECT count(*) FROM gamification.reward_rule_versions`)
	for field, item := range cases {
		_, err := e.publish(ctx, t, []command.ContentItem{item}, "invalid")
		var pe *problem.Error
		if !errors.As(err, &pe) || pe.Code != problem.ValidationFailed || len(pe.Fields) == 0 {
			t.Fatalf("%s: %v", field, err)
		}
		if want := trimSpace(field); pe.Fields[0].Field != want && pe.Fields[0].Field != "/"+want {
			t.Fatalf("%s: field %q (%v)", field, pe.Fields[0].Field, err)
		}
	}
	second := progressionItem(t, seedCurve, func(m map[string]any) {
		m["curve_id"] = uuid.NewString()
		m["code"] = "progression.second"
	})
	_, err := e.publish(ctx, t, []command.ContentItem{second}, "second curve")
	var pe *problem.Error
	if !errors.As(err, &pe) || pe.Code != problem.Conflict {
		t.Fatalf("second curve: %v", err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.progression_curve_versions`) +
		count(ctx, t, e.platform, `SELECT count(*) FROM gamification.reward_rule_versions`); n != versions {
		t.Fatalf("refused content published: %d → %d", versions, n)
	}
	// The seed is valid and published once: running it again publishes nothing.
	res, err := e.publish(ctx, t, progressionSeed(t), "seed again")
	if err != nil || res.Published != 0 || res.Unchanged != 10 {
		t.Fatalf("seed again: %+v %v", res, err)
	}
}

func trimSpace(s string) string {
	for len(s) > 0 && s[len(s)-1] == ' ' {
		s = s[:len(s)-1]
	}
	return s
}
