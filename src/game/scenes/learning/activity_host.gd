extends Control
## Activity player frame (STUDENT_GAME_UX_LEARNING §3, GODOT_ARCHITECTURE §6.2, F063 US1–US5) opened by the
## `activity_host` intent (F062 QuestJournal, quests…). It suspends the 3D world (F063-AC-09), asks LearningService
## to start/resume the attempt (F063-AC-01/07), picks the renderer of `activity_type` in ActivityRendererRegistry
## (or shows "update required", F063-AC-25), and runs the action bar: Kiểm tra → server grading in the feedback strip
## (F063-AC-14) → Tiếp / Hoàn thành → submit (F063-AC-08) → result screen (F063-AC-21…24). Header: exit, title in
## the attempt language (+ "no translation" label, F063-AC-13), progress, server-clock countdown refreshed once per
## second (P-6). Keys: Enter, Esc, H, R, 1–6, Tab (focus). UI glue only: every decision is the server's or the
## service's (AGENTS R1, R5); no answer or payload is logged or written to disk.
## F097 (STUDENT_GAME_UX_LEARNING §3.3, §3.7): the player is a board over the sky backdrop; the header shows a
## question trail (stars / ring / dots) beside "1/11"; the stem lives in the renderer's question card (no caption
## repeating it, F096-3); the coach Mây sits bottom-left with a speech bubble holding the cheer, the verdict (icon
## + words, kind wording), the hint button and errors; the main button is bottom-right with an icon.
## F098 (STUDENT_GAME_UX_LEARNING §4.1b): when InstantAnswerPolicy allows it (effective feedback timing immediate, not
## exam/diagnostic/competition, one-step question), a tap on an answer tile sends the answer at once — no "Kiểm tra"
## step; the tiles lock and the tapped one shows "Đang chấm…" until the server's verdict; a refused answer offers
## "Gửi lại". Grade bands in `eduworld/learning/tap_confirm_bands` confirm with a second tap on the same tile.

signal closed()

enum State { IDLE, LOADING, PRESENTING, AWAITING_RESULT, FEEDBACK, SUBMITTING, RESULT, RESULTS_LATER, ERROR,
	UPDATE_REQUIRED, TIME_UP }

const SUSPEND_REASON: StringName = &"activity"
## Media and simulation activities may have no question item (a video without cue, a simulation without question).
const ITEMLESS_TYPES: PackedStringArray = ["video_lesson", "audio_lesson", "simulation"]
const RESULT_SCENE: PackedScene = preload("res://scenes/learning/ActivityResult.tscn")
const UPDATE_SCENE: PackedScene = preload("res://scenes/learning/UpdateRequired.tscn")
## STUDENT_GAME_UX_LEARNING §3.5: loading longer than this shows "Thử lại".
const LOAD_RETRY_AFTER_SEC: float = 8.0
## Re-send of queued answers while offline (ApiClient already backs off inside one attempt).
const OFFLINE_RETRY_SEC: float = 5.0
const BAND_NO_CLOCK: String = "g1_3"
## F096-AC-15: the timed indicator is words, not the ⏳ glyph (missing from the shipped fonts; empty box on the Web build).
const TIMED_KEY: String = "ui.common.timed"
## Codes after which the attempt can no longer be answered: the player goes to the result flow.
const CLOSED_CODES: PackedStringArray = ["ATTEMPT_EXPIRED", "ATTEMPT_NOT_IN_PROGRESS", "ATTEMPT_ALREADY_SUBMITTED"]
## F097: the coach's cheer when the bubble has nothing else to say (with / without the hint button).
const CHEER_KEY: String = "ui.activity_player.coach_cheer"
const CHEER_HINT_KEY: String = "ui.activity_player.coach_cheer_hint"
## F097: icon of the main button per action (icon + words, STUDENT_GAME_UX_LEARNING §3.7).
const ICON_CHECK: Texture2D = preload("res://resources/ui/icons/check.svg")
const ICON_NEXT: Texture2D = preload("res://resources/ui/icons/next_light.svg")
## F098: Mây's line after the first tap when the band confirms with a second tap; the main button after a refusal.
const TAP_AGAIN_KEY: String = "ui.activity_player.tap_again"
const RESEND_KEY: String = "ui.activity_player.resend"
const ICON_RESEND: Texture2D = preload("res://resources/ui/icons/retry_light.svg")

