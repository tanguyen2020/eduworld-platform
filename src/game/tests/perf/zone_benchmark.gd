extends Node
## TEST-ONLY benchmark scene of a subject zone (F093-AC-03, TC-PERF-018; PERFORMANCE_BUDGET §7 row "Vùng môn học /
## khám phá", CONFIG_DEFAULTS `scene_budget_zone`, `perf_budget_tolerance_ci`). Same method as the Main Hub benchmark
## (main_hub_benchmark.gd: static stats headless, the engine's per-frame primitives / draw calls with a renderer):
##   godot --path src/game --rendering-method mobile --resolution 1280x720 res://tests/perf/ZoneBenchmark.tscn \
##         -- --area=learning_zone.math --seconds=12
## and `--rendering-method gl_compatibility` for the Web renderer. Prints one JSON line and quits.

const HubBench := preload("res://tests/perf/main_hub_benchmark.gd")
## `scene_budget_zone` (Mobile/Web, preset medium): visible primitives, draw calls, dynamic bodies; +5 % CI tolerance.
const BUDGET_TRIANGLES: int = 120000
const BUDGET_DRAW_CALLS: int = 90
const BUDGET_DYNAMIC_BODIES: int = 50
const TOLERANCE: float = 1.05
const ZONES_DIR: String = "res://scenes/world/zones"
const DEFAULT_SECONDS: float = 12.0

@export var area: String = "learning_zone.math"

var zone: Node3D = null
var seconds: float = DEFAULT_SECONDS
var load_ms: int = 0
var _frames: PackedFloat32Array = PackedFloat32Array()
var _primitives: Array[int] = []
var _draw_calls: Array[int] = []
var _elapsed: float = 0.0


static func zone_path(area_id: String) -> String:
	return "%s/%s/Zone.tscn" % [ZONES_DIR, area_id.replace(".", "_")]


func _ready() -> void:
	for arg: String in OS.get_cmdline_user_args():
		if arg.begins_with("--seconds="):
			seconds = arg.trim_prefix("--seconds=").to_float()
		elif arg.begins_with("--area=") and RegEx.create_from_string("^[a-z0-9_.]{1,64}$").search(arg.trim_prefix("--area=")) != null:
			area = arg.trim_prefix("--area=")
	var started: int = Time.get_ticks_msec()
	zone = (load(zone_path(area)) as PackedScene).instantiate() as Node3D
	add_child(zone)
	load_ms = Time.get_ticks_msec() - started
	set_process(DisplayServer.get_name() != "headless" and get_tree().current_scene == self)


func _process(delta: float) -> void:
	_elapsed += delta
	_frames.append(delta * 1000.0)
	_primitives.append(RenderingServer.get_rendering_info(RenderingServer.RENDERING_INFO_TOTAL_PRIMITIVES_IN_FRAME))
	_draw_calls.append(RenderingServer.get_rendering_info(RenderingServer.RENDERING_INFO_TOTAL_DRAW_CALLS_IN_FRAME))
	var rig: CameraRig = (zone as ZoneRoot).player.get("camera_rig") as CameraRig if (zone as ZoneRoot).player != null else null
	if rig != null:
		rig.set_yaw(_elapsed / seconds * TAU)
	if _elapsed >= seconds:
		set_process(false)
		print(JSON.stringify(report()))
		get_tree().quit()


func report() -> Dictionary:
	return {
		"scene": area, "load_ms": load_ms, "frames": _frames.size(),
		"fps_p50": QualityPolicy.fps_p50(_frames), "fps_p10": QualityPolicy.fps_p10(_frames),
		"frame_ms_p95": QualityPolicy.percentile(_frames, 95.0),
		"max_primitives": _primitives.max() if not _primitives.is_empty() else 0,
		"max_draw_calls": _draw_calls.max() if not _draw_calls.is_empty() else 0,
		"static": HubBench.scene_stats(zone), "preset": QualityManager.preset,
		"renderer": RenderingServer.get_current_rendering_method(),
	}
