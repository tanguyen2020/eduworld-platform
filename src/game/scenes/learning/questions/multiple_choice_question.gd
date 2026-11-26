extends QuestionRendererBase
## `multiple_choice`: choice cards with check state and "Chọn tất cả đáp án đúng"; keys `1–6` toggle. Emits
## `{option_ids}`.

var buttons: Dictionary = {}


func _make_answer() -> AnswerModel:
	return MultipleChoiceAnswer.new()


func _build() -> void:
	_add_stem(%Stem as Control)
	var grid: GridContainer = %Answers as GridContainer
	var model: MultipleChoiceAnswer = answer as MultipleChoiceAnswer
	var raw: Variant = item.render_payload.get("options", [])
	var options: Array = raw as Array if raw is Array else []
	grid.columns = 2 if options.size() <= 4 else 1
	var i: int = 0
	for option: Variant in options:
		var id: String = String((option as Dictionary).get("id", "")) if option is Dictionary else ""
		if not id in model.options:
			continue
		var b: Button = _make_choice((option as Dictionary).get("content"), true, i)
		b.pressed.connect(model.toggle.bind(id))
		grid.add_child(b)
		buttons[id] = b
		i += 1


func _refresh() -> void:
	var model: MultipleChoiceAnswer = answer as MultipleChoiceAnswer
	for id: Variant in buttons.keys():
		var b: Button = buttons[id] as Button
		b.set_pressed_no_signal(model.is_selected(String(id)))
		_lock_tile(b, locked)


func select_index(index: int) -> void:
	if not locked:
		(answer as MultipleChoiceAnswer).select_index(index)
