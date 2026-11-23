package consumer

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/reward"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

func envelope(t *testing.T, typ string, payload map[string]any) outbox.Envelope {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	tenant := uuid.New()
	return outbox.Envelope{EventID: uuid.New(), EventType: typ, SchemaVersion: 1, OccurredAt: time.Now(), TenantID: &tenant, Payload: b}
}

// F073-AC-19, AC-20 (F074-1): minigame.completed is a Reward Engine trigger restricted to the reward_rule_id of the config;
// an eligible play earns activity.completed, a play over max_plays_per_day or with the completion rule unmet earns
// nothing; activity.graded of a free mini-game play (mode minigame) earns nothing (no double reward).
func TestTriggerMiniGameCompleted(t *testing.T) {
	rule, play, student := uuid.New(), uuid.New(), uuid.New()
	base := func(eligible, met bool) map[string]any {
		return map[string]any{"play_id": play, "student_id": student, "minigame_config_id": uuid.New(), "minigame_config_version_id": uuid.New(),
			"minigame_type": "matching", "attempt_id": uuid.New(), "context_type": "free_play", "learning_score": 3, "max_score": 3, "game_score": 330,
			"correct_count": 3, "item_count": 3, "completion_met": met, "rewards_eligible": eligible, "plays_today": 1, "reward_rule_id": rule,
			"grade_levels": []int{3}, "completed_at": time.Now()}
	}
	s, tr, err := Trigger(envelope(t, "minigame.completed", base(true, true)))
	if err != nil || s != student || tr.SourceType != "minigame" || tr.SourceID != play || tr.RuleID == nil || *tr.RuleID != rule ||
		len(tr.SourceCodes) != 1 || tr.SourceCodes[0] != "activity.completed" || tr.Mode != "minigame" || tr.ContentVersionID == nil {
		t.Fatalf("eligible: %+v %v", tr, err)
	}
	for _, c := range []struct{ eligible, met bool }{{false, true}, {false, false}} {
		if _, tr, err := Trigger(envelope(t, "minigame.completed", base(c.eligible, c.met))); err != nil || len(tr.SourceCodes) != 0 {
			t.Fatalf("not eligible %+v: %+v %v", c, tr, err)
		}
	}
	if _, _, err := Trigger(envelope(t, "minigame.completed", map[string]any{"student_id": student})); err == nil {
		t.Fatal("invalid payload accepted")
	}
	_, tr, err = Trigger(envelope(t, "activity.graded", map[string]any{"attempt_id": uuid.New(), "student_id": student, "mode": "minigame",
		"correct_count": 3, "item_count": 3}))
	if err != nil || len(tr.SourceCodes) != 0 {
		t.Fatalf("activity.graded of a free play: %+v %v", tr, err)
	}
	// The rule restriction: only the reward rule of the config matches.
	other := reward.Rule{RuleID: uuid.New(), EventType: "minigame.completed", SourceCode: "activity.completed", Grants: []reward.Grant{{Type: "xp", Amount: 10}},
		Caps: reward.Caps{XPPerDay: 100}}
	mine := other
	mine.RuleID, mine.Code = rule, "mine"
	_, tr, _ = Trigger(envelope(t, "minigame.completed", base(true, true)))
	p := reward.Policy{DailyCap: 1000, Repeat: reward.Diminishing{Multipliers: []float64{1}, WindowDays: 1}, BelowGradeMultiplier: 1, BelowGradeGap: 2}
	awards := p.Evaluate(tr, []reward.Rule{other, mine}, reward.Student{Grade: 3}, func(reward.Rule) reward.History { return reward.History{} },
		map[string]int{})
	if len(awards) != 1 || awards[0].Rule.RuleID != rule {
		t.Fatalf("awards: %+v", awards)
	}
}
