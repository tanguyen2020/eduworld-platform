extends VBoxContainer
## HUD `ToastStack` (STUDENT_GAME_UX_CORE S03 §6, §13; F123-AC-16): at most 3 toasts at once — a new one drops the
## oldest —, each hidden after TOAST_VISIBLE_SEC and never blocking play (the stack takes no input outside its toasts).
## F123: a toast is the `NotificationToast` (tier 2 HUD panel, thin ink outline): the white bell icon, the text and a
## "tap to open" arrow, at least the band touch target high. Touching one emits `toast_pressed` with the tag given to
## `push` (the HUD owner opens the matching screen). No animation, no blinking, no sound. UI only.

signal toast_pressed(tag: Object)

const TOAST_SCENE: PackedScene = preload("res://scenes/ui/notifications/NotificationToast.tscn")
const MAX_TOASTS: int = 3
## Seconds a toast stays on screen (≥ 3 s reading time, STUDENT_GAME_UX §4).
const TOAST_VISIBLE_SEC: float = 6.0

var _ages: Array[float] = []
var _tags: Array[Object] = []


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE


## Shows an already translated text; `tag` comes back with `toast_pressed`.
func push(text: String, tag: Object = null) -> void:
	while get_child_count() >= MAX_TOASTS:
		_remove_at(0)
	var toast: Control = TOAST_SCENE.instantiate() as Control
	toast.mouse_filter = Control.MOUSE_FILTER_STOP
	toast.custom_minimum_size.y = UiBuild.target()
	var side: float = float(UiTokens.COMPONENT_ICON_SIZE)
	for icon_name: String in ["Icon", "Open"]:
		(toast.find_child(icon_name, true, false) as Control).custom_minimum_size = Vector2(side, side)
	# F096-AC-08: UiComponent shows Body from `body_key` in _ready (an empty key hides it), so the text is set first.
	var body: Label = toast.find_child("Body", true, false) as Label
	body.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
	toast.set("body_key", text)
	add_child(toast)
	toast.gui_input.connect(_on_toast_input.bind(toast))
	_ages.append(0.0)
	_tags.append(tag)


func texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for child: Node in get_children():
		out.append((child.find_child("Body", true, false) as Label).text)
	return out


## Touching the toast at `index` (also used by keyboard/gamepad focus and tests).
func press(index: int) -> void:
	if index < 0 or index >= get_child_count():
		return
	var tag: Object = _tags[index]
	clear()
	toast_pressed.emit(tag)


func clear() -> void:
	while get_child_count() > 0:
		_remove_at(0)


func _remove_at(index: int) -> void:
	var child: Node = get_child(index)
	remove_child(child)
	child.queue_free()
	_ages.remove_at(index)
	_tags.remove_at(index)


func _on_toast_input(event: InputEvent, toast: Control) -> void:
	var tapped: bool = (event is InputEventMouseButton and (event as InputEventMouseButton).pressed) \
			or (event is InputEventScreenTouch and (event as InputEventScreenTouch).pressed)
	if tapped and toast.get_parent() == self:
		press(toast.get_index())


func _process(delta: float) -> void:
	for i: int in range(_ages.size() - 1, -1, -1):
		_ages[i] += delta
		if _ages[i] >= TOAST_VISIBLE_SEC:
			_remove_at(i)
