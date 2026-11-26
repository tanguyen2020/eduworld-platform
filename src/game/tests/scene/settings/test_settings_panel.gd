extends GdUnitTestSuite
## S15 Cài đặt (F114-AC-05, AC-07…AC-14; F096-AC-04, AC-10; STUDENT_GAME_UX_SOCIAL §10): window with a title bar and a red
## Close, five tabs + "Đăng xuất" in the tab column, pages for language, graphics, sound (− / + step 10%), accessibility
## (text size chips, high contrast and reduce motion switches) and about (Mây's guide, licences, version); the chosen chip
## is yellow with a tick; the logout confirmation has the nested scrim, focus on "Hủy" and runs the logout only on
## "Đăng xuất"; Esc closes the confirmation, then the licences page, then the window. Settings writes to a temp file here.

const SCENE: String = "res://scenes/ui/settings/SettingsPanel.tscn"
const SETTINGS_PATH: String = "user://test_f114_settings.cfg"

var _locale: String = ""
var _seen: bool = false
var _requested: int = 0
var _path: String = ""
var _volumes: Dictionary = {}
var _font_scale: int = 0
var _contrast: bool = false
var _motion: bool = false
var _band: String = ""


func before_test() -> void:
	_locale = String(Settings.locale)
	_seen = Settings.onboarding_hub_seen
	_path = Settings.path
	_volumes = Settings.volumes.duplicate()
	_font_scale = Settings.font_scale
	_contrast = Settings.high_contrast
	_motion = Settings.reduce_motion
	_band = AppState.grade_band
	Settings.path = SETTINGS_PATH
	_requested = 0
	EventBus.onboarding_requested.connect(_on_requested)


func after_test() -> void:
	EventBus.onboarding_requested.disconnect(_on_requested)
	Settings.set_locale(_locale, false)
	Settings.onboarding_hub_seen = _seen
	Settings.volumes = _volumes
	Settings.set_font_scale(_font_scale)
	Settings.set_high_contrast(_contrast)
	Settings.set_reduce_motion(_motion)
	Settings.path = _path
	DirAccess.remove_absolute(SETTINGS_PATH)
	AppState.set_grade_band(_band)


func _on_requested() -> void:
	_requested += 1


func _panel() -> Control:
	var panel: Control = auto_free((load(SCENE) as PackedScene).instantiate()) as Control
	add_child(panel)
	panel.call("open")
	return panel


func _button(panel: Node, name: String) -> Button:
	return panel.find_child(name, true, false) as Button


func test_licences_page_lists_godot_mit_and_third_parties() -> void:
	var panel: Control = _panel()
	panel.call("show_tab", "info")
	_button(panel, "OpenLicenses").pressed.emit()
	assert_bool(bool(panel.call("licenses_visible"))).is_true()
	assert_bool((panel.find_child("Content", true, false) as Control).visible).is_false()
	var text: String = (panel.find_child("LicenseText", true, false) as Label).text
	assert_str(text).contains("Godot Engine")
	assert_str(text).contains("Permission is hereby granted, free of charge")
	assert_str(text).contains(Engine.get_license_text().strip_edges().left(60))
	for component: Dictionary in Engine.get_copyright_info():
		assert_str(text).contains(String(component["name"]))
	assert_str((panel.find_child("AboutVersion", true, false) as Label).text).contains(Config.client_version)
	assert_str(tr((panel.find_child("Project", true, false) as Label).text)).contains("MIT")
	assert_str(tr((panel.find_child("Project", true, false) as Label).text)).contains("Open Font License")
	_button(panel, "Back").pressed.emit()
	assert_bool(bool(panel.call("licenses_visible"))).is_false()


