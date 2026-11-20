class_name MatchingAnswer
extends AnswerModel
## `matching` → `{pairs: [[left_id, right_id], ...]}` (in left display order). Pairs are made by drag (PC) or by
## tapping a left card then a right card (mobile, STUDENT_GAME_UX_LEARNING §4.3, F063-AC-02).

## `MatchingResponse.pairs` maxItems.
const MAX_PAIRS: int = 10

var left: PackedStringArray = []
var right: PackedStringArray = []
## left id → right id.
var pairs: Dictionary = {}
## Left card picked by a first tap (empty when none).
var picked_left: String = ""


func _on_setup() -> void:
	left = AnswerModel.ids_of(payload.get("left"))
	right = AnswerModel.ids_of(payload.get("right"))
	pairs = {}
	picked_left = ""


func pair(left_id: String, right_id: String) -> void:
	if not left_id in left or not right_id in right:
		return
	if not pairs.has(left_id) and pairs.size() >= MAX_PAIRS:
		return
	for other: Variant in pairs.keys():
		if String(pairs[other]) == right_id and String(other) != left_id:
			pairs.erase(other)
	pairs[left_id] = right_id
	picked_left = ""
	_notify()


func unpair(left_id: String) -> void:
	if pairs.erase(left_id):
		_notify()


## Tap on a left card: picks it (tap again to cancel).
func tap_left(left_id: String) -> void:
	if not left_id in left:
		return
	picked_left = "" if picked_left == left_id else left_id
	_notify()


## Tap on a right card: pairs it with the picked left card.
func tap_right(right_id: String) -> void:
	if not picked_left.is_empty():
		pair(picked_left, right_id)


func partner_of(left_id: String) -> String:
	return String(pairs.get(left_id, ""))


func is_complete() -> bool:
	return not left.is_empty() and pairs.size() >= mini(left.size(), MAX_PAIRS)


func to_response() -> Dictionary:
	if not is_complete():
		return {}
	var out: Array = []
	for id: String in left:
		if pairs.has(id):
			out.append([id, String(pairs[id])])
	return {"pairs": out}


func restore(response: Dictionary) -> void:
	var saved: Variant = response.get("pairs")
	if not saved is Array:
		return
	for entry: Variant in (saved as Array):
		if entry is Array and (entry as Array).size() == 2:
			pair(String((entry as Array)[0]), String((entry as Array)[1]))
