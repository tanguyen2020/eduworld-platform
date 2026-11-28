class_name CommunityGoalModel
extends RefCounted
## The shared goal of an event (OpenAPI `CommunityGoalProgress`, AsyncAPI `event.community_goal.updated`; F111-AC-13,
## EVENT_SEASON_SYSTEM §6, ADR-0095, AGENTS R19): total / target, the 25/50/75/100 % milestones reached and "Em đã góp
## sức" (`my_contributed`). There is NO personal ranking. `by_grade` (only when the event shows grade contributions) is
## kept sorted by grade, without names or ranks, and never shown to lớp 1–3; a grade the server left out (fewer than 5
## contributing students) is never guessed.

const MARKS: PackedInt32Array = [25, 50, 75, 100]

var goal_id: String = ""
var event_run_id: String = ""
var metric: String = ""
var scope: String = ""
var target: int = 1
var progress: int = 0
var milestones_reached: PackedInt32Array = []
var my_contributed: bool = false
var by_grade: Array[Dictionary] = []
var updated_at: String = ""


static func from_dict(data: Variant) -> CommunityGoalModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var id: String = DataRead.str_of(d, "goal_id")
	if not Uuid.is_valid(id):
		return null
	var g: CommunityGoalModel = CommunityGoalModel.new()
	g.goal_id = id
	g.event_run_id = DataRead.str_of(d, "event_run_id").left(36)
	g.metric = DataRead.str_of(d, "metric").left(32)
	g.scope = DataRead.str_of(d, "scope").left(16)
	g.target = maxi(1, DataRead.int_of(d, "target", 1))
	g.progress = maxi(0, DataRead.int_of(d, "progress"))
	for v: Variant in DataRead.arr_of(d, "milestones_reached"):
		if (v is int or v is float) and int(v) in MARKS and not int(v) in g.milestones_reached:
			g.milestones_reached.append(int(v))
	g.my_contributed = DataRead.bool_of(d, "my_contributed")
	var rows: Array[Dictionary] = []
	for v: Variant in DataRead.arr_of(d, "by_grade"):
		if not v is Dictionary:
			continue
		var grade: int = DataRead.int_of(v as Dictionary, "grade_level")
		if grade >= 1 and grade <= 9 and rows.size() < 9:
			rows.append({"grade_level": grade, "progress": maxi(0, DataRead.int_of(v as Dictionary, "progress"))})
	rows.sort_custom(func(a: Dictionary, b: Dictionary) -> bool: return int(a["grade_level"]) < int(b["grade_level"]))
	g.by_grade = rows
	g.updated_at = DataRead.str_of(d, "updated_at").left(32)
	return g


## `event.community_goal.updated`: the server values replace the bar (never added up).
func apply_update(payload: Dictionary) -> void:
	target = maxi(1, DataRead.int_of(payload, "target", target))
	progress = maxi(0, DataRead.int_of(payload, "progress", progress))
	var mark: int = DataRead.int_of(payload, "milestone_reached", -1)
	if mark in MARKS and not mark in milestones_reached:
		milestones_reached.append(mark)


func ratio() -> float:
	return clampf(float(progress) / float(maxi(1, target)), 0.0, 1.0)


## Per-grade rows to show for a viewer of `band` (none for lớp 1–3).
func grade_rows(band: String) -> Array[Dictionary]:
	if band == "g1_3":
		return [] as Array[Dictionary]
	return by_grade.duplicate()


func reached(mark: int) -> bool:
	return mark in milestones_reached or ratio() * 100.0 >= float(mark)
