class_name MiniGameHost
extends Control
## Mini-game player (MINIGAME_ENGINE §4–§5, F073 US5/US6). Opened two ways:
## - screen `minigame_host` (free play from an NPC / station, `{minigame_config_id, context_type?, context_id?}`):
##   `startMiniGame` through LearningService (Idempotency-Key) → `MiniGameSession`;
## - inside the activity player (quest step, school assignment): MinigameRenderer calls `play()` with the session read
##   from the `minigame` activity attempt (`embedded`; ActivityHost keeps its header, exit, offline re-send and time-up).
## It picks the scene of `minigame_type` in MiniGameRegistry (unknown type → "Cần cập nhật game" + telemetry,
## F073-AC-21), feeds it the items one by one, sends each `response_prepared` with LearningService.respond (one
## Idempotency-Key per answer; a network loss queues it and the same key is re-sent when the network is back,
## F073-AC-23), hands the server's grading back (`show_feedback`, game points and streak from the server), submits at
## the end and shows the server's summary. `timed_challenge` (`prefetch_items`) gets every item up front: the next item
## shows at once and the grading of the previous one arrives in the background — still graded by the server
## (F073-AC-26). The countdown uses `deadline_at` and `server_time` with the local clock offset, once per second, and
## never stops while offline (F073-AC-23). UI glue only (AGENTS R1, R5): no scoring, no answer key, nothing logged.

signal closed()
## Embedded: the student left the summary ("Tiếp tục").
signal finished()
## Embedded: this client has no scene for `minigame_type` (ActivityHost shows its update screen).
signal update_required(minigame_type: String)

enum State { IDLE, LOADING, PLAYING, AWAITING_RESULT, FEEDBACK, SUBMITTING, FINISHED, ERROR, UPDATE_REQUIRED, TIME_UP }

const SUSPEND_REASON: StringName = &"minigame"
const UPDATE_SCENE: PackedScene = preload("res://scenes/learning/UpdateRequired.tscn")
## Re-send of queued answers / of the submit while offline (ApiClient already backs off inside one request).
const OFFLINE_RETRY_SEC: float = 5.0
const BAND_NO_CLOCK: String = "g1_3"
const HOURGLASS: String = "⏳"
## Codes after which the attempt can no longer be answered: the host reloads it and shows the summary.
const CLOSED_CODES: PackedStringArray = ["ATTEMPT_EXPIRED", "ATTEMPT_NOT_IN_PROGRESS", "ATTEMPT_ALREADY_SUBMITTED",
	"ATTEMPT_DEADLINE_PASSED"]
const PLAY_STATES: Array[State] = [State.PLAYING, State.AWAITING_RESULT, State.FEEDBACK]

## WorldRoot holds the break reminder while a learning screen is open (F062-AC-22).
var blocks_break_reminder: bool = true
var state: State = State.IDLE
var args: Dictionary = {}
var services: ServiceLocator = null
var service: LearningService = null
var session: MiniGameSessionModel = null
var scene: MiniGameBase = null
var context: RendererContext = null
var update_view: Control = null
## Inside ActivityHost (MinigameRenderer): no own exit, clock label, keyboard or world suspension.
var embedded: bool = false
var grade_band: String = ""
var index: int = -1
## position → ResponseResultModel received from the server.
var results: Dictionary = {}
## positions answered (sent, queued or saved on resume).
var answered: Dictionary = {}
## Sum of the server's `game_points` of this play (display only; the final score is the submit's `game_score`).
var game_points: int = 0
var streak: int = 0
var summary: MiniGameSummaryModel = null
var _retry_action: Callable = Callable()
var _shown_at_msec: int = 0
var _clock_accum: float = 0.0
var _offline_accum: float = 0.0
var _busy: bool = false
var _submit_waiting: bool = false

@onready var _hud: PanelContainer = %Hud
@onready var _exit: Button = %Exit
@onready var _title: Label = %Title
@onready var _progress: Label = %Progress
@onready var _points: Label = %Points
@onready var _streak: Label = %Streak
@onready var _clock: Label = %Clock
@onready var _offline: Label = %Offline
@onready var _stage: VBoxContainer = %Stage
@onready var _state_panel: VBoxContainer = %StatePanel
@onready var _state_label: Label = %StateLabel
@onready var _retry: Button = %Retry
@onready var _back: Button = %Back
@onready var _status: Label = %Status
@onready var _next: Button = %Next
@onready var _continue: Button = %Continue


func _ready() -> void:
	if not embedded:
		visible = false
	_exit.pressed.connect(ask_exit)
	_next.pressed.connect(next_item)
	_continue.pressed.connect(_on_continue)
	_retry.pressed.connect(_on_retry_pressed)
	_back.pressed.connect(close)
	_set_state(State.IDLE)


