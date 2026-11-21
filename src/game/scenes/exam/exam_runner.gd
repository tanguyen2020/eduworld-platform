extends Control
## Exam runner (STUDENT_GAME_UX_LEARNING §9.2–§9.4, F064 US1/US2): header with the title, the server-clock countdown
## (ExamCountdownLabel), "Nộp bài" and "Thoát"; the question list (QuestionGrid: answered / not answered / flagged);
## one F063 question scene at a time in exam mode — no right/wrong, no hint, no voice-over NPC, no effect (F064-AC-02,
## F064-AC-05); "Câu trước"/"Câu sau" and the flag. The 3D world is suspended for `exam` while it is shown. Every rule
## is the service's or the server's (AGENTS R1, R5): ExamRunnerState says where the student may go (F064-AC-06),
## ExamService saves answers, keeps the clock, warns, locks at time up and submits. Nothing is written to disk.

## Submitted, closed by the server or time up: the lobby shows the waiting/result screen.
signal finished(attempt_id: String)
## The student left before submitting: the attempt continues on the server (its time keeps running).
signal left()

const SUSPEND_REASON: StringName = &"exam"
## Typed answers are saved when the student leaves the question (or submits, leaves the exam, time is up) instead of
## at every keystroke; other answers are saved at once.
const TEXT_TYPES: PackedStringArray = ["fill_in_blank", "short_answer", "numeric"]
const WARNING_KEYS: Dictionary = {5: "ui.exam.time_warning_5", 1: "ui.exam.time_warning_1"}
## Seconds a warning stays under the header (shown once, never blinking).
const WARNING_VISIBLE_SEC: float = 5.0
const UNSUPPORTED_KEY: String = "ui.activity_player.update_required"

var service: ExamService = null
var context: RendererContext = null
var grade_band: String = ""
var state: ExamRunnerState = null
var question: QuestionRendererBase = null
var finishing: bool = false
var _dirty_position: int = 0
var _clock_accum: float = 0.0
var _notice_left: float = 0.0
var _submitting: bool = false

@onready var _title: Label = %Title
@onready var _clock: HBoxContainer = %Clock
@onready var _submit: Button = %Submit
@onready var _exit: Button = %Exit
@onready var _offline: Label = %Offline
@onready var _notice: Label = %Notice
@onready var _grid: GridContainer = %Grid
@onready var _position: Label = %Position
@onready var _flag: Button = %Flag
@onready var _content: VBoxContainer = %Content
@onready var _prev: Button = %Prev
@onready var _next: Button = %Next
@onready var _confirm: PanelContainer = %SubmitConfirm
@onready var _exit_confirm: PanelContainer = %ExitConfirm
@onready var _stay: Button = %Stay
@onready var _leave: Button = %Leave


func _ready() -> void:
	_submit.pressed.connect(ask_submit)
	_exit.pressed.connect(ask_exit)
	_prev.pressed.connect(go_prev)
	_next.pressed.connect(go_next)
	_flag.pressed.connect(toggle_flag)
	_grid.connect("index_selected", go_to)
	_confirm.connect("confirmed", _on_submit_confirmed)
	_stay.pressed.connect(func() -> void: _exit_confirm.hide())
	_leave.pressed.connect(leave)


## Shows the attempt of `exam` (already started or resumed by the lobby).
func begin(exam: ExamService, ctx: RendererContext, band: String) -> void:
	service = exam
	context = ctx
	grade_band = band
	finishing = false
	_bind()
	var positions: Array[int] = []
	for item: QuestionRenderModel in service.attempt.items:
		positions.append(item.position)
	var forward_only: bool = service.session != null and service.session.forward_only()
	state = ExamRunnerState.create(positions, forward_only, service.answered_positions())
	_title.text = service.session.title if service.session != null else ""
	_grid.call("build", state.count())
	_notice.visible = false
	_exit_confirm.hide()
	_confirm.hide()
	show()
	QualityManager.suspend_world(SUSPEND_REASON)
	_show_current()
	_render_clock()
	_render_offline()
	if service.locked:
		_lock()


