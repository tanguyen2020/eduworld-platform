extends Control
## S06 (a) — Núi Thử Thách, choosing a solo challenge (F069 US3, STUDENT_GAME_UX_CORE S06; WorldRoot screen
## `challenge_board`): the server's list (`listGameChallenges`, type chips) with name, type and tries left; the selected
## challenge's ChallengeInfoCard (`getGameChallenge`: description, rules, time, stamina, first-clear rewards) and the
## DifficultySelector with ONLY the server's difficulties — a locked one appears only with the server's reason
## (F069-AC-14). No tries left → "Em đã dùng hết lượt hôm nay…" + practice hint; cooldown → a calm server-clock
## countdown and "Vào" locked (F069-AC-15); a party challenge has no solo button. "Vào" starts the run through
## ChallengeClient (F069-AC-16). A run in progress is offered first ("Tiếp tục", F069-AC-20). UI only (AGENTS R1).

signal closed()

const MULTIPLIER_BANDS: PackedStringArray = ["g6_7", "g8_9"]

var client: ChallengeClient = null
var band: String = ""
var type_filter: String = ""
var selected_id: String = ""
var detail: ChallengeDetailModel = null
var difficulty: String = ""
var resume_id: String = ""
var _ui: Dictionary = {}
var _types: FilterChips = null
var _list: VBoxContainer = null
var _info: VBoxContainer = null
## Difficulty, notes and "Vào" stay visible under the scrolling details (never below the fold).
var _actions: VBoxContainer = null
var _state: Label = null
var _resume: PanelContainer = null
var _difficulties: FilterChips = null
var _enter: Button = null
var _note: Label = null
var _cooldown: Label = null
var _practice: Button = null
var _timer: Timer = null
var _group: ButtonGroup = ButtonGroup.new()


func _ready() -> void:
	visible = false
	_ui = UiBuild.window(self, "ui.challenge.board_title", "area_challenge_mountain")
	(_ui["close"] as Button).pressed.connect(close)
	var body: VBoxContainer = _ui["body"] as VBoxContainer
	_resume = UiBuild.panel(&"InfoBannerPanel")
	_resume.name = "Resume"
	body.add_child(_resume)
	var resume_row: HBoxContainer = UiBuild.hbox()
	_resume.add_child(resume_row)
	var resume_label: Label = UiBuild.label("ui.challenge.resume_hint", &"InfoBannerLabel")
	UiBuild.expand(resume_label)
	resume_row.add_child(resume_label)
	var resume_button: Button = UiBuild.button("ui.challenge.resume", "next_light")
	resume_button.name = "ResumeButton"
	resume_button.pressed.connect(resume_run)
	resume_row.add_child(resume_button)
	_types = FilterChips.new()
	_types.name = "Types"
	_types.filter_changed.connect(func(code: String) -> void:
		type_filter = code
		refresh())
	body.add_child(UiBuild.chip_scroller(_types))
	_state = UiBuild.label("", &"MutedLabel")
	_state.name = "StateLabel"
	body.add_child(_state)
	var columns: HBoxContainer = UiBuild.hbox()
	UiBuild.expand(columns, true, true)
	body.add_child(columns)
	var list_scroll: ScrollContainer = ScrollContainer.new()
	list_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	list_scroll.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	list_scroll.size_flags_stretch_ratio = 0.8
	list_scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
	columns.add_child(list_scroll)
	_list = UiBuild.vbox()
	_list.name = "List"
	UiBuild.expand(_list)
	list_scroll.add_child(_list)
	var info_panel: PanelContainer = UiBuild.panel(&"QuestCard")
	info_panel.name = "ChallengeInfoCard"
	UiBuild.expand(info_panel, true, true)
	columns.add_child(info_panel)
	var info_column: VBoxContainer = UiBuild.vbox()
	info_panel.add_child(info_column)
	var info_scroll: ScrollContainer = ScrollContainer.new()
	info_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	UiBuild.expand(info_scroll, true, true)
	info_column.add_child(info_scroll)
	_actions = UiBuild.vbox(&"FieldBox")
	_actions.name = "Actions"
	info_column.add_child(_actions)
	_info = UiBuild.vbox(&"FieldBox")
	_info.name = "Info"
	UiBuild.expand(_info)
	info_scroll.add_child(_info)
	_timer = Timer.new()
	_timer.wait_time = 1.0
	_timer.timeout.connect(_render_cooldown)
	add_child(_timer)
	EventBus.locale_changed.connect(_on_locale_changed)


