class_name ToggleSwitch
extends CheckButton
## F114 (DESIGN_SYSTEM §5.1): on/off switch on a CheckButton — the generated `ToggleSwitch` Theme variant draws an ink
## outlined track with a white knob (on = accent track, knob right; off = grey track, knob left) and the switch always
## writes its state as a word beside it ("Bật" / "Tắt"), so the state is never colour or position only (NFR-A11Y-001).
## `setting_key` names the setting for screen readers. UI only: the owner screen applies the value through Settings.

const ON_KEY: String = "ui.settings.on"
const OFF_KEY: String = "ui.settings.off"

## Translation key of the setting this switch changes (accessibility name: "Tương phản cao: Bật").
@export var setting_key: String = ""


func _init() -> void:
	theme_type_variation = &"ToggleSwitch"


func _ready() -> void:
	toggled.connect(_on_toggled)
	_render()


func _notification(what: int) -> void:
	if what == NOTIFICATION_TRANSLATION_CHANGED and is_node_ready():
		_render()


## Shows `on` without emitting `toggled` (the screen renders the saved value).
func show_state(on: bool) -> void:
	set_pressed_no_signal(on)
	_render()


func _on_toggled(_on: bool) -> void:
	_render()


func _render() -> void:
	text = ON_KEY if button_pressed else OFF_KEY
	var state: String = tr(text)
	accessibility_name = state if setting_key.is_empty() else "%s: %s" % [tr(setting_key), state]
