class_name CameraRig
extends Node3D
## Third-person camera of the local player (F062-AC-02): yaw on this node, pitch and distance on the SpringArm3D
## child (collision-aware). PC: hold the right mouse button (`camera_rotate`) and move to orbit, wheel to zoom
## (`camera_zoom_in/out`). Touch: a one-finger drag outside the joystick/HUD orbits ("vuốt camera"). Movement is
## relative to `yaw()`. Reduced motion (Settings) disables nothing here: there is no camera shake in F062.

@export var spring_arm: SpringArm3D = null
@export var mouse_sensitivity: float = 0.005
@export var touch_sensitivity: float = 0.008
@export var pitch_min_deg: float = -60.0
@export var pitch_max_deg: float = -5.0
@export var distance_min: float = 3.0
@export var distance_max: float = 9.0
@export var zoom_step: float = 0.5

var _rotating: bool = false


func yaw() -> float:
	return rotation.y


func set_yaw(value: float) -> void:
	rotation.y = wrapf(value, -PI, PI)


func pitch() -> float:
	return spring_arm.rotation.x if spring_arm != null else 0.0


## Rotates by a screen delta in pixels (`sensitivity` rad/px); pitch is clamped.
func orbit(delta_px: Vector2, sensitivity: float) -> void:
	set_yaw(rotation.y - delta_px.x * sensitivity)
	if spring_arm != null:
		spring_arm.rotation.x = clampf(spring_arm.rotation.x - delta_px.y * sensitivity,
				deg_to_rad(pitch_min_deg), deg_to_rad(pitch_max_deg))


func zoom(steps: float) -> void:
	if spring_arm != null:
		spring_arm.spring_length = clampf(spring_arm.spring_length + steps * zoom_step, distance_min, distance_max)


func _unhandled_input(event: InputEvent) -> void:
	if event.is_action_pressed("camera_rotate"):
		_rotating = true
	elif event.is_action_released("camera_rotate"):
		_rotating = false
	elif event.is_action_pressed("camera_zoom_in"):
		zoom(-1.0)
	elif event.is_action_pressed("camera_zoom_out"):
		zoom(1.0)
	elif event is InputEventMouseMotion and _rotating:
		orbit((event as InputEventMouseMotion).relative, mouse_sensitivity)
	elif event is InputEventScreenDrag:
		orbit((event as InputEventScreenDrag).relative, touch_sensitivity)
