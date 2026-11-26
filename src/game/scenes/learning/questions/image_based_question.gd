extends QuestionRendererBase
## `image_based`: the item image (MediaLoader) scaled to fit. Hotspot: a tap/click records a point in original
## `image_size` coordinates (markers drawn on top); label: labels are tapped then a drop point tapped (or dragged on
## PC). The client never sees the answer regions (ADR-0016). Emits `{points}` or `{label_placements}`.

var image: TextureRect = null
var markers: Control = null
var label_buttons: Dictionary = {}
var point_buttons: Dictionary = {}


func _make_answer() -> AnswerModel:
	return ImageBasedAnswer.new()


func _build() -> void:
	_add_stem(%Stem as Control)
	var model: ImageBasedAnswer = answer as ImageBasedAnswer
	var box: VBoxContainer = %Answers as VBoxContainer
	image = TextureRect.new()
	image.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	image.stretch_mode = TextureRect.STRETCH_SCALE
	image.custom_minimum_size = _display_size(model.image_size)
	image.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	image.mouse_filter = Control.MOUSE_FILTER_STOP
	image.gui_input.connect(_on_image_input)
	box.add_child(image)
	markers = Control.new()
	markers.mouse_filter = Control.MOUSE_FILTER_IGNORE
	markers.set_anchors_preset(Control.PRESET_FULL_RECT)
	image.add_child(markers)
	var url: String = item.media_url(String(item.render_payload.get("image_media_ref", "")))
	if not url.is_empty() and context != null and context.media != null:
		_load(url)
	if model.interaction != ImageBasedAnswer.INTERACTION_LABEL:
		return
	var tray: HFlowContainer = HFlowContainer.new()
	box.add_child(tray)
	var raw: Variant = item.render_payload.get("labels", [])
	for l: Variant in (raw as Array if raw is Array else []):
		var id: String = String((l as Dictionary).get("id", "")) if l is Dictionary else ""
		if not id in model.labels:
			continue
		var b: DropButton = DropButton.new()
		b.toggle_mode = true
		b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		b.text = _plain((l as Dictionary).get("content"))
		b.drag_source_id = id
		b.pressed.connect(model.tap_label.bind(id))
		tray.add_child(b)
		label_buttons[id] = b
	var n: int = 1
	for id: String in model.drop_points:
		var p: DropButton = DropButton.new()
		p.text = str(n)
		p.drop_target_id = id
		p.dropped.connect(_on_label_dropped)
		p.pressed.connect(model.tap_drop_point.bind(id))
		markers.add_child(p)
		point_buttons[id] = p
		n += 1


## The image fits ≤ 640 px wide at its aspect ratio (scaled by the Theme later through the container).
static func _display_size(original: Vector2) -> Vector2:
	var width: float = minf(original.x, 640.0)
	return Vector2(width, width * original.y / maxf(original.x, 1.0))


func _load(url: String) -> void:
	var tex: Texture2D = await context.media.texture(url)
	if is_instance_valid(image) and tex != null:
		image.texture = tex


## Converts a click/tap on the displayed image into original image coordinates.
func to_image_coords(local: Vector2) -> Vector2:
	var model: ImageBasedAnswer = answer as ImageBasedAnswer
	var shown: Vector2 = image.size if image.size.x > 0.0 else image.custom_minimum_size
	return Vector2(local.x / maxf(shown.x, 1.0) * model.image_size.x, local.y / maxf(shown.y, 1.0) * model.image_size.y)


func _on_image_input(event: InputEvent) -> void:
	var model: ImageBasedAnswer = answer as ImageBasedAnswer
	if locked or model.interaction != ImageBasedAnswer.INTERACTION_HOTSPOT:
		return
	var tapped: bool = (event is InputEventMouseButton and (event as InputEventMouseButton).pressed
			and (event as InputEventMouseButton).button_index == MOUSE_BUTTON_LEFT) \
			or (event is InputEventScreenTouch and (event as InputEventScreenTouch).pressed)
	if tapped:
		model.add_point(to_image_coords((event as InputEventMouseButton).position if event is InputEventMouseButton
				else (event as InputEventScreenTouch).position))


func _on_label_dropped(source_id: String, target_id: String) -> void:
	if not locked:
		(answer as ImageBasedAnswer).place_label(source_id, target_id)


func _refresh() -> void:
	var model: ImageBasedAnswer = answer as ImageBasedAnswer
	var shown: Vector2 = image.size if image.size.x > 0.0 else image.custom_minimum_size
	var scale: Vector2 = shown / model.image_size
	if model.interaction == ImageBasedAnswer.INTERACTION_HOTSPOT:
		for child: Node in markers.get_children():
			child.queue_free()
		for p: Vector2 in model.points:
			var mark: Label = Label.new()
			mark.text = "•"
			mark.position = p * scale
			markers.add_child(mark)
		return
	var raw: Variant = item.render_payload.get("drop_points", [])
	for dp: Variant in (raw as Array if raw is Array else []):
		var id: String = String((dp as Dictionary).get("id", "")) if dp is Dictionary else ""
		if point_buttons.has(id):
			var b: DropButton = point_buttons[id] as DropButton
			b.position = Vector2(float((dp as Dictionary).get("x", 0)), float((dp as Dictionary).get("y", 0))) * scale
			b.disabled = locked
	for id: Variant in label_buttons.keys():
		var b: DropButton = label_buttons[id] as DropButton
		b.set_pressed_no_signal(model.picked_label == String(id))
		b.disabled = locked or model.label_placements.has(id)
