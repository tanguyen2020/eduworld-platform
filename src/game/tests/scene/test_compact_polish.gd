extends GdUnitTestSuite
## F125 polish (STUDENT_GAME_UX_SOCIAL §7 MessageList, §8 ResultScreen / reconnect; F109 + F125 Clarifications
## 2026-11-28): for every grade band at the reference windows (the compact phones 844×390 / 740×360 → logical
## 1168×540 / 1110×540, and the PC windows):
## - the result of a match: the button row never covers the result, and the outcome (Mây's bubble with the
##   encouragement) and the score lines are wholly seen at rest above the buttons (BandLayoutProbe.footer_cuts);
## - the chat column: message cards never overlap (BandLayoutProbe.overlapping_messages) — the reply + edited card of
##   the match chat fixture is taller than the list's first estimate;
## - a question on the compact canvas while another player reconnects: every answer tile is seen without scrolling (the
##   chat folded to its header row beside the match header), the reconnect banner folds into the wifi sign on that
##   player's card (its sentence as tooltip + accessible name); on a PC window the banner still says it in words.
## - the arena Play tab: the action buttons ("Thách đấu bạn", "Tìm trận") never cut the mode cards (compact: a column at
##   their right).
## FakeHttp + contract fixtures (TEST-ONLY, AGENTS R17).

const SETTLE_FRAMES: int = 6
const RESULT_VIEWS: PackedStringArray = ["result_encouragement", "result_win", "result_loss", "result_quiz_goal",
	"result_quiz_rank", "result_cancelled"]

var s: F069SceneHarness = null
var _autostart: bool = true


func before() -> void:
	_autostart = AuthService.autostart
	AuthService.autostart = false


func after() -> void:
	AuthService.autostart = _autostart


func before_test() -> void:
	s = F069SceneHarness.new()
	await s.start("g1_3")


func after_test() -> void:
	s.h.http.delay_frames = 1
	s.dispose()
	AppState.set_compact(false)


## Builds `view` for every band × window and returns `probe.call(arena, vp, label, compact)` lines.
func _each(view: String, probe: Callable, sizes: Dictionary = BandLayoutProbe.SIZES) -> PackedStringArray:
	var failures: PackedStringArray = []
	for band: String in UiTokens.DENSITY_BANDS:
		for window: String in sizes.keys():
			AppState.set_grade_band(band)
			var compact: bool = BandLayoutProbe.compact_of(window)
			AppState.set_compact(compact)
			var vp: SubViewport = SubViewport.new()
			vp.size = sizes[window] as Vector2i
			vp.render_target_update_mode = SubViewport.UPDATE_DISABLED
			add_child(vp)
			var root: Control = Control.new()
			root.theme = AppState.theme_service.current_theme
			vp.add_child(root)
			root.size = Vector2(vp.size)
			var screen: Control = await CompetitionScreens.build(view, root, s.services, s.h.http)
			for i: int in SETTLE_FRAMES:
				await await_idle_frame()
			failures.append_array(probe.call(screen, vp, "%s %s %s" % [view, band, window], compact) as PackedStringArray)
			vp.queue_free()
			await await_idle_frame()
	AppState.set_compact(false)
	return failures


func _assert_none(failures: PackedStringArray) -> void:
	assert_array(failures).override_failure_message("\n".join(failures)).is_empty()


# ---------- result: the button row never covers the result ----------

func _result_cuts(arena: Control, vp: Viewport, label: String, _compact: bool) -> PackedStringArray:
	var result: MatchResultView = (arena as BattleArena).result_view()
	var scroll: ScrollContainer = null
	for child: Node in result.get_children():
		if child is ScrollContainer:
			scroll = child as ScrollContainer
	var keep: Array[Control] = []
	for name: String in ["Outcome", "Stats"]:
		var c: Control = result.find_child(name, true, false) as Control
		if c != null:
			keep.append(c)
	if scroll == null or keep.is_empty():
		return PackedStringArray(["%s: no result scroll box / summary" % label])
	return BandLayoutProbe.footer_cuts(scroll, result.find_child("Buttons", true, false) as Control, keep, vp, label)


func test_result_buttons_never_cover_the_outcome_and_score() -> void:
	var failures: PackedStringArray = []
	for view: String in RESULT_VIEWS:
		failures.append_array(await _each(view, _result_cuts))
	_assert_none(failures)


# ---------- chat: message cards never overlap ----------

func _messages(arena: Control, vp: Viewport, label: String, _compact: bool) -> PackedStringArray:
	var out: PackedStringArray = []
	var checked: int = 0
	for node: Node in arena.find_children("*", "ChatMessageList", true, false):
		if (node as Control).is_visible_in_tree() and not (node as ChatMessageList).bubbles().is_empty():
			checked += 1
	if checked == 0:
		out.append("%s: no visible message list" % label)
	for line: String in BandLayoutProbe.overlapping_messages(arena, vp):
		out.append("%s: %s" % [label, line])
	return out


func test_chat_cards_never_overlap_in_the_match_column() -> void:
	var failures: PackedStringArray = []
	for view: String in ["result_encouragement", "result_win", "result_loss"]:
		failures.append_array(await _each(view, _messages))
	_assert_none(failures)