## WorldRoot holds the break reminder while a learning screen is open (F062-AC-22).
var blocks_break_reminder: bool = true
var state: State = State.IDLE
var args: Dictionary = {}
var services: ServiceLocator = null
var service: LearningService = null
var renderer: ActivityRendererBase = null
var context: RendererContext = null
var result_view: Control = null
var update_view: Control = null
var last_result: ResponseResultModel = null
## `X-Client-Version` of this build (Config), compared with the content's minimum versions.
var client_version: String = ""
var grade_band: String = ""
var _retry_action: Callable = Callable()
var _opened_msec: int = 0
var _state_sec: float = 0.0
var _clock_accum: float = 0.0
var _offline_accum: float = 0.0
var _flushing: bool = false
## F098: the current item sends its answer on tap / asks for a second tap first (InstantAnswerPolicy, per item).
var _instant: bool = false
var _double_tap: bool = false

@onready var _exit: Button = %Exit
@onready var _title: Label = %Title
@onready var _no_translation: Label = %NoTranslation
@onready var _progress: Label = %Progress
@onready var _trail: ProgressTrail = %Trail
@onready var _frame: PanelContainer = %Frame
@onready var _coach: CloudyMascot = %Coach
@onready var _coach_line: Label = %CoachLine
@onready var _action_row: HBoxContainer = %ActionRow
@onready var _clock: Label = %Clock
@onready var _offline: Label = %Offline
@onready var _voice: PanelContainer = %VoiceOver
@onready var _body: VBoxContainer = %Body
@onready var _state_panel: VBoxContainer = %StatePanel
@onready var _state_label: Label = %StateLabel
@onready var _details_label: Label = %DetailsLabel
@onready var _details: Button = %Details
@onready var _retry: Button = %Retry
@onready var _back: Button = %Back
@onready var _feedback: VBoxContainer = %FeedbackStrip
@onready var _hints: VBoxContainer = %HintPanel
@onready var _status: Label = %Status
@onready var _action_details: Button = %ActionDetails
@onready var _primary: Button = %Primary
@onready var _exit_confirm: PanelContainer = %ExitConfirm
@onready var _continue: Button = %Continue
@onready var _pause: Button = %Pause
@onready var _worked: PanelContainer = %WorkedExample


func _ready() -> void:
	visible = false
	# F097: the sky shows around the board (component.activity_board.inset).
	var inset: float = float(UiTokens.COMPONENT_ACTIVITY_BOARD_INSET)
	_frame.offset_left = inset
	_frame.offset_top = inset
	_frame.offset_right = -inset
	_frame.offset_bottom = -inset
	_exit.pressed.connect(ask_exit)
	_primary.pressed.connect(press_primary)
	_retry.pressed.connect(_on_retry_pressed)
	_back.pressed.connect(close)
	_details.pressed.connect(func() -> void: _details_label.visible = not _details_label.visible)
	_action_details.pressed.connect(_on_action_details_pressed)
	_continue.pressed.connect(func() -> void: _exit_confirm.hide())
	_pause.pressed.connect(_on_pause_pressed)
	_hints.connect("hint_requested", _on_hint_requested)
	_hints.connect("math_failed", _on_math_failed)
	_feedback.connect("worked_example_requested", _on_worked_example_requested)
	_feedback.connect("math_failed", _on_math_failed)
	_exit_confirm.hide()
	if client_version.is_empty():
		client_version = String(Config.client_version)


## WorldRoot: `ui_requested("activity_host", args)` — `{activity_id | question_set_id | attempt_id, mode,
## context_type, context_id, recommendation_id?, activity_type?}`.
func open(intent: Dictionary = {}) -> void:
	args = intent
	if services == null:
		services = AppState.get_services()
	_bind_service(services.learning)
	if grade_band.is_empty():
		grade_band = AppState.grade_band
	show()
	QualityManager.suspend_world(SUSPEND_REASON)
	await start(intent)


