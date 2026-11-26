extends GridContainer
## Question list of the exam runner (STUDENT_GAME_UX_LEARNING §9.2: answered = tick icon, flagged = flag icon;
## F064-AC-06). One button per question; a question the navigation rule does not allow (`sequential_no_return`:
## earlier questions) is disabled. Reads ExamRunnerState only; the runner decides what a press does. UI only.

signal index_selected(index: int)

## F096-AC-15: icons of design/icons/game instead of ●/○/🚩 (not in the shipped fonts, the Web build has no fallback).
const ICON_ANSWERED: Texture2D = preload("res://resources/ui/icons/tick.svg")
const ICON_FLAGGED: Texture2D = preload("res://resources/ui/icons/flag.svg")

var _buttons: Array[Button] = []
var _states: Array[String] = []


func build(count: int) -> void:
	for b: Button in _buttons:
		remove_child(b)
		b.queue_free()
	_buttons.clear()
	_states.clear()
	for i: int in count:
		var b: Button = Button.new()
		b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		b.toggle_mode = true
		b.focus_mode = Control.FOCUS_ALL
		b.pressed.connect(_on_pressed.bind(i))
		add_child(b)
		_buttons.append(b)
		_states.append(ExamRunnerState.STATE_UNANSWERED)


func render(state: ExamRunnerState) -> void:
	for i: int in _buttons.size():
		var position: int = state.positions[i]
		var answered: bool = state.is_answered(position)
		_states[i] = ExamRunnerState.STATE_ANSWERED if answered else ExamRunnerState.STATE_UNANSWERED
		var b: Button = _buttons[i]
		b.text = str(i + 1)
		# Flagged wins (the student wants to come back to it); answered shows a tick; not answered shows the number only.
		b.icon = ICON_FLAGGED if state.is_flagged(position) else (ICON_ANSWERED if answered else null)
		b.set_pressed_no_signal(i == state.index)
		b.disabled = not state.can_go_to(i)


func buttons() -> Array[Button]:
	return _buttons


## `answered` / `unanswered` per question, in order.
func states() -> Array:
	return _states.duplicate()


func _on_pressed(i: int) -> void:
	index_selected.emit(i)
