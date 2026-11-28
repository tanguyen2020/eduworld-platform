extends GdUnitTestSuite
## F114-AC-01…AC-04, AC-06 (owner 2026-11-26, DESIGN_SYSTEM §5.1–§5.2, NFR-A11Y-001, TC-UI-001): the tiered cartoon
## outline comes from tokens. Tier 1 (interactive controls, window frames): ink outline `density.<band>.outline.interactive`
## and a hard ink bottom edge `density.<band>.depth.rest`; pressed, the edge is `depth.pressed`, the face is drawn lower
## by the difference and the content moves down by the same amount (same control height). Tier 2 (information): thin ink
## outline `outline.info`, no edge. Tier 3 (stem, question card, answer tiles, TextButton) keeps the tokens@0.16.0 values
## (fixture taken from the generated Themes before F114). High contrast: tier 1 and tier 2 outlines 3 px in every band.

const TIER3_FIXTURE: String = "res://tests/fixtures/theme/tier3_tokens_0_16_0.json"
const THEME_DIR: String = "res://resources/ui/theme/"
## Momentary tier 1 buttons: rest look on normal / hover, sunk look on pressed.
const BUTTONS: PackedStringArray = ["Button", "SecondaryButton", "DangerButton", "IconButton", "HintButton",
		"TabButtonSelected"]
## Toggle tier 1 chips: the chosen chip is the pressed look (yellow, sunk) — Godot draws a toggled-on button pressed.
const CHIPS: PackedStringArray = ["ChoiceChip", "PictureTile"]
## Window frames (type, stylebox): outline + edge, never pressed.
const FRAMES: Array = [["PanelContainer", "panel"], ["FormCard", "panel"], ["WindowTitleBar", "panel"],
		["ConfirmTitleBar", "panel"]]
## Tier 2 information surfaces (type, stylebox).
const INFO: Array = [["HudPanel", "panel"], ["SpeechBubble", "panel"], ["InfoBannerPanel", "panel"],
		["InlineErrorPanel", "panel"], ["ProgressBar", "background"], ["ProgressBar", "fill"], ["HudChip", "normal"],
		["HudChip", "pressed"], ["SectionTag", "normal"], ["ValueChip", "normal"], ["CountBadge", "normal"]]
## F114 variants declared by the token pipeline (F114-AC-01).
const VARIANTS: Dictionary = {
	"DangerButton": "Button", "HudChip": "Button", "TabButton": "Button", "TabButtonSelected": "Button",
	"ToggleSwitch": "CheckButton", "SectionTag": "Label", "ValueChip": "Label", "WindowTitleLabel": "Label",
	"ConfirmTitleLabel": "Label", "WindowTitleBar": "PanelContainer", "ConfirmTitleBar": "PanelContainer",
	"SettingsTabColumn": "PanelContainer", "Scrim": "Panel", "ScrimNested": "Panel",
	# F123: unread count pill / "Mới" tag (tier 2).
	"CountBadge": "Label",
}


func _assert_color(actual: Color, expected: Color) -> void:
	for i: int in 4:
		assert_float([actual.r, actual.g, actual.b, actual.a][i]).override_failure_message(
				"%s != %s" % [actual, expected]).is_equal_approx([expected.r, expected.g, expected.b, expected.a][i], 0.002)


func _theme(band: String) -> Theme:
	return load(String(UiTokens.THEME_PATHS[band])) as Theme


func _box(theme: Theme, type_name: String, box_name: String) -> StyleBoxFlat:
	var box: StyleBoxFlat = theme.get_stylebox(box_name, type_name) as StyleBoxFlat
	assert_object(box).override_failure_message("%s/%s missing" % [type_name, box_name]).is_not_null()
	return box


