class_name QuestionRendererRegistry
extends RefCounted
## `QuestionType` → question scene (GODOT_ARCHITECTURE §6.2, QUESTION_BANK). Used by the `quiz` and
## `interactive_exercise` renderers (and later the exam renderer, F064). A type missing here shows "update required"
## inside the item (F063-AC-26).

const SCENES: Dictionary = {
	"single_choice": "res://scenes/learning/questions/SingleChoice.tscn",
	"multiple_choice": "res://scenes/learning/questions/MultipleChoice.tscn",
	"true_false": "res://scenes/learning/questions/TrueFalse.tscn",
	"fill_in_blank": "res://scenes/learning/questions/FillInBlank.tscn",
	"short_answer": "res://scenes/learning/questions/ShortAnswer.tscn",
	"numeric": "res://scenes/learning/questions/Numeric.tscn",
	"matching": "res://scenes/learning/questions/Matching.tscn",
	"ordering": "res://scenes/learning/questions/Ordering.tscn",
	"drag_drop": "res://scenes/learning/questions/DragDrop.tscn",
	"image_based": "res://scenes/learning/questions/ImageBased.tscn",
	"interactive": "res://scenes/learning/questions/Interactive.tscn",
}


static func has(question_type: String) -> bool:
	return SCENES.has(question_type)


static func scene_path(question_type: String) -> String:
	return String(SCENES.get(question_type, ""))


## A new question scene for `question_type`, null when this client has none.
static func instantiate(question_type: String) -> QuestionRendererBase:
	var path: String = scene_path(question_type)
	if path.is_empty() or not ResourceLoader.exists(path):
		return null
	return (load(path) as PackedScene).instantiate() as QuestionRendererBase
