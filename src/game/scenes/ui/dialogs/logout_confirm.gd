extends Control
## LogoutConfirm (F061-AC-23, S01 §9; look F114-AC-13): a dialog over the Settings window with the nested scrim —
## yellow title bar "ĐĂNG XUẤT?", "Bạn muốn đăng xuất khỏi EduWorld?" + "Tiến độ của bạn đã được lưu rồi nhé.", a neutral
## "Hủy" (icon ✕, focused by default: the safe choice, AGENTS R19) and the red "Đăng xuất" (DangerButton, door icon).
## Confirm → AuthService.logout() (`endGameSession` → `gameLogout` → erase the stored session); Cancel hides the dialog.

signal dismissed()

var open_tween: Tween = null

@onready var _dialog: PanelContainer = %Dialog
@onready var _cancel: Button = %Cancel
@onready var _confirm: Button = %Confirm


func _ready() -> void:
	_cancel.pressed.connect(cancel)
	_confirm.pressed.connect(_on_confirm_pressed)
	_dialog.custom_minimum_size.x = float(UiTokens.COMPONENT_FORM_CARD_WIDTH)
	UiBuild.size_buttons(_dialog)


func open() -> void:
	_confirm.disabled = false
	show()
	_cancel.grab_focus.call_deferred()
	open_tween = UiMotion.pop_in(_dialog, Settings.reduce_motion)


## "Hủy" / Esc / Back: closes the dialog, nothing else happens.
func cancel() -> void:
	hide()
	dismissed.emit()


func _on_confirm_pressed() -> void:
	_confirm.disabled = true
	AuthService.logout()
