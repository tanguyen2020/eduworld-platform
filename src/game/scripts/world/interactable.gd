class_name Interactable
extends Area3D
## Interaction anchor placed by the level designer (WORLD_RENDERING §5, GODOT_ARCHITECTURE §6, F062-AC-04): a stable
## `interactable_id`, a `kind` and the translation key of its action prompt. It carries no business data and calls no
## service: it only reports the local player entering/leaving its area and emits `interacted(interactable_id)`
## (AGENTS R1). What happens next (open a panel, accept a quest…) is decided by the zone/service layer.

signal interacted(interactable_id: String)
signal player_entered(interactable_id: String)
signal player_exited(interactable_id: String)

const KINDS: PackedStringArray = [
	"npc", "quest_board", "portal", "quest_object", "discovery", "collectible", "info_sign", "minigame_station", "seat",
]
## Group of the local player's body (PlayerController).
const PLAYER_GROUP: StringName = &"local_player"

@export var interactable_id: String = ""
@export_enum("npc", "quest_board", "portal", "quest_object", "discovery", "collectible", "info_sign",
		"minigame_station", "seat") var kind: String = "info_sign"
## i18n key of the prompt, e.g. `ui.hub.action.read` ("Xem").
@export var prompt_key: String = ""

var player_inside: bool = false


func _ready() -> void:
	monitoring = true
	monitorable = false
	body_entered.connect(_on_body_entered)
	body_exited.connect(_on_body_exited)


## The player confirmed the action (key `interact`, the ContextActionButton or a tap).
func interact() -> void:
	interacted.emit(interactable_id)


func _on_body_entered(body: Node3D) -> void:
	if body.is_in_group(PLAYER_GROUP):
		player_inside = true
		player_entered.emit(interactable_id)


func _on_body_exited(body: Node3D) -> void:
	if body.is_in_group(PLAYER_GROUP):
		player_inside = false
		player_exited.emit(interactable_id)
