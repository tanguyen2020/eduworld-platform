extends VBoxContainer
## Feedback strip of the activity player (STUDENT_GAME_UX_LEARNING §3.3, §4.8, F063-AC-14/17): after the server grades
## an answer it shows ✓/✗ + text (never colour alone, NFR-A11Y-001) in the Theme feedback panel, the feedback of the
## option the student chose, the explanation (with formulas) and "Xem ví dụ giải từng bước" when a worked example
## exists. When FeedbackTiming hides the verdict (`is_correct`/`feedback`/`explanation` null) it only says the answer
## was saved — it never shows right/wrong. Display only: nothing here is computed.

signal worked_example_requested(lesson_id: String)
signal math_failed()

const ICON_CORRECT: String = "✓"
const ICON_INCORRECT: String = "✗"

var result: ResponseResultModel = null
var worked_example_ref: String = ""

@onready var _verdict_panel: PanelContainer = %VerdictPanel
@onready var _icon: Label = %Icon
@onready var _verdict: Label = %Verdict
@onready var _details: PanelContainer = %Details
@onready var _feedback: RichTextLabel = %FeedbackText
@onready var _explanation: RichTextLabel = %Explanation
@onready var _saved: Label = %Saved
@onready var _worked: Button = %WorkedExample


func _ready() -> void:
	_worked.pressed.connect(_on_worked_pressed)
	clear()


func clear() -> void:
	result = null
	worked_example_ref = ""
	visible = false


## Coroutine: shows the server's grading of one answer.
func show_result(graded: ResponseResultModel, math: MathSvgCache) -> void:
	result = graded
	visible = true
	_verdict_panel.visible = graded.has_verdict
	if graded.has_verdict:
		_verdict_panel.theme_type_variation = &"FeedbackCorrectPanel" if graded.is_correct else &"FeedbackIncorrectPanel"
		_icon.text = ICON_CORRECT if graded.is_correct else ICON_INCORRECT
		_verdict.text = "ui.feedback.correct" if graded.is_correct else "ui.feedback.incorrect"
	var fb: ResponseResultModel.AnswerFeedback = graded.feedback
	worked_example_ref = fb.worked_example_ref if fb != null else ""
	_worked.visible = not worked_example_ref.is_empty()
	_saved.visible = not graded.has_feedback_content()
	_feedback.visible = fb != null and not fb.text.is_empty()
	_explanation.visible = not graded.explanation.is_empty()
	_details.visible = _feedback.visible or _explanation.visible or _saved.visible or _worked.visible
	if _feedback.visible:
		await _fill(_feedback, fb.text, fb.math_assets, math)
	if _explanation.visible and result == graded:
		await _fill(_explanation, graded.explanation, graded.explanation_math, math)


## Text shown for the verdict (tests and read-aloud).
func verdict_text() -> String:
	return "%s %s" % [_icon.text, tr(_verdict.text)] if _verdict_panel.visible else ""


func _fill(label: RichTextLabel, text: String, formulas: Array[MathAssetRefModel], math: MathSvgCache) -> void:
	label.clear()
	label.add_text(text)
	for formula: MathAssetRefModel in formulas:
		if formula.is_block():
			label.newline()
		var ok: bool = await MathInline.append_to(label, formula, math)
		if not ok:
			math_failed.emit()


func _on_worked_pressed() -> void:
	if not worked_example_ref.is_empty():
		worked_example_requested.emit(worked_example_ref)
