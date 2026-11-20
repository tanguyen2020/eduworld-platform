class_name AttemptReviewModel
extends RefCounted
## Review after submit (`AttemptReview`, F031/F032): score, per-item verdict and feedback (when FeedbackTiming /
## ResultRelease allow), weak objectives and Mây's practice recommendations. Shown on the result screen (F063 US4);
## nothing here is computed by the client — counts are read from the server's per-item verdicts.

const RECOMMENDATION_TITLE_MAX_LENGTH: int = 255
const KIND_QUESTION_SET: String = "question_set"

var attempt_id: String = ""
var has_score: bool = false
var score: float = 0.0
var max_score: float = 0.0
var items: Array[ReviewItem] = []
var weak_objectives: PackedStringArray = []
var recommendations: Array[Recommendation] = []


class ReviewItem:
	extends RefCounted
	var position: int = 0
	var type: String = ""
	## -1 = not released, 0 = wrong, 1 = right.
	var verdict: int = -1
	var feedback: ResponseResultModel.AnswerFeedback = null
	var hints_used: int = 0


class Recommendation:
	extends RefCounted
	var kind: String = ""
	var ref_id: String = ""
	var recommendation_id: String = ""
	var title: String = ""

	## A practice that `startAdHocAttempt` can open (question set).
	func is_practice() -> bool:
		return kind == KIND_QUESTION_SET and Uuid.is_valid(ref_id)


static func from_dict(data: Dictionary) -> AttemptReviewModel:
	var r: AttemptReviewModel = AttemptReviewModel.new()
	r.attempt_id = String(data.get("attempt_id", "")) if data.get("attempt_id") is String else ""
	var s: Variant = data.get("score")
	var m: Variant = data.get("max_score")
	r.has_score = (s is int or s is float) and (m is int or m is float)
	if r.has_score:
		r.score = float(s)
		r.max_score = float(m)
	var raw_items: Variant = data.get("items", [])
	if raw_items is Array:
		for entry: Variant in (raw_items as Array):
			if entry is Dictionary:
				r.items.append(_item(entry as Dictionary))
	var weak: Variant = data.get("weak_objectives", [])
	if weak is Array:
		for id: Variant in (weak as Array):
			if id is String and Uuid.is_valid(id as String):
				r.weak_objectives.append(id as String)
	var recs: Variant = data.get("recommendations", [])
	if recs is Array:
		for entry: Variant in (recs as Array):
			if entry is Dictionary:
				r.recommendations.append(_recommendation(entry as Dictionary))
	return r


## Items the server marked correct.
func correct_count() -> int:
	var n: int = 0
	for item: ReviewItem in items:
		if item.verdict == 1:
			n += 1
	return n


## Items with a released verdict.
func graded_count() -> int:
	var n: int = 0
	for item: ReviewItem in items:
		if item.verdict >= 0:
			n += 1
	return n


func wrong_items() -> Array[ReviewItem]:
	var out: Array[ReviewItem] = []
	for item: ReviewItem in items:
		if item.verdict == 0:
			out.append(item)
	return out


static func _item(d: Dictionary) -> ReviewItem:
	var item: ReviewItem = ReviewItem.new()
	var pos: Variant = d.get("position")
	item.position = int(pos) if pos is int or pos is float else 0
	var question: Variant = d.get("question")
	if question is Dictionary and (question as Dictionary).get("type") is String:
		item.type = (question as Dictionary)["type"] as String
	var verdict: Variant = d.get("is_correct")
	item.verdict = (1 if verdict == true else 0) if verdict is bool else -1
	var fb: Variant = d.get("feedback")
	if fb is Dictionary:
		item.feedback = ResponseResultModel.AnswerFeedback.from_dict(fb as Dictionary)
	var used: Variant = d.get("hints_used")
	item.hints_used = maxi(0, int(used)) if used is int or used is float else 0
	return item


static func _recommendation(d: Dictionary) -> Recommendation:
	var rec: Recommendation = Recommendation.new()
	rec.kind = String(d.get("kind", "")) if d.get("kind") is String else ""
	rec.ref_id = String(d.get("ref_id", "")) if d.get("ref_id") is String else ""
	var rid: Variant = d.get("recommendation_id")
	rec.recommendation_id = rid as String if rid is String and Uuid.is_valid(rid as String) else ""
	rec.title = (d.get("title") as String).left(RECOMMENDATION_TITLE_MAX_LENGTH) if d.get("title") is String else ""
	return rec
