extends QuestionRendererBase
## `numeric`: large input + unit label and a custom number pad (`,`/`.` per the payload, `−`, `/` when fractions are
## allowed, backspace) — the system keyboard is not opened on mobile (STUDENT_GAME_UX_LEARNING §4.3). At most 64
## characters (F063-AC-12). Emits `{value}` as typed; never evaluated on the client.
## F098 (grade-band matrix): the pad is two wide rows in reading order (1 2 3 … 0, then the signs and backspace) beside
## the input, so it fits a 720/750 px tall board in every band instead of a tall calculator block under the input that
## had to be scrolled at g1_3 (keys as tall as an answer tile of the band).

## Pad rows (wide layout).
const PAD_ROWS: int = 2
const DIGITS: PackedStringArray = ["1", "2", "3", "4", "5", "6", "7", "8", "9", "0"]

var edit: LineEdit = null
var pad_buttons: Array[Button] = []


func _make_answer() -> AnswerModel:
	return NumericAnswer.new()


func _build() -> void:
	_add_stem(%Stem as Control)
	var box: VBoxContainer = %Answers as VBoxContainer
	var model: NumericAnswer = answer as NumericAnswer
	# F098: the input (with its unit) and the pad side by side, so the pad never needs scrolling on a short board.
	var layout: HBoxContainer = HBoxContainer.new()
	layout.name = "PadLayout"
	box.add_child(layout)
	var row: HBoxContainer = HBoxContainer.new()
	row.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	row.size_flags_vertical = Control.SIZE_SHRINK_BEGIN
	layout.add_child(row)
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
	pad.theme_type_variation = &"AnswerGrid"
	layout.add_child(pad)
	var keys: PackedStringArray = _wide_order(model.pad_keys())
	pad.columns = ceili(float(keys.size() + 1) / float(PAD_ROWS))
	for key: String in keys:
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


## The pad keys in reading order: digits 1–9, 0, then the other keys of the payload (sign, separator, fraction).
static func _wide_order(keys: PackedStringArray) -> PackedStringArray:
	var out: PackedStringArray = []
	for d: String in DIGITS:
		if d in keys:
			out.append(d)
	for k: String in keys:
		if not k in DIGITS:
			out.append(k)
	return out


func _refresh() -> void:
	var model: NumericAnswer = answer as NumericAnswer
	if edit.text != model.text:
		edit.text = model.text
		edit.caret_column = model.text.length()
	edit.editable = not locked
	for b: Button in pad_buttons:
		b.disabled = locked
