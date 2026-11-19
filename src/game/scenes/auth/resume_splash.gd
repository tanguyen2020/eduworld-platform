extends Control
## ResumeSplash (STUDENT_GAME_UX_CORE S01 `ResumingSession`, F061-AC-29/31): "Đang vào lớp…" while the stored
## session is resumed, "Đang kết nối…" while the network is retried; after OTHER_ACCOUNT_DELAY_SEC the secondary
## button "Đăng nhập bằng tài khoản khác" appears. Never shows the form.

## S01 §8: the secondary button appears after 5 s.
const OTHER_ACCOUNT_DELAY_SEC: float = 5.0

@onready var _status: Label = %Status
@onready var _other_account: Button = %OtherAccount


func _ready() -> void:
	_other_account.visible = false
	_other_account.pressed.connect(_on_other_account_pressed)
	AuthService.connection_retried.connect(_on_connection_retried)
	_render()
	await get_tree().create_timer(OTHER_ACCOUNT_DELAY_SEC).timeout
	if is_inside_tree():
		_other_account.visible = true


func _exit_tree() -> void:
	if AuthService.connection_retried.is_connected(_on_connection_retried):
		AuthService.connection_retried.disconnect(_on_connection_retried)


func _render() -> void:
	_status.text = tr("ui.login.connecting") if AuthService.connecting else tr("ui.login.resuming")


func _on_connection_retried(_attempt: int) -> void:
	_render()


func _on_other_account_pressed() -> void:
	_other_account.disabled = true
	AuthService.switch_account()
