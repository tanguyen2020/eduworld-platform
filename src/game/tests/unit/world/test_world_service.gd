extends GdUnitTestSuite
## F071 WorldService (T018, F071-AC-01/04/07/08/24 client part, ADR-0016): reads the areas the server lists, enters an
## area with the game session of the device and the walked wind gate, keeps the missing requirements of AREA_LOCKED,
## reports a discovery with an Idempotency-Key and no automatic 429 wait, caches dialogues by ETag (304) and refreshes
## on `world.area.unlocked`. Runs on FakeHttp (AuthHarness).

var h: AuthHarness = null
var store: WorldFixtures.FakeStore = null
var service: WorldService = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	store = auto_free(WorldFixtures.FakeStore.new())
	service = WorldService.new(h.api, store)
	h.http.reply(WorldFixtures.ROUTE_AREAS, 200, WorldFixtures.areas())


func after_test() -> void:
	h.dispose()


func test_areas_are_the_server_list() -> void:
	var result: ApiResult = await service.fetch_areas()
	assert_bool(result.ok).is_true()
	assert_int(service.areas.size()).is_equal(12)
	assert_bool(service.area("learning_zone.math").accessible).is_true()
	var ocean: WorldAreaModel = service.area("quest_zone.discovery_ocean")
	assert_bool(ocean.accessible).is_false()
	assert_str(String(ocean.requirements[0]["kind"])).is_equal("chapters_completed")
	assert_str(service.area("learning_zone.history").lock_reason).is_equal("grade_not_allowed")
	assert_str(service.area("main_hub").display_name("en")).is_equal("Spark Square")
	assert_int(int(store.world["area_count"])).is_equal(12)


func test_enter_sends_the_session_and_the_gate() -> void:
	h.http.reply(WorldFixtures.ROUTE_ENTER, 200, WorldFixtures.enter_math())
	var result: ApiResult = await service.enter_area("learning_zone.math", "portal.hub.to_math")
	assert_bool(result.ok).is_true()
	var rec: FakeHttp.Recorded = h.http.last(WorldFixtures.ROUTE_ENTER)
	assert_str(rec.path).contains("/game/world/areas/learning_zone.math:enter")
	assert_str(String(rec.body["game_session_id"])).is_equal(WorldFixtures.SESSION_ID)
	assert_str(String(rec.body["portal_id"])).is_equal("portal.hub.to_math")


func test_locked_area_keeps_the_missing_requirements() -> void:
	h.http.reply(WorldFixtures.ROUTE_ENTER, 403, WorldFixtures.area_locked())
	var result: ApiResult = await service.enter_area("challenge_mountain")
	assert_bool(result.ok).is_false()
	assert_str(result.code).is_equal(WorldService.CODE_AREA_LOCKED)
	var missing: Array[Dictionary] = WorldService.missing_requirements(result)
	assert_int(missing.size()).is_equal(1)
	assert_int(int(missing[0]["target"])).is_equal(2)


func test_discovery_carries_an_idempotency_key_and_reports_the_server_percentage() -> void:
	h.http.reply(WorldFixtures.ROUTE_DISCOVER, 200, {"area_id": "learning_zone.math", "interactable_id": "discovery.math.old_well",
		"kind": "discovery", "area_completion_pct": 33.33, "rewards": [], "unlocked_quest_ids": []})
	var seen: Array[float] = []
	service.discovery_recorded.connect(func(_a: String, _i: String, pct: float) -> void: seen.append(pct))
	var result: ApiResult = await service.report_discovery("learning_zone.math", "discovery.math.old_well")
	assert_bool(result.ok).is_true()
	assert_str(String(h.http.last(WorldFixtures.ROUTE_DISCOVER).headers.get("idempotency-key", ""))).is_not_empty()
	assert_float(seen[0]).is_equal_approx(33.33, 0.001)


func test_discovery_rate_limit_is_not_waited_on() -> void:
	h.http.reply(WorldFixtures.ROUTE_DISCOVER, 429, AuthHarness.problem(429, "RATE_LIMITED"), {"Retry-After": "30"})
	var result: ApiResult = await service.report_discovery("learning_zone.math", "discovery.math.old_well")
	assert_int(result.status).is_equal(429)
	assert_int(h.http.count(WorldFixtures.ROUTE_DISCOVER)).is_equal(1)


func test_dialogue_is_cached_by_etag() -> void:
	h.http.reply(WorldFixtures.ROUTE_DIALOGUE, 200, WorldFixtures.dialogue(), {"ETag": "\"v1\""})
	var first: ApiResult = await service.fetch_dialogue("npc.mr_numbers")
	assert_bool(first.ok).is_true()
	h.http.clear(WorldFixtures.ROUTE_DIALOGUE)
	h.http.reply(WorldFixtures.ROUTE_DIALOGUE, 304, null)
	var second: ApiResult = await service.fetch_dialogue("npc.mr_numbers")
	assert_bool(second.ok).is_true()
	assert_str(String(second.body()["npc_id"])).is_equal("npc.mr_numbers")
	assert_str(String(h.http.last(WorldFixtures.ROUTE_DIALOGUE).headers.get("if-none-match", ""))).is_equal("\"v1\"")


func test_area_unlocked_message_refreshes_the_areas() -> void:
	var unlocked: Array[String] = []
	service.area_unlocked.connect(func(id: String, _n: Dictionary) -> void: unlocked.append(id))
	service.call("_on_message", "world.area.unlocked", {"area_id": "arena", "name": {"vi": "Đấu trường", "en": "Arena"}, "unlocked_by": "quest"})
	await get_tree().process_frame
	await get_tree().process_frame
	assert_array(unlocked).contains_exactly(["arena"])
	assert_int(h.http.count(WorldFixtures.ROUTE_AREAS)).is_greater_equal(1)
