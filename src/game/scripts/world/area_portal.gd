class_name AreaPortal
extends Interactable
## Wind gate (Cổng gió, WORLD_DESIGN §6, F071 T026): an Interactable of kind `portal` with a stable `interactable_id`
## (= `portal_id` of the area layout) and the target area. It only reports the player's intent; the server decides if
## the target area is open (`enterWorldArea`, ADR-0016) and the client hides nothing by itself.
## Display (F067): the `Gate` mesh shows the destination's shape icon when a baked gate exists for `target_area_id`
## (res://assets/models/prop/prop_wind_gate_<area>.res), otherwise the default gate.

const GATE_DIR: String = "res://assets/models/prop"
## Same shape as the contract area ids: server data never builds an arbitrary resource path.
const AREA_ID_PATTERN: String = "^[a-z0-9_.]{1,64}$"

@export var target_area_id: String = ""


func _init() -> void:
	kind = "portal"
	prompt_key = "ui.world.portal.travel"


func _ready() -> void:
	super._ready()
	var gate: MeshInstance3D = get_node_or_null(^"Gate") as MeshInstance3D
	var mesh: Mesh = gate_mesh(target_area_id)
	if gate != null and mesh != null:
		gate.mesh = mesh


## Baked gate of `area_id`, or null (unknown id, no gate for it on this client).
static func gate_mesh(area_id: String) -> Mesh:
	if RegEx.create_from_string(AREA_ID_PATTERN).search(area_id) == null:
		return null
	var path: String = "%s/prop_wind_gate_%s.res" % [GATE_DIR, area_id.replace(".", "_")]
	return load(path) as Mesh if ResourceLoader.exists(path) else null
