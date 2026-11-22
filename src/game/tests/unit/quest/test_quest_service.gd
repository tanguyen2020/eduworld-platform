extends GdUnitTestSuite
## F072 QuestService (T019, T025 client part; F072-AC-01/04/09/14 as seen by the client, ADR-0016): the list is the
## server's, accept/report/start carry an Idempotency-Key and the game session of the device, a world interaction is
## reported only for an active step about it, a learning step opens the activity player on the server attempt, and
## `quest.progress.updated` updates the cached progress without a REST call. Runs on FakeHttp (AuthHarness).

const QUEST: String = "0193a5f4-6c1e-7b2a-9d1f-000000072001"
const ROUTE_LIST: String = "GET /game/quests"
const ROUTE_ACCEPT: String = "POST /game/quests/*:accept"
const ROUTE_REPORT: String = "POST /game/quests/*/steps/*:report"
const ROUTE_START: String = "POST /game/quests/*/activities:start"
const ROUTE_GET: String = "GET /game/quests/*"

var h: AuthHarness = null
var store: WorldFixtures.FakeStore = null
var service: QuestService = null
var opened: Array = []


static func steps() -> Array:
	return [
		{"step_id": "meet", "order": 1, "step_type": "talk_npc", "title": {"vi": "Gặp Ông Số", "en": "Meet Mr. Numbers"}, "optional": false,
			"area_id": "learning_zone.math", "npc_id": "npc.mr_numbers"},
		{"step_id": "solve", "order": 2, "step_type": "activity", "title": {"vi": "Giải bài", "en": "Solve"}, "optional": false},
		{"step_id": "baskets", "order": 3, "step_type": "collect", "title": {"vi": "Nhặt giỏ", "en": "Baskets"}, "optional": false,
			"area_id": "learning_zone.math", "interactable_ids": ["quest.math.apple_basket_1", "quest.math.apple_basket_2"], "count": 2},
	]


static func progress(states: Dictionary, status: String = "active") -> Dictionary:
	return {"quest_progress_id": "0193a5f4-6c1e-7b2a-9d1f-0000000f0001", "quest_id": QUEST, "quest_version_id": "v1", "status": status,
		"step_states": states, "started_at": "2026-11-22T01:00:00Z"}


static func listed(status: String, states: Dictionary = {}) -> Dictionary:
	var item: Dictionary = {"quest_id": QUEST, "quest_version_id": "v1", "code": "math.g3.market_helper", "quest_type": "side",
		"name": {"vi": "Phụ giúp phiên chợ", "en": "Market Helper"}, "grade_levels": [3], "status": status, "can_accept": status == "available",
		"start_trigger": "quest_board"}
	if status == "active":
		item["progress"] = progress(states)
	return {"data": [item], "page": {"has_more": false, "limit": 50}}


static func problem(code: String, status: int) -> Dictionary:
	return {"type": "https://errors.eduworld.vn/" + code.to_lower(), "title": "Conflict", "status": status, "code": code}


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	store = auto_free(WorldFixtures.FakeStore.new())
	service = QuestService.new(h.api, store)
	opened = []
	service.intent_sink = func(screen: StringName, args: Dictionary) -> void: opened.append([screen, args])


func after_test() -> void:
	h.dispose()


func test_the_list_is_the_server_list() -> void:
	h.http.reply(ROUTE_LIST, 200, listed("available"))
	var result: ApiResult = await service.refresh()
	assert_bool(result.ok).is_true()
	assert_int(service.quests.size()).is_equal(1)
	var q: QuestModel = service.quest(QUEST)
	assert_str(q.status).is_equal("available")
	assert_bool(q.can_accept).is_true()
	assert_str(q.display_name("en")).is_equal("Market Helper")


func test_accept_carries_an_idempotency_key_and_tracks_the_quest() -> void:
	h.http.reply(ROUTE_LIST, 200, listed("available"))
	await service.refresh()
	h.http.reply(ROUTE_ACCEPT, 200, progress({"meet": {"status": "active"}, "solve": {"status": "locked"}, "baskets": {"status": "locked"}}))
	h.http.reply(ROUTE_GET, 200, {"quest_id": QUEST, "status": "active", "steps": steps(),
		"progress": progress({"meet": {"status": "active"}, "solve": {"status": "locked"}, "baskets": {"status": "locked"}})})
	var result: ApiResult = await service.accept(QUEST)
	assert_bool(result.ok).is_true()
	var rec: FakeHttp.Recorded = h.http.last(ROUTE_ACCEPT)
	assert_str(String(rec.headers.get("idempotency-key", ""))).is_not_empty()
	assert_str(service.tracked_quest_id).is_equal(QUEST)
	assert_str(service.quest(QUEST).step_status("meet")).is_equal("active")


