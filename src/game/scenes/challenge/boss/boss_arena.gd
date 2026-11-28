class_name BossArena
extends Control
## S06 (b) — the boss screen, solo and party (F110 US1; STUDENT_GAME_UX_CORE S06, CHALLENGE_BOSS_SYSTEM §3,
## PERFORMANCE_BUDGET §7). Opened by the boss card ("Chơi một mình" → `{encounter_id, mode: solo}`) or by the party
## session (`party.session.started` `room_type = boss` → SceneRouter.enter_boss_encounter → `{encounter_id, mode: party}`):
## - a light 2D arena over the suspended 3D world (QualityManager.suspend_world), a fixed stage — no camera, no joystick
##   on a phone — the drawn boss (BossFigure, particles ≤ 200 and none at Low) and the question panel as Controls
##   (F110-AC-11);
## - the HUD: boss name, BossEnergyBar with the server %, "Phase n/total", PhaseTimer on the server clock (lớp 1–3 a bar
##   without seconds), "Rời" behind a confirmation (F110-AC-03, AC-10);
## - PartyReadyList in the lobby (✓ / clock), ContributionList during a phase (✓ "Đã đạt phần" or the clock, the
##   student's own right answers only — no ranking, AC-02, AC-06), faded members with words while reconnecting (AC-09);
## - QuestionPanelHost with the F063 renderer, locked while the answer is on its way or the connection is gone (AC-04,
##   AC-09); the student's own result only: light rays on a right answer, a playful "strike back" + the explanation on a
##   wrong one — never the word "Sai" (AC-05); "Thử lại phase với câu hỏi mới" on a retry, a phase transition ≤ 2 s with
##   the boss line (AC-07); BossResult at the end (AC-08);
## - the always-on party chat (F119 ChatDock, channel `party`) behind "Chat nhóm" in a party boss.
## UI only: BossClient / BossStore do the work (AGENTS R1); everything shown is the server's (R5).

signal closed()
## Back to the Hub after the encounter was gone (`ROOM_NOT_MEMBER` / `404`), through WorldRoot.
signal travel_requested(area_id: String)

const SUSPEND_REASON: StringName = &"boss"
const HUB_AREA_ID: String = "main_hub"
const PARTY_SCREEN: StringName = &"party"
const RETRY_BANNER_SEC: float = 5.0
const LINE_SEC: float = 2.0
const ACT_LEAVE: StringName = &"leave"

## Holds the break reminder while a boss runs (WorldRoot reads it).
var blocks_break_reminder: bool = true
var services: ServiceLocator = null
var client: BossClient = null
var encounter_id: String = ""
var mode: String = BossSummary.MODE_SOLO
var boss_title: String = ""
## `party.session.ended` arrived: the result goes back to the lobby.
var party_ended: bool = false
var _title: Label = null
var _energy: BossEnergyBar = null
var _phase: Label = null
var _timer: PhaseTimer = null
var _leave: Button = null
var _chat_button: Button = null
var _members: ContributionList = null
var _banner: Dictionary = {}
var _figure: BossFigure = null
var _line: Label = null
var _line_panel: PanelContainer = null
var _feedback: PanelContainer = null
var _feedback_label: Label = null
var _feedback_icon: TextureRect = null
var _panel: QuestionPanelHost = null
var _chat_column: VBoxContainer = null
var _chat_dock: ChatDock = null
var _confirm: ChatSheet = null
var _result: BossResult = null
var _banner_left: float = 0.0
var _line_left: float = 0.0
var _banner_key: String = ""
var _exit_target: String = ""


func _ready() -> void:
	visible = false
	_build()
	AppState.compact_changed.connect(func(_c: bool) -> void: _fit())
	EventBus.locale_changed.connect(func(_l: String) -> void:
		if visible:
			render())


func _exit_tree() -> void:
	_unbind()
	if visible:
		QualityManager.resume_world(SUSPEND_REASON)


