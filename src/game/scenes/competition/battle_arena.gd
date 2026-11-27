class_name BattleArena
extends Control
## The match screen (F109 US2/US3; STUDENT_GAME_UX_SOCIAL §8 wireframes, GAME_NETWORKING §7, PERFORMANCE_BUDGET §7):
## - no 3D world behind (`QualityManager.suspend_world("battle")`, a static sky backdrop + the cream board of the
##   activity player), updates by events only (F109-AC-12);
## - MatchHeader (sides, server scores, "Câu 2/3", the time bar — seconds only for lớp 4–9), the question drawn by the
##   F063 renderer of its type (one tap on a tile = the answer for one-step questions; keys 1–4 on a PC; "Gửi đáp án"
##   for the others), "Đã ghi nhận" at once, ✓ / ✗ + points only from the server (AC-05…AC-09);
## - the always-on match chat (F119 ChatDock on the server's `chat_channel_ids` {match, team?}, no option to turn it
##   off): the right column, shrunk to its 2-line strip while a question is on screen (`EventBus.question_shown` / `question_hidden`)
##   so it never covers the answers (AC-13);
## - banners "Đối thủ đang kết nối lại…" (until the server `grace_until`), "Em đang kết nối lại…"; on the compact phone
##   canvas while a question shows, the other player's banner folds into the wifi sign on their card (its sentence as
##   tooltip + accessible name) so every answer stays in view (F125 polish); the pause overlay;
##   the start countdown 3-2-1 from the server `starts_at` (AC-11) with "Trả lời đúng và nhanh…" only when the server
##   says `speed_bonus_enabled` (AC-09);
## - MatchResultView at the end (AC-14…AC-16); on the compact phone canvas it takes the whole column (the match header —
##   "Câu 1/3", the stale time bar — hides) so the outcome and the score are read at once above the buttons (F125 polish).
## UI only: BattleClient / MatchmakingService / LeaderboardService / MatchHistoryService do the work (R1).

signal closed()

const SUSPEND_REASON: StringName = &"battle"
const BLOCKING_ACTIVITY: StringName = &"battle"
## Share of the screen width of the chat column.
const CHAT_SHARE: float = 0.38
## Largest share of the width the chat column may take (its own minimum width at lớp 1–3 or with a long translation can
## be larger than CHAT_SHARE; the question side keeps the rest).
const CHAT_SHARE_MAX: float = 0.6
## One-step question types answered by a single tap (F098 tap-to-answer).
const INSTANT_TYPES: PackedStringArray = ["single_choice", "true_false"]

## Holds the break reminder while a match runs (WorldRoot reads it).
var blocks_break_reminder: bool = true
var services: ServiceLocator = null
var battle: BattleClient = null
var question: QuestionRendererBase = null
var match_id: String = ""
var mode: String = ""
var _question_on: bool = false
var _rendered_instance: String = ""
var _header: MatchHeader = null
var _banner: Dictionary = {}
var _status: Dictionary = {}
var _stage: PanelContainer = null
var _stage_box: VBoxContainer = null
var _stage_scroll: ScrollContainer = null
var _question_slot: VBoxContainer = null
var _submit: Button = null
var _countdown: Label = null
## "Trả lời đúng và nhanh để được thêm điểm!" — only while waiting / counting down and only when the server enabled it.
var _speed_hint: HBoxContainer = null
var _waiting: Label = null
var _reveal: AnswerRevealOverlay = null
var _result: MatchResultView = null
var _chat_column: Control = null
## F125: the strip was folded to its header for the compact canvas (unfolded when it moves beside the header again).
var _folded_strip: bool = false
var _chat_dock: ChatDock = null
## Bottom of the question column: the chat strip while a question is on screen.
var _strip_slot: VBoxContainer = null
## Fallback place of the strip (under the question) when the header row has no room for it (narrow window, long text).
var _strip_bottom: VBoxContainer = null
var _pause: Control = null
var _pause_label: Label = null
var _chat_ready: bool = false


