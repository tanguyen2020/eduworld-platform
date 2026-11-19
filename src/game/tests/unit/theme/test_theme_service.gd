extends GdUnitTestSuite
## F060-AC-08 (NFR-A11Y-001, DESIGN_SYSTEM §5.2): density follows the grade band, `g1_3` before bootstrap; a change
## re-applies the Theme to open windows at once; font scale and high contrast layer on the generated Theme.


func test_default_band_before_bootstrap_is_g1_3() -> void:
	assert_str(ThemeService.default_band()).is_equal("g1_3")
	var service: ThemeService = ThemeService.new()
	assert_str(service.band).is_equal("g1_3")
	assert_int(service.build_theme().default_font_size).is_equal(int(UiTokens.DENSITY_FONT_BASE["g1_3"]))


func test_band_change_updates_window_theme_and_emits() -> void:
	var window: Window = auto_free(Window.new()) as Window
	var service: ThemeService = ThemeService.new()
	var bands: Array[String] = []
	service.density_changed.connect(func(band: String) -> void: bands.append(band))
	service.attach(window)
	assert_int(window.theme.default_font_size).is_equal(int(UiTokens.DENSITY_FONT_BASE["g1_3"]))
	service.set_band("g6_7")
	assert_array(bands).is_equal(["g6_7"])
	assert_int(window.theme.default_font_size).is_equal(int(UiTokens.DENSITY_FONT_BASE["g6_7"]))
	assert_int(window.theme.get_font_size("font_size", "Button")).is_equal(int(UiTokens.DENSITY_FONT_BASE["g6_7"]))


func test_invalid_band_falls_back_to_default() -> void:
	var service: ThemeService = ThemeService.new("g6_7")
	service.set_band("g10_12")
	assert_str(service.band).is_equal("g1_3")


func test_font_scale_levels_enlarge_text() -> void:
	var service: ThemeService = ThemeService.new("g6_7")
	var base: int = service.build_theme().default_font_size
	service.set_font_scale(2)
	assert_int(service.build_theme().default_font_size).is_equal(roundi(base * UiTokens.NUMBER_RATIO_135))
	service.set_font_scale(9)
	assert_int(service.font_scale_level).is_equal(2)


func test_high_contrast_keeps_density_sizes() -> void:
	var service: ThemeService = ThemeService.new("g1_3")
	service.set_high_contrast(true)
	var theme: Theme = service.build_theme()
	var contrast: Theme = load(UiTokens.HIGH_CONTRAST_THEME_PATH) as Theme
	assert_that(theme.get_color("font_color", "Label")).is_equal(contrast.get_color("font_color", "Label"))
	assert_int(theme.default_font_size).is_equal(int(UiTokens.DENSITY_FONT_BASE["g1_3"]))
	var box: StyleBoxFlat = theme.get_stylebox("normal", "Button") as StyleBoxFlat
	assert_float(box.content_margin_left).is_equal(float(UiTokens.DENSITY_G1_3_SPACING_BASE))


func test_generated_theme_files_are_not_modified() -> void:
	var service: ThemeService = ThemeService.new("g1_3")
	service.set_high_contrast(true)
	service.set_font_scale(2)
	service.build_theme()
	var original: Theme = load(UiTokens.THEME_PATHS["g1_3"]) as Theme
	assert_int(original.default_font_size).is_equal(int(UiTokens.DENSITY_FONT_BASE["g1_3"]))


func test_app_state_grade_band_drives_density_and_event_bus() -> void:
	var previous: String = AppState.grade_band
	var bands: Array[String] = []
	var on_density: Callable = func(band: String) -> void: bands.append(band)
	EventBus.density_changed.connect(on_density)
	AppState.start_ui(get_tree().root)
	AppState.set_player({"grade_band": "g8_9"})
	assert_str(AppState.theme_service.band).is_equal("g8_9")
	assert_int(get_tree().root.theme.default_font_size).is_equal(int(UiTokens.DENSITY_FONT_BASE["g8_9"]))
	AppState.clear_player()
	assert_str(AppState.grade_band).is_equal("g1_3")
	EventBus.density_changed.disconnect(on_density)
	assert_array(bands).contains(["g8_9", "g1_3"])
	AppState.set_grade_band(previous)
