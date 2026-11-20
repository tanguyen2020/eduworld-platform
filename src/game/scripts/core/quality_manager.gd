extends Node
## Autoload #12 `QualityManager` (GODOT_ARCHITECTURE §4, PERFORMANCE_BUDGET §4–§6, §12; FR-GAME-009, NFR-COMP-001,
## ADR-0057; F062 US3). Chooses renderer and preset (first-run benchmark ≤ 5 s, `client-config.device_profiles`
## override), applies the preset to the 3D viewport, runs the dynamic resolution controller (3D only, never the UI),
## caps FPS (thermal one level lower, 15 FPS when idle > 10 s), suspends the world behind a blurred snapshot while a
## full-screen learning screen is open, and falls back to Compatibility after a renderer failure. Rules live in
## QualityPolicy / DynamicResolution (pure, unit-tested). The profile is per device (`Settings` `[quality]`), never
## sent to the server; only anonymous telemetry (`perf_sample`, `renderer_fallback`) leaves the device.

signal quality_changed(preset: String)
## The controller sat at the minimum scale and was still slow for 30 s: the Settings screen may suggest `preset`.
signal preset_suggested(preset: String)
signal world_suspended(suspended: bool)

const TELEMETRY_RENDERER_FALLBACK: String = "renderer_fallback"
## GAME_ANALYTICS §3.2: one `perf_sample` per 60 s of play.
const PERF_SAMPLE_SEC: float = 60.0
const PERF_SAMPLE_MAX_FRAMES: int = 3600
const SNAPSHOT_DOWNSCALE: int = 8
## Above the 3D world, below the HUD (layer 1+) and dialogs.
const SNAPSHOT_LAYER: int = -1
const BYTES_PER_MB: int = 1048576

## File named by `application/config/project_settings_override`: the renderer of the next run.
var override_path: String = "user://renderer_override.cfg"
## Test seams: the Settings node and the viewport the preset applies to (default: autoload / root viewport).
var settings: Node = null
var viewport: Viewport = null
var platform: String = ""
var preset: String = QualityPolicy.MEDIUM
var renderer: String = ""
var thermal_throttled: bool = false
var resolution: DynamicResolution = DynamicResolution.new()
var idle_sec: float = 0.0
var benchmark_frames: PackedFloat32Array = PackedFloat32Array()
var suspend_reasons: Dictionary = {}
## Writes the FPS cap to `Engine.max_fps`. Off when headless (CI/tests: no rendering, no battery to save — a 15 FPS
## idle cap would only slow the test runner); tests of the cap turn it on.
var apply_engine_fps: bool = DisplayServer.get_name() != "headless"

## Blurred snapshot shown while the world is suspended (null otherwise).
var snapshot_layer: CanvasLayer = null
var _world: Node = null
var _world_mode: Node.ProcessMode = Node.PROCESS_MODE_INHERIT
var _perf_frames: PackedFloat32Array = PackedFloat32Array()
var _perf_elapsed: float = 0.0


func _ready() -> void:
	process_mode = Node.PROCESS_MODE_ALWAYS
	if settings == null:
		settings = get_node_or_null("/root/Settings")
	if viewport == null:
		viewport = get_viewport()
	platform = RestApiClient.client_platform()
	resolution.lower_preset_suggested.connect(_on_lower_preset_suggested)
	_load_profile()
	if should_check_renderer_boot(DisplayServer.get_name(), OS.get_cmdline_args()):
		_check_renderer_boot.call_deferred()


# ---------- profile, preset and renderer ----------

## F062-AC-14: no preset saved on this device yet → run the benchmark before playing.
func needs_benchmark() -> bool:
	return settings == null or String(settings.get("quality_preset")).is_empty()


## Applies `preset_name` to the viewport and (when `persist`) saves it to `settings.cfg`. Never called by the controller
## (F062-AC-16: the preset changes only by benchmark, device profile or the student's choice).
func apply_preset(preset_name: String, persist: bool = true) -> void:
	preset = preset_name if QualityPolicy.is_valid_preset(preset_name) else QualityPolicy.MEDIUM
	resolution.configure(float(QualityPolicy.preset_value(preset, "render_scale")),
			float(QualityPolicy.preset_value(preset, "scale_min")), float(QualityPolicy.preset_value(preset, "scale_max")),
			QualityPolicy.target_frame_ms(fps_cap()))
	_apply_viewport()
	if persist and settings != null:
		settings.set("quality_preset", preset)
		settings.call("save")
	_update_telemetry_context()
	quality_changed.emit(preset)