## F114-AC-08: title bar in capitals, red Close with ✕, five tabs (icon + words) and Đăng xuất at the foot of the column,
## scrolling content; first opening on Ngôn ngữ, the next one on the tab used last.
func test_window_has_title_bar_tabs_and_logout_at_the_foot() -> void:
	var panel: Control = _panel()
	var title: Label = panel.find_child("Title", true, false) as Label
	assert_str(String(title.theme_type_variation)).is_equal("WindowTitleLabel")
	assert_bool(title.uppercase).is_true()
	assert_str(title.text).is_equal("ui.settings.title")
	var close: Button = _button(panel, "Close")
	assert_str(String(close.theme_type_variation)).is_equal("DangerButton")
	assert_str(close.icon.resource_path).ends_with("cross.svg")
	assert_str(close.text).is_not_empty()
	for i: int in 3:
		await await_idle_frame()
	var tabs: PackedStringArray = ["language", "graphics", "sound", "access", "info"]
	var previous_y: float = -1.0
	for tab: String in tabs:
		var button: Button = panel.call("tab_button", tab) as Button
		assert_object(button.icon).override_failure_message(tab).is_not_null()
		assert_str(button.text).override_failure_message(tab).is_not_empty()
		assert_float(button.position.y).is_greater(previous_y)
		previous_y = button.position.y
	var logout: Button = _button(panel, "Logout")
	assert_str(String(logout.theme_type_variation)).is_equal("DangerButton")
	assert_object(logout.icon).is_not_null()
	assert_bool(logout.get_parent().get_child(logout.get_parent().get_child_count() - 1) == logout).is_true()
	assert_object(panel.find_child("Content", true, false) as ScrollContainer).is_not_null()
	assert_str(String(panel.get("current_tab"))).is_equal("language")
	assert_bool((panel.call("page", "language") as Control).visible).is_true()
	assert_str(String((panel.call("tab_button", "language") as Button).theme_type_variation)).is_equal("TabButtonSelected")
	(panel.call("tab_button", "sound") as Button).pressed.emit()
	assert_bool((panel.call("page", "sound") as Control).visible).is_true()
	assert_bool((panel.call("page", "language") as Control).visible).is_false()
	assert_str(String((panel.call("tab_button", "language") as Button).theme_type_variation)).is_equal("TabButton")
	panel.call("close")
	panel.call("open")
	assert_str(String(panel.get("current_tab"))).is_equal("sound")
	assert_bool((panel.call("page", "sound") as Control).visible).is_true()


## F114-AC-05: the chosen chip is yellow with a tick, the others white with ink words and no tick.
func test_chosen_language_and_preset_are_yellow_chips_with_a_check() -> void:
	Settings.set_locale("vi-VN", false)
	var panel: Control = _panel()
	var vi: Button = _button(panel, "LangVi")
	var en: Button = _button(panel, "LangEn")
	assert_str(String(vi.theme_type_variation)).is_equal("ChoiceChip")
	assert_bool(vi.button_pressed).is_true()
	assert_str(vi.icon.resource_path).ends_with("check.svg")
	assert_object(en.icon).is_null()
	assert_that((vi.get_theme_stylebox(&"pressed") as StyleBoxFlat).bg_color) \
			.is_equal(UiTokens.COMPONENT_CHOICE_CHIP_SELECTED_BG)
	assert_that(vi.get_theme_color(&"font_pressed_color")).is_equal(UiTokens.COMPONENT_CHOICE_CHIP_SELECTED_TEXT)
	assert_that(vi.get_theme_color(&"icon_pressed_color")).is_equal(UiTokens.COMPONENT_CHOICE_CHIP_SELECTED_TEXT)
	assert_that((en.get_theme_stylebox(&"normal") as StyleBoxFlat).bg_color).is_equal(UiTokens.COMPONENT_CHOICE_CHIP_BG)
	assert_that(en.get_theme_color(&"font_color")).is_equal(UiTokens.COLOR_TEXT_PRIMARY)
	en.pressed.emit()
	assert_bool(en.button_pressed).is_true()
	assert_object(en.icon).is_not_null()
	assert_object(vi.icon).is_null()
	var quality: Node = panel.find_child("QualitySettings", true, false)
	var chosen: Button = quality.call("preset_button", QualityManager.preset) as Button
	assert_str(String(chosen.theme_type_variation)).is_equal("ChoiceChip")
	assert_bool(chosen.button_pressed).is_true()
	assert_str(chosen.icon.resource_path).ends_with("check.svg")
	for preset: String in ["low", "medium", "high"]:
		var chip: Button = quality.call("preset_button", preset) as Button
		assert_object(chip.icon).override_failure_message(preset + " picture").is_not_null()
		if preset != QualityManager.preset:
			assert_str(chip.icon.resource_path).contains("quality_")


