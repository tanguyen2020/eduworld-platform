extends GdUnitTestSuite
## T024 (F110-AC-19, F110-AC-20, F110-AC-21; SOCIAL_SYSTEM §4, MULTIPLAYER_GAMEPLAY §5): `listFriends` reads every page
## (cursor) and keeps only the public card (nickname, avatar, level, title, presence) — no real name, school or grade even
## when a body carries one; the list is grouped Online / Offline only; `presence.friend.updated` changes a friend at once
## but the list redraws at most once a second; a friend without data is offline; without network the presence reads
## "—"; a request this student sent always reads "Đang chờ" (also when the server says `rejected`).

var s: F110SceneHarness = null
var social: SocialService = null


func before_test() -> void:
	s = F110SceneHarness.new()
	await s.start("g4_5")
	social = s.services.social


func after_test() -> void:
	s.dispose()


func test_list_friends_reads_every_page_and_keeps_only_the_public_card() -> void:
	var page1: Dictionary = F110Fixtures.friends()
	page1["page"] = {"has_more": true, "next_cursor": "c2", "limit": 100}
	var leaky: Dictionary = (page1["data"] as Array)[0] as Dictionary
	leaky["full_name"] = "Nguyễn Văn A"
	leaky["school_name"] = "Trường X"
	leaky["grade_level"] = 3
	leaky["mastery"] = 0.9
	var page2: Dictionary = {"data": [F110Fixtures.card(F110Fixtures.STRANGER, "BuomXanh09", "away")],
		"page": {"has_more": false, "limit": 100}}
	s.h.http.clear(F110Fixtures.ROUTE_FRIENDS).reply(F110Fixtures.ROUTE_FRIENDS, 200, page1).reply(F110Fixtures.ROUTE_FRIENDS, 200, page2)
	var r: ApiResult = await social.load_friends()
	assert_bool(r.ok).is_true()
	assert_int(s.h.http.count(F110Fixtures.ROUTE_FRIENDS)).is_equal(2)
	assert_str(s.h.http.last(F110Fixtures.ROUTE_FRIENDS).query).contains("cursor=c2")
	assert_int(social.store.friends.size()).is_equal(4)
	var f: FriendModel = social.store.friend(F110Fixtures.FRIEND_ONLINE)
	for field: String in ["full_name", "school_name", "grade_level", "mastery"]:
		assert_bool(field in f).override_failure_message("FriendModel has %s" % field).is_false()
	assert_int(social.store.online_friends().size()).is_equal(3)
	assert_int(social.store.offline_friends().size()).is_equal(1)


func test_presence_updates_one_friend_and_the_list_redraws_at_most_once_a_second() -> void:
	await social.load_friends()
	var emits: int = social.store.emits
	s.push("presence.friend.updated", F110Fixtures.ws("presence.friend.updated", {"user_id": F110Fixtures.FRIEND_OFFLINE,
		"status": "in_match", "activity": "battle"}))
	s.push("presence.friend.updated", F110Fixtures.ws("presence.friend.updated", {"user_id": F110Fixtures.FRIEND_ONLINE,
		"status": "away"}))
	assert_str(social.store.friend(F110Fixtures.FRIEND_OFFLINE).presence).is_equal("in_match")
	assert_int(social.store.emits).is_equal(emits)
	var t: int = 10_000
	assert_bool(social.store.flush(t)).is_true()
	s.push("presence.friend.updated", F110Fixtures.ws("presence.friend.updated", {"user_id": F110Fixtures.FRIEND_OFFLINE,
		"status": "online"}))
	assert_bool(social.store.flush(t + 400)).is_false()
	assert_bool(social.store.flush(t + 1000)).is_true()
	assert_int(social.store.emits).is_equal(emits + 2)
	# An unknown value of the enum or no data is offline; another player is ignored.
	s.push("presence.friend.updated", {"user_id": F110Fixtures.FRIEND_OFFLINE, "status": "invisible"})
	assert_str(social.store.friend(F110Fixtures.FRIEND_OFFLINE).presence).is_equal("offline")
	assert_bool(social.store.apply_presence(F110Fixtures.STRANGER, "online")).is_false()
	var bare: FriendModel = FriendModel.from_card({"user_id": F110Fixtures.STRANGER, "nickname": "KhiCon21"})
	assert_str(bare.presence).is_equal("offline")


func test_without_network_the_presence_reads_a_dash() -> void:
	await social.load_friends()
	s.h.http.clear(F110Fixtures.ROUTE_FRIENDS).fail(F110Fixtures.ROUTE_FRIENDS)
	await social.load_friends()
	assert_bool(social.store.presence_unknown).is_true()
	var row: FriendRow = auto_free(FriendRow.new()) as FriendRow
	add_child(row)
	row.bind(social.store.friend(F110Fixtures.FRIEND_ONLINE), social.store.presence_unknown, false)
	assert_str(row.texts()[2]).is_equal(TranslationServer.translate("ui.friends.presence.unknown"))
	# The friends of the last answer stay on screen.
	assert_int(social.store.friends.size()).is_equal(3)


func test_requests_incoming_and_outgoing_and_a_sent_one_always_reads_pending() -> void:
	var r: ApiResult = await social.load_requests()
	assert_bool(r.ok).is_true()
	assert_int(social.store.incoming.size()).is_equal(1)
	assert_int(social.store.outgoing.size()).is_equal(1)
	assert_str(social.store.incoming[0].other.nickname).is_equal("SaoBang12")
	# The server marked the sent request `rejected`: on screen it is still "Đang chờ" (silent refusal).
	assert_str(social.store.outgoing[0].status_key()).is_equal("ui.friends.request_pending")
	var list: FriendRequestList = auto_free(FriendRequestList.new()) as FriendRequestList
	add_child(list)
	list.set_requests(social.store.incoming, social.store.outgoing)
	assert_array(list.outgoing_status_keys()).is_equal(["ui.friends.request_pending"])
	for key: String in ["ui.friends.request_rejected", "rejected"]:
		assert_bool(key in list.outgoing_status_keys()).is_false()


func test_accept_sends_a_key_and_reject_has_none_both_drop_the_row() -> void:
	await social.load_requests()
	var req: FriendRequestModel = social.store.incoming[0]
	var r: ApiResult = await social.accept(req)
	assert_bool(r.ok).is_true()
	var rec: FakeHttp.Recorded = s.h.http.last(F110Fixtures.ROUTE_ACCEPT)
	assert_str(rec.path).is_equal("/social/friend-requests/%s:accept" % F110Fixtures.REQUEST_IN)
	assert_bool(Uuid.is_valid(String(rec.headers.get("idempotency-key", "")))).is_true()
	assert_int(social.store.incoming.size()).is_equal(0)
	await social.load_requests()
	await social.reject(social.store.incoming[0])
	assert_str(s.h.http.last(F110Fixtures.ROUTE_REJECT).path).is_equal("/social/friend-requests/%s:reject" % F110Fixtures.REQUEST_IN)
	assert_int(social.store.incoming.size()).is_equal(0)
