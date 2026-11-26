extends QuestionRendererBase
## `numeric`: large input + unit label and a custom number pad (`,`/`.` per the payload, `−`, `/` when fractions are
## allowed, backspace) — the system keyboard is not opened on mobile (STUDENT_GAME_UX_LEARNING §4.3). At most 64
## characters (F063-AC-12). Emits `{value}` as typed; never evaluated on the client.

var edit: LineEdit = null
var pad_buttons: Array[Button] = []


func _make_answer() -> AnswerModel:
	return NumericAnswer.new()


func _build() -> void:
	_add_stem(%Stem as Control)
	var box: VBoxContainer = %Answers as VBoxContainer
	var model: NumericAnswer = answer as NumericAnswer
	var row: HBoxContainer = HBoxContainer.new()
	box.add_child(row)
	edit = LineEdit.new()
	edit.max_length = NumericAnswer.CONTRACT_MAX_LENGTH
	edit.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	edit.virtual_keyboard_enabled = false
	edit.text_changed.connect(model.set_text)
	row.add_child(edit)
	if not model.unit_label.is_empty():
		var unit: Label = Label.new()
		unit.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		unit.text = model.unit_label
		row.add_child(unit)
	var pad: GridContainer = GridContainer.new()
	pad.columns = 3
	pad.theme_type_variation = &"AnswerGrid"
	box.add_child(pad)
	for key: String in model.pad_keys():
		var b: Button = Button.new()
		# F097: big outlined keys, as tall as an answer tile of the band.
		b.theme_type_variation = &"SecondaryButton"
		b.custom_minimum_size = Vector2.ONE * _tile_height()
		b.text = key
		b.pressed.connect(model.press.bind(key))
		pad.add_child(b)
		pad_buttons.append(b)
	var back: Button = Button.new()
	back.theme_type_variation = &"SecondaryButton"
	back.text = "ui.question.backspace"
	back.tooltip_text = "ui.question.backspace"
	back.pressed.connect(model.backspace)
	pad.add_child(back)
	pad_buttons.append(back)


func _refresh() -> void:
	var model: NumericAnswer = answer as NumericAnswer
	if edit.text != model.text:
		edit.text = model.text
		edit.caret_column = model.text.length()
	edit.editable = not locked
	for b: Button in pad_buttons:
		b.disabled = locked
