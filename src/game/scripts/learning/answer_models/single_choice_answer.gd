class_name SingleChoiceAnswer
extends AnswerModel
## `single_choice` → `{option_id}`.

var options: PackedStringArray = []
var selected: String = ""


func _on_setup() -> void:
	options = AnswerModel.ids_of(payload.get("options"))
	selected = ""


func select(option_id: String) -> void:
	if option_id in options and option_id != selected:
		selected = option_id
		_notify()


## Keyboard `1–6` (STUDENT_GAME_UX_LEARNING §3.6): selects the n-th displayed option.
func select_index(index: int) -> void:
	if index >= 0 and index < options.size():
		select(options[index])


func is_complete() -> bool:
	return not selected.is_empty()


func to_response() -> Dictionary:
	return {"option_id": selected} if is_complete() else {}


func restore(response: Dictionary) -> void:
	var id: Variant = response.get("option_id")
	if id is String and (id as String) in options:
		selected = id as String
		_notify()