func _ready() -> void:
	visible = false
	_build()
	resized.connect(_layout)
	AppState.compact_changed.connect(_on_compact_changed)


func _on_compact_changed(_compact: bool) -> void:
	_fit_result()
	if battle != null:
		_render_banner(battle.now_ms())


func _exit_tree() -> void:
	_unbind()
	if visible:
		QualityManager.resume_world(SUSPEND_REASON)
		AppState.end_blocking(BLOCKING_ACTIVITY)
		_set_question_on(false)


## WorldRoot: `ui_requested("battle", {match_id, mode})`.
func open(args: Dictionary = {}) -> void:
	if services == null and AppState.services != null:
		services = AppState.get_services()
	match_id = String(args.get("match_id", ""))
	mode = String(args.get("mode", ""))
	_bind(services.battle if services != null else null)
	show()
	QualityManager.suspend_world(SUSPEND_REASON)
	AppState.begin_blocking(BLOCKING_ACTIVITY)
	_result.visible = false
	_fit_result()
	_stage.visible = true
	_reveal.hide()
	_clear_question()
	_chat_ready = false
	_layout()
	_render_state()
	if battle != null and Uuid.is_valid(match_id):
		battle.start(match_id, mode)
	_open_chat()


func close() -> void:
	_set_question_on(false)
	_clear_question()
	_chat_dock.clear_context()
	if battle != null:
		battle.leave()
	_unbind()
	hide()
	QualityManager.resume_world(SUSPEND_REASON)
	AppState.end_blocking(BLOCKING_ACTIVITY)
	closed.emit()


# ---------- test hooks ----------

func header() -> MatchHeader:
	return _header


func result_view() -> MatchResultView:
	return _result


func reveal() -> AnswerRevealOverlay:
	return _reveal


func chat_dock() -> ChatDock:
	return _chat_dock


func chat_column() -> Control:
	return _chat_column


func question_area() -> Control:
	return _question_slot


func banner_text() -> String:
	return (_banner["label"] as Label).text if (_banner["panel"] as PanelContainer).visible else ""


func status_text() -> String:
	return (_status["label"] as Label).text if (_status["panel"] as PanelContainer).visible else ""


func speed_hint_visible() -> bool:
	return _speed_hint.visible


func countdown_text() -> String:
	return _countdown.text if _countdown.visible else ""


func pause_visible() -> bool:
	return _pause.visible


func submit_button() -> Button:
	return _submit



func question_on() -> bool:
	return _question_on


## Taps answer tile `index` (tests; the same path as a tap or the number key).
func choose(index: int) -> void:
	if question != null:
		question.select_index(index)


# ---------- binding ----------

func _bind(next: BattleClient) -> void:
	_unbind()
	battle = next
	if battle == null:
		return
	for pair: Array in _signal_pairs():
		(pair[0] as Signal).connect(pair[1] as Callable)


func _unbind() -> void:
	if battle == null:
		return
	for pair: Array in _signal_pairs():
		if (pair[0] as Signal).is_connected(pair[1] as Callable):
			(pair[0] as Signal).disconnect(pair[1] as Callable)
	battle = null


func _signal_pairs() -> Array:
	return [[battle.view_changed, _on_view_changed], [battle.question_changed, _on_question_changed],
		[battle.answer_locked, _on_answer_locked], [battle.answer_evaluated, _on_evaluated],
		[battle.question_revealed, _on_revealed], [battle.scores_changed, _on_scores],
		[battle.player_status_changed, _on_player_status], [battle.paused_changed, _on_paused],
		[battle.match_ended, _on_match_ended], [battle.notice_requested, _on_notice]]


# ---------- per frame ----------

