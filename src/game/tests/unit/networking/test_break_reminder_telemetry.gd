extends GdUnitTestSuite
## F062-AC-24 (FR-GAME-012, FR-ANL-010, GAME_ANALYTICS §3.2, AGENTS R14; T034): each reminder actually shown queues one
## `break_reminder_shown` (fresh `client_event_id`, `occurred_at`, `reminder_no` 1, 2…, `minutes_played`) that goes out
## in the next `postTelemetry` batch; a deferred reminder sends nothing until shown; offline, the event stays queued and
## is resent with the same `client_event_id`; it is sent even when product telemetry is off; it carries no name,
## username, student id or position.

const ROUTE: String = "POST /game/telemetry"

var h: AuthHarness = null
var services: ServiceLocator = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	services = ServiceLocator.new(h.api, AppState, h.clock)
	services.start_session({
		"policies": {"play_time": {"break_reminder_minutes": 1}},
		"feature_flags": {"product_telemetry_enabled": false},
	})


func after_test() -> void:
	h.dispose()


func _play(seconds: float) -> void:
	for i: int in int(seconds):
		services.play_time.tick(1.0)


func _events(rec: FakeHttp.Recorded) -> Array:
	return rec.body.get("events", []) as Array


func test_shown_reminders_are_batched_with_increasing_numbers() -> void:
	h.http.reply(ROUTE, 202)
	_play(60.0)
	_play(60.0)
	var result: ApiResult = await services.telemetry.flush(true)
	assert_bool(result.ok).is_true()
	var events: Array = _events(h.http.last(ROUTE))
	assert_int(events.size()).is_equal(2)
	for i: int in 2:
		var e: Dictionary = events[i] as Dictionary
		assert_str(String(e["type"])).is_equal("break_reminder_shown")
		assert_int(int(e["reminder_no"])).is_equal(i + 1)
		assert_int(int(e["minutes_played"])).is_equal(i + 1)
		assert_bool(Uuid.is_valid(String(e["client_event_id"]))).is_true()
		assert_str(String(e["occurred_at"])).ends_with("Z")
	assert_str(String((events[0] as Dictionary)["client_event_id"])).is_not_equal(
			String((events[1] as Dictionary)["client_event_id"]))
	assert_str(String(h.http.last(ROUTE).headers.get("x-tenant-id", ""))).is_equal(AuthHarness.TENANT_ID)
	assert_bool(services.telemetry.queue.is_empty()).is_true()


func test_deferred_reminder_is_not_sent_until_shown() -> void:
	services.play_time.hold(&"exam_lobby")
	_play(90.0)
	assert_bool(services.telemetry.queue.is_empty()).is_true()
	services.play_time.release(&"exam_lobby")
	assert_int(services.telemetry.queue.size()).is_equal(1)


func test_offline_keeps_the_event_and_resends_the_same_id() -> void:
	_play(60.0)
	h.http.fail(ROUTE)
	var first: ApiResult = await services.telemetry.flush(true)
	assert_bool(first.ok).is_false()
	assert_int(services.telemetry.queue.size()).is_equal(1)
	var id: String = String(_events(h.http.last(ROUTE))[0]["client_event_id"])
	# An unkeyed POST is never retried by ApiClient: exactly one attempt per flush.
	assert_int(h.http.count(ROUTE)).is_equal(1)
	h.http.reply(ROUTE, 202)
	var second: ApiResult = await services.telemetry.flush(true)
	assert_bool(second.ok).is_true()
	assert_str(String(_events(h.http.last(ROUTE))[0]["client_event_id"])).is_equal(id)
	assert_bool(services.telemetry.queue.is_empty()).is_true()


func test_product_telemetry_off_drops_only_product_events() -> void:
	assert_bool(services.telemetry.product_telemetry_enabled).is_false()
	assert_bool(services.telemetry.queue_event("screen_view", {"screen_code": "hub"}).is_empty()).is_true()
	assert_bool(services.telemetry.queue_event("tutorial_step", {"step_code": "s1"}).is_empty()).is_true()
	_play(60.0)
	assert_int(services.telemetry.queue.size()).is_equal(1)
	assert_str(String(services.telemetry.queue[0]["type"])).is_equal("break_reminder_shown")


func test_event_carries_no_identity_or_position() -> void:
	AppState.player = AuthHarness.bootstrap()
	_play(60.0)
	h.http.reply(ROUTE, 202)
	await services.telemetry.flush(true)
	var raw: String = h.http.last(ROUTE).raw_body
	var e: Dictionary = _events(h.http.last(ROUTE))[0] as Dictionary
	for key: String in e.keys():
		assert_bool(key in ["type", "client_event_id", "occurred_at", "reminder_no", "minutes_played"]) \
				.override_failure_message(key).is_true()
	for forbidden: String in ["hs.an.3a", "MeoXanh42", "student_id", "username", "position", "nickname"]:
		assert_str(raw).override_failure_message(forbidden).not_contains(forbidden)


func test_flush_respects_the_rate_limit_interval() -> void:
	h.http.reply(ROUTE, 202)
	_play(60.0)
	assert_bool(services.telemetry.can_flush()).is_true()
	await services.telemetry.flush()
	_play(60.0)
	assert_bool(services.telemetry.can_flush()).is_false()
	h.clock.advance(TelemetryService.MIN_FLUSH_INTERVAL_SEC)
	assert_bool(services.telemetry.can_flush()).is_true()


func test_session_end_drops_unsent_events_of_the_student() -> void:
	_play(60.0)
	assert_int(services.telemetry.queue.size()).is_equal(1)
	services.end_session()
	assert_bool(services.telemetry.queue.is_empty()).is_true()
	assert_bool(services.play_time.is_enabled()).is_false()