## WorldRoot: `ui_requested("minigame_host", {minigame_config_id, context_type?, context_id?})`.
func open(intent: Dictionary = {}) -> void:
	args = intent
	if services == null:
		services = AppState.get_services()
	bind_service(services.learning)
	if grade_band.is_empty():
		grade_band = AppState.grade_band
	show()
	QualityManager.suspend_world(SUSPEND_REASON)
	await start(intent)


## Coroutine: `startMiniGame` then the game.
func start(intent: Dictionary) -> void:
	args = intent
	_clear_stage()
	_set_state(State.LOADING)
	var s: MiniGameSessionModel = await service.start_minigame(String(intent.get("minigame_config_id", "")),
			String(intent.get("context_type", "free_play")), String(intent.get("context_id", "")))
	if not visible:
		return
	if s == null:
		if service.last_error_code in LearningService.ACCESS_DENIED_CODES:
			close()
			return
		_show_error(service.last_error_code, start.bind(intent))
		return
	await play(s)


## Shows `s`: the scene of its type, resuming at the first unanswered item; a finished attempt shows its summary.
func play(s: MiniGameSessionModel, ctx: RendererContext = null) -> void:
	_clear_stage()
	session = s
	results.clear()
	answered.clear()
	game_points = 0
	streak = 0
	summary = null
	index = -1
	_submit_waiting = false
	context = ctx if ctx != null else _make_context(s)
	_title.text = s.config.display_name(context.content_language)
	_render_hud()
	if not MiniGameRegistry.has(s.minigame_type):
		_show_update_required(s.minigame_type)
		return
	scene = MiniGameRegistry.instantiate(s.minigame_type)
	scene.size_flags_vertical = Control.SIZE_EXPAND_FILL
	_stage.add_child(scene)
	scene.response_prepared.connect(_on_response_prepared)
	scene.quit_requested.connect(ask_exit)
	scene.setup(s.config, s, context)
	scene.apply_touch_targets(self)
	if s.attempt.is_finished():
		summary = MiniGameSummaryModel.from_attempt(s.attempt)
		_finish_scene()
		return
	for pos: Variant in s.attempt.saved.keys():
		answered[int(pos)] = true
	tick_clock()
	if not _present_next():
		await _submit()


## Enter / main action: answer (or skip an item this client cannot draw), next, leave the summary.
func press_primary() -> bool:
	match state:
		State.PLAYING:
			if scene != null and scene.current_item_unsupported():
				_move_on()
				return true
			return scene != null and scene.press_primary()
		State.FEEDBACK:
			next_item()
			return true
		State.FINISHED:
			_on_continue()
			return true
	return false


## Keyboard `1–6` on the current item.
func select_index(i: int) -> void:
	if state == State.PLAYING and scene != null:
		scene.select_index(i)


func lock(value: bool) -> void:
	if scene != null:
		scene.set_locked(value)


## "Tiếp" after the feedback of an item.
func next_item() -> void:
	if state == State.FEEDBACK or (state == State.PLAYING and scene != null and scene.current_item_unsupported()):
		await _move_on()


## Esc / "Thoát": a resumable game is paused on the server (progress kept); then the screen closes.
func ask_exit() -> void:
	if embedded:
		return
	if state in PLAY_STATES and session != null and session.config.resumable and service != null:
		await service.pause()
	close()


func close() -> void:
	teardown()
	_set_state(State.IDLE)
	if service != null and service.attempt != null and service.attempt.is_finished():
		service.clear()
	if not embedded:
		hide()
		QualityManager.resume_world(SUSPEND_REASON)
	closed.emit()


func teardown() -> void:
	_clear_stage()
	session = null
	_submit_waiting = false


func bind_service(learning: LearningService) -> void:
	if service == learning or learning == null:
		return
	service = learning
	service.response_result_received.connect(_on_result_received)
	service.offline_changed.connect(_on_offline_changed)
	service.access_denied.connect(_on_access_denied)


## Once per second: the server-clock countdown (keeps running offline); time up locks the game.
func tick_clock() -> void:
	if session == null:
		return
	var ms: int = session.remaining_ms(_now_unix())
	if scene != null:
		scene.update_timer(ms)
	_clock.visible = ms >= 0 and not embedded
	if ms < 0:
		return
	var left: int = int(ceil(ms / 1000.0))
	_clock.text = HOURGLASS if grade_band == BAND_NO_CLOCK else "%s %02d:%02d" % [HOURGLASS, left / 60, left % 60]
	if ms == 0 and state in PLAY_STATES:
		await _time_up()


func current_item() -> QuestionRenderModel:
	var list: Array[QuestionRenderModel] = session.items() if session != null else [] as Array[QuestionRenderModel]
	return list[index] if index >= 0 and index < list.size() else null


