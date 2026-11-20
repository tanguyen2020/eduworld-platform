// Package profile is the domain of the game profile of a student (F070, CHARACTER_SYSTEM §5): the aggregate
// GameProfile, the AvatarConfig and its rules (catalog palette, ownership, equipment slots), the nickname policy and
// the nickname generator. Pure: no I/O, clock and randomness are injected.
package profile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
)

// SchemaVersion is the only AvatarConfig.schema_version of this release.
const SchemaVersion = 1

// MaxAvatarBytes is the size limit of a stored avatar_config (data-model: CHECK octet_length <= 16384, F070-AC-17).
const MaxAvatarBytes = 16384

// MaxBadges is the size of the badge showcase (CHARACTER_SYSTEM §5.1).
const MaxBadges = 3

// Accessory slots (INVENTORY_ECONOMY §2.1); at most one item per slot.
var AccessorySlots = []string{"head", "face", "back", "hand"}

// Equipment slots of the items (item_definition_versions.equip_slot).
const (
	SlotHair   = "hair"
	SlotOutfit = "outfit"
	SlotEmote  = "emote"
	SlotPet    = "pet"
	SlotTitle  = "title"
	SlotFrame  = "frame"
	SlotBadge  = "badge"
)

var codeRe = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)

// ValidCode reports a catalog code (CatalogCode of the contract: ^[a-z0-9_]+$, 1..64).
func ValidCode(s string) bool { return codeRe.MatchString(s) }

// Hair is the hair item and its colour.
type Hair struct {
	ItemID string `json:"item_id"`
	Color  string `json:"color"`
}

// ItemRef references one item by code.
type ItemRef struct {
	ItemID string `json:"item_id"`
}

// Accessory is an accessory item in one accessory slot.
type Accessory struct {
	Slot   string `json:"slot"`
	ItemID string `json:"item_id"`
}

// AvatarConfig is the appearance of a player (CHARACTER_SYSTEM §5.1, the owning definition). Items are referenced by
// code (varchar 64), never by UUID. EmoteSlots nil means "absent" (the server fills the defaults).
type AvatarConfig struct {
	SchemaVersion int         `json:"schema_version"`
	BodyType      string      `json:"body_type"`
	SkinTone      string      `json:"skin_tone"`
	Hair          Hair        `json:"hair"`
	Outfit        ItemRef     `json:"outfit"`
	Accessories   []Accessory `json:"accessories,omitempty"`
	EmoteSlots    []string    `json:"emote_slots"`
	Pet           string      `json:"pet,omitempty"`
	Title         string      `json:"title,omitempty"`
	Frame         string      `json:"frame,omitempty"`
	BadgeShowcase []string    `json:"badge_showcase,omitempty"`
}

// Equip is one item placed in one slot.
type Equip struct {
	Slot   string // expected equip_slot of the item
	ItemID string
	Field  string // JSON pointer-ish name of the field, for errors
}

// Equipped lists every item of c with the slot it is placed in.
func (c AvatarConfig) Equipped() []Equip {
	out := []Equip{{Slot: SlotHair, ItemID: c.Hair.ItemID, Field: "hair.item_id"}, {Slot: SlotOutfit, ItemID: c.Outfit.ItemID, Field: "outfit.item_id"}}
	for i, a := range c.Accessories {
		out = append(out, Equip{Slot: a.Slot, ItemID: a.ItemID, Field: fmt.Sprintf("accessories[%d].item_id", i)})
	}
	for i, e := range c.EmoteSlots {
		out = append(out, Equip{Slot: SlotEmote, ItemID: e, Field: fmt.Sprintf("emote_slots[%d]", i)})
	}
	if c.Pet != "" {
		out = append(out, Equip{Slot: SlotPet, ItemID: c.Pet, Field: "pet"})
	}
	if c.Title != "" {
		out = append(out, Equip{Slot: SlotTitle, ItemID: c.Title, Field: "title"})
	}
	if c.Frame != "" {
		out = append(out, Equip{Slot: SlotFrame, ItemID: c.Frame, Field: "frame"})
	}
	for i, b := range c.BadgeShowcase {
		out = append(out, Equip{Slot: SlotBadge, ItemID: b, Field: fmt.Sprintf("badge_showcase[%d]", i)})
	}
	return out
}

// ItemIDs returns the distinct item codes of c.
func (c AvatarConfig) ItemIDs() []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range c.Equipped() {
		if !seen[e.ItemID] {
			seen[e.ItemID] = true
			out = append(out, e.ItemID)
		}
	}
	return out
}

// Normalized returns c in canonical form: accessories ordered by slot (head, face, back, hand), emote_slots never
// nil (an absent list is filled by defaults). The order of emotes is kept (it is the wheel position).
func (c AvatarConfig) Normalized(defaultEmotes []string) AvatarConfig {
	out := c
	out.Accessories = append([]Accessory(nil), c.Accessories...)
	rank := map[string]int{}
	for i, s := range AccessorySlots {
		rank[s] = i
	}
	sort.SliceStable(out.Accessories, func(i, j int) bool { return rank[out.Accessories[i].Slot] < rank[out.Accessories[j].Slot] })
	if c.EmoteSlots == nil {
		out.EmoteSlots = append([]string{}, defaultEmotes...)
	} else {
		out.EmoteSlots = append([]string{}, c.EmoteSlots...)
	}
	if len(c.BadgeShowcase) > 0 {
		out.BadgeShowcase = append([]string(nil), c.BadgeShowcase...)
	}
	return out
}

// Canonical is the canonical JSON of c (fixed field order of the struct): the stored value and the input of Hash.
func (c AvatarConfig) Canonical() ([]byte, error) { return json.Marshal(c) }

// Hash is avatar_config_hash: SHA-256 hex of the canonical JSON (CHARACTER_SYSTEM §8: clients cache by hash).
func (c AvatarConfig) Hash() (string, error) {
	b, err := c.Canonical()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
