class_name NpcAnchor
extends Interactable
## NPC of a zone (WORLD_DESIGN §2.3, WORLD_RENDERING §9, F071 T037): an Interactable of kind `npc` carrying the stable
## `npc_id` (`npc.mr_numbers`). Name and dialogue come from the server (`getNpcDialogue`), never from the scene (R2).

@export var npc_id: String = ""


func _init() -> void:
	kind = "npc"
	prompt_key = "ui.world.npc.talk"