func current_index() -> int:
	return state.index if state != null else -1


func go_next() -> void:
	_move(state.index + 1)


func go_prev() -> void:
	if state.can_go_prev():
		_move(state.index - 1)


func go_to(i: int) -> void:
	_move(i)


func toggle_flag() -> void:
	if state == null or service.locked:
		return
	state.toggle_flag()
	_render_nav()


## "Nộp bài": the confirmation lists the questions without an answer and the flagged ones (F064-AC-07).
func ask_submit() -> void:
	if service == null or service.locked or _submitting:
		return
	await _save_current()
	_confirm.call("open", state.unanswered_positions(), state.flagged_positions())


## "Thoát": the time keeps running while the student is away.
func ask_exit() -> void:
	if service != null and not service.locked and not finishing:
		_exit_confirm.show()
		_stay.grab_focus.call_deferred()
	else:
		leave()


## Leaves the runner (the attempt stays open on the server; reopening resumes it, F064-AC-15).
func leave() -> void:
	if service != null and not service.locked:
		await _save_current()
	_teardown()
	left.emit()


# ---------- answers ----------

func _show_current() -> void:
	_clear_question()
	var item: QuestionRenderModel = service.attempt.item_at(state.current_position())
	if item == null:
		return
	question = QuestionRendererRegistry.instantiate(item.type)
	if question == null:
		var notice: Label = Label.new()
		notice.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		notice.text = UNSUPPORTED_KEY
		_content.add_child(notice)
	else:
		question.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		_content.add_child(question)
		question.setup(item, context)
		question.restore(service.response_of(item.position))
		question.answer_changed.connect(_on_answer_changed.bind(item.position))
		question.set_locked(service.locked)
	_position.text = tr("ui.exam.question_of").format({"n": state.index + 1, "total": state.count()})
	_render_nav()


func _on_answer_changed(complete: bool, position: int) -> void:
	if service.locked or question == null or question.item.position != position or not complete:
		return
	if question.item.type in TEXT_TYPES:
		_dirty_position = position
		return
	_save(position)


## Saves the current typed answer if it changed (leaving the question, submit, exit, time up).
func _save_current() -> void:
	if question == null or _dirty_position != question.item.position or not question.is_complete():
		return
	await _save(_dirty_position)


func _save(position: int) -> void:
	if position == _dirty_position:
		_dirty_position = 0
	state.mark_answered(position)
	_render_nav()
	await service.save_answer(position, question.current_response())


func _move(i: int) -> void:
	if state == null or finishing or not state.can_go_to(i) or i == state.index:
		return
	_save_current()
	state.go_to(i)
	_show_current()


# ---------- submit, time up, closed ----------

func _on_submit_confirmed() -> void:
	if _submitting:
		return
	_submitting = true
	_submit.disabled = true
	_show_notice("ui.exam.submitting", 0.0)
	var ok: bool = await service.submit()
	_submitting = false
	if not is_instance_valid(self) or finishing:
		return
	if not ok:
		_submit.disabled = service.locked
		_show_notice("ui.exam.submit_failed" if service.last_error_code == ApiResult.CODE_NETWORK_ERROR
				else L10n.ERROR_PREFIX + service.last_error_code, 0.0)
		return
	_finish()


## Time up on the server clock (F064-AC-12): input locked, "bài đã được nộp tự động", queued answers then one submit.
func _on_time_expired() -> void:
	if question != null:
		question.set_locked(true)
	await _save_current()
	_lock()
	_show_notice("ui.exam.auto_submitted", 0.0)
	await service.auto_submit()
	_finish()


## `422 EXAM_TIME_OVER` (F064-AC-13): the attempt is closed; never retried.
func _on_attempt_closed() -> void:
	_lock()
	_show_notice("ui.exam.auto_submitted", 0.0)
	_finish()


## `409 EXAM_ALREADY_SUBMITTED` (submit or answer, F064-AC-08): already submitted — no more changes.
func _on_exam_submitted(_result: Dictionary) -> void:
	_lock()
	_finish()


