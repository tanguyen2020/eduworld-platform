extends GdUnitTestSuite
## T025 (F110-AC-22, F110-AC-24, F110-AC-25; SOCIAL_SYSTEM §5, §7, ADR-0054): "Kết bạn" sends `sendFriendRequest`
## (`addressee_id`, a key kept on retry); `403 SOCIAL_SCOPE_VIOLATION` → "Em chỉ kết bạn được với bạn cùng trường",
## `409 FRIEND_LIMIT_REACHED` / `429 RATE_LIMITED` (minutes of `Retry-After`) friendly sentences; there is no API to look
## a player up by name or id; "Mời party" goes through PartyService and creates the party first when there is none;
## block / mute / unblock / unmute; a blocked friend leaves the list at once; no screen tells who blocked the student.

var s: F110SceneHarness = null
var social: SocialService = null
var notices: Array[String] = []
var args_seen: Array[Dictionary] = []


func before_test() -> void:
	s = F110SceneHarness.new()
	await s.start("g4_5")
	social = s.services.social
	notices = []
	args_seen = []
	social.notice_requested.connect(func(key: String, args: Dictionary) -> void:
		notices.append(key)
		args_seen.append(args))


func after_test() -> void:
	s.dispose()


func test_send_request_uses_addressee_id_and_keeps_its_key_when_retried() -> void:
	s.h.http.clear(F110Fixtures.ROUTE_SEND).fail(F110Fixtures.ROUTE_SEND)
	await social.send_friend_request(F110Fixtures.STRANGER)
	var key: String = String(s.h.http.last(F110Fixtures.ROUTE_SEND).headers["idempotency-key"])
	s.h.http.clear(F110Fixtures.ROUTE_SEND).reply(F110Fixtures.ROUTE_SEND, 201, F110Fixtures.request(F110Fixtures.REQUEST_OUT, false, "BuomXanh09"))
	var r: ApiResult = await social.send_friend_request(F110Fixtures.STRANGER)
	assert_bool(r.ok).is_true()
	var rec: FakeHttp.Recorded = s.h.http.last(F110Fixtures.ROUTE_SEND)
	assert_str(String(rec.headers["idempotency-key"])).is_equal(key)
	assert_str(String(rec.body["addressee_id"])).is_equal(F110Fixtures.STRANGER)
	assert_int(social.store.outgoing.size()).is_equal(1)
	assert_str(social.store.outgoing[0].status_key()).is_equal("ui.friends.request_pending")


func test_refusals_are_friendly_sentences() -> void:
	var cases: Array = [[403, "SOCIAL_SCOPE_VIOLATION", "ui.error.SOCIAL_SCOPE_VIOLATION"],
		[409, "FRIEND_LIMIT_REACHED", "ui.error.FRIEND_LIMIT_REACHED"], [429, "RATE_LIMITED", "ui.friends.rate_limited"]]
	for c: Array in cases:
		notices.clear()
		args_seen.clear()
		s.h.http.clear(F110Fixtures.ROUTE_SEND).reply(F110Fixtures.ROUTE_SEND, int(c[0]), AuthHarness.problem(int(c[0]),
				String(c[1])), {"Retry-After": "120"})
		var r: ApiResult = await social.send_friend_request(F110Fixtures.STRANGER)
		assert_bool(r.ok).is_false()
		assert_array(notices).is_equal([String(c[2])])
	assert_int(int(args_seen[0]["minutes"])).is_equal(2)
	assert_str(TranslationServer.translate("ui.error.SOCIAL_SCOPE_VIOLATION")).is_equal("Em chỉ kết bạn được với bạn cùng trường")


func test_there_is_no_way_to_look_a_player_up_by_name_or_id() -> void:
	for m: Dictionary in social.get_method_list():
		var n: String = String(m["name"])
		assert_bool(n.contains("search") or n.contains("find_player") or n.contains("lookup")) \
				.override_failure_message("search API %s" % n).is_false()
	var tab: FindFriendsTab = auto_free(FindFriendsTab.new()) as FindFriendsTab
	add_child(tab)
	assert_bool(tab.has_search_field()).is_false()


