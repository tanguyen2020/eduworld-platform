extends GdUnitTestSuite
## F062-AC-13 (AGENTS R17, CODE_PLAN §2; T017): with the `mock` profile (Prism) the permanent "MOCK" label is shown
## above the in-world HUD; release export presets never ship `config/env/mock.json` (nor tests/tools/debug scenes).

const WORLD_HUD: String = "res://scenes/ui/hud/MainHud.tscn"
const EXPORT_PRESETS: String = "res://export_presets.cfg"

var _env: String = ""


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


func test_mock_profile_shows_the_label_above_the_hud() -> void:
	assert_int(Config.load_env("mock", false)).is_equal(OK)
	AppState.start_ui(get_tree().root)
	var runner: GdUnitSceneRunner = scene_runner(WORLD_HUD)
	await runner.simulate_frames(2)
	var hud: CanvasLayer = runner.scene() as CanvasLayer
	var badge: CanvasLayer = AppState.mock_badge
	assert_object(badge).is_not_null()
	assert_bool(badge.is_inside_tree()).is_true()
	assert_int(badge.layer).is_greater(hud.layer)
	var label: Label = badge.get_node("Badge/Label") as Label
	assert_bool(label.is_visible_in_tree()).is_true()
	assert_str(label.tr(label.text)).starts_with("MOCK")


func test_release_presets_exclude_the_mock_profile() -> void:
	var cfg: ConfigFile = ConfigFile.new()
	assert_int(cfg.load(EXPORT_PRESETS)).is_equal(OK)
	var presets: int = 0
	for section: String in cfg.get_sections():
		if not section.begins_with("preset.") or section.ends_with(".options"):
			continue
		presets += 1
		var include: String = String(cfg.get_value(section, "include_filter", ""))
		var exclude: String = String(cfg.get_value(section, "exclude_filter", ""))
		assert_str(include).not_contains("mock")
		for path: String in ["config/env/mock.json", "tests/*", "tools/*", "scenes/ui/debug/*"]:
			assert_str(exclude).override_failure_message("%s %s" % [section, path]).contains(path)
	# Android today; Web and iOS presets join with F133 T002 (no Windows preset, ADR-0101).
	assert_int(presets).is_greater_equal(1)
	# The release Config never accepts --env=mock: only debug builds read the command line.
	assert_str(Config.resolve_env_name(PackedStringArray(["--env=mock"]), false)).is_not_equal("mock")
