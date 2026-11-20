class_name CharacterCatalogModel
extends RefCounted
## The published `CharacterCatalog` (specs/openapi, CHARACTER_SYSTEM §5.0; F070-AC-02): creation presets, body
## types, skin tones, hair colours (code + name vi/en), the free default items and emotes with their names vi/en
## (`items`, T040). Platform content with a version (ADR-0043) — the client never hardcodes these lists (R2); display
## names come from here by locale.

## Label shown next to a name that has no text in the student's language (T040, same marker as the activity player).
const NO_TRANSLATION_KEY: String = "ui.learning.no_translation"

var catalog_version_id: String = ""
var version: int = 0
## Each entry: `{preset_code: String, name: Dictionary, avatar_config: AvatarConfigModel}`.
var presets: Array[Dictionary] = []
## Each entry: `{code: String, name: Dictionary}`.
var body_types: Array[Dictionary] = []
var skin_tones: Array[Dictionary] = []
var hair_colors: Array[Dictionary] = []
var default_item_ids: PackedStringArray = []
var default_emote_ids: PackedStringArray = []
## item_id → `{equip_slot: String, name: Dictionary}` (catalog `items`; empty for a version published before T040).
var items: Dictionary = {}


## Parses a `CharacterCatalog`; null when a required list is missing.
static func from_dict(data: Variant) -> CharacterCatalogModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	for key: String in ["presets", "body_types", "skin_tones", "hair_colors", "default_item_ids", "default_emote_ids"]:
		if not d.get(key) is Array:
			return null
	var c: CharacterCatalogModel = CharacterCatalogModel.new()
	c.catalog_version_id = String(d["catalog_version_id"]) if d.get("catalog_version_id") is String else ""
	c.version = int(d["version"]) if d.get("version") is int or d.get("version") is float else 0
	for entry: Variant in d["presets"] as Array:
		if not entry is Dictionary:
			continue
		var p: Dictionary = entry as Dictionary
		var config: AvatarConfigModel = AvatarConfigModel.from_dict(p.get("avatar_config"))
		if config != null and p.get("preset_code") is String:
			c.presets.append({"preset_code": String(p["preset_code"]), "name": _name(p.get("name")),
					"avatar_config": config})
	c.body_types = _named(d["body_types"])
	c.skin_tones = _named(d["skin_tones"])
	c.hair_colors = _named(d["hair_colors"])
	c.default_item_ids = _strings(d["default_item_ids"])
	c.default_emote_ids = _strings(d["default_emote_ids"])
	if d.get("items") is Array:
		for entry: Variant in d["items"] as Array:
			if entry is Dictionary and (entry as Dictionary).get("item_id") is String:
				var it: Dictionary = entry as Dictionary
				c.items[String(it["item_id"])] = {"equip_slot": String(it.get("equip_slot", "")), "name": _name(it.get("name"))}
	return c


func preset(code: String) -> Dictionary:
	for p: Dictionary in presets:
		if String(p["preset_code"]) == code:
			return p
	return {}


func preset_codes() -> PackedStringArray:
	var out: PackedStringArray = []
	for p: Dictionary in presets:
		out.append(String(p["preset_code"]))
	return out


## Codes of a named list (`body_types`, `skin_tones`, `hair_colors`).
static func codes_of(list: Array[Dictionary]) -> PackedStringArray:
	var out: PackedStringArray = []
	for entry: Dictionary in list:
		out.append(String(entry["code"]))
	return out


## Display name (current locale) of `code` in a named list; empty when unknown.
static func name_in(list: Array[Dictionary], code: String, locale: String = "") -> String:
	for entry: Dictionary in list:
		if String(entry["code"]) == code:
			return L10n.pick(entry["name"] as Dictionary, locale)
	return ""


## Name of an item or emote for `locale` (current locale when empty): `{text: String, translation_missing: bool}`.
## `translation_missing` = the locale's language has no text and another language is shown; `text` is empty when the
## catalog has no entry for the code (the UI keeps its ordinal label "Mẫu n").
func item_name(item_id: String, locale: String = "") -> Dictionary:
	var entry: Dictionary = items.get(item_id, {}) as Dictionary
	var names: Dictionary = entry.get("name", {}) as Dictionary
	var language: String = L10n.pick_language(names, locale)
	if language.is_empty():
		return {"text": "", "translation_missing": false}
	var wanted: String = L10n.data_language(locale if not locale.is_empty() else TranslationServer.get_locale())
	return {"text": String(names[language]), "translation_missing": language != wanted}


## Text for a button/label: the item name, "<name> (<ui.learning.no_translation>)" when only another language has it,
## `fallback` when the catalog has no name.
func item_label(item_id: String, fallback: String, locale: String = "") -> String:
	var n: Dictionary = item_name(item_id, locale)
	if String(n["text"]).is_empty():
		return fallback
	if bool(n["translation_missing"]):
		return "%s (%s)" % [String(n["text"]), TranslationServer.translate(NO_TRANSLATION_KEY)]
	return String(n["text"])


func is_default_item(item_id: String) -> bool:
	return item_id in default_item_ids or item_id in default_emote_ids


static func _named(list: Variant) -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	for entry: Variant in list as Array:
		if entry is Dictionary and (entry as Dictionary).get("code") is String:
			var d: Dictionary = entry as Dictionary
			out.append({"code": String(d["code"]), "name": _name(d.get("name"))})
	return out


static func _name(value: Variant) -> Dictionary:
	var out: Dictionary = {}
	if value is Dictionary:
		for language: String in ["vi", "en"]:
			if (value as Dictionary).get(language) is String:
				out[language] = String((value as Dictionary)[language])
	return out


static func _strings(list: Variant) -> PackedStringArray:
	var out: PackedStringArray = []
	for entry: Variant in list as Array:
		if entry is String:
			out.append(entry as String)
	return out
