extends GdUnitTestSuite
## T017, T023 menu side (F119-AC-12, AC-13, AC-14, AC-15, AC-19; TC-CHAT-019, TC-CHAT-041, AC-MOD-023, SC-001): every
## message of another player opens a menu with Tắt tiếng · Chặn · Báo cáo by a long press / right click / "⋯" — the
## report sheet is the second action; the student's own messages have none of them; recall always asks; copy takes
## what the bubble shows; the report goes out with the party context and ends with the thanks + "Chặn"; the players
## list reports / blocks a player without a message; a block hides the player's messages at once.

var s: ChatSceneHarness = null
var dock: ChatDock = null


func before_test() -> void:
	s = ChatSceneHarness.new()
	await s.start()
	# After the sign-in (the bootstrap sets the band of the sample student).
	AppState.set_grade_band("g4_5")
	dock = auto_free(await s.party_dock(self, Vector2(640, 620))) as ChatDock


func after_test() -> void:
	s.dispose()


func _message(id: String) -> ChatMessageModel:
	return s.chat.store.find(ChatFixtures.PARTY_CHANNEL, id)


## The bubble of message `id`, scrolled into view first (only bubbles in view exist).
func _bubble(id: String) -> ChatBubble:
	dock.list().scroll_to_key(id)
	await s.settle(dock, 3)
	return dock.list().bubble_for(id)


func _sheet_button(name: String) -> Button:
	return dock.sheet().find_child(name, true, false) as Button


func test_right_click_then_report_opens_the_report_sheet_in_two_actions() -> void:
	var bubble: ChatBubble = await _bubble(ChatFixtures.MSG_TEXT)
	assert_object(bubble).is_not_null()
	var click: InputEventMouseButton = InputEventMouseButton.new()
	click.button_index = MOUSE_BUTTON_RIGHT
	click.pressed = true
	bubble._gui_input(click)
	assert_bool(dock.sheet().visible).is_true()
	assert_object(_sheet_button("Act_mute")).is_not_null()
	assert_object(_sheet_button("Act_block")).is_not_null()
	_sheet_button("Act_report").pressed.emit()
	assert_str(String(dock.dialogs().view)).is_equal("report")
	assert_int(dock.sheet().find_child("Reasons", true, false).get_child_count()).is_equal(7)


func test_long_press_and_the_menu_button_open_the_same_menu() -> void:
	var bubble: ChatBubble = await _bubble(ChatFixtures.MSG_QUICK)
	var touch: InputEventScreenTouch = InputEventScreenTouch.new()
	touch.pressed = true
	bubble._gui_input(touch)
	bubble._process(ChatBubble.LONG_PRESS_SEC + 0.05)
	assert_bool(dock.sheet().visible).is_true()
	dock.sheet().dismiss()
	bubble.menu_button().pressed.emit()
	assert_bool(dock.sheet().visible).is_true()


func test_own_message_has_no_mute_block_or_report() -> void:
	var shown: Array[StringName] = dock.dialogs().open_menu(_message(ChatFixtures.MSG_MINE))
	assert_array(shown).not_contains([ChatDialogs.ACT_MUTE, ChatDialogs.ACT_BLOCK, ChatDialogs.ACT_REPORT])
	assert_array(shown).contains([ChatDialogs.ACT_EDIT, ChatDialogs.ACT_RECALL, ChatDialogs.ACT_COPY])
	var theirs: Array[StringName] = dock.dialogs().open_menu(_message(ChatFixtures.MSG_TEXT))
	assert_array(theirs).contains([ChatDialogs.ACT_MUTE, ChatDialogs.ACT_BLOCK, ChatDialogs.ACT_REPORT, ChatDialogs.ACT_REPLY])
	assert_array(theirs).not_contains([ChatDialogs.ACT_EDIT, ChatDialogs.ACT_RECALL])


func test_recall_always_asks_first() -> void:
	dock.dialogs().open_menu(_message(ChatFixtures.MSG_MINE))
	_sheet_button("Act_recall").pressed.emit()
	assert_str(String(dock.dialogs().view)).is_equal("recall")
	assert_bool(_message(ChatFixtures.MSG_MINE).recalled).is_false()
	dock.sheet().footer_button(ChatSheet.CANCEL).pressed.emit()
	assert_bool(_message(ChatFixtures.MSG_MINE).recalled).is_false()


func test_copy_takes_the_masked_text_shown() -> void:
	var m: ChatMessageModel = _message(ChatFixtures.MSG_MASKED)
	assert_str(s.chat.copy_text(m)).is_equal("Nhóm mình *** quá!")
	dock.dialogs().open_menu(m)
	_sheet_button("Act_copy").pressed.emit()
	assert_str(dock.notice_text()).is_equal(tr("ui.chat.copied"))