func advance(_delta: float) -> void:
	if _question_on:
		_place_chat(true)
	_fit_strip()
	if battle == null or battle.view == null:
		return
	var now: int = battle.now_ms()
	_header.render_time(battle.view, now)
	if battle.view.state == BattleView.STATE_COUNTDOWN and battle.view.countdown_at > 0:
		_countdown.text = str(maxi(1, ceili(float(battle.view.countdown_at - now) / 1000.0)))
	_render_banner(now)


func _process(delta: float) -> void:
	if visible:
		advance(delta)


func _unhandled_input(event: InputEvent) -> void:
	if not visible or question == null or not _question_on:
		return
	if event is InputEventKey and (event as InputEventKey).pressed and not (event as InputEventKey).echo:
		var k: Key = (event as InputEventKey).keycode
		if k >= KEY_1 and k <= KEY_4:
			question.select_index(int(k - KEY_1))
			get_viewport().set_input_as_handled()


# ---------- battle events ----------

func _on_view_changed(view: BattleView) -> void:
	_render_state()
	_header.render(view, battle.self_id)
	if not _chat_ready:
		_open_chat()
	if view.state != BattleView.STATE_QUESTION and view.state != BattleView.STATE_PAUSED:
		_set_question_on(false)
	if view.my_answer_locked and question != null:
		question.set_locked(true)


func _on_question_changed(view: BattleView) -> void:
	_reveal.hide()
	ArenaUi.show_notice(_status, "")
	_header.render(view, battle.self_id)
	if view.question_instance_id != _rendered_instance or question == null:
		_show_question(view)
	question.set_locked(view.my_answer_locked or battle.input_locked)
	if view.my_answer_locked:
		ArenaUi.show_notice(_status, "ui.arena.battle.recorded")
	_render_state()
	_set_question_on(true)


func _on_answer_locked() -> void:
	if question != null:
		question.set_locked(true)
	_submit.disabled = true
	ArenaUi.show_notice(_status, "ui.arena.battle.recorded")


func _on_evaluated(e: AnswerEvaluation) -> void:
	if question != null:
		question.show_verdict(e.to_result())
	ArenaUi.show_notice(_status, "")
	_reveal.show_evaluation(e)
	_show_reveal.call_deferred()


## F125 polish: the feedback strip ("Đúng rồi! +150 điểm") is scrolled into view once laid out — on the compact canvas it
## may sit just under the question's visible part.
func _show_reveal() -> void:
	await get_tree().process_frame
	if is_inside_tree() and _reveal.visible:
		_stage_scroll.ensure_control_visible(_reveal)


func _on_revealed(view: BattleView) -> void:
	_header.render(view, battle.self_id)
	_set_question_on(false)
	_render_state()


func _on_scores(view: BattleView) -> void:
	_header.render(view, battle.self_id)


func _on_player_status(_user_id: String, _connected: bool, _grace_until: int) -> void:
	_header.render(battle.view, battle.self_id)
	_render_banner(battle.now_ms())


func _on_paused(paused: bool, _deadline: int) -> void:
	_pause.visible = paused
	if paused:
		_set_question_on(false)


func _on_match_ended(summary: MatchSummary) -> void:
	_set_question_on(false)
	_pause.visible = false
	_clear_question()
	_stage.visible = false
	_result.visible = true
	_fit_result()
	_result.self_id = battle.self_id
	var again: bool = services != null and not services.matchmaking.last_search.is_empty() \
			and battle.view != null and battle.view.mode in ["battle_1v1", "team_battle"]
	_result.matchmaking = services.matchmaking if services != null else null
	_result.show_result(summary, battle.view, again)
	_header.render(battle.view, battle.self_id)
	if services == null:
		return
	_load_encouragement_names(summary)
	var leaderboards: LeaderboardService = services.leaderboards
	await leaderboards.load_tiers()
	if visible and battle != null and battle.view != null:
		_result.show_tier(leaderboards.tier_for(battle.view.mode))
	if summary.review_available:
		await services.match_history.load_detail(summary.match_id)
		if visible:
			_result.show_review(services.match_history.details.get(summary.match_id) as MatchRecord)


