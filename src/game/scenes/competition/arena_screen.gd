class_name ArenaScreen
extends Control
## Màn Đấu trường — the Competition Arena (F109 US1, US3–US6; STUDENT_GAME_UX_SOCIAL §8): a window like the Party
## screen (F114 tier 1 frame, blue title bar with the student's TierBadge) with four tabs:
## - "Thi đấu": Mây's line, the ModeCards of the modes the server opened (F109-AC-01 — never decided by grade here),
##   the SubjectTopicPicker, "Tìm trận" (≤ 3 taps from the HUD to a match, SC-001) and "Thách đấu bạn" (AC-27). After
##   "Tìm trận" the window closes and the HUD queue bar takes over: the student keeps walking (AC-02);
## - "Bảng xếp hạng" (LeaderboardView, AC-18…AC-22), "Lịch sử" (MatchHistoryView, AC-17), "Giải đấu"
##   (TournamentView, AC-23…AC-25).
## PC: the mode cards on one row; a phone (F113 landscape): the cards scroll sideways and "Tìm trận" stays at the
## bottom right. Every text is a key; nicknames are the only data of other students (R16). UI only (R1).

signal closed()

const TAB_PLAY: String = "play"
const TAB_LEADERBOARD: String = "leaderboard"
const TAB_HISTORY: String = "history"
const TAB_TOURNAMENTS: String = "tournaments"
const TABS: Array = [
	[TAB_PLAY, "ui.arena.tab.play", "tab_match"], [TAB_LEADERBOARD, "ui.arena.tab.leaderboard", "tab_podium"],
	[TAB_HISTORY, "ui.arena.tab.history", "tab_history"], [TAB_TOURNAMENTS, "ui.arena.tab.tournaments", "tab_bracket"],
]

## Tab shown when opened without a `tab` argument (the `leaderboard` screen id opens on its tab).
@export var default_tab: String = TAB_PLAY

var services: ServiceLocator = null
var tab: String = TAB_PLAY
var selected_mode: String = ""
var _ui: Dictionary = {}
var _tier: TierBadge = null
var _tab_buttons: Dictionary = {}
var _pages: Dictionary = {}
var _notice: Dictionary = {}
var _modes_row: HBoxContainer = null
var _mode_cards: Dictionary = {}
var _mode_group: ButtonGroup = ButtonGroup.new()
var _picker: SubjectTopicPicker = null
var _find: Button = null
var _play_row: HBoxContainer = null
var _play_actions: HFlowContainer = null
var _duel: Button = null
var _empty_modes: Label = null
var _leaderboard: LeaderboardView = null
var _history: MatchHistoryView = null
var _tournaments: TournamentView = null
var _duel_dialog: DuelInviteDialog = null


func _ready() -> void:
	visible = false
	_build()
	EventBus.locale_changed.connect(_on_locale_changed)
	AppState.compact_changed.connect(_on_compact_changed)


func _exit_tree() -> void:
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)
	if AppState.compact_changed.is_connected(_on_compact_changed):
		AppState.compact_changed.disconnect(_on_compact_changed)


func _on_compact_changed(_compact: bool) -> void:
	_fit_play()


## WorldRoot: `ui_requested("arena", {tab, tournament_id, duel})`.
func open(args: Dictionary = {}) -> void:
	if services == null and AppState.services != null:
		services = AppState.get_services()
	_bind()
	show()
	UiMotion.pop_in(_ui["window"] as Control, Settings.reduce_motion)
	var next: String = String(args.get("tab", default_tab))
	if args.get("tournament_id") is String and Uuid.is_valid(String(args["tournament_id"])):
		_tournaments.open_id = String(args["tournament_id"])
		next = TAB_TOURNAMENTS
	_show_tab(next)
	if args.get("duel") is Dictionary:
		var incoming: DuelModel = DuelModel.from_dict(args["duel"])
		if incoming != null:
			incoming.other_nickname = String(args.get("nickname", ""))
			_duel_dialog.open_receive(services.matchmaking, incoming)
	await refresh()
	select_tab(next)


func close() -> void:
	_duel_dialog.hide()
	hide()
	closed.emit()


## Coroutine: the modes, the subject names and the tiers (each tab reads its own data when shown).
func refresh() -> void:
	if services == null:
		return
	var mm: MatchmakingService = services.matchmaking
	if mm.subject_names.is_empty():
		await mm.load_subjects()
	await mm.load_modes()
	services.leaderboards.load_tiers()
	_render_modes()


func select_tab(next: String) -> void:
	_show_tab(next)
	match tab:
		TAB_LEADERBOARD:
			_leaderboard.refresh()
		TAB_HISTORY:
			_history.modes = services.matchmaking.modes if services != null else _history.modes
			_history.refresh()
		TAB_TOURNAMENTS:
			_tournaments.refresh()


