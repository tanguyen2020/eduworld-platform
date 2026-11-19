class_name UiComponent
extends Container
## Base of the reusable UI components (DESIGN_SYSTEM §5.1): styling comes only from the generated Theme; texts are
## translation keys set by the parent scene (auto-translated, so a locale change updates them without restart).
## Long strings wrap (autowrap) instead of being cut (`text_expansion_allowance`, F060-AC-17).

@export var title_key: String = ""
@export var body_key: String = ""
@export var confirm_key: String = ""
@export var cancel_key: String = ""


func _ready() -> void:
	_set_text(&"Title", title_key)
	_set_text(&"Body", body_key)
	_set_text(&"Confirm", confirm_key)
	_set_text(&"Cancel", cancel_key)


func _set_text(unique_name: StringName, key: String) -> void:
	var node: Node = get_node_or_null("%" + String(unique_name))
	if node == null:
		return
	node.set("text", key)
	node.set("visible", not key.is_empty())