func test_tokens_hold_the_confirmed_values() -> void:
	# F123 bumped the tokens to 0.18.0 (CountBadge), F102 to 0.19.0 (party card, warning banner, countdown), F119 to 0.20.0
	# (chat panel, bubbles, reaction pills), F125 to 0.21.0 (compact mode floors and Themes), the portal dark-theme fix to 0.22.0,
	# the portal info/success tag contrast (F058 T057) to 0.23.0: MINOR,
	# the F114 values below are unchanged.
	assert_str(UiTokens.TOKENS_VERSION).is_equal("0.23.0")
	_assert_color(UiTokens.COLOR_OUTLINE, Color("#1E2A44"))
	_assert_color(UiTokens.COLOR_OUTLINE, UiTokens.COLOR_TEXT_PRIMARY)
	var scrim: Color = UiTokens.COLOR_OVERLAY_SCRIM
	var nested: Color = UiTokens.COLOR_OVERLAY_SCRIM_NESTED
	_assert_color(Color(scrim, 1.0), UiTokens.COLOR_OUTLINE)
	assert_float(scrim.a).is_equal_approx(0.5, 0.01)
	_assert_color(Color(nested, 1.0), UiTokens.COLOR_OUTLINE)
	assert_float(nested.a).is_equal_approx(0.35, 0.01)
	assert_dict(UiTokens.DENSITY_OUTLINE_INTERACTIVE).is_equal({"g1_3": 3, "g4_5": 3, "g6_7": 2, "g8_9": 2})
	assert_dict(UiTokens.DENSITY_OUTLINE_INFO).is_equal({"g1_3": 2, "g4_5": 2, "g6_7": 2, "g8_9": 2})
	assert_dict(UiTokens.DENSITY_DEPTH_REST).is_equal({"g1_3": 5, "g4_5": 4, "g6_7": 3, "g8_9": 3})
	assert_dict(UiTokens.DENSITY_DEPTH_PRESSED).is_equal({"g1_3": 1, "g4_5": 1, "g6_7": 1, "g8_9": 1})
	_assert_color(UiTokens.COMPONENT_CHOICE_CHIP_SELECTED_BG, Color("#FFC93C"))
	_assert_color(UiTokens.COMPONENT_CHOICE_CHIP_SELECTED_TEXT, UiTokens.COLOR_OUTLINE)
	_assert_color(UiTokens.COMPONENT_CHOICE_CHIP_BG, Color.WHITE)
	_assert_color(UiTokens.COMPONENT_SECTION_TAG_BG, Color("#FFF1B8"))
	_assert_color(UiTokens.COMPONENT_SECTION_TAG_TEXT, Color("#6B4E00"))
	_assert_color(UiTokens.COMPONENT_BUTTON_DANGER_BG, UiTokens.COLOR_FEEDBACK_INCORRECT)
	_assert_color(UiTokens.COMPONENT_TOGGLE_TRACK_ON, UiTokens.COLOR_ACCENT_PRIMARY)
	_assert_color(UiTokens.COMPONENT_SETTINGS_TITLE_BG, UiTokens.COLOR_ACCENT_PRIMARY)
	assert_float(UiTokens.COMPONENT_WINDOW_POP_DURATION).is_less_equal(0.2)
	assert_float(UiTokens.COMPONENT_WINDOW_POP_SCALE).is_equal_approx(0.96, 0.0001)


func test_every_band_theme_declares_the_new_variants() -> void:
	for band: String in UiTokens.DENSITY_BANDS:
		var theme: Theme = _theme(band)
		for variant: String in VARIANTS:
			assert_str(String(theme.get_type_variation_base(variant))) \
					.override_failure_message("%s %s base" % [band, variant]).is_equal(String(VARIANTS[variant]))
		assert_object(theme.get_icon(&"checked", &"ToggleSwitch")).is_not_null()
		assert_object(theme.get_icon(&"grabber", &"HSlider")).is_not_null()
		_assert_color(_box(theme, "Scrim", "panel").bg_color, UiTokens.COLOR_OVERLAY_SCRIM)
		_assert_color(_box(theme, "ScrimNested", "panel").bg_color, UiTokens.COLOR_OVERLAY_SCRIM_NESTED)