## F062-AC-15: the first `client-config.device_profiles` entry matching this device forces preset and renderer
## (instead of the benchmark). Returns the matched profile or {}.
func apply_device_profiles(profiles: Array, device: Dictionary = {}) -> Dictionary:
	var info: Dictionary = device if not device.is_empty() else QualityPolicy.device_info(platform)
	var profile: Dictionary = QualityPolicy.match_device_profile(profiles, info)
	if profile.is_empty():
		return profile
	var method: String = QualityPolicy.rendering_method_of(String(profile.get("renderer", "")))
	if not method.is_empty():
		set_renderer_for_next_run(method)
	apply_preset(String(profile["preset"]))
	return profile


func fps_cap() -> int:
	return QualityPolicy.fps_cap(preset, platform)


## Preset value with the student's per-setting override (`Settings.quality_shadows/antialiasing`).
func shadows_enabled() -> bool:
	var override: int = int(settings.get("quality_shadows")) if settings != null else -1
	return override == 1 if override >= 0 else bool(QualityPolicy.preset_value(preset, "shadows"))


func antialiasing() -> String:
	var override: int = int(settings.get("quality_antialiasing")) if settings != null else -1
	if override == 0:
		return "none"
	var value: String = String(QualityPolicy.preset_value(preset, "aa"))
	return "fxaa" if override == 1 and value == "none" else value


func set_override(key: String, value: int) -> void:
	if settings == null or not key in ["quality_shadows", "quality_antialiasing"]:
		return
	settings.set(key, clampi(value, -1, 1))
	settings.call("save")
	_apply_viewport()
	quality_changed.emit(preset)


## Writes the renderer of the next start (`project_settings_override` file) and remembers it in settings.cfg.
func set_renderer_for_next_run(method: String) -> void:
	if not QualityPolicy.RENDERER_KINDS.has(method):
		return
	renderer = method
	if settings != null:
		settings.set("quality_renderer", method)
		settings.call("save")
	var cfg: ConfigFile = ConfigFile.new()
	cfg.set_value("rendering", "renderer/rendering_method", method)
	var err: Error = cfg.save(override_path)
	if err != OK:
		_log("warn", "renderer_override_save_failed", {"error": err})
	_update_telemetry_context()


## F062-AC-17: run at start-up (not headless). A previous run that never drew its first frame, or an engine that
## already fell back from Vulkan, means Compatibility next time + telemetry `renderer_fallback`.
func check_renderer_boot(current_method: String, project_method: String = "") -> bool:
	var saved: String = String(settings.get("quality_renderer")) if settings != null else ""
	var project: String = project_method if not project_method.is_empty() \
			else String(ProjectSettings.get_setting_with_override("rendering/renderer/rendering_method"))
	var configured: String = saved if not saved.is_empty() else project
	var pending: bool = settings != null and bool(settings.get("renderer_boot_pending"))
	var next: String = QualityPolicy.renderer_after_boot(configured, pending)
	if current_method == QualityPolicy.RENDERER_COMPATIBILITY and configured != QualityPolicy.RENDERER_COMPATIBILITY:
		next = QualityPolicy.RENDERER_COMPATIBILITY
	var fell_back: bool = next != configured
	if fell_back:
		set_renderer_for_next_run(next)
		_telemetry_error(TELEMETRY_RENDERER_FALLBACK)
		_log("warn", "renderer_fallback", {"from": configured, "to": next})
	else:
		renderer = configured
	return fell_back


## Only the real game checks its renderer: not headless (CI), not a tool script (`-s`), not a renderer forced on the
## command line (a developer choice, not an engine fallback).
static func should_check_renderer_boot(display: String, args: PackedStringArray) -> bool:
	if display == "headless":
		return false
	for arg: String in args:
		if arg in ["-s", "--script", "--rendering-method", "--rendering-driver"]:
			return false
	return true


## Marks the boot as in progress until the first frame is drawn.
func _mark_renderer_boot(pending: bool) -> void:
	if settings == null:
		return
	settings.set("renderer_boot_pending", pending)
	settings.call("save")


func _check_renderer_boot() -> void:
	check_renderer_boot(RenderingServer.get_current_rendering_method())
	_mark_renderer_boot(true)
	await RenderingServer.frame_post_draw
	_mark_renderer_boot(false)


# ---------- benchmark (F062-AC-14) ----------