## WorldRoot: `ui_requested("boss_arena", {encounter_id, mode, title?})`.
func open(args: Dictionary = {}) -> void:
	if services == null and AppState.services != null:
		services = AppState.get_services()
	encounter_id = String(args.get("encounter_id", ""))
	mode = BossSummary.MODE_PARTY if String(args.get("mode", "")) == BossSummary.MODE_PARTY else BossSummary.MODE_SOLO
	party_ended = false
	_exit_target = ""
	_bind(services.boss if services != null else null)
	show()
	QualityManager.suspend_world(SUSPEND_REASON)
	_result.hide()
	_confirm.dismiss()
	_feedback.visible = false
	_line_panel.visible = false
	_set_banner("")
	boss_title = String(args.get("title", ""))
	_fit()
	_open_chat()
	if client != null and Uuid.is_valid(encounter_id):
		await client.enter(encounter_id)
	render()


func close() -> void:
	_panel.clear()
	_confirm.dismiss()
	if _chat_dock != null:
		_chat_dock.clear_context()
	if client != null:
		client.store.clear()
	_unbind()
	hide()
	QualityManager.resume_world(SUSPEND_REASON)
	closed.emit()


# ---------- test hooks ----------

func energy_bar() -> BossEnergyBar:
	return _energy


func phase_text() -> String:
	return _phase.text


func timer() -> PhaseTimer:
	return _timer


func members() -> ContributionList:
	return _members


func question_panel() -> QuestionPanelHost:
	return _panel


func figure() -> BossFigure:
	return _figure


func result_view() -> BossResult:
	return _result


func confirm_sheet() -> ChatSheet:
	return _confirm


func leave_button() -> Button:
	return _leave


func chat_button() -> Button:
	return _chat_button


func chat_dock() -> ChatDock:
	return _chat_dock


func banner_text() -> String:
	return (_banner["label"] as Label).text if (_banner["panel"] as PanelContainer).visible else ""


func feedback_text() -> String:
	return _feedback_label.text if _feedback.visible else ""


func line_text() -> String:
	return _line.text if _line_panel.visible or (AppState.compact and _banner_key == _line.text) else ""


# ---------- binding ----------

func _bind(next: BossClient) -> void:
	_unbind()
	client = next
	if client == null:
		return
	for pair: Array in _pairs():
		(pair[0] as Signal).connect(pair[1] as Callable)


func _unbind() -> void:
	if client == null:
		return
	for pair: Array in _pairs():
		if (pair[0] as Signal).is_connected(pair[1] as Callable):
			(pair[0] as Signal).disconnect(pair[1] as Callable)
	client = null


func _pairs() -> Array:
	var s: BossStore = client.store
	var out: Array = [[s.encounter_changed, _on_encounter_changed], [s.question_changed, _on_question_changed],
		[s.answer_evaluated, _on_answer_evaluated], [s.phase_started, _on_phase_started],
		[s.phase_completed, _on_phase_completed], [s.encounter_ended, _on_encounter_ended],
		[client.notice_requested, _on_notice], [client.exit_requested, _on_exit_requested],
		[client.connection_changed, _on_connection_changed]]
	if services != null and services.party != null:
		out.append([services.party.store.session_ended, _on_party_session_ended])
	return out


# ---------- per frame ----------

func advance(delta: float) -> void:
	if client == null:
		return
	var e: BossEncounterModel = client.store.encounter
	_timer.render(client.phase_left_ms() if e != null and not e.is_over() else -1, UiBuild.band(),
			"%d:%s" % [e.current_phase, e.phase_id] if e != null else "")
	if _banner_left > 0.0:
		_banner_left -= delta
		if _banner_left <= 0.0:
			_set_banner("")
			_render_banner()
	if _line_left > 0.0:
		_line_left -= delta
		if _line_left <= 0.0:
			_line_panel.visible = false


func _process(delta: float) -> void:
	if visible:
		advance(delta)


