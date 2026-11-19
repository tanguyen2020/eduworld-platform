extends GdUnitTestSuite
## F060-AC-09 (NFR-A11Y-001, GODOT_ARCHITECTURE §10): Settings persists font scale, high contrast, reduced motion,
## language and volumes in settings.cfg, and never writes keys outside its schema (no token, no business data).

const SettingsScript := preload("res://scripts/core/settings.gd")
const TEST_PATH: String = "user://test_settings.cfg"

var _settings: Node = null


func before_test() -> void:
	DirAccess.remove_absolute(TEST_PATH)
	_settings = auto_free(SettingsScript.new()) as Node
	_settings.call("load_from", TEST_PATH)


func after_test() -> void:
	DirAccess.remove_absolute(TEST_PATH)


func test_preferences_round_trip() -> void:
	_settings.call("set_font_scale", 2)
	_settings.call("set_high_contrast", true)
	_settings.call("set_reduce_motion", true)
	_settings.call("set_volume", "music", 0.25)
	var other: Node = auto_free(SettingsScript.new()) as Node
	other.call("load_from", TEST_PATH)
	assert_int(int(other.get("font_scale"))).is_equal(2)
	assert_bool(bool(other.get("high_contrast"))).is_true()
	assert_bool(bool(other.get("reduce_motion"))).is_true()
	assert_float(float(other.call("get_volume", "music"))).is_equal_approx(0.25, 0.0001)


func test_only_schema_keys_are_written() -> void:
	var cfg: ConfigFile = ConfigFile.new()
	cfg.set_value("ui", "locale", "en-US")
	cfg.set_value("auth", "refresh_token", "tok")
	cfg.set_value("ui", "last_quest", "q-1")
	cfg.save(TEST_PATH)
	_settings.call("load_from", TEST_PATH)
	_settings.call("set_font_scale", 1)
	var written: ConfigFile = ConfigFile.new()
	written.load(TEST_PATH)
	assert_array(Array(written.get_sections())).contains_exactly_in_any_order(["ui", "audio", "login"])
	for section: String in written.get_sections():
		for key: String in written.get_section_keys(section):
			assert_bool(key in (SettingsScript.SCHEMA[section] as Array)).override_failure_message("%s/%s" % [section, key]).is_true()
	var text: String = FileAccess.get_file_as_string(TEST_PATH)
	assert_bool(text.contains("tok")).is_false()
	assert_str(String(written.get_value("ui", "locale"))).is_equal("en-US")


func test_invalid_values_fall_back() -> void:
	var cfg: ConfigFile = ConfigFile.new()
	cfg.set_value("ui", "font_scale", 7)
	cfg.set_value("audio", "sfx", 3.0)
	cfg.save(TEST_PATH)
	_settings.call("load_from", TEST_PATH)
	assert_int(int(_settings.get("font_scale"))).is_equal(2)
	assert_float(float(_settings.call("get_volume", "sfx"))).is_equal(1.0)


func test_changes_emit_signals() -> void:
	var events: Array[String] = []
	_settings.connect("font_scale_changed", func(level: int) -> void: events.append("font:%d" % level))
	_settings.connect("high_contrast_changed", func(on: bool) -> void: events.append("contrast:%s" % on))
	_settings.connect("volume_changed", func(bus: String, value: float) -> void: events.append("%s:%.1f" % [bus, value]))
	_settings.call("set_font_scale", 1)
	_settings.call("set_high_contrast", true)
	_settings.call("set_volume", "voice", 0.5)
	assert_array(events).is_equal(["font:1", "contrast:true", "voice:0.5"])
