class_name ExamResultModel
extends RefCounted
## Result of an exam attempt once released (`AttemptReview` of `getExamResult`, F042; F064-AC-17…19). Nothing is
## computed by the client (ADR-0016): the score is the server's (`score_scaled` on the school scale when sent, else
## `score`/`max_score`), the per-objective rows only tally the server's per-item verdicts, and the wrong items carry
## the explanation and option feedback only when the server sent them (`show_explanations`/`show_correct_answers`).
## No ranking and no other student (AGENTS R16).

## The underlying review (score, items with verdicts, weak objectives, recommendations).
var review: AttemptReviewModel = null
var attempt_id: String = ""
var has_scaled: bool = false
var score_scaled: float = 0.0
## `[{title: String, score: float, max_score: float}]` (`section_scores`).
var sections: Array[Dictionary] = []
var wrong: Array[WrongItem] = []
var objectives: Array[ObjectiveRow] = []


class WrongItem:
	extends RefCounted
	var position: int = 0
	var question: QuestionRenderModel = null
	## Rich content (`{blocks}`) or null when the session hides explanations.
	var explanation: Variant = null
	## Feedback of the chosen option, empty when not sent.
	var feedback_text: String = ""


class ObjectiveRow:
	extends RefCounted
	var objective_id: String = ""
	var correct: int = 0
	var total: int = 0


static func from_dict(data: Dictionary) -> ExamResultModel:
	var r: ExamResultModel = ExamResultModel.new()
	r.review = AttemptReviewModel.from_dict(data)
	r.attempt_id = r.review.attempt_id
	var scaled: Variant = data.get("score_scaled")
	r.has_scaled = scaled is int or scaled is float
	r.score_scaled = float(scaled) if r.has_scaled else 0.0
	var secs: Variant = data.get("section_scores", [])
	if secs is Array:
		for s: Variant in (secs as Array):
			if s is Dictionary and _is_number((s as Dictionary).get("score")) and _is_number((s as Dictionary).get("max_score")):
				var d: Dictionary = s as Dictionary
				r.sections.append({"title": String(d.get("title", "")) if d.get("title") is String else "",
					"score": float(d["score"]), "max_score": float(d["max_score"])})
	var by_objective: Dictionary = {}
	var order: Array[String] = []
	var items: Variant = data.get("items", [])
	if items is Array:
		for entry: Variant in (items as Array):
			if not entry is Dictionary:
				continue
			var d: Dictionary = entry as Dictionary
			var verdict: Variant = d.get("is_correct")
			if not verdict is bool:
				continue
			var ids: Variant = d.get("objective_ids", [])
			if ids is Array:
				for id: Variant in (ids as Array):
					if not (id is String and Uuid.is_valid(id as String)):
						continue
					if not by_objective.has(id):
						var row: ObjectiveRow = ObjectiveRow.new()
						row.objective_id = id as String
						by_objective[id] = row
						order.append(id as String)
					var o: ObjectiveRow = by_objective[id] as ObjectiveRow
					o.total += 1
					o.correct += 1 if verdict == true else 0
			if verdict == false:
				r.wrong.append(_wrong(d))
	for id: String in order:
		r.objectives.append(by_objective[id] as ObjectiveRow)
	return r


func has_score() -> bool:
	return has_scaled or review.has_score


## Objectives to practise: the server's weak objectives, in its order.
func weak_objectives() -> PackedStringArray:
	return review.weak_objectives


static func _wrong(d: Dictionary) -> WrongItem:
	var w: WrongItem = WrongItem.new()
	var pos: Variant = d.get("position")
	w.position = int(pos) if pos is int or pos is float else 0
	var q: Variant = d.get("question")
	w.question = QuestionRenderModel.from_dict(q as Dictionary) if q is Dictionary else null
	var exp: Variant = d.get("explanation")
	w.explanation = exp if exp is Dictionary else null
	var fb: Variant = d.get("feedback")
	if fb is Dictionary and (fb as Dictionary).get("text") is String:
		w.feedback_text = (fb as Dictionary)["text"] as String
	return w


static func _is_number(v: Variant) -> bool:
	return v is int or v is float
