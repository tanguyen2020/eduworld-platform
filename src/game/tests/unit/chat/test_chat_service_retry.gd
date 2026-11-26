extends GdUnitTestSuite
## T009 (F119-AC-04, AC-05, AC-06; TC-CHAT-026, TC-CHAT-028, TC-CHAT-020; SC-003): ChatService on a real
## GameRealtimeClient (FakeWebSocketPeer, AsyncAPI example frames): "sent" only with the server id + seq;
## `CHAT_TEMPORARILY_UNAVAILABLE` / no connection keeps the message pending ("Đang kết nối lại chat…") and resends it
## with the same `client_message_id` on `client_reconnect_backoff` — one message on the server, no duplicate; a gameplay
## channel not created yet (`NOT_FOUND` on join) is joined again on the backoff; `CHAT_CHANNEL_CLOSED` closes the
## channel; a seq gap asks one `chat.channel.sync`. TEST-ONLY fakes.

const SELF: String = "00000000-0000-4000-8000-000000000001"
const CH: String = ChatFixtures.PARTY_CHANNEL

var h: RealtimeHarness = null
var chat: ChatService = null


func before_test() -> void:
	h = RealtimeHarness.create()
	await h.connect_online()
	chat = ChatService.new(h.api, h.client)
	chat.rng.seed = 119
	chat.store.self_id = SELF
	chat.store.ensure_channel(ChatChannelModel.placeholder(CH, "party"))
	chat.store.load_history(CH, _none())


func after_test() -> void:
	h.client.unregister_handler(ChatService.PREFIX, chat.handle_message)
	chat.clear()
	h.dispose()


func _none() -> Array[ChatMessageModel]:
	var out: Array[ChatMessageModel] = []
	return out


func _sent(type: String) -> Array[Dictionary]:
	return h.peer().sent_of(type)


## Answers the last command of `type` with `system.command.failed` (`code`, optional details).
func _fail_last(type: String, code: String, details: Dictionary = {}) -> void:
	var sent: Array[Dictionary] = _sent(type)
	var payload: Dictionary = {"code": code, "message": "x"}
	if not details.is_empty():
		payload["details"] = details
	h.push(h.envelope("system.command.failed", payload, String(sent[sent.size() - 1]["id"])))


func _ack_send(seq: int) -> String:
	var sent: Array[Dictionary] = _sent(ChatService.CMD_SEND)
	var cmd: Dictionary = sent[sent.size() - 1]
	var mid: String = Uuid.v4()
	h.push(h.envelope("system.command.acked", {"result": {"client_message_id": (cmd["payload"] as Dictionary)["client_message_id"],
		"message_id": mid, "channel_id": CH, "seq": seq, "moderation_state": "allowed"}}, String(cmd["id"])))
	return mid


func test_message_is_sent_only_when_the_server_acknowledged_it() -> void:
	var m: ChatMessageModel = chat.send_text(CH, "Chào cả nhóm!")
	await get_tree().process_frame
	assert_bool(m.is_pending()).is_true()
	var payload: Dictionary = _sent(ChatService.CMD_SEND)[0]["payload"] as Dictionary
	assert_str(String(payload["client_message_id"])).is_equal(m.client_message_id)
	assert_str(String(payload["kind"])).is_equal("text")
	_ack_send(16)
	assert_int(m.local_state).is_equal(ChatMessageModel.LocalState.SENT)
	assert_int(m.seq).is_equal(16)
	assert_int(chat.pending_count()).is_equal(0)


func test_temporarily_unavailable_keeps_it_pending_and_resends_the_same_client_message_id() -> void:
	var reconnecting: Array[bool] = []
	chat.connection_changed.connect(func(on: bool) -> void: reconnecting.append(on))
	var m: ChatMessageModel = chat.send_text(CH, "Đợi mình nhé")
	await get_tree().process_frame
	_fail_last(ChatService.CMD_SEND, "CHAT_TEMPORARILY_UNAVAILABLE")
	assert_bool(m.is_pending()).is_true()
	assert_bool(chat.reconnecting).is_true()
	assert_array(reconnecting).contains([true])
	# Not before the backoff (0.5 s ± 20 % for the first retry)…
	chat.tick()
	assert_int(_sent(ChatService.CMD_SEND).size()).is_equal(1)
	h.advance_ms(700)
	chat.tick()
	await get_tree().process_frame
	var sends: Array[Dictionary] = _sent(ChatService.CMD_SEND)
	assert_int(sends.size()).is_equal(2)
	assert_str(String((sends[1]["payload"] as Dictionary)["client_message_id"])).is_equal(m.client_message_id)
	_ack_send(17)
	assert_int(m.local_state).is_equal(ChatMessageModel.LocalState.SENT)
	assert_int(chat.store.visible(CH).size()).is_equal(1)
	assert_bool(chat.reconnecting).is_false()


