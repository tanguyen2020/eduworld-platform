class_name SimulationRenderer
extends ActivityRendererBase
## Renderer of `simulation` (F066 US3, SIMULATION_ENGINE, STUDENT_GAME_UX_LEARNING §8): the SimulationHost with the
## packaged template, the goals (ticked only from the server's `SimulationRunResult`), the steps, "Thả" which sends
## the run (`run_submitted`, same Idempotency-Key on retry), "Làm lại", the runs left, and the conclusion questions
## (`kind = simulation_question`) through the F063 question renderers. A missing template shows
## `ui.activity_player.update_required` and emits `template_load_failed` (telemetry by the host), never a crash.
## Display and intents only: the server checker decides goals, score and completion (ADR-0052, AGENTS R1/R5).

## The template of the render cannot run in this client (`update_required`, `pack_mismatch`).
signal template_load_failed(reason: String)

var payload: Dictionary = {}
var goals: Array[Dictionary] = []
var questions: Array[Dictionary] = []
var answered: PackedInt32Array = []
var max_runs: int = -1
var runs_used: int = 0
var locked_runs: bool = false
var goal_reached: bool = false
var host: SimulationHost = null
var question: QuestionRendererBase = null
var open_position: int = 0
## The run sent and not answered yet (a retry resends exactly it, so the key stays the same).
var pending_run: Dictionary = {}
var _goal_boxes: Dictionary = {}
var _shown_at_msec: int = 0


func setup(attempt_model: AttemptModel, ctx: RendererContext) -> void:
	super.setup(attempt_model, ctx)
	payload = attempt.render.render_payload if attempt.render != null else {}
	var interaction: Dictionary = payload.get("interaction", {}) as Dictionary if payload.get("interaction") is Dictionary else {}
	var mr: Variant = interaction.get("max_runs")
	max_runs = int(mr) if mr is int or mr is float else -1
	runs_used = int(payload.get("runs_used", 0))
	goals = VideoLessonRenderer._dicts(payload.get("goals"))
	questions = VideoLessonRenderer._dicts(payload.get("questions"))
	for p: Variant in attempt.saved.keys():
		answered.append(int(p))
	_build_goals()
	_build_steps()
	(%Overlay as Control).visible = false
	(%Missing as Control).visible = false
	host = %Host as SimulationHost
	var lite: bool = QualityManager.preset == QualityPolicy.LOW or OS.has_feature("web")
	if not host.open(payload, lite):
		(%Missing as Control).visible = true
		(%Controls as Control).visible = false
		template_load_failed.emit(host.last_reason)
		return
	_bind()
	locked_runs = max_runs >= 0 and runs_used >= max_runs
	_refresh()


func _build_goals() -> void:
	var box: Control = %Goals as Control
	for c: Node in box.get_children():
		c.queue_free()
	_goal_boxes.clear()
	for g: Dictionary in goals:
		var cb: CheckBox = CheckBox.new()
		cb.disabled = true
		cb.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		cb.text = context.text(g.get("description")) if context != null else String(g.get("goal_id", ""))
		box.add_child(cb)
		_goal_boxes[String(g.get("goal_id", ""))] = cb


func _build_steps() -> void:
	var box: Control = %Steps as Control
	for c: Node in box.get_children():
		c.queue_free()
	for s: Dictionary in VideoLessonRenderer._dicts(payload.get("steps")):
		var l: Label = Label.new()
		l.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		l.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		l.text = "• " + (context.text(s.get("instruction")) if context != null else String(s.get("step_id", "")))
		box.add_child(l)


func _bind() -> void:
	var release_btn: Button = %Release as Button
	if not release_btn.pressed.is_connected(release):
		release_btn.pressed.connect(release)
		(%Retry as Button).pressed.connect(retry)
		(%Questions as Button).pressed.connect(open_next_question)


# ---------- runs (AC-22, AC-23) ----------

## "Thả": sends the run (or the same run again when the last send got no answer).
func release() -> void:
	if locked_runs or host == null:
		return
	if pending_run.is_empty():
		pending_run = host.release()
	_status("ui.simulation.checking")
	(%Release as Button).disabled = true
	run_submitted.emit(pending_run)


func retry() -> void:
	if locked_runs:
		return
	pending_run = {}
	goal_reached = false
	for cb: Variant in _goal_boxes.values():
		(cb as CheckBox).button_pressed = false
	host.new_run()
	_status("")
	_refresh()


