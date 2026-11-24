class_name ItemVisual
extends Resource
## Display-only description of a cosmetic code (CHARACTER_SYSTEM §5.2): `res://resources/items/<item_id>.tres` for
## items (hair, outfit, accessories, emotes) and `res://resources/avatar/<code>.tres` for body types. Skin tones and hair
## colours are design tokens keyed by the catalog code (`Art3dTokens.AVATAR_SKIN_TONES` / `AVATAR_HAIR_COLORS`, from
## design/game-tokens/art3d.json; F070 T041), read with `appearance_color()`. It says how to draw the code — mesh/scene, material, tint, attach offset, icon,
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
## Colour of a skin tone / hair colour code the client has no token for (catalog newer than this client).
const UNKNOWN_APPEARANCE_COLOR: Color = Color.WHITE
## Same pattern as the contract `CatalogCode`: server data never builds an arbitrary resource path.
const CODE_PATTERN: String = "^[a-z0-9_]{1,64}$"

## Visuals already loaded on this client (display data only): keeps the baked F067 meshes in memory so a preview
## redraw (S02, Wardrobe) does not load and decompress them again.
static var _loaded: Dictionary = {}

@export var code: String = ""
## `hair`, `outfit`, `head`, `face`, `back`, `hand`, `emote`, `body_type`, `skin_tone`, `hair_color`.
@export var slot: String = ""
## Final asset: a scene attached to the slot anchor (preferred) or a mesh.
@export var scene: PackedScene = null
@export var mesh: Mesh = null
@export var material_override: Material = null
## Albedo of the generated material when no `tint_token` is set (legacy). Skin tone / hair colour come from the tokens.
@export var tint: Color = Color(1, 1, 1, 1)
## Art palette colour of the item (`Art3dTokens.ART_PALETTE`, F067): the tint drawn on its ALBEDO faces (R32).
@export var tint_token: String = ""
## Position relative to the slot anchor.
@export var offset: Vector3 = Vector3.ZERO
## Body type proportions (body_type codes only).
@export var body_scale: Vector3 = Vector3.ONE
@export var icon: Texture2D = null
## One-shot clip of an emote (emote codes only); null → AnimDriver plays its placeholder clip.
@export var emote_clip: Animation = null
## True while the visual is a primitive stand-in for the OQ-16 art (AGENTS R17).
@export var placeholder: bool = false


## Colour drawn on the item: its art palette token, else `tint`.
func display_tint() -> Color:
	if Art3dTokens.ART_PALETTE_COLORS.has(tint_token):
		return Art3dTokens.ART_PALETTE_COLORS[tint_token] as Color
	return tint


static func is_valid_code(value: String) -> bool:
	return RegEx.create_from_string(CODE_PATTERN).search(value) != null


## Visual of an item code; null when the code is invalid or its resource is not available on this client.
static func load_item(item_id: String) -> ItemVisual:
	return _load(ITEMS_DIR, item_id)


## Colour of a skin tone or hair colour code from the avatar palette tokens (T041); `UNKNOWN_APPEARANCE_COLOR` when
## this client has no token for the code.
static func appearance_color(appearance_code: String) -> Color:
	if Art3dTokens.AVATAR_SKIN_TONES.has(appearance_code):
		return Art3dTokens.AVATAR_SKIN_TONES[appearance_code] as Color
	if Art3dTokens.AVATAR_HAIR_COLORS.has(appearance_code):
		return Art3dTokens.AVATAR_HAIR_COLORS[appearance_code] as Color
	return UNKNOWN_APPEARANCE_COLOR


## True when the avatar palette tokens have a colour for `appearance_code`.
static func has_appearance_color(appearance_code: String) -> bool:
	return Art3dTokens.AVATAR_SKIN_TONES.has(appearance_code) or Art3dTokens.AVATAR_HAIR_COLORS.has(appearance_code)


## Visual of an appearance code (body type); null when unavailable.
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
	if _loaded.has(path):
		return _loaded[path] as ItemVisual
	if not ResourceLoader.exists(path):
		return null
	var visual: ItemVisual = ResourceLoader.load(path) as ItemVisual
	if visual != null:
		_loaded[path] = visual
	return visual
