extends Control
## Exam Zone screen `exam_lobby` (STUDENT_GAME_UX_LEARNING §9.1, F064 US1–US3) opened by the intent of a school
## assignment (`{exam_session_id, assignment_id}`, F062) or the exam board of the Main Hub (`{}` → the list of
## `listExamSessions`). The rules screen comes from `getExamSession` (questions, time, navigation, result release,
## "time keeps running", F064-AC-01); "Bắt đầu" asks ExamService to start the attempt and opens the runner
## (F064-AC-02); a refusal shows `ui.error.<CODE>` here and no runner (F064-AC-09); an attempt open on another device
## offers "Chuyển sang thiết bị này" (`take_over`, F064-AC-16). After the runner: waiting / result screen. UI glue
## only: every decision is the server's or the service's (AGENTS R1, R5); nothing of the exam is written to disk.

signal closed()

enum View { IDLE, LOADING, LIST, RULES, RUNNING, AFTER, ERROR }

const RUNNER_SCENE: PackedScene = preload("res://scenes/exam/ExamRunner.tscn")
const RESULT_SCENE: PackedScene = preload("res://scenes/exam/ExamResult.tscn")
const SCREEN_ACTIVITY_HOST: StringName = &"activity_host"
const NAVIGATION_KEYS: Dictionary = {
	ExamSessionModel.NAV_FREE: "ui.exam.rules_navigation_free",
	ExamSessionModel.NAV_SEQUENTIAL: "ui.exam.rules_navigation_sequential",
}
const RELEASE_KEYS: Dictionary = {
	ExamSessionModel.RELEASE_IMMEDIATE: "ui.exam.rules_release_immediate",
	ExamSessionModel.RELEASE_AFTER_CLOSE: "ui.exam.rules_release_after_close",
	ExamSessionModel.RELEASE_MANUAL: "ui.exam.rules_release_manual",
}

## WorldRoot holds the break reminder while a timed exam screen is open (F062-AC-22).
var blocks_break_reminder: bool = true
var services: ServiceLocator = null
var service: ExamService = null
var grade_band: String = ""
var args: Dictionary = {}
var view: View = View.IDLE
var runner: Control = null
var result_view: Control = null
var session_id: String = ""
var attempt_id: String = ""
var _retry_action: Callable = Callable()

@onready var _window: PanelContainer = %Window
@onready var _close: Button = %Close
@onready var _state_label: Label = %StateLabel
@onready var _retry: Button = %Retry
@onready var _list: VBoxContainer = %List
@onready var _list_empty: Label = %ListEmpty
@onready var _sessions: VBoxContainer = %Sessions
@onready var _rules: VBoxContainer = %Rules
@onready var _rules_title: Label = %RulesTitle
@onready var _rules_questions: Label = %RulesQuestions
@onready var _rules_duration: Label = %RulesDuration
@onready var _rules_navigation: Label = %RulesNavigation
@onready var _rules_release: Label = %RulesRelease
@onready var _rules_offline: Label = %RulesOffline
@onready var _rules_error: Label = %RulesError
@onready var _start: Button = %Start
@onready var _take_over: Button = %TakeOver
@onready var _view_result: Button = %ViewResult
@onready var _after: VBoxContainer = %After
@onready var _after_label: Label = %AfterLabel
@onready var _after_back: Button = %AfterBack
@onready var _result_host: VBoxContainer = %ResultHost
@onready var _runner_host: Control = %RunnerHost


func _ready() -> void:
	visible = false
	_close.pressed.connect(close)
	_retry.pressed.connect(_on_retry_pressed)
	_start.pressed.connect(start_exam)
	_take_over.pressed.connect(start_exam.bind(true))
	_view_result.pressed.connect(_on_view_result_pressed)
	_after_back.pressed.connect(close)


## WorldRoot: `ui_requested("exam_lobby", args)` — `{exam_session_id, assignment_id?}` or `{}` (list).
func open(intent: Dictionary = {}) -> void:
	args = intent
	if services == null:
		services = AppState.get_services()
	_bind_service(services.exam)
	if grade_band.is_empty():
		grade_band = AppState.grade_band
	show()
	var id: Variant = intent.get("exam_session_id")
	if id is String and Uuid.is_valid(id as String):
		await show_rules(id as String)
	else:
		await show_list()


