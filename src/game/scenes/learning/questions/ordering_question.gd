extends QuestionRendererBase
## `ordering`: vertical cards in the order the server sent; ▲▼ buttons on each card (all bands, g1_3 included), drag
## a card onto another to move it there, `↑ ↓` move the focused card. Emits `{order}`.

var rows: Dictionary = {}
var labels: Dictionary = {}


func _make_answer() -> AnswerModel:
	return OrderingAnswer.new()


func _build() -> void:
	_add_stem(%Stem as Control)
	var model: OrderingAnswer = answer as OrderingAnswer
	var raw: Variant = item.render_payload.get("items", [])
	for e: Variant in (raw as Array if raw is Array else []):
		if e is Dictionary and String((e as Dictionary).get("id", "")) in model.order:
			labels[String((e as Dictionary)["id"])] = _plain((e as Dictionary).get("content"))
	var box: VBoxContainer = %Answers as VBoxContainer
	for id: String in model.order:
		var row: HBoxContainer = HBoxContainer.new()
		var card: DropButton = DropButton.new()
		card.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		card.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		card.alignment = HORIZONTAL_ALIGNMENT_LEFT
		card.drag_source_id = id
		card.drop_target_id = id
		card.dropped.connect(_on_dropped)
		card.gui_input.connect(_on_card_input.bind(id))
		var up: Button = Button.new()
		up.text = "ui.question.move_up"
		up.tooltip_text = "ui.question.move_up"
		up.pressed.connect(model.move.bind(id, -1))
		var down: Button = Button.new()
		down.text = "ui.question.move_down"
		down.tooltip_text = "ui.question.move_down"
		down.pressed.connect(model.move.bind(id, 1))
		row.add_child(card)
		row.add_child(up)
		row.add_child(down)
		box.add_child(row)
		rows[id] = row


func _on_dropped(source_id: String, target_id: String) -> void:
	var model: OrderingAnswer = answer as OrderingAnswer
	if not locked:
		model.move_to(model.order.find(source_id), model.order.find(target_id))


func _on_card_input(event: InputEvent, id: String) -> void:
	if locked or not event is InputEventKey or not event.is_pressed():
		return
	var key: Key = (event as InputEventKey).keycode
	if key == KEY_UP or key == KEY_DOWN:
		(answer as OrderingAnswer).move(id, -1 if key == KEY_UP else 1)
		accept_event()


func _refresh() -> void:
	var model: OrderingAnswer = answer as OrderingAnswer
	var box: VBoxContainer = %Answers as VBoxContainer
	var i: int = 0
	for id: String in model.order:
		var row: HBoxContainer = rows.get(id) as HBoxContainer
		if row == null:
			continue
		box.move_child(row, i)
		var card: DropButton = row.get_child(0) as DropButton
		card.text = "%d. %s" % [i + 1, String(labels.get(id, ""))]
		card.drag_enabled = not locked
		for b: Node in row.get_children():
			(b as Button).disabled = locked
		i += 1
