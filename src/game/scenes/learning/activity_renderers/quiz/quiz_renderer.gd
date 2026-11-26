class_name QuizActivityRenderer
extends ActivityRendererBase
## Renderer of `quiz` (and base of `interactive_exercise`): one question item at a time from `attempt.items`, drawn by
## the scene QuestionRendererRegistry gives for its `type` (GODOT_ARCHITECTURE §6.2). Resumes at the first item without
## a saved answer and restores saved answers (F063-AC-07). "Kiểm tra" turns the question's `response_prepared` into an
## `ActivityResponseSubmission` `{kind: question, position, response, client_elapsed_ms}` (F063-AC-05); the elapsed
## time is informative only — the server uses its own clock (ADR-0016). An item type this client cannot draw shows
## `ui.activity_player.update_required` in the content area (F063-AC-26).

const UNSUPPORTED_KEY: String = "ui.activity_player.update_required"

var index: int = -1
var question: QuestionRendererBase = null
var unsupported: bool = false
var shown_at_msec: int = 0


func setup(attempt_model: AttemptModel, ctx: RendererContext) -> void:
	super.setup(attempt_model, ctx)
	if not attempt.items.is_empty():
		show_index(attempt.resume_index())


func show_index(i: int) -> void:
	_clear()
	index = clampi(i, 0, attempt.items.size() - 1)
	var item: QuestionRenderModel = attempt.items[index]
	var content: Control = %Content as Control
	question = QuestionRendererRegistry.instantiate(item.type)
	unsupported = question == null
	if unsupported:
		var notice: Label = Label.new()
		notice.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		notice.text = UNSUPPORTED_KEY
		content.add_child(notice)
	else:
		wire_question(question)
		content.add_child(question)
		question.setup(item, context)
		question.restore(attempt.saved_response(item.position))
		question.answer_changed.connect(_on_question_changed)
		question.response_prepared.connect(_on_response_prepared)
		question.choice_tapped.connect(func(repeated: bool) -> void: choice_tapped.emit(repeated))
	shown_at_msec = context.now_msec() if context != null else Time.get_ticks_msec()
	item_shown.emit(item.position)
	answer_changed.emit(is_answer_complete())


func check() -> bool:
	return question != null and question.commit()


func show_result(result: ResponseResultModel) -> void:
	if question != null:
		question.set_locked(true)
		# F098: the graded tile shows the server's verdict (and the right answer only when the server sent it).
		question.show_verdict(result)


func supports_instant_answer() -> bool:
	return true


func set_instant_answer(on: bool, two_taps: bool = false) -> void:
	if question != null:
		question.set_instant_answer(on, two_taps)


func set_grading(on: bool) -> void:
	if question != null:
		question.set_grading(on)


func advance() -> bool:
	if is_last():
		return false
	show_index(index + 1)
	return true


func current_position() -> int:
	var item: QuestionRenderModel = current_item()
	return item.position if item != null else 0


func current_item() -> QuestionRenderModel:
	return attempt.items[index] if attempt != null and index >= 0 and index < attempt.items.size() else null


func is_last() -> bool:
	return attempt == null or index >= attempt.items.size() - 1


func progress() -> Vector2i:
	return Vector2i(index + 1, attempt.items.size() if attempt != null else 0)


func is_answer_complete() -> bool:
	return question != null and question.is_complete() and not question.locked


func select_index(i: int) -> void:
	if question != null:
		question.select_index(i)


func set_locked(locked: bool) -> void:
	if question != null:
		question.set_locked(locked)


func current_item_unsupported() -> bool:
	return unsupported


func teardown() -> void:
	_clear()


func _clear() -> void:
	var content: Control = get_node_or_null("%Content") as Control
	if content != null:
		for child: Node in content.get_children():
			content.remove_child(child)
			child.queue_free()
	question = null
	unsupported = false


func _on_question_changed(complete: bool) -> void:
	answer_changed.emit(complete and not question.locked)


func _on_response_prepared(position: int, response: Dictionary) -> void:
	var now: int = context.now_msec() if context != null else Time.get_ticks_msec()
	response_submitted.emit({
		"kind": "question", "position": position, "response": response, "client_elapsed_ms": maxi(0, now - shown_at_msec),
	})
