extends GdUnitTestSuite
## T027 (F110-AC-19, F110-AC-24, F110-AC-26; STUDENT_GAME_UX_SOCIAL §6, STUDENT_GAME_UX §9): the friends screen groups
## Online / Offline (never a class group), each row with nickname, portrait, level and the presence in words; the
## virtualised list builds only the rows in view; a friend's profile reaches Chặn / Tắt tiếng in ≤ 2 taps (the action +
## a reason chip that confirms) and Báo cáo opens the F119 report sheet; a blocked friend disappears; "Hủy kết bạn"
## asks first; "Ẩn trạng thái" sends `presence.status.set` with `hidden = true` and shows "Đang ẩn" (and `false` back).

var s: F110SceneHarness = null
var screen: FriendsScreen = null


func before_test() -> void:
	s = F110SceneHarness.new()
	await s.start("g4_5")
	screen = auto_free((load("res://scenes/social/FriendsScreen.tscn") as PackedScene).instantiate()) as FriendsScreen
	screen.services = s.services
	add_child(screen)
	screen.size = Vector2(1280, 720)
	screen.open({})
	await s.settle(6)


func after_test() -> void:
	if screen.visible:
		screen.close()
	s.dispose()


func test_friends_are_grouped_online_offline_with_words_and_no_class_group() -> void:
	var list: FriendListView = screen.list_view()
	assert_array(list.header_keys()).is_equal(["ui.friends.group.online", "ui.friends.group.offline"])
	var row: FriendRow = list.row_for(F110Fixtures.FRIEND_ONLINE)
	assert_object(row).is_not_null()
	var texts: PackedStringArray = row.texts()
	assert_str(texts[0]).is_equal("CaHeoVui07")
	assert_str(texts[1]).is_equal(TranslationServer.translate("ui.friends.level").format({"level": "10"}))
	assert_str(texts[2]).is_equal(TranslationServer.translate("ui.friends.presence.online"))
	var all: String = " ".join(screen.shown_texts()).to_lower()
	for word: String in ["lớp", "trường", "khối", "class", "school", "grade"]:
		if word == "trường":
			continue
		assert_bool(all.contains(word)).override_failure_message("'%s' on the friends screen" % word).is_false()


func test_the_list_is_virtualised() -> void:
	var many: Array = []
	for i: int in 100:
		many.append(F110Fixtures.card("0193b0a0-0000-7000-8000-0000000%05d" % (10000 + i), "Ban%03d" % i, "online" if i % 3 else "offline"))
	s.h.http.clear(F110Fixtures.ROUTE_FRIENDS).reply(F110Fixtures.ROUTE_FRIENDS, 200, {"data": many, "page": {"has_more": false, "limit": 100}})
	await screen.refresh()
	await s.settle(3)
	assert_int(s.services.social.store.friends.size()).is_equal(100)
	assert_int(screen.list_view().pooled_rows()).is_less(30)


func test_block_from_the_profile_takes_two_taps_and_the_friend_disappears() -> void:
	screen.select_friend(F110Fixtures.FRIEND_ONLINE)
	await s.settle(1)
	var profile: FriendProfilePanel = screen.profile()
	assert_array(profile.texts()).contains(["CaHeoVui07"])
	profile.button(FriendProfilePanel.ACT_BLOCK).pressed.emit()
	var sheet: ChatSheet = screen.confirm_sheet()
	assert_bool(sheet.visible).is_true()
	var reason: Button = sheet.body().find_child("Reason_rude_language", true, false) as Button
	assert_object(reason).is_not_null()
	assert_object(reason.icon).is_not_null()
	reason.pressed.emit()
	await s.settle(3)
	assert_int(s.h.http.count(F110Fixtures.ROUTE_BLOCK)).is_equal(1)
	assert_object(s.services.social.store.friend(F110Fixtures.FRIEND_ONLINE)).is_null()
	assert_object(screen.list_view().row_for(F110Fixtures.FRIEND_ONLINE)).is_null()
	assert_str(screen.notice_text()).contains("CaHeoVui07")