## F114-AC-07: scrim behind the window, nested scrim behind the confirmation; the opening zoom lasts ≤ 0.2 s and does
## not run with reduce motion.
func test_scrims_and_the_opening_zoom_follow_reduce_motion() -> void:
	Settings.set_reduce_motion(false)
	var panel: Control = _panel()
	var scrim: Panel = panel.find_child("Scrim", true, false) as Panel
	assert_bool(scrim.is_visible_in_tree()).is_true()
	assert_that((scrim.get_theme_stylebox(&"panel") as StyleBoxFlat).bg_color).is_equal(UiTokens.COLOR_OVERLAY_SCRIM)
	var tween: Tween = panel.get("open_tween") as Tween
	assert_object(tween).is_not_null()
	assert_float(UiMotion.POP_DURATION).is_less_equal(0.2)
	assert_float((panel.find_child("Window", true, false) as Control).scale.x).is_less_equal(1.0)
	_button(panel, "Logout").pressed.emit()
	var nested: Panel = panel.find_child("ScrimNested", true, false) as Panel
	assert_bool(nested.is_visible_in_tree()).is_true()
	assert_that((nested.get_theme_stylebox(&"panel") as StyleBoxFlat).bg_color) \
			.is_equal(UiTokens.COLOR_OVERLAY_SCRIM_NESTED)
	panel.call("go_back")
	Settings.set_reduce_motion(true)
	panel.call("close")
	panel.call("open")
	assert_object(panel.get("open_tween")).is_null()
	assert_that((panel.find_child("Window", true, false) as Control).scale).is_equal(Vector2.ONE)


## F114-AC-09: three volume rows; − / + step 10%, the percentage follows, the value is applied and saved; ends disable.
func test_volume_rows_step_by_ten_percent_and_save() -> void:
	Settings.set_volume("music", 1.0)
	var panel: Control = _panel()
	panel.call("show_tab", "sound")
	for key: String in ["music", "sfx", "voice"]:
		var row: VolumeRow = panel.call("volume_row", key) as VolumeRow
		assert_object(row).override_failure_message(key).is_not_null()
		assert_object(row.minus).is_not_null()
		assert_object(row.plus).is_not_null()
		assert_object(row.slider).is_not_null()
		assert_str(row.minus.accessibility_name).is_not_empty()
	var music: VolumeRow = panel.call("volume_row", "music") as VolumeRow
	assert_str(music.value_label.text).is_equal("100%")
	assert_bool(music.plus.disabled).is_true()
	music.minus.pressed.emit()
	assert_float(Settings.get_volume("music")).is_equal_approx(0.9, 0.0001)
	assert_str(music.value_label.text).is_equal("90%")
	music.minus.pressed.emit()
	assert_str(music.value_label.text).is_equal("80%")
	assert_float(music.slider.value).is_equal_approx(0.8, 0.0001)
	music.slider.value = 0.3
	assert_float(Settings.get_volume("music")).is_equal_approx(0.3, 0.0001)
	assert_str(music.value_label.text).is_equal("30%")
	for i: int in 4:
		music.minus.pressed.emit()
	assert_str(music.value_label.text).is_equal("0%")
	assert_bool(music.minus.disabled).is_true()
	assert_bool(music.plus.disabled).is_false()
	var saved: ConfigFile = ConfigFile.new()
	assert_int(saved.load(SETTINGS_PATH)).is_equal(OK)
	assert_float(float(saved.get_value("audio", "music", -1.0))).is_equal_approx(0.0, 0.0001)


## F114-AC-10: graphics page uses QualitySettings; g1_3 sees only the three picture chips.
func test_graphics_page_hides_switches_for_the_youngest_band() -> void:
	AppState.set_grade_band("g1_3")
	var panel: Control = _panel()
	panel.call("show_tab", "graphics")
	var quality: Node = panel.find_child("QualitySettings", true, false)
	assert_bool(bool(quality.call("toggles_shown"))).is_false()
	AppState.set_grade_band("g6_7")
	assert_bool(bool(quality.call("toggles_shown"))).is_true()
	var shadows: ToggleSwitch = quality.find_child("Shadows", true, false) as ToggleSwitch
	assert_str(String(shadows.theme_type_variation)).is_equal("ToggleSwitch")


## F114-AC-11: text size chips and the two switches apply at once and write Bật / Tắt beside the switch.
func test_accessibility_page_applies_text_size_contrast_and_motion() -> void:
	Settings.set_font_scale(0)
	Settings.set_high_contrast(false)
	Settings.set_reduce_motion(false)
	var panel: Control = _panel()
	panel.call("show_tab", "access")
	var chips: Array = panel.call("font_chips") as Array
	assert_int(chips.size()).is_equal(3)
	assert_bool((chips[0] as Button).button_pressed).is_true()
	(chips[2] as Button).pressed.emit()
	assert_int(Settings.font_scale).is_equal(2)
	assert_bool((chips[2] as Button).button_pressed).is_true()
	assert_object((chips[2] as Button).icon).is_not_null()
	assert_object((chips[0] as Button).icon).is_null()
	var contrast: ToggleSwitch = panel.find_child("HighContrast", true, false) as ToggleSwitch
	var motion: ToggleSwitch = panel.find_child("ReduceMotion", true, false) as ToggleSwitch
	assert_str(contrast.text).is_equal(ToggleSwitch.OFF_KEY)
	contrast.button_pressed = true
	assert_bool(Settings.high_contrast).is_true()
	assert_str(contrast.text).is_equal(ToggleSwitch.ON_KEY)
	assert_str(contrast.accessibility_name).contains(tr("ui.settings.high_contrast"))
	motion.button_pressed = true
	assert_bool(Settings.reduce_motion).is_true()
	assert_str(motion.text).is_equal(ToggleSwitch.ON_KEY)
	# The page stays open while the Theme is rebuilt (edge case: contrast / text size changed while open).
	assert_str(String(panel.get("current_tab"))).is_equal("access")
	assert_bool((panel.call("page", "access") as Control).visible).is_true()


