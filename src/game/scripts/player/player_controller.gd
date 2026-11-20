class_name PlayerController
extends CharacterBody3D
## Local player character (FR-GAME-003, GODOT_ARCHITECTURE §6, F062-AC-02/03): moves with WASD/arrows (or the HUD
## joystick via `set_move_input`) relative to the camera, jumps, and — for grade band `g1_3` — walks to a tapped
## point (TapToMove). Animation follows velocity through AnimDriver. Only local movement: no position sync until
## F092, no business logic, no service calls (AGENTS R1). Keyboard moves at run speed; the joystick scales speed.

@export var walk_speed: float = 2.5
@export var run_speed: float = 5.0
@export var jump_velocity: float = 4.5
@export var turn_speed: float = 10.0
@export var camera_rig: CameraRig = null
@export var anim_driver: AnimDriver = null
@export var tap_to_move: TapToMove = null
@export var avatar: Node3D = null

## Analog move input from the HUD joystick (x = right, y = forward), length ≤ 1.
var move_input: Vector2 = Vector2.ZERO
## Off while a panel owns the input (journal, dialogs).
var input_enabled: bool = true
var _jump_requested: bool = false
var _gravity: float = 9.8


func _ready() -> void:
	add_to_group(Interactable.PLAYER_GROUP)
	_gravity = float(ProjectSettings.get_setting("physics/3d/default_gravity", 9.8))
	if tap_to_move != null:
		tap_to_move.set_band(AppState.grade_band)
		AppState.grade_band_changed.connect(tap_to_move.set_band)


func _exit_tree() -> void:
	if tap_to_move != null and AppState.grade_band_changed.is_connected(tap_to_move.set_band):
		AppState.grade_band_changed.disconnect(tap_to_move.set_band)


func set_move_input(value: Vector2) -> void:
	move_input = value.limit_length(1.0)
	if not move_input.is_zero_approx() and tap_to_move != null:
		tap_to_move.clear()


func request_jump() -> void:
	_jump_requested = true


## Turns avatar and camera to `yaw` (spawn facing).
func face(yaw: float) -> void:
	if avatar != null:
		avatar.rotation.y = yaw
	if camera_rig != null:
		camera_rig.set_yaw(yaw)


## Keyboard (actions) + joystick input as a 2D vector (x right, y forward).
func read_input() -> Vector2:
	if not input_enabled:
		return Vector2.ZERO
	var keys: Vector2 = Input.get_vector("move_left", "move_right", "move_back", "move_forward")
	return keys if not keys.is_zero_approx() else move_input


## F062-AC-02: world-space horizontal velocity for an input vector, relative to the camera yaw. The keyboard (full
## deflection) runs; the joystick scales between walk and run speed.
func desired_velocity(input: Vector2, yaw: float) -> Vector3:
	if input.is_zero_approx():
		return Vector3.ZERO
	var forward: Vector3 = Vector3(0.0, 0.0, -1.0).rotated(Vector3.UP, yaw)
	var right: Vector3 = Vector3(1.0, 0.0, 0.0).rotated(Vector3.UP, yaw)
	var dir: Vector3 = (right * input.x + forward * input.y)
	var amount: float = minf(input.length(), 1.0)
	var speed: float = lerpf(walk_speed, run_speed, clampf((amount - 0.5) * 2.0, 0.0, 1.0))
	return dir.normalized() * speed


func _unhandled_input(event: InputEvent) -> void:
	if input_enabled and event.is_action_pressed("jump"):
		request_jump()


func _physics_process(delta: float) -> void:
	step(delta)


## One physics step (called by `_physics_process`; tests call it directly).
func step(delta: float) -> void:
	var yaw: float = camera_rig.yaw() if camera_rig != null else 0.0
	var input: Vector2 = read_input()
	var horizontal: Vector3 = desired_velocity(input, yaw)
	if input.is_zero_approx() and tap_to_move != null and tap_to_move.has_target:
		horizontal = tap_to_move.direction_from(global_position) * walk_speed
	elif not input.is_zero_approx() and tap_to_move != null:
		tap_to_move.clear()
	velocity.x = horizontal.x
	velocity.z = horizontal.z
	if is_on_floor():
		if _jump_requested:
			velocity.y = jump_velocity
	else:
		velocity.y -= _gravity * delta
	_jump_requested = false
	move_and_slide()
	if avatar != null and not horizontal.is_zero_approx():
		var target_yaw: float = atan2(-horizontal.x, -horizontal.z)
		avatar.rotation.y = lerp_angle(avatar.rotation.y, target_yaw, clampf(turn_speed * delta, 0.0, 1.0))
	if anim_driver != null:
		anim_driver.update(velocity, is_on_floor())
