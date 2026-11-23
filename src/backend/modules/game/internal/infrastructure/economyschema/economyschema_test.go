package economyschema

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// root is the repository root (this package is seven levels below it).
const root = "../../../../../../.."

func norm(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }

// The embedded copies equal the specs (single source, R33).
func TestEmbeddedCopiesMatchSpecs(t *testing.T) {
	for file, copy := range map[string][]byte{"item.schema.json": ItemJSON, "shop-item.schema.json": ShopItemJSON,
		"collection.schema.json": CollectionJSON, "achievement.schema.json": AchievementJSON} {
		spec, err := os.ReadFile(filepath.Join(root, "specs", "schemas", file))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(norm(spec), norm(copy)) {
			t.Fatalf("%s copy differs from specs/schemas: copy it again", file)
		}
	}
}

func example(t *testing.T, file string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "specs", "examples", file))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func pointer(t *testing.T, v interface{ Validate([]byte) error }, m map[string]any) string {
	t.Helper()
	b, _ := json.Marshal(m)
	var se *Error
	if err := v.Validate(b); !errors.As(err, &se) || !errors.Is(err, ErrInvalid) {
		t.Fatalf("accepted: %v", err)
	}
	return se.Pointers[0]
}

// F075-AC-11: the example payloads are valid; real money, a non-cosmetic kind, a tradable item are refused with their
// pointer.
func TestValidate(t *testing.T) {
	for file, v := range map[string]interface{ Validate([]byte) error }{"item.star-hat.json": Item{}, "shop-item.explorer-hat.json": ShopItem{},
		"collection.math-g3-cards.json": Collection{}, "achievement.explorer-math-zone.json": Achievement{}} {
		b, _ := json.Marshal(example(t, file))
		if err := v.Validate(b); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
	}
	shop := example(t, "shop-item.explorer-hat.json")
	shop["price"] = map[string]any{"currency_code": "vnd", "amount": 10000}
	if p := pointer(t, ShopItem{}, shop); p != "/price/currency_code" {
		t.Fatalf("real money pointer %s", p)
	}
	shop = example(t, "shop-item.explorer-hat.json")
	shop["price"] = map[string]any{"currency_code": "xu", "amount": 0}
	if p := pointer(t, ShopItem{}, shop); p != "/price/amount" {
		t.Fatalf("free price pointer %s", p)
	}
	shop = example(t, "shop-item.explorer-hat.json")
	shop["item_kind"] = "xp_booster"
	if p := pointer(t, ShopItem{}, shop); p != "/item_kind" {
		t.Fatalf("kind pointer %s", p)
	}
	item := example(t, "item.star-hat.json")
	item["tradable"] = true
	if p := pointer(t, Item{}, item); p != "/tradable" {
		t.Fatalf("tradable pointer %s", p)
	}
	item = example(t, "item.star-hat.json")
	item["kind"] = "loot_box"
	if p := pointer(t, Item{}, item); p != "/kind" {
		t.Fatalf("loot box pointer %s", p)
	}
	ach := example(t, "achievement.explorer-math-zone.json")
	ach["trigger"] = map[string]any{"event_types": []any{"student.game_logged_in"}}
	if p := pointer(t, Achievement{}, ach); p != "/trigger/event_types/0" {
		t.Fatalf("login trigger pointer %s", p)
	}
}
