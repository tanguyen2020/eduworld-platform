extends GdUnitTestSuite
## T013/T014/T023 service side (F119-AC-08, AC-10, AC-11, AC-18, AC-19; TC-CHAT-015, TC-CHAT-019, TC-CHAT-034): a quick
## message travels as its id only; the server's moderation answers become friendly notices (blocked by PII / by a term,
## held, masked, rate limited) without echoing anything the student wrote, and the refused text comes back to the
## composer; `chat.sanction.applied` locks writing with the time it opens again, reading goes on; the reaction limit is
## a friendly notice; typing and receipts are throttled; recall / edit permissions follow the author and features.

const SELF: String = "00000000-0000-4000-8000-000000000001"
const CH: String = ChatFixtures.PARTY_CHANNEL

var h: RealtimeHarness = null
var chat: ChatService = null
var notices: Array[Dictionary] = []
var drafts: Array[String] = []


func before_test() -> void:
	h = RealtimeHarness.create()
	await h.connect_online()
	chat = ChatService.new(h.api, h.client)
	chat.store.self_id = SELF
	chat.store.ensure_channel(ChatChannelModel.placeholder(CH, "party"))
	var none: Array[ChatMessageModel] = []
	chat.store.load_history(CH, none)
	notices.clear()
	drafts.clear()
	chat.feedback_requested.connect(func(key: String, args: Dictionary, tone: String) -> void:
		notices.append({"key": key, "args": args, "tone": tone}))
	chat.draft_returned.connect(func(_c: String, text: String) -> void: drafts.append(text))


func after_test() -> void:
	h.client.unregister_handler(ChatService.PREFIX, chat.handle_message)
	chat.clear()
	h.dispose()


func _last_send() -> Dictionary:
	var sent: Array[Dictionary] = h.peer().sent_of(ChatService.CMD_SEND)
	return sent[sent.size() - 1]


func _fail_last(type: String, code: String, details: Dictionary = {}) -> void:
	var sent: Array[Dictionary] = h.peer().sent_of(type)
	var payload: Dictionary = {"code": code, "message": "x"}
	if not details.is_empty():
		payload["details"] = details
	h.push(h.envelope("system.command.failed", payload, String(sent[sent.size() - 1]["id"])))


## Pushes an example envelope with the next connection seq (older seqs are dropped as duplicates).
func _push(env: Dictionary) -> void:
	h.next_seq += 1
	env["seq"] = h.next_seq
	h.push(env)


func _keys() -> Array[String]:
	var out: Array[String] = []
	for n: Dictionary in notices:
		out.append(String(n["key"]))
	return out


func test_quick_message_travels_as_its_id_only() -> void:
	chat.send_quick(CH, "qm_good_luck")
	await get_tree().process_frame
	var payload: Dictionary = _last_send()["payload"] as Dictionary
	assert_str(String(payload["kind"])).is_equal("quick")
	assert_str(String(payload["quick_message_id"])).is_equal("qm_good_luck")
	assert_bool(payload.has("body")).is_false()


func test_pii_block_shows_the_safety_sentence_and_returns_the_draft_without_echo() -> void:
	chat.send_text(CH, "placeholder-contact-text")
	await get_tree().process_frame
	_fail_last(ChatService.CMD_SEND, "CHAT_CONTENT_BLOCKED", {"category_code": "pii"})
	assert_array(_keys()).contains(["ui.chat.feedback.pii_blocked"])
	assert_array(chat.store.visible(CH)).is_empty()
	assert_array(drafts).is_equal(["placeholder-contact-text"])
	for n: Dictionary in notices:
		assert_str(JSON.stringify(n)).not_contains("placeholder-contact-text")


func test_term_block_and_care_category_use_their_own_sentences() -> void:
	chat.send_text(CH, "neutral placeholder one")
	await get_tree().process_frame
	_fail_last(ChatService.CMD_SEND, "CHAT_CONTENT_BLOCKED", {"category_code": "insult"})
	chat.send_text(CH, "neutral placeholder two")
	await get_tree().process_frame
	_fail_last(ChatService.CMD_SEND, "CHAT_CONTENT_BLOCKED", {"category_code": "self_harm"})
	assert_array(_keys()).is_equal(["ui.chat.feedback.term_blocked", "ui.chat.feedback.care"])
	assert_str(String(notices[1]["tone"])).is_equal(ChatFeedbackMapper.TONE_CARE)
	# The care message does not bring the text back.
	assert_array(drafts).is_equal(["neutral placeholder one"])


func test_rate_limited_and_masked_and_held_get_friendly_words() -> void:
	chat.send_text(CH, "một")
	await get_tree().process_frame
	_fail_last(ChatService.CMD_SEND, "CHAT_RATE_LIMITED")
	_push(ChatFixtures.envelope("chat.message.held.held"))
	var m: ChatMessageModel = chat.send_text(CH, "hai")
	await get_tree().process_frame
	var cmd: Dictionary = _last_send()
	h.push(h.envelope("system.command.acked", {"result": {"client_message_id": m.client_message_id, "message_id": Uuid.v4(),
		"channel_id": CH, "seq": 3, "moderation_state": "masked"}}, String(cmd["id"])))
	assert_array(_keys()).is_equal(["ui.chat.feedback.rate_limited", "ui.chat.feedback.held", "ui.chat.feedback.masked"])


