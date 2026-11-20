class_name OrderingAnswer
extends AnswerModel
## `ordering` → `{order: [...]}` (3–10 unique ids). Starts in the order the server sent (already shuffled); moved by
## drag, `↑ ↓` keys or ▲▼ buttons. Complete once the student has moved at least one card.

var order: PackedStringArray = []
var touched: bool = false


func _on_setup() -> void:
	order = AnswerModel.ids_of(payload.get("items"))
	touched = false


func move(item_id: String, delta: int) -> void:
	var from: int = order.find(item_id)
	if from < 0:
		return
	move_to(from, clampi(from + delta, 0, order.size() - 1))


func move_to(from: int, to: int) -> void:
	if from < 0 or from >= order.size() or to < 0 or to >= order.size():
		return
	touched = true
	if from != to:
		var id: String = order[from]
		order.remove_at(from)
		order.insert(to, id)
	_notify()


func is_complete() -> bool:
	return touched and order.size() >= 3


func to_response() -> Dictionary:
	return {"order": Array(order)} if is_complete() else {}


func restore(response: Dictionary) -> void:
	var saved: Variant = response.get("order")
	if not saved is Array or (saved as Array).size() != order.size():
		return
	var restored: PackedStringArray = []
	for id: Variant in (saved as Array):
		if not id is String or not (id as String) in order or (id as String) in restored:
			return
		restored.append(id as String)
	order = restored
	touched = true
	_notify()