## The subject / topic names of the encouragement line (read once, cached by MatchmakingService), then drawn again.
func _load_encouragement_names(summary: MatchSummary) -> void:
	var subject_id: String = String(summary.encouragement.get("subject_id", ""))
	if not Uuid.is_valid(subject_id):
		return
	var mm: MatchmakingService = services.matchmaking
	if not mm.subject_names.has(subject_id):
		await mm.load_subjects()
	if Uuid.is_valid(String(summary.encouragement.get("topic_id", ""))):
		await mm.load_topics(subject_id)
	if visible and _result.visible:
		_result.refresh_encouragement()


func _on_notice(key: String, args: Dictionary) -> void:
	ArenaUi.show_notice(_status, key, args, "error" if key.ends_with("NOT_FOUND") else "info")


# ---------- rendering ----------

func _render_state() -> void:
	var view: BattleView = battle.view if battle != null else null
	var state: String = view.state if view != null else BattleView.STATE_LOBBY
	_countdown.visible = state == BattleView.STATE_COUNTDOWN
	# F109-AC-09: the speed hint exists only when the pinned rule of the match enables it (never for lớp 1–5).
	_speed_hint.visible = view != null and view.speed_bonus_enabled and question == null \
			and state in [BattleView.STATE_LOBBY, BattleView.STATE_COUNTDOWN]
	var waiting_key: String = ""
	match state:
		BattleView.STATE_LOBBY:
			waiting_key = "ui.arena.battle.preparing"
		BattleView.STATE_ROUND_END, BattleView.STATE_REVEAL:
			waiting_key = "ui.arena.battle.next_soon" if _reveal.visible == false else ""
		BattleView.STATE_FINALIZING:
			waiting_key = "ui.arena.battle.finishing"
	_waiting.text = waiting_key
	_waiting.visible = not waiting_key.is_empty() and question == null
	_question_slot.visible = question != null
	_submit.visible = question != null and not question.item.type in INSTANT_TYPES and state == BattleView.STATE_QUESTION
	UiBuild.size_buttons(_stage_box)


func _render_banner(now: int) -> void:
	var key: String = ""
	var args: Dictionary = {}
	if services != null and services.realtime != null and services.realtime.is_reconnecting():
		key = "ui.arena.battle.self_reconnecting"
	elif AppState.compact and _question_on:
		# F125 polish: on the 540 tall canvas the banner row would push the answers out of view; the wifi sign on the
		# card of the player away says it (its sentence as tooltip + accessible name). The student's own reconnect
		# banner stays (their answers are locked meanwhile).
		key = ""
	elif battle != null and battle.view != null:
		for p: BattlePlayer in battle.view.participants:
			if p.user_id != battle.self_id and not p.connected and (p.grace_until <= 0 or p.grace_until > now):
				key = "ui.arena.battle.opponent_reconnecting"
				args = {"nickname": p.nickname}
				break
	var current: String = (_banner["label"] as Label).text
	var next: String = TranslationServer.translate(key).format(args) if not key.is_empty() else ""
	if next != current or (_banner["panel"] as PanelContainer).visible != not key.is_empty():
		ArenaUi.show_notice(_banner, key, args, "warning")


func _show_question(view: BattleView) -> void:
	_clear_question()
	_rendered_instance = view.question_instance_id
	if view.question == null:
		return
	question = QuestionRendererRegistry.instantiate(view.question.type)
	if question == null:
		_waiting.text = "ui.activity_player.update_required"
		_waiting.visible = true
		return
	question.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	question.size_flags_vertical = Control.SIZE_EXPAND_FILL
	_question_slot.add_child(question)
	question.setup(view.question, _context())
	if view.question.type in INSTANT_TYPES:
		question.set_instant_answer(true)
		question.choice_tapped.connect(_on_choice_tapped)
	question.answer_changed.connect(_on_answer_changed)
	question.response_prepared.connect(_on_response_prepared)
	_submit.disabled = true