## `409 EXAM_ACTIVE_ON_OTHER_DEVICE` while answering (F064-AC-16): locked here; only "Thoát" stays.
func _on_other_device() -> void:
	_lock()
	_show_notice(L10n.ERROR_PREFIX + ExamService.CODE_OTHER_DEVICE, 0.0)
	_exit.disabled = false


func _on_time_warning(minutes: int) -> void:
	_show_notice(String(WARNING_KEYS.get(minutes, "")), WARNING_VISIBLE_SEC)


func _lock() -> void:
	if question != null:
		question.set_locked(true)
	_submit.disabled = true
	_flag.disabled = true
	_prev.disabled = true
	_next.disabled = true


func _finish() -> void:
	if finishing:
		return
	finishing = true
	var attempt_id: String = service.attempt.attempt_id if service.attempt != null else ""
	_teardown()
	finished.emit(attempt_id)


func _teardown() -> void:
	_unbind()
	_clear_question()
	_confirm.hide()
	_exit_confirm.hide()
	hide()
	QualityManager.resume_world(SUSPEND_REASON)


# ---------- rendering ----------

func _render_nav() -> void:
	if state == null:
		return
	_grid.call("render", state)
	_prev.visible = not state.forward_only
	_prev.disabled = service.locked or not state.can_go_prev()
	_next.disabled = service.locked or not state.can_go_next()
	_flag.set_pressed_no_signal(state.is_flagged(state.current_position()))
	_flag.text = "ui.exam.unflag" if _flag.button_pressed else "ui.exam.flag"
	_flag.disabled = service.locked


func _render_clock() -> void:
	if service == null or service.attempt == null:
		return
	_clock.call("render", maxi(0, service.remaining_ms()), service.total_ms(), grade_band)


func _render_offline() -> void:
	_offline.visible = service != null and service.offline


## `key` "" hides; `seconds` 0 keeps it until the next notice.
func _show_notice(key: String, seconds: float) -> void:
	_notice.visible = not key.is_empty()
	_notice.text = (L10n.error_text(key.trim_prefix(L10n.ERROR_PREFIX)) if key.begins_with(L10n.ERROR_PREFIX)
			else tr(key)) if not key.is_empty() else ""
	_notice_left = seconds


func _clear_question() -> void:
	for child: Node in _content.get_children():
		_content.remove_child(child)
		child.queue_free()
	question = null


func _bind() -> void:
	for pair: Array in _connections():
		if not (pair[0] as Signal).is_connected(pair[1] as Callable):
			(pair[0] as Signal).connect(pair[1] as Callable)


func _unbind() -> void:
	if service == null:
		return
	for pair: Array in _connections():
		if (pair[0] as Signal).is_connected(pair[1] as Callable):
			(pair[0] as Signal).disconnect(pair[1] as Callable)


func _connections() -> Array:
	return [
		[service.time_expired, _on_time_expired], [service.attempt_closed, _on_attempt_closed],
		[service.exam_submitted, _on_exam_submitted], [service.other_device_detected, _on_other_device],
		[service.time_warning_reached, _on_time_warning], [service.offline_changed, _on_offline_changed],
		[service.answer_saved, _on_answer_saved],
	]


func _on_offline_changed(_offline_now: bool) -> void:
	_render_offline()


func _on_answer_saved(position: int) -> void:
	if state != null:
		state.mark_answered(position)
		_render_nav()


func _process(delta: float) -> void:
	if not visible or service == null:
		return
	if _notice_left > 0.0:
		_notice_left -= delta
		if _notice_left <= 0.0:
			_notice.visible = false
	_clock_accum += delta
	if _clock_accum >= 1.0:
		_clock_accum = 0.0
		service.tick()
		_render_clock()


func _unhandled_input(event: InputEvent) -> void:
	if not visible or not event is InputEventKey or not event.is_pressed() or event.is_echo():
		return
	if (event as InputEventKey).keycode == KEY_ESCAPE:
		if _exit_confirm.visible:
			_exit_confirm.hide()
		elif _confirm.visible:
			_confirm.call("cancel")
		else:
			ask_exit()
		get_viewport().set_input_as_handled()
