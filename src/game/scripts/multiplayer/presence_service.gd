class_name PresenceService
extends RefCounted
## Own presence over the realtime connection (MULTIPLAYER_GAMEPLAY §5, AsyncAPI `presence.status.set`; F092-AC-09):
## `online` in the hub/menus, `in_activity` while an activity screen is open (the world forwards the UI screen
## signals), `in_match` in a match/boss (set by later features), `away` after AWAY_AFTER_MS without input. Sent only
## when the status changes and again after a new connection (the gateway starts every connection as `online`).
## `offline` is never sent: the gateway sets it when no connection is left (F091-AC-22).

const TYPE_SET: String = "presence.status.set"
const ONLINE: String = "online"
const IN_ACTIVITY: String = "in_activity"
const IN_MATCH: String = "in_match"
const AWAY: String = "away"
## F092-AC-09 / MULTIPLAYER_GAMEPLAY §5: no input for 5 minutes → away.
const AWAY_AFTER_MS: int = 300000
## UI screens that mean an activity (`activity` of AsyncAPI: idle, learning, quest, battle, party_lobby, exam).
const SCREEN_STATUS: Dictionary = {
	&"activity_host": [IN_ACTIVITY, "learning"],
	&"exam_lobby": [IN_ACTIVITY, "exam"],
}

var realtime: GameRealtimeClient = null
var clock: ClientClock = null
## Status of the current activity (without idle).
var activity_status: String = ONLINE
var activity: String = "idle"
## Last status sent (or set by the gateway on connect).
var sent_status: String = ""
var away: bool = false
var _last_input_ms: int = 0
var _open_screens: Array[StringName] = []


func _init(client: GameRealtimeClient, time_source: ClientClock) -> void:
	realtime = client
	clock = time_source
	_last_input_ms = _now()
	if realtime != null:
		realtime.connection_state_changed.connect(_on_connection_state_changed)


func current_status() -> String:
	return AWAY if away else activity_status


func screen_opened(screen_id: StringName) -> void:
	if SCREEN_STATUS.has(screen_id) and not screen_id in _open_screens:
		_open_screens.append(screen_id)
	_refresh_activity()


func screen_closed(screen_id: StringName) -> void:
	_open_screens.erase(screen_id)
	_refresh_activity()


## Sets a status owned by another feature (`in_match` for a battle/boss) or back to `online`.
func set_activity(status: String, activity_name: String = "idle") -> void:
	if not status in [ONLINE, IN_ACTIVITY, IN_MATCH]:
		return
	activity_status = status
	activity = activity_name
	_publish()


## Any player input: leaves `away`.
func touch() -> void:
	_last_input_ms = _now()
	if away:
		away = false
		_publish()


## Per-frame check of the idle timer.
func tick() -> void:
	if not away and _now() - _last_input_ms >= AWAY_AFTER_MS:
		away = true
		_publish()


func _refresh_activity() -> void:
	if _open_screens.is_empty():
		set_activity(ONLINE, "idle")
		return
	var entry: Array = SCREEN_STATUS[_open_screens[_open_screens.size() - 1]] as Array
	set_activity(String(entry[0]), String(entry[1]))


func _publish() -> void:
	var status: String = current_status()
	if status == sent_status or realtime == null or not realtime.can_write():
		return
	sent_status = status
	var payload: Dictionary = {"status": status}
	if status != AWAY:
		payload["activity"] = activity
	var result: CommandResult = await realtime.send_command(TYPE_SET, payload)
	if not result.ok and sent_status == status:
		sent_status = ""


func _on_connection_state_changed(next: int) -> void:
	if next in [GameRealtimeClient.State.WELCOMED, GameRealtimeClient.State.RESUMING, GameRealtimeClient.State.RESYNC]:
		# Every new connection starts as `online` on the gateway (F091-AC-21).
		sent_status = ONLINE
	elif next == GameRealtimeClient.State.ONLINE:
		_publish()


func _now() -> int:
	return clock.now_msec() if clock != null else Time.get_ticks_msec()
