extends Node
## Autoload #9 `SceneRouter` (GODOT_ARCHITECTURE §4): scene changes with threaded loading. It reports progress
## through signals so a loading screen can render it; it knows nothing about quests or business rules.

signal scene_load_started(path: String)
signal scene_load_progressed(path: String, progress: float)
signal scene_changed(path: String)
signal scene_load_failed(path: String)

var current_path: String = ""
var _pending_path: String = ""
var _progress: Array = []


func _ready() -> void:
	set_process(false)


func is_loading() -> bool:
	return not _pending_path.is_empty()


## Starts a threaded load of `path`; the scene is switched when loading completes.
func goto(path: String) -> Error:
	if is_loading():
		return ERR_BUSY
	if not ResourceLoader.exists(path):
		scene_load_failed.emit(path)
		return ERR_FILE_NOT_FOUND
	var err: Error = ResourceLoader.load_threaded_request(path, "PackedScene")
	if err != OK:
		scene_load_failed.emit(path)
		return err
	_pending_path = path
	scene_load_started.emit(path)
	set_process(true)
	return OK


func _process(_delta: float) -> void:
	if _pending_path.is_empty():
		set_process(false)
		return
	var status: ResourceLoader.ThreadLoadStatus = ResourceLoader.load_threaded_get_status(_pending_path, _progress)
	match status:
		ResourceLoader.THREAD_LOAD_IN_PROGRESS:
			if not _progress.is_empty():
				scene_load_progressed.emit(_pending_path, float(_progress[0]))
		ResourceLoader.THREAD_LOAD_LOADED:
			var path: String = _pending_path
			_pending_path = ""
			set_process(false)
			var scene: PackedScene = ResourceLoader.load_threaded_get(path) as PackedScene
			if scene == null or get_tree().change_scene_to_packed(scene) != OK:
				scene_load_failed.emit(path)
				return
			current_path = path
			scene_changed.emit(path)
		_:
			var failed: String = _pending_path
			_pending_path = ""
			set_process(false)
			scene_load_failed.emit(failed)


# ---------- F071: zones of the world (WORLD_RENDERING §3) ----------

## A zone change was accepted by the server: WorldRoot swaps the zone (ZoneLoader) and places the player at `spawn`.
signal area_entered(area_id: String, scene_ref: String, spawn_point_id: String, enter: Dictionary)
## The server refused the area (`AREA_LOCKED` with `missing`, `CONFLICT`, `NOT_FOUND`…) or its bundle is not available.
signal area_change_failed(area_id: String, code: String, missing: Array)

## CONFIG_DEFAULTS `area_transition_max` (< 5 s on a Mid-tier device, NFR-PERF-005, scope code): checked and logged by
## WorldRoot once the zone is placed.
const AREA_TRANSITION_MAX_SEC: float = 5.0
const CODE_BUNDLE_MISSING: String = "BUNDLE_MISSING"

var _area_pending: String = ""


func is_changing_area() -> bool:
	return not _area_pending.is_empty()


## Coroutine (F071-AC-12): `enterWorldArea` through WorldService → `ensure_bundle` → `area_entered`. Returns OK when the
## zone may be swapped; the refusal goes to `area_change_failed` (the HUD explains it, never the scene).
func request_area(area_id: String, portal_id: String = "", world: WorldService = null) -> Error:
	if not _area_pending.is_empty():
		return ERR_BUSY
	if world == null:
		var services: Object = AppState.services
		world = services.get("world") as WorldService if services != null else null
	if world == null:
		area_change_failed.emit(area_id, ApiResult.CODE_NETWORK_ERROR, [])
		return ERR_UNAVAILABLE
	_area_pending = area_id
	var result: ApiResult = await world.enter_area(area_id, portal_id)
	if not result.ok:
		_area_pending = ""
		area_change_failed.emit(area_id, result.code, WorldService.missing_requirements(result))
		return FAILED
	var area: WorldAreaModel = world.area(area_id)
	if area == null:
		await world.fetch_areas()
		area = world.area(area_id)
	var scene_ref: String = area.scene_ref if area != null else ""
	if not ensure_bundle(scene_ref):
		_area_pending = ""
		area_change_failed.emit(area_id, CODE_BUNDLE_MISSING, [])
		return ERR_FILE_NOT_FOUND
	var enter: Dictionary = result.body()
	AppState.current_zone = area_id
	AppState.spawn_point_id = String(enter.get("spawn_point_id", ""))
	_area_pending = ""
	area_entered.emit(area_id, scene_ref, AppState.spawn_point_id, enter)
	return OK


## The zone scene of a level bundle is loadable. PLACEHOLDER (R17, OQ-16): the greybox zones of M2 ship inside the
## client, so a bundle is "ready" when its scene exists; downloading PCK bundles by url/sha256 comes with the real art.
static func ensure_bundle(scene_ref: String) -> bool:
	return not scene_ref.is_empty() and ResourceLoader.exists(scene_ref)
