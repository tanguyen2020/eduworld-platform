extends Control
## Cài đặt in the world (F062): language vi/en (applied at once, F062-AC-06), graphics quality (T027) and Tài khoản →
## "Đăng xuất" with LogoutConfirm (F061-AC-23, moved here from the removed WorldPlaceholder). UI only.

signal closed()

const LOCALE_VI: String = "vi-VN"
const LOCALE_EN: String = "en-US"

@onready var _close: Button = %Close
@onready var _lang_vi: Button = %LangVi
@onready var _lang_en: Button = %LangEn
@onready var _logout: Button = %Logout
@onready var _logout_confirm: PanelContainer = %LogoutConfirm


func _ready() -> void:
	_close.pressed.connect(close)
	_lang_vi.pressed.connect(_on_language_pressed.bind(LOCALE_VI))
	_lang_en.pressed.connect(_on_language_pressed.bind(LOCALE_EN))
	_logout.pressed.connect(_on_logout_pressed)


func open(_args: Dictionary = {}) -> void:
	show()


func close() -> void:
	_logout_confirm.hide()
	hide()
	closed.emit()


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed("ui_cancel"):
		close()
		get_viewport().set_input_as_handled()


func _on_language_pressed(locale: String) -> void:
	Settings.set_locale(locale)


func _on_logout_pressed() -> void:
	_logout_confirm.call("open")