func _clear_question() -> void:
	if question != null:
		_question_slot.remove_child(question)
		question.queue_free()
	question = null
	_rendered_instance = ""


func _context() -> RendererContext:
	var ctx: RendererContext = RendererContext.new()
	ctx.grade_band = ArenaUi.band()
	ctx.content_language = L10n.data_language(TranslationServer.get_locale())
	if services != null:
		ctx.math = services.math_svg
		ctx.media = services.media
	return ctx


func _on_choice_tapped(_repeated: bool) -> void:
	if question != null:
		question.commit()


func _on_answer_changed(complete: bool) -> void:
	_submit.disabled = not complete or battle == null or battle.input_locked


func _on_response_prepared(_position: int, response: Dictionary) -> void:
	if battle != null:
		battle.submit_answer(response)


func _set_question_on(on: bool) -> void:
	if on == _question_on:
		return
	_question_on = on
	if on:
		EventBus.question_shown.emit()
	else:
		EventBus.question_hidden.emit()
	_place_chat(on)
	if battle != null:
		_render_banner(battle.now_ms())


## F109-AC-13 / F119-AC-03: while a question is on screen the dock (its 2-line strip) sits beside the match header and
## the question takes the whole width below — never covering an answer, on a phone too; between questions it is the right
## column of the whole height.
func _place_chat(strip: bool) -> void:
	var target: Control = _chat_column
	if strip:
		var row: Control = _header.get_parent() as Control
		var w: float = row.size.x if row.size.x > 0.0 else get_viewport_rect().size.x - float(UiTokens.SIZE_SPACE_24) * 2.0
		# Room for the strip beside the header, with a little slack (separation, rounding).
		var room: float = w - _header.get_combined_minimum_size().x - float(UiTokens.SIZE_SPACE_16)
		# F125: the strip beside the header is as wide as the chat column share (_layout), so that is the room it needs;
		# on the compact canvas the folded dock (its header row) needs only its own width (F125 polish).
		var full: float = size.x if size.x > 0.0 else get_viewport_rect().size.x
		if AppState.compact:
			# F125 polish: the folded dock shows its chat icon (title as its tooltip), the unread count and "Mở chat" —
			# narrow enough to sit beside the match header, so the header row is the only row above the question.
			_chat_dock.allow_title(false)
		var need: float = _dock_panel_min().x if AppState.compact else maxf(_dock_panel_min().x, full * CHAT_SHARE)
		target = _strip_slot if room >= need else _strip_bottom
	if _chat_dock.get_parent() != target:
		_chat_dock.get_parent().remove_child(_chat_dock)
		target.add_child(_chat_dock)
	# F125 polish: the folded dock beside the header keeps its own height (no tall empty panel).
	_chat_dock.size_flags_vertical = Control.SIZE_SHRINK_BEGIN if strip and AppState.compact and target == _strip_slot \
			else Control.SIZE_EXPAND_FILL
	_strip_slot.visible = strip and target == _strip_slot
	_strip_bottom.visible = strip and target == _strip_bottom
	_chat_column.visible = not strip
	# F125: on the compact phone canvas (540 tall) a 2-line strip would take the answers' row: the chat folds to its
	# header (title, unread count, "Mở chat") until the question is answered — under the question, or beside the match
	# header when that row has room for it (F125 polish: the header row then stays as tall as the header).
	var fold: bool = strip and AppState.compact
	if fold and _chat_dock.mode == ChatDock.Mode.MINI:
		_chat_dock.set_mode(ChatDock.Mode.COLLAPSED)
		_folded_strip = true
	elif not fold and _folded_strip:
		_folded_strip = false
		if strip and _chat_dock.mode == ChatDock.Mode.COLLAPSED:
			_chat_dock.set_mode(ChatDock.Mode.MINI)
	_fit_strip()
	_layout()


