extends GdUnitTestSuite
## T033 (F110-AC-30; AGENTS R23, ADR-0065, ADR-0041): every F110 request goes through ApiClient with the tenant header;
## `403 GAME_ACCESS_STUDENT_ONLY` (staff) ends the session through the F061 flow (`access_denied`), `TENANT_MISMATCH` is
## ApiClient's own; a `404` of an encounter, a clue step or a friend closes the screen part and reads again without
## details; `422 VALIDATION_FAILED` queues a `client_bug` telemetry event with the operation id only (no PII).

var s: F110SceneHarness = null


func before_test() -> void:
	s = F110SceneHarness.new()
	await s.start("g4_5")


func after_test() -> void:
	s.dispose()


func test_every_f110_request_carries_the_tenant_header() -> void:
	await s.services.boss.list("math_kingdom")
	await s.services.boss.load_encounter(F110Fixtures.ENCOUNTER_ID)
	await s.services.coop_quest.load_clues(F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID)
	await s.services.social.load_friends()
	await s.services.social.load_requests()
	await s.services.social.load_relations()
	var checked: int = 0
	for rec: FakeHttp.Recorded in s.h.http.requests:
		if rec.path.begins_with("/game/bosses") or rec.path.begins_with("/game/boss-encounters") \
				or rec.path.contains("/clues") or rec.path.begins_with("/social/"):
			assert_str(String(rec.headers.get("x-tenant-id", ""))).is_equal(AuthHarness.TENANT_ID)
			assert_str(String(rec.headers.get("authorization", ""))).starts_with("Bearer ")
			checked += 1
	assert_int(checked).is_greater_equal(7)


func test_staff_refused_with_game_access_student_only_goes_to_the_f061_flow() -> void:
	var denied: Array[String] = []
	s.services.access_denied_handler = func(code: String) -> void: denied.append(code)
	for svc: GameApiService in [s.services.boss, s.services.coop_quest, s.services.social]:
		svc.access_denied.connect(s.services._on_access_denied)
	s.h.http.clear(F110Fixtures.ROUTE_BOSSES).reply(F110Fixtures.ROUTE_BOSSES, 403, AuthHarness.problem(403, "GAME_ACCESS_STUDENT_ONLY"))
	await s.services.boss.list()
	s.h.http.clear(F110Fixtures.ROUTE_FRIENDS).reply(F110Fixtures.ROUTE_FRIENDS, 403, AuthHarness.problem(403, "GAME_ACCESS_STUDENT_ONLY"))
	await s.services.social.load_friends()
	s.h.http.clear(F110Fixtures.ROUTE_CLUES).reply(F110Fixtures.ROUTE_CLUES, 403, AuthHarness.problem(403, "GAME_ACCESS_STUDENT_ONLY"))
	await s.services.coop_quest.load_clues(F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID)
	assert_array(denied).is_equal(["GAME_ACCESS_STUDENT_ONLY", "GAME_ACCESS_STUDENT_ONLY", "GAME_ACCESS_STUDENT_ONLY"])


func test_a_404_closes_the_part_and_shows_no_detail() -> void:
	var exits: Array[String] = []
	s.services.boss.exit_requested.connect(func(target: String, key: String) -> void: exits.append("%s|%s" % [target, key]))
	s.h.http.clear(F110Fixtures.ROUTE_ENCOUNTER).reply(F110Fixtures.ROUTE_ENCOUNTER, 404, AuthHarness.problem(404, "NOT_FOUND"))
	await s.services.boss.load_encounter(F110Fixtures.ENCOUNTER_ID)
	assert_array(exits).is_equal(["hub|%s" % BossClient.KEY_GONE])
	var closed: Array[int] = [0]
	s.services.coop_quest.step_closed.connect(func(_q: String, _s: String) -> void: closed[0] += 1)
	s.h.http.reply("GET /game/quests", 200, {"data": [], "page": {"has_more": false}})
	s.h.http.clear(F110Fixtures.ROUTE_CLUES).reply(F110Fixtures.ROUTE_CLUES, 404, AuthHarness.problem(404, "NOT_FOUND"))
	await s.services.coop_quest.load_clues(F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID)
	assert_int(closed[0]).is_equal(1)
	var notices: Array[String] = []
	s.services.social.notice_requested.connect(func(key: String, _a: Dictionary) -> void: notices.append(key))
	s.h.http.clear(F110Fixtures.ROUTE_BLOCK).reply(F110Fixtures.ROUTE_BLOCK, 404, AuthHarness.problem(404, "NOT_FOUND"))
	await s.services.social.block_user(F110Fixtures.STRANGER)
	assert_array(notices).is_empty()


func test_validation_failed_is_a_client_bug_event_without_personal_data() -> void:
	var telemetry: TelemetryService = s.services.telemetry
	for pair: Array in [[F110Fixtures.ROUTE_SEND, "sendFriendRequest"], [F110Fixtures.ROUTE_SHARE, "shareQuestStepClue"]]:
		telemetry.clear()
		s.h.http.clear(String(pair[0])).reply(String(pair[0]), 422, AuthHarness.problem(422, "VALIDATION_FAILED"))
		if String(pair[1]) == "sendFriendRequest":
			await s.services.social.send_friend_request(F110Fixtures.STRANGER)
		else:
			await s.services.coop_quest.share(F110Fixtures.QUEST_ID, F110Fixtures.STEP_ID, F110Fixtures.ROLE_MEASURE)
		assert_int(telemetry.queue.size()).is_equal(1)
		assert_str(String(telemetry.queue[0]["error_code"])).is_equal("client_bug")
		assert_str(String(telemetry.queue[0]["scene_code"])).is_equal(String(pair[1]))
		var dump: String = JSON.stringify(telemetry.queue[0])
		for pii: String in [F110Fixtures.STRANGER, "BuomXanh09", "MeoXanh42"]:
			assert_str(dump).not_contains(pii)