## The list re-measures a card whose height changed after it was first laid out (a new width of the column, a reply
## excerpt, a late picture): the next card moves down instead of being covered.
func test_chat_list_moves_the_next_card_when_a_card_grows() -> void:
	var failures: PackedStringArray = []
	for band: String in UiTokens.DENSITY_BANDS:
		AppState.set_grade_band(band)
		var vp: SubViewport = SubViewport.new()
		vp.size = Vector2i(1168, 540)
		add_child(vp)
		var holder: Control = Control.new()
		holder.theme = AppState.theme_service.current_theme
		vp.add_child(holder)
		var list: ChatMessageList = ChatMessageList.new()
		holder.add_child(list)
		list.size = Vector2(560, 400)
		var self_id: String = CompetitionPlayer.self_id(AppState)
		list.context = {"self_id": self_id}
		var entries: Array[ChatMessageModel] = []
		for m: Variant in ChatFixtures.history(self_id)["data"] as Array:
			entries.append(ChatMessageModel.from_dict(m as Dictionary))
		list.set_entries(entries)
		for i: int in SETTLE_FRAMES:
			await await_idle_frame()
		# The column gets narrower (the chat header drops its title, the band changes): every card wraps again.
		list.size = Vector2(300, 400)
		for i: int in SETTLE_FRAMES:
			await await_idle_frame()
		for line: String in BandLayoutProbe.overlapping_messages(holder, vp):
			failures.append("%s 300 px: %s" % [band, line])
		vp.queue_free()
		await await_idle_frame()
	_assert_none(failures)


# ---------- question + reconnect on the compact canvas ----------

func _answers_seen(arena: Control, vp: Viewport, label: String, compact: bool) -> PackedStringArray:
	var out: PackedStringArray = []
	var battle: BattleArena = arena as BattleArena
	var view: Rect2 = Rect2(Vector2.ZERO, Vector2(vp.size)).grow(BandLayoutProbe.TOLERANCE_PX)
	var tiles: int = 0
	for node: Node in battle.question_area().find_children("*", "BaseButton", true, false):
		var b: Control = node as Control
		if not b.is_visible_in_tree() or not String(b.theme_type_variation).begins_with("AnswerTile"):
			continue
		tiles += 1
		var seen: Rect2 = view
		var p: Node = b.get_parent()
		while p != null:
			if p is ScrollContainer:
				seen = seen.intersection((p as Control).get_global_rect().grow(BandLayoutProbe.TOLERANCE_PX))
			p = p.get_parent()
		# The compact canvas (F125 polish) — a PC window may scroll a long question as before.
		if compact and not seen.encloses(b.get_global_rect()):
			out.append("%s: answer %s not seen without scrolling (%s, seen %s)" % [label, b.name, b.get_global_rect(), seen])
	if tiles == 0:
		out.append("%s: no answer tile" % label)
	var away: Array[Node] = battle.header().find_children("Away*", "TextureRect", true, false)
	if away.is_empty():
		out.append("%s: no reconnect sign on the card of the player away" % label)
	for node: Node in away:
		var icon: TextureRect = node as TextureRect
		if icon.accessibility_name.is_empty() or icon.tooltip_text.is_empty():
			out.append("%s: the reconnect sign has no text (tooltip / accessible name)" % label)
	if compact and not battle.banner_text().is_empty():
		out.append("%s: the reconnect banner still takes a row over the question" % label)
	if not compact and battle.banner_text().is_empty():
		out.append("%s: the reconnect banner is missing on a PC window" % label)
	return out


func test_answers_stay_in_view_while_a_player_reconnects() -> void:
	var failures: PackedStringArray = []
	for view: String in ["battle_team", "battle_reconnect"]:
		failures.append_array(await _each(view, _answers_seen))
	_assert_none(failures)


# ---------- arena Play tab: the action buttons never cut the mode cards ----------

func _play_cuts(screen: Control, vp: Viewport, label: String, _compact: bool) -> PackedStringArray:
	var arena: ArenaScreen = screen as ArenaScreen
	var page: Control = arena.find_child("PlayPage", true, false) as Control
	var scroll: ScrollContainer = page.find_children("*", "ScrollContainer", true, false)[0] as ScrollContainer \
			if page != null else null
	var keep: Array[Control] = []
	for card: ModeCard in arena.mode_cards():
		keep.append(card)
	if scroll == null or keep.is_empty():
		return PackedStringArray(["%s: no Play scroll box / mode cards" % label])
	return BandLayoutProbe.footer_cuts(scroll, page.find_child("Actions", true, false) as Control, keep, vp, label)


func test_arena_actions_never_cut_the_mode_cards() -> void:
	_assert_none(await _each("arena", _play_cuts))


# ---------- chat composer: the input hint is never cut ----------

## A narrow chat column cut the hint ("Type a message…" → "Type a"): the send button then shows its icon alone (its word
## as tooltip + accessible name); a wide column shows the word again.
func test_narrow_composer_keeps_the_input_hint_whole() -> void:
	var locale: String = String(Settings.locale)
	Settings.set_locale("en-US", false)
	AppState.set_grade_band("g4_5")
	var vp: SubViewport = SubViewport.new()
	vp.size = Vector2i(1168, 540)
	add_child(vp)
	var holder: Control = Control.new()
	holder.theme = AppState.theme_service.current_theme
	vp.add_child(holder)
	var composer: ChatComposer = ChatComposer.new()
	holder.add_child(composer)
	composer.size = Vector2(330, 200)
	for i: int in SETTLE_FRAMES:
		await await_idle_frame()
	var send: Button = composer.send_button()
	assert_str(send.text).is_empty()
	assert_str(send.accessibility_name).is_not_empty()
	assert_str(send.tooltip_text).is_equal("ui.chat.send")
	composer.size = Vector2(900, 200)
	for i: int in SETTLE_FRAMES:
		await await_idle_frame()
	assert_str(send.text).is_equal("ui.chat.send")
	vp.queue_free()
	Settings.set_locale(locale, false)