## F114-AC-02: rest outline + edge on normal / hover, pressed = short edge, face drawn lower, content moved down.
func _assert_raised(theme: Theme, band: String, type_name: String, rest_name: String, pressed_name: String) -> void:
	var o: int = int(UiTokens.DENSITY_OUTLINE_INTERACTIVE[band])
	var d: int = int(UiTokens.DENSITY_DEPTH_REST[band])
	var p: int = int(UiTokens.DENSITY_DEPTH_PRESSED[band])
	var rest: StyleBoxFlat = _box(theme, type_name, rest_name)
	var pressed: StyleBoxFlat = _box(theme, type_name, pressed_name)
	var where: String = "%s %s %s/%s" % [band, type_name, rest_name, pressed_name]
	for box: StyleBoxFlat in [rest, pressed]:
		_assert_color(box.border_color, UiTokens.COLOR_OUTLINE)
		assert_int(box.border_width_left).override_failure_message(where + " outline").is_equal(o)
		assert_int(box.border_width_top).is_equal(o)
		assert_int(box.border_width_right).is_equal(o)
	assert_int(rest.border_width_bottom).override_failure_message(where + " rest edge").is_equal(o + d)
	assert_int(pressed.border_width_bottom).override_failure_message(where + " pressed edge").is_equal(o + p)
	assert_float(pressed.expand_margin_top).override_failure_message(where + " face sinks").is_equal(-float(d - p))
	assert_float(pressed.content_margin_top - rest.content_margin_top) \
			.override_failure_message(where + " content moves down").is_equal(float(d - p))
	assert_float(pressed.get_minimum_size().y).override_failure_message(where + " same height") \
			.is_equal(rest.get_minimum_size().y)
	# The content never sits on the bottom edge.
	assert_float(rest.content_margin_bottom).is_greater_equal(float(o + d))


func test_tier1_buttons_have_ink_outline_edge_and_sink_when_pressed() -> void:
	for band: String in UiTokens.DENSITY_BANDS:
		var theme: Theme = _theme(band)
		for type_name: String in BUTTONS:
			_assert_raised(theme, band, type_name, "normal", "pressed")
			_assert_raised(theme, band, type_name, "hover", "pressed")
		for type_name: String in CHIPS:
			_assert_raised(theme, band, type_name, "normal", "pressed")
			_assert_raised(theme, band, type_name, "hover", "hover_pressed")


func test_tier1_slider_and_switch_are_outlined() -> void:
	for band: String in UiTokens.DENSITY_BANDS:
		var theme: Theme = _theme(band)
		var o: int = int(UiTokens.DENSITY_OUTLINE_INTERACTIVE[band])
		var d: int = int(UiTokens.DENSITY_DEPTH_REST[band])
		for box_name: String in ["slider", "grabber_area"]:
			var box: StyleBoxFlat = _box(theme, "HSlider", box_name)
			_assert_color(box.border_color, UiTokens.COLOR_OUTLINE)
			assert_int(box.border_width_left).is_equal(o)
			assert_int(box.border_width_bottom).is_equal(o + d)
		# The switch track and knob are drawn by the generated icons with the ink outline (toggle_on / toggle_off).
		assert_str(theme.get_icon(&"checked", &"ToggleSwitch").resource_path).ends_with("toggle_on.svg")
		assert_str(theme.get_icon(&"unchecked", &"ToggleSwitch").resource_path).ends_with("toggle_off.svg")


## F114-AC-03: window frames = outline + edge (no pressed state); tier 2 = thin outline on every side, no edge.
func test_frames_have_an_edge_and_information_surfaces_a_thin_outline() -> void:
	for band: String in UiTokens.DENSITY_BANDS:
		var theme: Theme = _theme(band)
		var o: int = int(UiTokens.DENSITY_OUTLINE_INTERACTIVE[band])
		var oi: int = int(UiTokens.DENSITY_OUTLINE_INFO[band])
		var d: int = int(UiTokens.DENSITY_DEPTH_REST[band])
		for pair: Array in FRAMES:
			var frame: StyleBoxFlat = _box(theme, String(pair[0]), String(pair[1]))
			var where: String = "%s %s" % [band, pair[0]]
			_assert_color(frame.border_color, UiTokens.COLOR_OUTLINE)
			assert_int(frame.border_width_left).override_failure_message(where).is_equal(o)
			assert_int(frame.border_width_bottom).override_failure_message(where).is_equal(o + d)
			assert_float(frame.expand_margin_top).is_equal(0.0)
			assert_int(frame.shadow_size).override_failure_message(where + " no soft shadow").is_equal(0)
		for pair: Array in INFO:
			var info: StyleBoxFlat = _box(theme, String(pair[0]), String(pair[1]))
			var where: String = "%s %s/%s" % [band, pair[0], pair[1]]
			_assert_color(info.border_color, UiTokens.COLOR_OUTLINE)
			for side: int in [SIDE_LEFT, SIDE_TOP, SIDE_RIGHT, SIDE_BOTTOM]:
				assert_int(info.get_border_width(side)).override_failure_message(where + " side %d" % side).is_equal(oi)
			assert_float(info.expand_margin_top).is_equal(0.0)