## F114-AC-12: bilingual language title, two chips; about page has the guide, licences and the version.
func test_language_and_about_pages() -> void:
	var panel: Control = _panel()
	var tag: Label = panel.find_child("LanguageTag", true, false) as Label
	assert_str(String(tag.theme_type_variation)).is_equal("SectionTag")
	TranslationServer.set_locale("vi-VN")
	assert_str(tr(tag.text)).contains("Language")
	assert_str(tr(tag.text)).contains("Ngôn ngữ")
	TranslationServer.set_locale(Settings.locale)
	panel.call("show_tab", "info")
	assert_object(_button(panel, "ReplayGuide")).is_not_null()
	assert_object(_button(panel, "OpenLicenses")).is_not_null()
	assert_str((panel.find_child("InfoVersion", true, false) as Label).text).contains(Config.client_version)


## F114-AC-13: confirmation look, focus on Hủy, Esc order; the logout runs only on Đăng xuất (not tested here: the
## AuthService flow is covered by the F061 tests).
func test_logout_confirmation_and_back_order() -> void:
	var panel: Control = _panel()
	panel.call("show_tab", "info")
	_button(panel, "OpenLicenses").pressed.emit()
	_button(panel, "Logout").pressed.emit()
	await await_idle_frame()
	assert_bool(bool(panel.call("confirm_visible"))).is_true()
	var cancel: Button = _button(panel, "Cancel")
	var confirm: Button = _button(panel, "Confirm")
	assert_bool(cancel.has_focus()).is_true()
	assert_str(String(cancel.theme_type_variation)).is_equal("SecondaryButton")
	assert_str(String(confirm.theme_type_variation)).is_equal("DangerButton")
	assert_object(cancel.icon).is_not_null()
	assert_object(confirm.icon).is_not_null()
	assert_str(String((panel.find_child("TitleBar", true, false) as Control).theme_type_variation)) \
			.is_equal("WindowTitleBar")
	var confirm_title: Label = panel.find_child("LogoutConfirm", true, false).find_child("Title", true, false) as Label
	assert_str(String(confirm_title.theme_type_variation)).is_equal("ConfirmTitleLabel")
	assert_bool(confirm_title.uppercase).is_true()
	panel.call("go_back")
	assert_bool(bool(panel.call("confirm_visible"))).is_false()
	assert_bool(bool(panel.call("licenses_visible"))).is_true()
	assert_bool(panel.visible).is_true()
	panel.call("go_back")
	assert_bool(bool(panel.call("licenses_visible"))).is_false()
	assert_bool(panel.visible).is_true()
	panel.call("go_back")
	assert_bool(panel.visible).is_false()


## F114-AC-14: tabs lead into the page with the keyboard / gamepad (Phải), every control reaches the band's touch target.
func test_keyboard_reaches_the_page_and_controls_are_big_enough() -> void:
	var panel: Control = _panel()
	panel.call("show_tab", "sound")
	var tab: Button = panel.call("tab_button", "sound") as Button
	assert_str(String(tab.focus_neighbor_right)).is_not_empty()
	var target: Control = tab.get_node(tab.focus_neighbor_right) as Control
	assert_bool((panel.call("page", "sound") as Control).is_ancestor_of(target)).is_true()
	for node: Node in panel.find_children("*", "Button", true, false):
		var button: Button = node as Button
		assert_float(button.get_combined_minimum_size().y).override_failure_message(String(button.name)) \
				.is_greater_equal(float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT[UiBuild.band()]) - 0.5)


func test_replay_guide_forgets_the_flag_and_asks_for_the_guide() -> void:
	Settings.onboarding_hub_seen = true
	var panel: Control = _panel()
	panel.call("show_tab", "info")
	_button(panel, "ReplayGuide").pressed.emit()
	assert_bool(Settings.onboarding_hub_seen).is_false()
	assert_int(_requested).is_equal(1)
	assert_bool(panel.visible).is_false()
