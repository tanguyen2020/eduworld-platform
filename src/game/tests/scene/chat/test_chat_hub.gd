extends GdUnitTestSuite
## T009, T028 (F112-AC-01, AC-02, AC-04, AC-05, AC-06, AC-26; TC-CHAT-002, TC-CHAT-006, TC-CHAT-039, TC-UI-001): the HUD
## chat opens with the tabs of the listed channels (no class / grade tab); a channel tab shows the F119 ChatDock in
## `show_channel` mode and a report carries `context.channel_id`; only the tab on screen is joined — changing tab leaves
## the previous one, closing leaves it; the HUD button shows the server unread total ("99+") and a separate static "@"
## badge; during a test / exam the button hides and every persistent channel is left while the gameplay chat goes on; the
## window has exactly one red "✕ Đóng".

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
	AppState.end_blocking(&"exam")
	s.dispose()


func _open(args: Dictionary = {}) -> void:
	await hub_screen.open(args)
	await s.settle(10)


func _joined() -> PackedStringArray:
	var out: PackedStringArray = []
	for cmd: Dictionary in s.sent(ChatService.CMD_JOIN):
		out.append(String((cmd["payload"] as Dictionary)["channel_id"]))
	return out


## Answers every join sent so far (the server took them).
func _ack_joins() -> void:
	for cmd: Dictionary in s.sent(ChatService.CMD_JOIN):
		s.rt.push(s.rt.envelope("system.command.acked", {"result": {}}, String(cmd["id"])))


func _left() -> PackedStringArray:
	var out: PackedStringArray = []
	for cmd: Dictionary in s.sent(ChatService.CMD_LEAVE):
		out.append(String((cmd["payload"] as Dictionary)["channel_id"]))
	return out


func test_tabs_of_the_listed_channels() -> void:
	await _open()
	assert_array(Array(hub_screen.tab_ids())).is_equal(["area", "world", "club:%s" % F111Fixtures.CLUB_SCHOOL,
		"club:%s" % F111Fixtures.CLUB_OWN, "club:%s" % F111Fixtures.CLUB_LOCKED, "friends"])
	assert_str(hub_screen.tab_button("world").text).is_equal(TranslationServer.translate("ui.chat.hub.tab.world"))
	assert_str(hub_screen.tab_button("club:%s" % F111Fixtures.CLUB_SCHOOL).text).is_equal("CLB Toán")
	# Unread on the club tab (3), "@" on World, none on Area.
	assert_str(hub_screen.tab_badge_text("club:%s" % F111Fixtures.CLUB_SCHOOL)).is_equal("3")
	assert_str(hub_screen.tab_badge_text("world")).is_equal(TranslationServer.translate("ui.chat.mention_badge"))
	assert_str(hub_screen.tab_badge_text("area")).is_empty()


func test_only_the_tab_on_screen_is_joined() -> void:
	await _open({"tab": "world"})
	assert_array(Array(_joined())).is_equal([F112Fixtures.WORLD])
	_ack_joins()
	await s.settle(2)
	hub_screen.select_tab("club:%s" % F111Fixtures.CLUB_SCHOOL)
	await s.settle(10)
	assert_array(Array(_left())).contains([F112Fixtures.WORLD])
	assert_array(Array(_joined())).contains([F112Fixtures.TEAM_SCHOOL])
	_ack_joins()
	await s.settle(2)
	hub_screen.close()
	await s.settle(2)
	assert_array(Array(_left())).contains([F112Fixtures.TEAM_SCHOOL])
	# Nothing else of the student's channels was ever joined.
	for id: String in _joined():
		assert_bool(id in [F112Fixtures.WORLD, F112Fixtures.TEAM_SCHOOL]).override_failure_message(id).is_true()


func test_persistent_tab_reuses_dock_and_reports_with_channel_id() -> void:
	await _open({"tab": "club:%s" % F111Fixtures.CLUB_SCHOOL})
	var dock: ChatDock = hub_screen.dock()
	assert_bool(dock.is_visible_in_tree()).is_true()
	assert_str(String(dock.context_kind)).is_equal(String(ChatDock.CONTEXT_PERSISTENT))
	assert_str(dock.current_channel).is_equal(F112Fixtures.TEAM_SCHOOL)
	assert_object(dock.composer()).is_not_null()
	assert_str(String(dock.dialogs().context.get("channel_id", ""))).is_equal(F112Fixtures.TEAM_SCHOOL)


func test_hud_badge_total_99_plus_and_static_mention_badge() -> void:
	var button: ChatHubButton = auto_free(ChatHubButton.new()) as ChatHubButton
	add_child(button)
	button.bind_hub(s.services.chat_hub)
	await s.services.chat_hub.refresh()
	s.services.chat_hub.tick(Time.get_ticks_msec() + 5000)
	assert_str(button.badge_text()).is_equal("6")
	assert_bool(button.mention_visible()).is_true()
	s.services.chat.store.apply_unread({F112Fixtures.TEAM_SCHOOL: 120}, {})
	s.services.chat_hub.tick(Time.get_ticks_msec() + 10000)
	assert_str(button.badge_text()).is_equal(TranslationServer.translate("ui.chat.unread_many"))
	# Static: no animation node, nothing moving.
	assert_array(button.find_children("*", "AnimationPlayer", true, false)).is_empty()
	s.services.chat.store.apply_unread({F112Fixtures.TEAM_SCHOOL: 0, F112Fixtures.DM_KNOWN: 0, F112Fixtures.DM_UNKNOWN: 0},
			{F112Fixtures.WORLD: 0})
	s.services.chat_hub.tick(Time.get_ticks_msec() + 20000)
	assert_str(button.badge_text()).is_empty()
	assert_bool(button.mention_visible()).is_false()


func test_hub_hidden_during_exam() -> void:
	var button: ChatHubButton = auto_free(ChatHubButton.new()) as ChatHubButton
	add_child(button)
	button.bind_hub(s.services.chat_hub)
	# The gameplay chat of a party stays joined.
	var party: ChatDock = auto_free((load("res://scenes/chat/ChatDock.tscn") as PackedScene).instantiate()) as ChatDock
	add_child(party)
	party.bind_services(s.services.chat, s.services.social, s.services.reports)
	party.show_party(ChatFixtures.PARTY_ID, ChatFixtures.PARTY_CHANNEL)
	await _open({"tab": "world"})
	_ack_joins()
	await s.settle(2)
	assert_bool(s.services.chat.is_joined(F112Fixtures.WORLD)).is_true()
	AppState.begin_blocking(&"exam")
	await s.settle(2)
	assert_bool(button.visible).is_false()
	assert_bool(hub_screen.visible).is_false()
	assert_bool(s.services.chat.is_joined(F112Fixtures.WORLD)).is_false()
	assert_bool(_left().has(F112Fixtures.WORLD)).is_true()
	assert_bool(_left().has(ChatFixtures.PARTY_CHANNEL)).is_false()
	AppState.end_blocking(&"exam")
	await s.settle(2)
	assert_bool(button.visible).is_true()
	party.clear_context()


func test_window_close_and_tiers() -> void:
	await _open()
	var closes: Array[Button] = hub_screen.close_buttons()
	assert_int(closes.size()).is_equal(1)
	assert_str(String(closes[0].theme_type_variation)).is_equal("DangerButton")
	assert_str(closes[0].text).is_equal("ui.common.close")
	closes[0].pressed.emit()
	await s.settle(2)
	assert_bool(hub_screen.visible).is_false()
