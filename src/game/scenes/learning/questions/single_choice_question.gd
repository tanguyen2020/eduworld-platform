extends QuestionRendererBase
## `single_choice` (STUDENT_GAME_UX_LEARNING §4.1/4.2): stem + choice cards, 2×2 grid up to 4 short options, else one
## column; keys `1–6` select. Emits `{option_id}`.

var buttons: Dictionary = {}


func _make_answer() -> AnswerModel:
	return SingleChoiceAnswer.new()


func _build() -> void:
	_add_stem(%Stem as Control)
	var grid: GridContainer = %Answers as GridContainer
	var options: Array = render_options()
	grid.columns = 2 if options.size() <= 4 else 1
	var i: int = 0
	for option: Variant in options:
		var id: String = String((option as Dictionary).get("id", ""))
		var b: Button = _make_choice((option as Dictionary).get("content"), true, i)
		b.pressed.connect((answer as SingleChoiceAnswer).select.bind(id))
		grid.add_child(b)
		buttons[id] = b
		i += 1


func render_options() -> Array:
	var raw: Variant = item.render_payload.get("options", [])
	var out: Array = []
	if raw is Array:
		for o: Variant in (raw as Array):
			if o is Dictionary and String((o as Dictionary).get("id", "")) in (answer as SingleChoiceAnswer).options:
				out.append(o)
	return out


func _refresh() -> void:
	var model: SingleChoiceAnswer = answer as SingleChoiceAnswer
	for id: Variant in buttons.keys():
		var b: Button = buttons[id] as Button
		b.set_pressed_no_signal(String(id) == model.selected)
		b.disabled = locked


func select_index(index: int) -> void:
	if not locked:
		(answer as SingleChoiceAnswer).select_index(index)
