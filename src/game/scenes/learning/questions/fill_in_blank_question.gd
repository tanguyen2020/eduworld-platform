extends QuestionRendererBase
## `fill_in_blank`: one input per blank under the sentence (text / numeric `LineEdit` with `max_length` from the
## payload, F063-AC-12; `word_bank` / `dropdown` as an `OptionButton`). Emits `{blanks}`.

var inputs: Dictionary = {}


func _make_answer() -> AnswerModel:
	return FillInBlankAnswer.new()


func _build() -> void:
	_add_stem(%Stem as Control)
	var box: VBoxContainer = %Answers as VBoxContainer
	var model: FillInBlankAnswer = answer as FillInBlankAnswer
	var n: int = 1
	for id: String in model.blank_ids():
		var row: HBoxContainer = HBoxContainer.new()
		box.add_child(row)
		var tag: Label = Label.new()
		tag.text = "(%d)" % n
		row.add_child(tag)
		var kind: String = model.input_of(id)
		if kind == "word_bank" or kind == "dropdown":
			var pick: OptionButton = OptionButton.new()
			pick.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
			pick.size_flags_horizontal = Control.SIZE_EXPAND_FILL
			for choice: String in model.choices_of(id):
				pick.add_item(choice)
			pick.selected = -1
			pick.item_selected.connect(_on_choice.bind(id, pick))
			row.add_child(pick)
			inputs[id] = pick
		else:
			var edit: LineEdit = LineEdit.new()
			edit.max_length = model.max_length(id)
			edit.size_flags_horizontal = Control.SIZE_EXPAND_FILL
			edit.virtual_keyboard_type = LineEdit.KEYBOARD_TYPE_NUMBER if kind == "numeric" \
					else LineEdit.KEYBOARD_TYPE_DEFAULT
			edit.text_changed.connect(_on_text.bind(id))
			row.add_child(edit)
			inputs[id] = edit
		n += 1


func _on_text(text: String, blank_id: String) -> void:
	(answer as FillInBlankAnswer).set_blank(blank_id, text)


func _on_choice(index: int, blank_id: String, pick: OptionButton) -> void:
	(answer as FillInBlankAnswer).set_blank(blank_id, pick.get_item_text(index))


func _refresh() -> void:
	var model: FillInBlankAnswer = answer as FillInBlankAnswer
	for id: Variant in inputs.keys():
		var control: Control = inputs[id] as Control
		if control is LineEdit:
			var edit: LineEdit = control as LineEdit
			if edit.text != model.value_of(String(id)):
				edit.text = model.value_of(String(id))
			edit.editable = not locked
		elif control is OptionButton:
			(control as OptionButton).disabled = locked
