class_name ZoneLoader
extends RefCounted
## Loads the zone scene of an area for WorldRoot (WORLD_RENDERING §3, F071-AC-12): the scene path comes from the
## server (`WorldArea.scene_ref`), never from a list in the build (R2). Checks the transition against
## `SceneRouter.AREA_TRANSITION_MAX_SEC` and logs a warning when it is exceeded.


## Instantiates the zone scene at `scene_ref` (null when it is not a ZoneRoot scene).
static func instantiate(scene_ref: String) -> ZoneRoot:
	if scene_ref.is_empty() or not ResourceLoader.exists(scene_ref):
		return null
	var packed: PackedScene = load(scene_ref) as PackedScene
	if packed == null:
		return null
	var node: Node = packed.instantiate()
	if node is ZoneRoot:
		return node as ZoneRoot
	node.free()
	return null


## Logs the transition time (ms) of `area_id`; true when it is within `area_transition_max`.
static func check_budget(area_id: String, elapsed_ms: int) -> bool:
	var ok: bool = elapsed_ms <= int(SceneRouter.AREA_TRANSITION_MAX_SEC * 1000.0)
	if ok:
		Log.info("area_transition", {"area_id": area_id, "elapsed_ms": elapsed_ms})
	else:
		Log.warn("area_transition_over_budget", {"area_id": area_id, "elapsed_ms": elapsed_ms})
	return ok
