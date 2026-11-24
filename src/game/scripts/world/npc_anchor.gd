class_name NpcAnchor
extends Interactable
## NPC of a zone (WORLD_DESIGN §2.3, WORLD_RENDERING §9, F071 T037): an Interactable of kind `npc` carrying the stable
## `npc_id` (`npc.mr_numbers`). Name and dialogue come from the server (`getNpcDialogue`), never from the scene (R2).
## Display (F067): the chibi `Body` takes one of `body_variants`, chosen from `npc_id` so an NPC keeps its look.

@export var npc_id: String = ""
## Baked NPC bodies (res://assets/models/chr/chr_npc_*.res); display only.
@export var body_variants: Array[Mesh] = []


func _init() -> void:
	kind = "npc"
	prompt_key = "ui.world.npc.talk"


func _ready() -> void:
	super._ready()
	var body: MeshInstance3D = get_node_or_null(^"Body") as MeshInstance3D
	if body != null and not body_variants.is_empty():
		body.mesh = body_variants[variant_index(npc_id, body_variants.size())]


## Stable variant of `id` among `count` looks (0 when there is nothing to choose).
static func variant_index(id: String, count: int) -> int:
	if count <= 0:
		return 0
	var sum: int = 0
	for i: int in id.length():
		sum = (sum * 31 + id.unicode_at(i)) % 1000003
	return sum % count
