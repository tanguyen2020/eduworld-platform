class_name LeaderboardPageModel
extends RefCounted
## One page of a board as the server decided to show it to this student (`LeaderboardPage`; F109-AC-19…AC-21,
## COMPETITION_SYSTEM §9.4). `visibility` (the contract name of the spec's `display_mode`) drives the view:
## `grade_goal` (lớp 1–3 and every grade-goal board: collective progress of the grade, no entries, no other nickname),
## `top_n_percentile` (lớp 4–5: top 10 + "nhóm X%"), `ranked` (lớp 6–9: top + around me). The client never ranks, cuts
## or sorts the rows (R5).

const VIS_GRADE_GOAL: String = "grade_goal"
const VIS_TOP_PERCENTILE: String = "top_n_percentile"
const VIS_RANKED: String = "ranked"

var leaderboard_id: String = ""
var period_key: String = ""
var metric: String = ""
var visibility: String = ""
var stale: bool = false
var entries: Array[LeaderboardEntryModel] = []
var around_me: Array[LeaderboardEntryModel] = []
## `{rank (-1 none), value (-1 none), percentile_bucket (-1 none)}`; empty when the server sent no `me`.
var me: Dictionary = {}
## `{grade_level, current, target, reached, my_contribution (-1 none), contributors_count (-1 = hidden < 5)}`.
var grade_goal: Dictionary = {}
var next_cursor: String = ""


static func from_dict(data: Variant) -> LeaderboardPageModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var p: LeaderboardPageModel = LeaderboardPageModel.new()
	p.leaderboard_id = DataRead.str_of(d, "leaderboard_id")
	p.period_key = DataRead.str_of(d, "period_key").left(64)
	if not Uuid.is_valid(p.leaderboard_id) or p.period_key.is_empty() or not d.get("entries") is Array:
		return null
	p.metric = DataRead.str_of(d, "metric")
	p.visibility = DataRead.str_of(d, "visibility")
	if not p.visibility in [VIS_GRADE_GOAL, VIS_TOP_PERCENTILE, VIS_RANKED]:
		p.visibility = VIS_GRADE_GOAL if p.metric == "grade_goal" else VIS_RANKED
	p.stale = DataRead.bool_of(d, "stale")
	# A grade-goal board never lists students, whatever a server bug sends (ADR-0095, FR-LDB-005).
	if p.visibility != VIS_GRADE_GOAL:
		p.entries = LeaderboardEntryModel.list_from(d["entries"])
		p.around_me = LeaderboardEntryModel.list_from(d.get("around_me"))
	var me_raw: Dictionary = DataRead.dict_of(d, "me")
	if not me_raw.is_empty():
		p.me = {"rank": DataRead.nullable_int(me_raw, "rank"), "value": DataRead.num_of(me_raw, "value", -1.0),
			"percentile_bucket": DataRead.nullable_int(me_raw, "percentile_bucket")}
	var goal: Dictionary = DataRead.dict_of(d, "grade_goal")
	if not goal.is_empty() and (goal.get("current") is int or goal.get("current") is float):
		p.grade_goal = {"grade_level": DataRead.int_of(goal, "grade_level"), "current": DataRead.num_of(goal, "current"),
			"target": DataRead.num_of(goal, "target"), "reached": DataRead.bool_of(goal, "reached"),
			"my_contribution": DataRead.num_of(goal, "my_contribution", -1.0) if (goal.get("my_contribution") is int
				or goal.get("my_contribution") is float) else -1.0,
			"contributors_count": DataRead.nullable_int(goal, "contributors_count")}
	p.next_cursor = GameApiService.next_cursor(d)
	return p


## The student's row when the server put one in `entries` / `around_me` (`is_me`).
func my_entry() -> LeaderboardEntryModel:
	for e: LeaderboardEntryModel in entries + around_me:
		if e.is_me:
			return e
	return null


## Rows of `around_me` the top list does not show already (no duplicate of a rank).
func extra_around() -> Array[LeaderboardEntryModel]:
	var shown: Dictionary = {}
	for e: LeaderboardEntryModel in entries:
		shown[e.rank] = true
	var out: Array[LeaderboardEntryModel] = []
	for e: LeaderboardEntryModel in around_me:
		if not shown.has(e.rank):
			out.append(e)
	return out
