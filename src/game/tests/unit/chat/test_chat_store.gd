extends GdUnitTestSuite
## T008 (F119-AC-04, AC-15, AC-20, AC-21; TC-CHAT-028): ChatStore orders by `seq` only, marks the student's message
## "sent" only on `chat.message.acked`, asks for one sync per gap and gives up what the server cannot fill, keeps at
## most `chat_recent_buffer` messages, moves own messages to delivered / read from the receipts, hides blocked players
## at once (never the student's own messages) and keeps no content of a recalled message. Pure — no Node, no network.

const SELF: String = "00000000-0000-4000-8000-000000000001"
const CH: String = ChatFixtures.PARTY_CHANNEL

var store: ChatStore = null
var gaps: Array[Array] = []


func before_test() -> void:
	store = ChatStore.new()
	store.self_id = SELF
	store.ensure_channel(ChatChannelModel.placeholder(CH, "party"))
	store.load_history(CH, _typed([]))
	gaps.clear()
	store.gap_detected.connect(func(id: String, after: int, n: int) -> void: gaps.append([id, after, n]))


func _msg(seq: int, sender: String = ChatFixtures.OTHER, body: String = "Chào cả nhóm") -> ChatMessageModel:
	return ChatMessageModel.from_dict({"message_id": Uuid.v4(), "channel_id": CH, "seq": seq, "kind": "text", "body": body,
		"sender": {"user_id": sender, "nickname": "CaHeoVui07" if sender != SELF else "MeoXanh42"}, "created_at": 1763712000000 + seq})


func _typed(list: Array) -> Array[ChatMessageModel]:
	var out: Array[ChatMessageModel] = []
	for m: Variant in list:
		out.append(m as ChatMessageModel)
	return out


func _seqs() -> Array[int]:
	var out: Array[int] = []
	for m: ChatMessageModel in store.visible(CH):
		out.append(m.seq)
	return out


func test_messages_are_ordered_by_seq_not_by_arrival() -> void:
	store.insert(_msg(1))
	store.insert(_msg(3))
	store.insert(_msg(2))
	assert_array(_seqs()).is_equal([1, 2, 3])


func test_own_message_is_pending_until_acked_then_sent() -> void:
	var m: ChatMessageModel = ChatMessageModel.pending(CH, SELF, "MeoXanh42", "text", 1)
	m.body = "Mình sẵn sàng"
	store.add_pending(m)
	assert_bool(store.visible(CH).back().is_pending()).is_true()
	# A created event of someone else does not change the pending one.
	store.insert(_msg(1))
	assert_bool(m.is_pending()).is_true()
	var acked: ChatMessageModel = store.acknowledge({"client_message_id": m.client_message_id, "message_id": Uuid.v4(),
		"channel_id": CH, "seq": 2, "moderation_state": "allowed"})
	assert_object(acked).is_same(m)
	assert_int(m.local_state).is_equal(ChatMessageModel.LocalState.SENT)
	assert_int(m.seq).is_equal(2)
	assert_array(store.all_pending()).is_empty()


func test_ack_of_an_unknown_client_message_id_is_ignored() -> void:
	assert_object(store.acknowledge({"client_message_id": Uuid.v4(), "message_id": Uuid.v4(), "channel_id": CH, "seq": 4})).is_null()
	assert_int(store.count(CH)).is_equal(0)


func test_created_before_ack_keeps_one_copy() -> void:
	var m: ChatMessageModel = ChatMessageModel.pending(CH, SELF, "MeoXanh42", "text", 1)
	store.add_pending(m)
	var id: String = Uuid.v4()
	var created: ChatMessageModel = _msg(1, SELF)
	created.message_id = id
	store.insert(created)
	store.acknowledge({"client_message_id": m.client_message_id, "message_id": id, "channel_id": CH, "seq": 1})
	assert_int(store.visible(CH).size()).is_equal(1)
	assert_int(store.visible(CH)[0].local_state).is_equal(ChatMessageModel.LocalState.SENT)


func test_gap_asks_once_and_unfillable_gap_is_given_up_after_one_attempt() -> void:
	store.insert(_msg(1))
	store.insert(_msg(4))
	assert_int(gaps.size()).is_equal(1)
	assert_array(gaps[0]).is_equal([CH, 1, 2])
	store.begin_sync(CH)
	# The sync brought seq 2 only; seq 3 (held message of another student) never comes.
	store.load_history(CH, _typed([_msg(2)]))
	store.end_sync(CH)
	assert_int(store.contiguous_seq(CH)).is_equal(4)
	store.insert(_msg(5))
	assert_int(gaps.size()).is_equal(1)


