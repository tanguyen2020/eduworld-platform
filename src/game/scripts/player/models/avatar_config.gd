class_name AvatarConfigModel
extends RefCounted
## `AvatarConfig` (CHARACTER_SYSTEM §5.1, specs/openapi `AvatarConfig`, specs/schemas/avatar-config.schema.json):
## the appearance of a student by slot — body type, skin tone, hair (+ colour), outfit, accessories (one per slot
## head/face/back/hand), equipped emotes and the optional pet/title/frame/badge showcase. Item references are codes
## (`CatalogCode`), never UUIDs. The client only edits and displays it; the server checks the catalog, ownership
## (`ITEM_NOT_OWNED`) and slots (`EQUIP_SLOT_INVALID`).

const SCHEMA_VERSION: int = 1
## Accessory slots in the order sent to the server (AvatarConfig `accessories[].slot` enum).
const ACCESSORY_SLOTS: PackedStringArray = ["head", "face", "back", "hand"]
## Contract limits (`emote_slots` maxItems = CONFIG_DEFAULTS `emote_wheel_slots`, `badge_showcase` maxItems).
const MAX_EMOTE_SLOTS: int = 6
const MAX_BADGES: int = 3

var body_type: String = ""
var skin_tone: String = ""
var hair_item_id: String = ""
var hair_color: String = ""
var outfit_item_id: String = ""
## slot (`head`/`face`/`back`/`hand`) → item code.
var accessories: Dictionary = {}
var emote_slots: PackedStringArray = []
var pet: String = ""
var title: String = ""
var frame: String = ""
var badge_showcase: PackedStringArray = []


## Parses an `AvatarConfig` object; null when the required fields are missing (never crashes on bad data).
static func from_dict(data: Variant) -> AvatarConfigModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var hair: Dictionary = d.get("hair") as Dictionary if d.get("hair") is Dictionary else {}
	var outfit: Dictionary = d.get("outfit") as Dictionary if d.get("outfit") is Dictionary else {}
	var c: AvatarConfigModel = AvatarConfigModel.new()
	c.body_type = _text(d.get("body_type"))
	c.skin_tone = _text(d.get("skin_tone"))
	c.hair_item_id = _text(hair.get("item_id"))
	c.hair_color = _text(hair.get("color"))
	c.outfit_item_id = _text(outfit.get("item_id"))
	if c.body_type.is_empty() or c.skin_tone.is_empty() or c.hair_item_id.is_empty() or c.outfit_item_id.is_empty():
		return null
	if d.get("accessories") is Array:
		for entry: Variant in d["accessories"] as Array:
			if entry is Dictionary:
				var slot: String = _text((entry as Dictionary).get("slot"))
				var item: String = _text((entry as Dictionary).get("item_id"))
				if slot in ACCESSORY_SLOTS and not item.is_empty():
					c.accessories[slot] = item
	c.emote_slots = _codes(d.get("emote_slots"), MAX_EMOTE_SLOTS)
	c.pet = _text(d.get("pet"))
	c.title = _text(d.get("title"))
	c.frame = _text(d.get("frame"))
	c.badge_showcase = _codes(d.get("badge_showcase"), MAX_BADGES)
	return c


## Request body for `updateAvatar` / `saveAvatarPreset` (optional fields only when set).
func to_dict() -> Dictionary:
	var out: Dictionary = {
		"schema_version": SCHEMA_VERSION,
		"body_type": body_type,
		"skin_tone": skin_tone,
		"hair": {"item_id": hair_item_id, "color": hair_color},
		"outfit": {"item_id": outfit_item_id},
	}
	var list: Array = []
	for slot: String in ACCESSORY_SLOTS:
		if accessories.has(slot):
			list.append({"slot": slot, "item_id": String(accessories[slot])})
	out["accessories"] = list
	out["emote_slots"] = Array(emote_slots)
	if not pet.is_empty():
		out["pet"] = pet
	if not title.is_empty():
		out["title"] = title
	if not frame.is_empty():
		out["frame"] = frame
	if not badge_showcase.is_empty():
		out["badge_showcase"] = Array(badge_showcase)
	return out


func copy() -> AvatarConfigModel:
	return AvatarConfigModel.from_dict(to_dict())


func equals(other: AvatarConfigModel) -> bool:
	return other != null and JSON.stringify(to_dict()) == JSON.stringify(other.to_dict())


## Every item code referenced by the config (hair, outfit, accessories, emotes, pet, title, frame, badges).
func item_ids() -> PackedStringArray:
	var out: PackedStringArray = [hair_item_id, outfit_item_id]
	for slot: String in ACCESSORY_SLOTS:
		if accessories.has(slot):
			out.append(String(accessories[slot]))
	out.append_array(emote_slots)
	for extra: String in [pet, title, frame]:
		if not extra.is_empty():
			out.append(extra)
	out.append_array(badge_showcase)
	return out


static func _text(value: Variant) -> String:
	return String(value) if value is String else ""


static func _codes(value: Variant, limit: int) -> PackedStringArray:
	var out: PackedStringArray = []
	if value is Array:
		for code: Variant in value as Array:
			if code is String and not (code as String).is_empty() and out.size() < limit:
				out.append(code as String)
	return out
