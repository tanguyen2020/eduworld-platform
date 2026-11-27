class_name GradeGoalBar
extends PanelContainer
## "Mục tiêu của khối" (F109-AC-19; ADR-0095 mục 5, COMPETITION_SYSTEM §9.2, FR-LDB-005 — replaces ClassGoalBar): the
## collective progress of the student's grade in the school, the part the student added and, only when the server
## gives it, how many classmates of the grade helped (hidden below `analytics_min_cell_size`: no empty box, no 0). No
## personal rank, no other nickname. Also the end of a lớp 1–3 Timed Quiz (`battle.match.ended.grade_goal`).

var _title: Label = null
var _bar: ProgressBar = null
var _value: Label = null
var _mine: Label = null
var _helpers: Label = null
var _reached: Label = null


func _init() -> void:
	name = "GradeGoalBar"
	theme_type_variation = &"QuestCard"
	var box: VBoxContainer = UiBuild.vbox(&"FieldBox")
	add_child(box)
	var head: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	box.add_child(head)
	head.add_child(UiBuild.icon_rect("flag", 1.3))
	_title = UiBuild.data_label("", &"HeadingLabel")
	_title.name = "GoalTitle"
	UiBuild.expand(_title)
	head.add_child(_title)
	_reached = ArenaUi.tag("ui.leaderboard.goal.reached", &"TileTagCorrect")
	_reached.name = "Reached"
	head.add_child(_reached)
	_bar = UiBuild.progress_bar(0.0)
	_bar.name = "GoalBar"
	UiBuild.expand(_bar)
	_bar.custom_minimum_size.y = float(UiTokens.COMPONENT_PROGRESS_HEIGHT)
	box.add_child(_bar)
	_value = UiBuild.data_label("", &"InstructionLabel")
	_value.name = "GoalValue"
	box.add_child(_value)
	_mine = UiBuild.data_label("", &"HeadingLabel")
	_mine.name = "MyPart"
	box.add_child(_mine)
	_helpers = UiBuild.data_label("", &"MutedLabel")
	_helpers.name = "Helpers"
	box.add_child(_helpers)


## `goal`: `{grade_level?, current, target, reached, my_contribution (-1 none), contributors_count (-1 hidden)}`.
func show_goal(goal: Dictionary) -> void:
	var grade: int = int(goal.get("grade_level", 0))
	_title.text = TranslationServer.translate("ui.leaderboard.goal.title_grade").format({"grade": grade}) if grade > 0 \
			else TranslationServer.translate("ui.leaderboard.goal.title")
	var current: float = float(goal.get("current", 0.0))
	var target: float = maxf(0.0, float(goal.get("target", 0.0)))
	_bar.value = clampf(current / target, 0.0, 1.0) if target > 0.0 else 0.0
	_value.text = TranslationServer.translate("ui.leaderboard.goal.value").format({"current": ArenaUi.num(current),
		"target": ArenaUi.num(target)})
	var mine: float = float(goal.get("my_contribution", -1.0))
	_mine.visible = mine >= 0.0
	_mine.text = TranslationServer.translate("ui.leaderboard.goal.mine").format({"value": ArenaUi.num(mine)}) if mine >= 0.0 else ""
	var helpers: int = int(goal.get("contributors_count", -1))
	_helpers.visible = helpers > 0
	_helpers.text = TranslationServer.translate("ui.leaderboard.goal.helpers").format({"count": helpers}) if helpers > 0 else ""
	_reached.visible = bool(goal.get("reached", false))


func helpers_visible() -> bool:
	return _helpers.visible


func texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for l: Label in [_title, _value, _mine, _helpers]:
		if l.visible:
			out.append(l.text)
	return out


func bar_value() -> float:
	return _bar.value
