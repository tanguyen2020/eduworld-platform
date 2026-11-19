extends Control
## CurfewNotice (ADR-0078, F061-AC-36, AC-UX-0115): "Đã đến giờ nghỉ rồi! EduWorld mở lại lúc {time}." No form;
## the stored session is kept so the student enters directly after the curfew.

@onready var _body: Label = %Body
@onready var _close: Button = %Close


func _ready() -> void:
	_close.pressed.connect(_on_close_pressed)
	EventBus.locale_changed.connect(_on_locale_changed)
	_render()


func _render() -> void:
	_body.text = tr("ui.login.curfew").format({"time": AuthService.reopen_text})


func _on_close_pressed() -> void:
	_close.disabled = true
	AuthService.close_game()


func _on_locale_changed(_locale: String) -> void:
	_render()
