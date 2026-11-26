extends GdUnitTestSuite
## T022, T023 view side (F119-AC-16, AC-17, AC-18, AC-20; TC-CHAT-033, TC-CHAT-034, TC-CHAT-038, TC-PERF-019): bubbles
## show the reaction counts (the student's own marked), "đã chỉnh sửa", "Đã chuyển tiếp từ …", the recall placeholder,
## "Tin nhắn gốc không còn", the typing line for at most 3 nicknames and the state of the student's messages with an icon
## and a word; the smile board has Emoji / Sticker / GIF tabs, a lock + hint on a locked pack, topic chips from the
## internal library and no text field; at most 3 pictures animate.

var s: ChatSceneHarness = null
var dock: ChatDock = null


func before_test() -> void:
	s = ChatSceneHarness.new()
	await s.start()
	# After the sign-in (the bootstrap sets the band of the sample student).
	AppState.set_grade_band("g6_7")
	dock = auto_free(await s.party_dock(self, Vector2(700, 760))) as ChatDock


func after_test() -> void:
	AppState.set_grade_band("g4_5")
	s.dispose()


func _bubble(id: String) -> ChatBubble:
	return dock.list().bubble_for(id)


func test_reactions_show_counts_and_the_students_own() -> void:
	var b: ChatBubble = _bubble(ChatFixtures.MSG_MINE)
	assert_int(b.reaction_count()).is_equal(2)
	var upd: Dictionary = ChatFixtures.ws("chat.reaction.updated.counts")
	s.push("chat.reaction.updated", upd)
	s.chat.store.set_my_reaction(ChatFixtures.MSG_MINE, "heart", true)
	await s.settle(dock, 2)
	assert_object(b.find_child("*", true, false)).is_not_null()
	var mine_pills: int = 0
	for n: Node in b.find_children("*", "PanelContainer", true, false):
		if (n as PanelContainer).theme_type_variation == &"ReactionPillMine":
			mine_pills += 1
	assert_int(mine_pills).is_equal(1)


func test_edited_reply_forwarded_and_recalled_labels() -> void:
	var edited: ChatBubble = _bubble(ChatFixtures.MSG_MASKED)
	assert_bool((edited.find_child("Edited", true, false) as Control).visible).is_true()
	assert_str((edited.find_child("QuoteText", true, false) as Label).text).contains("Đi thôi nào!")
	s.push_example("chat.message.created.reply_forwarded")
	await s.settle(dock, 3)
	var fwd: ChatBubble = _bubble("0193b0a0-0000-7000-8000-000000000621")
	assert_str((fwd.find_child("Forwarded", true, false) as Label).text).is_equal(
			tr("ui.chat.forwarded_from").format({"nickname": "CaHeoVui07"}))
	s.push("chat.message.recalled", ChatFixtures.ws("chat.message.recalled.recalled"))
	await s.settle(dock, 3)
	assert_str(_bubble(ChatFixtures.MSG_TEXT).shown_text()).is_equal(tr("ui.chat.recalled"))


func test_reply_to_a_removed_message_says_the_original_is_gone() -> void:
	s.push("chat.message.removed", {"message_id": ChatFixtures.MSG_MINE, "channel_id": ChatFixtures.PARTY_CHANNEL,
		"reason": "moderation"})
	await s.settle(dock, 3)
	var reply: ChatBubble = _bubble(ChatFixtures.MSG_MASKED)
	assert_str((reply.find_child("QuoteText", true, false) as Label).text).contains(tr("ui.chat.original_gone"))


func test_typing_line_names_at_most_three_players() -> void:
	s.push("chat.typing.updated", {"channel_id": ChatFixtures.PARTY_CHANNEL, "typing": [
		{"user_id": ChatFixtures.OTHER, "nickname": "CaHeoVui07"}, {"user_id": ChatFixtures.THIRD, "nickname": "GauNau15"},
		{"user_id": Uuid.v4(), "nickname": "SaoBang12"}, {"user_id": Uuid.v4(), "nickname": "MayTrang9"}]})
	dock._render_typing()
	assert_str(dock.typing_text()).is_equal(tr("ui.chat.typing_many").format({"a": "CaHeoVui07", "b": "GauNau15", "c": "SaoBang12"}))
	assert_str(dock.typing_text()).not_contains("MayTrang9")


