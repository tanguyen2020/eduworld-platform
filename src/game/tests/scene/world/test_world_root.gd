extends GdUnitTestSuite
## WorldRoot wiring (F062-AC-09/10 UI host, F062-AC-14 first-run benchmark, F062-AC-15 device profiles, F062-AC-21
## break reminder dialog, F061-AC-36 curfew banner kept from the removed WorldPlaceholder): intents without a screen in
## this client show `ui.activity_player.update_required`; registered screens open as panels and closing one emits
## `EventBus.ui_closed`; the benchmark runs before play only when no preset is saved; the reminder never pauses the game.
## QualityManager uses a temp settings file here (never the device's settings.cfg).

const WORLD: String = "res://scenes/world/WorldRoot.tscn"
const SettingsScript := preload("res://scripts/core/settings.gd")
const SETTINGS_PATH: String = "user://test_f062_world_settings.cfg"
const OVERRIDE_PATH: String = "user://test_f062_world_renderer.cfg"

var _settings: Node = null
var _saved_settings: Node = null
var _saved_override: String = ""
var _config: ClientConfig = null
var _closed: Array[StringName] = []


func before_test() -> void:
	DirAccess.remove_absolute(SETTINGS_PATH)
	_settings = SettingsScript.new() as Node
	_settings.call("load_from", SETTINGS_PATH)
	_saved_settings = QualityManager.settings
	QualityManager.settings = _settings
	_saved_override = QualityManager.override_path
	QualityManager.override_path = OVERRIDE_PATH
	_config = AuthService.version_check.config
	AuthService.version_check.config = null
	_closed.clear()
	EventBus.ui_closed.connect(_on_ui_closed)
	AppState.spawn_point_id = ""


func after_test() -> void:
	EventBus.ui_closed.disconnect(_on_ui_closed)
	QualityManager.settings = _saved_settings
	QualityManager.override_path = _saved_override
	DirAccess.remove_absolute(OVERRIDE_PATH)
	QualityManager.apply_preset(QualityPolicy.MEDIUM, false)
	AuthService.version_check.config = _config
	_settings.free()
	DirAccess.remove_absolute(SETTINGS_PATH)
	AppState.get_services().play_time.configure({})


func _on_ui_closed(screen_id: StringName) -> void:
	_closed.append(screen_id)


func _world(preset: String = QualityPolicy.MEDIUM) -> Array:
	_settings.set("quality_preset", preset)
	var runner: GdUnitSceneRunner = scene_runner(WORLD)
	await runner.simulate_frames(2)
	return [runner, runner.scene()]


func test_first_run_measures_before_play_and_saves_the_preset() -> void:
	var r: Array = await _world("")
	var world: Node = r[1] as Node
	var benchmark: Control = world.find_child("QualityBenchmark", true, false) as Control
	var hud: CanvasLayer = world.find_child("MainHud", true, false) as CanvasLayer
	var zone: ZoneRoot = world.get("zone") as ZoneRoot
	assert_bool(benchmark.visible).is_true()
	assert_bool(hud.visible).is_false()
	assert_bool(bool(zone.player.get("input_enabled"))).is_false()
	benchmark.set_process(false)
	for i: int in 60:
		benchmark.call("step", 0.1)
	assert_bool(benchmark.visible).is_false()
	assert_bool(hud.visible).is_true()
	assert_bool(bool(zone.player.get("input_enabled"))).is_true()
	assert_str(String(_settings.get("quality_preset"))).is_not_empty()
	assert_float(float(benchmark.get("elapsed_sec"))).is_less_equal(QualityPolicy.BENCHMARK_MAX_SEC + 0.11)


func test_skip_then_choose_by_hand() -> void:
	var r: Array = await _world("")
	var world: Node = r[1] as Node
	var benchmark: Control = world.find_child("QualityBenchmark", true, false) as Control
	(benchmark.find_child("Skip", true, false) as Button).pressed.emit()
	assert_bool((benchmark.find_child("Manual", true, false) as Control).visible).is_true()
	(benchmark.find_child("Preset_low", true, false) as Button).pressed.emit()
	assert_str(String(_settings.get("quality_preset"))).is_equal(QualityPolicy.LOW)
	assert_bool(benchmark.visible).is_false()


