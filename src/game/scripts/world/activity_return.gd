class_name ActivityReturn
extends RefCounted
## Learning opened from an NPC or a station of the world (FR-GAME-018, F071-AC-20): remembers where the player stood;
## when the activity screen closes (after its summary), the player goes back to that spot once and nothing else opens
## by itself (no chaining into another activity, GAME_DESIGN §7.2).

const ACTIVITY_SCREENS: Array[StringName] = [&"activity_host", &"minigame_host"]

var area_id: String = ""
var position: Vector3 = Vector3.ZERO
var armed: bool = false


func remember(at_area: String, at: Vector3) -> void:
	area_id = at_area
	position = at
	armed = true


## The screen `screen_id` closed in `current_area`: returns true (and `position`) once when the player must go back.
func on_closed(screen_id: StringName, current_area: String) -> bool:
	if not armed or not screen_id in ACTIVITY_SCREENS:
		return false
	armed = false
	return current_area == area_id
