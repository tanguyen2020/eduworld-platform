class_name TournamentView
extends VBoxContainer
## Tab "Giải đấu" of the arena (F109 US5; COMPETITION_SYSTEM §10): the grade challenges of the school ("Vào phòng"
## inside the server join window → the Timed Quiz) and the tournaments the server lists for the student (status in
## words, start time, the student's entry). "Xem" opens the detail (`getTournament`): schedule, reward, "Đăng ký" /
## "Rút tên" / "Check-in" — enabled only by the server status and windows (F109-AC-24) — and the bracket: a tree for
## lớp 6–9, the simple "con đường" for lớp 1–5 (AC-25). Nicknames only. UI only: TournamentService does the work.

signal match_requested(match_id: String, mode: String)

var service: TournamentService = null
var self_id: String = ""
var open_id: String = ""
var _notice: Dictionary = {}
var _list_scroll: ScrollContainer = null
var _list: VBoxContainer = null
var _more: Button = null
var _detail_scroll: ScrollContainer = null
var _detail: VBoxContainer = null
var _empty: Control = null


func _init() -> void:
	name = "TournamentView"
	theme_type_variation = &"FormBox"
	UiBuild.expand(self, true, true)
	_notice = ArenaUi.notice()
	add_child(_notice["panel"] as PanelContainer)
	var sb: Dictionary = ArenaUi.scroll_box()
	_list_scroll = sb["scroll"] as ScrollContainer
	_list_scroll.name = "ListScroll"
	add_child(_list_scroll)
	_list = sb["box"] as VBoxContainer
	_list.name = "Tournaments"
	_more = UiBuild.button("ui.tournament.more", "arrow_down", &"SecondaryButton")
	_more.name = "More"
	_more.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	_more.pressed.connect(func() -> void: _load_more())
	add_child(_more)
	var db: Dictionary = ArenaUi.scroll_box()
	_detail_scroll = db["scroll"] as ScrollContainer
	_detail_scroll.name = "DetailScroll"
	_detail_scroll.visible = false
	add_child(_detail_scroll)
	_detail = db["box"] as VBoxContainer
	_detail.name = "Detail"
	_empty = ArenaUi.coach_line("ui.tournament.empty")
	_empty.visible = false
	add_child(_empty)


func bind(next: TournamentService) -> void:
	if service != null:
		for pair: Array in [[service.notice_requested, _on_notice], [service.detail_changed, _on_detail_changed],
				[service.bracket_changed, _on_bracket_changed]]:
			if (pair[0] as Signal).is_connected(pair[1] as Callable):
				(pair[0] as Signal).disconnect(pair[1] as Callable)
	service = next
	if service == null:
		return
	service.notice_requested.connect(_on_notice)
	service.detail_changed.connect(_on_detail_changed)
	service.bracket_changed.connect(_on_bracket_changed)


## Coroutine: challenges + tournaments (opening the tab or "Làm mới").
func refresh() -> void:
	if service == null:
		return
	await service.load_challenges()
	await service.load_list()
	if not open_id.is_empty():
		await open_detail(open_id)
	else:
		_show_list()


# ---------- test hooks ----------

func tournament_cards() -> Array[Node]:
	return _list.find_children("Tournament_*", "PanelContainer", false, false)


func challenge_cards() -> Array[Node]:
	return _list.find_children("Challenge_*", "PanelContainer", false, false)


func detail_visible() -> bool:
	return _detail_scroll.visible


func action_button(action: String) -> Button:
	return _detail.find_child(action, true, false) as Button


func bracket_tree() -> BracketTree:
	return _detail.find_child("BracketTree", true, false) as BracketTree


func bracket_path() -> BracketPath:
	return _detail.find_child("BracketPath", true, false) as BracketPath


func notice_text() -> String:
	return (_notice["label"] as Label).text if (_notice["panel"] as PanelContainer).visible else ""


func detail_texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for node: Node in _detail.find_children("*", "Label", true, false):
		var l: Label = node as Label
		out.append(l.tr(l.text) if l.can_auto_translate() else l.text)
	return out


## Coroutine: the detail of `tournament_id` with its bracket (once the bracket exists).
func open_detail(tournament_id: String) -> void:
	open_id = tournament_id
	var result: ApiResult = await service.load_detail(tournament_id)
	if not result.ok:
		open_id = ""
		_show_list()
		return
	var t: TournamentModel = service.tournament(tournament_id)
	if t != null and t.status in ["scheduled", "in_progress", "completed", "rewarded"]:
		await service.load_bracket(tournament_id)
	_render_detail()


# ---------- rendering ----------