func test_saved_preset_skips_the_benchmark() -> void:
	var r: Array = await _world(QualityPolicy.LOW)
	var world: Node = r[1] as Node
	assert_bool((world.find_child("QualityBenchmark", true, false) as Control).visible).is_false()
	assert_bool((world.find_child("MainHud", true, false) as CanvasLayer).visible).is_true()


func test_matching_device_profile_replaces_the_benchmark() -> void:
	var config: ClientConfig = ClientConfig.new()
	config.device_profiles.append({"match": {"platform": RestApiClient.client_platform()}, "renderer": "mobile",
		"preset": "low"})
	AuthService.version_check.config = config
	var r: Array = await _world("")
	var world: Node = r[1] as Node
	assert_bool((world.find_child("QualityBenchmark", true, false) as Control).visible).is_false()
	assert_str(String(_settings.get("quality_preset"))).is_equal(QualityPolicy.LOW)


func test_intent_without_a_screen_shows_update_required() -> void:
	var r: Array = await _world()
	var world: Node = r[1] as Node
	# `activity_host` is registered since F063 (tests/scene/learning/test_activity_host.gd); `exam_lobby` comes with F064.
	EventBus.ui_requested.emit(&"exam_lobby", {"exam_session_id": "y"})
	var notice: Control = world.find_child("Notice", true, false) as Control
	assert_bool(notice.visible).is_true()
	assert_str((notice.find_child("Body", true, false) as Label).text).is_equal(tr("ui.activity_player.update_required"))


func test_journal_opens_as_a_panel_and_closing_returns_to_the_hub() -> void:
	var r: Array = await _world()
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var world: Node = r[1] as Node
	var zone: ZoneRoot = world.get("zone") as ZoneRoot
	(world.find_child("JournalButton", true, false) as Button).pressed.emit()
	await runner.simulate_frames(1)
	var journal: Control = (world.get("panels") as Dictionary).get(&"quest_journal") as Control
	assert_object(journal).is_not_null()
	assert_bool(journal.visible).is_true()
	assert_bool(bool(zone.player.get("input_enabled"))).is_false()
	journal.call("close")
	assert_array(_closed).is_equal([&"quest_journal"])
	assert_bool(bool(zone.player.get("input_enabled"))).is_true()
	EventBus.ui_requested.emit(&"settings", {})
	var settings_panel: Control = (world.get("panels") as Dictionary).get(&"settings") as Control
	assert_bool(settings_panel.visible).is_true()


func test_break_reminder_dialog_is_gentle() -> void:
	var r: Array = await _world()
	var world: Node = r[1] as Node
	var services: ServiceLocator = AppState.get_services()
	services.play_time.configure({"play_time": {"break_reminder_minutes": 1}})
	for i: int in 60:
		services.play_time.tick(1.0)
	var dialog: Control = world.find_child("BreakReminderDialog", true, false) as Control
	assert_bool(dialog.visible).is_true()
	assert_str((dialog.find_child("Body", true, false) as Label).text).contains("1")
	assert_bool(get_tree().paused).is_false()
	assert_bool(bool((world.get("zone") as ZoneRoot).player.get("input_enabled"))).is_true()
	(dialog.find_child("Ok", true, false) as Button).pressed.emit()
	assert_bool(dialog.visible).is_false()


func test_curfew_warning_is_a_hud_banner() -> void:
	var r: Array = await _world()
	var world: Node = r[1] as Node
	AuthService.curfew_warning_raised.emit(10)
	var banner: Control = world.find_child("SystemBanner", true, false) as Control
	assert_bool(banner.visible).is_true()
	assert_str((world.find_child("BannerLabel", true, false) as Label).text).contains("10")
	assert_str(GameStateMachine.scene_for(GameStateMachine.State.IN_WORLD)).is_equal(WORLD)
	assert_bool(ResourceLoader.exists("res://scenes/world/WorldPlaceholder.tscn")).is_false()
