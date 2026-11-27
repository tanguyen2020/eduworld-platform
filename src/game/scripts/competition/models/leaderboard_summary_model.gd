class_name LeaderboardSummaryModel
extends RefCounted
## One board the server lets the student see (`LeaderboardSummary` of `listLeaderboards`; F109-AC-18,
## COMPETITION_SYSTEM §9.2). The server already filtered by grade (`leaderboard_visibility`), the school policy
## (`disabled_leaderboard_definitions`) and the platform config: the client shows this list only, never adds a board.
## `leaderboard_id` / `scope_id` are sent back as given (F109-AC-21, R8).

const TITLE_MAX_LENGTH: int = 128

var leaderboard_id: String = ""
var code: String = ""
var title: String = ""
var metric: String = ""
var scope_type: String = ""
var period: String = ""
var scope_id: String = ""
var grade_level: int = 0


static func from_dict(data: Variant) -> LeaderboardSummaryModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var s: LeaderboardSummaryModel = LeaderboardSummaryModel.new()
	s.leaderboard_id = DataRead.str_of(d, "leaderboard_id")
	s.title = DataRead.str_of(d, "title").left(TITLE_MAX_LENGTH)
	s.metric = DataRead.str_of(d, "metric")
	if not Uuid.is_valid(s.leaderboard_id) or s.title.is_empty() or s.metric.is_empty():
		return null
	s.code = DataRead.str_of(d, "code").left(64)
	s.scope_type = DataRead.str_of(d, "scope_type")
	s.period = DataRead.str_of(d, "period")
	s.scope_id = DataRead.str_of(d, "scope_id")
	s.grade_level = DataRead.nullable_int(d, "grade_level")
	return s


static func list_from(raw: Variant) -> Array[LeaderboardSummaryModel]:
	var out: Array[LeaderboardSummaryModel] = []
	if raw is Array:
		for item: Variant in raw as Array:
			var s: LeaderboardSummaryModel = from_dict(item)
			if s != null:
				out.append(s)
	return out


func is_grade_goal() -> bool:
	return metric == "grade_goal"