## Shows the page of `next` (no data read).
func _show_tab(next: String) -> void:
	tab = next if _pages.has(next) else TAB_PLAY
	for id: Variant in _pages.keys():
		(_pages[id] as Control).visible = String(id) == tab
		(_tab_buttons[id] as Button).theme_type_variation = &"TabButtonSelected" if String(id) == tab else &"TabButton"


# ---------- test hooks ----------

func mode_cards() -> Array[ModeCard]:
	var out: Array[ModeCard] = []
	for c: Variant in _mode_cards.values():
		out.append(c as ModeCard)
	return out


func mode_card(mode: String) -> ModeCard:
	return _mode_cards.get(mode) as ModeCard


func picker() -> SubjectTopicPicker:
	return _picker


func find_button() -> Button:
	return _find


func duel_button() -> Button:
	return _duel


func tier_badge() -> TierBadge:
	return _tier


func tab_button(id: String) -> Button:
	return _tab_buttons.get(id) as Button


func leaderboard() -> LeaderboardView:
	return _leaderboard


func history() -> MatchHistoryView:
	return _history


func tournaments() -> TournamentView:
	return _tournaments


func duel_dialog() -> DuelInviteDialog:
	return _duel_dialog


func notice_text() -> String:
	return (_notice["label"] as Label).text if (_notice["panel"] as PanelContainer).visible else ""


func empty_modes_visible() -> bool:
	return _empty_modes.visible


# ---------- binding ----------

func _bind() -> void:
	if services == null:
		return
	var mm: MatchmakingService = services.matchmaking
	if not mm.notice_requested.is_connected(_on_notice):
		mm.notice_requested.connect(_on_notice)
		mm.subjects_changed.connect(_render_picker)
		services.leaderboards.tiers_changed.connect(_render_tier)
	_leaderboard.bind(services.leaderboards)
	_history.self_id = CompetitionPlayer.self_id(AppState)
	_history.bind(services.match_history, services.chat, services.social, services.reports)
	_tournaments.self_id = _history.self_id
	_tournaments.bind(services.tournaments)


# ---------- rendering ----------

func _render_modes() -> void:
	UiBuild.clear(_modes_row)
	_mode_cards.clear()
	var mm: MatchmakingService = services.matchmaking
	for m: ArenaModeModel in mm.modes:
		var card: ModeCard = ModeCard.new()
		card.button_group = _mode_group
		card.bind(m)
		card.pressed.connect(_on_mode.bind(m.mode))
		_modes_row.add_child(card)
		_mode_cards[m.mode] = card
	if mm.mode(selected_mode) == null:
		selected_mode = mm.modes[0].mode if not mm.modes.is_empty() else ""
	if _mode_cards.has(selected_mode):
		(_mode_cards[selected_mode] as ModeCard).set_pressed_no_signal(true)
		(_mode_cards[selected_mode] as ModeCard).refresh_tag()
	_empty_modes.visible = mm.modes_loaded and mm.modes.is_empty()
	_duel.visible = mm.mode("battle_1v1") != null
	_render_picker()
	_render_tier()


func _render_picker() -> void:
	if services == null:
		return
	var mm: MatchmakingService = services.matchmaking
	var m: ArenaModeModel = mm.mode(selected_mode)
	_picker.visible = m != null
	if m == null:
		_find.disabled = true
		return
	_picker.set_subjects(mm.subjects_of(selected_mode), mm.subject_name)
	var allowed: bool = m.allows_topic()
	if allowed and not _picker.subject_id.is_empty() and not mm.topics.has(_picker.subject_id):
		mm.load_topics(_picker.subject_id)
	_picker.set_topics(mm.topics.get(_picker.subject_id, []) as Array, allowed)
	_find.disabled = _picker.subject_id.is_empty() or mm.in_queue() or mm.is_busy()
	UiBuild.size_buttons(_pages[TAB_PLAY] as Node)


func _render_tier() -> void:
	if services == null:
		return
	_tier.show_tier(services.leaderboards.tier_for(selected_mode if not selected_mode.is_empty() else "battle_1v1"))


func _on_mode(mode: String) -> void:
	selected_mode = mode
	for c: Variant in _mode_cards.values():
		(c as ModeCard).refresh_tag()
	_render_picker()
	_render_tier()


func _on_find() -> void:
	_find.disabled = true
	var result: ApiResult = await services.matchmaking.find_match(selected_mode, _picker.subject_id, _picker.topic_id)
	if result.ok or result.code == MatchmakingService.CODE_ACTIVE and services.matchmaking.in_queue():
		close()
		return
	_render_picker()


func _on_duel() -> void:
	var mm: MatchmakingService = services.matchmaking
	_duel_dialog.open_send(mm, services.party, mm.subjects_of("battle_1v1"))


