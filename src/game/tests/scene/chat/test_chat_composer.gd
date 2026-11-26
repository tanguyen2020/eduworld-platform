extends GdUnitTestSuite
## T013 (F119-AC-08, AC-09, AC-10; TC-I18N-008): the quick-message row is always above the input and sends
## `quick_message_id` in one tap, in the viewer's language; g1_3 gets taller quick buttons and the emoji board open by
## default; the counter follows the channel limit, Enter sends and Shift+Enter breaks the line; a school preset quick +
## emote hides the free text row; a blocked message shows the friendly sentence, never the text.

var s: ChatSceneHarness = null


func before_test() -> void:
	s = ChatSceneHarness.new()
	await s.start()
	# After the sign-in (the bootstrap sets the band of the sample student).
	AppState.set_grade_band("g4_5")


func after_test() -> void:
	AppState.set_grade_band("g4_5")
	s.dispose()


func _key(code: Key, shift: bool = false) -> InputEventKey:
	var e: InputEventKey = InputEventKey.new()
	e.keycode = code
	e.pressed = true
	e.shift_pressed = shift
	return e


func test_quick_row_is_above_the_input_and_sends_the_id_in_one_tap() -> void:
	var dock: ChatDock = auto_free(await s.party_dock(self)) as ChatDock
	var c: ChatComposer = dock.composer()
	var quick: Array[Node] = c.quick_buttons()
	assert_int(quick.size()).is_equal(6)
	assert_bool((quick[0] as Control).is_visible_in_tree()).is_true()
	var bar: Control = c.find_child("QuickBar", true, false) as Control
	var row: Control = c.find_child("InputRow", true, false) as Control
	assert_int(bar.get_index()).is_less(row.get_index())
	s.h.http.reply(ChatFixtures.ROUTE_SEND, 201, ChatFixtures.sent_message({"kind": "quick"}, 16, s.self_id))
	(quick[5] as Button).pressed.emit()
	await s.settle(dock, 4)
	var rec: FakeHttp.Recorded = s.h.http.last(ChatFixtures.ROUTE_SEND)
	assert_str(String(rec.body["kind"])).is_equal("quick")
	assert_str(String(rec.body["quick_message_id"])).is_equal("qm_good_luck")
	assert_bool(rec.body.has("body")).is_false()


func test_quick_words_follow_the_viewer_language() -> void:
	var dock: ChatDock = auto_free(await s.party_dock(self)) as ChatDock
	var before: String = String(Settings.locale)
	Settings.set_locale("en-US", false)
	EventBus.locale_changed.emit("en-US")
	var en: String = (dock.composer().quick_buttons()[5] as Button).text
	Settings.set_locale(before, false)
	EventBus.locale_changed.emit(before)
	var vi: String = (dock.composer().quick_buttons()[5] as Button).text
	assert_str(en).is_equal("Good luck!")
	assert_str(vi).is_equal("Chúc may mắn!")


func test_g1_3_has_taller_quick_buttons_and_the_emoji_board_open() -> void:
	AppState.set_grade_band("g1_3")
	var dock: ChatDock = auto_free(await s.party_dock(self)) as ChatDock
	var b: Button = dock.composer().quick_buttons()[0] as Button
	assert_float(b.custom_minimum_size.y).is_greater(UiBuild.target())
	assert_bool(dock.composer().picker().visible).is_true()
	assert_str(String(dock.composer().picker().current_tab)).is_equal("emoji")
	AppState.set_grade_band("g6_7")
	var older: ChatDock = auto_free(await s.party_dock(self)) as ChatDock
	assert_bool(older.composer().picker().visible).is_false()