## Closes the player: the world resumes and the game scene underneath is back as it was (F063-AC-09).
func close() -> void:
	_clear_body()
	_voice.call("stop")
	_exit_confirm.hide()
	_worked.call("close")
	_set_state(State.IDLE)
	if service != null and service.attempt != null and service.attempt.is_finished():
		service.clear()
	hide()
	QualityManager.resume_world(SUSPEND_REASON)
	closed.emit()


## Coroutine: starts (or resumes) the attempt of `intent` and shows its renderer.
func start(intent: Dictionary) -> void:
	args = intent
	_clear_body()
	_set_header("", false)
	_opened_msec = Time.get_ticks_msec()
	var known_type: String = String(intent.get("activity_type", ""))
	if not known_type.is_empty() and not _supported(known_type, ""):
		_show_update_required(known_type)
		return
	_set_state(State.LOADING)
	var attempt: AttemptModel = await service.start(intent)
	if not visible:
		return
	if attempt == null:
		_on_start_failed(known_type)
		return
	await present(attempt)


## Coroutine: shows `attempt` (renderer, result screen or "update required").
func present(attempt: AttemptModel) -> void:
	var min_version: String = attempt.render.min_client_version if attempt.render != null else ""
	if not _supported(attempt.activity_type, min_version):
		_show_update_required(attempt.activity_type)
		return
	context = _make_context(attempt)
	_set_header(attempt.render.title if attempt.render != null else "", attempt.translation_missing())
	if attempt.is_finished():
		await _show_results(false)
		return
	if attempt.items.is_empty() and not attempt.activity_type in ITEMLESS_TYPES:
		_show_error(ApiResult.CODE_INVALID_RESPONSE, start.bind(args))
		return
	renderer = ActivityRendererRegistry.instantiate(attempt.activity_type)
	# A renderer that runs its own player on this attempt (mini-game, F073) gets the same service — never ApiClient.
	if renderer.has_method("attach_service"):
		renderer.call("attach_service", service)
	renderer.size_flags_vertical = Control.SIZE_EXPAND_FILL
	_body.add_child(renderer)
	renderer.response_submitted.connect(_on_response_submitted)
	renderer.answer_changed.connect(_on_answer_changed)
	renderer.item_shown.connect(_on_item_shown)
	renderer.progress_reported.connect(_on_progress_reported)
	renderer.finish_requested.connect(_on_finish_requested)
	renderer.run_submitted.connect(_on_run_submitted)
	renderer.read_aloud_requested.connect(_on_read_aloud_requested)
	renderer.choice_tapped.connect(_on_choice_tapped)
	if renderer.has_signal("template_load_failed"):
		renderer.connect("template_load_failed", _on_template_load_failed.bind(attempt.activity_type))
	_set_state(State.PRESENTING)
	renderer.setup(attempt, context)
	service.report_open_time(Time.get_ticks_msec() - _opened_msec)


## Enter / main button: Kiểm tra → (server) → Tiếp / Hoàn thành.
func press_primary() -> void:
	match state:
		State.PRESENTING:
			if renderer != null and not renderer.check():
				_render_action_bar()
		State.FEEDBACK:
			await _next()


## Esc / "Thoát": asks to pause while an attempt is open; otherwise just closes.
func ask_exit() -> void:
	if state in [State.PRESENTING, State.AWAITING_RESULT, State.FEEDBACK, State.LOADING]:
		_exit_confirm.show()
		_continue.grab_focus.call_deferred()
	else:
		close()


# ---------- answer, feedback, submit ----------