func _show_list() -> void:
	open_id = ""
	_detail_scroll.visible = false
	_list_scroll.visible = true
	UiBuild.clear(_list)
	if service == null:
		return
	var now: int = service.now_ms()
	for c: GradeChallengeModel in service.challenges:
		if c.is_timed_quiz():
			_list.add_child(_challenge_card(c, now))
	for t: TournamentModel in service.tournaments:
		_list.add_child(_tournament_card(t))
	_more.visible = not service.page_cursor.is_empty()
	_empty.visible = service.tournaments_loaded and service.tournaments.is_empty() and _list.get_child_count() == 0
	UiBuild.size_buttons(self)


func _challenge_card(c: GradeChallengeModel, now: int) -> PanelContainer:
	var card: PanelContainer = UiBuild.panel(&"QuestCard")
	card.name = "Challenge_%s" % c.challenge_id.right(4)
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	card.add_child(row)
	row.add_child(UiBuild.icon_rect("clock", 1.2))
	var texts: VBoxContainer = UiBuild.vbox()
	UiBuild.expand(texts)
	row.add_child(texts)
	texts.add_child(UiBuild.data_label(c.name, &"HeadingLabel"))
	texts.add_child(UiBuild.data_label(TranslationServer.translate("ui.tournament.challenge.when").format({
		"time": _time(c.starts_ms)}), &"MutedLabel"))
	var join: Button = UiBuild.button("ui.tournament.challenge.join", "play_light")
	join.disabled = not c.can_join(now)
	# The white play sign is for the blue face; a disabled (light) button shows the blue one (F102 rule).
	if join.disabled:
		join.icon = UiBuild.icon("play")
	join.disabled = not c.can_join(now)
	join.tooltip_text = "ui.tournament.challenge.not_open" if join.disabled else ""
	join.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	join.pressed.connect(_on_join.bind(c.challenge_id))
	row.add_child(join)
	return card


func _tournament_card(t: TournamentModel) -> PanelContainer:
	var card: PanelContainer = UiBuild.panel(&"QuestCard")
	card.name = "Tournament_%s" % t.tournament_id.right(4)
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	card.add_child(row)
	row.add_child(UiBuild.icon_rect("bracket", 1.2))
	var texts: VBoxContainer = UiBuild.vbox()
	UiBuild.expand(texts)
	row.add_child(texts)
	texts.add_child(UiBuild.data_label(t.name, &"HeadingLabel"))
	var tags: HFlowContainer = HFlowContainer.new()
	texts.add_child(tags)
	tags.add_child(ArenaUi.tag(String(CompetitionText.TOURNAMENT_STATUS_KEYS.get(t.status, "ui.tournament.status.published"))))
	if t.my_entry != null:
		tags.add_child(ArenaUi.tag(String(CompetitionText.ENTRY_STATUS_KEYS.get(t.my_entry.status,
				"ui.tournament.entry.registered")), &"TileTagCorrect"))
	if t.starts_ms > 0:
		texts.add_child(UiBuild.data_label(TranslationServer.translate("ui.tournament.starts").format({
			"time": _time(t.starts_ms)}), &"MutedLabel"))
	var open: Button = UiBuild.button("ui.tournament.view", "next_light")
	open.name = "View"
	open.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	open.pressed.connect(open_detail.bind(t.tournament_id))
	row.add_child(open)
	return card


