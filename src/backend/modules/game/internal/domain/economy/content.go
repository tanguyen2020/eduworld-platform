package economy

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
)

// ErrContentInvalid is economy content that cannot be published (422 VALIDATION_FAILED naming the field, F075-AC-11).
var ErrContentInvalid = errors.New("economy: invalid content")

// FieldError names the invalid field of an item or shop item payload (no value).
type FieldError struct {
	Field, Reason string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Reason }
func (e *FieldError) Unwrap() error { return ErrContentInvalid }

func invalid(field, format string, args ...any) error {
	return &FieldError{Field: field, Reason: fmt.Sprintf(format, args...)}
}

// ItemKinds is the item catalog of INVENTORY_ECONOMY §2.1 (ItemKind); none of them gives a learning or game advantage.
var ItemKinds = []string{"outfit", "hair", "accessory", "emote", "pet", "title", "badge", "frame", "decoration", "knowledge_card",
	"consumable_cosmetic"}

// ShopKinds are the kinds the shop may sell (shop-item.schema.json item_kind): no badge or knowledge card (earned only).
var ShopKinds = []string{"outfit", "hair", "accessory", "emote", "pet", "title", "frame", "decoration", "consumable_cosmetic"}

// kindSlots are the equipment slots a kind may use (INVENTORY_ECONOMY §2.1, CHARACTER_SYSTEM).
var kindSlots = map[string][]string{
	"outfit": {"outfit"}, "hair": {"hair"}, "accessory": {"head", "face", "back", "hand"}, "emote": {"emote"}, "pet": {"pet"},
	"title": {"title"}, "badge": {"badge"}, "frame": {"frame"}, "decoration": {"none"}, "knowledge_card": {"none"},
	"consumable_cosmetic": {"none"},
}

// Name is a bilingual label.
type Name struct {
	VI string `json:"vi"`
	EN string `json:"en"`
}

// Item is one published item definition (item.schema.json).
type Item struct {
	ItemID    string         `json:"item_id"`
	Kind      string         `json:"kind"`
	Rarity    string         `json:"rarity"`
	EquipSlot string         `json:"equip_slot"`
	Stackable bool           `json:"stackable"`
	MaxStack  int            `json:"max_stack"`
	Tradable  bool           `json:"tradable"`
	Name      Name           `json:"name"`
	Sources   []string       `json:"sources"`
	AssetRefs map[string]any `json:"asset_refs"`
}

// ParseItem reads an item payload.
func ParseItem(raw []byte) (Item, error) {
	var it Item
	if err := json.Unmarshal(raw, &it); err != nil {
		return it, invalid("payload", "not an item object")
	}
	if it.MaxStack == 0 {
		it.MaxStack = 1
	}
	return it, nil
}

// Validate checks what the JSON Schema leaves to the server: a kind of the catalog, a slot of its kind, never tradable
// (trading_enabled is off, F075-AC-17), a stack rule that holds.
func (it Item) Validate() error {
	if !slices.Contains(ItemKinds, it.Kind) {
		return invalid("kind", "not in the item catalog")
	}
	if !slices.Contains(Rarities, it.Rarity) {
		return invalid("rarity", "not a rarity")
	}
	if !slices.Contains(kindSlots[it.Kind], it.EquipSlot) {
		return invalid("equip_slot", "not a slot of kind %s", it.Kind)
	}
	if it.Tradable {
		return invalid("tradable", "items are never tradable (trading_enabled = false)")
	}
	if !it.Stackable && it.MaxStack != 1 {
		return invalid("max_stack", "a non-stackable item has max_stack 1")
	}
	if it.Name.VI == "" || it.Name.EN == "" {
		return invalid("name", "vi and en are required")
	}
	return nil
}

// shopDoc is the stored shape of shop-item.schema.json the server reads.
type shopDoc struct {
	ShopItemID    uuid.UUID  `json:"shop_item_id"`
	ItemID        string     `json:"item_id"`
	ItemKind      string     `json:"item_kind"`
	PurchaseLimit int        `json:"purchase_limit"`
	AvailableFrom *time.Time `json:"available_from"`
	AvailableTo   *time.Time `json:"available_to"`
	GradeLevels   []int      `json:"grade_levels"`
	FeaturedOrder int        `json:"featured_order"`
	EventID       *uuid.UUID `json:"event_id"`
	SeasonID      *uuid.UUID `json:"season_id"`
	Price         struct {
		CurrencyCode string `json:"currency_code"`
		Amount       int    `json:"amount"`
	} `json:"price"`
}

// ShopItem is one shop item version payload.
type ShopItem struct {
	Listing
	FeaturedOrder int
}

// ParseShopItem reads a shop item payload.
func ParseShopItem(raw []byte) (ShopItem, error) {
	var d shopDoc
	if err := json.Unmarshal(raw, &d); err != nil {
		return ShopItem{}, invalid("payload", "not a shop item object")
	}
	return ShopItem{Listing: Listing{ShopItemID: d.ShopItemID, ItemID: d.ItemID, ItemKind: d.ItemKind, Price: d.Price.Amount,
		Currency: d.Price.CurrencyCode, PurchaseLimit: d.PurchaseLimit, From: d.AvailableFrom, To: d.AvailableTo, Grades: d.GradeLevels,
		EventID: d.EventID, SeasonID: d.SeasonID}, FeaturedOrder: d.FeaturedOrder}, nil
}

// Validate checks a shop item against the item it sells (F075-AC-11): price in Xu > 0 (never real money), a sellable
// cosmetic kind equal to the kind of the item, a purchase limit, a sale window that closes after it opens, a
// non-stackable item sold at most once per student.
func (s ShopItem) Validate(item *Item) error {
	if s.ShopItemID == uuid.Nil {
		return invalid("shop_item_id", "required")
	}
	if s.Currency != Xu {
		return invalid("price/currency_code", "only Xu (no real money, no second currency)")
	}
	if s.Price < 1 {
		return invalid("price/amount", "must be > 0")
	}
	if !slices.Contains(ShopKinds, s.ItemKind) {
		return invalid("item_kind", "not a cosmetic the shop may sell")
	}
	if item == nil {
		return invalid("item_id", "unknown item")
	}
	if item.Kind != s.ItemKind {
		return invalid("item_kind", "differs from the kind of item %s", item.ItemID)
	}
	if s.PurchaseLimit < 1 {
		return invalid("purchase_limit", "must be ≥ 1")
	}
	if !item.Stackable && s.PurchaseLimit != 1 {
		return invalid("purchase_limit", "a non-stackable item is bought once")
	}
	if s.From != nil && s.To != nil && !s.To.After(*s.From) {
		return invalid("available_to", "must be after available_from")
	}
	return nil
}