func _on_response_submitted(submission: Dictionary) -> void:
	_set_state(State.AWAITING_RESULT)
	# F098: no second tap reaches the item while the server grades (a queued answer keeps it locked).
	if _instant and renderer != null:
		renderer.set_grading(true)
	var result: ResponseResultModel = null
	if String(submission.get("kind", "")) == "branch_choice":
		result = await service.choose_branch(String(submission.get("branch_point_id", "")), String(submission.get("option_id", "")))
		if state == State.AWAITING_RESULT and result != null:
			renderer.show_result(result)
			_set_state(State.PRESENTING)
			return
	else:
		result = await service.respond(int(submission.get("position", 0)), submission.get("response", {}) as Dictionary,
				int(submission.get("client_elapsed_ms", 0)), submission)
	if state != State.AWAITING_RESULT:
		return
	if result != null:
		_apply_result(result)
		return
	if service.offline:
		_render_action_bar()
		return
	_on_action_failed(service.last_error_code)


func _apply_result(result: ResponseResultModel) -> void:
	last_result = result
	renderer.show_result(result)
	_feedback.call("show_result", result, context.math)
	if result.has_verdict:
		_coach.mood = CloudyMascot.Mood.HAPPY if result.is_correct else CloudyMascot.Mood.ENCOURAGE
	_set_state(State.FEEDBACK)


func _next() -> void:
	_feedback.call("clear")
	_coach.mood = CloudyMascot.Mood.NEUTRAL
	if renderer != null and renderer.advance():
		_set_state(State.PRESENTING)
		return
	await _submit()


func _submit() -> void:
	_set_state(State.SUBMITTING)
	if renderer != null:
		renderer.set_locked(true)
	var submitted: AttemptModel = await service.submit()
	if not visible:
		return
	if submitted == null:
		if service.attempt == null:
			_show_fatal(service.last_error_code)
		else:
			_show_error(service.last_error_code, _submit)
		return
	await _show_results(true)


## Result flow. Right after submit, results released later show "results_later" without asking the review
## (F063-AC-23); a reopened finished attempt asks the review once and falls back to the same message.
func _show_results(after_submit: bool) -> void:
	var attempt: AttemptModel = service.attempt
	_clear_body()
	# F096: the result screen has no question position ("1/11" belonged to the questions).
	_progress.text = ""
	_trail.set_progress(0, 0)
	result_view = RESULT_SCENE.instantiate() as Control
	_body.add_child(result_view)
	result_view.connect("practice_requested", _on_practice_requested)
	result_view.connect("back_requested", close)
	var granted: Array = service.last_rewards
	if attempt == null or (after_submit and not attempt.results_immediate()):
		result_view.call("show_results_later", granted)
		_set_state(State.RESULTS_LATER)
		return
	var review: AttemptReviewModel = await service.review()
	if not visible or result_view == null:
		return
	if review == null:
		result_view.call("show_results_later", granted)
		_set_state(State.RESULTS_LATER)
		return
	result_view.call("show_review", review, grade_band, granted)
	_set_state(State.RESULT)


func _on_practice_requested(recommendation: AttemptReviewModel.Recommendation) -> void:
	service.clear()
	await start(LearningService.practice_args(recommendation))


# ---------- hints, voice-over, worked example ----------

func _on_item_shown(position: int) -> void:
	var attempt: AttemptModel = service.attempt
	var item: QuestionRenderModel = attempt.item_at(position) if attempt != null else null
	_feedback.call("clear")
	_instant = false
	_double_tap = false
	if item == null:
		return
	# F098: tap-to-answer only where the server's timing allows per-question feedback and the item is one step.
	_instant = renderer.supports_instant_answer() and InstantAnswerPolicy.applies(attempt, item)
	_double_tap = _instant and InstantAnswerPolicy.double_tap(grade_band)
	renderer.set_instant_answer(_instant, _double_tap)
	_hints.call("configure", attempt.hints_allowed(), item.hint_count, attempt.hints_revealed(position))
	var p: Vector2i = renderer.progress()
	_progress.text = "%d/%d" % [p.x, p.y]
	_trail.set_progress(p.x, p.y)
	_trail.visible = p.y > 1
	_coach.mood = CloudyMascot.Mood.NEUTRAL
	_present_voice(item)
	_render_action_bar()


