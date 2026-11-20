extends PanelContainer
## Read-only worked example (STUDENT_GAME_UX_LEARNING §4.8, F063-AC-17): the lesson named by
## `feedback.worked_example_ref`, loaded by the host with `getLesson`, shown as its title and summary in the attempt's
## content language. No input, no navigation away from the activity.

signal closed()

@onready var _title: Label = %Title
@onready var _body: Label = %Body
@onready var _close: Button = %Close


func _ready() -> void:
	_close.pressed.connect(close)
	visible = false


## `lesson` is the decoded `Lesson` (title/summary are `LearningName`/`LearningText`).
func open_lesson(lesson: Dictionary, ctx: RendererContext) -> void:
	_title.text = ctx.text(lesson.get("title"))
	_body.text = ctx.text(lesson.get("summary"))
	_body.visible = not _body.text.is_empty()
	show()
	_close.grab_focus.call_deferred()


func close() -> void:
	if visible:
		hide()
		closed.emit()


func title_text() -> String:
	return _title.text