func is_last() -> bool:
	if session == null:
		return true
	var list: Array[QuestionRenderModel] = session.items()
	for i: int in range(index + 1, list.size()):
		if not answered.has(list[i].position):
			return false
	return true


# ---------- answers ----------

func _on_response_prepared(item_id: String, response: Dictionary) -> void:
	var current: QuestionRenderModel = current_item()
	if state != State.PLAYING or current == null or item_id != MiniGameBase.item_id_of(current):
		return
	var position: int = current.position
	answered[position] = true
	var elapsed: int = maxi(0, _now_msec() - _shown_at_msec)
	if session.config.prefetch_items:
		_send_in_background(position, response, elapsed)
		await _move_on()
		return
	_set_state(State.AWAITING_RESULT)
	var sent: bool = await _send(position, response, elapsed)
	if state != State.AWAITING_RESULT or session == null:
		return
	if sent or service.offline:
		# Graded without a verdict to show, or queued offline (re-sent with its key): the game goes on.
		_set_state(State.FEEDBACK)
		return
	var code: String = service.last_error_code
	if code in CLOSED_CODES:
		await _reload_and_finish()
		return
	answered.erase(position)
	_set_state(State.PLAYING)
	scene.set_locked(false)
	_status.text = L10n.error_text(code)
	_status.visible = true


## timed_challenge: the answer goes out while the next item is already on screen (its grading arrives by signal).
func _send_in_background(position: int, response: Dictionary, elapsed: int) -> void:
	await _send(position, response, elapsed)


func _send(position: int, response: Dictionary, elapsed: int) -> bool:
	var result: ResponseResultModel = await service.respond(position, response, elapsed)
	_render_hud()
	return result != null


## Every grading of this attempt (direct answer, or a queued one re-sent by this host or by ActivityHost).
func _on_result_received(result: ResponseResultModel) -> void:
	if session == null or scene == null or results.has(result.position):
		return
	results[result.position] = result
	if result.has_game_points:
		game_points += result.game_points
		streak = result.streak
	_render_hud()
	scene.show_feedback(result)
	var current: QuestionRenderModel = current_item()
	if current != null and result.position == current.position and state == State.AWAITING_RESULT:
		_set_state(State.FEEDBACK)


func _present_next() -> bool:
	var list: Array[QuestionRenderModel] = session.items()
	for i: int in range(index + 1, list.size()):
		if answered.has(list[i].position):
			continue
		index = i
		scene.present_item(list[i])
		_shown_at_msec = _now_msec()
		_progress.text = "%d/%d" % [i + 1, list.size()]
		_status.visible = false
		_set_state(State.PLAYING)
		return true
	return false


func _move_on() -> void:
	if not _present_next():
		await _submit()


# ---------- submit, summary, time up ----------

func _submit() -> void:
	if _busy:
		return
	_busy = true
	_set_state(State.SUBMITTING)
	if scene != null:
		scene.set_locked(true)
	var submitted: AttemptModel = await service.submit()
	_busy = false
	if session == null:
		return
	if submitted != null:
		_submit_waiting = false
		summary = MiniGameSummaryModel.from_attempt(submitted, _graded())
		_finish_scene()
		return
	var code: String = service.last_error_code
	if service.attempt == null:
		_show_fatal(code)
	elif code in CLOSED_CODES:
		await _reload_and_finish()
	elif code == ApiResult.CODE_NETWORK_ERROR or service.offline or code.begins_with("HTTP_5") or code == "UNAVAILABLE":
		_submit_waiting = true
		_offline.visible = true
	else:
		_show_error(code, _submit)


## The server closed the attempt (deadline, already submitted): reload it and show what the server graded.
func _reload_and_finish() -> void:
	if session == null:
		return
	var reloaded: AttemptModel = await service.resume(session.attempt.attempt_id)
	if session == null:
		return
	if reloaded == null:
		_show_fatal(service.last_error_code)
		return
	summary = MiniGameSummaryModel.from_attempt(reloaded, _graded())
	_finish_scene()


func _finish_scene() -> void:
	_set_state(State.FINISHED)
	if summary.has_game_score:
		game_points = summary.game_score
	_render_hud()
	if scene != null:
		scene.finish(summary)
	_continue.grab_focus.call_deferred()


func _time_up() -> void:
	_set_state(State.TIME_UP)
	if scene != null:
		scene.set_locked(true)
	if embedded:
		return
	if not service.pending.is_empty():
		await service.flush_pending()
	await _submit()


func _on_continue() -> void:
	if embedded:
		finished.emit()
	else:
		close()


func _graded() -> Array[ResponseResultModel]:
	var out: Array[ResponseResultModel] = []
	for r: Variant in results.values():
		out.append(r as ResponseResultModel)
	return out


# ---------- offline ----------