func test_an_interaction_is_reported_only_for_an_active_step_about_it() -> void:
	h.http.reply(ROUTE_LIST, 200, listed("active", {"meet": {"status": "active"}, "solve": {"status": "locked"}, "baskets": {"status": "locked"}}))
	h.http.reply(ROUTE_GET, 200, {"quest_id": QUEST, "steps": steps()})
	await service.refresh()
	assert_bool(await service.on_interaction("learning_zone.math", "quest.math.apple_basket_1")).is_false()
	assert_int(h.http.count(ROUTE_REPORT)).is_equal(0)
	h.http.reply(ROUTE_REPORT, 200, progress({"meet": {"status": "completed"}, "solve": {"status": "active"}, "baskets": {"status": "locked"}}))
	assert_bool(await service.on_interaction("learning_zone.math", "", "npc.mr_numbers")).is_true()
	var rec: FakeHttp.Recorded = h.http.last(ROUTE_REPORT)
	assert_str(rec.path).contains("/steps/meet:report")
	assert_str(String(rec.body["game_session_id"])).is_equal(WorldFixtures.SESSION_ID)
	assert_str(String(rec.body["npc_id"])).is_equal("npc.mr_numbers")
	assert_str(String(rec.headers.get("idempotency-key", ""))).is_not_empty()
	assert_str(service.quest(QUEST).step_status("solve")).is_equal("active")


func test_a_refused_report_shows_a_friendly_notice() -> void:
	h.http.reply(ROUTE_LIST, 200, listed("active", {"meet": {"status": "active"}}))
	h.http.reply(ROUTE_GET, 200, {"quest_id": QUEST, "steps": steps()})
	await service.refresh()
	var notices: Array[String] = []
	service.notice_requested.connect(func(key: String) -> void: notices.append(key))
	h.http.reply(ROUTE_REPORT, 409, problem("STEP_PRECONDITION_FAILED", 409))
	await service.report_step(QUEST, "meet", "main_hub", "", "npc.mr_numbers")
	assert_array(notices).contains(["ui.quest.notice.not_here"])
	assert_str(service.quest(QUEST).step_status("meet")).is_equal("active")


func test_a_learning_step_opens_the_activity_player_on_the_server_attempt() -> void:
	h.http.reply(ROUTE_START, 201, {"attempt_id": "a-1", "activity_id": "act-1", "mode": "quest", "status": "in_progress",
		"context_type": "quest_step", "context_id": "p-1", "started_at": "2026-11-22T01:00:00Z"})
	var result: ApiResult = await service.start_activity(QUEST, "solve")
	assert_bool(result.ok).is_true()
	assert_int(opened.size()).is_equal(1)
	assert_str(String(opened[0][0])).is_equal("activity_host")
	assert_str(String((opened[0][1] as Dictionary)["attempt_id"])).is_equal("a-1")
	assert_str(String((opened[0][1] as Dictionary)["mode"])).is_equal("quest")
	h.http.reply(ROUTE_START, 409, problem("QUEST_STEP_ATTEMPT_LIMIT", 409))
	var notices: Array[String] = []
	service.notice_requested.connect(func(key: String) -> void: notices.append(key))
	await service.start_activity(QUEST, "solve")
	assert_array(notices).contains(["ui.quest.notice.come_back_tomorrow"])


func test_realtime_progress_updates_the_cache_without_rest() -> void:
	h.http.reply(ROUTE_LIST, 200, listed("active", {"meet": {"status": "completed"}, "solve": {"status": "active"}, "baskets": {"status": "locked"}}))
	h.http.reply(ROUTE_GET, 200, {"quest_id": QUEST, "steps": steps()})
	await service.refresh()
	var before: int = h.http.requests.size()
	var completed: Array[String] = []
	service.quest_completed.connect(func(id: String) -> void: completed.append(id))
	service.call("_on_message", "quest.progress.updated", {"quest_id": QUEST, "status": "active", "step_id": "solve",
		"step_status": "completed", "next_step_ids": ["baskets"], "passed": true})
	assert_str(service.quest(QUEST).step_status("baskets")).is_equal("active")
	service.call("_on_message", "quest.progress.updated", {"quest_id": QUEST, "status": "completed", "step_id": "baskets",
		"step_status": "completed", "count": 2})
	assert_array(completed).contains([QUEST])
	assert_int(h.http.requests.size()).is_equal(before)
