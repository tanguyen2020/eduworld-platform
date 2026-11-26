extends QuestionRendererBase
## `matching`: two columns of cards. PC drags a left card onto a right card; every device can tap a left card then a
## right card (the only way on mobile, STUDENT_GAME_UX_LEARNING §4.3, F063-AC-02). A paired left card shows its
## partner's number. Emits `{pairs}`.

var left_buttons: Dictionary = {}
var right_buttons: Dictionary = {}
var right_numbers: Dictionary = {}


func _make_answer() -> AnswerModel:
	return MatchingAnswer.new()


func _build() -> void:
	_add_stem(%Stem as Control)
	var columns: HBoxContainer = %Answers as HBoxContainer
	var model: MatchingAnswer = answer as MatchingAnswer
	var left_col: VBoxContainer = VBoxContainer.new()
	var right_col: VBoxContainer = VBoxContainer.new()
	left_col.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	right_col.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	columns.add_child(left_col)
	columns.add_child(right_col)
	var n: int = 1
	for entry: Dictionary in _entries("right", model.right):
		var id: String = String(entry["id"])
		var b: DropButton = DropButton.new()
		b.text = "%s. %s" % [char(64 + n), _plain(entry.get("content"))]
		b.drop_target_id = id
		b.dropped.connect(_on_dropped)
		b.pressed.connect(model.tap_right.bind(id))
		_style(b)
		right_col.add_child(b)
		right_buttons[id] = b
		right_numbers[id] = char(64 + n)
		n += 1
	for entry: Dictionary in _entries("left", model.left):
		var id: String = String(entry["id"])
		var b: DropButton = DropButton.new()
		b.toggle_mode = true
		b.drag_source_id = id
		b.set_meta("label", _plain(entry.get("content")))
		b.pressed.connect(model.tap_left.bind(id))
		_style(b)
		left_col.add_child(b)
		left_buttons[id] = b


func _entries(key: String, allowed: PackedStringArray) -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	var raw: Variant = item.render_payload.get(key, [])
	for e: Variant in (raw as Array if raw is Array else []):
		if e is Dictionary and String((e as Dictionary).get("id", "")) in allowed:
			out.append(e as Dictionary)
	return out


func _style(b: Button) -> void:
	b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
	b.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	b.alignment = HORIZONTAL_ALIGNMENT_LEFT


func _on_dropped(source_id: String, target_id: String) -> void:
	if not locked:
		(answer as MatchingAnswer).pair(source_id, target_id)


func _refresh() -> void:
	var model: MatchingAnswer = answer as MatchingAnswer
	for id: Variant in left_buttons.keys():
		var b: DropButton = left_buttons[id] as DropButton
		var partner: String = model.partner_of(String(id))
		var label: String = String(b.get_meta("label", ""))
		b.text = label if partner.is_empty() else "%s » %s" % [label, String(right_numbers.get(partner, "?"))]
		b.set_pressed_no_signal(model.picked_left == String(id))
		b.disabled = locked
		b.drag_enabled = not locked
	for id: Variant in right_buttons.keys():
		(right_buttons[id] as Button).disabled = locked
