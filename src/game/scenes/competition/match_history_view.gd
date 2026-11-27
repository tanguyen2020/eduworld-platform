class_name MatchHistoryView
extends VBoxContainer
## Tab "Lịch sử" of the arena (F109-AC-17; STUDENT_GAME_UX_SOCIAL §8 `MatchHistoryList`): mode filter chips ("Tất cả" +
## the modes the server opened), one card per match (outcome in encouraging words + icon, mode, the others' nicknames,
## the student's points) and "Xem thêm" (cursor). "Xem" opens the detail (`getMatch`): every player as portrait +
## nickname only (R16), the student's score, the review after the match (question, ✓ / ✗ in words, right answer) and
## "Báo cáo người chơi" / "Chặn" (F119 ChatPlayerSafety + report sheet, F109-AC-16). A 404 shows "Không tìm thấy" and
## no data (AC-29). UI only.

var service: MatchHistoryService = null
var modes: Array[ArenaModeModel] = []
var self_id: String = ""
var _filters: HBoxContainer = null
var _group: ButtonGroup = ButtonGroup.new()
var _notice: Dictionary = {}
var _list_box: VBoxContainer = null
var _list_scroll: ScrollContainer = null
var _more: Button = null
var _detail_scroll: ScrollContainer = null
var _detail: VBoxContainer = null
var _empty: Control = null
var _sheet: ChatSheet = null
var _dialogs: ChatDialogs = null
var _safety: ChatPlayerSafety = null


func _init() -> void:
	name = "MatchHistoryView"
	theme_type_variation = &"FormBox"
	UiBuild.expand(self, true, true)
	_filters = UiBuild.hbox(&"ToolbarBox")
	_filters.name = "Filters"
	add_child(UiBuild.chip_scroller(_filters))
	_notice = ArenaUi.notice()
	add_child(_notice["panel"] as PanelContainer)
	var sb: Dictionary = ArenaUi.scroll_box()
	_list_scroll = sb["scroll"] as ScrollContainer
	_list_scroll.name = "ListScroll"
	add_child(_list_scroll)
	_list_box = sb["box"] as VBoxContainer
	_list_box.name = "Matches"
	_more = UiBuild.button("ui.arena.history.more", "arrow_down", &"SecondaryButton")
	_more.name = "More"
	_more.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	_more.pressed.connect(func() -> void: _load(true))
	add_child(_more)
	var db: Dictionary = ArenaUi.scroll_box()
	_detail_scroll = db["scroll"] as ScrollContainer
	_detail_scroll.name = "DetailScroll"
	_detail_scroll.visible = false
	add_child(_detail_scroll)
	_detail = db["box"] as VBoxContainer
	_detail.name = "Detail"
	_empty = ArenaUi.coach_line("ui.arena.history.empty")
	_empty.visible = false
	add_child(_empty)
	_sheet = ChatSheet.new()
	add_child(_sheet)
	_dialogs = ChatDialogs.new(_sheet)
	_dialogs.notice_requested.connect(func(key: String, args: Dictionary, _tone: String) -> void:
		ArenaUi.show_notice(_notice, key, args))


func bind(next: MatchHistoryService, chat: ChatService, social: SocialService, reports: ReportService) -> void:
	service = next
	_dialogs.bind(chat, social, reports)
	if service != null and not service.notice_requested.is_connected(_on_notice):
		service.notice_requested.connect(_on_notice)


## Coroutine: the first page for the current filter.
func refresh() -> void:
	_render_filters()
	await _load(false)


# ---------- test hooks ----------

func cards() -> Array[Node]:
	return _list_box.find_children("Match_*", "PanelContainer", false, false)


func detail_visible() -> bool:
	return _detail_scroll.visible


func detail_texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for node: Node in _detail.find_children("*", "Label", true, false):
		var l: Label = node as Label
		out.append(l.tr(l.text) if l.can_auto_translate() else l.text)
	return out


func safety() -> ChatPlayerSafety:
	return _safety


func dialogs() -> ChatDialogs:
	return _dialogs


func filter_chips() -> Array[Node]:
	return _filters.get_children()


func notice_text() -> String:
	return (_notice["label"] as Label).text if (_notice["panel"] as PanelContainer).visible else ""