## ChatDock is a plain Control (its panel is anchored inside): in the strip it gets the height of its 2-line panel.
func _fit_strip() -> void:
	var h: float = _dock_panel_min().y if _strip_slot.visible or _strip_bottom.visible else 0.0
	if not is_equal_approx(_chat_dock.custom_minimum_size.y, h):
		_chat_dock.custom_minimum_size.y = h


## F125 polish: on the compact canvas the result takes the whole question column — the header row (sides, "Câu 1/3", the
## time bar of the last question, the chat strip slot) hides while the result shows; a PC window keeps it.
func _fit_result() -> void:
	var top: Control = _header.get_parent() as Control
	top.visible = not (_result.visible and AppState.compact)


## Minimum width of the question side (its scroll box takes the column's minimum width).
func _left_min() -> float:
	var main: Control = _chat_column.get_parent() as Control
	var left: Control = main.get_node_or_null("LeftScroll") as Control
	return left.get_combined_minimum_size().x if left != null else 0.0


func _dock_panel_min() -> Vector2:
	var panel: Control = _chat_dock.get_node_or_null("Panel") as Control
	return panel.get_combined_minimum_size() if panel != null else Vector2.ZERO


## Where the strip is now (beside the header, or under the question on a narrow window).
func strip_slot() -> Control:
	return _strip_bottom if _strip_bottom.visible else _strip_slot


# ---------- chat (F119) ----------

## The always-on `match` channel (+ the student's `team` channel in a team battle): the server's `chat_channel_ids` of
## the snapshot, else of `matchmaking.match.assigned` (server delta 2026-11-27; the client never builds an id). The
## channel opens when the match starts — the dock joins it and retries until then. Only an older server without the ids
## falls back to the open `match` / `team` channels of `listMyChannels` (one match at a time).
func _open_chat() -> void:
	if services == null or services.chat == null or not Uuid.is_valid(match_id):
		return
	var ids: Dictionary = battle.view.chat_channel_ids if battle != null and battle.view != null else {}
	if ids.is_empty() and services.matchmaking != null:
		ids = services.matchmaking.chat_channels_for(match_id)
	var match_channel: String = String(ids.get("match", ""))
	var team_channel: String = String(ids.get("team", ""))
	if ids.is_empty():
		var found: Array[String] = await _open_channels_of_list()
		if not visible:
			return
		# The snapshot may have brought the server ids meanwhile: they win over the list.
		var later: Dictionary = battle.view.chat_channel_ids if battle != null and battle.view != null else {}
		match_channel = String(later.get("match", found[0]))
		team_channel = String(later.get("team", found[1] if later.is_empty() else ""))
	if match_channel.is_empty():
		return
	var others: Array[Dictionary] = battle.others() if battle != null else []
	var is_team: bool = mode == "team_battle" or (battle != null and battle.view != null and battle.view.is_team_battle())
	_chat_dock.show_match(match_id, match_channel, team_channel if is_team else "", others)
	_chat_ready = not others.is_empty()


## Fallback of an older server: `[match, team]` ids of the open gameplay channels of `listMyChannels`.
func _open_channels_of_list() -> Array[String]:
	var chat: ChatService = services.chat
	await chat.load_channels()
	var match_channel: String = ""
	var team_channel: String = ""
	for id: String in chat.store.channel_ids():
		var c: ChatChannelModel = chat.store.channel(id)
		if c == null or c.status != ChatChannelModel.STATUS_OPEN:
			continue
		if c.channel_type == ChatChannelModel.TYPE_MATCH and match_channel.is_empty():
			match_channel = id
		elif c.channel_type == ChatChannelModel.TYPE_TEAM and team_channel.is_empty():
			team_channel = id
	return [match_channel, team_channel]


# ---------- layout ----------

