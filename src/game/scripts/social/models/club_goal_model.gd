class_name ClubGoalModel
extends RefCounted
## A weekly goal of the whole club (OpenAPI `ClubGoal`; F111-AC-20): the group progress against the target and the
## student's OWN contribution ("Em đã góp {n}") — never another member's share (AGENTS R16, R19).

var goal_id: String = ""
var title: String = ""
var metric: String = ""
var target: int = 1
var progress: int = 0
var my_contribution: int = 0
var week_start: String = ""
var status: String = "active"


static func from_dict(data: Variant) -> ClubGoalModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var id: String = DataRead.str_of(d, "goal_id")
	if not Uuid.is_valid(id):
		return null
	var g: ClubGoalModel = ClubGoalModel.new()
	g.goal_id = id
	g.title = DataRead.str_of(d, "title").left(128)
	g.metric = DataRead.str_of(d, "metric").left(64)
	g.target = maxi(1, DataRead.int_of(d, "target", 1))
	g.progress = maxi(0, DataRead.int_of(d, "progress"))
	g.my_contribution = maxi(0, DataRead.int_of(d, "my_contribution"))
	g.week_start = DataRead.str_of(d, "week_start").left(10)
	g.status = DataRead.str_of(d, "status", "active").left(16)
	return g


static func list_from(data: Variant) -> Array[ClubGoalModel]:
	var out: Array[ClubGoalModel] = []
	if data is Array:
		for raw: Variant in data as Array:
			var g: ClubGoalModel = from_dict(raw)
			if g != null:
				out.append(g)
	return out


func ratio() -> float:
	return clampf(float(progress) / float(maxi(1, target)), 0.0, 1.0)


## Title of the goal, or the words of its metric when the club gave none.
func display_title() -> String:
	return title if not title.is_empty() else TranslationServer.translate("ui.club.goal.metric.%s" % metric)
