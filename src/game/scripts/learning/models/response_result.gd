class_name ResponseResultModel
extends RefCounted
## Server grading of one answer (`ActivityResponseResult`). Verdict, explanation and per-option feedback are present
## only when the attempt's FeedbackTiming allows (ACTIVITY_ENGINE §8); null fields mean "do not show right/wrong"
## (F063-AC-14). The client never computes them (ADR-0016).

## `explanation` / `AnswerFeedback.text` maxLength (specs/openapi).
const EXPLANATION_MAX_LENGTH: int = 4000
const FEEDBACK_MAX_LENGTH: int = 2000

var accepted: bool = false
var position: int = 0
## `is_correct` was sent (not null).
var has_verdict: bool = false
var is_correct: bool = false
var explanation: String = ""
var explanation_math: Array[MathAssetRefModel] = []
var feedback: AnswerFeedback = null
var hints_used: int = 0
## Segment to play after a branch choice of an interactive video (F065-AC-24), empty otherwise.
var next_segment_id: String = ""
## Mini-game attempts (F073): game points of this answer and the current correct streak, computed by the server
## (MINIGAME_ENGINE §6); `has_game_points` false when the server sent none.
var has_game_points: bool = false
var game_points: int = 0
var streak: int = 0
## F098: the right answer in the `QuestionResponse` shape of the item (`{option_id}`, `{value}`…), present only when the
## server's policy allows it after an immediate answer (contract `correct_response`); {} otherwise. Display only —
## never stored, logged or reused (AGENTS R6, R14).
var correct_response: Dictionary = {}


## Per-option feedback (`AnswerFeedback`).
class AnswerFeedback:
	extends RefCounted
	var option_id: String = ""
	var text: String = ""
	var math_assets: Array[MathAssetRefModel] = []
	## Lesson / learning material id of a worked example (F063-AC-17), empty when none.
	var worked_example_ref: String = ""

	static func from_dict(data: Dictionary) -> AnswerFeedback:
		var f: AnswerFeedback = AnswerFeedback.new()
		f.option_id = String(data.get("option_id", "")) if data.get("option_id") is String else ""
		f.text = (data.get("text") as String).left(FEEDBACK_MAX_LENGTH) if data.get("text") is String else ""
		f.math_assets = MathAssetRefModel.list_from(data.get("math_assets"))
		var ref: Variant = data.get("worked_example_ref")
		f.worked_example_ref = ref as String if ref is String and Uuid.is_valid(ref as String) else ""
		return f


static func from_dict(data: Dictionary) -> ResponseResultModel:
	var r: ResponseResultModel = ResponseResultModel.new()
	r.accepted = data.get("accepted") == true
	var pos: Variant = data.get("position")
	r.position = int(pos) if pos is int or pos is float else 0
	var verdict: Variant = data.get("is_correct")
	r.has_verdict = verdict is bool
	r.is_correct = verdict == true
	var text: Variant = data.get("explanation")
	r.explanation = (text as String).left(EXPLANATION_MAX_LENGTH) if text is String else ""
	r.explanation_math = MathAssetRefModel.list_from(data.get("explanation_math_assets"))
	var fb: Variant = data.get("feedback")
	if fb is Dictionary:
		r.feedback = AnswerFeedback.from_dict(fb as Dictionary)
	var used: Variant = data.get("hints_used")
	r.hints_used = maxi(0, int(used)) if used is int or used is float else 0
	var segment: Variant = data.get("next_segment_id")
	r.next_segment_id = (segment as String).left(64) if segment is String else ""
	var points: Variant = data.get("game_points")
	r.has_game_points = points is int or points is float
	r.game_points = maxi(0, int(points)) if r.has_game_points else 0
	var streak: Variant = data.get("streak")
	r.streak = maxi(0, int(streak)) if streak is int or streak is float else 0
	var correct: Variant = data.get("correct_response")
	r.correct_response = (correct as Dictionary).duplicate(true) if correct is Dictionary else {}
	return r


## Anything to show in the feedback strip besides "answer saved".
func has_feedback_content() -> bool:
	return has_verdict or not explanation.is_empty() or feedback != null
