extends Control
## TakeoverConfirm (F061-AC-21, AC-UX-0103): `createGameSession` answered `409 ACTIVE_SESSION_ON_OTHER_DEVICE`.
## Shows the other device (label or platform, last activity), never closes by itself; the safe button
## "Ở lại máy kia" is on the left. Neither button asks for the password.

## Platform names for `other_device.platform` when the server sent no `device_label`.
const PLATFORM_KEYS: Dictionary = {
	"windows": "ui.login.platform.windows",
	"android": "ui.login.platform.android",
	"ios": "ui.login.platform.ios",
	"web": "ui.login.platform.web",
	"macos": "ui.login.platform.macos",
	"linux": "ui.login.platform.linux",
}

@onready var _body: Label = %Body
@onready var _stay: Button = %Stay
@onready var _continue: Button = %Continue


func _ready() -> void:
	_stay.pressed.connect(_on_stay_pressed)
	_continue.pressed.connect(_on_continue_pressed)
	EventBus.locale_changed.connect(_on_locale_changed)
	_render()
	_stay.grab_focus.call_deferred()


func _render() -> void:
	var device: ApiProblem.OtherDevice = AuthService.other_device
	var label: String = ""
	var time_text: String = ""
	if device != null:
		label = device.device_label if not device.device_label.is_empty() \
				else tr(String(PLATFORM_KEYS.get(device.platform, "ui.login.platform.unknown")))
		var unix: float = CurfewWatcher.parse_iso_unix(device.last_seen_at)
		if unix > 0.0:
			time_text = LocaleFormat.time(LocaleFormat.from_unix(int(unix)))
	_body.text = tr("ui.login.takeover.body").format({"device": label, "time": time_text})


func _on_stay_pressed() -> void:
	_set_busy()
	AuthService.decline_takeover()


func _on_continue_pressed() -> void:
	_set_busy()
	AuthService.confirm_takeover()


func _set_busy() -> void:
	_stay.disabled = true
	_continue.disabled = true


func _on_locale_changed(_locale: String) -> void:
	_render()