func _on_notice(key: String, args: Dictionary) -> void:
	if visible:
		ArenaUi.show_notice(_notice, key, args, "error" if key.begins_with("ui.arena.error.") else "info")


func _on_locale_changed(_locale: String) -> void:
	if visible and services != null:
		_render_modes()


func _build() -> void:
	_ui = ArenaUi.window(self, "ui.arena.title", "arena")
	(_ui["close"] as Button).pressed.connect(close)
	_tier = TierBadge.new()
	var bar_row: HBoxContainer = _ui["bar_row"] as HBoxContainer
	bar_row.add_child(_tier)
	bar_row.move_child(_tier, (_ui["close"] as Button).get_index())
	var box: VBoxContainer = _ui["box"] as VBoxContainer
	var tabs: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	tabs.name = "Tabs"
	box.add_child(UiBuild.chip_scroller(tabs))
	for t: Array in TABS:
		var b: Button = UiBuild.button(String(t[1]), String(t[2]), &"TabButton")
		b.name = "Tab_%s" % String(t[0])
		b.pressed.connect(select_tab.bind(String(t[0])))
		tabs.add_child(b)
		_tab_buttons[String(t[0])] = b
	_notice = ArenaUi.notice()
	box.add_child(_notice["panel"] as PanelContainer)
	var body: VBoxContainer = UiBuild.vbox()
	body.name = "Body"
	UiBuild.expand(body, true, true)
	box.add_child(body)
	_pages[TAB_PLAY] = _build_play()
	_leaderboard = LeaderboardView.new()
	_pages[TAB_LEADERBOARD] = _leaderboard
	_history = MatchHistoryView.new()
	_pages[TAB_HISTORY] = _history
	_tournaments = TournamentView.new()
	_tournaments.match_requested.connect(_on_challenge_match)
	_pages[TAB_TOURNAMENTS] = _tournaments
	for id: Variant in _pages.keys():
		body.add_child(_pages[id] as Control)
	_duel_dialog = DuelInviteDialog.new()
	add_child(_duel_dialog)


func _build_play() -> Control:
	var page: VBoxContainer = UiBuild.vbox(&"FormBox")
	page.name = "PlayPage"
	UiBuild.expand(page, true, true)
	# F125 polish: on the compact canvas the action buttons stand in a column right of the modes (a row under them would
	# cut the mode cards); a PC window keeps the row under the scroll box.
	_play_row = UiBuild.hbox(&"ToolbarBox")
	_play_row.name = "PlayRow"
	UiBuild.expand(_play_row, true, true)
	page.add_child(_play_row)
	var sb: Dictionary = ArenaUi.scroll_box()
	_play_row.add_child(sb["scroll"] as ScrollContainer)
	var box: VBoxContainer = sb["box"] as VBoxContainer
	var hint: Label = UiBuild.label("ui.arena.coach", &"InstructionLabel")
	hint.name = "Hint"
	box.add_child(hint)
	_modes_row = UiBuild.hbox(&"ToolbarBox")
	_modes_row.name = "Modes"
	box.add_child(UiBuild.chip_scroller(_modes_row))
	_empty_modes = UiBuild.label("ui.arena.modes.empty", &"MutedLabel")
	_empty_modes.visible = false
	box.add_child(_empty_modes)
	_picker = SubjectTopicPicker.new()
	_picker.selection_changed.connect(_render_picker)
	box.add_child(_picker)
	var buttons: HFlowContainer = HFlowContainer.new()
	buttons.name = "Actions"
	buttons.alignment = FlowContainer.ALIGNMENT_END
	page.add_child(buttons)
	_play_actions = buttons
	_duel = UiBuild.button("ui.arena.duel.open", "invite", &"SecondaryButton")
	_duel.name = "Duel"
	_duel.pressed.connect(_on_duel)
	buttons.add_child(_duel)
	_find = UiBuild.button("ui.arena.find_match", "search_light")
	_find.name = "FindMatch"
	_find.pressed.connect(_on_find)
	buttons.add_child(_find)
	_fit_play()
	return page


## F125 polish: the Play actions under the modes (PC) or in a column at their right, at the bottom (compact canvas).
func _fit_play() -> void:
	if _play_actions == null:
		return
	var side: bool = AppState.compact
	var target: Control = _play_row if side else _play_row.get_parent() as Control
	if _play_actions.get_parent() != target:
		_play_actions.get_parent().remove_child(_play_actions)
		target.add_child(_play_actions)
	_play_actions.vertical = side
	for b: Button in [_duel, _find]:
		b.size_flags_horizontal = Control.SIZE_FILL


func _on_challenge_match(match_id: String, mode: String) -> void:
	close()
	services.matchmaking.enter_match(match_id, mode)


func _unhandled_input(event: InputEvent) -> void:
	if not visible or not event.is_action_pressed("ui_cancel") or _duel_dialog.visible:
		return
	get_viewport().set_input_as_handled()
	close()
