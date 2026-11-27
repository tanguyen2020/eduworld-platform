class_name LeaderboardView
extends VBoxContainer
## Tab "Bảng xếp hạng" of the arena (F109 US4; STUDENT_GAME_UX_SOCIAL §8 `LeaderboardView`, COMPETITION_SYSTEM §9.4):
## one chip per board the server returns (F109-AC-18 — an empty list is a friendly empty state), then the board as the
## server shows it to this student (`visibility`, AC-19): `grade_goal` → GradeGoalBar (no rank, no nickname);
## `top_n_percentile` → the top rows + "Em đang ở nhóm 25% — cố lên nhé!"; `ranked` → the top rows and the rows around
## the student. Rows: rank, portrait, nickname, value, "Em" tag (AC-20: nickname + avatar only). "Đang cập nhật" when
## `stale`; a 404 drops the tab with "Bảng này hiện không mở" (AC-21). No timer: "Làm mới" reads it again (AC-22).
## UI only: LeaderboardService does the work.

var service: LeaderboardService = null
var selected: String = ""
var _chips_row: HBoxContainer = null
var _group: ButtonGroup = ButtonGroup.new()
var _notice: Dictionary = {}
var _head: HBoxContainer = null
var _title: Label = null
var _stale: Label = null
var _refresh: Button = null
var _content: VBoxContainer = null
var _empty: Control = null


func _init() -> void:
	name = "LeaderboardView"
	theme_type_variation = &"FormBox"
	UiBuild.expand(self, true, true)
	_chips_row = UiBuild.hbox(&"ToolbarBox")
	_chips_row.name = "Boards"
	add_child(UiBuild.chip_scroller(_chips_row))
	_notice = ArenaUi.notice()
	add_child(_notice["panel"] as PanelContainer)
	_head = UiBuild.hbox(&"ToolbarBox")
	_head.name = "Head"
	add_child(_head)
	_title = UiBuild.data_label("", &"HeadingLabel")
	_title.name = "BoardTitle"
	UiBuild.expand(_title)
	_head.add_child(_title)
	_stale = ArenaUi.tag("ui.leaderboard.stale", &"TileTagPending")
	_stale.name = "Stale"
	_stale.visible = false
	_head.add_child(_stale)
	_refresh = UiBuild.button("ui.leaderboard.refresh", "rotate_right", &"SecondaryButton")
	_refresh.name = "Refresh"
	_refresh.pressed.connect(func() -> void: reload_selected())
	_head.add_child(_refresh)
	var sb: Dictionary = ArenaUi.scroll_box()
	add_child(sb["scroll"] as ScrollContainer)
	_content = sb["box"] as VBoxContainer
	_content.name = "Rows"
	_empty = ArenaUi.coach_line("ui.leaderboard.empty")
	_empty.visible = false
	add_child(_empty)


func bind(next: LeaderboardService) -> void:
	if service != null:
		for pair: Array in [[service.boards_changed, _render_chips], [service.page_changed, _on_page],
				[service.board_closed, _on_board_closed]]:
			if (pair[0] as Signal).is_connected(pair[1] as Callable):
				(pair[0] as Signal).disconnect(pair[1] as Callable)
	service = next
	if service == null:
		return
	service.boards_changed.connect(_render_chips)
	service.page_changed.connect(_on_page)
	service.board_closed.connect(_on_board_closed)


## Coroutine: reads the boards and the first one (opening the tab — never on a timer, AC-22).
func refresh() -> void:
	if service == null:
		return
	await service.load_boards()
	_render_chips()
	if service.board(selected) == null:
		selected = service.boards[0].leaderboard_id if not service.boards.is_empty() else ""
	_render_chips()
	await reload_selected()


func reload_selected() -> void:
	var b: LeaderboardSummaryModel = service.board(selected) if service != null else null
	if b == null:
		_render_page()
		return
	_refresh.disabled = true
	await service.load_page(b)
	_refresh.disabled = false
	_render_page()


func select(leaderboard_id: String) -> void:
	selected = leaderboard_id
	_render_chips()
	reload_selected()


# ---------- test hooks ----------

func board_chips() -> Array[Node]:
	return _chips_row.get_children()


func rows() -> Array[Node]:
	return _content.find_children("Row*", "PanelContainer", true, false)


func grade_goal_bar() -> GradeGoalBar:
	return _content.find_child("GradeGoalBar", true, false) as GradeGoalBar


func percentile_text() -> String:
	var l: Label = _content.find_child("PercentileText", true, false) as Label
	return l.text if l != null else ""


func notice_text() -> String:
	return (_notice["label"] as Label).text if (_notice["panel"] as PanelContainer).visible else ""


func empty_visible() -> bool:
	return _empty.visible


func stale_visible() -> bool:
	return _stale.visible


func visible_texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for node: Node in find_children("*", "Label", true, false):
		var l: Label = node as Label
		if l.is_visible_in_tree():
			out.append(l.tr(l.text) if l.can_auto_translate() else l.text)
	return out


