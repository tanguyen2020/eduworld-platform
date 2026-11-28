class_name CommunityGoalBar
extends VBoxContainer
## The shared goal of an event (F111-AC-13; EVENT_SEASON_SYSTEM §6, ADR-0095, AGENTS R19): the title by scope ("Mục tiêu
## cả trường"…), the bar total / target, the 25/50/75/100 % marks (✓ icon + "%", never colour alone) and "Em đã góp sức"
## (or the neutral way to help). There is NO personal ranking anywhere. The per-grade rows — only those the server sent
## (grades with fewer than 5 contributing students are left out by the server and never guessed) — are sorted by grade,
## without names or ranks, and not shown to lớp 1–3. Values replace on each update (no animation, no blinking). UI only.

var _title: Label = null
var _bar: ProgressBar = null
var _value: Label = null
var _marks: HBoxContainer = null
var _mine: Label = null
var _mine_icon: TextureRect = null
var _grades_title: Label = null
var _grades: VBoxContainer = null
var _reached: Dictionary = {}


func _init() -> void:
	name = "CommunityGoalBar"
	theme_type_variation = &"FormBox"
	var head: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	add_child(head)
	var icon: TextureRect = UiBuild.icon_rect("group", 0.9)
	icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	head.add_child(icon)
	_title = UiBuild.keep_words(UiBuild.label("", &"HeadingLabel"))
	_title.name = "GoalTitle"
	UiBuild.expand(_title)
	head.add_child(_title)
	_value = UiBuild.data_label("", &"SectionTag", false)
	_value.name = "GoalValue"
	_value.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	head.add_child(_value)
	_bar = UiBuild.progress_bar()
	_bar.name = "GoalProgress"
	UiBuild.expand(_bar)
	add_child(_bar)
	_marks = UiBuild.hbox()
	_marks.name = "Marks"
	add_child(_marks)
	for mark: int in CommunityGoalModel.MARKS:
		var m: HBoxContainer = UiBuild.hbox()
		m.name = "Mark%d" % mark
		UiBuild.expand(m)
		m.alignment = BoxContainer.ALIGNMENT_END
		var mi: TextureRect = UiBuild.icon_rect("checkbox_off", 0.6)
		mi.name = "MarkIcon"
		mi.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		m.add_child(mi)
		var ml: Label = UiBuild.data_label("", &"MutedLabel", false)
		ml.name = "MarkLabel"
		m.add_child(ml)
		_marks.add_child(m)
	var mine: HBoxContainer = UiBuild.hbox()
	add_child(mine)
	_mine_icon = UiBuild.icon_rect("heart", 0.7)
	_mine_icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	mine.add_child(_mine_icon)
	_mine = UiBuild.keep_words(UiBuild.label("", &"InstructionLabel"))
	_mine.name = "Contributed"
	UiBuild.expand(_mine)
	mine.add_child(_mine)
	_grades_title = UiBuild.label("ui.events.by_grade", &"MutedLabel")
	_grades_title.name = "GradesTitle"
	add_child(_grades_title)
	_grades = UiBuild.vbox()
	_grades.name = "Grades"
	add_child(_grades)


func show_goal(g: CommunityGoalModel, band: String) -> void:
	_title.text = "ui.events.community_goal.%s" % (g.scope if g.scope in ["school", "tenant", "platform"] else "school")
	_bar.value = g.ratio()
	_value.text = tr("ui.events.goal_progress").format({"progress": LocaleFormat.number(g.progress),
		"target": LocaleFormat.number(g.target)})
	_reached.clear()
	for m: Node in _marks.get_children():
		var mark: int = String(m.name).trim_prefix("Mark").to_int()
		var hit: bool = g.reached(mark)
		_reached[mark] = hit
		(m.get_node("MarkIcon") as TextureRect).texture = UiBuild.icon("checkbox_on" if hit else "checkbox_off")
		(m.get_node("MarkLabel") as Label).text = tr("ui.events.goal_mark").format({"pct": mark})
	_mine.text = "ui.events.contributed" if g.my_contributed else "ui.events.not_contributed"
	_mine_icon.texture = UiBuild.icon("heart" if g.my_contributed else "star")
	UiBuild.clear(_grades)
	var rows: Array[Dictionary] = g.grade_rows(band)
	for r: Dictionary in rows:
		var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
		row.name = "Grade%d" % int(r["grade_level"])
		var l: Label = UiBuild.data_label(tr("ui.events.grade").format({"grade": LocaleFormat.number(int(r["grade_level"]))}),
				&"InstructionLabel", false)
		l.name = "GradeLabel"
		l.custom_minimum_size.x = UiBuild.target() * 1.6
		row.add_child(l)
		var bar: ProgressBar = UiBuild.progress_bar(clampf(float(int(r["progress"])) / float(maxi(1, g.target)), 0.0, 1.0))
		UiBuild.expand(bar)
		row.add_child(bar)
		_grades.add_child(row)
	_grades_title.visible = not rows.is_empty()
	_grades.visible = not rows.is_empty()


func mark_reached(mark: int) -> bool:
	return bool(_reached.get(mark, false))


## Words of the per-grade rows in order ("Khối 4", "Khối 7").
func grade_texts() -> PackedStringArray:
	var out: PackedStringArray = []
	if not _grades.visible:
		return out
	for row: Node in _grades.get_children():
		if row.is_queued_for_deletion():
			continue
		out.append((row.get_node("GradeLabel") as Label).text)
	return out


func texts() -> PackedStringArray:
	var out: PackedStringArray = [tr(_title.text), _value.text, tr(_mine.text)]
	out.append_array(grade_texts())
	return out