func _process(delta: float) -> void:
	if not is_visible_in_tree() or session == null:
		return
	_clock_accum += delta
	if _clock_accum >= 1.0:
		_clock_accum = 0.0
		tick_clock()
	var wants_retry: bool = _submit_waiting or (not embedded and service != null and service.offline)
	if not wants_retry or _busy:
		_offline_accum = 0.0
		return
	_offline_accum += delta
	if _offline_accum >= OFFLINE_RETRY_SEC:
		_offline_accum = 0.0
		retry_pending()


## Re-sends the queued answers with their keys, then a submit that was waiting for the network.
func retry_pending() -> void:
	if _busy or service == null:
		return
	if not service.pending.is_empty():
		_busy = true
		await service.flush_pending()
		_busy = false
	if _submit_waiting and service.pending.is_empty():
		await _submit()
	_render_hud()


func _on_offline_changed(_offline_now: bool) -> void:
	_render_hud()


func _on_access_denied(_code: String) -> void:
	if visible and not embedded:
		close()


# ---------- states and views ----------

func _set_state(next: State) -> void:
	state = next
	var unsupported: bool = next == State.PLAYING and scene != null and scene.current_item_unsupported()
	_next.visible = next == State.FEEDBACK or unsupported
	_next.text = "ui.activity_player.finish" if is_last() else "ui.activity_player.next"
	_continue.visible = next == State.FINISHED
	_state_panel.visible = next in [State.LOADING, State.ERROR]
	if next == State.LOADING:
		_state_label.text = "ui.activity_player.loading"
		_retry.visible = false
	if next == State.SUBMITTING:
		_status.text = "ui.activity_player.waiting"
		_status.visible = true
	elif next == State.TIME_UP:
		_status.text = "ui.minigame.time_up"
		_status.visible = true
	elif next == State.FINISHED:
		_status.visible = false
	_hud.visible = next != State.IDLE
	_exit.visible = not embedded
	_back.visible = not embedded
	_render_hud()
	if _next.visible:
		_next.grab_focus.call_deferred()


func _render_hud() -> void:
	if _points == null:
		return
	_points.text = tr("ui.minigame.points").format({"points": game_points})
	_streak.text = tr("ui.minigame.streak").format({"streak": streak})
	_streak.visible = streak > 1
	_offline.visible = service != null and (service.offline or _submit_waiting) and state != State.IDLE


func _show_update_required(minigame_type: String) -> void:
	_clear_stage()
	if service != null:
		service.report_minigame_update_required(minigame_type)
	if embedded:
		_set_state(State.UPDATE_REQUIRED)
		update_required.emit(minigame_type)
		return
	update_view = UPDATE_SCENE.instantiate() as Control
	_stage.add_child(update_view)
	update_view.connect("back_pressed", close)
	_set_state(State.UPDATE_REQUIRED)


func _show_error(code: String, retry: Callable) -> void:
	_retry_action = retry
	_set_state(State.ERROR)
	_state_label.text = L10n.error_text(code) if not code.is_empty() else tr("ui.activity_player.error_load")
	_retry.visible = retry.is_valid()


func _show_fatal(code: String) -> void:
	_clear_stage()
	if code in LearningService.ACCESS_DENIED_CODES:
		close()
		return
	_show_error(code, Callable())


func _on_retry_pressed() -> void:
	if _retry_action.is_valid():
		var action: Callable = _retry_action
		_retry_action = Callable()
		await action.call()


func _clear_stage() -> void:
	if scene != null:
		scene.teardown()
	for child: Node in _stage.get_children():
		_stage.remove_child(child)
		child.queue_free()
	scene = null
	update_view = null
	if _status != null:
		_status.visible = false


func _make_context(s: MiniGameSessionModel) -> RendererContext:
	var ctx: RendererContext = RendererContext.new()
	ctx.content_language = s.attempt.content_language()
	ctx.grade_band = grade_band
	if services != null:
		ctx.math = services.math_svg
		ctx.media = services.media
	ctx.clock = service.clock if service != null else null
	ctx.auto_voice = grade_band == BAND_NO_CLOCK
	return ctx


func _now_unix() -> float:
	return service.clock.unix_time() if service != null and service.clock != null else Time.get_unix_time_from_system()


func _now_msec() -> int:
	return service.clock.now_msec() if service != null and service.clock != null else Time.get_ticks_msec()


func _unhandled_input(event: InputEvent) -> void:
	if embedded or not visible or not event is InputEventKey or not event.is_pressed() or event.is_echo():
		return
	var key: Key = (event as InputEventKey).keycode
	var handled: bool = true
	if key == KEY_ENTER or key == KEY_KP_ENTER:
		press_primary()
	elif key == KEY_ESCAPE:
		ask_exit()
	elif key >= KEY_1 and key <= KEY_6:
		select_index(key - KEY_1)
	else:
		handled = false
	if handled:
		get_viewport().set_input_as_handled()