func _exit_tree() -> void:
	_bind(null)
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)


## WorldRoot: `ui_requested("challenge_board", {})`.
func open(_args: Dictionary = {}) -> void:
	_bind(AppState.get_services().challenge)
	band = UiBuild.band()
	selected_id = ""
	detail = null
	show()
	refresh()


func close() -> void:
	_timer.stop()
	hide()
	closed.emit()


## Coroutine: the list of the server (the old list of a previous grade is never kept).
func refresh() -> void:
	_state.text = "ui.challenge.loading"
	_state.visible = true
	var result: ApiResult = await client.list({"type": type_filter})
	if not result.ok:
		_state.text = L10n.error_text(result.code)
		_state.visible = true
	else:
		_state.text = "ui.challenge.empty" if client.challenges.is_empty() else ""
		_state.visible = client.challenges.is_empty()
	if type_filter.is_empty():
		var options: Array[Dictionary] = [{"code": "", "key": "ui.inventory.filter.all"}]
		for s: ChallengeSummaryModel in client.challenges:
			var known: bool = false
			for o: Dictionary in options:
				known = known or String(o["code"]) == s.challenge_type
			if not known:
				options.append({"code": s.challenge_type, "key": s.type_key()})
		_types.set_options(options, "")
	_render_list()
	if selected_id.is_empty() and not client.challenges.is_empty():
		await select(client.challenges[0].challenge_id)
	_render_resume()


## Coroutine: shows the info card of `challenge_id` (`getGameChallenge`).
func select(challenge_id: String) -> void:
	selected_id = challenge_id
	for b: Node in _list.get_children():
		(b as Button).set_pressed_no_signal(String(b.get_meta(&"id", "")) == challenge_id)
	detail = await client.get_challenge(challenge_id)
	difficulty = ""
	if detail != null:
		for o: Dictionary in detail.difficulty_options():
			if not bool(o["locked"]):
				difficulty = String(o["difficulty"])
				break
	_render_info()


## Coroutine: "Vào" (F069-AC-16).
func enter() -> void:
	if detail == null or difficulty.is_empty() or not client.can_write():
		return
	_enter.disabled = true
	var result: ApiResult = await client.start(detail.challenge_id, difficulty)
	if result.ok:
		if visible:
			close()
	elif is_inside_tree():
		_enter.disabled = false


## Coroutine: continues the run in progress (F069-AC-20).
func resume_run() -> void:
	var id: String = resume_id if not resume_id.is_empty() else (client.attempt().challenge_attempt_id if client.attempt() != null else "")
	if id.is_empty():
		return
	if client.attempt() == null or client.attempt().challenge_attempt_id != id:
		await client.resume(id)
	if client.is_running():
		close()
		await client.continue_run()


func difficulty_codes() -> PackedStringArray:
	var out: PackedStringArray = []
	if _difficulties != null:
		for b: Node in _difficulties.get_children():
			out.append(String(b.get_meta(&"code")))
	return out


func locked_codes() -> PackedStringArray:
	var out: PackedStringArray = []
	if _difficulties != null:
		for b: Node in _difficulties.get_children():
			if (b as Button).disabled:
				out.append(String(b.get_meta(&"code")))
	return out


func enter_button() -> Button:
	return _enter


func note_text() -> String:
	return _note.text if _note != null and _note.visible else ""


func cooldown_text() -> String:
	return _cooldown.text if _cooldown != null and _cooldown.visible else ""


func info_texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for l: Node in _info.find_children("*", "Label", true, false) + _actions.find_children("*", "Label", true, false):
		out.append((l as Label).text)
	return out


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed("ui_cancel"):
		close()
		get_viewport().set_input_as_handled()