func _layout() -> void:
	if _chat_column == null:
		return
	var w: float = size.x if size.x > 0.0 else get_viewport_rect().size.x
	var dock_min: float = _dock_panel_min().x
	# F125: between questions (the dock in its own column) the question side keeps its minimum first — compact phone
	# canvas, long translations; the chat column takes CHAT_SHARE of the rest, never less than the panel needs, and a
	# column too narrow for the chat header title drops it (the chat icon keeps it as tooltip). While a question is on
	# screen the dock is a strip inside the question side: the share alone applies (no feedback loop on its width).
	var room: float = w
	if _chat_dock.get_parent() == _chat_column:
		var main: Control = _chat_column.get_parent() as Control
		room = (main.size.x if main.size.x > 0.0 else w) - _left_min() - float(main.get_theme_constant(&"separation"))
		var title_w: float = _chat_dock.title_width()
		if _chat_dock.title_allowed and dock_min > room:
			_chat_dock.allow_title(false)
			dock_min = _dock_panel_min().x
			_layout.call_deferred()
		elif not _chat_dock.title_allowed and dock_min + title_w + float(UiTokens.SIZE_SPACE_16) <= room:
			_chat_dock.allow_title(true)
			dock_min = _dock_panel_min().x
			_layout.call_deferred()
	var column: float = clampf(maxf(minf(w * CHAT_SHARE, room), dock_min), UiBuild.target() * 5.0, w * CHAT_SHARE_MAX)
	_chat_column.custom_minimum_size.x = column
	# F125 polish: the folded dock beside the header (compact, question on screen) takes only its own width.
	_strip_slot.custom_minimum_size.x = _dock_panel_min().x if AppState.compact and _question_on \
			else maxf(column, _dock_panel_min().x)


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
	var main: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	main.name = "Main"
	margin.add_child(main)
	# F125: on the compact phone canvas (540 tall) a long translation can make the column taller than the screen; it then
	# scrolls as a whole (when it fits, the column fills the height exactly as before and the question keeps its room).
	var left_scroll: ScrollContainer = ScrollContainer.new()
	left_scroll.name = "LeftScroll"
	left_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	UiBuild.expand(left_scroll, true, true)
	main.add_child(left_scroll)
	left_scroll.minimum_size_changed.connect(_layout, CONNECT_DEFERRED)
	var left: VBoxContainer = UiBuild.vbox(&"FormBox")
	left.name = "Left"
	UiBuild.expand(left, true, true)
	left_scroll.add_child(left)
	var top: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	left.add_child(top)
	_header = MatchHeader.new()
	UiBuild.expand(_header)
	top.add_child(_header)
	# While a question is on screen the chat strip sits beside the header (top right), never over the answers.
	_strip_slot = UiBuild.vbox()
	_strip_slot.name = "ChatStrip"
	_strip_slot.visible = false
	top.add_child(_strip_slot)
	_banner = ArenaUi.notice()
	left.add_child(_banner["panel"] as PanelContainer)
	_status = ArenaUi.notice()
	(_status["panel"] as PanelContainer).name = "Status"
	left.add_child(_status["panel"] as PanelContainer)
	_stage = UiBuild.panel(&"ActivityBoard")
	_stage.name = "Stage"
	UiBuild.expand(_stage, true, true)
	left.add_child(_stage)
	# The question scrolls only when it cannot fit (very long text, the +35% pseudo-locale) — the screen never overflows.
	var stage_scroll: ScrollContainer = ScrollContainer.new()
	_stage_scroll = stage_scroll
	stage_scroll.name = "StageScroll"
	stage_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	UiBuild.expand(stage_scroll, true, true)
	_stage.add_child(stage_scroll)
	_stage_box = UiBuild.vbox(&"FormBox")
	UiBuild.expand(_stage_box, true, true)
	stage_scroll.add_child(_stage_box)
	_countdown = UiBuild.data_label("", &"CountdownLabel", false)
	_countdown.name = "Countdown"
	_countdown.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_countdown.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	UiBuild.expand(_countdown, true, true)
	_countdown.visible = false
	_stage_box.add_child(_countdown)
	_speed_hint = UiBuild.hbox(&"ToolbarBox")
	_speed_hint.name = "SpeedHint"
	_speed_hint.alignment = BoxContainer.ALIGNMENT_CENTER
	_speed_hint.visible = false
	var rocket: TextureRect = UiBuild.icon_rect("emoji_rocket")
	rocket.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_speed_hint.add_child(rocket)
	var hint: Label = UiBuild.keep_words(UiBuild.label("ui.arena.battle.speed_hint", &"InstructionLabel"))
	hint.name = "SpeedHintLabel"
	hint.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	_speed_hint.add_child(hint)
	_stage_box.add_child(_speed_hint)
	_waiting = UiBuild.label("", &"InstructionLabel")
	_waiting.name = "Waiting"
	_waiting.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_waiting.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	UiBuild.expand(_waiting, true, true)
	_stage_box.add_child(_waiting)
	_question_slot = UiBuild.vbox()
	_question_slot.name = "QuestionPanel"
	UiBuild.expand(_question_slot, true, true)
	_stage_box.add_child(_question_slot)
	_reveal = AnswerRevealOverlay.new()
	_stage_box.add_child(_reveal)
	_submit = UiBuild.button("ui.arena.battle.submit", "send")
	_submit.name = "Submit"
	_submit.size_flags_horizontal = Control.SIZE_SHRINK_END
	_submit.visible = false
	_submit.pressed.connect(func() -> void:
		if question != null:
			question.commit())
	_stage_box.add_child(_submit)
	_result = MatchResultView.new()
	_result.visible = false
	UiBuild.expand(_result, true, true)
	_result.play_again_requested.connect(_on_play_again)
	_result.back_requested.connect(_on_back)
	var result_panel: PanelContainer = UiBuild.panel(&"FormCard")
	result_panel.name = "ResultPanel"
	UiBuild.expand(result_panel, true, true)
	result_panel.add_child(_result)
	result_panel.visible = false
	_result.visibility_changed.connect(func() -> void: result_panel.visible = _result.visible)
	left.add_child(result_panel)
	_strip_bottom = UiBuild.vbox()
	_strip_bottom.name = "ChatStripBottom"
	_strip_bottom.visible = false
	left.add_child(_strip_bottom)
	_chat_column = UiBuild.vbox()
	_chat_column.name = "ChatColumn"
	_chat_column.size_flags_vertical = Control.SIZE_EXPAND_FILL
	main.add_child(_chat_column)
	_chat_dock = (load("res://scenes/chat/ChatDock.tscn") as PackedScene).instantiate() as ChatDock
	UiBuild.expand(_chat_dock, true, true)
	_chat_column.add_child(_chat_dock)
	_build_pause()
	if AppState.services != null:
		var s: ServiceLocator = AppState.get_services()
		_result.bind_safety(s.chat, s.social, s.reports)


func _build_pause() -> void:
	_pause = Control.new()
	_pause.name = "Pause"
	_pause.set_anchors_preset(Control.PRESET_FULL_RECT)
	_pause.mouse_filter = Control.MOUSE_FILTER_STOP
	_pause.visible = false
	add_child(_pause)
	_pause.add_child(UiBuild.scrim(true))
	var center: CenterContainer = CenterContainer.new()
	center.set_anchors_preset(Control.PRESET_FULL_RECT)
	_pause.add_child(center)
	var card: PanelContainer = UiBuild.panel(&"FormCard")
	center.add_child(card)
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	card.add_child(row)
	row.add_child(UiBuild.icon_rect("pause", 1.4))
	_pause_label = UiBuild.label("ui.arena.battle.paused", &"HeadingLabel")
	row.add_child(_pause_label)


func _on_play_again() -> void:
	var mm: MatchmakingService = services.matchmaking
	var s: Dictionary = mm.last_search
	close()
	await mm.find_match(String(s.get("mode", "")), String(s.get("subject_id", "")), String(s.get("topic_id", "")))


func _on_back() -> void:
	close()
	EventBus.ui_requested.emit(&"arena", {})
