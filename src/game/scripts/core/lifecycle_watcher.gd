class_name LifecycleWatcher
extends Node
## App lifecycle for the session (GODOT_ARCHITECTURE §5, F061-AC-23/25/26/32): window close request, mobile
## pause/resume and idle time on a shared device. It only reports; AuthService decides (personal device: close =
## `:end` only and keep the stored session; shared device: close or idle `shared_device_idle_logout` = log out).

signal close_requested()
signal app_paused()
signal app_resumed(paused_sec: float)
signal idle_limit_reached()

## CONFIG_DEFAULTS `shared_device_idle_logout` = 30 minutes [Proposed].
const SHARED_DEVICE_IDLE_LOGOUT_SEC: float = 30.0 * 60.0
const IDLE_CHECK_INTERVAL_SEC: float = 1.0

var clock: ClientClock = ClientClock.new()
var idle_limit_sec: float = SHARED_DEVICE_IDLE_LOGOUT_SEC
## Idle logout runs only on a shared device with a signed-in student.
var idle_enabled: bool = false
var _last_input_msec: int = 0
var _paused_at_msec: int = -1
var _since_check: float = 0.0


func _ready() -> void:
	touch()
	# The close request is handled (end session / log out) before the app quits.
	get_tree().set_auto_accept_quit(false)


func _notification(what: int) -> void:
	match what:
		NOTIFICATION_WM_CLOSE_REQUEST:
			close_requested.emit()
		NOTIFICATION_APPLICATION_PAUSED:
			_paused_at_msec = clock.now_msec()
			app_paused.emit()
		NOTIFICATION_APPLICATION_RESUMED:
			var paused: float = 0.0
			if _paused_at_msec >= 0:
				paused = float(clock.now_msec() - _paused_at_msec) / 1000.0
			_paused_at_msec = -1
			touch()
			app_resumed.emit(paused)


func _input(event: InputEvent) -> void:
	if event is InputEventKey or event is InputEventMouseButton or event is InputEventScreenTouch \
			or event is InputEventJoypadButton or event is InputEventMouseMotion or event is InputEventScreenDrag:
		touch()


func _process(delta: float) -> void:
	_since_check += delta
	if _since_check < IDLE_CHECK_INTERVAL_SEC:
		return
	_since_check = 0.0
	check_idle()


func touch() -> void:
	_last_input_msec = clock.now_msec()


## Emits `idle_limit_reached` once per idle period; returns true when it did.
func check_idle() -> bool:
	if not idle_enabled:
		return false
	if float(clock.now_msec() - _last_input_msec) / 1000.0 < idle_limit_sec:
		return false
	touch()
	idle_limit_reached.emit()
	return true
