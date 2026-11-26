extends QuestionRendererBase
## `single_choice` (STUDENT_GAME_UX_LEARNING §4.1/4.2): stem + choice cards, 2×2 grid up to 4 short options, else one
## column; keys `1–6` select. Emits `{option_id}`. F098: a tap (or its key) reports `choice_tapped` — the host sends the
## answer at once when the attempt allows it — and the graded tiles show the server's verdict.

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
		b.pressed.connect(_tap.bind(id))
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
		_lock_tile(b, is_input_locked())


func select_index(index: int) -> void:
	var model: SingleChoiceAnswer = answer as SingleChoiceAnswer
	if not is_input_locked() and index >= 0 and index < model.options.size():
		_tap(model.options[index])


## A tile tapped (or its number key): selects it and reports the tap (`repeated` = it was already selected).
func _tap(option_id: String) -> void:
	if is_input_locked():
		_refresh()
		return
	var model: SingleChoiceAnswer = answer as SingleChoiceAnswer
	var repeated: bool = model.selected == option_id
	model.select(option_id)
	# A second press on a toggle tile would untoggle it: the selection stays (one answer is always chosen).
	_refresh()
	choice_tapped.emit(repeated)


func _chosen_tile() -> Button:
	return buttons.get((answer as SingleChoiceAnswer).selected) as Button


func _tile_for_response(response: Dictionary) -> Button:
	return buttons.get(String(response.get("option_id", ""))) as Button
