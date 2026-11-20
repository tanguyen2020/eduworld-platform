class_name PlayTimeService
extends RefCounted
## Break reminder (FR-GAME-012, CHILD_SAFETY, GODOT_ARCHITECTURE §10, F062-AC-21/22). The threshold is the server
## value `PlayerBootstrap.policies.play_time.break_reminder_minutes` (CONFIG_DEFAULTS `break_reminder`, decided per
## platform/tenant); the client only counts continuous play time to show a gentle, closable reminder — it never
## blocks play (blocking outside the access window is the server's `403 ACCESS_OUTSIDE_ALLOWED_HOURS`).
## Reminders repeat every `break_reminder_minutes` of play. While a hold is active (timed exam, mandatory dialog) a
## due reminder waits and is shown once the last hold is released; only shown reminders emit `break_reminder_shown`
## (that signal feeds the `break_reminder_shown` telemetry, F062-AC-24).

signal break_reminder_shown(reminder_no: int, minutes_played: int)

## `TelemetryEvent.reminder_no` maximum (1–20) and `minutes_played` maximum (0–1440).
const MAX_REMINDERS: int = 20
const MAX_MINUTES_PLAYED: int = 1440
const SECONDS_PER_MINUTE: float = 60.0

## 0 = no reminder (policy missing).
var interval_minutes: int = 0
var played_sec: float = 0.0
var reminders_shown: int = 0
var pending: bool = false
var _next_due_sec: float = 0.0
var _holds: Dictionary = {}


## Reads `policies.play_time.break_reminder_minutes` of the bootstrap and restarts the count.
func configure(policies: Dictionary) -> void:
	var play_time: Variant = policies.get("play_time")
	var minutes: Variant = (play_time as Dictionary).get("break_reminder_minutes") if play_time is Dictionary else null
	interval_minutes = maxi(0, int(minutes)) if (minutes is int or minutes is float) else 0
	reset()


func reset() -> void:
	played_sec = 0.0
	reminders_shown = 0
	pending = false
	_holds.clear()
	_next_due_sec = float(interval_minutes) * SECONDS_PER_MINUTE


func is_enabled() -> bool:
	return interval_minutes > 0


func is_held() -> bool:
	return not _holds.is_empty()


## Adds continuous play time (called every frame while in the world; not while the app is in the background).
func tick(delta_sec: float) -> void:
	if not is_enabled() or delta_sec <= 0.0:
		return
	played_sec += delta_sec
	if pending or reminders_shown >= MAX_REMINDERS or played_sec < _next_due_sec:
		return
	pending = true
	if not is_held():
		_show()


## F062-AC-22: a timed exam or a mandatory dialog defers the reminder until every hold is released.
func hold(reason: StringName) -> void:
	_holds[reason] = true


func release(reason: StringName) -> void:
	_holds.erase(reason)
	if pending and not is_held():
		_show()


func minutes_played() -> int:
	return clampi(int(played_sec / SECONDS_PER_MINUTE), 0, MAX_MINUTES_PLAYED)


func _show() -> void:
	pending = false
	reminders_shown += 1
	_next_due_sec = played_sec + float(interval_minutes) * SECONDS_PER_MINUTE
	break_reminder_shown.emit(reminders_shown, minutes_played())
