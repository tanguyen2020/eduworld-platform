class_name DataRead
extends RefCounted
## Tolerant readers for JSON bodies of the game API (F069 models; GAME_NETWORKING §9 "JSON → model typed"): a wrong
## type or a missing field gives the fallback, never a crash; unknown fields are ignored. A display text is kept as
## a `{vi, en}` dictionary (`LocalizedName` / `LocalizedText`, ADR-0056); a plain string sent by an older contract
## version (already in the request language) is kept under the key `_` so `L10n.pick()` still shows it.

const PLAIN_TEXT_KEY: String = "_"


static func str_of(d: Dictionary, key: String, fallback: String = "") -> String:
	var v: Variant = d.get(key)
	return v as String if v is String else fallback


static func int_of(d: Dictionary, key: String, fallback: int = 0) -> int:
	var v: Variant = d.get(key)
	if v is int:
		return v as int
	if v is float:
		return int(v as float)
	return fallback


static func num_of(d: Dictionary, key: String, fallback: float = 0.0) -> float:
	var v: Variant = d.get(key)
	if v is int or v is float:
		return float(v)
	return fallback


static func bool_of(d: Dictionary, key: String, fallback: bool = false) -> bool:
	var v: Variant = d.get(key)
	return v as bool if v is bool else fallback


## `null` and absent fields both read as -1 (e.g. `xp_to_next = null` at the maximum level).
static func nullable_int(d: Dictionary, key: String) -> int:
	var v: Variant = d.get(key)
	if v is int or v is float:
		return maxi(0, int(v))
	return -1


static func dict_of(d: Dictionary, key: String) -> Dictionary:
	var v: Variant = d.get(key)
	return v as Dictionary if v is Dictionary else {}


static func arr_of(d: Dictionary, key: String) -> Array:
	var v: Variant = d.get(key)
	return v as Array if v is Array else []


## A display text (`LocalizedName`, `LocalizedText` or a plain string) as a dictionary for `L10n.pick()`.
static func text_of(d: Dictionary, key: String) -> Dictionary:
	var v: Variant = d.get(key)
	if v is Dictionary:
		var out: Dictionary = {}
		for k: Variant in (v as Dictionary).keys():
			if (v as Dictionary)[k] is String:
				out[String(k)] = (v as Dictionary)[k]
		return out
	if v is String and not (v as String).is_empty():
		return {PLAIN_TEXT_KEY: v}
	return {}


static func strings_of(d: Dictionary, key: String, max_items: int = 50) -> PackedStringArray:
	var out: PackedStringArray = []
	for v: Variant in arr_of(d, key):
		if v is String and out.size() < max_items:
			out.append(v as String)
	return out
