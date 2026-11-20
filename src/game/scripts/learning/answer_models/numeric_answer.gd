class_name NumericAnswer
extends AnswerModel
## `numeric` → `{value}`: the string the student typed (decimal separator of the payload, `a/b` only when
## `allow_fraction_input`), at most 64 characters (F063-AC-12). Only digits, the separator, a leading minus and the
## fraction bar are kept; the value is never evaluated on the client (ADR-0016).

## `NumericResponse.value` maxLength.
const CONTRACT_MAX_LENGTH: int = 64

var text: String = ""
var decimal_separator: String = ","
var allow_fraction: bool = false
var unit_label: String = ""


func _on_setup() -> void:
	text = ""
	decimal_separator = "." if payload.get("decimal_separator") == "." else ","
	allow_fraction = payload.get("allow_fraction_input") == true
	unit_label = String(payload.get("unit_label", "")) if payload.get("unit_label") is String else ""


## Keys of the custom number pad (STUDENT_GAME_UX_LEARNING §4.3).
func pad_keys() -> PackedStringArray:
	var keys: PackedStringArray = ["7", "8", "9", "4", "5", "6", "1", "2", "3", "-", "0", decimal_separator]
	if allow_fraction:
		keys.append("/")
	return keys


func set_text(value: String) -> void:
	var filtered: String = ""
	for c: String in value:
		if c.is_valid_int() or c == decimal_separator or (allow_fraction and c == "/") \
				or (c == "-" and (filtered.is_empty() or (allow_fraction and filtered.ends_with("/")))):
			filtered += c
	filtered = AnswerModel.clip(filtered, CONTRACT_MAX_LENGTH)
	if filtered != text:
		text = filtered
		_notify()


func press(key: String) -> void:
	set_text(text + key)


func backspace() -> void:
	set_text(text.left(text.length() - 1))


func is_complete() -> bool:
	for c: String in text:
		if c.is_valid_int():
			return true
	return false


func to_response() -> Dictionary:
	return {"value": text} if is_complete() else {}


func restore(response: Dictionary) -> void:
	if response.get("value") is String:
		set_text(response["value"] as String)