# ---------- rendering ----------

func _render_chips() -> void:
	UiBuild.clear(_chips_row)
	if service == null:
		return
	for b: LeaderboardSummaryModel in service.boards:
		var chip: Button = UiBuild.chip("", _group)
		chip.name = "Board_%s" % b.code
		chip.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		chip.text = b.title
		chip.set_pressed_no_signal(b.leaderboard_id == selected)
		chip.pressed.connect(select.bind(b.leaderboard_id))
		_chips_row.add_child(chip)
	UiBuild.mark_chips(_chips_row.get_children())
	var none: bool = service.boards_loaded and service.boards.is_empty()
	_empty.visible = none
	_head.visible = not none


func _render_page() -> void:
	UiBuild.clear(_content)
	var b: LeaderboardSummaryModel = service.board(selected) if service != null else null
	var p: LeaderboardPageModel = service.page(selected) if b != null else null
	_title.text = b.title if b != null else ""
	_stale.visible = p != null and p.stale
	if b == null or p == null:
		return
	match p.visibility:
		LeaderboardPageModel.VIS_GRADE_GOAL:
			var bar: GradeGoalBar = GradeGoalBar.new()
			_content.add_child(bar)
			var goal: Dictionary = p.grade_goal.duplicate()
			if not goal.has("grade_level") or int(goal["grade_level"]) <= 0:
				goal["grade_level"] = b.grade_level
			bar.show_goal(goal)
			_content.add_child(ArenaUi.coach_line("ui.leaderboard.goal.coach", {}, CloudyMascot.Mood.HAPPY))
		LeaderboardPageModel.VIS_TOP_PERCENTILE:
			_add_rows(p.entries)
			var me: LeaderboardEntryModel = p.my_entry()
			if me == null and int(p.me.get("percentile_bucket", -1)) > 0:
				var card: PanelContainer = UiBuild.panel(&"QuestCard")
				card.name = "PercentileCard"
				var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
				card.add_child(row)
				row.add_child(UiBuild.icon_rect("emoji_muscle", 1.2))
				var l: Label = UiBuild.data_label(TranslationServer.translate("ui.leaderboard.percentile").format({
					"bucket": int(p.me["percentile_bucket"])}), &"HeadingLabel")
				l.name = "PercentileText"
				UiBuild.expand(l)
				row.add_child(l)
				_content.add_child(card)
		_:
			_add_rows(p.entries)
			var extra: Array[LeaderboardEntryModel] = p.extra_around()
			if not extra.is_empty():
				_content.add_child(ArenaUi.section("ui.leaderboard.around_me"))
				_add_rows(extra)
	if p.entries.is_empty() and p.visibility != LeaderboardPageModel.VIS_GRADE_GOAL:
		_content.add_child(UiBuild.label("ui.leaderboard.no_rows", &"MutedLabel"))
	UiBuild.size_buttons(self)


func _add_rows(list: Array[LeaderboardEntryModel]) -> void:
	for e: LeaderboardEntryModel in list:
		var card: PanelContainer = UiBuild.panel(&"QuestCard" if e.is_me else &"PartyMemberCard")
		card.name = "Row%d" % e.rank
		var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
		card.add_child(row)
		var rank: Label = UiBuild.data_label(TranslationServer.translate("ui.leaderboard.rank").format({"rank": e.rank}),
				&"HeadingLabel", false)
		rank.name = "Rank"
		rank.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		row.add_child(rank)
		row.add_child(ArenaUi.portrait(e.player.avatar_config, 0.8))
		var nick: Label = UiBuild.data_label(e.player.nickname, &"HeadingLabel", false)
		nick.name = "Nickname"
		# A nickname stays on one line (≤ 32 characters, never cut): its row grows instead (F109-AC-30 edge case).
		nick.tooltip_text = e.player.nickname
		nick.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		UiBuild.expand(nick)
		row.add_child(nick)
		if e.is_me:
			row.add_child(ArenaUi.tag("ui.leaderboard.me", &"TileTagCorrect"))
		if e.value != LeaderboardEntryModel.VALUE_NONE:
			var value: Label = UiBuild.data_label(TranslationServer.translate("ui.leaderboard.value").format({
				"value": ArenaUi.num(e.value)}), &"InstructionLabel", false)
			value.name = "Value"
			value.size_flags_vertical = Control.SIZE_SHRINK_CENTER
			row.add_child(value)
		_content.add_child(card)


func _on_page(leaderboard_id: String) -> void:
	if leaderboard_id == selected and is_visible_in_tree():
		_render_page()


func _on_board_closed(leaderboard_id: String) -> void:
	ArenaUi.show_notice(_notice, "ui.leaderboard.closed", {}, "info")
	if leaderboard_id == selected:
		selected = service.boards[0].leaderboard_id if not service.boards.is_empty() else ""
		_render_chips()
		if not selected.is_empty():
			reload_selected()
		else:
			_render_page()
