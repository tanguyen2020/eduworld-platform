extends Control
## Virtual joystick (STUDENT_GAME_UX §4, F062-AC-02): drag inside the base to move; emits a vector (x right, y
## forward, length ≤ 1) and zero on release. Shown on touch screens only. No movement logic: WorldRoot forwards the
## vector to PlayerController.set_move_input.

signal moved(vector: Vector2)

## Diameter in touch-target units (UiTokens control size × 2.5).
const SIZE_FACTOR: float = 2.5

var vector: Vector2 = Vector2.ZERO
var _dragging: bool = false

@onready var _knob: Control = %Knob


func _ready() -> void:
	visible = DisplayServer.is_touchscreen_available()
	custom_minimum_size = Vector2.ONE * float(UiTokens.SIZE_CONTROL_64) * SIZE_FACTOR


func radius() -> float:
	return maxf(size.x, size.y) / 2.0


## Joystick vector of a point in local coordinates (y up = forward).
func vector_at(local_point: Vector2) -> Vector2:
	var r: float = radius()
	if r <= 0.0:
		return Vector2.ZERO
	var offset: Vector2 = (local_point - size / 2.0) / r
	return Vector2(offset.x, -offset.y).limit_length(1.0)


func _gui_input(event: InputEvent) -> void:
	if event is InputEventScreenTouch or event is InputEventMouseButton:
		var pressed: bool = event.is_pressed()
		_dragging = pressed
		_set_vector(vector_at((event as InputEventMouse).position if event is InputEventMouse
				else (event as InputEventScreenTouch).position) if pressed else Vector2.ZERO)
		accept_event()
	elif _dragging and (event is InputEventScreenDrag or event is InputEventMouseMotion):
		var point: Vector2 = (event as InputEventScreenDrag).position if event is InputEventScreenDrag \
				else (event as InputEventMouseMotion).position
		_set_vector(vector_at(point))
		accept_event()


func _set_vector(value: Vector2) -> void:
	vector = value
	if _knob != null:
		_knob.position = size / 2.0 + Vector2(value.x, -value.y) * radius() * 0.6 - _knob.size / 2.0
	moved.emit(vector)
