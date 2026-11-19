extends Control
## S01b "Tiếp tục là <nickname>?" (F061-AC-35, `resume_confirm_screen`): shown after a successful resume, before the
## game session is created. Only the game nickname is shown (never the real name, S01 §14). Focus starts on
## "Tiếp tục"; there is no automatic continue.

@onready var _title: Label = %Title
@onready var _continue: Button = %Continue
@onready var _not_me: Button = %NotMe


func _ready() -> void:
	_continue.pressed.connect(_on_continue_pressed)
	_not_me.pressed.connect(_on_not_me_pressed)
	EventBus.locale_changed.connect(_on_locale_changed)
	_render()
	_continue.grab_focus.call_deferred()


func _render() -> void:
	if AuthService.nickname.is_empty():
		_title.text = tr("ui.login.resume_confirm.title_generic")
	else:
		_title.text = tr("ui.login.resume_confirm.title").format({"nickname": AuthService.nickname})


func _on_continue_pressed() -> void:
	_set_busy()
	AuthService.continue_resume()


func _on_not_me_pressed() -> void:
	_set_busy()
	AuthService.switch_account()


func _set_busy() -> void:
	_continue.disabled = true
	_not_me.disabled = true


func _on_locale_changed(_locale: String) -> void:
	_render()
