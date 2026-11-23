package reward

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func challengeRule(code string, statuses []string, first bool, grants ...Grant) Rule {
	return Rule{RuleID: uuid.New(), VersionID: uuid.New(), Version: 1, Code: code, SourceCode: "challenge.completed",
		EventType: "challenge.completed", Conditions: Conditions{ChallengeStatuses: statuses, FirstCompletionOnly: first}, Grants: grants,
		Caps: Caps{XPPerDay: 600}}
}

func completed(status string, mult float64, firstClear bool) Trigger {
	cv := uuid.New()
	return Trigger{EventType: "challenge.completed", SourceType: "challenge", SourceID: uuid.New(), SourceCodes: []string{"challenge.completed"},
		ContentVersionID: &cv, ChallengeStatus: status, RewardMultiplier: mult, FirstClear: firstClear}
}

// F076-AC-11, F074 caps: challenge.completed rewards a succeeded attempt (default) with the XP scaled by the
// reward_multiplier of the level (rounded down, before the caps); failed only when a rule lists it; abandoned never; a
// first-clear rule only on the first clear; nothing is ever negative (difficulty never punishes).
func TestChallengeCompleted(t *testing.T) {
	base := challengeRule("xp.challenge", nil, false, Grant{Type: "xp", Amount: 50, ScaleBy: "reward_multiplier"})
	if got := amount(t, eval(policy(), completed("succeeded", 1.6, false), []Rule{base}, 3, History{}, nil)); got != 80 {
		t.Fatalf("hard ×1.6: %d", got)
	}
	if got := amount(t, eval(policy(), completed("succeeded", 2.5, false), []Rule{base}, 3, History{}, nil)); got != 125 {
		t.Fatalf("legendary ×2.5: %d", got)
	}
	for _, s := range []string{"failed", "abandoned"} {
		if as := eval(policy(), completed(s, 1.6, false), []Rule{base}, 3, History{}, nil); len(as) != 0 {
			t.Fatalf("%s rewarded by a default rule: %+v", s, as)
		}
	}
	effort := challengeRule("xp.challenge_effort", []string{"failed"}, false, Grant{Type: "xp", Amount: 10})
	if got := amount(t, eval(policy(), completed("failed", 1.6, false), []Rule{effort}, 3, History{}, nil)); got != 10 {
		t.Fatalf("effort reward on failure: %d", got)
	}
	both := challengeRule("xp.challenge_any", []string{"succeeded", "failed"}, false, Grant{Type: "xp", Amount: 10})
	if as := eval(policy(), completed("abandoned", 1, false), []Rule{both}, 3, History{}, nil); len(as) != 0 {
		t.Fatal("abandoned is never rewarded")
	}
	first := challengeRule("xp.challenge_first", nil, true, Grant{Type: "xp", Amount: 100}, Grant{Type: "currency", CurrencyCode: "xu",
		Amount: 15, ScaleBy: "reward_multiplier"})
	as := eval(policy(), completed("succeeded", 2, true), []Rule{first}, 3, History{}, nil)
	if amount(t, as) != 100 || len(as[0].Rewarded) != 1 || as[0].Rewarded[0].Amount != 30 {
		t.Fatalf("first clear: %+v", as)
	}
	if as := eval(policy(), completed("succeeded", 2, false), []Rule{first}, 3, History{}, nil); len(as) != 0 {
		t.Fatal("first clear only once (not the first clear)")
	}
	if as := eval(policy(), completed("succeeded", 2, true), []Rule{first}, 3, History{Ever: 1}, nil); len(as) != 0 {
		t.Fatal("first clear only once (already posted)")
	}
	// Caps still apply after the multiplier (F074: never bypassed).
	capped := challengeRule("xp.challenge", nil, false, Grant{Type: "xp", Amount: 500, ScaleBy: "reward_multiplier"})
	if as := eval(policy(), completed("succeeded", 2.5, false), []Rule{capped}, 3, History{}, map[string]int{TotalBucket: 900}); amount(t, as) != 100 ||
		!as[0].Capped {
		t.Fatalf("capped: %+v", as)
	}
	// scale_by is a challenge.completed grant only; the trigger is a Reward Engine trigger.
	r := challengeRule("xp.challenge", nil, false, Grant{Type: "xp", Amount: 10})
	if err := r.Validate(func(string) bool { return true }); err != nil {
		t.Fatal(err)
	}
	bad := activityRule(10, 0)
	bad.Grants[0].ScaleBy = "reward_multiplier"
	var fe *FieldError
	if err := bad.Validate(func(string) bool { return true }); !errors.As(err, &fe) || fe.Field != "grants/0/scale_by" {
		t.Fatalf("scale_by on activity.graded: %v", err)
	}
	st := challengeRule("xp.challenge", []string{"failed"}, false, Grant{Type: "xp", Amount: 10})
	st.EventType, st.SourceCode = "quest.completed", "quest.main_completed"
	if err := st.Validate(func(string) bool { return true }); !errors.As(err, &fe) || fe.Field != "conditions/challenge_statuses" {
		t.Fatalf("challenge_statuses on quest.completed: %v", err)
	}
}
