extends Node
## TEST-ONLY benchmark scene of the Main Hub (F062-AC-19/20, TC-PERF-018, TC-PERF-020; PERFORMANCE_BUDGET §7, §9, §11).
## - Headless (CI, `tests/perf/test_main_hub_benchmark.gd`): static budget of the hub (F067 art) — visible triangles,
##   draw-call proxy (mesh surfaces × material passes, MultiMeshes, 3D labels), dynamic bodies — and HUD cost (Control count, render time per update).
## - With a renderer (`godot --path src/game res://tests/perf/MainHubBenchmark.tscn -- --seconds=20`): orbits the camera,
##   records frame times and the engine's per-frame primitives/draw calls, prints one JSON line and quits (lab devices).
## Budgets are CONFIG_DEFAULTS `scene_budget_hub`, `ui_cpu_budget`, `perf_budget_tolerance_ci` (Mobile/Web column).

## `scene_budget_hub` (Mobile/Web, preset medium) and `ui_cpu_budget` (mobile); +5 % CI tolerance.
const BUDGET_TRIANGLES: int = 150000
const BUDGET_DRAW_CALLS: int = 100
const BUDGET_DYNAMIC_BODIES: int = 60
const BUDGET_HUD_CONTROLS: int = 150
const BUDGET_HUD_MS: float = 3.0
const TOLERANCE: float = 1.05
const ZONE: String = "res://scenes/world/zones/main_hub/Zone.tscn"
const HUD: String = "res://scenes/ui/hud/MainHud.tscn"
const DEFAULT_SECONDS: float = 20.0

var zone: Node3D = null
var hud: CanvasLayer = null
var seconds: float = DEFAULT_SECONDS
var load_ms: int = 0
var _frames: PackedFloat32Array = PackedFloat32Array()
var _primitives: Array[int] = []
var _draw_calls: Array[int] = []
var _elapsed: float = 0.0


func _ready() -> void:
	var started: int = Time.get_ticks_msec()
	zone = (load(ZONE) as PackedScene).instantiate() as Node3D
	add_child(zone)
	hud = (load(HUD) as PackedScene).instantiate() as CanvasLayer
	add_child(hud)
	load_ms = Time.get_ticks_msec() - started
	for arg: String in OS.get_cmdline_user_args():
		if arg.begins_with("--seconds="):
			seconds = arg.trim_prefix("--seconds=").to_float()
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


## Measured run (renderer only): FPS percentiles and the engine's worst-frame primitives/draw calls.
func report() -> Dictionary:
	return {
		"scene": "main_hub", "load_ms": load_ms, "frames": _frames.size(),
		"fps_p50": QualityPolicy.fps_p50(_frames), "fps_p10": QualityPolicy.fps_p10(_frames),
		"frame_ms_p95": QualityPolicy.percentile(_frames, 95.0),
		"max_primitives": _primitives.max() if not _primitives.is_empty() else 0,
		"max_draw_calls": _draw_calls.max() if not _draw_calls.is_empty() else 0,
		"static": scene_stats(zone), "hud": hud_stats(hud),
		"preset": QualityManager.preset, "renderer": RenderingServer.get_current_rendering_method(),
	}


## Static budget of a 3D subtree: triangles of visible meshes (MultiMesh: × drawn instances), draw-call proxy (one
## per mesh surface and material pass — the F067 toon outline is a second pass — or 3D label) and dynamic bodies
## (characters + rigid bodies).
static func scene_stats(root: Node) -> Dictionary:
	var triangles: int = 0
	var draw_calls: int = 0
	var dynamic_bodies: int = 0
	for node: Node in root.find_children("*", "", true, false):
		if node is MeshInstance3D and (node as MeshInstance3D).mesh != null and (node as MeshInstance3D).is_visible_in_tree():
			var mi: MeshInstance3D = node as MeshInstance3D
			triangles += mi.mesh.get_faces().size() / 3
			for s: int in mi.mesh.get_surface_count():
				draw_calls += material_passes(mi.material_override if mi.material_override != null else mi.get_active_material(s))
		elif node is MultiMeshInstance3D and (node as MultiMeshInstance3D).is_visible_in_tree():
			var mmi: MultiMeshInstance3D = node as MultiMeshInstance3D
			if mmi.multimesh != null and mmi.multimesh.mesh != null:
				var count: int = mmi.multimesh.visible_instance_count
				count = mmi.multimesh.instance_count if count < 0 else count
				triangles += mmi.multimesh.mesh.get_faces().size() / 3 * count
				for s: int in mmi.multimesh.mesh.get_surface_count():
					draw_calls += material_passes(mmi.material_override)
		elif node is Label3D:
			draw_calls += 1
		if node is CharacterBody3D or node is RigidBody3D:
			dynamic_bodies += 1
	return {"triangles": triangles, "draw_calls": draw_calls, "dynamic_bodies": dynamic_bodies}


## Render passes of a material: itself plus its `next_pass` chain (at least one).
static func material_passes(material: Material) -> int:
	var passes: int = 1
	var next: Material = material.next_pass if material != null else null
	while next != null:
		passes += 1
		next = next.next_pass
	return passes


## HUD cost: number of Controls and the average time of one full re-render (`_render`) in ms.
static func hud_stats(layer: CanvasLayer, iterations: int = 50) -> Dictionary:
	var controls: int = layer.find_children("*", "Control", true, false).size()
	var started: int = Time.get_ticks_usec()
	for i: int in iterations:
		layer.call("_render")
	var per_update_ms: float = float(Time.get_ticks_usec() - started) / 1000.0 / float(iterations)
	return {"controls": controls, "update_ms": per_update_ms}
