class_name MatchSummary
extends RefCounted
## The end of a match as the server recorded it (`battle.match.ended`; F109-AC-14, AC-16, COMPETITION_SYSTEM §3–§4, §12).
## Kept: status, each result row (team, outcome, score, correct count, Competition Points), the Timed Quiz parts the
## server sends by grade (lớp 1–3: `grade_goal` — collective progress, no rank; lớp ≥ 4: `top` + `my_rank`) and the
## `encouragement` the server sends to this student only (§12, `BattleEncouragement`). A `rating_delta` / raw rating is
## never copied (FR-RNK-003).

const OUTCOMES: PackedStringArray = ["win", "loss", "draw", "abandoned", "forfeit", "no_contest"]

var match_id: String = ""
var status: String = ""
var end_reason: String = ""
var rated: bool = false
var review_available: bool = false
## `{user_id, team, outcome, score, correct_count, competition_points}` (numbers only, no names: nicknames come from
## the snapshot of the match screen).
var results: Array[Dictionary] = []
## Timed Quiz lớp ≥ 4: top 5 `{user_id, rank, score}` and the student's own place.
var top: Array[Dictionary] = []
var my_rank: int = 0
## Timed Quiz lớp 1–3: `{current, target, reached, contributors_count (-1 = hidden)}`; empty when not sent.
var grade_goal: Dictionary = {}
## `{code, message_code, subject_id, topic_id}` (COMPETITION_SYSTEM §12): an encouraging line + the "ôn lại" suggestion of
## the match's subject / topic; `topic_id` is "" when the match had no topic. Empty when not sent.
var encouragement: Dictionary = {}


static func from_dict(data: Variant) -> MatchSummary:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var s: MatchSummary = MatchSummary.new()
	s.match_id = DataRead.str_of(d, "match_id")
	s.status = DataRead.str_of(d, "status")
	if not Uuid.is_valid(s.match_id) or not s.status in ["completed", "aborted", "cancelled"] or not d.get("results") is Array:
		return null
	s.end_reason = DataRead.str_of(d, "end_reason")
	s.rated = DataRead.bool_of(d, "rated")
	s.review_available = DataRead.bool_of(d, "review_available")
	for raw: Variant in d["results"] as Array:
		if not raw is Dictionary:
			continue
		var r: Dictionary = raw as Dictionary
		var uid: String = DataRead.str_of(r, "user_id")
		if not Uuid.is_valid(uid):
			continue
		var outcome: String = DataRead.str_of(r, "outcome")
		s.results.append({"user_id": uid, "team": DataRead.int_of(r, "team"),
			"outcome": outcome if outcome in OUTCOMES else "", "score": maxi(0, DataRead.int_of(r, "score")),
			"correct_count": maxi(0, DataRead.int_of(r, "correct_count")),
			"competition_points": maxi(0, DataRead.int_of(r, "competition_points"))})
	for raw: Variant in DataRead.arr_of(d, "top"):
		if raw is Dictionary and Uuid.is_valid(DataRead.str_of(raw as Dictionary, "user_id")):
			var t: Dictionary = raw as Dictionary
			s.top.append({"user_id": DataRead.str_of(t, "user_id"), "rank": maxi(1, DataRead.int_of(t, "rank", 1)),
				"score": maxi(0, DataRead.int_of(t, "score"))})
	s.my_rank = maxi(0, DataRead.int_of(d, "my_rank"))
	var goal: Dictionary = DataRead.dict_of(d, "grade_goal")
	if not goal.is_empty():
		s.grade_goal = {"current": maxi(0, DataRead.int_of(goal, "current")), "target": maxi(0, DataRead.int_of(goal, "target")),
			"reached": DataRead.bool_of(goal, "reached"), "contributors_count": DataRead.nullable_int(goal, "contributors_count")}
	var enc: Dictionary = DataRead.dict_of(d, "encouragement")
	if not DataRead.str_of(enc, "code").is_empty():
		var subject: String = DataRead.str_of(enc, "subject_id")
		var topic: String = DataRead.str_of(enc, "topic_id")
		s.encouragement = {"code": DataRead.str_of(enc, "code").left(64), "message_code": DataRead.str_of(enc,
			"message_code").left(128), "subject_id": subject if Uuid.is_valid(subject) else "",
			"topic_id": topic if Uuid.is_valid(topic) else ""}
	return s


func result_of(user_id: String) -> Dictionary:
	for r: Dictionary in results:
		if String(r["user_id"]) == user_id:
			return r
	return {}


## "Trận đã hủy, không tính kết quả": `cancelled` / `aborted` (F109-AC-14).
func is_void() -> bool:
	return status != "completed"