func _bind(value: ChallengeClient) -> void:
	if client != null:
		if client.notice_requested.is_connected(_on_notice):
			client.notice_requested.disconnect(_on_notice)
			client.resume_offered.disconnect(_on_resume_offered)
	client = value
	if client != null:
		client.notice_requested.connect(_on_notice)
		client.resume_offered.connect(_on_resume_offered)


func _render_list() -> void:
	UiBuild.clear(_list)
	for s: ChallengeSummaryModel in client.challenges:
		var b: Button = UiBuild.button("", "area_challenge_mountain", &"ChoiceChip")
		b.name = "Challenge_%s" % s.challenge_id.left(8)
		b.set_meta(&"id", s.challenge_id)
		b.toggle_mode = true
		b.button_group = _group
		b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		b.alignment = HORIZONTAL_ALIGNMENT_LEFT
		b.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		var lines: PackedStringArray = [s.display_title(), tr(s.type_key())]
		if s.attempts_left >= 0:
			lines.append(tr("ui.challenge.attempts_left").format({"n": LocaleFormat.number(s.attempts_left)}))
		b.text = "\n".join(lines)
		b.custom_minimum_size.y = UiBuild.target() * 1.6
		b.set_pressed_no_signal(s.challenge_id == selected_id)
		b.pressed.connect(func() -> void: select(s.challenge_id))
		_list.add_child(b)


func _render_info() -> void:
	UiBuild.clear(_info)
	UiBuild.clear(_actions)
	_difficulties = null
	_enter = null
	_note = null
	_cooldown = null
	_timer.stop()
	if detail == null:
		return
	_info.add_child(UiBuild.data_label(detail.display_title(), &"HeadingLabel"))
	var type_row: HBoxContainer = UiBuild.hbox()
	type_row.add_child(UiBuild.icon_rect("flag", 0.75))
	type_row.add_child(UiBuild.label(detail.type_key(), &"MutedLabel", false))
	_info.add_child(type_row)
	if not detail.description_text().is_empty():
		_info.add_child(UiBuild.data_label(detail.description_text()))
	if not detail.rules_text().is_empty():
		_info.add_child(UiBuild.data_label(detail.rules_text(), &"MutedLabel"))
	var facts: HFlowContainer = HFlowContainer.new()
	_info.add_child(facts)
	if detail.time_limit_seconds > 0:
		facts.add_child(_fact("clock", tr("ui.challenge.time_limit").format({"time": _duration(detail.time_limit_seconds)})))
	if detail.lives > 0:
		facts.add_child(_fact("heart", tr("ui.challenge.lives").format({"n": LocaleFormat.number(detail.lives)})))
	if detail.attempts_left >= 0:
		facts.add_child(_fact("play", tr("ui.challenge.attempts_left").format({"n": LocaleFormat.number(detail.attempts_left)})))
	if not detail.first_clear_rewards.is_empty():
		_info.add_child(UiBuild.label("ui.challenge.first_clear", &"MutedLabel"))
		var rewards: RewardRow = RewardRow.new()
		_info.add_child(rewards)
		rewards.set_rewards(detail.first_clear_rewards, band == "g1_3")
	if not detail.can_play_solo():
		_note = UiBuild.label("ui.challenge.party_only", &"InfoBannerLabel")
		_actions.add_child(_note)
		return
	var difficulty_title: Label = UiBuild.label("ui.challenge.difficulty_title", &"HeadingLabel")
	difficulty_title.name = "DifficultyTitle"
	_actions.add_child(difficulty_title)
	_difficulties = FilterChips.new()
	_difficulties.name = "DifficultySelector"
	_actions.add_child(_difficulties)
	var options: Array[Dictionary] = []
	var reasons: PackedStringArray = []
	for o: Dictionary in detail.difficulty_options():
		var level: String = String(o["difficulty"])
		options.append({"code": level, "key": "ui.challenge.difficulty." + level,
			"icon": "lock" if bool(o["locked"]) else ""})
		if bool(o["locked"]):
			reasons.append("%s: %s" % [tr("ui.challenge.difficulty." + level), tr(String(o["reason_key"]))])
	_difficulties.set_options(options, difficulty)
	for o: Dictionary in detail.difficulty_options():
		if bool(o["locked"]):
			var chip: Button = _difficulties.chip(String(o["difficulty"]))
			chip.disabled = true
			chip.tooltip_text = String(o["reason_key"])
	_difficulties.filter_changed.connect(func(code: String) -> void:
		difficulty = code
		_render_multiplier())
	if not reasons.is_empty():
		_actions.add_child(UiBuild.data_label("\n".join(reasons), &"MutedLabel"))
	var multiplier: Label = UiBuild.data_label("", &"MutedLabel")
	multiplier.name = "Multiplier"
	_actions.add_child(multiplier)
	_note = UiBuild.data_label("", &"InfoBannerLabel")
	_note.name = "Note"
	_actions.add_child(_note)
	_cooldown = UiBuild.data_label("", &"InfoBannerLabel")
	_cooldown.name = "Cooldown"
	_actions.add_child(_cooldown)
	var actions: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	actions.alignment = BoxContainer.ALIGNMENT_END
	_actions.add_child(actions)
	_practice = UiBuild.button("ui.challenge.practice", "map", &"SecondaryButton")
	_practice.name = "Practice"
	_practice.pressed.connect(func() -> void: EventBus.ui_requested.emit(&"world_map", {}))
	actions.add_child(_practice)
	_enter = UiBuild.button("ui.challenge.enter", "next_light")
	_enter.name = "Enter"
	_enter.pressed.connect(enter)
	actions.add_child(_enter)
	UiBuild.size_buttons(_actions)
	_render_multiplier()
	_render_cooldown()
	if client.cooldown_left_ms(detail) > 0:
		_timer.start()


