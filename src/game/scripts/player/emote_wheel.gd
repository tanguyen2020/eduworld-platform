class_name EmoteWheel
extends Control
## Emote wheel (CHARACTER_SYSTEM §6–§7, STUDENT_GAME_UX §3–§4; F070-AC-20): opened with the input action
## `emote_wheel` (T) or the touch button; shows only the equipped emotes (`AvatarConfig.emote_slots`, at most
## `emote_wheel_slots`) in a ring and plays the chosen one once on the character through `anim_driver.gd`, which then
## returns to Locomotion. Local only in F070: sending `hub.player.emote` to the others comes with F091/F092.
## UI: generated Theme, touch targets from UiTokens density (`g1_3` ≥ 64 dp), strings `ui.emote.*`.

signal emote_selected(emote_id: String)
signal wheel_closed()

## Ring radius in touch-target units.
const RADIUS_FACTOR: float = 1.6

@export var anim_driver: AnimDriver = null
## Published catalog for the emote names (T040); null → ordinal labels "Emote n".
var catalog: CharacterCatalogModel = null

var _emotes: PackedStringArray = []
var _buttons: Array[Button] = []
var _target: float = 0.0

@onready var _touch: Button = %EmoteButton
@onready var _ring: Control = %Ring
@onready var _close: Button = %CloseWheel


func _ready() -> void:
	_ring.visible = false
	_touch.pressed.connect(toggle)
	_close.pressed.connect(close)
	AppState.grade_band_changed.connect(_on_band_changed)
	_on_band_changed(AppState.grade_band)
	_refresh_touch()


func _exit_tree() -> void:
	if AppState.grade_band_changed.is_connected(_on_band_changed):
		AppState.grade_band_changed.disconnect(_on_band_changed)


## Equipped emotes from the profile (`avatar_config.emote_slots`); blank codes are skipped, the list is capped.
func set_emotes(emote_slots: PackedStringArray) -> void:
	_emotes.clear()
	for code: String in emote_slots:
		if not code.is_empty() and not code in _emotes and _emotes.size() < AvatarConfigModel.MAX_EMOTE_SLOTS:
			_emotes.append(code)
	_rebuild()
	_refresh_touch()
	if _emotes.is_empty():
		close()


func shown_emotes() -> PackedStringArray:
	return _emotes.duplicate()


func slot_buttons() -> Array[Button]:
	return _buttons.duplicate()


func touch_button() -> Button:
	return _touch


func is_open() -> bool:
	return _ring.visible


func open() -> void:
	if _emotes.is_empty():
		return
	_ring.visible = true
	_layout()
	if not _buttons.is_empty():
		_buttons[0].grab_focus.call_deferred()


func close() -> void:
	if not _ring.visible:
		return
	_ring.visible = false
	wheel_closed.emit()


func toggle() -> void:
	if is_open():
		close()
	else:
		open()


## Plays `emote_id` on the character (clip from its ItemVisual, placeholder otherwise) and closes the wheel.
func choose(emote_id: String) -> void:
	close()
	if anim_driver != null:
		var visual: ItemVisual = ItemVisual.load_item(emote_id)
		anim_driver.play_emote(emote_id, visual.emote_clip if visual != null else null)
	emote_selected.emit(emote_id)


func _unhandled_input(event: InputEvent) -> void:
	if event.is_action_pressed("emote_wheel"):
		toggle()
		_handled()
	elif is_open() and event.is_action_pressed("ui_cancel"):
		close()
		_handled()


func _handled() -> void:
	var viewport: Viewport = get_viewport()
	if viewport != null:
		viewport.set_input_as_handled()


func _rebuild() -> void:
	for b: Button in _buttons:
		_ring.remove_child(b)
		b.queue_free()
	_buttons.clear()
	for i: int in _emotes.size():
		var b: Button = Button.new()
		b.name = "Emote%d" % i
		var ordinal: String = tr("ui.emote.slot").format({"n": i + 1})
		b.text = catalog.item_label(_emotes[i], ordinal) if catalog != null else ordinal
		b.tooltip_text = b.text
		b.custom_minimum_size = Vector2(_target, _target)
		b.pressed.connect(choose.bind(_emotes[i]))
		_ring.add_child(b)
		_buttons.append(b)
	_layout()


## Places the buttons on a circle around the ring centre (sizes from the density tokens, no fixed pixels).
func _layout() -> void:
	if _buttons.is_empty():
		return
	var radius: float = _target * RADIUS_FACTOR
	var centre: Vector2 = _ring.size / 2.0
	for i: int in _buttons.size():
		var angle: float = -PI / 2.0 + TAU * float(i) / float(_buttons.size())
		var b: Button = _buttons[i]
		b.size = b.get_combined_minimum_size()
		b.position = centre + Vector2.from_angle(angle) * radius - b.size / 2.0


func _refresh_touch() -> void:
	_touch.disabled = _emotes.is_empty()


func _on_band_changed(band: String) -> void:
	var valid: String = band if ThemeService.is_valid_band(band) else ThemeService.default_band()
	_target = float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT[valid])
	_touch.custom_minimum_size = Vector2(_target, _target)
	_close.custom_minimum_size = Vector2(_target, _target)
	_ring.custom_minimum_size = Vector2(_target, _target) * (RADIUS_FACTOR * 2.0 + 1.0)
	for b: Button in _buttons:
		b.custom_minimum_size = Vector2(_target, _target)
	_layout()
