extends GdUnitTestSuite
## T013, T016 (F112-AC-09, AC-11, AC-12, AC-13, AC-14; TC-CHAT-005, TC-CHAT-046, TC-CHAT-048, TC-CHAT-008, TC-CHAT-036):
## the states of a persistent channel in the HUD chat, the same in every grade band — a frozen DM locks writing with "Cuộc
## trò chuyện này đang tạm đóng" (history still readable, nobody named) and unlocks when open again; `CHAT_NOT_ALLOWED`
## drops the unsent message (no automatic resend) and locks the channel with "Kênh này đang nghỉ…" until the next
## `listMyChannels`; `CURFEW_ACTIVE` locks every channel with the end of the curfew from the bootstrap; `sendable_kinds`
## [quick, emoji] hides typing / stickers / GIFs and [] locks the composer; a `read_only` club reads "Club đang tạm
## khóa…"; leaving / being removed from a club or a history `404` removes its tab and its messages, with no reason; World /
## Area have no forward / pin / seen and no member list.

const BANDS: PackedStringArray = ["g1_3", "g4_5", "g6_7", "g8_9"]

var s: F112SceneHarness = null
var hub_screen: ChatHub = null
var _autostart: bool = true


func before() -> void:
	_autostart = AuthService.autostart
	AuthService.autostart = false


func after() -> void:
	AuthService.autostart = _autostart


func before_test() -> void:
	s = F112SceneHarness.new()
	await s.start("g6_7")
	hub_screen = auto_free((load("res://scenes/chat/ChatHub.tscn") as PackedScene).instantiate()) as ChatHub
	hub_screen.services = s.services
	add_child(hub_screen)
	hub_screen.size = Vector2(1280, 720)


func after_test() -> void:
	if hub_screen.visible:
		hub_screen.close()
	s.dispose()


func _open(tab: String) -> ChatDock:
	await hub_screen.open({"tab": tab})
	await s.settle(10)
	return hub_screen.dock()


func _without(channel_id: String) -> Dictionary:
	var body: Dictionary = F112Fixtures.channels()
	body["data"] = (body["data"] as Array).filter(func(c: Variant) -> bool:
		return String((c as Dictionary)["channel_id"]) != channel_id)
	return body


func _lock(dock: ChatDock) -> String:
	return dock.composer().lock_text()


func test_frozen_dm_locks_with_a_neutral_sentence_and_unlocks() -> void:
	await hub_screen.open({})
	await s.services.chat_hub.open_dm(F112Fixtures.FRIEND_DM)
	await s.settle(10)
	var dock: ChatDock = hub_screen.dock()
	assert_str(dock.current_channel).is_equal(F112Fixtures.DM_KNOWN)
	var env: Dictionary = RealtimeHarness.fixture("chat.channel.membership_changed.dm_frozen")
	var payload: Dictionary = (env["payload"] as Dictionary).duplicate()
	payload["channel_id"] = F112Fixtures.DM_KNOWN
	s.push_chat("chat.channel.membership_changed", payload)
	await s.settle(2)
	var frozen: String = TranslationServer.translate("ui.chat.dm.frozen")
	assert_str(_lock(dock)).is_equal(frozen)
	assert_bool(dock.list().is_visible_in_tree()).is_true()
	for w: String in ["chặn", "block", "bị", "đã chặn"]:
		assert_bool(frozen.to_lower().contains(w)).override_failure_message("'%s'" % w).is_false()
	payload["status"] = "open"
	s.push_chat("chat.channel.membership_changed", payload)
	await s.settle(2)
	assert_str(_lock(dock)).is_empty()


func test_not_allowed_drops_the_message_and_locks_until_refresh() -> void:
	var dock: ChatDock = await _open("world")
	var m: ChatMessageModel = s.services.chat.send_text(F112Fixtures.WORLD, "Chào cả trường")
	assert_object(m).is_not_null()
	await s.settle(2)
	s.ack_last(ChatService.CMD_SEND, false, "CHAT_NOT_ALLOWED")
	await s.settle(4)
	assert_object(s.services.chat.store.find_pending(m.client_message_id)).is_null()
	assert_str(_lock(dock)).is_equal(TranslationServer.translate("ui.chat.hub.locked_not_allowed"))
	var sends: int = s.sent(ChatService.CMD_SEND).size()
	s.rt.advance_ms(60000)
	await s.settle(2)
	assert_int(s.sent(ChatService.CMD_SEND).size()).is_equal(sends)
	await s.services.chat_hub.refresh()
	await s.settle(2)
	assert_str(_lock(dock)).is_empty()


func test_curfew_locks_every_channel_with_the_end_time() -> void:
	var player: Dictionary = AppState.player.duplicate(true)
	var policies: Dictionary = player.get("policies", {}) as Dictionary
	policies["play_time"] = {"curfew": {"start": "21:30", "end": "06:00"}}
	player["policies"] = policies
	AppState.player = player
	var dock: ChatDock = await _open("club:%s" % F111Fixtures.CLUB_SCHOOL)
	s.services.chat.send_quick(F112Fixtures.TEAM_SCHOOL, "qm_hello")
	await s.settle(2)
	s.ack_last(ChatService.CMD_SEND, false, "CURFEW_ACTIVE")
	await s.settle(4)
	var expected: String = TranslationServer.translate("ui.chat.hub.locked_curfew").format({"time": LocaleFormat.time(
			{"hour": 6, "minute": 0})})
	assert_str(_lock(dock)).is_equal(expected)
	hub_screen.select_tab("world")
	await s.settle(6)
	assert_str(_lock(dock)).is_equal(expected)


