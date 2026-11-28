class_name EventDetailPanel
extends VBoxContainer
## The detail of one live-ops event (F111-AC-10…14; EVENT_SEASON_SYSTEM §3–§6, §9): name, time left in days, description,
## the event points and the points still possible today in neutral words, "Tham gia" (live and not joined yet), the
## event quests with a big "Đi" (QuestService leads the way, F072), the milestones ("Nhận" on a reached one, "Đã nhận",
## "Chưa đạt", a static "Mới đạt!" badge — never a blocking popup) and the CommunityGoalBar. A paused event hides its
## parts behind "Sự kiện tạm dừng"; an ended one lets only reached milestones be claimed until `claim_deadline_at` and says
## "Phần thưởng chưa nhận sẽ được gửi tự động" (F111-AC-12). No countdown, no streak, no "chỉ còn…" (AGENTS R19). UI
## only: the Event center calls LiveEventService on the signals (AGENTS R1).

signal back_requested()
signal join_requested(event_id: String)
signal claim_requested(event_id: String, milestone_id: String)
signal go_requested(quest_id: String)

var event_id: String = ""
var _back: Button = null
var _name: Label = null
var _time_left: Label = null
var _paused: PanelContainer = null
var _ended: PanelContainer = null
var _ended_label: Label = null
var _description: Label = null
var _points: Label = null
var _daily: Label = null
var _join: Button = null
var _quests_box: VBoxContainer = null
var _quests: VBoxContainer = null
var _milestones_box: VBoxContainer = null
var _milestones: VBoxContainer = null
var _goal: CommunityGoalBar = null
var _claims: Dictionary = {}
var _busy: bool = false
var _titles: VBoxContainer = null
var _title_line: HFlowContainer = null
var _time_row: HBoxContainer = null
var _points_row: HFlowContainer = null


func _init() -> void:
	name = "EventDetail"
	theme_type_variation = &"FormBox"
	var head: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	head.name = "Head"
	add_child(head)
	_back = UiBuild.button("ui.events.back", "arrow_left", &"SecondaryButton")
	_back.name = "Back"
	_back.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_back.pressed.connect(func() -> void: back_requested.emit())
	head.add_child(_back)
	var titles: VBoxContainer = UiBuild.vbox()
	UiBuild.expand(titles)
	titles.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	head.add_child(titles)
	_titles = titles
	# F111 compact polish: on a compact phone the name, the time left and the points share this one line.
	_title_line = HFlowContainer.new()
	_title_line.name = "TitleLine"
	_title_line.visible = false
	titles.add_child(_title_line)
	_name = UiBuild.keep_words(UiBuild.data_label("", &"HeadingLabel"))
	_name.name = "EventName"
	_name.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	titles.add_child(_name)
	var tl: HBoxContainer = UiBuild.hbox()
	tl.name = "TimeRow"
	tl.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	titles.add_child(tl)
	_time_row = tl
	var clock: TextureRect = UiBuild.icon_rect("clock", 0.7)
	clock.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	tl.add_child(clock)
	_time_left = UiBuild.data_label("", &"InstructionLabel", false)
	_time_left.name = "TimeLeft"
	tl.add_child(_time_left)
	_paused = UiBuild.panel(&"WarningBannerPanel")
	_paused.name = "Paused"
	_paused.add_child(UiBuild.keep_words(UiBuild.label("ui.events.suspended", &"WarningBannerLabel")))
	add_child(_paused)
	_ended = UiBuild.panel(&"InfoBannerPanel")
	_ended.name = "EndedNote"
	_ended_label = UiBuild.keep_words(UiBuild.data_label("", &"InfoBannerLabel"))
	_ended.add_child(_ended_label)
	add_child(_ended)
	_description = UiBuild.keep_words(UiBuild.data_label("", &"InstructionLabel"))
	_description.name = "Description"
	add_child(_description)
	var points: HFlowContainer = HFlowContainer.new()
	points.name = "PointsRow"
	add_child(points)
	_points_row = points
	_points = UiBuild.data_label("", &"SectionTag", false)
	_points.name = "Points"
	_points.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	points.add_child(_points)
	_daily = UiBuild.keep_words(UiBuild.data_label("", &"MutedLabel"))
	_daily.name = "DailyLeft"
	# Its own line under the points tag: in a flow a wrapping label gets no width and cuts its words.
	add_child(_daily)
	_join = UiBuild.button("ui.events.join", "play_light")
	_join.name = "Join"
	_join.size_flags_horizontal = Control.SIZE_SHRINK_BEGIN
	_join.pressed.connect(func() -> void: join_requested.emit(event_id))
	add_child(_join)
	_quests_box = _section("Quests", "ui.events.quests", "journal")
	_quests = _quests_box.get_node("Rows") as VBoxContainer
	_milestones_box = _section("Milestones", "ui.events.milestones", "gift")
	_milestones = _milestones_box.get_node("Rows") as VBoxContainer
	_goal = CommunityGoalBar.new()
	add_child(_goal)


