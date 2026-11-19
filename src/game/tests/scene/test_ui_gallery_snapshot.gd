extends GdUnitTestSuite
## F060-AC-10 (DESIGN_SYSTEM §9, NFR-A11Y-001, TC-UI-001 Godot part): UiGallery shows the base components at the
## 4 densities, high contrast, and the locales vi / en / xx-PS. This suite checks structure and sizing headless;
## pixel comparison against baselines is done by res://tools/screenshot_gallery.gd with a renderer (CI: xvfb),
## see `make godot-screenshots`.

const GALLERY: String = "res://scenes/ui/debug/UiGallery.tscn"
const COMPONENTS: PackedStringArray = [
	"Scroll/Content/Buttons/Primary", "Scroll/Content/Buttons/Secondary", "Scroll/Content/Buttons/Hint",
	"Scroll/Content/Card", "Scroll/Content/Dialog", "Scroll/Content/Toast", "Scroll/Content/Progress",
	"Scroll/Content/Heading", "Scroll/Content/Muted", "Scroll/Content/Input", "%SampleVietnamese", "%FormatSample",
]
const LOCALES: PackedStringArray = ["vi-VN", "en-US", "xx-PS"]

var _locale: String = ""
var _band: String = ""


func before_test() -> void:
	_locale = String(Settings.locale)
	_band = AppState.grade_band


func after_test() -> void:
	Settings.set_locale(_locale, false)
	AppState.set_grade_band(_band)
	if AppState.theme_service != null:
		AppState.theme_service.set_high_contrast(false)


func test_gallery_is_debug_only() -> void:
	var presets: String = FileAccess.get_file_as_string("res://export_presets.cfg")
	assert_str(presets).contains("scenes/ui/debug/*")


func test_components_render_at_every_density_and_locale() -> void:
	var runner: GdUnitSceneRunner = scene_runner(GALLERY)
	await runner.simulate_frames(2)
	var gallery: Node = runner.scene()
	for band: String in UiTokens.DENSITY_BANDS:
		AppState.set_grade_band(band)
		for locale: String in LOCALES:
			assert_bool(Settings.set_locale(locale, false)).is_true()
			await runner.simulate_frames(2)
			assert_int(gallery.get_tree().root.theme.default_font_size).is_equal(int(UiTokens.DENSITY_FONT_BASE[band]))
			for path: String in COMPONENTS:
				var node: Control = gallery.get_node(path) as Control
				assert_object(node).override_failure_message(path).is_not_null()
				assert_bool(node.is_visible_in_tree()).override_failure_message("%s hidden" % path).is_true()
			var primary: Button = gallery.get_node("Scroll/Content/Buttons/Primary") as Button
			assert_float(primary.get_combined_minimum_size().y).override_failure_message("%s button height" % band) \
				.is_greater_equal(float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT[band]) - float(UiTokens.SIZE_SPACE_4) * 4.0)


func test_high_contrast_toggle_applies_contrast_theme() -> void:
	var runner: GdUnitSceneRunner = scene_runner(GALLERY)
	await runner.simulate_frames(2)
	var toggle: CheckButton = runner.scene().get_node("%HighContrast") as CheckButton
	toggle.button_pressed = true
	await runner.simulate_frames(1)
	var contrast: Theme = load(UiTokens.HIGH_CONTRAST_THEME_PATH) as Theme
	assert_that(get_tree().root.theme.get_color("font_color", "Label")).is_equal(contrast.get_color("font_color", "Label"))


func test_screenshot_tool_lists_every_combination() -> void:
	var tool_script: GDScript = load("res://tools/screenshot_gallery.gd") as GDScript
	assert_object(tool_script).is_not_null()
	var combos: Array = tool_script.call("combinations") as Array
	assert_int(combos.size()).is_equal(UiTokens.DENSITY_BANDS.size() * LOCALES.size() + 1)