func close() -> void:
	if runner != null:
		await runner.call("leave")
	_drop_runner()
	_drop_result()
	if service != null and (service.submitted or service.attempt == null):
		service.clear()
	_set_view(View.IDLE)
	hide()
	closed.emit()


## Coroutine: the exam sessions of the student (`listExamSessions`).
func show_list() -> void:
	_set_view(View.LOADING)
	var sessions: Array[ExamSessionModel] = await service.list_sessions()
	if not visible:
		return
	if not service.last_error_code.is_empty():
		_on_failed(show_list)
		return
	for child: Node in _sessions.get_children():
		_sessions.remove_child(child)
		child.queue_free()
	for s: ExamSessionModel in sessions:
		var b: Button = Button.new()
		b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		b.alignment = HORIZONTAL_ALIGNMENT_LEFT
		b.text = s.title
		b.pressed.connect(show_rules.bind(s.exam_session_id))
		_sessions.add_child(b)
	_list_empty.visible = sessions.is_empty()
	_set_view(View.LIST)


## Coroutine: the rules screen of one exam (`getExamSession`, F064-AC-01).
func show_rules(exam_session_id: String) -> void:
	session_id = exam_session_id
	_set_view(View.LOADING)
	var s: ExamSessionModel = await service.load_session(exam_session_id)
	if not visible:
		return
	if s == null:
		_on_failed(show_rules.bind(exam_session_id))
		return
	_rules_title.text = s.title
	_rules_questions.text = tr("ui.exam.rules_questions").format({"count": s.question_count})
	_rules_duration.text = tr("ui.exam.rules_duration").format({"minutes": s.duration_minutes()})
	_rules_navigation.text = tr(String(NAVIGATION_KEYS.get(s.navigation, NAVIGATION_KEYS[ExamSessionModel.NAV_FREE])))
	_rules_release.text = tr(String(RELEASE_KEYS.get(s.result_release, RELEASE_KEYS[ExamSessionModel.RELEASE_MANUAL])))
	_rules_offline.text = tr("ui.exam.rules_offline")
	_rules_error.visible = false
	_take_over.visible = false
	attempt_id = _submitted_attempt_id()
	_view_result.visible = not attempt_id.is_empty()
	_start.disabled = false
	_set_view(View.RULES)


## Coroutine: "Bắt đầu" (or "Chuyển sang thiết bị này" with `take_over`): starts or resumes the attempt and opens the
## runner (F064-AC-02, F064-AC-15); a refusal stays on the rules screen with its message (F064-AC-09, F064-AC-16).
func start_exam(take_over: bool = false) -> void:
	if service == null or session_id.is_empty():
		return
	_start.disabled = true
	_take_over.disabled = true
	_rules_error.visible = false
	var attempt: AttemptModel = await service.start(session_id, take_over)
	_start.disabled = false
	_take_over.disabled = false
	if not visible:
		return
	if attempt == null:
		var code: String = service.last_error_code
		if code in ExamService.ACCESS_DENIED_CODES:
			close()
			return
		if service.session == null:
			_on_failed(show_rules.bind(session_id))
			return
		_rules_error.text = L10n.error_text(code)
		_rules_error.visible = true
		_take_over.visible = code == ExamService.CODE_OTHER_DEVICE
		return
	if service.submitted:
		await show_after(attempt.attempt_id)
		return
	_open_runner()


## Coroutine: after the runner (submitted, closed or time up) or "Xem kết quả": asks `getExamResult` once; not
## released yet → "Nhà trường sẽ công bố kết quả sau" (F064-AC-17); released → score, objectives, wrong questions and
## practice (F064-AC-18, F064-AC-19).
func show_after(finished_attempt_id: String) -> void:
	attempt_id = finished_attempt_id
	_after_label.text = tr("ui.exam.submitted") if service.submitted else ""
	_after_label.visible = service.submitted
	_drop_result()
	result_view = RESULT_SCENE.instantiate() as Control
	_result_host.add_child(result_view)
	result_view.connect("practice_requested", _on_practice_requested)
	_set_view(View.AFTER)
	var released: ExamResultModel = await service.result(finished_attempt_id)
	if not visible or result_view == null:
		return
	if released == null:
		if service.last_error_code in ExamService.ACCESS_DENIED_CODES:
			close()
			return
		result_view.call("show_pending")
		return
	result_view.call("show_result", released, grade_band, _result_context())