func _render_detail() -> void:
	var t: TournamentModel = service.tournament(open_id)
	if t == null:
		_show_list()
		return
	_list_scroll.visible = false
	_more.visible = false
	_empty.visible = false
	_detail_scroll.visible = true
	UiBuild.clear(_detail)
	var back: Button = UiBuild.button("ui.tournament.back", "arrow_left", &"SecondaryButton")
	back.name = "Back"
	back.size_flags_horizontal = Control.SIZE_SHRINK_BEGIN
	back.pressed.connect(_show_list)
	_detail.add_child(back)
	var head: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	_detail.add_child(head)
	head.add_child(UiBuild.icon_rect("bracket", 1.4))
	var title: Label = UiBuild.data_label(t.name, &"HeadingLabel")
	UiBuild.expand(title)
	head.add_child(title)
	head.add_child(ArenaUi.tag(String(CompetitionText.TOURNAMENT_STATUS_KEYS.get(t.status, "ui.tournament.status.published"))))
	var info: VBoxContainer = UiBuild.vbox(&"FieldBox")
	_detail.add_child(info)
	if t.registration_closes_ms > 0:
		info.add_child(_info_line("clock", TranslationServer.translate("ui.tournament.registration_closes").format({
			"time": _time(t.registration_closes_ms)})))
	if t.check_in_opens_ms > 0:
		info.add_child(_info_line("tick", TranslationServer.translate("ui.tournament.check_in_opens").format({
			"time": _time(t.check_in_opens_ms)})))
	if t.starts_ms > 0:
		info.add_child(_info_line("play", TranslationServer.translate("ui.tournament.starts").format({"time": _time(t.starts_ms)})))
	info.add_child(_info_line("group", TranslationServer.translate("ui.tournament.entries").format({
		"count": t.entry_count, "max": t.max_entries})))
	if not t.reward_summary.is_empty():
		info.add_child(_info_line("gift", t.reward_summary))
	if t.my_entry != null:
		info.add_child(_info_line("tick", TranslationServer.translate(String(CompetitionText.ENTRY_STATUS_KEYS.get(
			t.my_entry.status, "ui.tournament.entry.registered")))))
	var actions: HFlowContainer = HFlowContainer.new()
	actions.name = "Actions"
	_detail.add_child(actions)
	var now: int = service.now_ms()
	if t.can_register():
		actions.add_child(_action("Register", "ui.tournament.register", "plus", _on_register.bind(t.tournament_id), true))
	if t.can_withdraw():
		actions.add_child(_action("Withdraw", "ui.tournament.withdraw", "cancel", _on_withdraw.bind(t.tournament_id), false))
	if t.registered() and not t.checked_in():
		var check: Button = _action("CheckIn", "ui.tournament.check_in", "check", _on_check_in.bind(t.tournament_id), true)
		check.disabled = not t.can_check_in(now)
		check.tooltip_text = "ui.tournament.check_in_wait" if check.disabled else ""
		# The white tick is for the blue face; a disabled (light) button shows the blue one (F102 rule).
		check.icon = UiBuild.icon("tick" if check.disabled else "check")
		actions.add_child(check)
	var b: BracketModel = service.brackets.get(open_id) as BracketModel
	if b != null:
		var section: Control = ArenaUi.section("ui.tournament.bracket.title")
		_detail.add_child(section)
		# A running tournament opens on its bracket: "where am I, who is next" is what the student came for.
		if t.status == "in_progress":
			_scroll_to.call_deferred(section)
		if ArenaUi.band() in ["g1_3", "g4_5"]:
			var path: BracketPath = BracketPath.new()
			_detail.add_child(path)
			path.show_bracket(b, self_id)
		else:
			var tree: BracketTree = BracketTree.new()
			_detail.add_child(tree)
			tree.show_bracket(b, self_id)
	UiBuild.size_buttons(_detail)


func _scroll_to(target: Control) -> void:
	if not is_inside_tree():
		return
	await get_tree().process_frame
	await get_tree().process_frame
	if is_instance_valid(target) and _detail_scroll.visible:
		_detail_scroll.scroll_vertical = int(target.get_global_rect().position.y - _detail.get_global_rect().position.y)


func _info_line(icon: String, text: String) -> HBoxContainer:
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	row.add_child(UiBuild.icon_rect(icon, 0.8))
	var l: Label = UiBuild.data_label(text, &"MutedLabel")
	UiBuild.expand(l)
	row.add_child(l)
	return row


func _action(node_name: String, key: String, icon: String, cb: Callable, primary: bool) -> Button:
	var b: Button = UiBuild.button(key, icon if not primary else icon, &"" if primary else &"SecondaryButton")
	if primary and icon == "plus":
		b.icon = UiBuild.icon("check")
	b.name = node_name
	b.pressed.connect(cb)
	return b


static func _time(ms: int) -> String:
	if ms <= 0:
		return ""
	var dt: Dictionary = LocaleFormat.from_unix(int(ms / 1000.0))
	return "%s %s" % [LocaleFormat.time(dt), LocaleFormat.date(dt)]


func _load_more() -> void:
	await service.load_list(true)
	_show_list()


func _on_join(challenge_id: String) -> void:
	var result: ApiResult = await service.join_challenge(challenge_id)
	if result.ok:
		match_requested.emit(DataRead.str_of(result.body(), "match_id"), "timed_quiz")


func _on_register(tournament_id: String) -> void:
	var result: ApiResult = await service.register(tournament_id)
	if result.ok:
		ArenaUi.show_notice(_notice, "ui.tournament.registered_done", {})
	_render_detail()


func _on_withdraw(tournament_id: String) -> void:
	var t: TournamentModel = service.tournament(tournament_id)
	if t != null:
		await service.withdraw(t)
	_render_detail()


func _on_check_in(tournament_id: String) -> void:
	var t: TournamentModel = service.tournament(tournament_id)
	if t == null:
		return
	var result: ApiResult = await service.check_in(t)
	if result.ok:
		ArenaUi.show_notice(_notice, "ui.tournament.checked_in_done", {})
	_render_detail()


func _on_notice(key: String, args: Dictionary) -> void:
	ArenaUi.show_notice(_notice, key, args, "error")


func _on_detail_changed(tournament_id: String) -> void:
	if tournament_id == open_id and _detail_scroll.visible:
		_render_detail()


func _on_bracket_changed(tournament_id: String) -> void:
	if tournament_id == open_id and _detail_scroll.visible:
		_render_detail()
