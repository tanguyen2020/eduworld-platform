class_name DropButton
extends Button
## A card that can be dragged (`drag_source_id`) and/or receive a drop (`drop_target_id`) — matching, drag & drop
## and label questions on PC. Touch devices use the tap-tap path of the answer models (F063-AC-02).

signal dropped(source_id: String, target_id: String)

const DRAG_KEY: String = "eduworld_drag"

var drag_source_id: String = ""
var drop_target_id: String = ""
var drag_enabled: bool = true


func _get_drag_data(_at_position: Vector2) -> Variant:
	if drag_source_id.is_empty() or not drag_enabled:
		return null
	var preview: Label = Label.new()
	preview.text = text
	preview.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
	set_drag_preview(preview)
	return {DRAG_KEY: drag_source_id}


func _can_drop_data(_at_position: Vector2, data: Variant) -> bool:
	return not drop_target_id.is_empty() and not disabled and data is Dictionary and (data as Dictionary).has(DRAG_KEY)


func _drop_data(_at_position: Vector2, data: Variant) -> void:
	dropped.emit(String((data as Dictionary)[DRAG_KEY]), drop_target_id)
