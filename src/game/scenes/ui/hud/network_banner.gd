extends PanelContainer
## Network banner of the HUD (GODOT_ARCHITECTURE §9, F092-AC-08, AC-17, AC-18): "Đang kết nối lại…" while the realtime
## connection is coming back (hidden for the quiet reconnect after `system.server.draining`), a degraded indicator on
## `system.service.degraded`, and short hub notices (room full, join denied, emote limits). Reads RealtimeClient
## signals only; texts are `ui.realtime.*`/`ui.hub.*` keys translated by the Label (a locale change re-renders them
## at once); style from the generated Theme (`HudPanel`, `HudLabel`).

## Seconds a hub notice stays visible.
const NOTICE_SEC: float = 4.0
const KEY_RECONNECTING: String = "ui.realtime.reconnecting"
const KEY_DEGRADED: String = "ui.realtime.degraded"

var client: GameRealtimeClient = null
var notice_key: String = ""
## F111: format args of the notice ("Em đã đến chỗ {nickname}").
var notice_args: Dictionary = {}
var _notice_left: float = 0.0

@onready var _status: Label = %NetworkStatus
@onready var _notice: Label = %HubNotice


func _ready() -> void:
	bind(get_node_or_null("/root/RealtimeClient") as GameRealtimeClient)


func _exit_tree() -> void:
	_unbind()


## Follows `realtime` (the autoload in the app, a test instance in scene tests).
func bind(realtime: GameRealtimeClient) -> void:
	_unbind()
	client = realtime
	if client != null:
		client.connection_state_changed.connect(_on_state_changed)
		client.service_degraded.connect(_on_degraded)
	_render()


func show_hub_notice(key: String, args: Dictionary = {}) -> void:
	notice_key = key
	notice_args = args
	_notice_left = NOTICE_SEC
	_render()


func status_key() -> String:
	if client == null:
		return ""
	if client.is_reconnecting():
		return KEY_RECONNECTING
	if not client.degraded_features.is_empty():
		return KEY_DEGRADED
	return ""


func _process(delta: float) -> void:
	if _notice_left > 0.0:
		_notice_left -= delta
		if _notice_left <= 0.0:
			notice_key = ""
			_render()


func _render() -> void:
	if _status == null:
		return
	var key: String = status_key()
	_status.text = key
	_status.visible = not key.is_empty()
	_notice.text = tr(notice_key).format(notice_args) if not notice_args.is_empty() else notice_key
	_notice.visible = not notice_key.is_empty()
	visible = _status.visible or _notice.visible


func _on_state_changed(_state: int) -> void:
	_render()


func _on_degraded(_features: PackedStringArray) -> void:
	_render()


func _unbind() -> void:
	if client == null:
		return
	if client.connection_state_changed.is_connected(_on_state_changed):
		client.connection_state_changed.disconnect(_on_state_changed)
	if client.service_degraded.is_connected(_on_degraded):
		client.service_degraded.disconnect(_on_degraded)
	client = null
