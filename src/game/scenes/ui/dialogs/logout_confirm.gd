extends PanelContainer
## LogoutConfirm (F061-AC-23, S01 §9): "Em muốn đăng xuất? Lần sau em sẽ cần nhập mật khẩu." Confirm →
## AuthService.logout() (`endGameSession` → `gameLogout` → erase the stored session); Cancel hides the panel.

signal dismissed()

@onready var _cancel: Button = %Cancel
@onready var _confirm: Button = %Confirm


func _ready() -> void:
	_cancel.pressed.connect(_on_cancel_pressed)
	_confirm.pressed.connect(_on_confirm_pressed)


func open() -> void:
	_confirm.disabled = false
	show()
	_cancel.grab_focus()


func _on_cancel_pressed() -> void:
	hide()
	dismissed.emit()


func _on_confirm_pressed() -> void:
	_confirm.disabled = true
	AuthService.logout()
