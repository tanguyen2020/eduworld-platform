extends GdUnitTestSuite
## T019 (F111-AC-09, F111-AC-10, F111-AC-12; EVENT_SEASON_SYSTEM §3–§5, §9–§11): `listGameEvents` with
## `status = live, upcoming` (repeated keys) — the server already filtered by scope and grade, nothing is filtered here;
## a tenant name without `en` shows `vi`; the time left is whole days from `ends_at` on the server clock; "Tham gia" is
## `joinGameEvent` with one Idempotency-Key kept when retried; `404 EVENT_NOT_AVAILABLE` closes the detail with a
## friendly sentence and reads the list again, `409 EVENT_ENDED` refreshes with a friendly sentence;
## `event.status.changed`: suspended / ended change the card at once and the list is read again, cancelled removes it.

var s: F111SceneHarness = null
var live: LiveEventService = null
var notices: Array[String] = []
var closed: Array[String] = []


func before_test() -> void:
	s = F111SceneHarness.new()
	await s.start("g4_5")
	live = s.services.live_events
	notices.clear()
	closed.clear()
	live.notice_requested.connect(func(key: String, _args: Dictionary) -> void: notices.append(key))
	live.detail_closed.connect(func(event_id: String, _key: String) -> void: closed.append(event_id))


func after_test() -> void:
	s.dispose()


func test_list_live_and_upcoming_unfiltered_with_days_left() -> void:
	var r: ApiResult = await live.load_list()
	assert_bool(r.ok).is_true()
	assert_str(s.h.http.last(F111Fixtures.ROUTE_EVENTS).query).starts_with("status=live&status=upcoming")
	assert_int(live.store.events.size()).is_equal(3)
	var school: LiveEventModel = live.store.event(F111Fixtures.EVENT_SCHOOL)
	assert_str(school.display_name("en")).is_equal("Tuần lễ đọc sách")
	assert_int(live.store.event(F111Fixtures.EVENT_FESTIVAL).days_left(live.now_unix())).is_equal(5)
	assert_str(live.store.event(F111Fixtures.EVENT_UPCOMING).time_left_key(live.now_unix())).is_equal("ui.events.upcoming")
	assert_bool(live.store.has_new).is_true()
	live.store.mark_seen()
	assert_bool(live.store.has_new).is_false()


func test_join_keeps_its_key_when_retried() -> void:
	await live.load_list()
	s.h.http.clear(F111Fixtures.ROUTE_EVENT_JOIN).fail(F111Fixtures.ROUTE_EVENT_JOIN)
	await live.join(F111Fixtures.EVENT_FESTIVAL)
	var key: String = String(s.h.http.last(F111Fixtures.ROUTE_EVENT_JOIN).headers["idempotency-key"])
	assert_str(key).is_not_empty()
	s.h.http.clear(F111Fixtures.ROUTE_EVENT_JOIN).reply(F111Fixtures.ROUTE_EVENT_JOIN, 200,
			{"event_run_id": F111Fixtures.RUN_FESTIVAL, "joined_at": F111Fixtures.iso(s.now_sec), "event_points": 0})
	var r: ApiResult = await live.join(F111Fixtures.EVENT_FESTIVAL)
	assert_bool(r.ok).is_true()
	assert_str(String(s.h.http.last(F111Fixtures.ROUTE_EVENT_JOIN).headers["idempotency-key"])).is_equal(key)
	assert_bool(live.store.event(F111Fixtures.EVENT_FESTIVAL).joined).is_true()
	# A new intent after the answer gets a new key.
	await live.join(F111Fixtures.EVENT_FESTIVAL)
	assert_str(String(s.h.http.last(F111Fixtures.ROUTE_EVENT_JOIN).headers["idempotency-key"])).is_not_equal(key)


func test_gone_closes_the_detail_and_ended_refreshes_with_a_sentence() -> void:
	await live.load_list()
	s.h.http.clear(F111Fixtures.ROUTE_EVENT).reply(F111Fixtures.ROUTE_EVENT, 404, AuthHarness.problem(404, "EVENT_NOT_AVAILABLE"))
	var lists: int = s.h.http.count(F111Fixtures.ROUTE_EVENTS)
	await live.get_detail(F111Fixtures.EVENT_FESTIVAL)
	assert_array(closed).contains([F111Fixtures.EVENT_FESTIVAL])
	assert_int(s.h.http.count(F111Fixtures.ROUTE_EVENTS)).is_greater(lists)
	assert_array(notices).contains(["ui.error.EVENT_NOT_AVAILABLE"])
	s.h.http.clear(F111Fixtures.ROUTE_EVENT).reply(F111Fixtures.ROUTE_EVENT, 200, F111Fixtures.detail(s.now_sec))
	s.h.http.clear(F111Fixtures.ROUTE_EVENT_JOIN).reply(F111Fixtures.ROUTE_EVENT_JOIN, 409, AuthHarness.problem(409, "EVENT_ENDED"))
	var details: int = s.h.http.count(F111Fixtures.ROUTE_EVENT)
	await live.join(F111Fixtures.EVENT_FESTIVAL)
	assert_array(notices).contains(["ui.error.EVENT_ENDED"])
	assert_int(s.h.http.count(F111Fixtures.ROUTE_EVENT)).is_greater(details)


func test_status_changes_update_the_cards_and_read_the_list_again() -> void:
	await live.load_list()
	var lists: int = s.h.http.count(F111Fixtures.ROUTE_EVENTS)
	s.push("event.status.changed", F111Fixtures.ws("event.status.changed", {"event_id": F111Fixtures.EVENT_FESTIVAL,
		"event_run_id": F111Fixtures.RUN_FESTIVAL, "status": "suspended"}))
	assert_bool(live.store.event(F111Fixtures.EVENT_FESTIVAL).is_suspended()).is_true()
	await s.settle(4)
	assert_int(s.h.http.count(F111Fixtures.ROUTE_EVENTS)).is_greater(lists)
	s.push("event.status.changed", F111Fixtures.ws("event.status.changed", {"event_id": F111Fixtures.EVENT_SCHOOL,
		"event_run_id": F111Fixtures.EVENT_SCHOOL, "status": "cancelled"}))
	assert_object(live.store.event(F111Fixtures.EVENT_SCHOOL)).is_null()
	s.push("event.status.changed", F111Fixtures.ws("event.status.changed", {"event_id": F111Fixtures.EVENT_UPCOMING,
		"event_run_id": F111Fixtures.EVENT_UPCOMING, "status": "ended"}))
	assert_bool(live.store.event(F111Fixtures.EVENT_UPCOMING).is_ended()).is_true()
	await s.settle(4)