func test_invite_to_party_creates_the_party_first_when_there_is_none() -> void:
	s.h.http.reply(PartyFixtures.ROUTE_CREATE, 201, PartyFixtures.party_alone(s.self_id))
	s.h.http.reply(PartyFixtures.ROUTE_INVITE, 201, {})
	assert_object(s.services.party.store.party).is_null()
	var r: ApiResult = await social.invite_to_party(F110Fixtures.FRIEND_ONLINE)
	assert_bool(r.ok).is_true()
	assert_int(s.h.http.count(PartyFixtures.ROUTE_CREATE)).is_equal(1)
	assert_str(String(s.h.http.last(PartyFixtures.ROUTE_INVITE).body["invitee_id"])).is_equal(F110Fixtures.FRIEND_ONLINE)
	# In a party already: only the invitation.
	await social.invite_to_party(F110Fixtures.FRIEND_OFFLINE)
	assert_int(s.h.http.count(PartyFixtures.ROUTE_CREATE)).is_equal(1)
	assert_int(s.h.http.count(PartyFixtures.ROUTE_INVITE)).is_equal(2)


func test_block_removes_the_friend_at_once_and_mute_unblock_unmute_call_their_routes() -> void:
	await social.load_friends()
	var changed: Array[String] = []
	social.relation_changed.connect(func(id: String, kind: String) -> void: changed.append("%s|%s" % [id, kind]))
	var r: ApiResult = await social.block_user(F110Fixtures.FRIEND_ONLINE)
	assert_bool(r.ok).is_true()
	assert_object(social.store.friend(F110Fixtures.FRIEND_ONLINE)).is_null()
	assert_str(s.h.http.last(F110Fixtures.ROUTE_BLOCK).path).is_equal("/social/blocks/%s" % F110Fixtures.FRIEND_ONLINE)
	await social.mute_user(F110Fixtures.FRIEND_OFFLINE)
	assert_object(social.store.friend(F110Fixtures.FRIEND_OFFLINE)).is_not_null()
	assert_array(changed).is_equal(["%s|block" % F110Fixtures.FRIEND_ONLINE, "%s|mute" % F110Fixtures.FRIEND_OFFLINE])
	s.h.http.clear(F110Fixtures.ROUTE_BLOCKS).reply(F110Fixtures.ROUTE_BLOCKS, 200, {"data": [F110Fixtures.relation(
			F110Fixtures.FRIEND_ONLINE, "CaHeoVui07")], "page": {"has_more": false, "limit": 100}})
	s.h.http.clear(F110Fixtures.ROUTE_MUTES).reply(F110Fixtures.ROUTE_MUTES, 200, {"data": [F110Fixtures.relation(
			F110Fixtures.FRIEND_OFFLINE, "ThoTrang03")], "page": {"has_more": false, "limit": 100}})
	await social.load_relations()
	assert_int(social.store.blocks.size()).is_equal(1)
	assert_int(social.store.mutes.size()).is_equal(1)
	await social.unblock_user(F110Fixtures.FRIEND_ONLINE)
	await social.unmute_user(F110Fixtures.FRIEND_OFFLINE)
	assert_str(s.h.http.last(F110Fixtures.ROUTE_UNBLOCK).method).is_equal("DELETE")
	assert_str(s.h.http.last(F110Fixtures.ROUTE_UNMUTE).path).is_equal("/social/mutes/%s" % F110Fixtures.FRIEND_OFFLINE)
	assert_int(social.store.blocks.size()).is_equal(0)
	assert_int(social.store.mutes.size()).is_equal(0)


func test_nothing_tells_who_blocked_this_student() -> void:
	for m: Dictionary in social.get_method_list():
		var n: String = String(m["name"])
		assert_bool(n.contains("blocked_by") or n.contains("blockers")).is_false()
	for key: String in ["ui.friends.blocked_by", "ui.friends.you_are_blocked"]:
		assert_bool(L10n.has_key(key)).is_false()
	var list: BlockList = auto_free(BlockList.new()) as BlockList
	add_child(list)
	list.set_relations([] as Array[RelationModel], [] as Array[RelationModel])
	for n: Node in list.find_children("*", "Label", true, false):
		assert_bool(TranslationServer.translate((n as Label).text).to_lower().contains("chặn em")).is_false()


func test_a_gone_friend_reads_the_lists_again_without_details() -> void:
	await social.load_friends()
	var refreshed: Array[int] = [0]
	social.refreshed_after_gone.connect(func() -> void: refreshed[0] += 1)
	s.h.http.clear(F110Fixtures.ROUTE_REMOVE).reply(F110Fixtures.ROUTE_REMOVE, 404, AuthHarness.problem(404, "NOT_FOUND"))
	var reads: int = s.h.http.count(F110Fixtures.ROUTE_FRIENDS)
	await social.remove_friend(F110Fixtures.FRIEND_OFFLINE)
	assert_int(refreshed[0]).is_equal(1)
	assert_int(s.h.http.count(F110Fixtures.ROUTE_FRIENDS)).is_equal(reads + 1)
	assert_array(notices).is_empty()
