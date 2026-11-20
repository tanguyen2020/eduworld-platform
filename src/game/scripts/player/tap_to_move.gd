class_name TapToMove
extends Node
## "Bấm-để-đi" (STUDENT_GAME_UX §4, S03 §10, F062-AC-02): only for grade band `g1_3`. A tap/click on the ground (not
## on the HUD — the HUD consumes its own input first) sets a destination; the controller walks there. Dragging is
## camera/joystick input, not a tap.

signal target_set(position: Vector3)
signal target_cleared()

const ENABLED_BANDS: PackedStringArray = ["g1_3"]
const RAY_LENGTH: float = 200.0
## A press that moves more than this (px) before release is a drag, not a tap.
const TAP_MAX_TRAVEL_PX: float = 16.0
const ARRIVE_DISTANCE: float = 0.4

@export var camera: Camera3D = null

var enabled: bool = false
var has_target: bool = false
var target: Vector3 = Vector3.ZERO
var _press_position: Vector2 = Vector2.ZERO
var _pressed: bool = false


static func is_enabled_for(band: String) -> bool:
	return band in ENABLED_BANDS


func set_band(band: String) -> void:
	enabled = is_enabled_for(band)
	if not enabled:
		clear()


func set_target(point: Vector3) -> void:
	if not enabled:
		return
	target = point
	has_target = true
	target_set.emit(point)


func clear() -> void:
	if has_target:
		has_target = false
		target_cleared.emit()


## Horizontal direction (unit or zero) from `from` to the target; clears the target on arrival.
func direction_from(from: Vector3) -> Vector3:
	if not has_target:
		return Vector3.ZERO
	var delta: Vector3 = Vector3(target.x - from.x, 0.0, target.z - from.z)
	if delta.length() <= ARRIVE_DISTANCE:
		clear()
		return Vector3.ZERO
	return delta.normalized()


func _unhandled_input(event: InputEvent) -> void:
	if not enabled:
		return
	var pressed: bool = false
	var released: bool = false
	var position: Vector2 = Vector2.ZERO
	if event is InputEventScreenTouch:
		pressed = (event as InputEventScreenTouch).pressed
		released = not pressed
		position = (event as InputEventScreenTouch).position
	elif event is InputEventMouseButton and (event as InputEventMouseButton).button_index == MOUSE_BUTTON_LEFT:
		pressed = (event as InputEventMouseButton).pressed
		released = not pressed
		position = (event as InputEventMouseButton).position
	else:
		return
	if pressed:
		_pressed = true
		_press_position = position
	elif released and _pressed:
		_pressed = false
		if position.distance_to(_press_position) <= TAP_MAX_TRAVEL_PX:
			_tap(position)


func _tap(screen_position: Vector2) -> void:
	if camera == null or not camera.is_inside_tree():
		return
	var from: Vector3 = camera.project_ray_origin(screen_position)
	var to: Vector3 = from + camera.project_ray_normal(screen_position) * RAY_LENGTH
	var query: PhysicsRayQueryParameters3D = PhysicsRayQueryParameters3D.create(from, to)
	var hit: Dictionary = camera.get_world_3d().direct_space_state.intersect_ray(query)
	if not hit.is_empty():
		set_target(hit["position"] as Vector3)
