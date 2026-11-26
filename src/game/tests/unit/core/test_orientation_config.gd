extends GdUnitTestSuite
## F113-AC-01, F113-AC-02 (STUDENT_GAME_UX §2 landscape on every platform, GODOT_ARCHITECTURE §10.2, §11): Android and
## iOS are locked to sensor landscape by project.godot, and no export preset overrides it (no Android orientation
## option, no extra iOS UISupportedInterfaceOrientations). Reads the files as text configs, like CI does.

const PROJECT: String = "res://project.godot"
const EXPORT_PRESETS: String = "res://export_presets.cfg"
const ORIENTATION_KEY: String = "window/handheld/orientation"


func _project() -> ConfigFile:
	var cfg: ConfigFile = ConfigFile.new()
	assert_int(cfg.load(PROJECT)).is_equal(OK)
	return cfg


func _presets() -> ConfigFile:
	var cfg: ConfigFile = ConfigFile.new()
	assert_int(cfg.load(EXPORT_PRESETS)).is_equal(OK)
	return cfg


func _options_of(cfg: ConfigFile, platform: String) -> String:
	for section: String in cfg.get_sections():
		if section.begins_with("preset.") and not section.ends_with(".options") \
				and String(cfg.get_value(section, "platform", "")) == platform:
			return "%s.options" % section
	return ""


func test_project_locks_handhelds_to_sensor_landscape() -> void:
	var cfg: ConfigFile = _project()
	assert_bool(cfg.has_section_key("display", ORIENTATION_KEY)) \
		.override_failure_message("project.godot must set display/%s explicitly" % ORIENTATION_KEY).is_true()
	assert_int(int(cfg.get_value("display", ORIENTATION_KEY, -1))).is_equal(DisplayServer.SCREEN_SENSOR_LANDSCAPE)
	# The running project sees the same value (no override file changes it).
	assert_int(int(ProjectSettings.get_setting("display/" + ORIENTATION_KEY, -1))) \
		.is_equal(DisplayServer.SCREEN_SENSOR_LANDSCAPE)


func test_landscape_base_canvas_is_kept() -> void:
	var cfg: ConfigFile = _project()
	assert_int(int(cfg.get_value("display", "window/size/viewport_width", 0))).is_equal(1280)
	assert_int(int(cfg.get_value("display", "window/size/viewport_height", 0))).is_equal(720)
	assert_str(String(cfg.get_value("display", "window/stretch/mode", ""))).is_equal("canvas_items")
	assert_str(String(cfg.get_value("display", "window/stretch/aspect", ""))).is_equal("expand")


func test_android_preset_does_not_override_the_orientation() -> void:
	var cfg: ConfigFile = _presets()
	var section: String = _options_of(cfg, "Android")
	assert_str(section).is_not_empty()
	for key: String in cfg.get_section_keys(section):
		assert_bool(key.to_lower().contains("orientation")) \
			.override_failure_message("Android preset overrides the orientation: %s" % key).is_false()


func test_ios_preset_keeps_landscape_left_and_right_only() -> void:
	var cfg: ConfigFile = _presets()
	var section: String = _options_of(cfg, "iOS")
	assert_str(section).is_not_empty()
	for key: String in cfg.get_section_keys(section):
		assert_bool(key.to_lower().contains("orientation")) \
			.override_failure_message("iOS preset overrides the orientation: %s" % key).is_false()
	# Godot writes UISupportedInterfaceOrientations (Landscape Left + Right) from the project setting; extra plist
	# content must not add its own list nor a portrait value.
	var plist: String = String(cfg.get_value(section, "application/additional_plist_content", ""))
	assert_str(plist).not_contains("UISupportedInterfaceOrientations")
	assert_str(plist).not_contains("Portrait")


## The check itself fails on a wrong file (a guard against a silent test).
func test_check_flags_a_portrait_override() -> void:
	var cfg: ConfigFile = ConfigFile.new()
	cfg.set_value("preset.0", "platform", "Android")
	cfg.set_value("preset.0.options", "screen/orientation", 1)
	var flagged: bool = false
	for key: String in cfg.get_section_keys(_options_of(cfg, "Android")):
		flagged = flagged or key.to_lower().contains("orientation")
	assert_bool(flagged).is_true()
