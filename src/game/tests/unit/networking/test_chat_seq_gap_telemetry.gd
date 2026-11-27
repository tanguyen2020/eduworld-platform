extends GdUnitTestSuite
## Client telemetry `chat_seq_gap` (GAME_ANALYTICS §3.2, MONITORING §3 `eduworld_chat_seq_gaps_total{channel_type}`;
## F119 Clarifications 2026-11-28, AGENTS R14): a seq gap that ChatStore detects queues one `chat_seq_gap` with only
## `channel_type` and `gap_count` (1–1 000); gaps of the same channel type are merged into the event still waiting for
## the next batch (one event per channel type per flush window), never the event of a batch already on its way; an
## unknown channel type sends nothing; the event goes even when product telemetry is off and stays queued while
## offline. No channel id, user id, nickname or message content ever leaves in the batch. TEST-ONLY fakes.

const SELF: String = "00000000-0000-4000-8000-000000000001"
const CH: String = ChatFixtures.PARTY_CHANNEL
const DM: String = "0193b0a0-0000-7000-8000-0000000009d1"
const ROUTE: String = "POST /game/telemetry"
const ALLOWED_KEYS: Array[String] = ["type", "client_event_id", "occurred_at", "channel_type", "gap_count"]

var h: RealtimeHarness = null
var telemetry: TelemetryService = null
var chat: ChatService = null


func before_test() -> void:
	h = RealtimeHarness.create()
	await h.connect_online()
	telemetry = TelemetryService.new(h.api, h.clock)
	telemetry.product_telemetry_enabled = false
	chat = ChatService.new(h.api, h.client, null, telemetry)
	chat.store.self_id = SELF
	for pair: Array in [[CH, "party"], [DM, "direct"]]:
		chat.store.ensure_channel(ChatChannelModel.placeholder(String(pair[0]), String(pair[1])))
		chat.store.load_history(String(pair[0]), _none())


func after_test() -> void:
	h.client.unregister_handler(ChatService.PREFIX, chat.handle_message)
	chat.clear()
	h.dispose()


func _none() -> Array[ChatMessageModel]:
	var out: Array[ChatMessageModel] = []
	return out


func _created(channel_id: String, seq: int) -> Dictionary:
	var p: Dictionary = ChatFixtures.ws("chat.message.created.text")
	p["message_id"] = Uuid.v4()
	p["channel_id"] = channel_id
	p["seq"] = seq
	return p


func _gaps() -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	for e: Dictionary in telemetry.queue:
		if String(e["type"]) == TelemetryService.TYPE_CHAT_SEQ_GAP:
			out.append(e)
	return out


func _sent_events() -> Array:
	return h.http.last(ROUTE).body.get("events", []) as Array


func test_a_detected_gap_queues_one_event_with_the_channel_type_and_count() -> void:
	chat.handle_message("chat.message.created", _created(CH, 1))
	chat.handle_message("chat.message.created", _created(CH, 5))
	await get_tree().process_frame
	var gaps: Array[Dictionary] = _gaps()
	assert_int(gaps.size()).is_equal(1)
	assert_str(String(gaps[0]["channel_type"])).is_equal("party")
	assert_int(int(gaps[0]["gap_count"])).is_equal(3)
	assert_bool(Uuid.is_valid(String(gaps[0]["client_event_id"]))).is_true()
	assert_str(String(gaps[0]["occurred_at"])).ends_with("Z")


func test_contiguous_messages_send_nothing() -> void:
	for seq: int in [1, 2, 3]:
		chat.handle_message("chat.message.created", _created(CH, seq))
	await get_tree().process_frame
	assert_array(_gaps()).is_empty()


func test_gap_count_is_clamped_to_one_through_one_thousand() -> void:
	assert_int(int(telemetry.queue_chat_seq_gap("match", 5000)["gap_count"])).is_equal(1000)
	telemetry.clear()
	assert_int(int(telemetry.queue_chat_seq_gap("match", 0)["gap_count"])).is_equal(1)
	telemetry.clear()
	assert_int(int(telemetry.queue_chat_seq_gap("match", -7)["gap_count"])).is_equal(1)


