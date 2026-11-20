extends QuestionRendererBase
## `true_false`: two large buttons (full width on mobile), or one true/false pair per statement. Labels come from the
## payload (`labels.true/false`, content language) or `ui.question.true` / `ui.question.false`. Emits `{value}` or
## `{statement_values}`.

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
		label.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		label.text = _plain((s as Dictionary).get("content"))
		box.add_child(label)
		pairs[id] = _add_pair(box, id)


func _add_pair(box: VBoxContainer, statement_id: String) -> Array[Button]:
	var row: HBoxContainer = HBoxContainer.new()
	box.add_child(row)
	var out: Array[Button] = []
	for value: bool in [true, false]:
		var b: Button = Button.new()
		b.toggle_mode = true
		b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		b.text = _label(value)
		b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		if statement_id.is_empty():
			b.pressed.connect((answer as TrueFalseAnswer).set_value.bind(value))
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
		row[0].disabled = locked
		row[1].disabled = locked


func select_index(index: int) -> void:
	if not locked:
		(answer as TrueFalseAnswer).select_index(index)
