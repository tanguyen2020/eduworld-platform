class_name ThemeService
extends RefCounted
## Applies the Theme generated from design/game-tokens (F005) to the game window (F060-AC-08, NFR-A11Y-001, R32).
## Density variant follows the grade band (`g1_3` before bootstrap); the user's font scale (3 levels) and high
## contrast are layered on a copy of the generated Theme at runtime — the generated `.tres` files are never edited.
## Held by `AppState`; changing any input re-applies the Theme so open screens update without restart.

signal theme_applied(band: String)
signal density_changed(band: String)

## Font scale levels (Settings `font_scale` 0/1/2) as ratios from UiTokens: 1.0, midpoint, 1.35 [Proposed].
const FONT_SCALES: Array[float] = [
	UiTokens.NUMBER_RATIO_100,
	(UiTokens.NUMBER_RATIO_100 + UiTokens.NUMBER_RATIO_135) / 2.0,
	UiTokens.NUMBER_RATIO_135,
]

var band: String = ""
var font_scale_level: int = 0
var high_contrast: bool = false
var current_theme: Theme = null

var _window: Window = null
var _cache: Dictionary = {}


func _init(initial_band: String = "") -> void:
	band = initial_band if is_valid_band(initial_band) else default_band()


static func default_band() -> String:
	var configured: String = String(ProjectSettings.get_setting("eduworld/ui/pre_bootstrap_density", "g1_3"))
	return configured if is_valid_band(configured) else UiTokens.DENSITY_BANDS[0]


static func is_valid_band(value: String) -> bool:
	return value in UiTokens.DENSITY_BANDS


## Binds the service to a window (normally the root) and applies the current Theme to it.
func attach(window: Window) -> void:
	_window = window
	_apply()


func set_band(value: String) -> void:
	var next: String = value if is_valid_band(value) else default_band()
	if next == band:
		return
	band = next
	_apply()
	density_changed.emit(band)


func set_font_scale(level: int) -> void:
	var next: int = clampi(level, 0, FONT_SCALES.size() - 1)
	if next == font_scale_level:
		return
	font_scale_level = next
	_apply()


func set_high_contrast(enabled: bool) -> void:
	if enabled == high_contrast:
		return
	high_contrast = enabled
	_apply()


## Builds (or reuses) the effective Theme for the current band, scale and contrast.
func build_theme() -> Theme:
	var key: String = "%s|%d|%s" % [band, font_scale_level, high_contrast]
	if _cache.has(key):
		return _cache[key] as Theme
	var base: Theme = load(String(UiTokens.THEME_PATHS[band])) as Theme
	var theme: Theme = base.duplicate(true) as Theme
	if high_contrast:
		_apply_contrast(theme, load(UiTokens.HIGH_CONTRAST_THEME_PATH) as Theme)
	var scale: float = FONT_SCALES[font_scale_level]
	if not is_equal_approx(scale, 1.0):
		_scale_fonts(theme, scale)
	_cache[key] = theme
	return theme


func _apply() -> void:
	current_theme = build_theme()
	if _window != null and is_instance_valid(_window):
		_window.theme = current_theme
	theme_applied.emit(band)


## Colours come from the high-contrast Theme; sizes and margins stay those of the density variant.
static func _apply_contrast(theme: Theme, contrast: Theme) -> void:
	for type_name: String in contrast.get_type_list():
		for color_name: String in contrast.get_color_list(type_name):
			theme.set_color(color_name, type_name, contrast.get_color(color_name, type_name))
		for box_name: String in contrast.get_stylebox_list(type_name):
			if not theme.has_stylebox(box_name, type_name):
				continue
			var source: StyleBoxFlat = contrast.get_stylebox(box_name, type_name) as StyleBoxFlat
			var target: StyleBoxFlat = theme.get_stylebox(box_name, type_name) as StyleBoxFlat
			if source == null or target == null:
				continue
			target.bg_color = source.bg_color
			target.border_color = source.border_color


static func _scale_fonts(theme: Theme, scale: float) -> void:
	theme.default_font_size = roundi(theme.default_font_size * scale)
	for type_name: String in theme.get_type_list():
		for size_name: String in theme.get_font_size_list(type_name):
			theme.set_font_size(size_name, type_name, roundi(theme.get_font_size(size_name, type_name) * scale))