func start_benchmark() -> void:
	benchmark_frames = PackedFloat32Array()
	apply_preset(QualityPolicy.MEDIUM, false)


func push_benchmark_frame(frame_ms: float) -> void:
	benchmark_frames.append(frame_ms)


## Chooses and saves the preset from the measured frames (medium when nothing was measured).
func finish_benchmark() -> String:
	var chosen: String = QualityPolicy.MEDIUM
	if not benchmark_frames.is_empty():
		chosen = QualityPolicy.choose_preset(QualityPolicy.fps_p10(benchmark_frames), platform)
	apply_preset(chosen)
	_log("info", "quality_benchmark_finished", {
		"preset": chosen, "fps_p10": QualityPolicy.fps_p10(benchmark_frames), "frames": benchmark_frames.size(),
	})
	return chosen


# ---------- world registration, frame loop, suspend (F062-AC-16, F062-AC-18) ----------

## Called by ZoneRoot when a zone enters/leaves the tree.
func register_world(world: Node) -> void:
	_world = world
	_world_mode = world.process_mode
	_perf_frames = PackedFloat32Array()
	_perf_elapsed = 0.0
	_apply_viewport()


func unregister_world(world: Node) -> void:
	if _world == world:
		if is_world_suspended():
			suspend_reasons.clear()
			_resume_now()
		_world = null


func is_world_suspended() -> bool:
	return not suspend_reasons.is_empty()


## A full-screen learning screen (activity player F063, exam F064) stops world physics/animation and replaces the 3D
## render with a blurred snapshot until every reason is resumed.
func suspend_world(reason: StringName) -> void:
	var first: bool = suspend_reasons.is_empty()
	suspend_reasons[reason] = true
	if not first:
		return
	_show_snapshot()
	if _world != null:
		_world_mode = _world.process_mode
		_world.process_mode = Node.PROCESS_MODE_DISABLED
	if viewport != null:
		viewport.disable_3d = true
	world_suspended.emit(true)


func resume_world(reason: StringName) -> void:
	if not suspend_reasons.has(reason):
		return
	suspend_reasons.erase(reason)
	if suspend_reasons.is_empty():
		_resume_now()


func set_thermal_throttled(throttled: bool) -> void:
	thermal_throttled = throttled
	_apply_fps()


func _resume_now() -> void:
	if _world != null:
		_world.process_mode = _world_mode
	if viewport != null:
		viewport.disable_3d = false
	if snapshot_layer != null:
		snapshot_layer.queue_free()
		snapshot_layer = null
	world_suspended.emit(false)


func _input(event: InputEvent) -> void:
	if event is InputEventMouseMotion or event is InputEventKey or event is InputEventMouseButton \
			or event is InputEventScreenTouch or event is InputEventScreenDrag or event is InputEventJoypadMotion \
			or event is InputEventJoypadButton:
		touch_activity()


## Any player input ends the idle throttle.
func touch_activity() -> void:
	var was_idle: bool = idle_sec > QualityPolicy.IDLE_AFTER_SEC
	idle_sec = 0.0
	if was_idle:
		_apply_fps()


func _process(delta: float) -> void:
	tick(delta)


## One frame of the controller (also driven by tests with synthetic frame times).
func tick(delta: float) -> void:
	var was_idle: bool = idle_sec > QualityPolicy.IDLE_AFTER_SEC
	idle_sec += delta
	if was_idle != (idle_sec > QualityPolicy.IDLE_AFTER_SEC):
		_apply_fps()
	if _world == null or is_world_suspended() or idle_sec > QualityPolicy.IDLE_AFTER_SEC:
		return
	var frame_ms: float = delta * 1000.0
	var before: float = resolution.scale
	resolution.push_frame(frame_ms)
	if not is_equal_approx(before, resolution.scale) and viewport != null:
		viewport.scaling_3d_scale = resolution.scale
	if _perf_frames.size() < PERF_SAMPLE_MAX_FRAMES:
		_perf_frames.append(frame_ms)
	_perf_elapsed += delta
	if _perf_elapsed >= PERF_SAMPLE_SEC:
		_queue_perf_sample()


