class_name AnswerRevealOverlay
extends PanelContainer
## The server's verdict on the student's answer (F109-AC-08; STUDENT_GAME_UX_SOCIAL §8 `AnswerRevealOverlay`): the
## F097 feedback strip — light panel with a border, the ✓ / ✗ picture **and** the words "Đúng rồi!" / "Chưa đúng lần
## này" (never colour alone), the question points, and "Thưởng tốc độ +50" only when the server sent `time_bonus > 0`
## (never a speed hint otherwise, AC-09). A short explanation when the server gives one. Shown only after
## `battle.answer.evaluated` (ADR-0016).

var _icon: TextureRect = null
var _title: Label = null
var _points: Label = null
var _bonus: Label = null
var _explanation: Label = null


func _init() -> void:
	name = "AnswerReveal"
	visible = false
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	add_child(row)
	_icon = UiBuild.icon_rect("verdict_correct", 1.4)
	_icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(_icon)
	var texts: VBoxContainer = UiBuild.vbox()
	UiBuild.expand(texts)
	row.add_child(texts)
	# Verdict, points and the speed bonus share one flowing line, so the strip stays short on a phone (844×390)
	# and the answers above it keep their room; a long locale flows onto a second line.
	var line: HFlowContainer = HFlowContainer.new()
	texts.add_child(line)
	_title = UiBuild.label("", &"HeadingLabel", false)
	_title.name = "Verdict"
	line.add_child(_title)
	_points = UiBuild.data_label("", &"HeadingLabel", false)
	_points.name = "Points"
	line.add_child(_points)
	_bonus = UiBuild.data_label("", &"MutedLabel", false)
	_bonus.name = "SpeedBonus"
	_bonus.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	line.add_child(_bonus)
	_explanation = UiBuild.data_label("", &"MutedLabel")
	_explanation.name = "Explanation"
	texts.add_child(_explanation)


func show_evaluation(e: AnswerEvaluation) -> void:
	if e == null:
		hide()
		return
	var right: bool = e.answered and e.is_correct
	theme_type_variation = &"FeedbackCorrectPanel" if right else &"FeedbackIncorrectPanel"
	_icon.texture = UiBuild.icon("verdict_correct" if right else "verdict_try")
	if not e.answered:
		_title.text = "ui.arena.reveal.no_answer"
	else:
		_title.text = "ui.arena.reveal.correct" if right else "ui.arena.reveal.try"
	_points.text = TranslationServer.translate("ui.arena.reveal.points").format({"points": ArenaUi.num(float(e.points))})
	_points.visible = e.answered
	_bonus.visible = e.time_bonus > 0
	_bonus.text = TranslationServer.translate("ui.arena.reveal.speed_bonus").format({"bonus": ArenaUi.num(float(e.time_bonus))}) \
			if e.time_bonus > 0 else ""
	_explanation.text = e.explanation
	_explanation.visible = not e.explanation.is_empty()
	show()


func verdict_text() -> String:
	return tr(_title.text) if visible else ""


func bonus_visible() -> bool:
	return visible and _bonus.visible


func texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for l: Label in [_title, _points, _bonus, _explanation]:
		if visible and l.visible:
			out.append(tr(l.text) if l == _title else l.text)
	return out