func test_sendable_kinds_quick_only_and_none_same_in_every_band() -> void:
	var quick: Dictionary = F112Fixtures.channel(F112Fixtures.WORLD, "world")
	quick["sendable_kinds"] = ["quick", "emoji"]
	for band: String in BANDS:
		AppState.set_grade_band(band)
		var body: Dictionary = F112Fixtures.channels()
		body["data"] = (body["data"] as Array).filter(func(c: Variant) -> bool:
			return String((c as Dictionary)["channel_id"]) != F112Fixtures.WORLD)
		(body["data"] as Array).append(quick.duplicate(true))
		s.h.http.clear(F112Fixtures.ROUTE_CHANNELS).reply(F112Fixtures.ROUTE_CHANNELS, 200, body)
		var dock: ChatDock = await _open("world")
		assert_bool(dock.composer().text_row_visible()).override_failure_message(band).is_false()
		assert_bool(dock.composer().quick_buttons().is_empty()).override_failure_message(band).is_false()
		hub_screen.close()
	var none: Dictionary = quick.duplicate(true)
	none["sendable_kinds"] = []
	var body2: Dictionary = F112Fixtures.channels()
	body2["data"] = (body2["data"] as Array).filter(func(c: Variant) -> bool:
		return String((c as Dictionary)["channel_id"]) != F112Fixtures.WORLD)
	(body2["data"] as Array).append(none)
	s.h.http.clear(F112Fixtures.ROUTE_CHANNELS).reply(F112Fixtures.ROUTE_CHANNELS, 200, body2)
	var locked: ChatDock = await _open("world")
	assert_str(_lock(locked)).is_equal(TranslationServer.translate("ui.chat.hub.locked_none"))


func test_read_only_club_and_removed_club() -> void:
	var dock: ChatDock = await _open("club:%s" % F111Fixtures.CLUB_LOCKED)
	assert_str(_lock(dock)).is_equal(TranslationServer.translate("ui.chat.hub.locked_read_only"))
	hub_screen.select_tab("club:%s" % F111Fixtures.CLUB_SCHOOL)
	await s.settle(8)
	assert_int(s.services.chat.store.count(F112Fixtures.TEAM_SCHOOL)).is_greater_equal(0)
	# The server lists the channel no more once the student left the club.
	s.h.http.clear(F112Fixtures.ROUTE_CHANNELS).reply(F112Fixtures.ROUTE_CHANNELS, 200, _without(F112Fixtures.TEAM_SCHOOL))
	var env: Dictionary = RealtimeHarness.fixture("chat.channel.membership_changed.club_left")
	s.push_chat("chat.channel.membership_changed", env["payload"] as Dictionary)
	await s.settle(4)
	assert_bool(hub_screen.tab_ids().has("club:%s" % F111Fixtures.CLUB_SCHOOL)).is_false()
	assert_object(s.services.chat.store.channel(F112Fixtures.TEAM_SCHOOL)).is_null()
	assert_str(hub_screen.notice_text()).is_empty()


func test_history_404_removes_the_tab() -> void:
	s.h.http.clear(F112Fixtures.ROUTE_MESSAGES).reply(F112Fixtures.ROUTE_MESSAGES, 404, AuthHarness.problem(404, "NOT_FOUND"))
	# Listed while the hub opens; once the history answered 404 the server lists it no more.
	s.h.http.clear(F112Fixtures.ROUTE_CHANNELS).reply(F112Fixtures.ROUTE_CHANNELS, 200, F112Fixtures.channels()).reply(
			F112Fixtures.ROUTE_CHANNELS, 200, F112Fixtures.channels()).reply(F112Fixtures.ROUTE_CHANNELS, 200,
			_without(F112Fixtures.TEAM_OWN))
	await _open("club:%s" % F111Fixtures.CLUB_OWN)
	await s.settle(6)
	assert_bool(hub_screen.tab_ids().has("club:%s" % F111Fixtures.CLUB_OWN)).is_false()


func test_world_has_no_forward_pin_seen_or_members() -> void:
	var dock: ChatDock = await _open("world")
	var page: Dictionary = ChatFixtures.history(s.services.chat.store.self_id)
	for m: Variant in page["data"] as Array:
		(m as Dictionary)["channel_id"] = F112Fixtures.WORLD
	s.h.http.clear(F112Fixtures.ROUTE_MESSAGES).reply(F112Fixtures.ROUTE_MESSAGES, 200, page)
	await s.services.chat.load_history(F112Fixtures.WORLD)
	await s.settle(4)
	var chat: ChatService = s.services.chat
	for m: ChatMessageModel in chat.store.visible(F112Fixtures.WORLD):
		assert_bool(chat.can_forward(m)).is_false()
		assert_bool(chat.can_pin(m)).is_false()
	assert_str(chat.store.channel(F112Fixtures.WORLD).receipts_mode()).is_equal(ChatChannelModel.RECEIPTS_NONE)
	assert_bool(dock.players_button().visible).is_false()
