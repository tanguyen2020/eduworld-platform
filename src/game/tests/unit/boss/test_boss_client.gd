extends GdUnitTestSuite
## T009 (F110-AC-01, F110-AC-10; CHALLENGE_BOSS_SYSTEM §3, §9): "Chơi một mình" creates a `solo` encounter with one
## Idempotency-Key kept while the same intent is retried after a network failure, a boss without `solo` only offers "Rủ
## bạn" (BossInfoCard), `403 CHALLENGE_NOT_ELIGIBLE` / `409 CHALLENGE_ATTEMPT_LIMIT` become friendly sentences,
## "Rời" sends `leaveBossEncounter` with a key and goes back to the zone (solo) or the lobby (party), and
## `ROOM_NOT_MEMBER` / `404` leave the scene to the Hub with a general sentence. FakeHttp + contract fixtures.

var s: F110SceneHarness = null
var notices: Array[String] = []
var exits: Array[String] = []


func before_test() -> void:
	s = F110SceneHarness.new()
	await s.start("g4_5")
	notices = []
	exits = []
	s.services.boss.notice_requested.connect(func(key: String, _args: Dictionary) -> void: notices.append(key))
	s.services.boss.exit_requested.connect(func(target: String, _key: String) -> void: exits.append(target))


func after_test() -> void:
	s.dispose()


func _boss(solo: bool = true) -> BossSummary:
	return BossSummary.from_dict(F110Fixtures.boss(solo))


func test_start_solo_posts_mode_solo_and_keeps_its_key_when_the_intent_is_retried() -> void:
	s.h.http.clear(F110Fixtures.ROUTE_CREATE).fail(F110Fixtures.ROUTE_CREATE)
	var first: ApiResult = await s.services.boss.start_solo(_boss())
	assert_bool(first.network_error).is_true()
	var failed_key: String = String(s.h.http.last(F110Fixtures.ROUTE_CREATE).headers.get("idempotency-key", ""))
	assert_bool(Uuid.is_valid(failed_key)).is_true()
	s.h.http.clear(F110Fixtures.ROUTE_CREATE).reply(F110Fixtures.ROUTE_CREATE, 201, F110Fixtures.encounter(s.self_id))
	var second: ApiResult = await s.services.boss.start_solo(_boss())
	assert_bool(second.ok).is_true()
	var rec: FakeHttp.Recorded = s.h.http.last(F110Fixtures.ROUTE_CREATE)
	assert_str(String(rec.headers["idempotency-key"])).is_equal(failed_key)
	assert_str(String(rec.body["mode"])).is_equal("solo")
	assert_str(s.services.boss.store.encounter_id()).is_equal(F110Fixtures.ENCOUNTER_ID)
	# A new intent after the server answered gets a new key.
	await s.services.boss.start_solo(_boss())
	assert_str(String(s.h.http.last(F110Fixtures.ROUTE_CREATE).headers["idempotency-key"])).is_not_equal(failed_key)


func test_a_boss_without_solo_is_never_started_alone() -> void:
	var before: int = s.h.http.count(F110Fixtures.ROUTE_CREATE)
	var r: ApiResult = await s.services.boss.start_solo(_boss(false))
	assert_bool(r.ok).is_false()
	assert_int(s.h.http.count(F110Fixtures.ROUTE_CREATE)).is_equal(before)


func test_boss_card_offers_only_invite_friends_for_a_party_only_boss() -> void:
	var card: BossInfoCard = auto_free((load("res://scenes/challenge/boss/BossInfoCard.tscn") as PackedScene).instantiate()) as BossInfoCard
	card.services = s.services
	add_child(card)
	await card.open({"area_id": "math_kingdom"})
	await s.settle()
	assert_object(card.solo_button(F110Fixtures.BOSS_ID)).is_not_null()
	assert_object(card.party_button(F110Fixtures.BOSS_ID)).is_not_null()
	assert_object(card.solo_button(F110Fixtures.BOSS_PARTY_ONLY_ID)).is_null()
	assert_object(card.party_button(F110Fixtures.BOSS_PARTY_ONLY_ID)).is_not_null()
	assert_str(s.h.http.last(F110Fixtures.ROUTE_BOSSES).query).contains("area_id=math_kingdom")
	card.close()


