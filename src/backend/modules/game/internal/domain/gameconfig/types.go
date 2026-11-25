// Package gameconfig is the domain of the Game Operations Console (F078, ADMIN_PORTAL §3.3, §3.12, CONTENT_MODEL §5–6,
// ADR-0043): the game config types served by module game, the review workflow of a version (draft → in_review →
// approved/rejected → published → archived, reviewer ≠ author), the release rules (publish, schedule, rollback,
// unpublish), the diff of two versions, the reference and translation checks of a submission, the preview token and the
// limits of game media. Pure Go, no I/O.
package gameconfig

import (
	"errors"
	"slices"
)

// ErrUnknownType: the config type is not served by module game (competition, chat) or has no registered type module
// yet (404 NOT_FOUND, F078-AC-03).
var ErrUnknownType = errors.New("gameconfig: config type not served by module game")

// Type describes one config type of the console: its path name, the item type of the release core (F071, F072–F077),
// the payload fields the server owns and what the type supports.
type Type struct {
	Name     string // path config_type (kebab case), e.g. quests
	ItemType string // item_type of the release core and game.content_releases, e.g. quest
	// IDField is the payload field of the item key; UUIDKey: the key is the config_id (server generated) — otherwise
	// the key is an authored code (area_id, chapter_code, item_id) and config_id is a separate UUID.
	IDField string
	UUIDKey bool
	// VersionIDField is the payload field of the version id ("" = the schema has none); Versioned: the schema has the
	// fields version/status/owner_scope the server fills.
	VersionIDField string
	Versioned      bool
	// Has3D: a published version of the type gets a thumbnail and a turntable rendered from its 3D asset (FR-CMS-017).
	Has3D bool
	// Catalog: quests and mini-game configs carry the school catalog metadata and can be assigned to tenants (FR-LMS-007).
	Catalog bool
	// Pointer: the runtime of the type follows the release pointer, so rollback/unpublish are allowed. Events (run
	// schedule, F077) and reward rules (effective timeline, F074) do not: a fix is a new version.
	Rollback, Unpublish bool
	// Assignable is the resource_type of infra.tenant_resource_assignments ("" = not assignable).
	Assignable string
}

// types are the config types with a registered type module (F071–F077, F076 challenges). progression-levels (derived from
// a curve) and currencies have none yet: ErrUnknownType.
var types = []Type{
	{Name: "quests", ItemType: "quest", IDField: "quest_id", UUIDKey: true, VersionIDField: "quest_version_id", Versioned: true, Has3D: true,
		Catalog: true, Rollback: true, Unpublish: true, Assignable: "quest"},
	{Name: "minigame-configs", ItemType: "minigame_config", IDField: "minigame_config_id", UUIDKey: true,
		VersionIDField: "minigame_config_version_id", Versioned: true, Catalog: true, Rollback: true, Unpublish: true, Assignable: "minigame_config"},
	{Name: "npc-dialogues", ItemType: "npc_dialogue", IDField: "dialogue_id", UUIDKey: true, VersionIDField: "dialogue_version_id",
		Versioned: true, Has3D: true, Rollback: true, Unpublish: true},
	{Name: "world-areas", ItemType: "world_area", IDField: "area_id", Has3D: true, Rollback: true, Unpublish: true},
	{Name: "story-chapters", ItemType: "story_chapter", IDField: "chapter_code", Rollback: true, Unpublish: true},
	{Name: "items", ItemType: "item", IDField: "item_id", VersionIDField: "item_version_id", Versioned: true, Has3D: true, Rollback: true,
		Unpublish: true, Assignable: "item"},
	{Name: "shop-items", ItemType: "shop_item", IDField: "shop_item_id", UUIDKey: true, VersionIDField: "shop_item_version_id", Versioned: true,
		Rollback: true, Unpublish: true, Assignable: "shop_item"},
	{Name: "collections", ItemType: "collection", IDField: "collection_id", UUIDKey: true, VersionIDField: "collection_version_id",
		Versioned: true, Rollback: true, Unpublish: true},
	{Name: "achievements", ItemType: "achievement", IDField: "achievement_id", UUIDKey: true, VersionIDField: "achievement_version_id",
		Versioned: true, Rollback: true, Unpublish: true},
	{Name: "reward-rules", ItemType: "reward_rule", IDField: "reward_rule_id", UUIDKey: true, Versioned: true, Unpublish: true},
	{Name: "progression-curves", ItemType: "progression_curve", IDField: "curve_id", UUIDKey: true, VersionIDField: "curve_version_id",
		Versioned: true, Rollback: true, Unpublish: true},
	// F076: challenges on the release core (resource_type challenge of the assignments).
	{Name: "challenges", ItemType: "challenge", IDField: "challenge_id", UUIDKey: true, VersionIDField: "challenge_version_id", Versioned: true,
		Has3D: true, Rollback: true, Unpublish: true, Assignable: "challenge"},
	// F103: bosses on the release core (resource_type boss of the assignments).
	{Name: "bosses", ItemType: "boss", IDField: "boss_id", UUIDKey: true, VersionIDField: "boss_version_id", Versioned: true,
		Rollback: true, Unpublish: true, Assignable: "boss"},
	{Name: "events", ItemType: "event", IDField: "event_id", UUIDKey: true, VersionIDField: "event_version_id", Versioned: true},
	{Name: "event-templates", ItemType: "event_template", IDField: "template_id", UUIDKey: true, VersionIDField: "template_version_id",
		Versioned: true, Rollback: true, Unpublish: true, Assignable: "event_template"},
	{Name: "season-tracks", ItemType: "season_track", IDField: "season_track_id", UUIDKey: true, VersionIDField: "season_track_version_id",
		Versioned: true, Rollback: true, Unpublish: true},
}

// Lookup returns the config type of a path name.
func Lookup(name string) (Type, error) {
	i := slices.IndexFunc(types, func(t Type) bool { return t.Name == name })
	if i < 0 {
		return Type{}, ErrUnknownType
	}
	return types[i], nil
}

// ByItemType returns the config type of a release-core item type.
func ByItemType(itemType string) (Type, bool) {
	i := slices.IndexFunc(types, func(t Type) bool { return t.ItemType == itemType })
	if i < 0 {
		return Type{}, false
	}
	return types[i], true
}

// ByAssignable returns the config type of an assignable resource_type.
func ByAssignable(resourceType string) (Type, bool) {
	i := slices.IndexFunc(types, func(t Type) bool { return t.Assignable != "" && t.Assignable == resourceType })
	if i < 0 {
		return Type{}, false
	}
	return types[i], true
}

// Types returns every served config type (test matrices, metrics).
func Types() []Type { return slices.Clone(types) }

// Normalize fills the payload fields the server owns (item key, version id, version, status, owner_scope) so the JSON
// Schema of the type checks the authored content; client values of these fields are ignored.
func (t Type) Normalize(doc map[string]any, key, versionID string, version int, status Status) {
	if t.UUIDKey {
		doc[t.IDField] = key
	}
	if t.VersionIDField != "" {
		doc[t.VersionIDField] = versionID
	}
	if t.Versioned {
		doc["version"], doc["status"], doc["owner_scope"] = version, string(status), "platform"
	}
}

// Key returns the item key an authored payload declares (a code-keyed type) or "".
func (t Type) Key(doc map[string]any) string {
	if t.UUIDKey {
		return ""
	}
	s, _ := doc[t.IDField].(string)
	return s
}