## `quests`: quest_id → QuestModel known to QuestService (names); an unknown quest reads "Nhiệm vụ sự kiện".
func show_event(e: LiveEventModel, now_unix: int, band: String, quests: Dictionary) -> void:
	event_id = e.event_id
	_name.text = e.display_name()
	_time_left.text = e.time_left_text(now_unix)
	var paused: bool = e.is_suspended()
	var ended: bool = e.is_ended()
	var claims: bool = e.claims_open(now_unix)
	_paused.visible = paused
	_ended.visible = ended
	if ended:
		var note: String = tr("ui.events.ended_auto_grant")
		if claims and not e.claim_deadline_at.is_empty():
			note += "\n" + tr("ui.events.claim_until").format({"date": LiveOpsTime.ict_date(LiveOpsTime.unix_of(e.claim_deadline_at))})
		_ended_label.text = note
	_description.text = e.display_description()
	_description.visible = not paused and not _description.text.is_empty()
	_points.text = tr("ui.events.points").format({"points": LocaleFormat.number(e.event_points)})
	_points.visible = e.joined and not paused
	if e.daily_points_remaining > 0:
		_daily.text = tr("ui.events.daily_remaining").format({"points": LocaleFormat.number(e.daily_points_remaining)})
	elif e.daily_points_remaining == 0:
		_daily.text = tr("ui.events.daily_full")
	else:
		_daily.text = ""
	_daily.visible = e.joined and e.is_live() and not _daily.text.is_empty()
	_join.visible = e.is_live() and not e.joined
	_join.disabled = _busy
	# Quests: only while the event is live (a paused / ended event has nothing to go to).
	UiBuild.clear(_quests)
	for qid: String in e.quest_ids:
		_quests.add_child(_quest_row(qid, quests.get(qid) as QuestModel))
	_quests_box.visible = e.is_live() and not e.quest_ids.is_empty()
	UiBuild.clear(_milestones)
	_claims.clear()
	for m: EventMilestoneModel in e.milestones:
		_milestones.add_child(_milestone_row(m, claims and not paused))
	_milestones_box.visible = not paused and not e.milestones.is_empty()
	_goal.visible = e.community_goal != null and not paused
	if e.community_goal != null:
		_goal.show_goal(e.community_goal, band)
	UiBuild.size_buttons(self)
	# F111 compact polish: on a compact phone "Quay lại" shows its arrow alone (the words stay as tooltip and accessible
	# name), so the points and the first quest show without scrolling.
	UiBuild.icon_only(_back, "ui.events.back", AppState.compact)
	_apply_compact()


## F111 compact polish: the name, the time left and the points tag on one line of the header (compact phone), else the
## name over the time left and the points under the description.
func _apply_compact() -> void:
	var on: bool = AppState.compact
	var order: Array[Control] = [_name, _time_row, _points]
	for i: int in order.size():
		var node: Control = order[i]
		var parent: Node = _title_line if on else (_points_row if node == _points else _titles)
		if node.get_parent() != parent:
			node.reparent(parent, false)
		if on:
			_title_line.move_child(node, i)
	_name.autowrap_mode = TextServer.AUTOWRAP_OFF if on else TextServer.AUTOWRAP_WORD_SMART
	_title_line.visible = on
	_points_row.visible = not on