func _present_voice(item: QuestionRenderModel) -> void:
	var caption: String = RichText.plain_text(item.stem(), context)
	var url: String = item.voice_url()
	_voice.set("always_caption", context.auto_voice)
	# F096-3: the question card already shows the stem; the voice-over only plays it.
	_voice.set("stem_on_screen", renderer != null and renderer.shows_stem())
	_voice.call("present", caption, null, false)
	if url.is_empty() or context.media == null:
		return
	var clip: AudioStream = await context.media.audio(url)
	if clip != null and renderer != null and renderer.current_position() == item.position:
		_voice.call("present", caption, clip, context.auto_voice)


## F098: a tile of a one-step question tapped (or its key pressed). With tap-to-answer it sends the answer at once
## through the same path as "Kiểm tra" (the state turns AWAITING_RESULT before any further tap is handled, so an
## answer is never sent twice); a band confirming with two taps waits for a second tap on the same tile.
func _on_choice_tapped(repeated: bool) -> void:
	if not _instant or state != State.PRESENTING or renderer == null or not renderer.is_answer_complete():
		return
	if _double_tap and not repeated:
		_refresh_coach()
		return
	await press_primary()


## "Đọc to" on the question card (F097): same as key R.
func _on_read_aloud_requested() -> void:
	_voice.call("replay")


func _on_hint_requested() -> void:
	if renderer == null or service.attempt == null or not state in [State.PRESENTING, State.FEEDBACK]:
		return
	_hints.call("set_busy", true)
	var position: int = renderer.current_position()
	var hint: HintResultModel = await service.request_next_hint(service.attempt.attempt_id, position)
	_hints.call("set_busy", false)
	if hint != null:
		renderer.show_hint(hint)
		_hints.call("show_hint", hint, context.math)
		_refresh_coach()
		return
	match service.last_error_code:
		LearningService.CODE_HINTS_EXHAUSTED:
			_hints.call("mark_exhausted")
		LearningService.CODE_HINTS_NOT_ALLOWED:
			_hints.call("hide_hints")
		_:
			if service.attempt == null:
				_show_fatal(service.last_error_code)
			else:
				_hints.call("show_error")


func _on_worked_example_requested(lesson_id: String) -> void:
	var lesson: Dictionary = await service.get_lesson(lesson_id)
	if not lesson.is_empty() and visible:
		_worked.call("open_lesson", lesson, context)


func _on_math_failed() -> void:
	pass


# ---------- states and errors ----------

func _on_start_failed(known_type: String) -> void:
	var code: String = service.last_error_code
	if code == LearningService.CODE_VIDEO_NOT_READY:
		_show_error(code, start.bind(args), "ui.video.not_ready")
	elif code == LearningService.CODE_MEDIA_NOT_READY:
		_show_error(code, start.bind(args), "ui.audio.not_ready")
	elif code == LearningService.CODE_NOT_SUPPORTED:
		_show_update_required(known_type if not known_type.is_empty() else String(args.get("activity_type", "")))
	elif code in LearningService.ACCESS_DENIED_CODES:
		close()
	else:
		_show_error(code, start.bind(args), "ui.activity_player.error_load")


## A refused answer/action (F063-AC-10): the message `ui.error.<CODE>` with the code under "Chi tiết"; what the
## student entered stays on screen. A closed attempt goes to its result; a vanished attempt shows nothing of it.
func _on_action_failed(code: String) -> void:
	if service.attempt == null:
		_show_fatal(code)
		return
	if code in CLOSED_CODES:
		_status.text = L10n.error_text(code)
		await _show_results(true)
		return
	_set_state(State.PRESENTING)
	# F098: the tiles open again with the choice kept; "Gửi lại" (or a tap on the same tile) sends it again.
	if renderer != null:
		renderer.set_grading(false)
	_status.text = L10n.error_text(code)
	_status.visible = true
	_action_details.set_meta("code", _details_text(code))
	_action_details.visible = true
	_render_action_bar()


