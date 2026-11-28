extends GdUnitTestSuite
## T012 (F112-AC-07, AC-08, AC-10; TC-CHAT-005, TC-CHAT-047, TC-CHAT-039): "Nhắn tin" calls `openDirectChannel` with the
## friend's id (`200` and `201` alike) and opens a DM tab named by the friend's nickname (the channel ↔ friend link only in
## RAM); the button exists only for a current friend while `friends_enabled` / `direct_message_enabled` are not false;
## `403 CHAT_NOT_ALLOWED` / `404` → one neutral sentence, no retry, the button hidden for the session; listing channels
## never opens a DM; a DM whose friend is unknown learns it from the other student's message (never a guess).

var s: F112SceneHarness = null
var hub: ChatHubService = null
var notices: Array[String] = []


func before_test() -> void:
	s = F112SceneHarness.new()
	await s.start("g6_7")
	hub = s.services.chat_hub
	notices.clear()
	hub.notice_requested.connect(func(key: String, _a: Dictionary) -> void: notices.append(key))


func after_test() -> void:
	AppState.feature_flags = {}
	AppState.school_policies = {}
	s.dispose()


func test_message_opens_the_dm_with_the_friend() -> void:
	await hub.refresh()
	assert_int(s.h.http.count(F112Fixtures.ROUTE_DIRECT)).is_equal(0)
	assert_bool(hub.can_message(F112Fixtures.FRIEND_NEW)).is_true()
	var ok: bool = await hub.open_dm(F112Fixtures.FRIEND_NEW)
	assert_bool(ok).is_true()
	var req: FakeHttp.Recorded = s.h.http.last(F112Fixtures.ROUTE_DIRECT)
	assert_str(String(req.body.get("user_id", ""))).is_equal(F112Fixtures.FRIEND_NEW)
	assert_str(hub.state.peers.channel_of(F112Fixtures.FRIEND_NEW)).is_equal(F112Fixtures.DM_NEW)
	var tab: PersistentChannelTab = hub.state.tab("dm:%s" % F112Fixtures.FRIEND_NEW)
	assert_object(tab).is_not_null()
	assert_str(tab.title).is_equal("SaoBang12")
	assert_str(tab.channel_id).is_equal(F112Fixtures.DM_NEW)


func test_200_is_handled_like_201() -> void:
	s.h.http.clear(F112Fixtures.ROUTE_DIRECT).reply(F112Fixtures.ROUTE_DIRECT, 200,
			F112Fixtures.direct_created(F112Fixtures.FRIEND_NEW, "SaoBang12", "200"))
	assert_bool(await hub.open_dm(F112Fixtures.FRIEND_NEW)).is_true()
	assert_object(hub.state.tab("dm:%s" % F112Fixtures.FRIEND_NEW)).is_not_null()


func test_refused_dm_says_one_neutral_sentence_and_hides_the_button() -> void:
	for status: int in [403, 404]:
		s.h.http.clear(F112Fixtures.ROUTE_DIRECT).reply(F112Fixtures.ROUTE_DIRECT, status,
				F112Fixtures.openapi("openDirectChannel.403") if status == 403 else AuthHarness.problem(404, "NOT_FOUND"))
		var friend: String = F112Fixtures.FRIEND_NEW if status == 403 else F112Fixtures.FRIEND_OTHER
		var before: int = s.h.http.count(F112Fixtures.ROUTE_DIRECT)
		assert_bool(await hub.open_dm(friend)).is_false()
		assert_int(s.h.http.count(F112Fixtures.ROUTE_DIRECT)).is_equal(before + 1)
		assert_bool(hub.can_message(friend)).is_false()
		# No retry: the button is gone, a second call does nothing.
		assert_bool(await hub.open_dm(friend)).is_false()
		assert_int(s.h.http.count(F112Fixtures.ROUTE_DIRECT)).is_equal(before + 1)
	assert_array(notices).is_equal([ChatHubService.KEY_DM_UNAVAILABLE, ChatHubService.KEY_DM_UNAVAILABLE])
	var sentence: String = TranslationServer.translate(ChatHubService.KEY_DM_UNAVAILABLE).to_lower()
	for w: String in ["chặn", "block", "không còn là bạn", "tắt", "common"]:
		assert_bool(sentence.contains(w)).override_failure_message("'%s' in the refusal" % w).is_false()


func test_button_only_for_friends_and_while_dms_are_allowed() -> void:
	assert_bool(hub.can_message("0193b0a0-0000-7000-8000-0000000000a3")).is_false()
	assert_bool(hub.can_message(F112Fixtures.FRIEND_NEW)).is_true()
	AppState.feature_flags = {"social": {"friends_enabled": false}}
	assert_bool(hub.can_message(F112Fixtures.FRIEND_NEW)).is_false()
	AppState.feature_flags = {}
	AppState.school_policies = {"chat": {"direct_message_enabled": false}}
	assert_bool(hub.can_message(F112Fixtures.FRIEND_NEW)).is_false()
	AppState.school_policies = {"chat": {}}
	assert_bool(hub.can_message(F112Fixtures.FRIEND_NEW)).is_true()


func test_unknown_dm_learns_its_friend_from_their_message() -> void:
	await hub.refresh()
	assert_bool(hub.state.peers.knows(F112Fixtures.DM_UNKNOWN)).is_false()
	var page: Dictionary = ChatFixtures.history(s.services.chat.store.self_id)
	var list: Array = []
	for m: Variant in page["data"] as Array:
		var d: Dictionary = (m as Dictionary).duplicate(true)
		d["channel_id"] = F112Fixtures.DM_UNKNOWN
		if String((d.get("sender", {}) as Dictionary).get("user_id", "")) != s.services.chat.store.self_id:
			list.append(d)
	page["data"] = list.slice(0, 1)
	s.h.http.clear(F112Fixtures.ROUTE_MESSAGES).reply(F112Fixtures.ROUTE_MESSAGES, 200, page)
	await s.services.chat.load_history(F112Fixtures.DM_UNKNOWN)
	assert_bool(hub.state.peers.knows(F112Fixtures.DM_UNKNOWN)).is_true()
	assert_int(s.h.http.count(F112Fixtures.ROUTE_DIRECT)).is_equal(0)