func more_button() -> Button:
	return _more


## Coroutine: opens the detail of `match_id` (also used by the result screen's "Xem lại").
func open_detail(match_id: String) -> void:
	var result: ApiResult = await service.load_detail(match_id)
	UiBuild.clear(_detail)
	_safety = null
	var m: MatchRecord = service.details.get(match_id) as MatchRecord
	if not result.ok or m == null:
		_show_list()
		return
	_list_scroll.visible = false
	_more.visible = false
	_filters.get_parent().visible = false
	_detail_scroll.visible = true
	var back: Button = UiBuild.button("ui.arena.history.back", "arrow_left", &"SecondaryButton")
	back.name = "Back"
	back.size_flags_horizontal = Control.SIZE_SHRINK_BEGIN
	back.pressed.connect(_show_list)
	_detail.add_child(back)
	var mine: Dictionary = m.row_of(self_id)
	_detail.add_child(_outcome_line(m, mine))
	for p: Dictionary in m.participants:
		var pl: PublicPlayerModel = p["player"] as PublicPlayerModel
		var tag_key: String = "ui.arena.history.you" if pl.user_id == self_id else ""
		var row: PanelContainer = ArenaUi.player_row(pl.nickname, pl.avatar_config, tag_key)
		if int(p["score"]) >= 0:
			var score: Label = UiBuild.data_label(TranslationServer.translate("ui.arena.battle.score").format({
				"score": ArenaUi.num(float(int(p["score"])))}), &"InstructionLabel", false)
			score.size_flags_vertical = Control.SIZE_SHRINK_CENTER
			(row.get_child(0) as HBoxContainer).add_child(score)
		_detail.add_child(row)
	if not m.review.is_empty():
		_detail.add_child(ArenaUi.section("ui.arena.review.title"))
		for r: Dictionary in m.review:
			_detail.add_child(_review_row(r))
	var others: Array[Dictionary] = []
	for pl: PublicPlayerModel in m.others(self_id):
		others.append({"user_id": pl.user_id, "nickname": pl.nickname, "avatar_config": pl.avatar_config})
	if not others.is_empty() and not m.is_bot:
		_detail.add_child(ArenaUi.section("ui.arena.safety.title"))
		_safety = ChatPlayerSafety.new()
		_safety.set_players(others)
		_dialogs.context = {"match_id": m.match_id}
		_safety.report_requested.connect(func(id: String, nick: String) -> void:
			_dialogs.open_report(ReportService.TARGET_USER, id, nick, id))
		_safety.block_requested.connect(func(id: String, nick: String) -> void:
			_dialogs.relation(ChatDialogs.ACT_BLOCK, id, nick))
		_detail.add_child(_safety)
	UiBuild.size_buttons(_detail)


# ---------- internals ----------

func _load(more: bool) -> void:
	if service == null:
		return
	var mode: String = service.mode_filter
	await service.load_history(mode, more)
	_show_list()


func _render_filters() -> void:
	UiBuild.clear(_filters)
	var all: Button = UiBuild.chip("ui.arena.history.all", _group)
	all.name = "Filter_all"
	all.set_pressed_no_signal(service == null or service.mode_filter.is_empty())
	all.pressed.connect(_on_filter.bind(""))
	_filters.add_child(all)
	for m: ArenaModeModel in modes:
		var chip: Button = UiBuild.chip(CompetitionText.mode_key(m.mode), _group)
		chip.name = "Filter_%s" % m.mode
		chip.set_pressed_no_signal(service != null and service.mode_filter == m.mode)
		chip.pressed.connect(_on_filter.bind(m.mode))
		_filters.add_child(chip)
	UiBuild.mark_chips(_filters.get_children())


func _on_filter(mode: String) -> void:
	UiBuild.mark_chips(_filters.get_children())
	await service.load_history(mode, false)
	_show_list()


func _show_list() -> void:
	_detail_scroll.visible = false
	_list_scroll.visible = true
	_filters.get_parent().visible = true
	UiBuild.clear(_list_box)
	if service == null:
		return
	for m: MatchRecord in service.matches:
		_list_box.add_child(_card(m))
	_more.visible = service.has_more()
	_empty.visible = service.loaded and service.matches.is_empty()
	UiBuild.size_buttons(self)


