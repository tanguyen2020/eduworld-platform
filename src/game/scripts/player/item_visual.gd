class_name ItemVisual
extends Resource
## Display-only description of a cosmetic code (CHARACTER_SYSTEM §5.2): `res://resources/items/<item_id>.tres` for
## items (hair, outfit, accessories, emotes) and `res://resources/avatar/<code>.tres` for appearance codes (body
## type, skin tone, hair colour). It says how to draw the code — mesh/scene, material, tint, attach offset, icon,
## emote clip — never whether the student owns it or where the server lets it be equipped (server-authoritative).
## Item codes, names and ownership come from the backend catalog; a code without a visual resource (cosmetic bundle
## not loaded yet) is drawn with the default item of the same slot (F070-AC-18).

## Wardrobe grouping of the display (the server keeps the authoritative `equip_slot`).
const SLOT_HAIR: String = "hair"
const SLOT_OUTFIT: String = "outfit"
const SLOT_EMOTE: String = "emote"
const SLOT_BODY_TYPE: String = "body_type"
const SLOT_SKIN_TONE: String = "skin_tone"
const SLOT_HAIR_COLOR: String = "hair_color"
const ITEMS_DIR: String = "res://resources/items"
const APPEARANCE_DIR: String = "res://resources/avatar"
## Same pattern as the contract `CatalogCode`: server data never builds an arbitrary resource path.
const CODE_PATTERN: String = "^[a-z0-9_]{1,64}$"

@export var code: String = ""
## `hair`, `outfit`, `head`, `face`, `back`, `hand`, `emote`, `body_type`, `skin_tone`, `hair_color`.
@export var slot: String = ""
## Final asset: a scene attached to the slot anchor (preferred) or a mesh.
@export var scene: PackedScene = null
@export var mesh: Mesh = null
@export var material_override: Material = null
## Albedo of the generated material (skin tone, hair colour, placeholder item colour).
@export var tint: Color = Color(1, 1, 1, 1)
## Position relative to the slot anchor.
@export var offset: Vector3 = Vector3.ZERO
## Body type proportions (body_type codes only).
@export var body_scale: Vector3 = Vector3.ONE
@export var icon: Texture2D = null
## One-shot clip of an emote (emote codes only); null → AnimDriver plays its placeholder clip.
@export var emote_clip: Animation = null
## True while the visual is a primitive stand-in for the OQ-16 art (AGENTS R17).
@export var placeholder: bool = false


static func is_valid_code(value: String) -> bool:
	return RegEx.create_from_string(CODE_PATTERN).search(value) != null


## Visual of an item code; null when the code is invalid or its resource is not available on this client.
static func load_item(item_id: String) -> ItemVisual:
	return _load(ITEMS_DIR, item_id)


## Visual of an appearance code (body type, skin tone, hair colour); null when unavailable.
static func load_appearance(appearance_code: String) -> ItemVisual:
	return _load(APPEARANCE_DIR, appearance_code)


## First code of `candidates` whose visual exists and belongs to `wanted_slot` (the default of that slot).
static func first_for_slot(wanted_slot: String, candidates: PackedStringArray, exclude: String = "") -> String:
	for candidate: String in candidates:
		if candidate == exclude:
			continue
		var visual: ItemVisual = load_item(candidate)
		if visual != null and visual.slot == wanted_slot:
			return candidate
	return ""


static func _load(dir: String, value: String) -> ItemVisual:
	if not is_valid_code(value):
		return null
	var path: String = "%s/%s.tres" % [dir, value]
	if not ResourceLoader.exists(path):
		return null
	return ResourceLoader.load(path) as ItemVisual
