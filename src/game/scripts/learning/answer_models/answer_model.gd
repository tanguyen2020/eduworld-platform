class_name AnswerModel
extends RefCounted
## The answer a student is building for one question item (F063 T018). Pure state, no scoring and no answer key
## (ADR-0016, AGENTS R5/R6): it only turns the student's input into the `QuestionResponse` shape of its type
## (specs/openapi `QuestionResponse` = specs/schemas/question.schema.json#/$defs/response, F063-AC-03) and enforces the
## length limits of the render payload / contract (ADR-0041, F063-AC-12). Question scenes call these methods; the
## models are unit-tested without a scene.

signal changed()

## `ItemId` pattern of option/blank/zone ids (specs/openapi).
const ITEM_ID_PATTERN: String = "^[a-z]_[a-z0-9_]{1,24}$"

var payload: Dictionary = {}


func setup(render_payload: Dictionary) -> void:
	payload = render_payload
	_on_setup()


## True when the answer can be sent ("Kiểm tra" enabled).
func is_complete() -> bool:
	return false


## The `QuestionResponse` of this type; empty when the answer is not complete.
func to_response() -> Dictionary:
	return {}


## Restores a saved response (`Attempt.saved_responses`, resume F063-AC-07); unknown ids are ignored.
func restore(_response: Dictionary) -> void:
	pass


func _on_setup() -> void:
	pass


func _notify() -> void:
	changed.emit()


## Ids of the entries of a payload list (`options`, `items`, `left`…), in display order.
static func ids_of(list: Variant) -> PackedStringArray:
	var out: PackedStringArray = []
	if not list is Array:
		return out
	var re: RegEx = RegEx.create_from_string(ITEM_ID_PATTERN)
	for entry: Variant in (list as Array):
		if entry is Dictionary and (entry as Dictionary).get("id") is String:
			var id: String = (entry as Dictionary)["id"] as String
			if re.search(id) != null and not id in out:
				out.append(id)
	return out


## Truncates `text` to `limit` characters (input boxes never send a longer string, F063-AC-12).
static func clip(text: String, limit: int) -> String:
	return text.left(maxi(0, limit))