func _unhandled_input(event: InputEvent) -> void:
	if not visible or _result.visible or _confirm.visible:
		return
	if event.is_action_pressed("ui_cancel"):
		get_viewport().set_input_as_handled()
		ask_leave()
		return
	if event is InputEventKey and (event as InputEventKey).pressed and not (event as InputEventKey).echo:
		var k: Key = (event as InputEventKey).keycode
		if k >= KEY_1 and k <= KEY_4 and not _panel.is_locked():
			_panel.choose(int(k - KEY_1))
			get_viewport().set_input_as_handled()


# ---------- rendering ----------

func render() -> void:
	if client == null:
		return
	var e: BossEncounterModel = client.store.encounter
	_title.text = _boss_title()
	UiBuild.fit_words(_title)
	_energy.show_energy(e.energy_remaining_pct if e != null else 100.0)
	_phase.text = tr("ui.boss.phase").format({"n": LocaleFormat.number(e.phase_number() if e != null else 1),
		"total": LocaleFormat.number(e.phase_count if e != null else 1)})
	_figure.set_phase(e.current_phase if e != null else 0)
	_figure.set_defeated(e != null and e.outcome == BossEncounterModel.OUTCOME_DEFEATED)
	var lobby: bool = e == null or e.status == BossEncounterModel.STATUS_LOBBY
	if e != null:
		_members.show_members(e.members, _self_id(), e.my_correct_count, lobby, e.is_party())
	_members.visible = e != null and (e.is_party() or not lobby)
	_render_banner()
	_render_question()


func _render_question() -> void:
	if client == null:
		return
	var e: BossEncounterModel = client.store.encounter
	var q: BossQuestionModel = client.store.current_question()
	if e == null or e.is_over():
		_panel.clear("")
		return
	if q == null:
		var key: String = "ui.boss.waiting_question"
		if e.status == BossEncounterModel.STATUS_LOBBY:
			key = "ui.boss.waiting_party" if e.is_party() else "ui.boss.get_ready"
		elif e.status == BossEncounterModel.STATUS_TRANSITION:
			key = "ui.boss.next_phase_soon"
		_panel.clear(key)
		return
	_panel.show_question(q, _context())
	# F110-AC-09: no answer can be sent while the connection is gone.
	if client.is_offline():
		_panel.set_sending(true)


func _render_banner() -> void:
	if client != null and client.is_offline():
		ArenaUi.show_notice(_banner, "ui.boss.reconnecting", {}, "warning")
		return
	if not _banner_key.is_empty():
		ArenaUi.show_notice(_banner, _banner_key, {}, "info")
		return
	ArenaUi.show_notice(_banner, "")


func _set_banner(key: String, seconds: float = RETRY_BANNER_SEC) -> void:
	_banner_key = key
	_banner_left = seconds if not key.is_empty() else 0.0


func _boss_title() -> String:
	if not boss_title.is_empty():
		return boss_title
	var e: BossEncounterModel = client.store.encounter if client != null else null
	var b: BossSummary = client.boss(e.boss_id) if e != null else null
	if b != null:
		return b.display_title()
	var party: PartyModel = services.party.store.party if services != null and services.party != null else null
	if party != null and not party.activity_title.is_empty():
		return L10n.pick(party.activity_title)
	return tr("ui.boss.default_title")


func _context() -> RendererContext:
	var ctx: RendererContext = RendererContext.new()
	ctx.grade_band = UiBuild.band()
	ctx.content_language = L10n.data_language(TranslationServer.get_locale())
	if services != null:
		ctx.math = services.math_svg
		ctx.media = services.media
	return ctx


func _self_id() -> String:
	return CompetitionPlayer.self_id(AppState)


# ---------- events ----------

func _on_encounter_changed(_e: BossEncounterModel) -> void:
	render()


func _on_question_changed(_q: BossQuestionModel) -> void:
	_render_question()


