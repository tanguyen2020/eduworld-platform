class_name QuestDayTab
extends VBoxContainer
## "Hôm nay" / "Tuần này" of the Event center (F111-AC-15, AC-18; QUEST_SYSTEM §2.4, AC-UX-111, FR-EVT-007): the daily or
## weekly quests QuestService read with `quest_type` — name, state in words with an icon, the step progress of the
## server `QuestProgress` ("1/2 bước"; the "học 4/7 ngày" of a weekly goal is the server's step count, never counted by
## the client), the reset time from `available_until` in ICT ("Làm mới lúc 00:00, ngày 29/11/2026") and a big "Đi"
## (QuestGo → F072). Once every quest is done: Mây says "Tuyệt! Em đã xong nhiệm vụ hôm nay. Nghỉ ngơi chút nhé" and
## nothing else is suggested (no "play more", no streak — AGENTS R19). Lớp 1–3: bigger "Đi" and the quest icon as a
## picture. UI only (AGENTS R1).

signal go_requested(quest_id: String)

## quest status → icon.
const STATUS_ICONS: Dictionary = {"available": "play", "active": "clock", "completed": "check", "expired": "moon"}

var quest_type: String = "daily"
var _reset: Label = null
var _list: VBoxContainer = null
var _done: Control = null
var _none: Control = null
var _rows: Dictionary = {}
var _go: Array[Button] = []


func _init(kind: String = "daily") -> void:
	quest_type = kind
	name = "QuestDayTab_%s" % kind
	theme_type_variation = &"FormBox"
	var head: HBoxContainer = UiBuild.hbox()
	head.name = "ResetRow"
	add_child(head)
	var clock: TextureRect = UiBuild.icon_rect("clock", 0.7)
	clock.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	head.add_child(clock)
	_reset = UiBuild.keep_words(UiBuild.data_label("", &"MutedLabel"))
	_reset.name = "ResetTime"
	UiBuild.expand(_reset)
	head.add_child(_reset)
	_done = FriendsUi.empty_line("ui.events.all_done" if kind == "daily" else "ui.events.week_done", "AllDone")
	_done.visible = false
	add_child(_done)
	_none = FriendsUi.empty_line("ui.events.quests_none", "NoQuests")
	_none.visible = false
	add_child(_none)
	_list = UiBuild.vbox(&"FormBox")
	_list.name = "Quests"
	add_child(_list)


func show_quests(list: Array[QuestModel], _now_unix: int) -> void:
	UiBuild.clear(_list)
	_rows.clear()
	_go.clear()
	var until: int = 0
	var done: int = 0
	for q: QuestModel in list:
		var u: int = LiveOpsTime.unix_of(q.available_until)
		if u > 0 and (until == 0 or u < until):
			until = u
		if q.status == QuestModel.STATUS_COMPLETED:
			done += 1
	var all_done: bool = not list.is_empty() and done == list.size()
	_done.visible = all_done
	_none.visible = list.is_empty()
	_reset.text = tr("ui.events.reset_time").format({"time": LiveOpsTime.ict_time(until), "date": LiveOpsTime.ict_date(until)}) \
			if until > 0 else ""
	(_reset.get_parent() as Control).visible = until > 0
	for q: QuestModel in list:
		var row: Control = _row(q, all_done)
		_rows[q.quest_id] = row
		_list.add_child(row)
	UiBuild.size_buttons(self)
	var big: float = UiBuild.target() * (1.25 if UiBuild.band() == "g1_3" else 1.0)
	for b: Button in _go:
		b.custom_minimum_size = Vector2(maxf(b.custom_minimum_size.x, big), big)


func reset_text() -> String:
	return _reset.text


func all_done_visible() -> bool:
	return _done.visible


func all_done_text() -> String:
	var labels: Array[Node] = _done.find_children("*", "Label", true, false)
	return tr((labels[0] as Label).text) if not labels.is_empty() else ""


func go_buttons() -> Array[Button]:
	return _go.duplicate()


func row_texts(quest_id: String) -> PackedStringArray:
	var out: PackedStringArray = []
	var row: Control = _rows.get(quest_id) as Control
	if row == null:
		return out
	for node: Node in row.find_children("*", "Label", true, false):
		out.append(tr((node as Label).text))
	return out


static func steps_of(q: QuestModel) -> Vector2i:
	var total: int = maxi(q.step_states.size(), q.steps.size())
	var done: int = 0
	for st: Variant in q.step_states.values():
		if st is Dictionary and String((st as Dictionary).get("status", "")) == QuestModel.STEP_COMPLETED:
			done += 1
	return Vector2i(done, total)


func _row(q: QuestModel, all_done: bool) -> Control:
	var card: PanelContainer = UiBuild.panel(&"PartyMemberCard")
	card.name = "Quest"
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	card.add_child(row)
	var icon: TextureRect = UiBuild.icon_rect(String(STATUS_ICONS.get(q.status, "journal")), 1.0)
	icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(icon)
	var texts: VBoxContainer = UiBuild.vbox()
	UiBuild.expand(texts)
	texts.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(texts)
	var title: Label = UiBuild.keep_words(UiBuild.data_label(q.display_name(), &"HeadingLabel"))
	title.name = "QuestName"
	texts.add_child(title)
	var line: HFlowContainer = HFlowContainer.new()
	texts.add_child(line)
	var status: Label = UiBuild.label("ui.events.quest_status.%s" % q.status if q.status in STATUS_ICONS else
			"ui.events.quest_status.available", &"InstructionLabel", false)
	status.name = "QuestStatus"
	line.add_child(status)
	var steps: Vector2i = steps_of(q)
	if steps.y > 0 and q.status != QuestModel.STATUS_AVAILABLE:
		var progress: Label = UiBuild.data_label(tr("ui.events.quest_steps").format({"done": LocaleFormat.number(steps.x),
			"total": LocaleFormat.number(steps.y)}), &"SectionTag", false)
		progress.name = "QuestSteps"
		line.add_child(progress)
	if not all_done and (q.status == QuestModel.STATUS_AVAILABLE or q.status == QuestModel.STATUS_ACTIVE):
		var go: Button = UiBuild.button("ui.events.go", "arrow_right")
		go.name = "Go"
		go.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		go.pressed.connect(func() -> void: go_requested.emit(q.quest_id))
		row.add_child(go)
		_go.append(go)
	return card