## 404 / session refusal (F063-AC-11): no data of the attempt stays on screen.
func _show_fatal(code: String) -> void:
	_clear_body()
	_set_header("", false)
	if code in LearningService.ACCESS_DENIED_CODES:
		close()
		return
	_show_error(code, Callable(), "ui.activity_player.error_load")


func _show_error(code: String, retry: Callable, fallback_key: String = "") -> void:
	_retry_action = retry
	_set_state(State.ERROR)
	_state_label.text = L10n.error_text(code) if ("ui.error." + code) in L10n.ERROR_KEYS or fallback_key.is_empty() \
			else tr(fallback_key)
	_details_label.text = _details_text(code)
	_details_label.visible = false
	_retry.visible = retry.is_valid()


func _show_update_required(activity_type: String) -> void:
	_clear_body()
	update_view = UPDATE_SCENE.instantiate() as Control
	_body.add_child(update_view)
	update_view.connect("back_pressed", close)
	service.report_update_required(activity_type)
	_set_state(State.UPDATE_REQUIRED)


func _time_up() -> void:
	_set_state(State.TIME_UP)
	if renderer != null:
		renderer.set_locked(true)
	var attempt_id: String = service.attempt.attempt_id if service.attempt != null else ""
	var reloaded: AttemptModel = await service.resume(attempt_id)
	if not visible:
		return
	if reloaded == null:
		_show_fatal(service.last_error_code)
		return
	await _show_results(true)


func _details_text(code: String) -> String:
	return "%s %s" % [code, service.last_trace_id] if service != null else code


## "Chi tiết": the error code and trace id, for school staff helping the student (GODOT_ARCHITECTURE §9).
func _on_action_details_pressed() -> void:
	var detail: String = String(_action_details.get_meta("code", ""))
	if not detail.is_empty() and not _status.text.ends_with(detail):
		_status.text = "%s (%s)" % [_status.text, detail]


func _on_retry_pressed() -> void:
	if _retry_action.is_valid():
		var action: Callable = _retry_action
		_retry_action = Callable()
		await action.call()


func _on_pause_pressed() -> void:
	_exit_confirm.hide()
	if renderer != null:
		var report: Dictionary = renderer.final_report()
		if not report.is_empty():
			await service.report_progress(report)
	await service.pause()
	close()


## A media heartbeat (F065-AC-19): the server state goes back to the renderer; a refused seek returns it to the frontier.
func _on_progress_reported(report: Dictionary) -> void:
	if service == null or service.attempt == null:
		return
	var st: Dictionary = await service.report_progress(report)
	if renderer == null or not visible:
		return
	if not st.is_empty():
		renderer.apply_progress(st)
	else:
		renderer.progress_refused(service.last_error_code)


## The simulation template cannot run in this client (F066-AC-21): telemetry + the update screen, no crash.
func _on_template_load_failed(_reason: String, activity_type: String) -> void:
	if service != null:
		service.report_update_required(activity_type)
	_show_update_required.call_deferred(activity_type)


## A simulation run (F066-AC-22, AC-23): sent with its key; the renderer shows only what the server verified.
func _on_run_submitted(run: Dictionary) -> void:
	if service == null or service.attempt == null:
		return
	var result: Dictionary = await service.submit_simulation_run(run)
	if renderer == null or not visible:
		return
	if not result.is_empty():
		renderer.show_run_result(result)
	else:
		renderer.run_refused(service.last_error_code)


## The media renderer reached the end with nothing pending (F065-AC-21): submit; a finished mini-game shows its result.
func _on_finish_requested() -> void:
	if state in [State.PRESENTING, State.FEEDBACK]:
		# A mini-game submits its attempt itself (F073): go straight to the result screen.
		if service.attempt != null and service.attempt.is_finished():
			await _show_results(true)
			return
		await _submit()


func _on_answer_changed(_complete: bool) -> void:
	if state == State.PRESENTING:
		_status.visible = false
		_action_details.visible = false
	_render_action_bar()