func _on_answer_evaluated(result: Dictionary) -> void:
	var correct: bool = bool(result.get("is_correct", false))
	_feedback.theme_type_variation = &"FeedbackCorrectPanel" if correct else &"InfoBannerPanel"
	_feedback_label.theme_type_variation = &"FeedbackLabel" if correct else &"InfoBannerLabel"
	_feedback_icon.texture = UiBuild.icon("emoji_sparkles" if correct else "emoji_thinking")
	var text: String = tr("ui.boss.hit") if correct else tr("ui.boss.strike_back")
	var explanation: String = L10n.pick(result.get("explanation", {}) as Dictionary)
	if not correct and not explanation.is_empty():
		text += "\n" + explanation
	_feedback_label.text = text
	_feedback.visible = true
	if correct:
		_figure.hit()
	else:
		_figure.strike_back()
	render()


func _on_phase_started(retry: bool, _presentation_ref: String) -> void:
	_feedback.visible = false
	if retry:
		_set_banner("ui.boss.retry_phase")
	_show_line("ui.boss.line.phase_start")
	render()


func _on_phase_completed(phase_index: int) -> void:
	_feedback.visible = false
	_figure.transform_phase(phase_index + 1)
	_show_line("ui.boss.line.phase_cleared")
	render()


func _on_encounter_ended(_outcome: String, _rewards: Array) -> void:
	_panel.clear("")
	_set_banner("")
	render()
	_result.show_result(client.store.result, _boss_title(), mode == BossSummary.MODE_PARTY)


func _on_notice(key: String, args: Dictionary) -> void:
	if key == "ui.boss.reconnecting":
		_render_banner()
		return
	ArenaUi.show_notice(_banner, key, args, "info")
	_banner_key = key
	_banner_left = RETRY_BANNER_SEC


func _on_connection_changed(_offline: bool) -> void:
	_render_banner()
	_render_question()


## `hub`: the encounter is gone (`ROOM_NOT_MEMBER` / `404`) — back to the Hub with a general sentence; `zone` (solo
## left) — the student is in the zone of the gate; `lobby` — the party lobby.
func _on_exit_requested(target: String, notice_key: String) -> void:
	if target == BossClient.EXIT_HUB:
		# The general sentence in the result window; its button goes to the Hub (no detail of why, F110-AC-30).
		_panel.clear("")
		_exit_target = target
		_result.show_message(notice_key if not notice_key.is_empty() else BossClient.KEY_GONE)
		return
	var party: bool = mode == BossSummary.MODE_PARTY
	close()
	if target == BossClient.EXIT_LOBBY or party:
		EventBus.ui_requested.emit(PARTY_SCREEN, {})


func _on_party_session_ended(_info: Dictionary) -> void:
	party_ended = true
	if visible and not _result.visible:
		close()
		EventBus.ui_requested.emit(PARTY_SCREEN, {})


func _on_result_done() -> void:
	var party: bool = mode == BossSummary.MODE_PARTY
	var target: String = _exit_target
	_exit_target = ""
	close()
	if target == BossClient.EXIT_HUB:
		travel_requested.emit(HUB_AREA_ID)
	elif party:
		EventBus.ui_requested.emit(PARTY_SCREEN, {})


## The boss line (≤ 2 s): a speech bubble above the boss; on the compact phone canvas the banner says it (the narrow
## boss column would break the words), unless a banner of the server (retry, reconnecting) is on.
func _show_line(key: String) -> void:
	_line.text = tr(key).format({"boss": _boss_title()})
	_line_left = LINE_SEC
	if not AppState.compact:
		_line_panel.visible = true
	elif _banner_key.is_empty() and (client == null or not client.is_offline()):
		ArenaUi.show_notice(_banner, _line.text, {}, "info")
		_banner_key = _line.text
		_banner_left = LINE_SEC


# ---------- leave ----------

## "Rời" (or Esc): one confirmation, then `leaveBossEncounter` (F110-AC-10).
func ask_leave() -> void:
	var body: VBoxContainer = _confirm.open_sheet(TranslationServer.translate("ui.boss.leave_confirm_title"), true)
	body.add_child(UiBuild.keep_words(UiBuild.label("ui.boss.leave_confirm")))
	_confirm.add_button(ChatSheet.CANCEL, "ui.boss.stay", "play")
	_confirm.add_button(ACT_LEAVE, "ui.boss.leave", "exit", &"DangerButton")