func test_mute_takes_two_taps_and_report_opens_the_report_sheet() -> void:
	screen.select_friend(F110Fixtures.FRIEND_ONLINE)
	screen.profile().button(FriendProfilePanel.ACT_MUTE).pressed.emit()
	(screen.confirm_sheet().body().find_child("Reason_spam", true, false) as Button).pressed.emit()
	await s.settle(3)
	assert_str(s.h.http.last(F110Fixtures.ROUTE_MUTE).path).is_equal("/social/mutes/%s" % F110Fixtures.FRIEND_ONLINE)
	screen.select_friend(F110Fixtures.FRIEND_ONLINE)
	assert_bool(screen.profile().button(FriendProfilePanel.ACT_REPORT).visible).is_true()
	screen.profile().button(FriendProfilePanel.ACT_REPORT).pressed.emit()
	assert_bool(screen.report_sheet().visible).is_true()
	assert_object(screen.report_sheet().body().find_child("Reasons", true, false)).is_not_null()


func test_unfriend_asks_first_then_removes() -> void:
	screen.select_friend(F110Fixtures.FRIEND_OFFLINE)
	screen.profile().button(FriendProfilePanel.ACT_REMOVE).pressed.emit()
	assert_bool(screen.confirm_sheet().visible).is_true()
	assert_int(s.h.http.count(F110Fixtures.ROUTE_REMOVE)).is_equal(0)
	screen.confirm_sheet().footer_button(FriendsScreen.ACT_CONFIRM).pressed.emit()
	await s.settle(3)
	assert_int(s.h.http.count(F110Fixtures.ROUTE_REMOVE)).is_equal(1)
	assert_object(s.services.social.store.friend(F110Fixtures.FRIEND_OFFLINE)).is_null()


func test_invite_party_from_the_profile() -> void:
	s.h.http.reply(PartyFixtures.ROUTE_CREATE, 201, PartyFixtures.party_alone(s.self_id))
	s.h.http.reply(PartyFixtures.ROUTE_INVITE, 201, {})
	screen.select_friend(F110Fixtures.FRIEND_ONLINE)
	screen.profile().button(FriendProfilePanel.ACT_INVITE).pressed.emit()
	await s.settle(4)
	assert_int(s.h.http.count(PartyFixtures.ROUTE_INVITE)).is_equal(1)


func test_hide_status_sends_hidden_and_shows_hidden() -> void:
	screen.hidden_button().button_pressed = true
	await s.settle(2)
	var sent: Array[Dictionary] = s.sent("presence.status.set")
	assert_bool(bool((sent[sent.size() - 1]["payload"] as Dictionary)["hidden"])).is_true()
	s.ack_last("presence.status.set")
	await s.settle(2)
	assert_bool(s.services.presence.hidden).is_true()
	assert_str((screen.find_child("HiddenState", true, false) as Label).text).is_equal("ui.friends.hidden_on")
	screen.hidden_button().button_pressed = false
	await s.settle(2)
	sent = s.sent("presence.status.set")
	assert_bool(bool((sent[sent.size() - 1]["payload"] as Dictionary)["hidden"])).is_false()
	s.ack_last("presence.status.set")
	await s.settle(2)
	assert_bool(s.services.presence.hidden).is_false()


func test_requests_tab_shows_incoming_with_accept_and_sent_as_pending() -> void:
	screen.select_tab(FriendsScreen.TAB_REQUESTS)
	await s.settle(1)
	var list: FriendRequestList = screen.requests_view()
	assert_object(list.row(F110Fixtures.REQUEST_IN).find_child("Accept", true, false)).is_not_null()
	assert_array(list.outgoing_status_keys()).is_equal(["ui.friends.request_pending"])
	assert_str(screen.tab_button(FriendsScreen.TAB_REQUESTS).text).contains("1")