## F114-AC-04: tier 3 keeps every value of tokens@0.16.0 (border, colours, edge, margins, fonts) in every Theme file.
func test_tier3_variants_keep_their_tokens_0_16_0_values() -> void:
	var fixture: Dictionary = JSON.parse_string(FileAccess.get_file_as_string(TIER3_FIXTURE)) as Dictionary
	assert_str(String(fixture["source"])).is_equal("tokens@0.16.0")
	var themes: Dictionary = fixture["themes"] as Dictionary
	assert_int(themes.size()).is_equal(6)
	var checked: int = 0
	for file: String in themes:
		var theme: Theme = load(THEME_DIR + file) as Theme
		var entries: Dictionary = themes[file] as Dictionary
		for key: String in entries:
			var parts: PackedStringArray = key.split("/")
			var where: String = "%s %s" % [file, key]
			var expected: Variant = entries[key]
			match parts[1]:
				"styles":
					var box: StyleBoxFlat = theme.get_stylebox(parts[2], parts[0]) as StyleBoxFlat
					assert_object(box).override_failure_message(where).is_not_null()
					for prop: String in expected as Dictionary:
						_assert_value(box.get(prop), (expected as Dictionary)[prop], "%s.%s" % [where, prop])
				"colors":
					_assert_value(theme.get_color(parts[2], parts[0]), expected, where)
				"font_sizes":
					assert_int(theme.get_font_size(parts[2], parts[0])).override_failure_message(where) \
							.is_equal(int(expected))
				"constants":
					assert_int(theme.get_constant(parts[2], parts[0])).override_failure_message(where) \
							.is_equal(int(expected))
			checked += 1
	assert_int(checked).is_greater(600)


func _assert_value(actual: Variant, expected: Variant, where: String) -> void:
	if expected is Array:
		var e: Array = expected as Array
		if actual is Color:
			var c: Color = actual as Color
			for i: int in 4:
				assert_float([c.r, c.g, c.b, c.a][i]).override_failure_message(where).is_equal_approx(float(e[i]), 0.0002)
		elif actual is Vector2:
			assert_that(actual).override_failure_message(where).is_equal(Vector2(float(e[0]), float(e[1])))
		return
	if expected is bool:
		assert_bool(bool(actual)).override_failure_message(where).is_equal(bool(expected))
		return
	assert_float(float(actual)).override_failure_message(where).is_equal_approx(float(expected), 0.0001)


## F114-AC-06: high contrast → tier 1 and tier 2 outlines 3 px in every band, the bottom edge keeps the band depth;
## tier 3 boxes do not change.
func test_high_contrast_outlines_are_three_px_in_every_band() -> void:
	for band: String in UiTokens.DENSITY_BANDS:
		var service: ThemeService = ThemeService.new(band)
		service.high_contrast = true
		var theme: Theme = service.build_theme()
		var d: int = int(UiTokens.DENSITY_DEPTH_REST[band])
		var p: int = int(UiTokens.DENSITY_DEPTH_PRESSED[band])
		for type_name: String in BUTTONS:
			var rest: StyleBoxFlat = _box(theme, type_name, "normal")
			assert_int(rest.border_width_left).override_failure_message("%s %s hc" % [band, type_name]).is_equal(3)
			assert_int(rest.border_width_bottom).override_failure_message("%s %s hc edge" % [band, type_name]) \
					.is_equal(3 + d)
			assert_int(_box(theme, type_name, "pressed").border_width_bottom).is_equal(3 + p)
		for pair: Array in FRAMES:
			assert_int(_box(theme, String(pair[0]), String(pair[1])).border_width_left).is_equal(3)
		for pair: Array in INFO:
			var info: StyleBoxFlat = _box(theme, String(pair[0]), String(pair[1]))
			assert_int(info.border_width_left).override_failure_message("%s %s hc" % [band, pair[0]]).is_equal(3)
			assert_int(info.border_width_bottom).is_equal(3)
		var normal_theme: Theme = _theme(band)
		for type_name: String in ["AnswerTile1", "AnswerTileRest", "TextButton"]:
			assert_int(_box(theme, type_name, "normal").border_width_bottom) \
					.is_equal(_box(normal_theme, type_name, "normal").border_width_bottom)
