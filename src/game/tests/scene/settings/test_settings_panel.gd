extends GdUnitTestSuite
## F096-AC-04, F096-AC-13, F096-AC-10 (STUDENT_GAME_UX_SOCIAL §10, Godot MIT licence): Cài đặt → Thông tin → Giấy phép shows
## the game version, Godot Engine's MIT licence and its third-party notices (from the engine) and the project's own
## licences; the chosen language and preset are selected chips with a check icon; "Xem lại hướng dẫn của Mây" forgets
## the device flag and asks the world to open the guide again.

const SCENE: String = "res://scenes/ui/settings/SettingsPanel.tscn"

var _locale: String = ""
var _seen: bool = false
var _requested: int = 0


func before_test() -> void:
	_locale = String(Settings.locale)
	_seen = Settings.onboarding_hub_seen
	_requested = 0
	EventBus.onboarding_requested.connect(_on_requested)


func after_test() -> void:
	EventBus.onboarding_requested.disconnect(_on_requested)
	Settings.set_locale(_locale, false)
	Settings.onboarding_hub_seen = _seen


func _on_requested() -> void:
	_requested += 1


func _panel() -> Control:
	var panel: Control = auto_free((load(SCENE) as PackedScene).instantiate()) as Control
	add_child(panel)
	panel.call("open")
	return panel


func test_licences_page_lists_godot_mit_and_third_parties() -> void:
	var panel: Control = _panel()
	(panel.find_child("OpenLicenses", true, false) as Button).pressed.emit()
	assert_bool(bool(panel.call("licenses_visible"))).is_true()
	var text: String = (panel.find_child("LicenseText", true, false) as Label).text
	assert_str(text).contains("Godot Engine")
	assert_str(text).contains("Permission is hereby granted, free of charge")
	assert_str(text).contains(Engine.get_license_text().strip_edges().left(60))
	for component: Dictionary in Engine.get_copyright_info():
		assert_str(text).contains(String(component["name"]))
	assert_str((panel.find_child("AboutVersion", true, false) as Label).text).contains(Config.client_version)
	assert_str(tr((panel.find_child("Project", true, false) as Label).text)).contains("MIT")
	assert_str(tr((panel.find_child("Project", true, false) as Label).text)).contains("Open Font License")
	(panel.find_child("Back", true, false) as Button).pressed.emit()
	assert_bool(bool(panel.call("licenses_visible"))).is_false()


func test_chosen_language_and_preset_are_chips_with_a_check() -> void:
	Settings.set_locale("vi-VN", false)
	var panel: Control = _panel()
	var vi: Button = panel.find_child("LangVi", true, false) as Button
	var en: Button = panel.find_child("LangEn", true, false) as Button
	assert_str(String(vi.theme_type_variation)).is_equal("ChoiceChip")
	assert_bool(vi.button_pressed).is_true()
	assert_object(vi.icon).is_not_null()
	assert_object(en.icon).is_null()
	en.pressed.emit()
	assert_bool(en.button_pressed).is_true()
	assert_object(en.icon).is_not_null()
	assert_object(vi.icon).is_null()
	var quality: Node = panel.find_child("QualitySettings", true, false)
	var chosen: Button = quality.call("preset_button", QualityManager.preset) as Button
	assert_str(String(chosen.theme_type_variation)).is_equal("ChoiceChip")
	assert_bool(chosen.button_pressed).is_true()
	assert_object(chosen.icon).is_not_null()
	# Selected chip text on the accent fill reaches WCAG AA (contrast is checked on tokens by design-tokens-check).
	assert_object(chosen.get_theme_color(&"font_pressed_color")).is_equal(UiTokens.COMPONENT_CHIP_SELECTED_TEXT)


func test_replay_guide_forgets_the_flag_and_asks_for_the_guide() -> void:
	Settings.onboarding_hub_seen = true
	var panel: Control = _panel()
	(panel.find_child("ReplayGuide", true, false) as Button).pressed.emit()
	assert_bool(Settings.onboarding_hub_seen).is_false()
	assert_int(_requested).is_equal(1)
	assert_bool(panel.visible).is_false()