func _on_confirm_chosen(action: StringName) -> void:
	if action == ACT_LEAVE and client != null:
		await client.leave()


# ---------- party chat (F119) ----------

func _open_chat() -> void:
	var party: PartyModel = services.party.store.party if services != null and services.party != null else null
	var has_chat: bool = mode == BossSummary.MODE_PARTY and party != null and Uuid.is_valid(party.chat_channel_id)
	_chat_button.visible = has_chat
	_chat_column.visible = false
	if not has_chat:
		_chat_dock.clear_context()
		return
	var others: Array[Dictionary] = []
	for m: PartyMemberModel in party.members:
		if m.student_id != _self_id():
			others.append({"user_id": m.student_id, "nickname": m.nickname, "avatar_config": m.avatar_config})
	_chat_dock.show_party(party.party_id, party.chat_channel_id, others)


func toggle_chat() -> void:
	_chat_column.visible = not _chat_column.visible
	if _chat_column.visible:
		_chat_dock.expand()


# ---------- build ----------

func _fit() -> void:
	if _figure == null:
		return
	# The boss is as wide as its column and as tall as the stage leaves (never pushing the board off the view).
	var side: float = float(UiTokens.COMPONENT_COACH_SIZE if AppState.compact else UiTokens.COMPONENT_MASCOT_SIZE)
	_figure.custom_minimum_size = Vector2(side, float(UiTokens.COMPONENT_COACH_SIZE) * 0.8)
	_figure.size_flags_vertical = Control.SIZE_EXPAND_FILL
	_chat_column.custom_minimum_size.x = float(UiTokens.SIZE_LAYOUT_280)
	UiBuild.size_buttons(self)


func _build() -> void:
	set_anchors_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	var sky: SkyBackdrop = SkyBackdrop.new()
	sky.name = "Backdrop"
	sky.set_anchors_preset(Control.PRESET_FULL_RECT)
	add_child(sky)
	var margin: MarginContainer = MarginContainer.new()
	margin.theme_type_variation = &"ScreenMargin"
	margin.set_anchors_preset(Control.PRESET_FULL_RECT)
	add_child(margin)
	# F125: on the compact phone canvas a long translation or a reconnect banner can make the column taller than the
	# screen; it then scrolls as a whole (when it fits, it fills the height and the board keeps its room).
	var page: ScrollContainer = ScrollContainer.new()
	page.name = "Page"
	page.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	# No bar: a bar that comes and goes would change the width, re-wrap the title and loop (dragging still scrolls).
	page.vertical_scroll_mode = ScrollContainer.SCROLL_MODE_SHOW_NEVER
	margin.add_child(page)
	var main: VBoxContainer = UiBuild.vbox(&"FormBox")
	main.name = "Main"
	UiBuild.expand(main, true, true)
	page.add_child(main)
	main.add_child(_build_hud())
	_banner = ArenaUi.notice()
	(_banner["panel"] as PanelContainer).name = "Banner"
	main.add_child(_banner["panel"] as PanelContainer)
	var stage: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	stage.name = "Stage"
	UiBuild.expand(stage, true, true)
	main.add_child(stage)
	var boss_side: VBoxContainer = UiBuild.vbox(&"FormBox")
	boss_side.name = "BossSide"
	boss_side.alignment = BoxContainer.ALIGNMENT_CENTER
	stage.add_child(boss_side)
	_line_panel = UiBuild.panel(&"SpeechBubble")
	_line_panel.name = "BossLine"
	_line_panel.visible = false
	_line = UiBuild.keep_words(UiBuild.data_label("", &"SpeechBubbleLabel"))
	_line.custom_minimum_size.x = float(UiTokens.COMPONENT_COACH_SIZE)
	_line_panel.add_child(_line)
	boss_side.add_child(_line_panel)
	_figure = BossFigure.new()
	_figure.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	boss_side.add_child(_figure)
	var board: PanelContainer = UiBuild.panel(&"ActivityBoard")
	board.name = "Board"
	UiBuild.expand(board, true, true)
	stage.add_child(board)
	var scroll: ScrollContainer = ScrollContainer.new()
	scroll.name = "BoardScroll"
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	UiBuild.expand(scroll, true, true)
	board.add_child(scroll)
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	UiBuild.expand(box, true, true)
	scroll.add_child(box)
	_feedback = UiBuild.panel(&"FeedbackCorrectPanel")
	_feedback.name = "Feedback"
	_feedback.visible = false
	var frow: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	_feedback.add_child(frow)
	_feedback_icon = UiBuild.icon_rect("emoji_sparkles")
	_feedback_icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	frow.add_child(_feedback_icon)
	_feedback_label = UiBuild.keep_words(UiBuild.data_label("", &"FeedbackLabel"))
	UiBuild.expand(_feedback_label)
	frow.add_child(_feedback_label)
	box.add_child(_feedback)
	_panel = QuestionPanelHost.new()
	UiBuild.expand(_panel, true, true)
	_panel.answer_ready.connect(_on_answer_ready)
	box.add_child(_panel)
	_chat_column = UiBuild.vbox()
	_chat_column.name = "ChatColumn"
	_chat_column.visible = false
	_chat_column.size_flags_vertical = Control.SIZE_EXPAND_FILL
	stage.add_child(_chat_column)
	_chat_dock = (load("res://scenes/chat/ChatDock.tscn") as PackedScene).instantiate() as ChatDock
	UiBuild.expand(_chat_dock, true, true)
	_chat_column.add_child(_chat_dock)
	if AppState.services != null:
		var s: ServiceLocator = AppState.get_services()
		_chat_dock.bind_services(s.chat, s.social, s.reports)
	_confirm = ChatSheet.new()
	_confirm.name = "LeaveConfirm"
	_confirm.chosen.connect(_on_confirm_chosen)
	add_child(_confirm)
	_result = BossResult.new()
	_result.done.connect(_on_result_done)
	add_child(_result)
	_result.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	_fit()