func _render_multiplier() -> void:
	var label: Label = _actions.find_child("Multiplier", true, false) as Label
	if label == null:
		return
	var m: float = detail.multiplier_of(difficulty) if detail != null else 0.0
	label.visible = band in MULTIPLIER_BANDS and m > 0.0
	label.text = tr("ui.challenge.multiplier").format({"x": LocaleFormat.number(m, "", 1)}) if label.visible else ""


func _render_cooldown() -> void:
	if _enter == null or detail == null:
		return
	var left: int = client.cooldown_left_ms(detail)
	var no_tries: bool = detail.attempts_left == 0
	_note.visible = no_tries
	_note.text = tr("ui.challenge.no_attempts") if no_tries else ""
	# No try left today: the difficulty choice is pointless — the kind message and "Luyện tập" take its place.
	_difficulties.visible = not no_tries
	var title: Node = _actions.find_child("DifficultyTitle", true, false)
	if title != null:
		(title as Control).visible = not no_tries
	_practice.visible = no_tries or left > 0
	_cooldown.visible = left > 0
	_cooldown.text = tr("ui.challenge.cooldown").format({"time": ExamCountdown.clock_text(left)}) if left > 0 else ""
	_enter.disabled = no_tries or left > 0 or difficulty.is_empty() or not client.can_write()
	if left <= 0:
		_timer.stop()


func _render_resume() -> void:
	_resume.visible = client.is_running() or not resume_id.is_empty()


func _fact(icon_name: String, text: String) -> Control:
	var h: HBoxContainer = UiBuild.hbox()
	h.add_child(UiBuild.icon_rect(icon_name, 0.75))
	h.add_child(UiBuild.data_label(text, &"", false))
	return h


func _on_notice(key: String, args: Dictionary) -> void:
	if not visible:
		return
	if _note != null:
		_note.text = tr(key).format(args)
		_note.visible = true
	else:
		_state.text = tr(key).format(args)
		_state.visible = true


func _on_resume_offered(challenge_attempt_id: String) -> void:
	resume_id = challenge_attempt_id
	_render_resume()


func _on_locale_changed(_locale: String) -> void:
	if visible:
		_render_list()
		_render_info()


## "3 phút" for whole minutes (kids read it faster than "3 phút 0 giây"); the locale duration otherwise.
static func _duration(seconds: int) -> String:
	if seconds >= 60 and seconds % 60 == 0:
		return TranslationServer.translate("ui.challenge.minutes").format({"m": LocaleFormat.number(seconds / 60)})
	return LocaleFormat.duration(seconds)
