class_name QuestionPanelHost
extends VBoxContainer
## S06 `QuestionPanelHost` (F110-AC-04, AC-05; STUDENT_GAME_UX_CORE S06 §13 "panel câu hỏi là Control 2D"): the boss
## question the server gave to THIS student, drawn by the F063 renderer of its type (QuestionRendererRegistry — the
## payload has no answer key, ADR-0016). One tap on a tile answers a one-step question (F098); the other types use
## "Gửi đáp án". While the answer is on its way (`sending`) the question is locked so it cannot be sent twice; an
## answer that could not be sent unlocks it again (same question). Emits `EventBus.question_shown` / `question_hidden`
## so the chat dock shrinks to its strip (F119-AC-03). UI only: BossArena forwards `answer_ready` to BossClient.

signal answer_ready(response: Dictionary)

## One-step question types answered by a single tap (F098 tap-to-answer).
const INSTANT_TYPES: PackedStringArray = ["single_choice", "true_false"]

var renderer: QuestionRendererBase = null
var question_instance_id: String = ""
var _slot: VBoxContainer = null
var _submit: Button = null
var _state: Label = null
var _shown: bool = false


func _init() -> void:
	name = "QuestionPanelHost"
	theme_type_variation = &"FormBox"
	_state = UiBuild.keep_words(UiBuild.label("", &"InstructionLabel"))
	_state.name = "QuestionState"
	_state.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_state.visible = false
	add_child(_state)
	_slot = UiBuild.vbox()
	_slot.name = "Question"
	UiBuild.expand(_slot, true, true)
	add_child(_slot)
	_submit = UiBuild.button("ui.boss.submit", "send")
	_submit.name = "Submit"
	_submit.size_flags_horizontal = Control.SIZE_SHRINK_END
	_submit.visible = false
	_submit.pressed.connect(func() -> void:
		if renderer != null:
			renderer.commit())
	add_child(_submit)


func _exit_tree() -> void:
	_set_shown(false)


## Shows `q` (same question again = only its lock state changes) or the waiting words `waiting_key` when null.
func show_question(q: BossQuestionModel, ctx: RendererContext, waiting_key: String = "ui.boss.waiting_question") -> void:
	if q == null or q.render == null:
		clear(waiting_key)
		return
	if q.question_instance_id != question_instance_id or renderer == null:
		_clear_renderer()
		question_instance_id = q.question_instance_id
		renderer = QuestionRendererRegistry.instantiate(q.question_type)
		if renderer == null:
			_state.text = "ui.activity_player.update_required"
			_state.visible = true
			return
		renderer.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		renderer.size_flags_vertical = Control.SIZE_EXPAND_FILL
		_slot.add_child(renderer)
		renderer.setup(q.render, ctx)
		var instant: bool = q.question_type in INSTANT_TYPES
		if instant:
			renderer.set_instant_answer(true)
			renderer.choice_tapped.connect(func(_repeated: bool) -> void:
				if renderer != null:
					renderer.commit())
		renderer.answer_changed.connect(func(complete: bool) -> void: _submit.disabled = not complete)
		renderer.response_prepared.connect(func(_position: int, response: Dictionary) -> void: answer_ready.emit(response))
		_submit.visible = not instant
		_submit.disabled = true
	set_sending(q.state == BossQuestionModel.STATE_SENDING)
	_set_shown(true)


## Locked while the answer is on its way ("Đang gửi câu trả lời…"); unlocked when it could not be sent.
func set_sending(sending: bool) -> void:
	if renderer == null:
		return
	renderer.set_locked(sending)
	_submit.disabled = sending or not renderer.is_complete()
	_state.text = "ui.boss.sending" if sending else ""
	_state.visible = sending


func is_locked() -> bool:
	return renderer == null or renderer.locked


## No question on screen: the waiting words (between phases, before the first question, after `BOSS_PHASE_CLOSED`).
func clear(waiting_key: String = "") -> void:
	_clear_renderer()
	_state.text = waiting_key
	_state.visible = not waiting_key.is_empty()
	_set_shown(false)


func state_text() -> String:
	return _state.text if _state.visible else ""


func submit_button() -> Button:
	return _submit


## Taps answer tile `index` (tests; the same path as a tap or the number key).
func choose(index: int) -> void:
	if renderer != null:
		renderer.select_index(index)


func _clear_renderer() -> void:
	if renderer != null:
		_slot.remove_child(renderer)
		renderer.queue_free()
	renderer = null
	question_instance_id = ""
	_submit.visible = false


func _set_shown(on: bool) -> void:
	if on == _shown:
		return
	_shown = on
	if on:
		EventBus.question_shown.emit()
	else:
		EventBus.question_hidden.emit()