func test_solo_on_the_card_opens_the_arena_on_the_created_encounter() -> void:
	var opened: Array = []
	var sink: Callable = func(screen: StringName, args: Dictionary) -> void: opened.append([screen, args])
	EventBus.ui_requested.connect(sink)
	var card: BossInfoCard = auto_free((load("res://scenes/challenge/boss/BossInfoCard.tscn") as PackedScene).instantiate()) as BossInfoCard
	card.services = s.services
	add_child(card)
	await card.open({"area_id": "math_kingdom"})
	card.solo_button(F110Fixtures.BOSS_ID).pressed.emit()
	await s.settle(6)
	EventBus.ui_requested.disconnect(sink)
	assert_int(opened.size()).is_equal(1)
	assert_str(String(opened[0][0])).is_equal("boss_arena")
	assert_str(String((opened[0][1] as Dictionary)["encounter_id"])).is_equal(F110Fixtures.ENCOUNTER_ID)
	assert_str(String((opened[0][1] as Dictionary)["mode"])).is_equal("solo")


func test_eligibility_and_attempt_limit_refusals_are_friendly_sentences() -> void:
	for pair: Array in [[403, "CHALLENGE_NOT_ELIGIBLE"], [409, "CHALLENGE_ATTEMPT_LIMIT"]]:
		notices.clear()
		s.h.http.clear(F110Fixtures.ROUTE_CREATE).reply(F110Fixtures.ROUTE_CREATE, int(pair[0]),
				AuthHarness.problem(int(pair[0]), String(pair[1])))
		var r: ApiResult = await s.services.boss.start_solo(_boss())
		assert_bool(r.ok).is_false()
		assert_array(notices).contains(["ui.error.%s" % String(pair[1])])
		assert_bool(L10n.has_key("ui.error.%s" % String(pair[1]))).is_true()


func test_leave_sends_a_key_and_goes_back_to_the_zone_or_the_lobby() -> void:
	await s.services.boss.load_encounter(F110Fixtures.ENCOUNTER_ID)
	var r: ApiResult = await s.services.boss.leave()
	assert_bool(r.ok).is_true()
	var rec: FakeHttp.Recorded = s.h.http.last(F110Fixtures.ROUTE_LEAVE)
	assert_str(rec.path).is_equal("/game/boss-encounters/%s:leave" % F110Fixtures.ENCOUNTER_ID)
	assert_bool(Uuid.is_valid(String(rec.headers.get("idempotency-key", "")))).is_true()
	assert_array(exits).is_equal([BossClient.EXIT_ZONE])
	assert_object(s.services.boss.store.encounter).is_null()
	s.h.http.clear(F110Fixtures.ROUTE_ENCOUNTER).reply(F110Fixtures.ROUTE_ENCOUNTER, 200, F110Fixtures.encounter(s.self_id, "party"))
	await s.services.boss.load_encounter(F110Fixtures.ENCOUNTER_ID)
	await s.services.boss.leave()
	assert_array(exits).is_equal([BossClient.EXIT_ZONE, BossClient.EXIT_LOBBY])


func test_not_a_member_or_gone_leaves_to_the_hub_with_a_general_sentence() -> void:
	var gone: Array[String] = []
	s.services.boss.exit_requested.connect(func(_t: String, key: String) -> void: gone.append(key))
	await s.services.boss.load_encounter(F110Fixtures.ENCOUNTER_ID)
	s.h.http.clear(F110Fixtures.ROUTE_LEAVE).reply(F110Fixtures.ROUTE_LEAVE, 409, AuthHarness.problem(409, "ROOM_NOT_MEMBER"))
	await s.services.boss.leave()
	assert_array(exits).is_equal([BossClient.EXIT_HUB])
	assert_array(gone).is_equal([BossClient.KEY_GONE])
	s.h.http.clear(F110Fixtures.ROUTE_ENCOUNTER).reply(F110Fixtures.ROUTE_ENCOUNTER, 404, AuthHarness.problem(404, "NOT_FOUND"))
	await s.services.boss.load_encounter(F110Fixtures.ENCOUNTER_ID)
	assert_array(exits).is_equal([BossClient.EXIT_HUB, BossClient.EXIT_HUB])
	# No raw code and no detail in the sentence.
	assert_bool(notices.has("ui.error.ROOM_NOT_MEMBER")).is_false()
