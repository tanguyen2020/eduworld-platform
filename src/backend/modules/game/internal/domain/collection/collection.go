// Package collection is the domain of the collections of module game (F075, INVENTORY_ECONOMY §6): a published
// collection with its items and their public "how to obtain" hints, the progress of a student (owned items, completion
// once) and the knowledge card rule (granted at mastery proficient or above, never taken back). Pure.
package collection

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
)

// ErrInvalid is a collection that cannot be published (422 VALIDATION_FAILED naming the field).
var ErrInvalid = errors.New("collection: invalid")

// FieldError names the invalid field (no value).
type FieldError struct {
	Field, Reason string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Reason }
func (e *FieldError) Unwrap() error { return ErrInvalid }

func invalid(field, format string, args ...any) error {
	return &FieldError{Field: field, Reason: fmt.Sprintf(format, args...)}
}

// Types and visibility of a collection (CollectionType, collection.schema.json).
var (
	Types        = []string{"pet", "knowledge_card", "cosmetic", "badge", "exploration", "event"}
	Visibilities = []string{"always", "hidden_until_first"}
)

// HiddenUntilFirst hides a collection from a student until the first item.
const HiddenUntilFirst = "hidden_until_first"

// RewardTypes are the completion reward types (no xp: XP only comes from reward rules — spec Clarifications).
var RewardTypes = []string{"currency", "item", "badge", "title"}

// Hint is the bilingual "how to obtain" of an item.
type Hint struct {
	VI string `json:"vi"`
	EN string `json:"en"`
}

// Item is one item of a collection.
type Item struct {
	ItemID      string     `json:"item_id"`
	SourceHint  Hint       `json:"source_hint"`
	ObjectiveID *uuid.UUID `json:"objective_id"`
}

// Collection is one collection version payload.
type Collection struct {
	CollectionID uuid.UUID        `json:"collection_id"`
	Code         string           `json:"code"`
	Type         string           `json:"collection_type"`
	Visibility   string           `json:"visibility"`
	Title        Hint             `json:"title"`
	Items        []Item           `json:"items"`
	Rewards      []map[string]any `json:"completion_rewards"`
}

// Parse reads a collection payload.
func Parse(raw []byte) (Collection, error) {
	var c Collection
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, invalid("payload", "not a collection object")
	}
	return c, nil
}

// Validate checks what the JSON Schema leaves to the server: unique items, an objective on every knowledge card, completion
// rewards of the allowed types with a registered applier.
func (c Collection) Validate(hasApplier func(string) bool) error {
	if c.CollectionID == uuid.Nil {
		return invalid("collection_id", "required")
	}
	if !slices.Contains(Types, c.Type) {
		return invalid("collection_type", "not a collection type")
	}
	if !slices.Contains(Visibilities, c.Visibility) {
		return invalid("visibility", "always or hidden_until_first")
	}
	if len(c.Items) == 0 {
		return invalid("items", "at least one item")
	}
	seen := map[string]bool{}
	objectives := map[uuid.UUID]bool{}
	for i, it := range c.Items {
		if seen[it.ItemID] {
			return invalid(fmt.Sprintf("items/%d/item_id", i), "listed twice")
		}
		seen[it.ItemID] = true
		if it.SourceHint.VI == "" || it.SourceHint.EN == "" {
			return invalid(fmt.Sprintf("items/%d/source_hint", i), "vi and en are required")
		}
		if c.Type == "knowledge_card" {
			if it.ObjectiveID == nil {
				return invalid(fmt.Sprintf("items/%d/objective_id", i), "a knowledge card names its learning objective")
			}
			if objectives[*it.ObjectiveID] {
				return invalid(fmt.Sprintf("items/%d/objective_id", i), "one card per learning objective")
			}
			objectives[*it.ObjectiveID] = true
		}
	}
	for i, r := range c.Rewards {
		t, _ := r["type"].(string)
		if !slices.Contains(RewardTypes, t) {
			return invalid(fmt.Sprintf("completion_rewards/%d/type", i), "not a completion reward type")
		}
		if hasApplier == nil || !hasApplier(t) {
			return invalid(fmt.Sprintf("completion_rewards/%d/type", i), "no grant applier for %q", t)
		}
	}
	return nil
}

// ItemIDs are the item codes of the collection in order.
func (c Collection) ItemIDs() []string {
	out := make([]string, 0, len(c.Items))
	for _, it := range c.Items {
		out = append(out, it.ItemID)
	}
	return out
}

// Progress is the progress of a student on a collection.
type Progress struct {
	Owned     []string
	Completed bool
}

// Add records item in the progress of a collection of total items: added is false when the item was already counted;
// completed is true only on the call that makes the collection complete (F075-AC-18: completion rewards once).
func (p *Progress) Add(item string, total int) (added, completed bool) {
	if slices.Contains(p.Owned, item) {
		return false, false
	}
	p.Owned = append(p.Owned, item)
	if !p.Completed && total > 0 && len(p.Owned) >= total {
		p.Completed = true
		return true, true
	}
	return true, false
}

// CardLevels are the mastery levels that earn a knowledge card (INVENTORY_ECONOMY §6.1: proficient or above).
var CardLevels = []string{"proficient", "mastered"}

// EarnsCard says whether a mastery level earns the knowledge card of its objective (F075-AC-19). A lower level never
// takes a card back.
func EarnsCard(level string) bool { return slices.Contains(CardLevels, level) }
