class_name ClueCardModel
extends RefCounted
## A clue card of an `interdependent` step as the server rendered it (OpenAPI `QuestStepClue` of `getQuestStepClues` /
## `shareQuestStepClue`; F110 data-model §1): the card of the student's own role, or one a member shared. `content` is
## the learning material render (display blocks, no answer — ADR-0016). Kept in RAM only, never written to disk, never
## combined or guessed by the client (F110-AC-15).

var role: String = ""
var title: Dictionary = {}
var icon_key: String = ""
var shared: bool = false
var shared_by_nickname: String = ""
var content: Dictionary = {}


static func from_dict(data: Variant) -> ClueCardModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var role_code: String = DataRead.str_of(d, "role")
	var t: Dictionary = DataRead.text_of(d, "title")
	if role_code.is_empty() or role_code.length() > 32 or t.is_empty():
		return null
	var c: ClueCardModel = ClueCardModel.new()
	c.role = role_code
	c.title = t
	c.icon_key = DataRead.str_of(d, "icon_key").left(64)
	c.shared = DataRead.bool_of(d, "shared")
	c.shared_by_nickname = DataRead.str_of(d, "shared_by_nickname").left(32)
	c.content = DataRead.dict_of(d, "content").duplicate(true)
	return c


func display_title(locale: String = "") -> String:
	return L10n.pick(title, locale)


## Plain words of the render blocks (`text`/`body`/`value` of the blocks, a `LocalizedText`), for the card body.
func body_text(locale: String = "") -> String:
	var parts: PackedStringArray = []
	_collect(content, parts, locale)
	return "\n".join(parts)


static func _collect(v: Variant, parts: PackedStringArray, locale: String) -> void:
	if parts.size() >= 12:
		return
	if v is Dictionary:
		var d: Dictionary = v as Dictionary
		if d.has("vi") or d.has("en"):
			var s: String = L10n.pick(DataRead.text_of({"t": d}, "t"), locale)
			if not s.is_empty():
				parts.append(s.left(400))
			return
		for k: String in ["text", "body", "value", "caption"]:
			if d.get(k) is String and not String(d[k]).is_empty():
				parts.append(String(d[k]).left(400))
			elif d.get(k) is Dictionary:
				_collect(d[k], parts, locale)
		for k: String in ["blocks", "items"]:
			if d.get(k) is Array:
				_collect(d[k], parts, locale)
	elif v is Array:
		for item: Variant in v as Array:
			_collect(item, parts, locale)
