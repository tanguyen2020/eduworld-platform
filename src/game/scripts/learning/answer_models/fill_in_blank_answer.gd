class_name FillInBlankAnswer
extends AnswerModel
## `fill_in_blank` → `{blanks: {blank_id: text}}`. Each value is clipped to the blank's `max_length` (render payload)
## and never longer than the contract limit (F063-AC-12).

## `FillInBlankResponse.blanks` value maxLength.
const CONTRACT_MAX_LENGTH: int = 200

## `[{id, input, max_length, choices}]` in display order.
var blanks: Array[Dictionary] = []
var values: Dictionary = {}


func _on_setup() -> void:
	blanks = []
	values = {}
	var re: RegEx = RegEx.create_from_string(AnswerModel.ITEM_ID_PATTERN)
	var raw: Variant = payload.get("blanks", [])
	if not raw is Array:
		return
	for entry: Variant in (raw as Array):
		if entry is Dictionary and (entry as Dictionary).get("id") is String \
				and re.search((entry as Dictionary)["id"] as String) != null:
			blanks.append(entry as Dictionary)


func blank_ids() -> PackedStringArray:
	var out: PackedStringArray = []
	for b: Dictionary in blanks:
		out.append(String(b["id"]))
	return out


## Characters allowed in a blank: its `max_length`, capped by the contract.
func max_length(blank_id: String) -> int:
	for b: Dictionary in blanks:
		if String(b["id"]) == blank_id:
			var limit: Variant = b.get("max_length")
			if limit is int or limit is float:
				return clampi(int(limit), 1, CONTRACT_MAX_LENGTH)
	return CONTRACT_MAX_LENGTH


## Input kind of a blank (`text`, `numeric`, `word_bank`, `dropdown`).
func input_of(blank_id: String) -> String:
	for b: Dictionary in blanks:
		if String(b["id"]) == blank_id:
			return String(b.get("input", "text"))
	return "text"


## Choices of a `dropdown` blank, or the payload `word_bank` for a `word_bank` blank.
func choices_of(blank_id: String) -> PackedStringArray:
	var out: PackedStringArray = []
	var source: Variant = payload.get("word_bank", [])
	for b: Dictionary in blanks:
		if String(b["id"]) == blank_id and b.get("choices") is Array:
			source = b["choices"]
	if source is Array:
		for c: Variant in (source as Array):
			if c is String:
				out.append(c as String)
	return out


func set_blank(blank_id: String, text: String) -> void:
	if not blank_id in blank_ids():
		return
	var clipped: String = AnswerModel.clip(text, max_length(blank_id))
	if String(values.get(blank_id, "")) == clipped:
		return
	values[blank_id] = clipped
	_notify()


func value_of(blank_id: String) -> String:
	return String(values.get(blank_id, ""))


func is_complete() -> bool:
	if blanks.is_empty():
		return false
	for id: String in blank_ids():
		if value_of(id).strip_edges().is_empty():
			return false
	return true


func to_response() -> Dictionary:
	if not is_complete():
		return {}
	var out: Dictionary = {}
	for id: String in blank_ids():
		out[id] = value_of(id).strip_edges()
	return {"blanks": out}


func restore(response: Dictionary) -> void:
	var saved: Variant = response.get("blanks")
	if not saved is Dictionary:
		return
	for id: Variant in (saved as Dictionary).keys():
		if (saved as Dictionary)[id] is String:
			set_blank(String(id), (saved as Dictionary)[id] as String)