func test_state_of_own_messages_has_an_icon_and_a_word() -> void:
	var mine: ChatBubble = _bubble(ChatFixtures.MSG_MINE)
	assert_str(mine.state_text()).is_equal("ui.chat.state.sent")
	s.push("chat.receipt.updated", {"channel_id": ChatFixtures.PARTY_CHANNEL,
		"member": {"user_id": ChatFixtures.OTHER, "nickname": "CaHeoVui07"}, "delivered_seq": 13, "read_seq": 12})
	await s.settle(dock, 2)
	assert_str(mine.state_text()).is_equal("ui.chat.state.delivered")
	s.push("chat.receipt.updated", {"channel_id": ChatFixtures.PARTY_CHANNEL,
		"member": {"user_id": ChatFixtures.OTHER, "nickname": "CaHeoVui07"}, "delivered_seq": 13, "read_seq": 13})
	await s.settle(dock, 2)
	assert_str(mine.state_text()).is_equal("ui.chat.state.read")
	assert_str(_bubble(ChatFixtures.MSG_TEXT).state_text()).is_empty()


func test_mention_bubble_is_yellow_with_words() -> void:
	var p: Dictionary = ChatFixtures.ws("chat.message.created.text")
	p["message_id"] = Uuid.v4()
	p["seq"] = 30
	p["mentions"] = [{"user_id": s.self_id, "nickname": "MeoXanh42"}]
	s.push("chat.message.created", p)
	await s.settle(dock, 3)
	var b: ChatBubble = _bubble(String(p["message_id"]))
	assert_bool(b.is_mention_style()).is_true()
	assert_bool((b.find_child("Mention", true, false) as Control).visible).is_true()


func test_picker_has_three_tabs_a_locked_pack_hint_topics_and_no_text_field() -> void:
	var picker: ChatPicker = dock.composer().picker()
	picker.visible = true
	assert_int(picker.emoji_buttons().size()).is_equal(30)
	picker.tab_button(ChatPicker.TAB_STICKER).pressed.emit()
	await s.settle(dock, 4)
	assert_int(picker.sticker_buttons().size()).is_equal(2)
	assert_int(picker.locked_hints().size()).is_equal(1)
	var hint: Node = picker.locked_hints()[0]
	assert_object(hint.find_child("*", true, false)).is_not_null()
	picker.tab_button(ChatPicker.TAB_GIF).pressed.emit()
	await s.settle(dock, 4)
	assert_int(picker.topic_chips().size()).is_greater_equal(2)
	assert_int(picker.gif_buttons().size()).is_equal(2)
	assert_bool(picker.has_text_field()).is_false()
	for rec: FakeHttp.Recorded in s.h.http.requests:
		assert_bool(rec.url.begins_with(s.h.http.base_url) or rec.url.begins_with(ChatFixtures.CDN)).is_true()


func test_picking_an_emoji_sends_its_code() -> void:
	var picker: ChatPicker = dock.composer().picker()
	picker.visible = true
	s.h.http.reply(ChatFixtures.ROUTE_SEND, 201, ChatFixtures.sent_message({"kind": "emoji"}, 16, s.self_id))
	(picker.emoji_buttons()[5] as Button).pressed.emit()
	await s.settle(dock, 3)
	var rec: FakeHttp.Recorded = s.h.http.last(ChatFixtures.ROUTE_SEND)
	assert_str(String(rec.body["kind"])).is_equal("emoji")
	assert_str(String(rec.body["emoji"])).is_equal("thumbs_up")


func test_at_most_three_stickers_animate_at_once() -> void:
	s.chat.assets.load_packs(ChatFixtures.stickers()["data"])
	for i: int in 5:
		var p: Dictionary = ChatFixtures.ws("chat.message.created.sticker")
		p["message_id"] = Uuid.v4()
		p["seq"] = 40 + i
		s.push("chat.message.created", p)
	await s.settle(dock, 8)
	var playing: int = 0
	for b: ChatBubble in dock.list().bubbles():
		if b.is_animating():
			playing += 1
	assert_int(playing).is_less_equal(ChatAnimationBudget.MAX_PLAYING)
	assert_int(s.chat.budget.playing_count()).is_less_equal(ChatAnimationBudget.MAX_PLAYING)