func test_counter_follows_the_channel_limit_and_blocks_sending_over_it() -> void:
	var body: Dictionary = ChatFixtures.channels()
	((body["data"] as Array)[0] as Dictionary)["max_text_length"] = 20
	s.h.http.clear(ChatFixtures.ROUTE_CHANNELS).reply(ChatFixtures.ROUTE_CHANNELS, 200, body)
	var dock: ChatDock = auto_free(await s.party_dock(self)) as ChatDock
	var c: ChatComposer = dock.composer()
	c.input().text = "Chào"
	c.input().text_changed.emit()
	assert_str(c.counter_text()).is_equal(tr("ui.chat.counter").format({"count": 4, "max": 20}))
	assert_bool(c.send_button().disabled).is_false()
	c.input().text = "Một câu khá là dài hơn hai mươi ký tự"
	c.input().text_changed.emit()
	assert_bool(c.send_button().disabled).is_true()


func test_enter_sends_and_shift_enter_does_not() -> void:
	var dock: ChatDock = auto_free(await s.party_dock(self)) as ChatDock
	var c: ChatComposer = dock.composer()
	var sent: Array[String] = []
	c.text_submitted.connect(func(text: String, _r: String, _q: Dictionary, _m: PackedStringArray) -> void: sent.append(text))
	c.input().text = "Dòng một"
	c.input()._gui_input(_key(KEY_ENTER, true))
	assert_array(sent).is_empty()
	c.input()._gui_input(_key(KEY_ENTER))
	assert_array(sent).is_equal(["Dòng một"])
	assert_str(c.input().text).is_empty()


func test_each_message_keeps_its_own_client_message_id() -> void:
	var dock: ChatDock = auto_free(await s.party_dock(self)) as ChatDock
	s.h.http.reply(ChatFixtures.ROUTE_SEND, 201, ChatFixtures.sent_message({"kind": "text"}, 16, s.self_id))
	var a: ChatMessageModel = s.chat.send_text(ChatFixtures.PARTY_CHANNEL, "Một")
	var b: ChatMessageModel = s.chat.send_text(ChatFixtures.PARTY_CHANNEL, "Hai")
	assert_str(a.client_message_id).is_not_equal(b.client_message_id)
	assert_bool(Uuid.is_valid(a.client_message_id)).is_true()
	await s.settle(dock, 2)


func test_school_preset_quick_and_emote_hides_the_free_text_row() -> void:
	s.h.http.clear(ChatFixtures.ROUTE_CHANNELS).reply(ChatFixtures.ROUTE_CHANNELS, 200, ChatFixtures.channels_with(["quick", "emoji"]))
	var dock: ChatDock = auto_free(await s.party_dock(self)) as ChatDock
	assert_bool(dock.composer().text_row_visible()).is_false()
	assert_bool((dock.composer().quick_buttons()[0] as Control).is_visible_in_tree()).is_true()
	assert_bool(dock.composer().picker().tab_button(ChatPicker.TAB_STICKER).visible).is_false()
	assert_bool(dock.composer().picker().tab_button(ChatPicker.TAB_GIF).visible).is_false()


func test_blocked_message_shows_the_friendly_sentence_never_the_text() -> void:
	var dock: ChatDock = auto_free(await s.party_dock(self)) as ChatDock
	var problem: Dictionary = AuthHarness.problem(451, "CHAT_CONTENT_BLOCKED")
	problem["category_code"] = "pii"
	s.h.http.reply(ChatFixtures.ROUTE_SEND, 451, problem)
	dock.composer().input().text = "placeholder-private-detail"
	dock.composer().submit()
	await s.settle(dock, 6)
	assert_str(dock.notice_text()).is_equal(tr("ui.chat.feedback.pii_blocked"))
	assert_str(dock.notice_text()).not_contains("placeholder-private-detail")
	assert_str(dock.composer().input().text).is_equal("placeholder-private-detail")


func test_smile_board_and_text_row_take_turns() -> void:
	var dock: ChatDock = auto_free(await s.party_dock(self)) as ChatDock
	var c: ChatComposer = dock.composer()
	assert_bool(c.text_row_visible()).is_true()
	assert_bool(c.picker().visible).is_false()
	c.smile_button().pressed.emit()
	assert_bool(c.picker().visible).is_true()
	assert_bool(c.text_row_visible()).is_false()
	(c.picker().find_child("Keyboard", true, false) as Button).pressed.emit()
	assert_bool(c.text_row_visible()).is_true()
	assert_bool(c.picker().visible).is_false()
