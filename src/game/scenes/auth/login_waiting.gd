extends Control
## LoginWaiting (`429 LOGIN_QUEUE_BUSY`, F061-AC-06, FR-AUTH-027): "Đang vào lớp… nhiều bạn đang vào cùng lúc";
## AuthService retries after `Retry-After` + jitter by itself; "Hủy" stops the retries and wipes the password.

@onready var _eta: Label = %Eta
@onready var _cancel: Button = %Cancel


func _ready() -> void:
	_eta.visible = false
	_cancel.pressed.connect(_on_cancel_pressed)
	AuthService.queue_waited.connect(_on_queue_waited)


func _exit_tree() -> void:
	if AuthService.queue_waited.is_connected(_on_queue_waited):
		AuthService.queue_waited.disconnect(_on_queue_waited)


func _on_queue_waited(seconds: float) -> void:
	_eta.visible = true
	_eta.text = tr("ui.login.queue_eta").format({"seconds": int(ceilf(seconds))})


func _on_cancel_pressed() -> void:
	AuthService.cancel_login()