func set_busy(busy: bool) -> void:
	_busy = busy
	_join.disabled = busy
	for b: Button in _claims.values():
		b.disabled = busy


func join_button() -> Button:
	return _join


func claim_button(milestone_id: String) -> Button:
	return _claims.get(milestone_id) as Button


func paused_visible() -> bool:
	return _paused.visible


func ended_note_visible() -> bool:
	return _ended.visible


func quests_visible() -> bool:
	return _quests_box.visible


func milestones_visible() -> bool:
	return _milestones_box.visible


func goal_bar() -> CommunityGoalBar:
	return _goal


func back_button() -> Button:
	return _back


func texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for node: Node in find_children("*", "Label", true, false):
		if (node as Label).is_visible_in_tree() or (node as Label).visible:
			out.append(tr((node as Label).text))
	return out


func _section(node_name: String, title_key: String, icon_name: String) -> VBoxContainer:
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	box.name = node_name
	add_child(box)
	box.add_child(FriendsUi.group_title(title_key, icon_name))
	var rows: VBoxContainer = UiBuild.vbox(&"FormBox")
	rows.name = "Rows"
	box.add_child(rows)
	return box


func _quest_row(quest_id: String, q: QuestModel) -> Control:
	var card: PanelContainer = UiBuild.panel(&"PartyMemberCard")
	card.name = "Quest"
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	card.add_child(row)
	var icon: TextureRect = UiBuild.icon_rect("journal", 0.9)
	icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(icon)
	var title: Label = UiBuild.keep_words(UiBuild.data_label(q.display_name() if q != null else tr("ui.events.quest_generic"),
			&"InstructionLabel"))
	title.name = "QuestName"
	UiBuild.expand(title)
	title.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(title)
	var go: Button = UiBuild.button("ui.events.go", "next_light")
	go.name = "Go"
	go.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	go.pressed.connect(func() -> void: go_requested.emit(quest_id))
	row.add_child(go)
	return card


func _milestone_row(m: EventMilestoneModel, claims_open: bool) -> Control:
	var card: PanelContainer = UiBuild.panel(&"PartyMemberCard")
	card.name = "Milestone_%s" % m.milestone_id.validate_node_name()
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	card.add_child(row)
	var icon: TextureRect = UiBuild.icon_rect("gift", 0.9)
	icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(icon)
	var texts: VBoxContainer = UiBuild.vbox()
	UiBuild.expand(texts)
	texts.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(texts)
	var pts: Label = UiBuild.data_label(tr("ui.events.milestone_points").format({"points": LocaleFormat.number(m.points_required)}),
			&"HeadingLabel", false)
	pts.name = "MilestonePoints"
	texts.add_child(pts)
	var words: PackedStringArray = []
	for r: RewardModel in m.rewards:
		words.append(r.display_text())
	var rewards: Label = UiBuild.keep_words(UiBuild.data_label(", ".join(words), &"MutedLabel"))
	rewards.name = "Rewards"
	texts.add_child(rewards)
	if m.just_reached and not m.claimed:
		var badge: Label = UiBuild.label("ui.events.just_reached", &"SectionTag", false)
		badge.name = "JustReached"
		badge.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		row.add_child(badge)
	if m.claimed:
		row.add_child(FriendsUi.status("ui.events.claimed", "verdict_correct"))
	elif m.reached and claims_open:
		var claim: Button = UiBuild.button("ui.events.claim", "gift")
		claim.name = "Claim"
		claim.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		claim.disabled = _busy
		claim.pressed.connect(func() -> void: claim_requested.emit(event_id, m.milestone_id))
		row.add_child(claim)
		_claims[m.milestone_id] = claim
	elif not m.reached:
		row.add_child(FriendsUi.status("ui.events.not_reached", "lock"))
	return card
