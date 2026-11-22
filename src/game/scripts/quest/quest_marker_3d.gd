class_name QuestMarker3D
extends Node3D
## Marker above an interactable that is a target of an active step of the tracked quest (F072): placed and removed by
## ZoneRoot.mark_quest_targets from the delivery the server sent (`interactable_ids` of the step). Bobs slowly; purely
## visual (AGENTS R1).

const HEIGHT_M: float = 2.2
const BOB_M: float = 0.15
const BOB_HZ: float = 0.5

var _t: float = 0.0


func _ready() -> void:
	position = Vector3(0.0, HEIGHT_M, 0.0)


func _process(delta: float) -> void:
	_t += delta
	position.y = HEIGHT_M + sin(_t * TAU * BOB_HZ) * BOB_M
