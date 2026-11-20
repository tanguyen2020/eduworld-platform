// Package catalog is the domain of the character catalog (F070, CHARACTER_SYSTEM §5.0, ADR-0043): platform content
// with versions — presets, body types, skin tones, hair colours, free default items and emotes, approved nickname
// words per language. Payload shape: specs/schemas/character-catalog.schema.json. Pure.
package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/profile"
)

// Code is the code of the platform catalog (game.character_catalogs.code).
const Code = "default"

// Name is a bilingual display name (vi and en required, NFR-I18N-002).
type Name struct {
	VI string `json:"vi"`
	EN string `json:"en"`
}

// NamedCode is a palette entry.
type NamedCode struct {
	Code string `json:"code"`
	Name Name   `json:"name"`
}

// Preset is a ready-made character of the creation screen.
type Preset struct {
	PresetCode       string               `json:"preset_code"`
	Name             Name                 `json:"name"`
	AvatarConfig     profile.AvatarConfig `json:"avatar_config"`
	ThumbnailAssetID *uuid.UUID           `json:"thumbnail_asset_id,omitempty"`
}

// Payload is one version of the catalog (character-catalog.schema.json).
type Payload struct {
	SchemaVersion   int                      `json:"schema_version"`
	Presets         []Preset                 `json:"presets"`
	BodyTypes       []NamedCode              `json:"body_types"`
	SkinTones       []NamedCode              `json:"skin_tones"`
	HairColors      []NamedCode              `json:"hair_colors"`
	DefaultItemIDs  []string                 `json:"default_item_ids"`
	DefaultEmoteIDs []string                 `json:"default_emote_ids"`
	NicknameParts   map[string]profile.Words `json:"nickname_parts"`
}

// Catalog is the current published version.
type Catalog struct {
	VersionID uuid.UUID
	Version   int
	Payload
}

func has(list []NamedCode, code string) bool {
	return slices.ContainsFunc(list, func(n NamedCode) bool { return n.Code == code })
}

// HasBodyType implements profile.Palette.
func (c Catalog) HasBodyType(code string) bool { return has(c.BodyTypes, code) }

// HasSkinTone implements profile.Palette.
func (c Catalog) HasSkinTone(code string) bool { return has(c.SkinTones, code) }

// HasHairColor implements profile.Palette.
func (c Catalog) HasHairColor(code string) bool { return has(c.HairColors, code) }

// IsDefault reports an item that is free for every student (default items and default emotes).
func (c Catalog) IsDefault(itemID string) bool {
	return slices.Contains(c.DefaultItemIDs, itemID) || slices.Contains(c.DefaultEmoteIDs, itemID)
}

// Words returns the nickname words of lang ("vi", "en"), falling back to vi.
func (c Catalog) Words(lang string) profile.Words {
	if w, ok := c.NicknameParts[lang]; ok {
		return w
	}
	return c.NicknameParts["vi"]
}

// ETag is the entity tag of getCharacterCatalog: the version id (a published version never changes).
func (c Catalog) ETag() string { return c.VersionID.String() }

// Parse decodes a payload and checks what the server relies on (CheckPayload).
func Parse(raw []byte) (Payload, error) {
	var p Payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return Payload{}, fmt.Errorf("character catalog: %w", err)
	}
	return p, CheckPayload(p)
}

// Sha256 is the hex SHA-256 of the canonical JSON of p (detects an unchanged republish).
func Sha256(p Payload) (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func checkNames(kind string, list []NamedCode) error {
	if len(list) == 0 {
		return fmt.Errorf("character catalog: %s is empty", kind)
	}
	seen := map[string]bool{}
	for _, n := range list {
		if !profile.ValidCode(n.Code) || n.Name.VI == "" || n.Name.EN == "" || seen[n.Code] {
			return fmt.Errorf("character catalog: %s %q needs a unique code and names vi and en", kind, n.Code)
		}
		seen[n.Code] = true
	}
	return nil
}

// CheckPayload checks the rules of a publishable catalog: schema version, bilingual palette, at least one default
// emote, nickname words for vi and en, and presets that only use the palette and the free items.
func CheckPayload(p Payload) error {
	if p.SchemaVersion != 1 {
		return errors.New("character catalog: schema_version must be 1")
	}
	for kind, list := range map[string][]NamedCode{"body_types": p.BodyTypes, "skin_tones": p.SkinTones, "hair_colors": p.HairColors} {
		if err := checkNames(kind, list); err != nil {
			return err
		}
	}
	if len(p.DefaultEmoteIDs) == 0 {
		return errors.New("character catalog: default_emote_ids is empty")
	}
	for _, lang := range []string{"vi", "en"} {
		w, ok := p.NicknameParts[lang]
		if !ok || len(w.Adjectives) == 0 || len(w.Nouns) == 0 {
			return fmt.Errorf("character catalog: nickname_parts.%s needs adjectives and nouns", lang)
		}
	}
	if len(p.Presets) == 0 {
		return errors.New("character catalog: presets is empty")
	}
	c := Catalog{Payload: p}
	for _, pr := range p.Presets {
		if !profile.ValidCode(pr.PresetCode) || pr.Name.VI == "" || pr.Name.EN == "" {
			return fmt.Errorf("character catalog: preset %q needs a code and names vi and en", pr.PresetCode)
		}
		if err := pr.AvatarConfig.CheckShape(profile.Rules{}); err != nil {
			return fmt.Errorf("character catalog: preset %s: %w", pr.PresetCode, err)
		}
		if err := pr.AvatarConfig.CheckPalette(c); err != nil {
			return fmt.Errorf("character catalog: preset %s: %w", pr.PresetCode, err)
		}
		for _, id := range pr.AvatarConfig.ItemIDs() {
			if !c.IsDefault(id) {
				return fmt.Errorf("character catalog: preset %s uses %s, which is not a default item", pr.PresetCode, id)
			}
		}
	}
	return nil
}

// ItemDefinition is one item of the item seed (game.item_definitions + version).
type ItemDefinition struct {
	ItemID    string   `json:"item_id"`
	Kind      string   `json:"kind"`
	EquipSlot string   `json:"equip_slot"`
	Rarity    string   `json:"rarity"`
	Name      Name     `json:"name"`
	Sources   []string `json:"sources"`
}
