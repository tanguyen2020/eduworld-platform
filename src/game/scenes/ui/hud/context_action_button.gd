extends Button
## ContextActionButton (STUDENT_GAME_UX_CORE S03, F062-AC-04): shows the prompt of the Interactable the player stands
## in (translation key `prompt_key`, e.g. "Xem") and hides when there is none. Pressing it is the same intent as the
## `interact` key; the world decides what happens.

var prompt_key: String = ""
var interactable_id: String = ""


func _ready() -> void:
	visible = false


func show_prompt(id: String, key: String) -> void:
	interactable_id = id
	prompt_key = key
	visible = not id.is_empty()
	text = key if not key.is_empty() else "ui.hub.action.interact"
	tooltip_text = text
