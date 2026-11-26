class_name VolumeRow
extends HBoxContainer
## F114-AC-09 (STUDENT_GAME_UX_SOCIAL §10): one volume row of Cài đặt → Âm thanh — icon + name, "−", slider, "+" and
## the value as a percentage. − / + move by Settings.VOLUME_STEP (10%) so a young student does not need to drag
## precisely; dragging the slider still works. Each change applies at once through Settings (saved locally); at 0% / 100%
## the − / + button is disabled (faded, outline kept). UI only — the value lives in Settings (AGENTS R1).

const NAME_KEYS: Dictionary = {
	"music": "ui.settings.volume.music", "sfx": "ui.settings.volume.sfx", "voice": "ui.settings.volume.voice",
}
const ICONS: Dictionary = {"music": "music", "sfx": "sound_on", "voice": "voice"}

var bus_key: String = ""
var minus: Button = null
var plus: Button = null
var slider: HSlider = null
var value_label: Label = null
var _settings: Node = null


## Builds the row for `key` ("music", "sfx", "voice") on `settings` (the Settings autoload, a test double in tests).
func setup(key: String, settings: Node) -> void:
	bus_key = key
	_settings = settings
	name = "Volume_%s" % key
	theme_type_variation = &"ToolbarBox"
	add_child(UiBuild.icon_rect(String(ICONS[key])))
	var title: Label = UiBuild.label(String(NAME_KEYS[key]))
	title.name = "Name"
	title.custom_minimum_size.x = float(UiTokens.SIZE_LAYOUT_120)
	title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	title.size_flags_stretch_ratio = 0.6
	add_child(title)
	minus = UiBuild.button("", "minus", &"IconButton")
	minus.name = "Minus"
	minus.pressed.connect(_on_step.bind(-1))
	add_child(minus)
	slider = HSlider.new()
	slider.name = "Slider"
	slider.min_value = 0.0
	slider.max_value = 1.0
	slider.step = Settings.VOLUME_STEP
	slider.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	slider.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	slider.custom_minimum_size = Vector2(float(UiTokens.SIZE_LAYOUT_120), UiBuild.target())
	slider.value_changed.connect(_on_slider_changed)
	add_child(slider)
	plus = UiBuild.button("", "plus", &"IconButton")
	plus.name = "Plus"
	plus.pressed.connect(_on_step.bind(1))
	add_child(plus)
	value_label = UiBuild.data_label("", &"ValueChip", false)
	value_label.name = "Value"
	value_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	value_label.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	add_child(value_label)
	render()


## Shows the saved value (slider, percentage, − / + enabled state) without writing it back.
func render() -> void:
	var value: float = float(_settings.call("get_volume", bus_key))
	slider.set_value_no_signal(value)
	var percent: int = roundi(clampf(value, 0.0, 1.0) * 100.0)
	value_label.text = "%d%%" % percent
	minus.disabled = percent <= 0
	plus.disabled = percent >= 100
	var item: String = tr(String(NAME_KEYS[bus_key]))
	minus.accessibility_name = tr("ui.settings.volume.down").format({"name": item})
	plus.accessibility_name = tr("ui.settings.volume.up").format({"name": item})
	slider.accessibility_name = "%s: %d%%" % [item, percent]


func _notification(what: int) -> void:
	if slider == null:
		return
	if what == NOTIFICATION_TRANSLATION_CHANGED:
		render()
	elif what == NOTIFICATION_THEME_CHANGED or what == NOTIFICATION_ENTER_TREE:
		_fit_value()


## The value chip is as wide as "100%" in the current Theme, so the row does not jump while the value changes.
func _fit_value() -> void:
	var font: Font = value_label.get_theme_font(&"font")
	var box: StyleBox = value_label.get_theme_stylebox(&"normal")
	var width: float = font.get_string_size("100%", HORIZONTAL_ALIGNMENT_LEFT, -1.0,
			value_label.get_theme_font_size(&"font_size")).x
	if box != null:
		width += box.get_margin(SIDE_LEFT) + box.get_margin(SIDE_RIGHT)
	value_label.custom_minimum_size.x = ceilf(width)


func _on_step(steps: int) -> void:
	_settings.call("nudge_volume", bus_key, steps)
	render()


func _on_slider_changed(value: float) -> void:
	_settings.call("set_volume", bus_key, value)
	render()
