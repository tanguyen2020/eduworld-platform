extends GdUnitTestSuite
## T019 (F112-AC-15, AC-16, AC-17; TC-CHAT-040): the magnifier of the HUD chat opens the search over the tabs — "Tìm" is
## grey under 2 characters, the counter follows the contract bound (100); the hits show the sender, the channel name, the
## time and a plain-text excerpt (no markup, nothing parsed); "Xem" opens the channel at that message (history around its
## seq, the message lit); a `404` says "Tin này không còn xem được" and drops the hit; no hit → Mây with a hint.

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
	await hub_screen.open({"tab": "club:%s" % F111Fixtures.CLUB_SCHOOL})
	await s.settle(10)


func after_test() -> void:
	if hub_screen.visible:
		hub_screen.close()
	s.dispose()


func _panel() -> ChatSearchPanel:
	assert_bool(hub_screen.search_button().visible).is_true()
	hub_screen.search_button().pressed.emit()
	await s.settle(2)
	return hub_screen.search_panel()


func test_go_is_grey_outside_the_query_bounds() -> void:
	var panel: ChatSearchPanel = await _panel()
	assert_bool(panel.visible).is_true()
	panel.set_query("d")
	assert_bool(panel.go_button().disabled).is_true()
	panel.set_query("di boss")
	assert_bool(panel.go_button().disabled).is_false()
	assert_str(panel.counter_text()).is_equal(TranslationServer.translate("ui.chat.search.counter").format({"count": "7",
		"max": "100"}))


func test_hits_show_sender_channel_time_and_plain_excerpt() -> void:
	var panel: ChatSearchPanel = await _panel()
	panel.set_query("di boss")
	await panel.submit()
	await s.settle(2)
	assert_int(panel.hit_rows().size()).is_equal(3)
	var row: PackedStringArray = panel.row_texts(0)
	assert_array(Array(row)).contains(["CaHeoVui07", "CLB Toán"])
	var excerpts: PackedStringArray = panel.excerpts()
	assert_str(excerpts[1]).is_equal("Hẹn đi boss lúc 19h nhé")
	for e: String in excerpts:
		for tag: String in ["[b]", "[/b]", "<em>", "</em>"]:
			assert_bool(e.contains(tag)).is_false()
	# The DM hit names the friend.
	assert_array(Array(panel.row_texts(2))).contains(["CaHeoVui07"])


func test_hit_opens_the_channel_at_the_message() -> void:
	var panel: ChatSearchPanel = await _panel()
	panel.set_query("di boss")
	await panel.submit()
	await s.settle(2)
	var hit: Dictionary = s.services.chat_search.hits[0]
	var gets: int = s.h.http.count(F112Fixtures.ROUTE_MESSAGES)
	await hub_screen.jump_to(hit)
	await s.settle(4)
	assert_bool(panel.visible).is_false()
	assert_int(s.h.http.count(F112Fixtures.ROUTE_MESSAGES)).is_greater(gets)
	assert_str(s.h.http.last(F112Fixtures.ROUTE_MESSAGES).query).contains("before_seq=")
	assert_str(hub_screen.dock().current_channel).is_equal(F112Fixtures.TEAM_SCHOOL)
	assert_str(hub_screen.dock().list().flash_key).is_equal((hit["message"] as ChatMessageModel).key())


func test_404_hit_is_gone_and_dropped() -> void:
	var panel: ChatSearchPanel = await _panel()
	panel.set_query("di boss")
	await panel.submit()
	await s.settle(2)
	s.h.http.clear(F112Fixtures.ROUTE_MESSAGES).reply(F112Fixtures.ROUTE_MESSAGES, 404, AuthHarness.problem(404, "NOT_FOUND"))
	await hub_screen.jump_to(s.services.chat_search.hits[0])
	await s.settle(2)
	assert_str(hub_screen.notice_text()).is_equal(TranslationServer.translate("ui.chat.search.gone"))
	assert_int(s.services.chat_search.hits.size()).is_equal(2)


func test_no_hit_shows_the_empty_hint() -> void:
	s.h.http.clear(F112Fixtures.ROUTE_SEARCH).reply(F112Fixtures.ROUTE_SEARCH, 200, {"data": [], "page": {"has_more": false}})
	var panel: ChatSearchPanel = await _panel()
	panel.set_query("không có")
	await panel.submit()
	await s.settle(2)
	assert_bool(panel.empty_visible()).is_true()
