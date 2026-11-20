extends GdUnitTestSuite
## F062-AC-14, F062-AC-15, F062-AC-16, F062-AC-17 (FR-GAME-009, NFR-COMP-001, PERFORMANCE_BUDGET §4–§6, TC-PERF-022,
## TC-PERF-009; T022, T023): first run (no saved preset) needs the ≤ 5 s benchmark and picks the preset from the
## measured p10 FPS; `client-config.device_profiles` force preset and renderer; the dynamic resolution controller keeps
## the 3D scale inside the preset range, never touches the UI or the preset (only suggests); a renderer that failed
## before drawing its first frame (or an engine fallback) means Compatibility next run + `renderer_fallback`.
## Uses a private QualityManager instance on a SubViewport and a temp settings file (never the device's).

const QualityManagerScript := preload("res://scripts/core/quality_manager.gd")
const SettingsScript := preload("res://scripts/core/settings.gd")
const SETTINGS_PATH: String = "user://test_f062_quality_settings.cfg"
const OVERRIDE_PATH: String = "user://test_f062_renderer_override.cfg"

var qm: Node = null
var settings: Node = null
var viewport: SubViewport = null
var telemetry: TelemetryService = null
var _services: ServiceLocator = null
var _max_fps: int = 0


func before_test() -> void:
	_max_fps = Engine.max_fps
	DirAccess.remove_absolute(SETTINGS_PATH)
	DirAccess.remove_absolute(OVERRIDE_PATH)
	settings = SettingsScript.new() as Node
	settings.call("load_from", SETTINGS_PATH)
	viewport = SubViewport.new()
	add_child(viewport)
	_services = AppState.services
	AppState.services = ServiceLocator.new(null, null, FakeClock.new())
	telemetry = AppState.services.telemetry
	qm = _make("windows")


func after_test() -> void:
	if is_instance_valid(qm):
		qm.free()
	viewport.free()
	settings.free()
	AppState.services = _services
	DirAccess.remove_absolute(SETTINGS_PATH)
	DirAccess.remove_absolute(OVERRIDE_PATH)
	Engine.max_fps = _max_fps


func _make(platform: String) -> Node:
	var node: Node = QualityManagerScript.new() as Node
	node.set("settings", settings)
	node.set("viewport", viewport)
	node.set("override_path", OVERRIDE_PATH)
	node.set("apply_engine_fps", true)
	add_child(node)
	node.set("platform", platform)
	node.call("apply_preset", QualityPolicy.MEDIUM, false)
	return node


func _frames(ms: float, count: int) -> PackedFloat32Array:
	var out: PackedFloat32Array = PackedFloat32Array()
	for i: int in count:
		out.append(ms)
	return out


# ---------- F062-AC-14 benchmark ----------

func test_first_run_needs_benchmark_and_saves_the_chosen_preset() -> void:
	assert_bool(qm.call("needs_benchmark")).is_true()
	assert_float(QualityPolicy.BENCHMARK_MAX_SEC).is_less_equal(5.0)
	qm.call("start_benchmark")
	for ms: float in _frames(16.0, 120):
		qm.call("push_benchmark_frame", ms)
	assert_str(String(qm.call("finish_benchmark"))).is_equal(QualityPolicy.MEDIUM)
	assert_bool(qm.call("needs_benchmark")).is_false()
	var reloaded: Node = SettingsScript.new() as Node
	reloaded.call("load_from", SETTINGS_PATH)
	assert_str(String(reloaded.get("quality_preset"))).is_equal(QualityPolicy.MEDIUM)
	reloaded.free()


func test_preset_choice_from_p10_fps() -> void:
	# PC target 60 FPS: ≥ 90 → high, ≥ 54 → medium, else low. Mobile target 30: never high.
	assert_str(QualityPolicy.choose_preset(95.0, "windows")).is_equal(QualityPolicy.HIGH)
	assert_str(QualityPolicy.choose_preset(58.0, "windows")).is_equal(QualityPolicy.MEDIUM)
	assert_str(QualityPolicy.choose_preset(40.0, "windows")).is_equal(QualityPolicy.LOW)
	assert_str(QualityPolicy.choose_preset(120.0, "android")).is_equal(QualityPolicy.MEDIUM)
	assert_str(QualityPolicy.choose_preset(27.0, "android")).is_equal(QualityPolicy.MEDIUM)
	assert_str(QualityPolicy.choose_preset(20.0, "android")).is_equal(QualityPolicy.LOW)
	# p10 = the 10 % slowest frames: 90 fast frames + 10 slow ones → the slow frame rate.
	var frames: PackedFloat32Array = _frames(10.0, 85)
	frames.append_array(_frames(50.0, 15))
	assert_float(QualityPolicy.fps_p10(frames)).is_equal_approx(20.0, 0.01)