func _queue_perf_sample() -> void:
	var telemetry: TelemetryService = _telemetry()
	if telemetry != null and not _perf_frames.is_empty():
		telemetry.queue_event(TelemetryService.TYPE_PERF_SAMPLE, {
			"scene_code": _current_zone(),
			"fps_p50": snappedf(QualityPolicy.fps_p50(_perf_frames), 0.1),
			"fps_p10": snappedf(QualityPolicy.fps_p10(_perf_frames), 0.1),
			"frame_ms_p95": snappedf(QualityPolicy.percentile(_perf_frames, 95.0), 0.1),
			"render_scale": resolution.scale,
			"mem_mb": int(OS.get_static_memory_usage() / BYTES_PER_MB),
		})
	_perf_frames = PackedFloat32Array()
	_perf_elapsed = 0.0


func _apply_viewport() -> void:
	if viewport != null:
		viewport.scaling_3d_scale = resolution.scale
		viewport.mesh_lod_threshold = float(QualityPolicy.preset_value(preset, "lod_threshold"))
		var aa: String = antialiasing()
		viewport.screen_space_aa = Viewport.SCREEN_SPACE_AA_FXAA if aa == "fxaa" else Viewport.SCREEN_SPACE_AA_DISABLED
		viewport.msaa_3d = Viewport.MSAA_2X if aa == "msaa2" else Viewport.MSAA_DISABLED
	_apply_fps()


func _apply_fps() -> void:
	var cap: int = QualityPolicy.effective_fps_cap(fps_cap(), thermal_throttled, idle_sec)
	if apply_engine_fps and Engine.max_fps != cap:
		Engine.max_fps = cap


func _show_snapshot() -> void:
	if viewport == null or snapshot_layer != null:
		return
	snapshot_layer = CanvasLayer.new()
	snapshot_layer.name = "WorldSnapshot"
	snapshot_layer.layer = SNAPSHOT_LAYER
	var rect: TextureRect = TextureRect.new()
	rect.name = "Snapshot"
	rect.set_anchors_preset(Control.PRESET_FULL_RECT)
	rect.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	rect.stretch_mode = TextureRect.STRETCH_SCALE
	rect.mouse_filter = Control.MOUSE_FILTER_IGNORE
	# Headless (CI) has no framebuffer to read: the overlay stays empty there.
	var can_capture: bool = DisplayServer.get_name() != "headless" and viewport.get_texture() != null
	var image: Image = viewport.get_texture().get_image() if can_capture else null
	if image != null and not image.is_empty():
		# Blur: downscale then let the TextureRect upscale with linear filtering (no extra shader pass).
		image.resize(maxi(1, image.get_width() / SNAPSHOT_DOWNSCALE), maxi(1, image.get_height() / SNAPSHOT_DOWNSCALE),
				Image.INTERPOLATE_BILINEAR)
		rect.texture = ImageTexture.create_from_image(image)
		rect.texture_filter = CanvasItem.TEXTURE_FILTER_LINEAR
	snapshot_layer.add_child(rect)
	add_child(snapshot_layer)


func _on_lower_preset_suggested() -> void:
	var index: int = QualityPolicy.PRESET_NAMES.find(preset)
	if index > 0:
		preset_suggested.emit(QualityPolicy.PRESET_NAMES[index - 1])


func _load_profile() -> void:
	var saved: String = String(settings.get("quality_preset")) if settings != null else ""
	preset = saved if QualityPolicy.is_valid_preset(saved) else QualityPolicy.MEDIUM
	renderer = String(settings.get("quality_renderer")) if settings != null else ""
	if renderer.is_empty():
		renderer = String(ProjectSettings.get_setting("rendering/renderer/rendering_method", QualityPolicy.RENDERER_MOBILE))
	apply_preset(preset, false)


func _update_telemetry_context() -> void:
	var telemetry: TelemetryService = _telemetry()
	if telemetry != null:
		telemetry.preset = preset
		telemetry.renderer = QualityPolicy.renderer_kind(renderer)


func _telemetry_error(code: String) -> void:
	var telemetry: TelemetryService = _telemetry()
	if telemetry != null:
		telemetry.queue_client_error(code, "boot")


func _telemetry() -> TelemetryService:
	var app_state: Node = get_node_or_null("/root/AppState")
	if app_state == null:
		return null
	var services: ServiceLocator = app_state.call("get_services") as ServiceLocator
	return services.telemetry if services != null else null


func _current_zone() -> String:
	var app_state: Node = get_node_or_null("/root/AppState")
	return String(app_state.get("current_zone")) if app_state != null else ""


func _log(level: String, event: String, fields: Dictionary) -> void:
	var log_node: Node = get_node_or_null("/root/Log")
	if log_node != null:
		log_node.call(level, event, fields)
