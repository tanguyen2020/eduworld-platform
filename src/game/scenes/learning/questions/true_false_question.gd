extends QuestionRendererBase
## `true_false`: two large buttons (full width on mobile), or one true/false pair per statement. Labels come from the
## payload (`labels.true/false`, content language) or `ui.question.true` / `ui.question.false`. Emits `{value}` or
## `{statement_values}`. F098: with one statement a tap (or key `1`/`2`) reports `choice_tapped` and the graded tiles
## show the server's verdict; several statements are a multi-step answer (no tap report).

## statement id ("" for the single statement) → [true button, false button].
var pairs: Dictionary = {}


func _make_answer() -> AnswerModel:
	return TrueFalseAnswer.new()


func _build() -> void:
	_add_stem(%Stem as Control)
	var box: VBoxContainer = %Answers as VBoxContainer
	var model: TrueFalseAnswer = answer as TrueFalseAnswer
	if not model.has_statements():
		pairs[""] = _add_pair(box, "")
		return
	var raw: Variant = item.render_payload.get("statements", [])
	for s: Variant in (raw as Array if raw is Array else []):
		var id: String = String((s as Dictionary).get("id", "")) if s is Dictionary else ""
		if not id in model.statements:
			continue
		var label: Label = Label.new()
		label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		label.theme_type_variation = &"InstructionLabel"
		label.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		label.text = _plain((s as Dictionary).get("content"))
		box.add_child(label)
		pairs[id] = _add_pair(box, id)


func _add_pair(box: VBoxContainer, statement_id: String) -> Array[Button]:
	var row: GridContainer = GridContainer.new()
	row.columns = 2
	row.theme_type_variation = &"AnswerGrid"
	box.add_child(row)
	var out: Array[Button] = []
	for value: bool in [true, false]:
		var b: Button = Button.new()
		b.toggle_mode = true
		# F097: two big tiles (1 = true, 2 = false: the keyboard keys).
		_style_tile(b, 0 if value else 1)
		b.alignment = HORIZONTAL_ALIGNMENT_CENTER
		b.text = _label(value)
		if statement_id.is_empty():
			b.pressed.connect(_tap_value.bind(value))
		else:
			b.pressed.connect((answer as TrueFalseAnswer).set_statement.bind(statement_id, value))
		row.add_child(b)
		out.append(b)
	return out


func _label(value: bool) -> String:
	var labels: Variant = item.render_payload.get("labels")
	var key: String = "true" if value else "false"
	if labels is Dictionary and (labels as Dictionary).get(key) is String:
		return (labels as Dictionary)[key] as String
	return tr("ui.question.true") if value else tr("ui.question.false")


func _refresh() -> void:
	var model: TrueFalseAnswer = answer as TrueFalseAnswer
	for id: Variant in pairs.keys():
		var row: Array[Button] = pairs[id] as Array[Button]
		var chosen: int = model.value
		if not String(id).is_empty():
			chosen = (1 if bool(model.statement_values[id]) else 0) if model.statement_values.has(id) else -1
		row[0].set_pressed_no_signal(chosen == 1)
		row[1].set_pressed_no_signal(chosen == 0)
		_lock_tile(row[0], is_input_locked())
		_lock_tile(row[1], is_input_locked())


func select_index(index: int) -> void:
	if is_input_locked():
		return
	var model: TrueFalseAnswer = answer as TrueFalseAnswer
	if not model.has_statements() and (index == 0 or index == 1):
		_tap_value(index == 0)
	else:
		model.select_index(index)


## The single statement's tile tapped (or key `1` = true, `2` = false).
func _tap_value(value: bool) -> void:
	if is_input_locked():
		_refresh()
		return
	var model: TrueFalseAnswer = answer as TrueFalseAnswer
	var repeated: bool = model.value == (1 if value else 0)
	model.set_value(value)
	_refresh()
	choice_tapped.emit(repeated)


func _chosen_tile() -> Button:
	var model: TrueFalseAnswer = answer as TrueFalseAnswer
	if model.has_statements() or model.value < 0 or not pairs.has(""):
		return null
	var row: Array[Button] = pairs[""] as Array[Button]
	return row[0] if model.value == 1 else row[1]


func _tile_for_response(response: Dictionary) -> Button:
	if not response.get("value") is bool or not pairs.has(""):
		return null
	var row: Array[Button] = pairs[""] as Array[Button]
	return row[0] if bool(response["value"]) else row[1]
