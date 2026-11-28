class_name PartyProgressStrip
extends VBoxContainer
## The shared progress of a party quest step on the Quest Tracker (F110-AC-12; QUEST_SYSTEM §6, PARTY_SYSTEM §7.1,
## US-PTY-003): the group share of `collective_progress` as a bar AND "75%", and whether THIS student reached their part
## (✓ "Em đã góp đủ phần" / the clock "Em góp thêm phần của mình nhé") — a frame step done for the whole party reads
## ✓ "Cả nhóm đã xong bước này". Never a count, a result or a name of another member (FR-PTY-013, R19). UI only.

var _bar: ProgressBar = null
var _pct: Label = null
var _mine_icon: TextureRect = null
var _mine: Label = null
var _bar_row: HBoxContainer = null


func _init() -> void:
	name = "PartyProgressStrip"
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	_bar_row = UiBuild.hbox()
	_bar_row.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(_bar_row)
	var group: TextureRect = UiBuild.icon_rect("group", 0.7)
	group.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_bar_row.add_child(group)
	_bar = UiBuild.progress_bar(0.0)
	_bar.name = "GroupBar"
	_bar.mouse_filter = Control.MOUSE_FILTER_IGNORE
	UiBuild.expand(_bar)
	_bar_row.add_child(_bar)
	_pct = UiBuild.data_label("", &"MutedLabel", false)
	_pct.name = "GroupPercent"
	_bar_row.add_child(_pct)
	var mine: HBoxContainer = UiBuild.hbox()
	mine.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(mine)
	_mine_icon = UiBuild.icon_rect("clock", 0.7)
	_mine_icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	mine.add_child(_mine_icon)
	_mine = UiBuild.keep_words(UiBuild.label("", &"MutedLabel"))
	_mine.name = "Mine"
	UiBuild.expand(_mine)
	mine.add_child(_mine)
	visible = false


func show_progress(p: PartyQuestProgressModel) -> void:
	visible = p != null
	if p == null:
		return
	_bar_row.visible = p.has_collective() and not p.shared_completed
	_bar.value = maxf(p.collective_progress, 0.0)
	_pct.text = tr("ui.quest.coop.group_progress").format({"pct": LocaleFormat.number(roundf(maxf(p.collective_progress, 0.0) * 100.0), "", 0)})
	if p.shared_completed:
		_mine_icon.texture = UiBuild.icon("verdict_correct")
		_mine.text = "ui.quest.coop.shared_done"
	elif p.my_contribution_met:
		_mine_icon.texture = UiBuild.icon("verdict_correct")
		_mine.text = "ui.quest.coop.my_part_done"
	else:
		_mine_icon.texture = UiBuild.icon("clock")
		_mine.text = "ui.quest.coop.my_part_needed"


func percent_text() -> String:
	return _pct.text if _bar_row.visible else ""


func mine_text() -> String:
	return _mine.text


func bar_value() -> float:
	return _bar.value
