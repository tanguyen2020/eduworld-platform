class_name ImageBasedAnswer
extends AnswerModel
## `image_based` → `{points: [{x, y}]}` (hotspot: taps in original image coordinates, `image_size`) or
## `{label_placements: {label_id: drop_point_id}}` (label). The client never knows the regions (answer key,
## server only, ADR-0016).

const INTERACTION_HOTSPOT: String = "hotspot"
const INTERACTION_LABEL: String = "label"
## `ImageBasedResponse` maxItems / maxProperties.
const MAX_POINTS: int = 10
const MAX_LABELS: int = 16

var interaction: String = INTERACTION_HOTSPOT
var image_size: Vector2 = Vector2.ONE
var max_selections: int = 1
var points: Array[Vector2] = []
var labels: PackedStringArray = []
var drop_points: PackedStringArray = []
var label_placements: Dictionary = {}
var picked_label: String = ""


func _on_setup() -> void:
	interaction = INTERACTION_LABEL if payload.get("interaction") == INTERACTION_LABEL else INTERACTION_HOTSPOT
	var size: Variant = payload.get("image_size", {})
	if size is Dictionary:
		image_size = Vector2(maxf(1.0, float((size as Dictionary).get("w", 1))), maxf(1.0, float((size as Dictionary).get("h", 1))))
	var sel: Variant = payload.get("max_selections")
	max_selections = clampi(int(sel), 1, MAX_POINTS) if sel is int or sel is float else 1
	points = []
	labels = AnswerModel.ids_of(payload.get("labels"))
	drop_points = AnswerModel.ids_of(payload.get("drop_points"))
	label_placements = {}
	picked_label = ""


## Hotspot tap at `pos` in original image coordinates; the oldest point is replaced beyond `max_selections`.
func add_point(pos: Vector2) -> void:
	if interaction != INTERACTION_HOTSPOT:
		return
	points.append(Vector2(clampf(pos.x, 0.0, image_size.x), clampf(pos.y, 0.0, image_size.y)))
	while points.size() > max_selections:
		points.pop_front()
	_notify()


func clear_points() -> void:
	points.clear()
	_notify()


func place_label(label_id: String, point_id: String) -> void:
	if label_id in labels and point_id in drop_points and label_placements.size() < MAX_LABELS:
		label_placements[label_id] = point_id
		picked_label = ""
		_notify()


func tap_label(label_id: String) -> void:
	if label_id in labels:
		picked_label = "" if picked_label == label_id else label_id
		_notify()


func tap_drop_point(point_id: String) -> void:
	if not picked_label.is_empty():
		place_label(picked_label, point_id)


func is_complete() -> bool:
	if interaction == INTERACTION_LABEL:
		return not labels.is_empty() and label_placements.size() >= mini(labels.size(), MAX_LABELS)
	return not points.is_empty()


func to_response() -> Dictionary:
	if not is_complete():
		return {}
	if interaction == INTERACTION_LABEL:
		return {"label_placements": label_placements.duplicate()}
	var out: Array = []
	for p: Vector2 in points:
		out.append({"x": snappedf(p.x, 0.01), "y": snappedf(p.y, 0.01)})
	return {"points": out}


func restore(response: Dictionary) -> void:
	var saved_points: Variant = response.get("points")
	if saved_points is Array:
		for p: Variant in (saved_points as Array):
			if p is Dictionary:
				add_point(Vector2(float((p as Dictionary).get("x", 0)), float((p as Dictionary).get("y", 0))))
	var saved_labels: Variant = response.get("label_placements")
	if saved_labels is Dictionary:
		for id: Variant in (saved_labels as Dictionary).keys():
			place_label(String(id), String((saved_labels as Dictionary)[id]))
