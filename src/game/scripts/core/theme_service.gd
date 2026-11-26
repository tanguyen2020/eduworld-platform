class_name ThemeService
extends RefCounted
## Applies the Theme generated from design/game-tokens (F005) to the game window (F060-AC-08, NFR-A11Y-001, R32).
## Density variant follows the grade band (`g1_3` before bootstrap); the user's font scale (3 levels) and high
## contrast are layered on a copy of the generated Theme at runtime — the generated `.tres` files are never edited.
## Held by `AppState`; changing any input re-applies the Theme so open screens update without restart.
## F114: a Control right under a CanvasLayer (HUD, world panels, dialogs) does not inherit the Window's Theme in Godot 4
## (it fell back to the project theme `g1_3` for every band); the service gives those layer roots the current Theme
## when they enter the tree and keeps them in step with every change. A layer root with a Theme of its own (the rotate
## prompt) is left alone.

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
## Layer roots given the current Theme: instance id → WeakRef.
var _layer_roots: Dictionary = {}


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
	var tree: SceneTree = window.get_tree()
	if tree != null and not tree.node_added.is_connected(_on_node_added):
		tree.node_added.connect(_on_node_added)
	_apply()
	for node: Node in window.find_children("*", "Control", true, false):
		_on_node_added(node)


## Layer roots currently following the service (tests, diagnostics).
func layer_root_count() -> int:
	return _layer_roots.size()


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
	var previous: Theme = current_theme
	current_theme = build_theme()
	if _window != null and is_instance_valid(_window):
		_window.theme = current_theme
	for id: int in _layer_roots.keys():
		var root: Control = (_layer_roots[id] as WeakRef).get_ref() as Control
		if root == null or root.theme != previous:
			_layer_roots.erase(id)
		else:
			root.theme = current_theme
	theme_applied.emit(band)


func _on_node_added(node: Node) -> void:
	var control: Control = node as Control
	if control == null or control.theme != null or not (control.get_parent() is CanvasLayer):
		return
	if _window == null or not is_instance_valid(_window) or control.get_window() != _window:
		return
	control.theme = current_theme
	_layer_roots[control.get_instance_id()] = weakref(control)


## Colours come from the high-contrast Theme; sizes and margins stay those of the density variant. F114-AC-06: the
## outline widths come from the high-contrast Theme too (tier 1 and tier 2 outlines 3 px in every band) while the hard
## bottom edge keeps the band's depth — the bottom border grows by the same amount as the outline.
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
			_apply_outline(target, source)


## Outline sides of `source`; the bottom border keeps the depth of `target` above its outline. Boxes without an
## outline (focus ring, tier 3 tiles, tabs at rest) have the same widths in both Themes and stay as they are.
static func _apply_outline(target: StyleBoxFlat, source: StyleBoxFlat) -> void:
	var outline: int = source.border_width_left
	var delta: int = outline - target.border_width_left
	if delta == 0 or target.border_width_left == 0:
		return
	target.border_width_left = outline
	target.border_width_top = source.border_width_top
	target.border_width_right = source.border_width_right
	target.border_width_bottom += delta


static func _scale_fonts(theme: Theme, scale: float) -> void:
	theme.default_font_size = roundi(theme.default_font_size * scale)
	for type_name: String in theme.get_type_list():
		for size_name: String in theme.get_font_size_list(type_name):
			theme.set_font_size(size_name, type_name, roundi(theme.get_font_size(size_name, type_name) * scale))