func _set_state(next: State) -> void:
	state = next
	_state_sec = 0.0
	var screen_states: Array[State] = [State.LOADING, State.ERROR]
	_state_panel.visible = next in screen_states
	if next == State.LOADING:
		_state_label.text = tr("ui.activity_player.loading")
		_retry.visible = false
		_details.visible = false
		_details_label.visible = false
	elif next == State.ERROR:
		_details.visible = true
	var answering: bool = next in [State.PRESENTING, State.AWAITING_RESULT, State.FEEDBACK, State.SUBMITTING, State.TIME_UP]
	# F097: after grading the bubble holds the verdict; the hint button returns with the next question.
	# F098: while a tapped answer is graded the bubble only cheers (the tile says "Đang chấm…").
	_hints.visible = next in [State.PRESENTING, State.AWAITING_RESULT] and bool(_hints.get("allowed")) \
			and not (_instant and next == State.AWAITING_RESULT)
	if not answering:
		_feedback.call("clear")
		_voice.visible = false
	if next == State.TIME_UP:
		_status.text = "ui.activity_player.time_up"
	_trail.visible = answering and _trail.total > 1
	_render_action_bar()


func _render_action_bar() -> void:
	var answering: bool = state in [State.PRESENTING, State.AWAITING_RESULT, State.FEEDBACK, State.SUBMITTING,
			State.TIME_UP]
	_primary.visible = answering
	_offline.visible = service != null and service.offline and answering
	match state:
		State.PRESENTING:
			# F098: with tap-to-answer there is no "Kiểm tra" step; the button returns as "Gửi lại" after a refusal
			# (and stays for the second-tap confirmation of a band that asks for it).
			var resend: bool = _instant and _status.visible
			_primary.visible = (renderer == null or renderer.wants_action_bar()) and (not _instant or _double_tap or resend)
			_primary.text = RESEND_KEY if resend else "ui.activity_player.check"
			_primary.icon = ICON_RESEND if resend else ICON_CHECK
			var complete: bool = renderer != null and renderer.is_answer_complete()
			_primary.disabled = not complete
			# F097: "Chọn một đáp án" is the question's own instruction; the bar does not repeat it.
		State.FEEDBACK:
			_primary.disabled = false
			_primary.text = "ui.activity_player.finish" if renderer != null and renderer.is_last() \
					else "ui.activity_player.next"
			_primary.icon = ICON_NEXT
			_status.visible = false
		State.AWAITING_RESULT, State.SUBMITTING:
			# F098: the tapped tile already says "Đang chấm…"; no second waiting button.
			_primary.visible = not (_instant and state == State.AWAITING_RESULT)
			_primary.disabled = true
			_primary.text = "ui.activity_player.waiting"
			_primary.icon = null
			_status.visible = false
		State.TIME_UP:
			_primary.disabled = true
			_status.visible = true
	_refresh_coach()


## The coach row (F097): Mây + bubble while answering. The bubble shows the verdict, the hint and errors; when it has
## nothing else it cheers ("Em làm được mà!", with "bấm Gợi ý" when hints are allowed) — never a countdown, a streak
## or a comparison (AGENTS R19, NFR-SAFE-003).
func _refresh_coach() -> void:
	var answering: bool = state in [State.PRESENTING, State.AWAITING_RESULT, State.FEEDBACK, State.SUBMITTING,
			State.TIME_UP]
	(_coach.get_parent() as Control).visible = answering
	_action_row.visible = _status.visible or _action_details.visible
	var busy: bool = _feedback.visible or _action_row.visible or bool(_hints.call("is_showing_hint"))
	_coach_line.visible = not busy
	_coach_line.text = TAP_AGAIN_KEY if _confirm_ready() else (CHEER_HINT_KEY if _hints.visible else CHEER_KEY)


## F098: a band that confirms with two taps has a chosen tile waiting for its second tap.
func _confirm_ready() -> bool:
	return _double_tap and state == State.PRESENTING and renderer != null and renderer.is_answer_complete()


func _set_header(title: String, translation_missing: bool) -> void:
	_title.text = title
	_no_translation.visible = translation_missing
	_progress.text = ""
	_clock.visible = false


