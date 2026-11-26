extends QuestionRendererBase
## `drag_drop`: zones (labels in the content language) above a tray of cards. PC drags a card onto a zone; every
## device can tap a card then a zone (STUDENT_GAME_UX_LEARNING §4.3, F063-AC-02); tapping a placed card sends it
## back to the tray. Emits `{placements}`.

## F096-AC-15: icon of a card already dropped in a zone.
const PLACED_ICON: Texture2D = preload("res://resources/ui/icons/tick.svg")

var cards: Dictionary = {}
var zones: Dictionary = {}
var card_labels: Dictionary = {}
var zone_labels: Dictionary = {}


func _make_answer() -> AnswerModel:
	return DragDropAnswer.new()


func _build() -> void:
	_add_stem(%Stem as Control)
	var model: DragDropAnswer = answer as DragDropAnswer
	var box: VBoxContainer = %Answers as VBoxContainer
	var zone_row: GridContainer = GridContainer.new()
	zone_row.theme_type_variation = &"AnswerGrid"
	box.add_child(zone_row)
	var raw_zones: Variant = item.render_payload.get("zones", [])
	for z: Variant in (raw_zones as Array if raw_zones is Array else []):
		var id: String = String((z as Dictionary).get("id", "")) if z is Dictionary else ""
		if not id in model.zones:
			continue
		var b: DropButton = DropButton.new()
		# F097: a drop zone is a white rounded target; the cards are colourful tiles.
		b.theme_type_variation = &"DropZone"
		b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		b.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		b.drop_target_id = id
		b.dropped.connect(_on_dropped)
		b.pressed.connect(model.tap_zone.bind(id))
		zone_labels[id] = context.text((z as Dictionary).get("label"))
		zone_row.add_child(b)
		zones[id] = b
	zone_row.columns = maxi(zones.size(), 1)
	var tray: HFlowContainer = HFlowContainer.new()
	tray.theme_type_variation = &"AnswerFlow"
	box.add_child(tray)
	var raw_cards: Variant = item.render_payload.get("draggables", [])
	for c: Variant in (raw_cards as Array if raw_cards is Array else []):
		var id: String = String((c as Dictionary).get("id", "")) if c is Dictionary else ""
		if not id in model.draggables:
			continue
		var b: DropButton = DropButton.new()
		b.toggle_mode = true
		_style_tile(b, cards.size())
		# A card in the tray is as wide as its words (no wrapping inside a flow row).
		b.autowrap_mode = TextServer.AUTOWRAP_OFF
		b.size_flags_horizontal = Control.SIZE_FILL
		b.drag_source_id = id
		b.pressed.connect(_on_card_pressed.bind(id))
		card_labels[id] = _plain((c as Dictionary).get("content"))
		tray.add_child(b)
		cards[id] = b


func _on_card_pressed(id: String) -> void:
	var model: DragDropAnswer = answer as DragDropAnswer
	if locked:
		return
	if not model.zone_of(id).is_empty():
		model.remove(id)
	else:
		model.tap_card(id)


func _on_dropped(source_id: String, target_id: String) -> void:
	if not locked:
		(answer as DragDropAnswer).place(source_id, target_id)


func _refresh() -> void:
	var model: DragDropAnswer = answer as DragDropAnswer
	for id: Variant in zones.keys():
		var inside: PackedStringArray = []
		for card: Variant in model.placements.keys():
			if String(model.placements[card]) == String(id):
				inside.append(String(card_labels.get(card, "")))
		var b: DropButton = zones[id] as DropButton
		b.text = String(zone_labels.get(id, "")) + ("" if inside.is_empty() else ": " + ", ".join(inside))
		b.disabled = locked
	for id: Variant in cards.keys():
		var b: DropButton = cards[id] as DropButton
		var zone: String = model.zone_of(String(id))
		b.text = String(card_labels.get(id, ""))
		# F096-AC-15: a placed card shows the tick icon (no ✓ glyph in the shipped fonts).
		b.icon = null if zone.is_empty() else PLACED_ICON
		b.set_pressed_no_signal(model.picked == String(id))
		b.disabled = locked
		b.drag_enabled = not locked
