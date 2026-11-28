extends GdUnitTestSuite
## T016 (F111-AC-06, F111-AC-07; FR-SOC-001…003, ADR-0095, AGENTS R16): `SocialService.list_suggestions(source, cursor)`
## reads `listFriendSuggestions` (`source = recently_played` only — no classmates source, no free search) page by page;
## the group shows only nickname / avatar / level and the context of the last game together; the client additionally hides
## friends, players with a request waiting either way and blocked / muted players (the server filters first); "Kết bạn"
## is F110's `sendFriendRequest` (one Idempotency-Key per intent) with the F110 friendly codes; a failing list is marked
## so the screen hides the group.

var s: F111SceneHarness = null
var social: SocialService = null


func before_test() -> void:
	s = F111SceneHarness.new()
	await s.start("g4_5")
	social = s.services.social


func after_test() -> void:
	s.dispose()


func test_reads_recently_played_page_by_page() -> void:
	var page1: Dictionary = F111Fixtures.suggestions()
	page1["page"] = {"has_more": true, "next_cursor": "c2", "limit": 20}
	var page2: Dictionary = F111Fixtures.page([F111Fixtures.suggestion("0193b0a0-0000-7000-8000-0000000000a9", "KhiCon21", "match")])
	s.h.http.clear(F111Fixtures.ROUTE_SUGGESTIONS).reply(F111Fixtures.ROUTE_SUGGESTIONS, 200, page1) \
			.reply(F111Fixtures.ROUTE_SUGGESTIONS, 200, page2)
	var r: ApiResult = await social.list_suggestions(FriendSuggestionModel.SOURCE_RECENTLY_PLAYED)
	assert_bool(r.ok).is_true()
	var q: String = s.h.http.last(F111Fixtures.ROUTE_SUGGESTIONS).query
	assert_str(q).contains("source=recently_played")
	assert_int(social.store.suggestions.size()).is_equal(3)
	assert_bool(social.store.suggestions_more()).is_true()
	await social.list_suggestions(FriendSuggestionModel.SOURCE_RECENTLY_PLAYED, social.store.suggestions_cursor)
	assert_str(s.h.http.last(F111Fixtures.ROUTE_SUGGESTIONS).query).contains("cursor=c2")
	assert_int(social.store.suggestions.size()).is_equal(4)
	assert_bool(social.store.suggestions_more()).is_false()
	# Only the recently-played source exists (ADR-0095).
	var bad: ApiResult = await social.list_suggestions("classmates")
	assert_bool(bad.ok).is_false()
	assert_int(s.h.http.count(F111Fixtures.ROUTE_SUGGESTIONS)).is_equal(2)


func test_hides_friends_pending_requests_and_blocked_players() -> void:
	var body: Dictionary = F111Fixtures.suggestions()
	(body["data"] as Array).append(F111Fixtures.suggestion(F111Fixtures.FRIEND_ONLINE, F111Fixtures.FRIEND_NICK))
	(body["data"] as Array).append(F111Fixtures.suggestion(F110Fixtures.STRANGER, "BuomXanh09"))
	(body["data"] as Array).append(F111Fixtures.suggestion("0193b0a0-0000-7000-8000-0000000000c9", "NaiVang77"))
	s.h.http.clear(F111Fixtures.ROUTE_SUGGESTIONS).reply(F111Fixtures.ROUTE_SUGGESTIONS, 200, body)
	await social.load_friends()
	await social.load_requests()
	s.h.http.clear(F110Fixtures.ROUTE_BLOCKS).reply(F110Fixtures.ROUTE_BLOCKS, 200, F111Fixtures.page([
		F110Fixtures.relation("0193b0a0-0000-7000-8000-0000000000c9", "NaiVang77")]))
	await social.load_relations()
	social.store.add_outgoing(FriendRequestModel.from_dict({"request_id": "0193b0a0-0000-7000-8000-000000000d09",
		"addressee": F110Fixtures.card(F110Fixtures.STRANGER, "BuomXanh09"), "status": "pending"},
		FriendRequestModel.DIRECTION_OUTGOING))
	await social.list_suggestions(FriendSuggestionModel.SOURCE_RECENTLY_PLAYED)
	var shown: PackedStringArray = []
	for sug: FriendSuggestionModel in social.visible_suggestions():
		shown.append(sug.player.nickname)
	assert_array(Array(shown)).contains_exactly(["GauTruc22", "CaVang15", "SoiNho31"])


func test_add_from_a_suggestion_uses_the_f110_request_and_its_codes() -> void:
	await social.list_suggestions(FriendSuggestionModel.SOURCE_RECENTLY_PLAYED)
	var r: ApiResult = await social.send_friend_request(F111Fixtures.SUGGESTED_COOP)
	assert_bool(r.ok).is_true()
	var sent: FakeHttp.Recorded = s.h.http.last(F110Fixtures.ROUTE_SEND)
	assert_str(String(sent.body["addressee_id"])).is_equal(F111Fixtures.SUGGESTED_COOP)
	assert_str(String(sent.headers.get("idempotency-key", ""))).is_not_empty()
	var keys: Array[String] = []
	social.notice_requested.connect(func(key: String, _args: Dictionary) -> void: keys.append(key))
	for c: Array in [[403, "SOCIAL_SCOPE_VIOLATION", "ui.error.SOCIAL_SCOPE_VIOLATION"],
			[409, "FRIEND_LIMIT_REACHED", "ui.error.FRIEND_LIMIT_REACHED"], [429, "RATE_LIMITED", "ui.friends.rate_limited"]]:
		s.h.http.clear(F110Fixtures.ROUTE_SEND).reply(F110Fixtures.ROUTE_SEND, int(c[0]), AuthHarness.problem(int(c[0]), String(c[1])))
		await social.send_friend_request(F111Fixtures.SUGGESTED_MATCH)
		assert_str(keys.back()).is_equal(String(c[2]))


func test_a_failing_list_is_marked_and_keeps_nothing_private() -> void:
	s.h.http.clear(F111Fixtures.ROUTE_SUGGESTIONS).reply(F111Fixtures.ROUTE_SUGGESTIONS, 503,
			AuthHarness.problem(503, "UNAVAILABLE"))
	var r: ApiResult = await social.list_suggestions(FriendSuggestionModel.SOURCE_RECENTLY_PLAYED)
	assert_bool(r.ok).is_false()
	assert_bool(social.store.suggestions_failed).is_true()
	s.h.http.clear(F111Fixtures.ROUTE_SUGGESTIONS).reply(F111Fixtures.ROUTE_SUGGESTIONS, 200, F111Fixtures.suggestions())
	await social.list_suggestions(FriendSuggestionModel.SOURCE_RECENTLY_PLAYED)
	assert_bool(social.store.suggestions_failed).is_false()
	social.clear()
	assert_bool(social.store.suggestions.is_empty()).is_true()
