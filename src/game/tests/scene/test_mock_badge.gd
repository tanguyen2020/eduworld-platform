extends GdUnitTestSuite
## F060-AC-20 (AGENTS R17, CODE_PLAN §2): with the mock environment (`is_mock: true`) the MOCK badge is shown on top
## of every screen and survives scene changes; without it there is no badge.

var _env: String = ""
var _autostart: bool = true


func before() -> void:
	# Splash would start the F061 entry flow (network); these tests only render screens.
	_autostart = AuthService.autostart
	AuthService.autostart = false


func after() -> void:
	AuthService.autostart = _autostart


func before_test() -> void:
	_env = Config.env_name
	_remove_badge()


func after_test() -> void:
	Config.load_env(_env if not _env.is_empty() else "local", false)
	_remove_badge()


func _remove_badge() -> void:
	if AppState.mock_badge != null:
		AppState.mock_badge.free()
		AppState.mock_badge = null


func test_mock_env_shows_badge_on_every_scene() -> void:
	assert_int(Config.load_env("mock", false)).is_equal(OK)
	assert_bool(Config.is_mock).is_true()
	AppState.start_ui(get_tree().root)
	var badge: CanvasLayer = AppState.mock_badge
	assert_object(badge).is_not_null()
	assert_bool(badge.is_inside_tree()).is_true()
	assert_int(badge.layer).is_greater_equal(128)
	var label: Label = badge.get_node("Badge/Label") as Label
	assert_str(label.text).is_equal("ui.debug.mock_badge")
	for locale: String in ["vi-VN", "en-US"]:
		assert_str(TranslationServer.get_translation_object(locale).get_message("ui.debug.mock_badge")).starts_with("MOCK")
	for scene_path: String in ["res://scenes/bootstrap/Splash.tscn", "res://scenes/ui/debug/UiGallery.tscn"]:
		var runner: GdUnitSceneRunner = scene_runner(scene_path)
		await runner.simulate_frames(2)
		assert_bool(AppState.mock_badge.is_inside_tree()).is_true()
		assert_bool(label.is_visible_in_tree()).is_true()


func test_real_env_has_no_badge() -> void:
	assert_int(Config.load_env("local", false)).is_equal(OK)
	AppState.start_ui(get_tree().root)
	assert_object(AppState.mock_badge).is_null()
