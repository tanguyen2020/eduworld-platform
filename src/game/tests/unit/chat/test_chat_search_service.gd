extends GdUnitTestSuite
## T018 (F112-AC-15, AC-16; TC-CHAT-040): `searchMessages` only with a query inside the contract bounds (2–100
## characters); the scope is every searchable channel or one `channel_id`; pages follow `cursor`; each hit keeps the
## sender card, channel and time and a plain-text excerpt (markup removed, keyword ranges); the query and the excerpts
## never reach a log or telemetry (AGENTS R14).

var h: AuthHarness = null
var search: ChatSearchService = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	search = ChatSearchService.new(h.api)
	h.http.reply(F112Fixtures.ROUTE_SEARCH, 200, F112Fixtures.search())
	Log.clear()


func after_test() -> void:
	h.dispose()


func test_query_bounds_follow_the_contract() -> void:
	assert_bool(ChatSearchService.query_ok("a")).is_false()
	assert_bool(ChatSearchService.query_ok(" a ")).is_false()
	assert_bool(ChatSearchService.query_ok("di")).is_true()
	assert_bool(ChatSearchService.query_ok("x".repeat(101))).is_false()
	var r: ApiResult = await search.search("a")
	assert_bool(r.ok).is_false()
	assert_int(h.http.count(F112Fixtures.ROUTE_SEARCH)).is_equal(0)


func test_search_sends_q_and_scope_and_parses_plain_hits() -> void:
	var r: ApiResult = await search.search("di boss", F112Fixtures.TEAM_OWN)
	assert_bool(r.ok).is_true()
	var q: String = h.http.last(F112Fixtures.ROUTE_SEARCH).query
	assert_str(q).contains("q=di%20boss")
	assert_str(q).contains("channel_id=%s" % F112Fixtures.TEAM_OWN)
	assert_int(search.hits.size()).is_equal(3)
	var tagged: Dictionary = search.hits[1]
	assert_str(String(tagged["excerpt"])).is_equal("Hẹn đi boss lúc 19h nhé")
	assert_array(tagged["ranges"] as Array).is_not_empty()
	var first: Dictionary = search.hits[0]
	assert_str((first["message"] as ChatMessageModel).sender_nickname).is_equal("CaHeoVui07")
	# The server ranges of the example are kept (code points of `highlight`).
	assert_array(first["ranges"] as Array).is_equal([Vector2i(25, 4), Vector2i(30, 3)])


func test_all_scope_and_cursor_pages() -> void:
	var page1: Dictionary = F112Fixtures.search()
	page1["page"] = {"has_more": true, "next_cursor": "c2", "limit": 20}
	h.http.clear(F112Fixtures.ROUTE_SEARCH).reply(F112Fixtures.ROUTE_SEARCH, 200, page1).reply(F112Fixtures.ROUTE_SEARCH,
			200, F112Fixtures.search())
	await search.search("boss")
	assert_str(h.http.last(F112Fixtures.ROUTE_SEARCH).query).not_contains("channel_id")
	assert_bool(search.has_more).is_true()
	await search.more()
	assert_str(h.http.last(F112Fixtures.ROUTE_SEARCH).query).contains("cursor=c2")
	assert_int(search.hits.size()).is_equal(6)
	assert_bool(search.has_more).is_false()


func test_query_and_excerpts_never_logged() -> void:
	await search.search("di boss")
	var logs: String = JSON.stringify(Log.entries())
	assert_str(logs).not_contains("di boss")
	assert_str(logs).not_contains("luyện")
