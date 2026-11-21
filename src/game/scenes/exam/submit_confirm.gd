extends PanelContainer
## "Nộp bài?" (STUDENT_GAME_UX_LEARNING §9.4, F064-AC-07): lists the questions without an answer and the flagged ones
## before the runner submits. Emits the choice; the runner and ExamService do the submit. UI only.

signal confirmed()
signal cancelled()

@onready var _unanswered: Label = %Unanswered
@onready var _flagged: Label = %Flagged
@onready var _all_done: Label = %AllDone
@onready var _confirm: Button = %Confirm
@onready var _cancel: Button = %Cancel


func _ready() -> void:
	_confirm.pressed.connect(confirm)
	_cancel.pressed.connect(cancel)


## `unanswered` / `flagged`: question numbers (positions) to list.
func open(unanswered: Array[int], flagged: Array[int]) -> void:
	_unanswered.visible = not unanswered.is_empty()
	_unanswered.text = tr("ui.exam.confirm_unanswered").format({"list": _join(unanswered)})
	_flagged.visible = not flagged.is_empty()
	_flagged.text = tr("ui.exam.confirm_flagged").format({"list": _join(flagged)})
	_all_done.visible = unanswered.is_empty()
	show()
	_cancel.grab_focus.call_deferred()


func confirm() -> void:
	hide()
	confirmed.emit()


func cancel() -> void:
	hide()
	cancelled.emit()


static func _join(numbers: Array[int]) -> String:
	var parts: PackedStringArray = []
	for n: int in numbers:
		parts.append(str(n))
	return ", ".join(parts)