func show_run_result(result: Dictionary) -> void:
	pending_run = {}
	runs_used = int(result.get("runs_used", runs_used + 1))
	var mr: Variant = result.get("max_runs")
	if mr is int or mr is float:
		max_runs = int(mr)
	for g: Dictionary in VideoLessonRenderer._dicts(result.get("goals_met")):
		var cb: CheckBox = _goal_boxes.get(String(g.get("goal_id", ""))) as CheckBox
		if cb != null:
			cb.button_pressed = bool(g.get("met", false)) and bool(result.get("verified", false))
	goal_reached = bool(result.get("goal_met", false)) and bool(result.get("verified", false))
	_status("ui.simulation.goal_reached" if goal_reached else "ui.simulation.goal_not_reached")
	locked_runs = max_runs >= 0 and runs_used >= max_runs
	_refresh()


func run_refused(code: String) -> void:
	match code:
		LearningService.CODE_LOG_INVALID:
			pending_run = {}
			runs_used += 1
			_status("ui.simulation.log_invalid")
			locked_runs = max_runs >= 0 and runs_used >= max_runs
		LearningService.CODE_RUN_LIMIT:
			pending_run = {}
			locked_runs = true
			_status("ui.simulation.runs_exhausted")
			open_next_question()
		_:
			# Network: the same run (same key) is sent again by "Thả".
			_status("ui.activity_player.offline")
	_refresh()


func _refresh() -> void:
	var release_btn: Button = %Release as Button
	release_btn.disabled = locked_runs
	(%Retry as Button).disabled = locked_runs
	(%Runs as Label).visible = max_runs >= 0
	(%Runs as Label).text = "%d / %d" % [runs_used, max_runs]
	(%Questions as Button).visible = not questions.is_empty() and _unanswered() > 0
	if locked_runs and (%Status as Label).text != "ui.simulation.runs_exhausted" and not goal_reached:
		_status("ui.simulation.runs_exhausted")


func _status(key: String) -> void:
	var l: Label = %Status as Label
	l.text = key
	l.visible = not key.is_empty()


# ---------- conclusion questions ----------

func _unanswered() -> int:
	var n: int = 0
	for q: Dictionary in questions:
		if not int(q.get("position", 0)) in answered:
			n += 1
	return n


## Opens the next unanswered conclusion question; none left → the attempt can be submitted.
func open_next_question() -> bool:
	for q: Dictionary in questions:
		var position: int = int(q.get("position", 0))
		if position in answered:
			continue
		open_position = position
		var item: QuestionRenderModel = attempt.item_at(position)
		var content: Control = %OverlayContent as Control
		for child: Node in content.get_children():
			child.queue_free()
		question = QuestionRendererRegistry.instantiate(item.type) if item != null else null
		if question == null:
			var notice: Label = Label.new()
			notice.text = "ui.activity_player.update_required"
			content.add_child(notice)
		else:
			wire_question(question)
			content.add_child(question)
			question.setup(item, context)
			question.answer_changed.connect(func(complete: bool) -> void: answer_changed.emit(complete and not question.locked))
			question.response_prepared.connect(_on_response_prepared)
		(%Overlay as Control).visible = true
		_shown_at_msec = Time.get_ticks_msec()
		item_shown.emit(position)
		answer_changed.emit(false)
		return true
	finish_requested.emit()
	return false


func _on_response_prepared(position: int, response: Dictionary) -> void:
	response_submitted.emit({"kind": "simulation_question", "position": position, "response": response,
		"client_elapsed_ms": maxi(0, Time.get_ticks_msec() - _shown_at_msec)})


func check() -> bool:
	return question != null and (%Overlay as Control).visible and question.commit()


func show_result(_result: ResponseResultModel) -> void:
	if question != null:
		question.set_locked(true)
	if open_position > 0 and not open_position in answered:
		answered.append(open_position)


func advance() -> bool:
	if not (%Overlay as Control).visible:
		return _unanswered() > 0
	(%Overlay as Control).visible = false
	open_position = 0
	_refresh()
	if _unanswered() > 0:
		return open_next_question()
	finish_requested.emit()
	return false


func is_last() -> bool:
	return _unanswered() <= 1 and (%Overlay as Control).visible


func is_answer_complete() -> bool:
	return question != null and (%Overlay as Control).visible and question.is_complete() and not question.locked


func wants_action_bar() -> bool:
	return (%Overlay as Control).visible or (questions.is_empty() and (goal_reached or locked_runs))


func current_position() -> int:
	return open_position


func current_item() -> QuestionRenderModel:
	return attempt.item_at(open_position) if attempt != null else null


func progress() -> Vector2i:
	return Vector2i(questions.size() - _unanswered(), questions.size())


func select_index(i: int) -> void:
	if question != null:
		question.select_index(i)


func set_locked(locked: bool) -> void:
	if question != null:
		question.set_locked(locked)


func teardown() -> void:
	if host != null:
		host.close()
