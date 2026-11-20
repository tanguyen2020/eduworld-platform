package profile

import (
	"fmt"
	"slices"
)

// InvalidError is a value outside the catalog or a malformed config: 422 VALIDATION_FAILED (F070-AC-04).
type InvalidError struct {
	Field  string
	Reason string
}

func (e *InvalidError) Error() string { return fmt.Sprintf("avatar_config.%s: %s", e.Field, e.Reason) }

// NotOwnedError is an item the student neither owns nor gets for free: 422 ITEM_NOT_OWNED (F070-AC-13).
type NotOwnedError struct {
	Field  string
	ItemID string
}

func (e *NotOwnedError) Error() string {
	return fmt.Sprintf("avatar_config.%s: item %s not owned", e.Field, e.ItemID)
}

// SlotError is an item in the wrong slot, a slot with two items or too many emotes: 422 EQUIP_SLOT_INVALID
// (F070-AC-14).
type SlotError struct {
	Field  string
	Reason string
}

func (e *SlotError) Error() string { return fmt.Sprintf("avatar_config.%s: %s", e.Field, e.Reason) }

// Palette is the free appearance of the current catalog (body types, skin tones, hair colours, CHARACTER_SYSTEM §5.0).
type Palette interface {
	HasBodyType(code string) bool
	HasSkinTone(code string) bool
	HasHairColor(code string) bool
}

// Item is what the rules need of an item definition.
type Item struct {
	Kind      string
	EquipSlot string
}

// Rules are the limits of the equipment (CONFIG_DEFAULTS emote_wheel_slots).
type Rules struct {
	EmoteWheelSlots int
}

// CheckShape checks the parts of c that need no catalog: schema version, codes, accessory slots, one item per
// slot, the size of the emote wheel and of the badge showcase (InvalidError or SlotError).
func (c AvatarConfig) CheckShape(r Rules) error {
	if c.SchemaVersion != SchemaVersion {
		return &InvalidError{Field: "schema_version", Reason: "unsupported"}
	}
	for _, f := range []struct{ name, v string }{{"body_type", c.BodyType}, {"skin_tone", c.SkinTone}, {"hair.color", c.Hair.Color}} {
		if !ValidCode(f.v) {
			return &InvalidError{Field: f.name, Reason: "not a catalog code"}
		}
	}
	for _, e := range c.Equipped() {
		if !ValidCode(e.ItemID) {
			return &InvalidError{Field: e.Field, Reason: "not a catalog code"}
		}
	}
	used := map[string]bool{}
	for i, a := range c.Accessories {
		if !slices.Contains(AccessorySlots, a.Slot) {
			return &SlotError{Field: fmt.Sprintf("accessories[%d].slot", i), Reason: "unknown accessory slot"}
		}
		if used[a.Slot] {
			return &SlotError{Field: fmt.Sprintf("accessories[%d].slot", i), Reason: "slot already used"}
		}
		used[a.Slot] = true
	}
	if r.EmoteWheelSlots > 0 && len(c.EmoteSlots) > r.EmoteWheelSlots {
		return &SlotError{Field: "emote_slots", Reason: "more emotes than emote_wheel_slots"}
	}
	if dup := firstDuplicate(c.EmoteSlots); dup >= 0 {
		return &SlotError{Field: fmt.Sprintf("emote_slots[%d]", dup), Reason: "emote already on the wheel"}
	}
	if len(c.BadgeShowcase) > MaxBadges {
		return &SlotError{Field: "badge_showcase", Reason: "more than 3 badges"}
	}
	if dup := firstDuplicate(c.BadgeShowcase); dup >= 0 {
		return &SlotError{Field: fmt.Sprintf("badge_showcase[%d]", dup), Reason: "badge already shown"}
	}
	return nil
}

func firstDuplicate(list []string) int {
	seen := map[string]bool{}
	for i, v := range list {
		if seen[v] {
			return i
		}
		seen[v] = true
	}
	return -1
}

// CheckPalette checks body type, skin tone and hair colour against the free palette of the catalog (F070-AC-04).
func (c AvatarConfig) CheckPalette(p Palette) error {
	switch {
	case !p.HasBodyType(c.BodyType):
		return &InvalidError{Field: "body_type", Reason: "not in the catalog"}
	case !p.HasSkinTone(c.SkinTone):
		return &InvalidError{Field: "skin_tone", Reason: "not in the catalog"}
	case !p.HasHairColor(c.Hair.Color):
		return &InvalidError{Field: "hair.color", Reason: "not in the catalog"}
	}
	return nil
}

// CheckEquipment checks every item of c: owned (owned reports a free default item or an inventory item) and known
// (items holds the current definitions), then placed in its equip_slot (F070-AC-13, AC-14). Ownership is checked
// before the slot so that an unknown item is never described.
func (c AvatarConfig) CheckEquipment(items map[string]Item, owned func(itemID string) bool) error {
	eq := c.Equipped()
	for _, e := range eq {
		if _, known := items[e.ItemID]; !known || !owned(e.ItemID) {
			return &NotOwnedError{Field: e.Field, ItemID: e.ItemID}
		}
	}
	for _, e := range eq {
		if items[e.ItemID].EquipSlot != e.Slot {
			return &SlotError{Field: e.Field, Reason: "item does not fit this slot"}
		}
	}
	return nil
}

// Validate runs CheckShape, CheckPalette and CheckEquipment in that order.
func (c AvatarConfig) Validate(r Rules, p Palette, items map[string]Item, owned func(itemID string) bool) error {
	if err := c.CheckShape(r); err != nil {
		return err
	}
	if err := c.CheckPalette(p); err != nil {
		return err
	}
	return c.CheckEquipment(items, owned)
}
