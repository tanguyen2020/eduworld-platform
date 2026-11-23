class_name MiniGameQuestionScene
extends MiniGameBase
## Shared look of the mini-game scenes (F073 US5/US6): a stage panel (restyled by the variant), the item drawn by the
## F063 question scene of its type (QuestionRendererRegistry — the same answer models, so the response is exactly the
## `QuestionResponse` of the type), a "Xong" button, an effect strip (✓/✗ icon + text + the server's points) and the
## end summary. `auto_commit`: a single-choice / single true-false item is sent as soon as it is tapped (balloon, gate,
## sign), the other types wait for "Xong" / Enter. An item whose type the mini-game does not accept, or that this client
## cannot draw, shows "update required" and the host lets the student skip it. Display only (AGENTS R1).

const UNSUPPORTED_KEY: String = "ui.activity_player.update_required"
const AUTO_TYPES: PackedStringArray = ["single_choice", "true_false"]
const HOURGLASS: String = "⏳"
const BAND_NO_CLOCK: String = "g1_3"

## Tap = answer for single-choice / single true-false items.
@export var auto_commit: bool = false

var question: QuestionRendererBase = null
var unsupported: bool = false

@onready var _stage: PanelContainer = %Stage
@onready var _content: VBoxContainer = %Content
@onready var _done: Button = %Done
@onready var _effect: PanelContainer = %Effect
@onready var _effect_icon: Label = %EffectIcon
@onready var _effect_text: Label = %EffectText
@onready var _effect_points: Label = %EffectPoints
@onready var _explanation: Label = %Explanation
@onready var _timer: Label = %Timer
@onready var _summary: VBoxContainer = %Summary
@onready var _summary_score: Label = %SummaryScore
@onready var _summary_completion: Label = %SummaryCompletion
@onready var _summary_marks: Label = %SummaryMarks


func _ready() -> void:
	_done.pressed.connect(func() -> void: press_primary())
	_effect.visible = false
	_summary.visible = false
	_timer.visible = false
	_done.disabled = true


func _apply_variant(style: String) -> void:
	_stage.theme_type_variation = StringName(style)


func _on_present(next: QuestionRenderModel) -> void:
	_clear_content()
	_effect.visible = false
	_stage.visible = true
	_summary.visible = false
	question = null
	unsupported = not MiniGameRegistry.accepts(minigame_type, next.type)
	if not unsupported:
		question = QuestionRendererRegistry.instantiate(next.type)
		unsupported = question == null
	if unsupported:
		var notice: Label = Label.new()
		notice.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		notice.text = UNSUPPORTED_KEY
		_content.add_child(notice)
		_done.visible = false
		return
	_done.visible = not (auto_commit and _auto_type(next))
	_content.add_child(question)
	question.setup(next, context)
	question.answer_changed.connect(_on_answer_changed)
	question.response_prepared.connect(_on_question_prepared)
	_done.disabled = true


func press_primary() -> bool:
	if locked or question == null:
		return false
	return question.commit()


func select_index(index: int) -> void:
	if not locked and question != null:
		question.select_index(index)


func set_locked(value: bool) -> void:
	super.set_locked(value)
	if question != null:
		question.set_locked(value)
	_done.disabled = value or question == null or not question.is_complete()


func is_answer_complete() -> bool:
	return question != null and not locked and question.is_complete()


func current_item_unsupported() -> bool:
	return unsupported


func _on_answer_changed(complete: bool) -> void:
	_done.disabled = locked or not complete
	apply_touch_targets(_content)
	if complete and auto_commit and item != null and _auto_type(item):
		press_primary()


func _on_question_prepared(position: int, response: Dictionary) -> void:
	set_locked(true)
	response_prepared.emit(str(position), response)


func _show_effect(kind: String, result: ResponseResultModel) -> void:
	_effect.visible = true
	match kind:
		MiniGameFeedbackSfx.KIND_CORRECT:
			_effect.theme_type_variation = &"FeedbackCorrectPanel"
			_effect_text.text = "ui.feedback.correct"
		MiniGameFeedbackSfx.KIND_INCORRECT:
			_effect.theme_type_variation = &"FeedbackIncorrectPanel"
			_effect_text.text = "ui.feedback.incorrect"
		_:
			_effect.theme_type_variation = &""
			_effect_text.text = "ui.feedback.saved"
	_effect_icon.text = MiniGameBase.icon_of(kind)
	_effect_points.text = ("+%d" % result.game_points) if result.has_game_points and result.game_points > 0 else ""
	_explanation.text = result.explanation
	_explanation.visible = not result.explanation.is_empty()


func _on_deferred(_result: ResponseResultModel) -> void:
	_effect.visible = true
	_effect.theme_type_variation = &""
	_effect_icon.text = MiniGameBase.ICON_SAVED
	_effect_text.text = "ui.feedback.saved"
	_effect_points.text = ""
	_explanation.visible = false


func _on_timer(ms: int) -> void:
	_timer.visible = ms >= 0
	if ms < 0:
		return
	var left: int = int(ceil(ms / 1000.0))
	_timer.text = HOURGLASS if context != null and context.grade_band == BAND_NO_CLOCK \
			else "%s %02d:%02d" % [HOURGLASS, left / 60, left % 60]


func _on_finish(summary: MiniGameSummaryModel, marks: Array[ResponseResultModel]) -> void:
	_clear_content()
	question = null
	_stage.visible = false
	_done.visible = false
	_effect.visible = false
	_summary.visible = true
	_summary_score.visible = summary != null and summary.has_game_score
	if _summary_score.visible:
		_summary_score.text = tr("ui.minigame.game_score").format({"points": summary.game_score})
	_summary_completion.visible = summary != null and summary.has_completion
	if _summary_completion.visible:
		_summary_completion.text = "%s %s" % [MiniGameBase.ICON_CORRECT if summary.completion_met else MiniGameBase.ICON_SAVED,
				tr("ui.minigame.completed" if summary.completion_met else "ui.minigame.not_completed")]
	var parts: PackedStringArray = []
	for r: ResponseResultModel in marks:
		parts.append("%d %s" % [r.position, MiniGameBase.icon_of(MiniGameBase.kind_of(r))])
	_summary_marks.text = "  ".join(parts)
	_summary_marks.visible = not parts.is_empty()


func _on_teardown() -> void:
	_clear_content()
	question = null


func _clear_content() -> void:
	if _content == null:
		return
	for child: Node in _content.get_children():
		_content.remove_child(child)
		child.queue_free()


func _auto_type(q: QuestionRenderModel) -> bool:
	if not q.type in AUTO_TYPES:
		return false
	return q.type != "true_false" or not q.render_payload.has("statements")