func test_skipping_lets_the_student_choose() -> void:
	qm.call("apply_preset", QualityPolicy.LOW)
	assert_str(String(settings.get("quality_preset"))).is_equal(QualityPolicy.LOW)
	assert_bool(qm.call("needs_benchmark")).is_false()


# ---------- F062-AC-15 device profiles ----------

func test_device_profile_forces_preset_and_renderer() -> void:
	var profiles: Array = [
		{"match": {"platform": "android", "gpu_model_pattern": "Mali-G5[0-9]"}, "renderer": "compatibility", "preset": "low"},
		{"match": {"platform": "windows", "gpu_vendor": "intel", "gpu_model_pattern": "UHD Graphics 6[0-9]0"},
			"renderer": "compatibility", "preset": "low", "reason": "driver bug"},
	]
	var device: Dictionary = {"platform": "windows", "gpu_vendor": "Intel", "gpu_model": "Intel(R) UHD Graphics 630",
		"device_model": "PC", "os_version": "10.0"}
	var matched: Dictionary = qm.call("apply_device_profiles", profiles, device) as Dictionary
	assert_str(String(matched.get("reason", ""))).is_equal("driver bug")
	assert_str(String(qm.get("preset"))).is_equal(QualityPolicy.LOW)
	assert_str(String(settings.get("quality_renderer"))).is_equal(QualityPolicy.RENDERER_COMPATIBILITY)
	var cfg: ConfigFile = ConfigFile.new()
	assert_int(cfg.load(OVERRIDE_PATH)).is_equal(OK)
	assert_str(String(cfg.get_value("rendering", "renderer/rendering_method"))).is_equal("gl_compatibility")
	assert_bool(qm.call("needs_benchmark")).is_false()


func test_unmatched_or_invalid_profiles_change_nothing() -> void:
	var device: Dictionary = {"platform": "android", "gpu_model": "Adreno 610", "os_version": "13"}
	var profiles: Array = [
		{"match": {"platform": "android", "gpu_model_pattern": "Mali"}, "renderer": "compatibility", "preset": "low"},
		{"match": {"platform": "android", "gpu_model_pattern": "(["}, "renderer": "compatibility", "preset": "low"},
		{"match": {"platform": "android", "os_version_max": "12"}, "renderer": "compatibility", "preset": "low"},
		{"match": {"platform": "android"}, "renderer": "compatibility", "preset": "ultra"},
		"not a profile",
	]
	assert_bool((qm.call("apply_device_profiles", profiles, device) as Dictionary).is_empty()).is_true()
	assert_bool(qm.call("needs_benchmark")).is_true()
	var newer: Array = [{"match": {"platform": "android", "os_version_max": "13.1"}, "renderer": "mobile", "preset": "medium"}]
	assert_bool((qm.call("apply_device_profiles", newer, device) as Dictionary).is_empty()).is_false()


func test_client_config_parses_device_profiles() -> void:
	var data: Dictionary = AuthHarness.fixture("ClientConfig")
	data["device_profiles"] = [{"match": {"platform": "android"}, "renderer": "compatibility", "preset": "low"}, 3]
	var config: ClientConfig = ClientConfig.from_dict(data)
	assert_int(config.device_profiles.size()).is_equal(1)


# ---------- F062-AC-16 dynamic resolution ----------

func test_controller_lowers_and_raises_scale_inside_the_preset_range() -> void:
	var dr: DynamicResolution = DynamicResolution.new()
	dr.configure(0.9, 0.7, 1.0, 1000.0 / 30.0)
	for i: int in 60:
		dr.push_frame(40.0)
	assert_float(dr.scale).is_less(0.9)
	for i: int in 2000:
		dr.push_frame(40.0)
	assert_float(dr.scale).is_equal_approx(0.7, 0.001)
	for i: int in 5000:
		dr.push_frame(20.0)
	assert_float(dr.scale).is_equal_approx(1.0, 0.001)


func test_controller_waits_ten_seconds_before_raising() -> void:
	var dr: DynamicResolution = DynamicResolution.new()
	dr.configure(0.8, 0.7, 1.0, 1000.0 / 30.0)
	for i: int in int(9.0 / 0.02):
		dr.push_frame(20.0)
	assert_float(dr.scale).is_equal_approx(0.8, 0.001)
	for i: int in int(2.0 / 0.02):
		dr.push_frame(20.0)
	assert_float(dr.scale).is_equal_approx(0.85, 0.001)


func test_at_minimum_scale_only_a_lower_preset_is_suggested() -> void:
	var suggested: Array[String] = []
	qm.connect("preset_suggested", func(p: String) -> void: suggested.append(p))
	var world: Node3D = Node3D.new()
	qm.call("register_world", world)
	for i: int in int(80.0 / 0.05):
		qm.call("tick", 0.05)
		qm.call("touch_activity")
	assert_str(String(qm.get("preset"))).is_equal(QualityPolicy.MEDIUM)
	var dr: DynamicResolution = qm.get("resolution") as DynamicResolution
	assert_float(dr.scale).is_equal_approx(0.7, 0.001)
	assert_float(viewport.scaling_3d_scale).is_equal_approx(0.7, 0.001)
	assert_array(suggested).is_equal([QualityPolicy.LOW])
	qm.call("unregister_world", world)
	world.free()