func test_report_goes_out_with_the_party_context_then_offers_to_block() -> void:
	dock.dialogs().open_menu(_message(ChatFixtures.MSG_TEXT))
	_sheet_button("Act_report").pressed.emit()
	var send: Button = dock.sheet().footer_button(ChatDialogs.ACT_SEND)
	assert_bool(send.disabled).is_true()
	_sheet_button("Reason_rude_language").pressed.emit()
	assert_bool(send.disabled).is_false()
	send.pressed.emit()
	await s.settle(dock, 4)
	var rec: FakeHttp.Recorded = s.h.http.last(ChatFixtures.ROUTE_REPORT)
	assert_dict(rec.body).is_equal({"target_type": "message", "target_id": ChatFixtures.MSG_TEXT, "reason": "rude_language",
		"context": {"channel_id": ChatFixtures.PARTY_CHANNEL, "party_id": ChatFixtures.PARTY_ID}})
	assert_str(String(dock.dialogs().view)).is_equal("thanks")
	dock.sheet().footer_button(ChatDialogs.ACT_BLOCK).pressed.emit()
	await s.settle(dock, 4)
	assert_str(s.h.http.last(ChatFixtures.ROUTE_BLOCK).path).is_equal("/social/blocks/" + ChatFixtures.THIRD)


func test_block_from_the_menu_hides_the_player_at_once() -> void:
	dock.dialogs().open_menu(_message(ChatFixtures.MSG_QUICK))
	_sheet_button("Act_block").pressed.emit()
	await s.settle(dock, 4)
	for m: ChatMessageModel in dock.list().entries:
		assert_str(m.sender_id).is_not_equal(ChatFixtures.OTHER)
	assert_str(dock.notice_text()).contains("CaHeoVui07")


func test_players_list_reports_a_player_with_the_party_id() -> void:
	dock.players_button().pressed.emit()
	var list: ChatPlayerSafety = dock.sheet().find_child("ChatPlayerSafety", true, false) as ChatPlayerSafety
	assert_int(list.player_count()).is_equal(2)
	list.report_button(ChatFixtures.OTHER).pressed.emit()
	_sheet_button("Reason_bullying").pressed.emit()
	dock.sheet().footer_button(ChatDialogs.ACT_SEND).pressed.emit()
	await s.settle(dock, 4)
	var rec: FakeHttp.Recorded = s.h.http.last(ChatFixtures.ROUTE_REPORT)
	assert_str(String(rec.body["target_type"])).is_equal("user")
	assert_str(String(rec.body["target_id"])).is_equal(ChatFixtures.OTHER)
	assert_str(String((rec.body["context"] as Dictionary)["party_id"])).is_equal(ChatFixtures.PARTY_ID)


func test_match_result_safety_row_reports_with_the_match_id() -> void:
	var row: ChatPlayerSafety = auto_free(ChatPlayerSafety.new()) as ChatPlayerSafety
	add_child(row)
	row.set_players(ChatSceneHarness.others())
	var sheet: ChatSheet = auto_free(ChatSheet.new()) as ChatSheet
	add_child(sheet)
	var dialogs: ChatDialogs = ChatDialogs.new(sheet)
	dialogs.bind(s.chat, s.services.social, s.services.reports)
	dialogs.context = {"match_id": ChatFixtures.MATCH_ID}
	row.report_requested.connect(func(id: String, nick: String) -> void: dialogs.open_report("user", id, nick, id))
	row.block_requested.connect(func(id: String, nick: String) -> void: dialogs.relation(ChatDialogs.ACT_BLOCK, id, nick))
	assert_str(row.report_button(ChatFixtures.THIRD).text).is_equal("ui.chat.players.report")
	row.report_button(ChatFixtures.THIRD).pressed.emit()
	(sheet.find_child("Reason_cheating", true, false) as Button).pressed.emit()
	sheet.footer_button(ChatDialogs.ACT_SEND).pressed.emit()
	await s.settle(row, 4)
	assert_dict(s.h.http.last(ChatFixtures.ROUTE_REPORT).body["context"] as Dictionary).is_equal({"match_id": ChatFixtures.MATCH_ID})
	row.block_button(ChatFixtures.THIRD).pressed.emit()
	await s.settle(row, 4)
	assert_str(s.h.http.last(ChatFixtures.ROUTE_BLOCK).path).is_equal("/social/blocks/" + ChatFixtures.THIRD)
