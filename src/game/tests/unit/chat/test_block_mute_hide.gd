extends GdUnitTestSuite
## T019 (F119-AC-15; TC-SOC-003, AC-CHAT-005): blocking or muting calls `blockUser` / `muteUser` and, once the server
## confirmed, the player's messages leave the chat at once (the server already filters fan-out and history); a refused
## call hides nothing.

var s: ChatSceneHarness = null


func before_test() -> void:
	s = ChatSceneHarness.new()
	await s.start()
	s.chat.store.ensure_channel(ChatChannelModel.placeholder(ChatFixtures.PARTY_CHANNEL, "party"))
	await s.chat.load_history(ChatFixtures.PARTY_CHANNEL)


func after_test() -> void:
	s.dispose()


func _senders() -> Array[String]:
	var out: Array[String] = []
	for m: ChatMessageModel in s.chat.store.visible(ChatFixtures.PARTY_CHANNEL):
		out.append(m.sender_id)
	return out


func test_block_calls_the_api_and_hides_the_player_at_once() -> void:
	assert_array(_senders()).contains([ChatFixtures.OTHER])
	var r: ApiResult = await s.services.social.block_user(ChatFixtures.OTHER)
	assert_bool(r.ok).is_true()
	assert_str(s.h.http.last(ChatFixtures.ROUTE_BLOCK).path).is_equal("/social/blocks/" + ChatFixtures.OTHER)
	assert_array(_senders()).not_contains([ChatFixtures.OTHER])
	assert_array(_senders()).contains([s.self_id])


func test_mute_calls_the_api_and_hides_the_player() -> void:
	await s.services.social.mute_user(ChatFixtures.THIRD)
	assert_str(s.h.http.last(ChatFixtures.ROUTE_MUTE).path).is_equal("/social/mutes/" + ChatFixtures.THIRD)
	assert_array(_senders()).not_contains([ChatFixtures.THIRD])


func test_refused_block_hides_nothing() -> void:
	s.h.http.clear(ChatFixtures.ROUTE_BLOCK).reply(ChatFixtures.ROUTE_BLOCK, 404, AuthHarness.problem(404, "NOT_FOUND"))
	var r: ApiResult = await s.services.social.block_user(ChatFixtures.OTHER)
	assert_bool(r.ok).is_false()
	assert_array(_senders()).contains([ChatFixtures.OTHER])
