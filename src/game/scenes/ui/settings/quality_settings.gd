extends VBoxContainer
## Cài đặt → Đồ họa (PERFORMANCE_BUDGET §5–§6, F062 T027): the student picks the preset and overrides shadows and
## anti-aliasing; choices are saved per device by QualityManager (`settings.cfg [quality]`, never sent to the server).
## When the dynamic resolution controller stays at its minimum, QualityManager only suggests a lower preset here — it
## never switches by itself (F062-AC-16).

const PRESET_BUTTONS: Dictionary = {"low": "PresetLow", "medium": "PresetMedium", "high": "PresetHigh"}
## F096-AC-13: the chosen preset is a selected `ChoiceChip` with a check icon (not colour only).
const ICON_CHECK: Texture2D = preload("res://resources/ui/icons/check.svg")

var suggested_preset: String = ""

@onready var _shadows: CheckButton = %Shadows
@onready var _antialiasing: CheckButton = %Antialiasing
@onready var _suggestion: Label = %Suggestion


func _ready() -> void:
	var group: ButtonGroup = ButtonGroup.new()
	for preset: String in PRESET_BUTTONS.keys():
		var button: Button = preset_button(preset)
		button.toggle_mode = true
		button.button_group = group
		button.pressed.connect(_on_preset_pressed.bind(preset))
	_shadows.toggled.connect(_on_shadows_toggled)
	_antialiasing.toggled.connect(_on_antialiasing_toggled)
	QualityManager.quality_changed.connect(_on_quality_changed)
	QualityManager.preset_suggested.connect(_on_preset_suggested)
	_render()


func _exit_tree() -> void:
	if QualityManager.quality_changed.is_connected(_on_quality_changed):
		QualityManager.quality_changed.disconnect(_on_quality_changed)
	if QualityManager.preset_suggested.is_connected(_on_preset_suggested):
		QualityManager.preset_suggested.disconnect(_on_preset_suggested)


func preset_button(preset: String) -> Button:
	return find_child(String(PRESET_BUTTONS[preset]), true, false) as Button


func _render() -> void:
	for preset: String in PRESET_BUTTONS.keys():
		var chosen: bool = QualityManager.preset == preset
		preset_button(preset).set_pressed_no_signal(chosen)
		preset_button(preset).icon = ICON_CHECK if chosen else null
	_shadows.set_pressed_no_signal(QualityManager.shadows_enabled())
	_antialiasing.set_pressed_no_signal(QualityManager.antialiasing() != "none")
	_suggestion.visible = not suggested_preset.is_empty() and suggested_preset != QualityManager.preset
	_suggestion.text = tr("ui.quality.suggest_lower") if _suggestion.visible else ""


func _on_preset_pressed(preset: String) -> void:
	QualityManager.apply_preset(preset)


func _on_shadows_toggled(on: bool) -> void:
	QualityManager.set_override("quality_shadows", 1 if on else 0)


func _on_antialiasing_toggled(on: bool) -> void:
	QualityManager.set_override("quality_antialiasing", 1 if on else 0)


func _on_quality_changed(_preset: String) -> void:
	_render()


func _on_preset_suggested(preset: String) -> void:
	suggested_preset = preset
	_render()
