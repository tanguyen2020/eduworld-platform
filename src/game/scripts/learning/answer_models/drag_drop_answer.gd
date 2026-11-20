class_name DragDropAnswer
extends AnswerModel
## `drag_drop` → `{placements: {draggable_id: zone_id}}`. Cards are dragged onto zones (PC) or tapped then the zone
## tapped (mobile alternative, STUDENT_GAME_UX_LEARNING §4.3, F063-AC-02). Zone `capacity` (null = unlimited) is
## respected; the answer is complete when every card is placed.

## `DragDropResponse.placements` maxProperties.
const MAX_PLACEMENTS: int = 16

var draggables: PackedStringArray = []
var zones: PackedStringArray = []
var capacity: Dictionary = {}
var placements: Dictionary = {}
var picked: String = ""


func _on_setup() -> void:
	draggables = AnswerModel.ids_of(payload.get("draggables"))
	zones = AnswerModel.ids_of(payload.get("zones"))
	capacity = {}
	placements = {}
	picked = ""
	var raw: Variant = payload.get("zones", [])
	if raw is Array:
		for z: Variant in (raw as Array):
			if z is Dictionary and ((z as Dictionary).get("capacity") is int or (z as Dictionary).get("capacity") is float):
				capacity[String((z as Dictionary).get("id", ""))] = int((z as Dictionary)["capacity"])


func place(draggable_id: String, zone_id: String) -> bool:
	if not draggable_id in draggables or not zone_id in zones:
		return false
	if not placements.has(draggable_id) and placements.size() >= MAX_PLACEMENTS:
		return false
	if capacity.has(zone_id) and count_in(zone_id) >= int(capacity[zone_id]) \
			and String(placements.get(draggable_id, "")) != zone_id:
		return false
	placements[draggable_id] = zone_id
	picked = ""
	_notify()
	return true


func remove(draggable_id: String) -> void:
	if placements.erase(draggable_id):
		_notify()


func tap_card(draggable_id: String) -> void:
	if draggable_id in draggables:
		picked = "" if picked == draggable_id else draggable_id
		_notify()


func tap_zone(zone_id: String) -> void:
	if not picked.is_empty():
		place(picked, zone_id)


func count_in(zone_id: String) -> int:
	var n: int = 0
	for id: Variant in placements.keys():
		if String(placements[id]) == zone_id:
			n += 1
	return n


func zone_of(draggable_id: String) -> String:
	return String(placements.get(draggable_id, ""))


func is_complete() -> bool:
	return not draggables.is_empty() and placements.size() >= mini(draggables.size(), MAX_PLACEMENTS)


func to_response() -> Dictionary:
	return {"placements": placements.duplicate()} if is_complete() else {}


func restore(response: Dictionary) -> void:
	var saved: Variant = response.get("placements")
	if saved is Dictionary:
		for id: Variant in (saved as Dictionary).keys():
			place(String(id), String((saved as Dictionary)[id]))