func _card(m: MatchRecord) -> PanelContainer:
	var card: PanelContainer = UiBuild.panel(&"QuestCard")
	card.name = "Match_%s" % m.match_id.right(4)
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	card.add_child(row)
	var mine: Dictionary = m.row_of(self_id)
	var outcome: String = "cancelled" if m.is_void() else String(mine.get("outcome", ""))
	row.add_child(UiBuild.icon_rect(String(CompetitionText.OUTCOME_ICONS.get(outcome, "info")), 1.2))
	var texts: VBoxContainer = UiBuild.vbox()
	UiBuild.expand(texts)
	row.add_child(texts)
	var title: String = TranslationServer.translate("ui.arena.history.void") if m.is_void() \
			else TranslationServer.translate(CompetitionText.outcome_key(outcome)).format({"topic": ""}).strip_edges()
	var head: Label = UiBuild.data_label("%s · %s" % [title, TranslationServer.translate(CompetitionText.mode_key(m.mode))],
			&"HeadingLabel")
	head.name = "Outcome"
	texts.add_child(head)
	var names: PackedStringArray = []
	for pl: PublicPlayerModel in m.others(self_id):
		names.append(TranslationServer.translate("ui.arena.battle.robot") if m.is_bot else pl.nickname)
	if not names.is_empty():
		var vs: Label = UiBuild.data_label(TranslationServer.translate("ui.arena.history.with").format({
			"names": ", ".join(names)}), &"MutedLabel")
		vs.name = "Others"
		texts.add_child(vs)
	if int(mine.get("competition_points", -1)) > 0:
		texts.add_child(UiBuild.data_label(TranslationServer.translate("ui.arena.result.points").format({
			"points": ArenaUi.num(float(int(mine["competition_points"])))}), &"MutedLabel"))
	var open: Button = UiBuild.button("ui.arena.history.view", "next_light")
	open.name = "View"
	open.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	open.pressed.connect(open_detail.bind(m.match_id))
	row.add_child(open)
	return card


func _outcome_line(m: MatchRecord, mine: Dictionary) -> Control:
	if m.is_void():
		return ArenaUi.coach_line("ui.arena.result.void")
	var outcome: String = String(mine.get("outcome", ""))
	var key: String = CompetitionText.outcome_key(outcome)
	return ArenaUi.coach_line(key, {"topic": TranslationServer.translate("ui.arena.result.topic_any")},
			CloudyMascot.Mood.HAPPY if outcome == "win" else CloudyMascot.Mood.ENCOURAGE)


static func review_row(r: Dictionary) -> PanelContainer:
	var card: PanelContainer = UiBuild.panel(&"PartyMemberCard")
	card.name = "Review%d" % int(r["index"])
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	card.add_child(row)
	var verdict: Variant = r.get("my_answer_correct")
	var right: bool = verdict is bool and bool(verdict)
	row.add_child(UiBuild.icon_rect("verdict_correct" if right else ("verdict_try" if verdict is bool else "help")))
	var texts: VBoxContainer = UiBuild.vbox()
	UiBuild.expand(texts)
	row.add_child(texts)
	var stem: String = RichText.plain_text(r.get("stem", {}), RendererContext.new())
	texts.add_child(UiBuild.data_label(TranslationServer.translate("ui.arena.review.question").format({
		"n": int(r["index"]) + 1, "text": stem}), &"HeadingLabel"))
	var word: String = "ui.arena.review.right" if right else ("ui.arena.review.try" if verdict is bool else "ui.arena.review.skipped")
	texts.add_child(UiBuild.label(word, &"MutedLabel"))
	var explanation: String = String(r.get("explanation", ""))
	if not explanation.is_empty():
		texts.add_child(UiBuild.data_label(explanation, &"MutedLabel"))
	return card


func _review_row(r: Dictionary) -> PanelContainer:
	return review_row(r)


func _on_notice(key: String, args: Dictionary) -> void:
	ArenaUi.show_notice(_notice, key, args, "error" if key.ends_with("NOT_FOUND") else "info")
