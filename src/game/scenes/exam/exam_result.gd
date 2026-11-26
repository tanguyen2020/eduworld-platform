extends VBoxContainer
## Exam result (STUDENT_GAME_UX_LEARNING §10, LEARNING_EXPERIENCE §7/§9, F064-AC-17…19). Shows only what the server
## released (`getExamResult`): the score by grade band (`g1_3`: right answers with a star, never a 10-point scale;
## others: the school-scale score), the per-objective rows (server verdicts tallied), the wrong questions with the
## explanation and option feedback only when the server sent them, then "Luyện LO này" for each weak objective and
## the server's practice recommendations. Not released yet → "Nhà trường sẽ công bố kết quả sau". No rank, no other
## student, no teacher comment (AGENTS R16). Emits intents; the lobby opens them. UI only.

## Arguments of an `activity_host` intent (a practice attempt).
signal practice_requested(args: Dictionary)

const BAND_COUNT_ONLY: String = "g1_3"

var result: ExamResultModel = null

@onready var _pending: Label = %Pending
@onready var _score: Label = %Score
@onready var _correct: Label = %Correct
## F096-AC-15: the `g1_3` star is an icon of design/icons/game (no emoji on the Web build).
@onready var _star: TextureRect = %StarIcon
@onready var _sections: VBoxContainer = %Sections
@onready var _objectives: VBoxContainer = %Objectives
@onready var _wrong: VBoxContainer = %WrongList
@onready var _practice: VBoxContainer = %Practice


## `403 EXAM_RESULTS_NOT_RELEASED` (or still being graded): only the waiting message (F064-AC-17).
func show_pending() -> void:
	result = null
	_clear()
	_pending.visible = true
	_score.visible = false
	_correct.visible = false


func show_result(model: ExamResultModel, grade_band: String, context: RendererContext) -> void:
	result = model
	_clear()
	_pending.visible = false
	var count_only: bool = grade_band == BAND_COUNT_ONLY
	_score.visible = not count_only and model.has_score()
	if model.has_scaled:
		_score.text = tr("ui.exam.result_score_scaled").format({"score": _number(model.score_scaled)})
	else:
		_score.text = tr("ui.exam.result_score").format({"score": _number(model.review.score),
			"max": _number(model.review.max_score)})
	var total: int = model.review.graded_count()
	var correct_text: String = tr("ui.exam.result_correct").format({"correct": model.review.correct_count(), "total": total})
	_correct.text = correct_text
	_correct.visible = total > 0
	_star.custom_minimum_size = Vector2.ONE * UiTokens.COMPONENT_ICON_SIZE
	_star.visible = count_only and total > 0
	if not count_only:
		for s: Dictionary in model.sections:
			_add_label(_sections, tr("ui.exam.result_section").format({"title": String(s["title"]),
				"score": _number(float(s["score"])), "max": _number(float(s["max_score"]))}))
	for i: int in model.objectives.size():
		var row: ExamResultModel.ObjectiveRow = model.objectives[i]
		_add_label(_objectives, tr("ui.exam.result_objective").format({"n": i + 1, "correct": row.correct, "total": row.total}))
	if not model.wrong.is_empty():
		_add_label(_wrong, tr("ui.exam.result_wrong_heading"))
	for w: ExamResultModel.WrongItem in model.wrong:
		_add_label(_wrong, tr("ui.exam.result_wrong_item").format({"n": w.position}))
		if w.question != null:
			_add_label(_wrong, RichText.plain_text(w.question.stem(), context))
		if w.explanation != null:
			_add_label(_wrong, RichText.plain_text(w.explanation, context))
		if not w.feedback_text.is_empty():
			_add_label(_wrong, w.feedback_text)
	for objective_id: String in model.weak_objectives():
		var b: Button = Button.new()
		b.text = "ui.exam.practice_objective"
		b.pressed.connect(func() -> void: practice_requested.emit(
				{"objective_id": objective_id, "mode": "practice", "context_type": "free_play"}))
		_practice.add_child(b)
	for rec: AttemptReviewModel.Recommendation in model.review.recommendations:
		if rec.is_practice() and not rec.title.is_empty():
			var r: Button = Button.new()
			r.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
			r.text = rec.title
			r.pressed.connect(func() -> void: practice_requested.emit(LearningService.practice_args(rec)))
			_practice.add_child(r)


func _clear() -> void:
	for box: Container in [_sections, _objectives, _wrong, _practice]:
		for child: Node in box.get_children():
			box.remove_child(child)
			child.queue_free()


func _add_label(box: Container, text: String) -> void:
	var label: Label = Label.new()
	label.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
	label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	label.text = text
	box.add_child(label)


static func _number(value: float) -> String:
	return str(int(value)) if is_equal_approx(value, roundf(value)) else "%.2f" % value
