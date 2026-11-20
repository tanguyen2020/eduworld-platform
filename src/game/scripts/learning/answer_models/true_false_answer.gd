class_name TrueFalseAnswer
extends AnswerModel
## `true_false` → `{value}` (one statement in the stem) or `{statement_values: {id: bool}}` (`statements[]`).

var statements: PackedStringArray = []
## Single statement: -1 = not answered, 0 = false, 1 = true.
var value: int = -1
var statement_values: Dictionary = {}


func _on_setup() -> void:
	statements = AnswerModel.ids_of(payload.get("statements"))
	value = -1
	statement_values = {}


func has_statements() -> bool:
	return not statements.is_empty()


func set_value(v: bool) -> void:
	value = 1 if v else 0
	_notify()


func set_statement(statement_id: String, v: bool) -> void:
	if statement_id in statements:
		statement_values[statement_id] = v
		_notify()


## Keyboard: `1` = true, `2` = false (single statement).
func select_index(index: int) -> void:
	if not has_statements() and (index == 0 or index == 1):
		set_value(index == 0)


func is_complete() -> bool:
	if has_statements():
		return statement_values.size() == statements.size()
	return value >= 0


func to_response() -> Dictionary:
	if not is_complete():
		return {}
	if has_statements():
		return {"statement_values": statement_values.duplicate()}
	return {"value": value == 1}


func restore(response: Dictionary) -> void:
	if response.get("value") is bool:
		value = 1 if bool(response["value"]) else 0
	var values: Variant = response.get("statement_values")
	if values is Dictionary:
		for id: Variant in (values as Dictionary).keys():
			if String(id) in statements and (values as Dictionary)[id] is bool:
				statement_values[String(id)] = bool((values as Dictionary)[id])
	_notify()