func _clear_body() -> void:
	if renderer != null:
		renderer.teardown()
	for child: Node in _body.get_children():
		_body.remove_child(child)
		child.queue_free()
	renderer = null
	result_view = null
	update_view = null
	_feedback.call("clear")
	_status.visible = false


func _supported(activity_type: String, min_version: String) -> bool:
	var mins: PackedStringArray = [min_version, String(service.activity_min_client_versions.get(activity_type, ""))]
	return ActivityRendererRegistry.check(activity_type, client_version, mins) == ActivityRendererRegistry.REASON_OK


func _make_context(attempt: AttemptModel) -> RendererContext:
	var ctx: RendererContext = RendererContext.new()
	ctx.content_language = attempt.content_language()
	ctx.grade_band = grade_band
	ctx.math = services.math_svg
	ctx.media = services.media
	ctx.video = services.video_files
	ctx.clock = service.clock
	ctx.auto_voice = grade_band == BAND_NO_CLOCK
	return ctx


func _bind_service(learning: LearningService) -> void:
	if service == learning:
		return
	service = learning
	service.offline_changed.connect(func(_offline_now: bool) -> void: _render_action_bar())
	service.access_denied.connect(_on_access_denied)


func _on_access_denied(code: String) -> void:
	if visible:
		_show_fatal(code)


func _process(delta: float) -> void:
	if not visible:
		return
	_state_sec += delta
	if state == State.LOADING and _state_sec >= LOAD_RETRY_AFTER_SEC and not _retry.visible:
		_retry_action = start.bind(args)
		_retry.visible = true
	if service != null and service.offline and not _flushing:
		_offline_accum += delta
		if _offline_accum >= OFFLINE_RETRY_SEC:
			_offline_accum = 0.0
			_flush()
	_clock_accum += delta
	if _clock_accum >= 1.0:
		_clock_accum = 0.0
		_tick_clock()


## Re-sends queued answers; the grading of the current item continues the flow (F063-AC-06).
func _flush() -> void:
	_flushing = true
	var results: Array[ResponseResultModel] = await service.flush_pending()
	_flushing = false
	if not visible or renderer == null:
		return
	for result: ResponseResultModel in results:
		if result.position == renderer.current_position() and state == State.AWAITING_RESULT:
			_apply_result(result)
	_render_action_bar()


## Countdown on the server clock, once per second (P-6); time up locks input and loads the result.
func _tick_clock() -> void:
	var attempt: AttemptModel = service.attempt if service != null else null
	if attempt == null or not state in [State.PRESENTING, State.AWAITING_RESULT, State.FEEDBACK]:
		return
	var left: int = attempt.seconds_left(Time.get_unix_time_from_system())
	_clock.visible = left >= 0
	if left < 0:
		return
	_clock.text = tr(TIMED_KEY) if grade_band == BAND_NO_CLOCK else "%02d:%02d" % [left / 60, left % 60]
	if left == 0:
		_time_up()


func _unhandled_input(event: InputEvent) -> void:
	if not visible or not event is InputEventKey or not event.is_pressed() or event.is_echo():
		return
	var key: Key = (event as InputEventKey).keycode
	var handled: bool = true
	if _exit_confirm.visible:
		if key == KEY_ESCAPE:
			_exit_confirm.hide()
		else:
			handled = false
	elif key == KEY_ENTER or key == KEY_KP_ENTER:
		press_primary()
	elif key == KEY_ESCAPE:
		ask_exit()
	elif key == KEY_H:
		_hints.call("request")
	elif key == KEY_R:
		_voice.call("replay")
	elif key >= KEY_1 and key <= KEY_6 and renderer != null and state == State.PRESENTING:
		renderer.select_index(key - KEY_1)
	else:
		handled = false
	if handled:
		get_viewport().set_input_as_handled()


## F069: room left at the top for the solo-challenge strip (ChallengeHud) while a run is open; 0 = full screen.
func set_top_inset(px: float) -> void:
	var frame: Control = get_node_or_null("%Frame") as Control
	if frame != null:
		frame.offset_top = maxf(0.0, px)
