extends VBoxContainer
## HUD `ToastStack` (STUDENT_GAME_UX_CORE S03 §6, §13): at most 3 toasts at once — a new one drops the oldest —, each
## hidden after TOAST_VISIBLE_SEC; touching one emits `toast_pressed` (the HUD owner opens the matching screen). No
## animation, no blinking. UI only.

signal toast_pressed()

const TOAST_SCENE: PackedScene = preload("res://scenes/ui/components/Toast.tscn")
const MAX_TOASTS: int = 3
## Seconds a toast stays on screen.
const TOAST_VISIBLE_SEC: float = 6.0

var _ages: Array[float] = []


## Shows an already translated text.
func push(text: String) -> void:
	while get_child_count() >= MAX_TOASTS:
		_remove_at(0)
	var toast: Control = TOAST_SCENE.instantiate() as Control
	toast.mouse_filter = Control.MOUSE_FILTER_STOP
	add_child(toast)
	var body: Label = toast.find_child("Body", true, false) as Label
	body.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
	body.text = text
	toast.gui_input.connect(_on_toast_input.bind(toast))
	_ages.append(0.0)


func texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for child: Node in get_children():
		out.append((child.find_child("Body", true, false) as Label).text)
	return out


## Touching the toast at `index` (also used by keyboard/gamepad focus and tests).
func press(index: int) -> void:
	if index < 0 or index >= get_child_count():
		return
	clear()
	toast_pressed.emit()


func clear() -> void:
	while get_child_count() > 0:
		_remove_at(0)


func _remove_at(index: int) -> void:
	var child: Node = get_child(index)
	remove_child(child)
	child.queue_free()
	_ages.remove_at(index)


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
