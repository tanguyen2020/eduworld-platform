extends GdUnitTestSuite
## T007, T010 (F119-AC-01, AC-02, AC-03, AC-06, AC-07; TC-CHAT-017, TC-CHAT-020, TC-CHAT-021): the ChatDock shows the
## gameplay channel at once in a party and in a match, can be collapsed (badge for new messages) or shrunk to a 2-line
## strip but never turned off, has the tabs "Đội" / "Cả trận" with their own badges in a team battle, shrinks while a
## question is shown and keeps receiving, locks the composer when the channel closed and shows the retention days from
## the server `purge_after`. Contract fixtures + FakeHttp (TEST-ONLY).

var s: ChatSceneHarness = null


func before_test() -> void:
	s = ChatSceneHarness.new()
	await s.start()
	# After the sign-in (the bootstrap sets the band of the sample student).
	AppState.set_grade_band("g4_5")


func after_test() -> void:
	s.dispose()


func _created(seq: int, sender_id: String = ChatFixtures.OTHER, channel: String = ChatFixtures.PARTY_CHANNEL) -> Dictionary:
	var p: Dictionary = ChatFixtures.ws("chat.message.created.text", s.self_id)
	p["message_id"] = Uuid.v4()
	p["seq"] = seq
	p["channel_id"] = channel
	(p["sender"] as Dictionary)["user_id"] = sender_id
	return p


func test_party_dock_shows_the_channel_at_once_without_any_off_option() -> void:
	var dock: ChatDock = auto_free(await s.party_dock(self)) as ChatDock
	assert_str(dock.current_channel).is_equal(ChatFixtures.PARTY_CHANNEL)
	assert_bool(dock.list().is_visible_in_tree()).is_true()
	assert_int(dock.list().entries.size()).is_equal(5)
	assert_bool(dock.composer().is_visible_in_tree()).is_true()
	assert_bool(dock.collapse_button().visible).is_true()
	for n: String in dock.control_names():
		assert_bool(n.to_lower().contains("off") or n.to_lower().contains("disable") or n.to_lower().contains("toggle_chat")) \
				.override_failure_message("chat off option: " + n).is_false()


func test_collapsed_dock_counts_new_messages_with_a_badge() -> void:
	var dock: ChatDock = auto_free(await s.party_dock(self)) as ChatDock
	dock.collapse_button().pressed.emit()
	assert_int(dock.mode).is_equal(ChatDock.Mode.COLLAPSED)
	assert_bool(dock.visible).is_true()
	assert_bool(dock.list().is_visible_in_tree()).is_false()
	s.push("chat.message.created", _created(16))
	s.push("chat.message.created", _created(17))
	assert_str(dock.unread_badge_text()).is_equal("2")
	dock.expand_button().pressed.emit()
	assert_int(dock.mode).is_equal(ChatDock.Mode.EXPANDED)
	assert_int(dock.total_unread()).is_equal(0)


func test_team_battle_has_team_and_match_tabs_with_their_own_badges() -> void:
	var dock: ChatDock = auto_free(await s.match_dock(self)) as ChatDock
	var team: Button = dock.tab_button(ChatFixtures.TEAM_CHANNEL)
	var whole: Button = dock.tab_button(ChatFixtures.MATCH_CHANNEL)
	assert_object(team).is_not_null()
	assert_object(whole).is_not_null()
	assert_str(team.text).is_equal("ui.chat.tab.team")
	assert_str(whole.text).is_equal("ui.chat.tab.match")
	assert_str(dock.current_channel).is_equal(ChatFixtures.TEAM_CHANNEL)
	s.chat.store.set_status(ChatFixtures.MATCH_CHANNEL, "open")
	s.push("chat.message.created", _created(9, ChatFixtures.OTHER, ChatFixtures.MATCH_CHANNEL))
	assert_str(dock.tab_badge_text(ChatFixtures.MATCH_CHANNEL)).is_not_empty()
	assert_str(dock.tab_badge_text(ChatFixtures.TEAM_CHANNEL)).is_empty()
	whole.pressed.emit()
	assert_str(dock.current_channel).is_equal(ChatFixtures.MATCH_CHANNEL)
	assert_str(dock.tab_badge_text(ChatFixtures.MATCH_CHANNEL)).is_empty()


func test_question_shrinks_the_dock_to_two_lines_and_messages_keep_coming() -> void:
	var dock: ChatDock = auto_free(await s.party_dock(self)) as ChatDock
	EventBus.question_shown.emit()
	assert_int(dock.mode).is_equal(ChatDock.Mode.MINI)
	assert_bool(dock.composer().is_visible_in_tree()).is_false()
	assert_bool(dock.list().is_visible_in_tree()).is_false()
	s.push("chat.message.created", _created(16))
	assert_int(s.chat.store.count(ChatFixtures.PARTY_CHANNEL)).is_equal(6)
	assert_int(dock.mini_text().split("\n").size()).is_less_equal(2)
	assert_str(dock.mini_text()).contains("CaHeoVui07")
	EventBus.question_hidden.emit()
	assert_int(dock.mode).is_equal(ChatDock.Mode.EXPANDED)


func test_closed_match_locks_writing_and_shows_the_retention_days_from_the_server() -> void:
	s.h.http.clear(ChatFixtures.ROUTE_CHANNELS).reply(ChatFixtures.ROUTE_CHANNELS, 200,
			ChatFixtures.channels_closed_match(s.now_ms(), 5))
	var dock: ChatDock = auto_free(await s.match_dock(self)) as ChatDock
	dock.select_channel(ChatFixtures.MATCH_CHANNEL)
	await s.settle(dock, 2)
	assert_bool(dock.composer().locked).is_true()
	assert_str(dock.composer().lock_text()).is_not_empty()
	assert_str(dock.retention_text()).contains("5")
	assert_bool(dock.composer().send_button().disabled).is_true()


func test_retention_days_follow_the_server_value_not_a_constant() -> void:
	s.h.http.clear(ChatFixtures.ROUTE_CHANNELS).reply(ChatFixtures.ROUTE_CHANNELS, 200,
			ChatFixtures.channels_closed_match(s.now_ms(), 3))
	var dock: ChatDock = auto_free(await s.match_dock(self)) as ChatDock
	dock.select_channel(ChatFixtures.MATCH_CHANNEL)
	await s.settle(dock, 2)
	assert_str(dock.retention_text()).contains("3")
	assert_str(dock.retention_text()).not_contains("7")


func test_reconnecting_banner_while_messages_wait() -> void:
	var dock: ChatDock = auto_free(await s.party_dock(self)) as ChatDock
	s.h.http.clear(ChatFixtures.ROUTE_SEND).fail(ChatFixtures.ROUTE_SEND)
	s.chat.send_text(ChatFixtures.PARTY_CHANNEL, "Chào cả nhóm!")
	await s.settle(dock, 30)
	assert_bool(s.chat.reconnecting).is_true()
	dock._render_banner()
	assert_str(dock.banner_text()).is_equal(tr("ui.chat.reconnecting"))
	var last: ChatMessageModel = dock.list().entries.back()
	assert_bool(last.is_pending()).is_true()
