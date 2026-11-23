package collection

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

// F075-AC-18: an item is counted once; completion is reported only by the call that completes the collection.
func TestProgressCompletesOnce(t *testing.T) {
	p := Progress{}
	if added, done := p.Add("pet_cat", 2); !added || done {
		t.Fatal("first item")
	}
	if added, _ := p.Add("pet_cat", 2); added {
		t.Fatal("same item counted twice")
	}
	if added, done := p.Add("pet_dog", 2); !added || !done {
		t.Fatal("completion")
	}
	if _, done := p.Add("pet_fox", 2); done {
		t.Fatal("completion reported twice")
	}
}

// F075-AC-19: proficient or mastered earns a card; lower levels never do (and never take one back).
func TestEarnsCard(t *testing.T) {
	for level, want := range map[string]bool{"proficient": true, "mastered": true, "developing": false, "beginning": false, "": false} {
		if EarnsCard(level) != want {
			t.Fatalf("%q", level)
		}
	}
}

func TestValidate(t *testing.T) {
	obj := uuid.New()
	c := Collection{CollectionID: uuid.New(), Code: "cards.math_g3", Type: "knowledge_card", Visibility: HiddenUntilFirst,
		Items:   []Item{{ItemID: "card_a", SourceHint: Hint{"Học", "Learn"}, ObjectiveID: &obj}},
		Rewards: []map[string]any{{"type": "currency", "currency_code": "xu", "amount": 50.0}}}
	yes := func(string) bool { return true }
	if err := c.Validate(yes); err != nil {
		t.Fatal(err)
	}
	for name, mut := range map[string]func(*Collection){
		"no objective": func(c *Collection) { c.Items = []Item{{ItemID: "card_a", SourceHint: Hint{"a", "b"}}} },
		"twice": func(c *Collection) {
			c.Type, c.Items = "pet", []Item{{ItemID: "p", SourceHint: Hint{"a", "b"}}, {ItemID: "p", SourceHint: Hint{"a", "b"}}}
		},
		"hint":       func(c *Collection) { c.Items[0].SourceHint.EN = "" },
		"xp reward":  func(c *Collection) { c.Rewards = []map[string]any{{"type": "xp", "amount": 5.0}} },
		"visibility": func(c *Collection) { c.Visibility = "secret" },
		"empty":      func(c *Collection) { c.Items = nil },
	} {
		cc := c
		cc.Items = append([]Item(nil), c.Items...)
		mut(&cc)
		if err := cc.Validate(yes); !errors.Is(err, ErrInvalid) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if c.ItemIDs()[0] != "card_a" {
		t.Fatal("ItemIDs")
	}
}
