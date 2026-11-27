class_name AnswerEvaluation
extends RefCounted
## The server's verdict on the student's answer of one question (`battle.answer.evaluated`, sent only after the
## question closed; F109-AC-08, ADR-0016). `time_bonus` > 0 is the only case the screen mentions speed.

var question_instance_id: String = ""
var answered: bool = false
var is_correct: bool = false
var points: int = 0
var time_bonus: int = 0
var correct_response: Dictionary = {}
var explanation: String = ""


static func from_dict(data: Variant) -> AnswerEvaluation:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	if not d.get("is_correct") is bool:
		return null
	var e: AnswerEvaluation = AnswerEvaluation.new()
	e.question_instance_id = DataRead.str_of(d, "question_instance_id")
	e.answered = DataRead.bool_of(d, "answered", true)
	e.is_correct = bool(d["is_correct"])
	e.points = maxi(0, DataRead.int_of(d, "points"))
	e.time_bonus = maxi(0, DataRead.int_of(d, "time_bonus"))
	e.correct_response = DataRead.dict_of(d, "correct_response").duplicate(true)
	e.explanation = DataRead.str_of(DataRead.dict_of(d, "explanation"), "text").left(2000)
	return e


## The verdict as the F063 renderers draw it on the tiles (✓ / ✗ with words, right answer).
func to_result() -> ResponseResultModel:
	var r: ResponseResultModel = ResponseResultModel.new()
	r.accepted = true
	r.has_verdict = answered
	r.is_correct = is_correct
	r.correct_response = correct_response
	r.explanation = explanation
	return r
