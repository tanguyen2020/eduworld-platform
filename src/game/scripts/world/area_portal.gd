class_name AreaPortal
extends Interactable
## Wind gate (Cổng gió, WORLD_DESIGN §6, F071 T026): an Interactable of kind `portal` with a stable `interactable_id`
## (= `portal_id` of the area layout) and the target area. It only reports the player's intent; the server decides if
## the target area is open (`enterWorldArea`, ADR-0016) and the client hides nothing by itself.

@export var target_area_id: String = ""


func _init() -> void:
	kind = "portal"
	prompt_key = "ui.world.portal.travel"
