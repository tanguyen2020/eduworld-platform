class_name MultipleChoiceAnswer
extends AnswerModel
## `multiple_choice` → `{option_ids: [...]}` (1–10 unique ids, in display order).

## `MultipleChoiceResponse.option_ids` maxItems.
const MAX_SELECTED: int = 10

var options: PackedStringArray = []
var selected: PackedStringArray = []


func _on_setup() -> void:
	options = AnswerModel.ids_of(payload.get("options"))
	selected = []


func toggle(option_id: String) -> void:
	if not option_id in options:
		return
	if option_id in selected:
		selected.remove_at(selected.find(option_id))
	elif selected.size() < MAX_SELECTED:
		selected.append(option_id)
	else:
		return
	_notify()


func select_index(index: int) -> void:
	if index >= 0 and index < options.size():
		toggle(options[index])


func is_selected(option_id: String) -> bool:
	return option_id in selected


func is_complete() -> bool:
	return not selected.is_empty()


func to_response() -> Dictionary:
	if not is_complete():
		return {}
	var ordered: Array = []
	for id: String in options:
		if id in selected:
			ordered.append(id)
	return {"option_ids": ordered}


func restore(response: Dictionary) -> void:
	var ids: Variant = response.get("option_ids")
	if not ids is Array:
		return
	selected = []
	for id: Variant in (ids as Array):
		if id is String and (id as String) in options and not (id as String) in selected \
				and selected.size() < MAX_SELECTED:
			selected.append(id as String)
	_notify()
