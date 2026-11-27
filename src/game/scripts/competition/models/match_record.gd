class_name MatchRecord
extends RefCounted
## One match of the student's history (`Match` of `listMyMatches` / `getMatch`; F109-AC-17, F105-AC-23). The other
## players are a nickname, an avatar and (when sent) a tier only (AGENTS R16); `rating_delta` / raw ratings are never
## copied. `review` (after the match ended) carries the questions, the right answer and the student's own verdict.

var match_id: String = ""
var mode: String = ""
var status: String = ""
var subject_id: String = ""
var ranked: bool = false
var is_bot: bool = false
var started_at: String = ""
var ended_at: String = ""
var end_reason: String = ""
var question_count: int = 0
## `{player: PublicPlayerModel, team, outcome, score (-1 none), correct_count (-1 none), competition_points (-1 none)}`.
var participants: Array[Dictionary] = []
## `{index, question (QuestionRenderModel or null), correct_response, explanation, my_answer_correct (null/bool), my_points}`.
var review: Array[Dictionary] = []


static func from_dict(data: Variant) -> MatchRecord:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var m: MatchRecord = MatchRecord.new()
	m.match_id = DataRead.str_of(d, "match_id")
	m.mode = DataRead.str_of(d, "mode")
	m.status = DataRead.str_of(d, "status")
	if not Uuid.is_valid(m.match_id) or m.mode.is_empty() or m.status.is_empty():
		return null
	m.subject_id = DataRead.str_of(d, "subject_id")
	m.ranked = DataRead.bool_of(d, "ranked")
	m.is_bot = DataRead.bool_of(d, "is_bot")
	m.started_at = DataRead.str_of(d, "started_at")
	m.ended_at = DataRead.str_of(d, "ended_at")
	m.end_reason = DataRead.str_of(d, "end_reason")
	m.question_count = maxi(0, DataRead.int_of(d, "question_count"))
	for raw: Variant in DataRead.arr_of(d, "participants"):
		if not raw is Dictionary:
			continue
		var r: Dictionary = raw as Dictionary
		var player: PublicPlayerModel = PublicPlayerModel.from_dict(r.get("player"))
		if player == null:
			continue
		m.participants.append({"player": player, "team": DataRead.int_of(r, "team_no"), "outcome": DataRead.str_of(r, "outcome"),
			"score": DataRead.nullable_int(r, "score"), "correct_count": DataRead.nullable_int(r, "correct_count"),
			"competition_points": DataRead.nullable_int(r, "competition_points")})
	for raw: Variant in DataRead.arr_of(d, "review"):
		if not raw is Dictionary:
			continue
		var r: Dictionary = raw as Dictionary
		var verdict: Variant = r.get("my_answer_correct")
		m.review.append({"index": maxi(0, DataRead.int_of(r, "index")), "stem": DataRead.dict_of(DataRead.dict_of(r, "question"), "stem"),
			"correct_response": DataRead.dict_of(r, "correct_response"), "explanation": DataRead.str_of(r, "explanation").left(2000),
			"my_answer_correct": verdict if verdict is bool else null, "my_points": DataRead.nullable_int(r, "my_points")})
	return m


static func list_from(raw: Variant) -> Array[MatchRecord]:
	var out: Array[MatchRecord] = []
	if raw is Array:
		for item: Variant in raw as Array:
			var m: MatchRecord = from_dict(item)
			if m != null:
				out.append(m)
	return out


## The row of `user_id` (the student), or {}.
func row_of(user_id: String) -> Dictionary:
	for p: Dictionary in participants:
		if (p["player"] as PublicPlayerModel).user_id == user_id:
			return p
	return {}


## The other players (nickname / avatar only) — for the detail, the safety buttons, never the student.
func others(self_id: String) -> Array[PublicPlayerModel]:
	var out: Array[PublicPlayerModel] = []
	for p: Dictionary in participants:
		var pl: PublicPlayerModel = p["player"] as PublicPlayerModel
		if pl.user_id != self_id:
			out.append(pl)
	return out


func is_void() -> bool:
	return status in ["cancelled", "aborted"]
