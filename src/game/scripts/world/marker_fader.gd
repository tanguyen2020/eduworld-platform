class_name MarkerFader
extends Node
## Hides world markers that come too close to the camera (F097, F096-4 — owner order "chốt hết"; WORLD_RENDERING §5.1):
## a marker (`WorldAnchor` Tia Sáng stone, `QuestMarker3D`) in the group `camera_fade` whose distance to the active
## camera is below `eduworld/world/marker_near_fade_m` and which is nearer to the camera than the local player is hidden,
## so it never stands between the camera and the player (Hub spawn: `quest.hub.welcome_sign` right in front of the
## camera), while a marker beside or beyond the player stays visible even with the camera zoomed in. It shows again
## past the distance plus a small hysteresis, so it does not flicker while the camera moves. Rendering only: no seed object moves, the interactable
## (Area3D) keeps working, nothing is reported to a service (AGENTS R1). Checked a few times per second, not per frame.

const GROUP: StringName = &"camera_fade"
const SETTING: String = "eduworld/world/marker_near_fade_m"
## Default of the setting (CONFIG_DEFAULTS `marker_near_fade_m`).
const DEFAULT_NEAR_M: float = 5.0
## Extra distance before a hidden marker shows again (no flicker at the edge).
const HYSTERESIS_M: float = 0.5
const CHECK_INTERVAL_SEC: float = 0.1

var near_m: float = DEFAULT_NEAR_M
## Markers hidden by this fader (it only shows again what it hid itself).
var hidden: Array[Node3D] = []
var _accum: float = 0.0


func _ready() -> void:
	name = "MarkerFader"
	near_m = float(ProjectSettings.get_setting(SETTING, DEFAULT_NEAR_M))


func _process(delta: float) -> void:
	_accum += delta
	if _accum < CHECK_INTERVAL_SEC:
		return
	_accum = 0.0
	var camera: Camera3D = get_viewport().get_camera_3d() if get_viewport() != null else null
	if camera == null:
		return
	var player: Node3D = get_tree().get_first_node_in_group(Interactable.PLAYER_GROUP) as Node3D
	var player_distance: float = INF if player == null else player.global_position.distance_to(camera.global_position)
	refresh(camera.global_position, player_distance)


## Applies the rule for a camera at `camera_position` with the player `player_distance` away (also used by tests).
func refresh(camera_position: Vector3, player_distance: float = INF) -> void:
	for node: Node in get_tree().get_nodes_in_group(GROUP):
		var marker: Node3D = node as Node3D
		if marker == null or not marker.is_inside_tree():
			continue
		var distance: float = marker.global_position.distance_to(camera_position)
		var was_hidden: bool = marker in hidden
		if not was_hidden and marker.visible and should_hide(distance, player_distance):
			marker.visible = false
			hidden.append(marker)
		elif was_hidden and not should_hide(distance - HYSTERESIS_M, player_distance - HYSTERESIS_M):
			marker.visible = true
			hidden.erase(marker)
	for i: int in range(hidden.size() - 1, -1, -1):
		if not is_instance_valid(hidden[i]):
			hidden.remove_at(i)


## The rule: near the camera and in front of the player (between the camera and the player).
func should_hide(distance: float, player_distance: float = INF) -> bool:
	return distance < near_m and distance < player_distance
