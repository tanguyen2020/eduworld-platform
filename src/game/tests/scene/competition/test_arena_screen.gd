extends GdUnitTestSuite
## T011, T016, T026 (F109-AC-01…AC-04, AC-27; TC-CMP-001, TC-MM-001, TC-CMP-002, TC-CMP-010; SC-001): the arena
## screen, the HUD queue bar and the ready check on the contract fixtures —
## - one ModeCard per mode the server returned (no card the server did not send), the tier badge in words from
##   `getMyRatings` (never a number of rating), the subject chips of the mode and the topic chips when it queues by topic;
## - "Tìm trận" in ≤ 3 taps: mode → (subject) → Tìm trận; the window closes and the queue bar shows the wait, the mode
##   and "Hủy"; the robot offer; the timeout with "Chọn chế độ khác" / "Tìm lại";
## - the ready check: players by nickname, the bar on the server clock, seconds for lớp 4–9, none for lớp 1–3,
##   "Vào trận" sends the accept once;
## - "Thách đấu bạn" opens the friend picker.

var s: CompetitionSceneHarness = null
var screen: ArenaScreen = null
var _autostart: bool = true


func before() -> void:
	_autostart = AuthService.autostart
	AuthService.autostart = false


func after() -> void:
	AuthService.autostart = _autostart


func before_test() -> void:
	s = CompetitionSceneHarness.new()
	await s.start("g4_5")
	PartyFixtures.script(s.h.http, s.self_id)


func after_test() -> void:
	if screen != null:
		screen.free()
		screen = null
	s.dispose()


func _open(args: Dictionary = {}) -> void:
	screen = (load("res://scenes/competition/ArenaScreen.tscn") as PackedScene).instantiate() as ArenaScreen
	screen.services = s.services
	add_child(screen)
	await screen.open(args)
	for i: int in 4:
		await await_idle_frame()


func test_mode_cards_are_the_server_modes_with_the_tier_in_words() -> void:
	await _open()
	var modes: Array[String] = []
	for c: ModeCard in screen.mode_cards():
		modes.append(c.mode)
	modes.sort()
	assert_array(modes).is_equal(["battle_1v1", "coop_challenge", "team_battle"])
	assert_bool(screen.mode_card("timed_quiz") == null).is_true()
	assert_str(screen.tier_badge().text()).contains("Bạc II")
	var digits: RegEx = RegEx.create_from_string("\\d{3,}")
	assert_object(digits.search(screen.tier_badge().text())).is_null()
	assert_bool(screen.mode_card("battle_1v1").is_chosen()).is_true()
	assert_int(screen.picker().subject_chips().size()).is_equal(2)
	assert_bool(screen.picker().topic_chips().size() >= 1).is_true()


func test_no_mode_from_the_server_is_a_friendly_empty_state() -> void:
	s.h.http.clear(CompetitionFixtures.ROUTE_MODES).reply(CompetitionFixtures.ROUTE_MODES, 200, {"data": []})
	await _open()
	assert_int(screen.mode_cards().size()).is_equal(0)
	assert_bool(screen.empty_modes_visible()).is_true()
	assert_bool(screen.find_button().disabled).is_true()


func test_find_match_closes_the_window_and_the_queue_bar_takes_over() -> void:
	await _open()
	var hud: ArenaHud = ArenaHud.new()
	add_child(hud)
	hud.bind(s.services.matchmaking, s.services.tournaments)
	s.h.http.reply(CompetitionFixtures.ROUTE_TICKET_CREATE, 201, CompetitionFixtures.openapi("createMatchmakingTicket.201"))
	# SC-001: the card is already chosen, the subject too — one tap on "Tìm trận".
	screen.mode_card("team_battle").pressed.emit()
	screen.find_button().pressed.emit()
	for i: int in 6:
		await await_idle_frame()
	assert_bool(screen.visible).is_false()
	var bar: QueueStatusBar = hud.queue_bar()
	assert_bool(bar.visible).is_true()
	assert_str(bar.status_text()).contains(tr("ui.arena.mode.battle_1v1"))
	assert_bool(bar.cancel_button().visible).is_true()
	assert_str(String(s.h.http.last(CompetitionFixtures.ROUTE_TICKET_CREATE).body["mode"])).is_equal("team_battle")
	# The server offers the robot after a while; then a timeout suggests another mode.
	s.push_example("matchmaking.ticket.updated", "widened")
	hud.advance(0.1)
	assert_bool(bar.robot_button().visible).is_true()
	assert_str(bar.note_text()).is_not_empty()
	s.push_example("matchmaking.ticket.updated", "expired")
	assert_bool(bar.visible).is_true()
	assert_bool(bar.other_mode_button().visible).is_true()
	assert_bool(bar.again_button().visible).is_true()
	assert_bool(bar.cancel_button().visible).is_false()
	hud.free()


func test_ready_check_shows_players_and_the_server_clock() -> void:
	var hud: ArenaHud = ArenaHud.new()
	hud.theme = AppState.theme_service.current_theme
	add_child(hud)
	hud.bind(s.services.matchmaking, s.services.tournaments)
	var p: Dictionary = s.ws("matchmaking.proposal.created", "battle_1v1")
	p["deadline_at"] = s.services.matchmaking.now_ms() + 15000
	s.push("matchmaking.proposal.created", p)
	var dialog: ReadyCheckDialog = hud.ready_check()
	assert_bool(dialog.visible).is_true()
	hud.advance(0.1)
	assert_str(dialog.status_text()).contains("15")
	assert_float(dialog.bar_value()).is_greater(0.9)
	var names: PackedStringArray = []
	for node: Node in dialog.find_children("Nickname", "Label", true, false):
		names.append((node as Label).text)
	assert_array(Array(names)).is_equal(["CaHeoVui07"])
	# Lớp 1–3: the bar only, no number of seconds (AC-09 style, low pressure).
	AppState.set_grade_band("g1_3")
	hud.advance(0.1)
	var digits: RegEx = RegEx.create_from_string("\\d")
	assert_object(digits.search(dialog.status_text())).is_null()
	dialog.accept_button().pressed.emit()
	await await_idle_frame()
	assert_int(s.rt.peer().sent_of(MatchmakingService.CMD_ACCEPT).size()).is_equal(1)
	s.push_example("matchmaking.match.assigned", "battle_1v1")
	assert_bool(dialog.visible).is_false()
	hud.free()


func test_duel_button_opens_the_friend_picker() -> void:
	await _open()
	screen.duel_button().pressed.emit()
	for i: int in 6:
		await await_idle_frame()
	var dialog: DuelInviteDialog = screen.duel_dialog()
	assert_bool(dialog.visible).is_true()
	assert_int(dialog.friend_chips().size()).is_greater(0)
	assert_bool(dialog.send_button().disabled).is_true()
	(dialog.friend_chips()[0] as Button).pressed.emit()
	assert_bool(dialog.send_button().disabled).is_false()


func test_incoming_duel_expired_says_so() -> void:
	await _open({"duel": CompetitionFixtures.openapi("createDuel.201"), "nickname": "CaHeoVui07"})
	var dialog: DuelInviteDialog = screen.duel_dialog()
	assert_bool(dialog.accept_button().visible).is_true()
	s.h.http.reply(CompetitionFixtures.ROUTE_DUEL_ACCEPT, 410, AuthHarness.problem(410, "DUEL_EXPIRED"))
	dialog.accept_button().pressed.emit()
	for i: int in 6:
		await await_idle_frame()
	assert_str(dialog.status_text()).is_equal(tr("ui.arena.error.DUEL_EXPIRED"))
