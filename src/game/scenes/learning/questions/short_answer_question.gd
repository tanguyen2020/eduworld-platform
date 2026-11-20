extends QuestionRendererBase
## `short_answer`: one-line input with a character counter; `max_length` from the payload (default 100, contract
## ≤ 500, F063-AC-12). Graded by the server against its accepted answers. Emits `{text}`.

var edit: LineEdit = null
var counter: Label = null


func _make_answer() -> AnswerModel:
	return ShortAnswerAnswer.new()


func _build() -> void:
	_add_stem(%Stem as Control)
	var box: VBoxContainer = %Answers as VBoxContainer
	var model: ShortAnswerAnswer = answer as ShortAnswerAnswer
	edit = LineEdit.new()
	edit.max_length = model.limit
	edit.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	edit.text_changed.connect(model.set_text)
	box.add_child(edit)
	counter = Label.new()
	counter.theme_type_variation = &"MutedLabel"
	counter.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT
	box.add_child(counter)


func _refresh() -> void:
	var model: ShortAnswerAnswer = answer as ShortAnswerAnswer
	if edit.text != model.text:
		edit.text = model.text
	edit.editable = not locked
	counter.text = "%d/%d" % [model.text.length(), model.limit]
