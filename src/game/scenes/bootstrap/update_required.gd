extends Control
## "Cần cập nhật" (F133 T023, F133-AC-12, TC-AUTH-066; F061-AC-01/27; replaces ForceUpdate): Mây says there is a new
## version, with one big button whose meaning UpdateService chooses for the platform — "Cập nhật ngay" (store page),
## "Tải lại trang" (Web) or, on a school tablet / without a store link, "Nhờ thầy cô cập nhật" + "Thử lại" (runs the
## version check again). The minimum version is a small line for the teacher. No login from here; the stored session
## is kept. Strings from the translation files, style from the Theme (R1, R32): no logic beyond showing the action.

const BODY_KEYS: Dictionary = {
	UpdateService.Action.OPEN_STORE: "ui.update.body_store",
	UpdateService.Action.RELOAD_PAGE: "ui.update.body_reload",
	UpdateService.Action.ASK_TEACHER: "ui.update.body_teacher",
}
const BUTTON_KEYS: Dictionary = {
	UpdateService.Action.OPEN_STORE: "ui.update.button_store",
	UpdateService.Action.RELOAD_PAGE: "ui.update.button_reload",
	UpdateService.Action.ASK_TEACHER: "ui.update.button_retry",
}

var _action: int = UpdateService.Action.ASK_TEACHER

@onready var _body: Label = %Body
@onready var _action_button: Button = %Action
@onready var _version: Label = %Version



func _ready() -> void:
	_action_button.pressed.connect(_on_action_pressed)
	EventBus.locale_changed.connect(_on_locale_changed)
	_action = int(AuthService.update_action()["action"])
	_render()
	_action_button.grab_focus.call_deferred()


func _render() -> void:
	_body.text = tr(String(BODY_KEYS[_action]))
	_action_button.text = tr(String(BUTTON_KEYS[_action]))
	var version: String = AuthService.min_client_version
	_version.visible = not version.is_empty()
	_version.text = tr("ui.update.version").format({"version": version})


## Shows another action (UiGallery / review captures of each platform; the flow itself uses UpdateService).
func show_action(action: int) -> void:
	_action = action if BODY_KEYS.has(action) else UpdateService.Action.ASK_TEACHER
	_render()


func _on_action_pressed() -> void:
	if _action == UpdateService.Action.ASK_TEACHER:
		_action_button.disabled = true
		AuthService.start()
		return
	AuthService.open_store()


func _on_locale_changed(_locale: String) -> void:
	_render()