func test_no_gap_check_while_syncing() -> void:
	store.insert(_msg(1))
	store.begin_sync(CH)
	store.insert(_msg(9))
	assert_int(gaps.size()).is_equal(0)


func test_at_most_recent_buffer_messages_are_kept() -> void:
	for i: int in range(1, ChatStore.RECENT_BUFFER + 61):
		store.insert(_msg(i))
	assert_int(store.count(CH)).is_equal(ChatStore.RECENT_BUFFER)
	assert_int(store.oldest_seq(CH)).is_equal(61)
	assert_bool(store.state(CH).has_older).is_true()


func test_receipts_move_own_messages_to_delivered_then_read() -> void:
	var mine: ChatMessageModel = _msg(3, SELF)
	store.insert(mine)
	store.apply_receipt({"channel_id": CH, "member": {"user_id": ChatFixtures.OTHER, "nickname": "CaHeoVui07"},
		"delivered_seq": 3, "read_seq": 1})
	assert_int(mine.local_state).is_equal(ChatMessageModel.LocalState.DELIVERED)
	store.apply_receipt({"channel_id": CH, "member": {"user_id": ChatFixtures.OTHER, "nickname": "CaHeoVui07"},
		"delivered_seq": 2, "read_seq": 3})
	assert_int(mine.local_state).is_equal(ChatMessageModel.LocalState.READ)
	assert_array(Array(store.readers(mine)["read"])).contains(["CaHeoVui07"])


func test_hidden_player_leaves_the_list_at_once_but_own_messages_stay() -> void:
	store.insert(_msg(1, ChatFixtures.OTHER))
	store.insert(_msg(2, SELF))
	store.apply_typing(CH, [{"user_id": ChatFixtures.OTHER, "nickname": "CaHeoVui07"}])
	store.hide_user(ChatFixtures.OTHER)
	assert_array(_seqs()).is_equal([2])
	assert_array(Array(store.typing_nicknames(CH))).is_empty()
	store.hide_user(SELF)
	assert_array(_seqs()).is_equal([2])


func test_recall_keeps_no_content_and_replies_lose_their_excerpt() -> void:
	var original: ChatMessageModel = _msg(1)
	store.insert(original)
	var reply: ChatMessageModel = _msg(2)
	reply.reply_to_message_id = original.message_id
	reply.reply_excerpt = "Chào cả nhóm"
	store.insert(reply)
	store.apply_pins(CH, [original.message_id])
	store.apply_gone({"channel_id": CH, "message_id": original.message_id}, false)
	assert_bool(original.recalled).is_true()
	assert_str(original.body).is_empty()
	assert_str(reply.reply_excerpt).is_empty()
	assert_array(Array(store.pins(CH))).is_empty()
	assert_str(ChatText.visible_text(original)).is_equal(tr(ChatText.KEY_RECALLED))


func test_edit_applies_only_a_newer_revision() -> void:
	var m: ChatMessageModel = _msg(1)
	store.insert(m)
	store.apply_edit({"channel_id": CH, "message_id": m.message_id, "body": "Bản mới", "revision": 2, "edited_at": 1763712100000})
	store.apply_edit({"channel_id": CH, "message_id": m.message_id, "body": "Bản cũ", "revision": 2, "edited_at": 1763712000000})
	assert_str(m.body).is_equal("Bản mới")
	assert_int(m.edited_at_ms).is_greater(0)


func test_unread_counts_only_while_not_viewing() -> void:
	store.set_viewing(CH, true)
	store.insert(_msg(1))
	assert_int(store.unread(CH)).is_equal(0)
	store.set_viewing(CH, false)
	store.insert(_msg(2))
	store.add_mention(CH)
	assert_int(store.unread(CH)).is_equal(1)
	assert_int(store.mentions(CH)).is_equal(1)


func test_sanction_lock_ends_at_its_time() -> void:
	store.apply_sanction({"sanction_type": "mute", "until": 5000, "reason_code": "auto_spam"})
	assert_str(store.sanction_lock(4000)).is_equal("mute")
	assert_str(store.sanction_lock(6000)).is_empty()
	store.apply_sanction({"sanction_type": "restrict_competition"})
	assert_str(store.sanction_lock(1)).is_empty()
