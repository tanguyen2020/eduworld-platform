extends VBoxContainer
## Cài đặt → Đồ họa (PERFORMANCE_BUDGET §5–§6, F062 T027): the student picks the preset and overrides shadows and
## anti-aliasing; choices are saved per device by QualityManager (`settings.cfg [quality]`, never sent to the server).
## When the dynamic resolution controller stays at its minimum, QualityManager only suggests a lower preset here — it
## never switches by itself (F062-AC-16).
## F114-AC-10 (STUDENT_GAME_UX_SOCIAL §10): three preset chips with a picture (1, 2, 3 bars); the chosen one is yellow
## with a tick (F114-AC-05). Shadows and anti-aliasing are `ToggleSwitch`es with the words Bật / Tắt, hidden in `g1_3`
## (the youngest students only see the three picture chips). UI only: QualityManager holds the logic (AGENTS R1).

const PRESET_BUTTONS: Dictionary = {"low": "PresetLow", "medium": "PresetMedium", "high": "PresetHigh"}
## The grade band that only sees the three preset chips (STUDENT_GAME_UX_SOCIAL §10 "3 nút hình").
const PRESETS_ONLY_BAND: String = "g1_3"

var suggested_preset: String = ""
var _idle_icons: Dictionary = {}

@onready var _toggles: Control = %Toggles
@onready var _shadows: ToggleSwitch = %Shadows
@onready var _antialiasing: ToggleSwitch = %Antialiasing
@onready var _suggestion: Label = %Suggestion


func _ready() -> void:
	var group: ButtonGroup = ButtonGroup.new()
	for preset: String in PRESET_BUTTONS.keys():
		var button: Button = preset_button(preset)
		button.toggle_mode = true
		button.button_group = group
		_idle_icons[button] = button.icon
		button.pressed.connect(_on_preset_pressed.bind(preset))
	_shadows.toggled.connect(_on_shadows_toggled)
	_antialiasing.toggled.connect(_on_antialiasing_toggled)
	QualityManager.quality_changed.connect(_on_quality_changed)
	QualityManager.preset_suggested.connect(_on_preset_suggested)
	AppState.grade_band_changed.connect(_on_grade_band_changed)
	_render()


func _exit_tree() -> void:
	if QualityManager.quality_changed.is_connected(_on_quality_changed):
		QualityManager.quality_changed.disconnect(_on_quality_changed)
	if QualityManager.preset_suggested.is_connected(_on_preset_suggested):
		QualityManager.preset_suggested.disconnect(_on_preset_suggested)
	if AppState.grade_band_changed.is_connected(_on_grade_band_changed):
		AppState.grade_band_changed.disconnect(_on_grade_band_changed)


func preset_button(preset: String) -> Button:
	return find_child(String(PRESET_BUTTONS[preset]), true, false) as Button


## True when the shadow / anti-aliasing switches are shown (every band but `g1_3`).
func toggles_shown() -> bool:
	return _toggles.visible


func _render() -> void:
	var chips: Array = []
	for preset: String in PRESET_BUTTONS.keys():
		var button: Button = preset_button(preset)
		button.set_pressed_no_signal(QualityManager.preset == preset)
		chips.append(button)
	UiBuild.mark_chips(chips, _idle_icons)
	_toggles.visible = UiBuild.band() != PRESETS_ONLY_BAND
	_shadows.show_state(QualityManager.shadows_enabled())
	_antialiasing.show_state(QualityManager.antialiasing() != "none")
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


func _on_grade_band_changed(_band: String) -> void:
	_render()