func test_unknown_channel_type_sends_nothing() -> void:
	for kind: String in ["class", "study_group", "", "party\n"]:
		assert_bool(telemetry.queue_chat_seq_gap(kind, 3).is_empty()).override_failure_message(kind).is_true()
	assert_bool(telemetry.queue.is_empty()).is_true()


func test_gaps_of_one_channel_type_are_merged_until_the_next_batch() -> void:
	var first: Dictionary = telemetry.queue_chat_seq_gap("party", 3)
	var second: Dictionary = telemetry.queue_chat_seq_gap("party", 4)
	telemetry.queue_chat_seq_gap("match", 2)
	assert_int(_gaps().size()).is_equal(2)
	assert_str(String(second["client_event_id"])).is_equal(String(first["client_event_id"]))
	assert_int(int(first["gap_count"])).is_equal(7)
	# The merged count never passes the contract maximum: the rest opens a new event.
	telemetry.queue_chat_seq_gap("party", 999)
	var party: Array[Dictionary] = _gaps().filter(func(e: Dictionary) -> bool: return e["channel_type"] == "party")
	assert_int(party.size()).is_equal(2)
	assert_int(int(party[0]["gap_count"])).is_equal(1000)
	assert_int(int(party[1]["gap_count"])).is_equal(6)


func test_an_event_already_in_a_batch_is_never_changed() -> void:
	h.http.fail(ROUTE)
	var first: Dictionary = telemetry.queue_chat_seq_gap("party", 3)
	var result: ApiResult = await telemetry.flush(true)
	assert_bool(result.ok).is_false()
	var later: Dictionary = telemetry.queue_chat_seq_gap("party", 2)
	assert_str(String(later["client_event_id"])).is_not_equal(String(first["client_event_id"]))
	assert_int(int(first["gap_count"])).is_equal(3)
	# Offline: both stay queued; the first is resent with its id and count unchanged.
	h.http.reply(ROUTE, 202)
	assert_bool((await telemetry.flush(true)).ok).is_true()
	var events: Array = _sent_events()
	assert_int(events.size()).is_equal(2)
	assert_str(String((events[0] as Dictionary)["client_event_id"])).is_equal(String(first["client_event_id"]))
	assert_int(int((events[0] as Dictionary)["gap_count"])).is_equal(3)
	assert_bool(telemetry.queue.is_empty()).is_true()


func test_sent_even_when_product_telemetry_is_off() -> void:
	assert_bool(telemetry.product_telemetry_enabled).is_false()
	assert_bool(telemetry.queue_chat_seq_gap("world", 1).is_empty()).is_false()


func test_a_direct_channel_gap_carries_no_id_user_or_content() -> void:
	chat.handle_message("chat.message.created", _created(DM, 1))
	chat.handle_message("chat.message.created", _created(DM, 9))
	await get_tree().process_frame
	h.http.reply(ROUTE, 202)
	assert_bool((await telemetry.flush(true)).ok).is_true()
	var events: Array = _sent_events()
	assert_int(events.size()).is_equal(1)
	var e: Dictionary = events[0] as Dictionary
	assert_str(String(e["channel_type"])).is_equal("direct")
	assert_int(int(e["gap_count"])).is_equal(7)
	for key: Variant in e.keys():
		assert_bool(String(key) in ALLOWED_KEYS).override_failure_message(String(key)).is_true()
	var created: Dictionary = ChatFixtures.ws("chat.message.created.text")
	var sender: Dictionary = created["sender"] as Dictionary
	var raw: String = h.http.last(ROUTE).raw_body
	for forbidden: String in [DM, CH, SELF, String(sender["user_id"]), String(sender["nickname"]),
			String(created["body"]), "channel_id", "user_id", "message_id", "body"]:
		assert_str(raw).override_failure_message(forbidden).not_contains(forbidden)