func test_sanction_locks_writing_with_the_reopening_time_and_keeps_reading() -> void:
	var env: Dictionary = ChatFixtures.envelope("chat.sanction.applied.auto_mute")
	(env["payload"] as Dictionary)["until"] = chat.now_ms() + 600000
	_push(env)
	assert_str(chat.store.sanction_lock(chat.now_ms())).is_equal("mute")
	assert_str(String(notices[0]["key"])).is_equal("ui.chat.muted_until")
	assert_str(String((notices[0]["args"] as Dictionary)["time"])).is_not_empty()
	assert_object(chat.send_text(CH, "ba")).is_null()
	assert_int(h.peer().sent_of(ChatService.CMD_SEND).size()).is_equal(0)
	# Reading still works.
	_push(ChatFixtures.envelope("chat.message.created.text"))
	assert_int(chat.store.count(CH)).is_equal(1)


func test_block_and_alert_sanction_shows_the_care_message() -> void:
	_push(ChatFixtures.envelope("chat.sanction.applied.block_and_alert"))
	assert_str(String(notices[0]["key"])).is_equal("ui.chat.feedback.care")
	assert_str(String(notices[0]["tone"])).is_equal(ChatFeedbackMapper.TONE_CARE)


func test_chat_muted_answer_locks_without_a_known_end() -> void:
	chat.send_text(CH, "bốn")
	await get_tree().process_frame
	_fail_last(ChatService.CMD_SEND, "CHAT_MUTED")
	assert_str(chat.store.sanction_lock(chat.now_ms())).is_equal("mute")


func test_reaction_limit_is_a_friendly_notice_and_counts_follow_the_server() -> void:
	_push(ChatFixtures.envelope("chat.message.created.text"))
	var m: ChatMessageModel = chat.store.visible(CH)[0]
	chat.toggle_reaction(m, "heart")
	await get_tree().process_frame
	_fail_last(ChatService.CMD_REACTION, "CHAT_REACTION_LIMIT")
	assert_array(_keys()).contains(["ui.chat.feedback.reaction_limit"])
	assert_array(Array(m.my_reactions)).is_empty()
	var upd: Dictionary = ChatFixtures.ws("chat.reaction.updated.counts")
	upd["message_id"] = m.message_id
	chat.handle_message("chat.reaction.updated", upd)
	assert_int(int(m.reactions["thumbs_up"])).is_equal(3)
	chat.toggle_reaction(m, "thumbs_up")
	await get_tree().process_frame
	h.ack_last(ChatService.CMD_REACTION)
	assert_array(Array(m.my_reactions)).contains(["thumbs_up"])


func test_typing_is_throttled_and_receipts_coalesced() -> void:
	chat.typing_started(CH)
	chat.typing_started(CH)
	assert_int(h.peer().sent_of(ChatService.CMD_TYPING_START).size()).is_equal(1)
	h.advance_ms(3100)
	chat.typing_started(CH)
	assert_int(h.peer().sent_of(ChatService.CMD_TYPING_START).size()).is_equal(2)
	chat.typing_stopped(CH)
	assert_int(h.peer().sent_of(ChatService.CMD_TYPING_STOP).size()).is_equal(1)
	for i: int in range(1, 6):
		var p: Dictionary = ChatFixtures.ws("chat.message.created.text")
		p["message_id"] = Uuid.v4()
		p["seq"] = i
		chat.handle_message("chat.message.created", p)
	assert_int(h.peer().sent_of(ChatService.CMD_DELIVERED).size()).is_equal(1)
	h.advance_ms(2100)
	chat.tick()
	var delivered: Array[Dictionary] = h.peer().sent_of(ChatService.CMD_DELIVERED)
	assert_int(delivered.size()).is_equal(2)
	assert_int(int((delivered[1]["payload"] as Dictionary)["seq"])).is_equal(5)


func test_menu_permissions_follow_the_author_and_the_features() -> void:
	_push(ChatFixtures.envelope("chat.message.created.text"))
	var theirs: ChatMessageModel = chat.store.visible(CH)[0]
	var mine_env: Dictionary = ChatFixtures.envelope("chat.message.created.text", SELF)
	(mine_env["payload"] as Dictionary)["message_id"] = Uuid.v4()
	(mine_env["payload"] as Dictionary)["seq"] = 17
	((mine_env["payload"] as Dictionary)["sender"] as Dictionary)["user_id"] = SELF
	_push(mine_env)
	var mine: ChatMessageModel = chat.store.visible(CH)[1]
	assert_bool(chat.can_edit(mine)).is_true()
	assert_bool(chat.can_recall(mine)).is_true()
	assert_bool(chat.can_edit(theirs)).is_false()
	assert_bool(chat.can_recall(theirs)).is_false()
	assert_bool(chat.can_reply(theirs)).is_true()
	# Forward / pin stay hidden until the server lists them in `features` (always-on fallback).
	assert_bool(chat.can_pin(theirs)).is_false()
	assert_bool(chat.can_forward(theirs)).is_false()


func test_copy_text_is_the_masked_text_or_the_placeholder() -> void:
	_push(ChatFixtures.envelope("chat.message.created.masked"))
	var m: ChatMessageModel = chat.store.visible(CH)[0]
	assert_str(chat.copy_text(m)).is_equal("Bạn *** quá!")
	chat.handle_message("chat.message.recalled", {"message_id": m.message_id, "channel_id": CH, "seq": m.seq})
	assert_str(chat.copy_text(m)).is_equal(tr("ui.chat.recalled"))


func test_recall_asks_the_server_and_shows_the_placeholder() -> void:
	var env: Dictionary = ChatFixtures.envelope("chat.message.created.text")
	((env["payload"] as Dictionary)["sender"] as Dictionary)["user_id"] = SELF
	_push(env)
	var m: ChatMessageModel = chat.store.visible(CH)[0]
	chat.recall(m)
	await get_tree().process_frame
	assert_int(h.peer().sent_of(ChatService.CMD_RECALL).size()).is_equal(1)
	h.ack_last(ChatService.CMD_RECALL)
	assert_bool(m.recalled).is_true()
	assert_str(m.body).is_empty()