# ---------- internals ----------

func _open_runner() -> void:
	_drop_runner()
	runner = RUNNER_SCENE.instantiate() as Control
	_runner_host.add_child(runner)
	runner.connect("finished", _on_runner_finished)
	runner.connect("left", _on_runner_left)
	_set_view(View.RUNNING)
	runner.call("begin", service, _context(), grade_band)


func _on_runner_finished(finished_attempt_id: String) -> void:
	_drop_runner.call_deferred()
	await show_after(finished_attempt_id)


func _on_runner_left() -> void:
	_drop_runner.call_deferred()
	if visible and not session_id.is_empty():
		_set_view(View.RULES)


func _drop_runner() -> void:
	if runner == null:
		return
	if runner.get_parent() == _runner_host:
		_runner_host.remove_child(runner)
	runner.queue_free()
	runner = null


func _drop_result() -> void:
	if result_view == null:
		return
	if result_view.get_parent() == _result_host:
		_result_host.remove_child(result_view)
	result_view.queue_free()
	result_view = null


## "Luyện LO này" / a recommended practice: the activity player opens the practice (it closes this screen).
func _on_practice_requested(practice: Dictionary) -> void:
	service.clear()
	EventBus.ui_requested.emit(SCREEN_ACTIVITY_HOST, practice)


## Content language of the result texts: the attempt's when known, else the UI locale's data language.
func _result_context() -> RendererContext:
	var ctx: RendererContext = RendererContext.new()
	ctx.content_language = service.attempt.content_language() if service.attempt != null 			else L10n.data_language(TranslationServer.get_locale())
	ctx.grade_band = grade_band
	ctx.math = services.math_svg
	return ctx


func _context() -> RendererContext:
	var ctx: RendererContext = RendererContext.new()
	ctx.content_language = service.attempt.content_language() if service.attempt != null else L10n.FALLBACK_LANGUAGE
	ctx.grade_band = grade_band
	ctx.math = services.math_svg
	ctx.media = services.media
	ctx.clock = service.clock
	# Exam mode: no automatic voice-over, no NPC (STUDENT_GAME_UX_LEARNING §9.4).
	ctx.auto_voice = false
	return ctx


## The submitted attempt of the assignment that opened this exam, if any ("Xem kết quả").
func _submitted_attempt_id() -> String:
	var assignment_id: Variant = args.get("assignment_id")
	if not assignment_id is String:
		return ""
	for a: MyAssignmentModel in AppState.assignments:
		if a.assignment_id == assignment_id and a.status in ["submitted", "graded"] and not a.attempt_id.is_empty():
			return a.attempt_id
	return ""


func _on_view_result_pressed() -> void:
	await show_after(attempt_id)


## A failed load: nothing of the exam stays on screen (404, F064-AC-10); a session refusal ends the session (F061).
func _on_failed(retry: Callable) -> void:
	var code: String = service.last_error_code
	if code in ExamService.ACCESS_DENIED_CODES:
		close()
		return
	_retry_action = retry if code != ExamService.CODE_NOT_FOUND else Callable()
	_state_label.text = L10n.error_text(code)
	_set_view(View.ERROR)


func _on_retry_pressed() -> void:
	if _retry_action.is_valid():
		var action: Callable = _retry_action
		_retry_action = Callable()
		await action.call()


func _set_view(next: View) -> void:
	view = next
	_window.visible = next != View.RUNNING
	_state_label.visible = next == View.LOADING or next == View.ERROR
	if next == View.LOADING:
		_state_label.text = tr("ui.exam.loading")
	_retry.visible = next == View.ERROR and _retry_action.is_valid()
	_list.visible = next == View.LIST
	_rules.visible = next == View.RULES
	_after.visible = next == View.AFTER


func _bind_service(exam: ExamService) -> void:
	if service == exam:
		return
	service = exam
	service.access_denied.connect(func(_code: String) -> void:
		if visible:
			close()
	)


func _unhandled_input(event: InputEvent) -> void:
	if visible and view != View.RUNNING and event.is_action_pressed("ui_cancel"):
		close()
		get_viewport().set_input_as_handled()
