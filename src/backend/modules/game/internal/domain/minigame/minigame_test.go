package minigame

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func example(t *testing.T) (Config, []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "..", "..", "specs", "examples", "minigame.math-g3-matching.json"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return c, raw
}

// F073-AC-01: the example of the spec parses and validates; adaptive difficulty, an unknown type and a source without
// kind are refused.
func TestParseValidate(t *testing.T) {
	c, _ := example(t)
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := c.CheckTranslations(); err != nil {
		t.Fatal(err)
	}
	if c.Type != "matching" || c.QuestionCount != 3 || !c.ForGrade(3) || c.ForGrade(4) {
		t.Fatalf("parsed: %+v", c)
	}
	bad := c
	bad.Difficulty = json.RawMessage(`"adaptive"`)
	if err := bad.Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("adaptive: %v", err)
	}
	bad = c
	bad.Type = "racing"
	if err := bad.Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("type: %v", err)
	}
	bad = c
	bad.Source = QuestionSource{Kind: "inline"}
	if err := bad.Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("source: %v", err)
	}
	if len(Types) != 11 {
		t.Fatalf("types: %d", len(Types))
	}
}

// F073-AC-05: name and instructions need vi and en.
func TestTranslations(t *testing.T) {
	c, _ := example(t)
	c.Name.EN = " "
	if !errors.Is(c.CheckTranslations(), ErrTranslationIncomplete) {
		t.Fatal("name without en accepted")
	}
	c, _ = example(t)
	c.Instructions = &Text{VI: "Chơi nhé"}
	if !errors.Is(c.CheckTranslations(), ErrTranslationIncomplete) {
		t.Fatal("instructions without en accepted")
	}
}

// F073-AC-03, AC-07: the display config carries no question, answer, scoring rule or feedback; grade 3 sees no speed bonus,
// grade 7 does; a timed challenge is prefetched and never resumable.
func TestDisplay(t *testing.T) {
	c, _ := example(t)
	g3, g7 := 3, 7
	b, _ := json.Marshal(c.Display(&g3))
	var keys func(v any)
	keys = func(v any) {
		if m, ok := v.(map[string]any); ok {
			for k, x := range m {
				for _, bad := range []string{"question_source", "answer_key", "correct", "scoring_rule", "option_feedback", "wrong_penalty", "reward_rule_id", "questions"} {
					if k == bad {
						t.Fatalf("display leaks %q: %s", k, b)
					}
				}
				keys(x)
			}
		}
	}
	var doc any
	_ = json.Unmarshal(b, &doc)
	keys(doc)
	if c.Display(&g3)["scoring"].(map[string]any)["time_bonus_enabled"] != false || c.Display(nil)["scoring"].(map[string]any)["time_bonus_enabled"] != false {
		t.Fatal("speed bonus shown to a young or unknown grade")
	}
	if c.Display(&g7)["scoring"].(map[string]any)["time_bonus_enabled"] != true {
		t.Fatal("speed bonus hidden from grade 7")
	}
	c.Type = TimedChallenge
	if d := c.Display(&g7); d["prefetch_items"] != true || d["resumable"] != false {
		t.Fatalf("timed: %v", d)
	}
}

// F073-AC-02: incompatible or too small sources are refused.
func TestCheckSource(t *testing.T) {
	c, _ := example(t)
	if err := c.CheckSource(SourceCheck{Compatible: true, AvailableCount: 3}); err != nil {
		t.Fatal(err)
	}
	if err := c.CheckSource(SourceCheck{Compatible: false, Reasons: []string{"incompatible_question_type"}}); !errors.Is(err, ErrSourceInvalid) {
		t.Fatal(err)
	}
	if err := c.CheckSource(SourceCheck{Compatible: true, AvailableCount: 2}); !errors.Is(err, ErrSourceInvalid) {
		t.Fatal(err)
	}
}

// F073-AC-19: over max_plays_per_day the play is practice only; an unmet completion rule never earns rewards.
func TestRewardsEligible(t *testing.T) {
	c, _ := example(t) // max_plays_per_day 5
	if !c.RewardsEligible(true, 5) || c.RewardsEligible(true, 6) || c.RewardsEligible(false, 1) {
		t.Fatal("eligibility")
	}
	c.Limits.MaxPlaysPerDay = nil
	if !c.RewardsEligible(true, 100) {
		t.Fatal("no limit")
	}
}
