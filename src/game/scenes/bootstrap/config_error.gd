extends Control
## ConfigError scene (F060-AC-03 edge case): translated message, the error code for support staff, Quit button.

@onready var _detail: Label = %Detail
@onready var _quit: Button = %Quit


func _ready() -> void:
	_detail.text = tr("ui.config_error.detail").format({"code": Config.error_code})
	_quit.pressed.connect(_on_quit_pressed)
	EventBus.locale_changed.connect(_on_locale_changed)


func _on_locale_changed(_locale: String) -> void:
	_detail.text = tr("ui.config_error.detail").format({"code": Config.error_code})


func _on_quit_pressed() -> void:
	get_tree().quit(1)