func _build_hud() -> Control:
	var hud: PanelContainer = UiBuild.panel(&"HudPanel")
	hud.name = "BossHud"
	var col: VBoxContainer = UiBuild.vbox()
	hud.add_child(col)
	# Row 1: the boss name, phase, timer and the buttons; row 2: the energy across the width; row 3: the members.
	# A flow: on a phone with a long translation the buttons go to a second line instead of out of the view.
	var row: HFlowContainer = HFlowContainer.new()
	row.name = "Top"
	col.add_child(row)
	var fire: TextureRect = UiBuild.icon_rect("fire")
	fire.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(fire)
	_title = UiBuild.keep_words(UiBuild.data_label("", &"HudLabel"))
	_title.name = "BossName"
	_title.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(_title)
	row.add_child(_title)
	_energy = BossEnergyBar.new()
	UiBuild.expand(_energy)
	col.add_child(_energy)
	_phase = UiBuild.data_label("", &"HudLabel", false)
	_phase.name = "PhaseIndicator"
	_phase.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(_phase)
	_timer = PhaseTimer.new()
	row.add_child(_timer)
	_chat_button = UiBuild.button("ui.boss.chat", "chat", &"SecondaryButton")
	_chat_button.name = "ChatButton"
	_chat_button.visible = false
	_chat_button.pressed.connect(toggle_chat)
	row.add_child(_chat_button)
	_leave = UiBuild.button("ui.boss.leave", "exit", &"SecondaryButton")
	_leave.name = "Leave"
	_leave.pressed.connect(ask_leave)
	row.add_child(_leave)
	_members = ContributionList.new()
	col.add_child(_members)
	return hud


## The feedback of the previous answer stays (time to read the explanation) until the student answers again.
func _on_answer_ready(response: Dictionary) -> void:
	_feedback.visible = false
	if client != null:
		await client.submit(response)
