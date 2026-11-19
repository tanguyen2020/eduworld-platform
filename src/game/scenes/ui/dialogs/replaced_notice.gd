extends Control
## ReplacedNotice (F061-AC-22): "Em đã được đăng xuất vì tài khoản vừa đăng nhập trên thiết bị khác…". Shown after
## a heartbeat `409`/`404`; never closes by itself; tokens are already erased and nothing reconnects.

@onready var _ok: Button = %Ok


func _ready() -> void:
	_ok.pressed.connect(_on_ok_pressed)
	_ok.grab_focus.call_deferred()


func _on_ok_pressed() -> void:
	AuthService.back_to_login()