func test_without_connection_the_rest_fallback_is_retried_with_the_same_key_and_no_duplicate() -> void:
	h.peer().server_close(1006)
	h.client.tick()
	assert_bool(h.client.can_write()).is_false()
	h.http.fail(ChatFixtures.ROUTE_SEND)
	var m: ChatMessageModel = chat.send_text(CH, "Mình vào ngay")
	for i: int in 30:
		await get_tree().process_frame
	assert_bool(m.is_pending()).is_true()
	var first: Array[FakeHttp.Recorded] = h.http.find(ChatFixtures.ROUTE_SEND)
	assert_int(first.size()).is_greater_equal(1)
	assert_str(String(first[0].headers.get("idempotency-key", ""))).is_equal(m.client_message_id)
	h.http.clear(ChatFixtures.ROUTE_SEND).reply(ChatFixtures.ROUTE_SEND, 201,
			ChatFixtures.sent_message({"channel_id": CH, "kind": "text", "body": "Mình vào ngay"}, 18, SELF))
	for i: int in 20:
		h.advance_ms(2000)
		chat.tick()
		await get_tree().process_frame
		if not m.is_pending():
			break
	assert_int(m.local_state).is_equal(ChatMessageModel.LocalState.SENT)
	for rec: FakeHttp.Recorded in h.http.find(ChatFixtures.ROUTE_SEND):
		assert_str(String(rec.body["client_message_id"])).is_equal(m.client_message_id)
		assert_str(String(rec.headers.get("idempotency-key", ""))).is_equal(m.client_message_id)
	assert_int(chat.store.visible(CH).size()).is_equal(1)


func test_join_not_found_is_retried_on_the_backoff() -> void:
	var joining: bool = false
	chat.join(CH)
	await get_tree().process_frame
	_fail_last(ChatService.CMD_JOIN, "NOT_FOUND")
	assert_bool(chat.join_pending(CH)).is_true()
	assert_bool(chat.is_joined(CH)).is_false()
	h.advance_ms(700)
	chat.tick()
	await get_tree().process_frame
	assert_int(_sent(ChatService.CMD_JOIN).size()).is_equal(2)
	h.ack_last(ChatService.CMD_JOIN)
	joining = chat.is_joined(CH)
	assert_bool(joining).is_true()
	assert_bool(chat.join_pending(CH)).is_false()


func test_channel_closed_answer_locks_the_channel_and_drops_the_message() -> void:
	var notices: Array[String] = []
	chat.feedback_requested.connect(func(key: String, _a: Dictionary, _t: String) -> void: notices.append(key))
	h.http.reply(ChatFixtures.ROUTE_CHANNELS, 200, {"data": []})
	chat.send_text(CH, "Còn ai không?")
	await get_tree().process_frame
	_fail_last(ChatService.CMD_SEND, "CHAT_CHANNEL_CLOSED")
	assert_bool(chat.store.channel(CH).is_closed()).is_true()
	assert_array(chat.store.visible(CH)).is_empty()
	assert_array(notices).contains(["ui.chat.feedback.channel_closed"])
	assert_object(chat.send_text(CH, "Thêm một tin")).is_null()


func test_a_seq_gap_sends_one_channel_sync_from_the_cursor() -> void:
	chat.handle_message("chat.message.created", _created(1))
	chat.handle_message("chat.message.created", _created(5))
	await get_tree().process_frame
	var syncs: Array[Dictionary] = _sent(ChatService.CMD_SYNC)
	assert_int(syncs.size()).is_equal(1)
	assert_int(int(((syncs[0]["payload"] as Dictionary)["channels"] as Dictionary)[CH])).is_equal(1)
	h.push_fixture("chat.history.loaded.sync")
	assert_bool(chat.store.is_syncing(CH)).is_false()
	chat.handle_message("chat.message.created", _created(6))
	assert_int(_sent(ChatService.CMD_SYNC).size()).is_equal(1)


func test_seq_gap_larger_than_the_buffer_loads_the_rest_history() -> void:
	h.http.reply(ChatFixtures.ROUTE_MESSAGES, 200, ChatFixtures.history())
	chat.handle_message("chat.message.created", _created(1))
	chat.handle_message("chat.message.created", _created(ChatStore.RECENT_BUFFER + 10))
	for i: int in 6:
		await get_tree().process_frame
	assert_int(_sent(ChatService.CMD_SYNC).size()).is_equal(0)
	assert_int(h.http.count(ChatFixtures.ROUTE_MESSAGES)).is_greater_equal(1)


func _created(seq: int) -> Dictionary:
	var p: Dictionary = ChatFixtures.ws("chat.message.created.text")
	p["message_id"] = Uuid.v4()
	p["seq"] = seq
	return p