func test_preset_range_and_ui_is_not_scaled() -> void:
	qm.call("apply_preset", QualityPolicy.LOW, false)
	assert_float(viewport.scaling_3d_scale).is_equal_approx(0.75, 0.001)
	var dr: DynamicResolution = qm.get("resolution") as DynamicResolution
	assert_float(dr.scale_min).is_equal_approx(0.6, 0.001)
	assert_float(dr.scale_max).is_equal_approx(0.75, 0.001)
	# The UI keeps its size: only the 3D render scale changes (canvas stretch untouched).
	assert_float(get_tree().root.content_scale_factor).is_equal_approx(1.0, 0.001)
	qm.call("apply_preset", QualityPolicy.HIGH, false)
	assert_float(viewport.scaling_3d_scale).is_equal_approx(1.0, 0.001)
	assert_int(viewport.msaa_3d).is_equal(Viewport.MSAA_2X)


func test_fps_caps_thermal_and_idle() -> void:
	assert_int(QualityPolicy.fps_cap(QualityPolicy.MEDIUM, "windows")).is_equal(60)
	assert_int(QualityPolicy.fps_cap(QualityPolicy.MEDIUM, "android")).is_equal(30)
	assert_int(QualityPolicy.effective_fps_cap(60, true, 0.0)).is_equal(30)
	assert_int(QualityPolicy.effective_fps_cap(30, false, 11.0)).is_equal(QualityPolicy.IDLE_FPS)
	qm.call("set_thermal_throttled", true)
	assert_int(Engine.max_fps).is_equal(30)
	qm.call("set_thermal_throttled", false)
	assert_int(Engine.max_fps).is_equal(60)


# ---------- F062-AC-17 renderer fallback ----------

func test_renderer_that_never_drew_falls_back_to_compatibility() -> void:
	settings.set("quality_renderer", QualityPolicy.RENDERER_MOBILE)
	settings.set("renderer_boot_pending", true)
	assert_bool(qm.call("check_renderer_boot", QualityPolicy.RENDERER_MOBILE)).is_true()
	assert_str(String(settings.get("quality_renderer"))).is_equal(QualityPolicy.RENDERER_COMPATIBILITY)
	var cfg: ConfigFile = ConfigFile.new()
	assert_int(cfg.load(OVERRIDE_PATH)).is_equal(OK)
	assert_str(String(cfg.get_value("rendering", "renderer/rendering_method"))).is_equal("gl_compatibility")
	assert_int(telemetry.queue.size()).is_equal(1)
	assert_str(String(telemetry.queue[0]["type"])).is_equal("client_error")
	assert_str(String(telemetry.queue[0]["error_code"])).is_equal("renderer_fallback")


func test_engine_fallback_is_remembered_and_reported() -> void:
	assert_bool(qm.call("check_renderer_boot", QualityPolicy.RENDERER_COMPATIBILITY, QualityPolicy.RENDERER_MOBILE)).is_true()
	assert_str(String(settings.get("quality_renderer"))).is_equal(QualityPolicy.RENDERER_COMPATIBILITY)
	assert_int(telemetry.queue.size()).is_equal(1)


func test_healthy_boot_changes_nothing() -> void:
	settings.set("quality_renderer", QualityPolicy.RENDERER_MOBILE)
	settings.set("renderer_boot_pending", false)
	assert_bool(qm.call("check_renderer_boot", QualityPolicy.RENDERER_MOBILE)).is_false()
	var script: GDScript = QualityManagerScript as GDScript
	assert_bool(script.call("should_check_renderer_boot", "headless", PackedStringArray())).is_false()
	assert_bool(script.call("should_check_renderer_boot", "Windows", PackedStringArray(["--rendering-method", "mobile"]))).is_false()
	assert_bool(script.call("should_check_renderer_boot", "Windows", PackedStringArray(["-s", "res://x.gd"]))).is_false()
	assert_bool(script.call("should_check_renderer_boot", "Windows", PackedStringArray(["--path", "."]))).is_true()
	assert_bool(FileAccess.file_exists(OVERRIDE_PATH)).is_false()
	assert_bool(telemetry.queue.is_empty()).is_true()
	assert_str(QualityPolicy.default_renderer("web")).is_equal(QualityPolicy.RENDERER_COMPATIBILITY)
	assert_str(QualityPolicy.default_renderer("android", QualityPolicy.HIGH)).is_equal(QualityPolicy.RENDERER_MOBILE)
	assert_str(QualityPolicy.default_renderer("windows", QualityPolicy.HIGH)).is_equal(QualityPolicy.RENDERER_FORWARD_PLUS)
