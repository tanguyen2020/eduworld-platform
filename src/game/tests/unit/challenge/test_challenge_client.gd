extends GdUnitTestSuite
## F069 ChallengeClient (T025, T026; F069-AC-15…AC-20; CHALLENGE_BOSS_SYSTEM §2, ADR-0016): start keeps its key on retry
## and maps the refusals to friendly notices (retry time, continue the run in progress), `next` uses one key per turn,
## retries a `503` with the same key and opens the activity player on the server's attempt, the run ends on `done`, the
## clock follows `deadline_at` on the server time (a changed device clock changes nothing), messages of another attempt
## are ignored, `active_attempt_id` resumes a run, `422 INVALID_STATE_TRANSITION` reloads the server state.

var h: AuthHarness = null
var client: ChallengeClient = null
var store: BlockingStoreFake = null
var opened: Array = []
var notices: Array[String] = []


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	store = auto_free(BlockingStoreFake.new())
	client = ChallengeClient.new(h.api, store, h.clock)
	opened = []
	notices = []
	client.intent_sink = func(screen: StringName, args: Dictionary) -> void: opened.append([screen, args])
	client.notice_requested.connect(func(key: String, _args: Dictionary) -> void: notices.append(key))
	h.http.reply(F069Fixtures.ROUTE_CHALLENGES, 200, F069Fixtures.challenges("g6_7"))
	h.http.reply(F069Fixtures.ROUTE_CHALLENGE, 200, F069Fixtures.challenge_detail("g6_7"), {"Date": F069Fixtures.SERVER_DATE})


func after_test() -> void:
	h.dispose()


func _start() -> void:
	await client.list()
	await client.get_challenge(F069Fixtures.CHALLENGE_TIME)
	h.http.reply(F069Fixtures.ROUTE_START, 201, F069Fixtures.attempt(), {"Date": F069Fixtures.SERVER_DATE})
	h.http.reply(F069Fixtures.ROUTE_NEXT, 200, F069Fixtures.next_activity(F069Fixtures.LEARNING_ATTEMPT_1), {"Date": F069Fixtures.SERVER_DATE})
	await client.start(F069Fixtures.CHALLENGE_TIME, "hard")


func test_start_then_next_opens_the_activity_player_on_the_server_attempt() -> void:
	await _start()
	var start: FakeHttp.Recorded = h.http.last(F069Fixtures.ROUTE_START)
	assert_str(String(start.body["difficulty"])).is_equal("hard")
	assert_bool(Uuid.is_valid(String(start.headers.get("idempotency-key", "")))).is_true()
	assert_bool(client.is_running()).is_true()
	assert_str(String(store.blocking_activity)).is_equal("challenge")
	assert_int(opened.size()).is_equal(1)
	assert_str(String(opened[0][0])).is_equal("activity_host")
	var args: Dictionary = opened[0][1] as Dictionary
	assert_str(String(args["attempt_id"])).is_equal(F069Fixtures.LEARNING_ATTEMPT_1)
	assert_str(String(args["context_id"])).is_equal(F069Fixtures.ATTEMPT_ID)
	assert_str(client.activity_attempt_id).is_equal(F069Fixtures.LEARNING_ATTEMPT_1)


func test_next_uses_one_key_per_turn_and_retries_503_with_the_same_key() -> void:
	await _start()
	var first_key: String = String(h.http.last(F069Fixtures.ROUTE_NEXT).headers["idempotency-key"])
	h.http.clear(F069Fixtures.ROUTE_NEXT)
	h.http.reply(F069Fixtures.ROUTE_NEXT, 503, F069Fixtures.problem("UNAVAILABLE", 503), {"Retry-After": "1"})
	h.http.reply(F069Fixtures.ROUTE_NEXT, 503, F069Fixtures.problem("UNAVAILABLE", 503), {"Retry-After": "1"})
	h.http.reply(F069Fixtures.ROUTE_NEXT, 503, F069Fixtures.problem("UNAVAILABLE", 503), {"Retry-After": "1"})
	h.http.reply(F069Fixtures.ROUTE_NEXT, 503, F069Fixtures.problem("UNAVAILABLE", 503), {"Retry-After": "1"})
	h.http.reply(F069Fixtures.ROUTE_NEXT, 200, F069Fixtures.next_activity(F069Fixtures.LEARNING_ATTEMPT_2, 1))
	var before: int = h.http.count(F069Fixtures.ROUTE_NEXT)
	var result: ApiResult = await client.next()
	assert_bool(result.ok).is_true()
	var turn_keys: PackedStringArray = []
	var recs: Array[FakeHttp.Recorded] = h.http.find(F069Fixtures.ROUTE_NEXT)
	for i: int in range(before, recs.size()):
		turn_keys.append(String(recs[i].headers["idempotency-key"]))
	assert_int(turn_keys.size()).is_greater(1)
	for k: String in turn_keys:
		assert_str(k).is_equal(turn_keys[0])
	assert_str(turn_keys[0]).is_not_equal(first_key)
	assert_str(String((opened.back() as Array)[1]["attempt_id"])).is_equal(F069Fixtures.LEARNING_ATTEMPT_2)
	assert_int(client.attempt().activities_completed).is_equal(1)


func test_learning_submission_fetches_the_next_activity() -> void:
	await _start()
	h.http.reply(F069Fixtures.ROUTE_NEXT, 200, F069Fixtures.next_activity(F069Fixtures.LEARNING_ATTEMPT_2, 1))
	var submitted: AttemptModel = AttemptModel.from_dict(AuthHarness.fixture("Attempt"), 0.0)
	submitted.attempt_id = F069Fixtures.LEARNING_ATTEMPT_1
	await client.call("_on_learning_submitted", submitted)
	assert_str(client.activity_attempt_id).is_equal(F069Fixtures.LEARNING_ATTEMPT_2)


func test_done_finishes_the_run_and_opens_the_result() -> void:
	await _start()
	var finished: Array[ChallengeAttemptModel] = []
	client.challenge_finished.connect(func(a: ChallengeAttemptModel) -> void: finished.append(a))
	h.http.reply(F069Fixtures.ROUTE_NEXT, 200, F069Fixtures.next_activity("", 5, true))
	await client.next()
	assert_int(finished.size()).is_equal(1)
	assert_str(finished[0].status).is_equal("succeeded")
	assert_str(String((opened.back() as Array)[0])).is_equal("challenge_result")
	# The run stays "blocking" until the result screen is closed (data-model §2: Starting … Result).
	assert_str(String(store.blocking_activity)).is_equal("challenge")
	client.dismiss_result()
	assert_str(String(store.blocking_activity)).is_empty()


func test_start_refusals_are_friendly_notices() -> void:
	await client.list()
	h.http.reply(F069Fixtures.ROUTE_START, 403, F069Fixtures.problem("CHALLENGE_NOT_ELIGIBLE", 403))
	await client.start(F069Fixtures.CHALLENGE_TIME, "expert")
	assert_array(notices).contains(["ui.challenge.notice.not_eligible"])
	h.http.clear(F069Fixtures.ROUTE_START)
	h.http.reply(F069Fixtures.ROUTE_START, 409, F069Fixtures.problem("CHALLENGE_ATTEMPT_LIMIT", 409, {"retry_after": 3600}),
			{"Date": F069Fixtures.SERVER_DATE})
	await client.start(F069Fixtures.CHALLENGE_TIME, "easy")
	assert_array(notices).contains(["ui.challenge.notice.attempt_limit_at"])
	var limited: ApiResult = ApiResult.new()
	limited.data = F069Fixtures.problem("CHALLENGE_ATTEMPT_LIMIT", 409, {"retry_after": 3600})
	limited.headers = {"date": F069Fixtures.SERVER_DATE}
	var at: int = ChallengeClient.http_date_ms(F069Fixtures.SERVER_DATE) / 1000 + 3600
	assert_str(client.retry_time_text(limited)).is_equal(LocaleFormat.time(LocaleFormat.from_unix(at)))


func test_conflict_offers_to_continue_the_run_in_progress() -> void:
	h.http.clear(F069Fixtures.ROUTE_CHALLENGES)
	h.http.reply(F069Fixtures.ROUTE_CHALLENGES, 200, F069Fixtures.challenges("g6_7", 3, F069Fixtures.ATTEMPT_ID))
	var offered: Array[String] = []
	client.resume_offered.connect(func(id: String) -> void: offered.append(id))
	await client.list()
	assert_array(offered).contains([F069Fixtures.ATTEMPT_ID])
	h.http.reply(F069Fixtures.ROUTE_START, 409, F069Fixtures.problem("CONFLICT", 409))
	await client.start(F069Fixtures.CHALLENGE_TIME, "easy")
	assert_array(notices).contains(["ui.challenge.notice.in_progress"])
	assert_int(offered.size()).is_equal(2)


func test_start_key_is_kept_on_a_network_retry() -> void:
	await client.list()
	h.http.fail(F069Fixtures.ROUTE_START)
	await client.start(F069Fixtures.CHALLENGE_TIME, "easy")
	var key: String = String(h.http.last(F069Fixtures.ROUTE_START).headers["idempotency-key"])
	h.http.clear(F069Fixtures.ROUTE_START)
	h.http.reply(F069Fixtures.ROUTE_START, 201, F069Fixtures.attempt())
	h.http.reply(F069Fixtures.ROUTE_NEXT, 200, F069Fixtures.next_activity(F069Fixtures.LEARNING_ATTEMPT_1))
	await client.start(F069Fixtures.CHALLENGE_TIME, "easy")
	assert_str(String(h.http.last(F069Fixtures.ROUTE_START).headers["idempotency-key"])).is_equal(key)


func test_clock_follows_deadline_and_ignores_the_device_clock() -> void:
	await _start()
	# Server said 07:30:00, deadline 07:33:00 → 180 s left.
	assert_int(ExamCountdown.display_seconds(client.store.remaining_ms())).is_equal(180)
	h.clock.unix += 600.0 # the device clock jumps 10 minutes
	assert_int(ExamCountdown.display_seconds(client.store.remaining_ms())).is_equal(180)
	h.clock.now_ms += 30_000 # 30 s of real (monotonic) time
	assert_int(ExamCountdown.display_seconds(client.store.remaining_ms())).is_equal(150)
	h.clock.unix -= 1200.0
	assert_int(ExamCountdown.display_seconds(client.store.remaining_ms())).is_equal(150)


func test_messages_of_another_attempt_are_ignored() -> void:
	await _start()
	client.handle_message(ChallengeClient.MSG_UPDATED, {"challenge_attempt_id": Uuid.v7(), "status": "failed"})
	assert_bool(client.is_running()).is_true()
	client.handle_message(ChallengeClient.MSG_UPDATED, {"challenge_attempt_id": F069Fixtures.ATTEMPT_ID,
		"status": "in_progress", "score": 30, "lives_left": 2, "activities_completed": 3})
	assert_int(client.attempt().lives_left).is_equal(2)
	assert_int(client.attempt().activities_completed).is_equal(3)
	client.handle_message(ChallengeClient.MSG_UPDATED, {"challenge_attempt_id": F069Fixtures.ATTEMPT_ID, "status": "failed",
		"activities_completed": 4})
	assert_bool(client.is_running()).is_false()
	# The last activity is still on screen: its feedback stays, the result opens when the player closes.
	assert_str(String((opened.back() as Array)[0])).is_equal("activity_host")
	client.on_activity_closed()
	assert_str(String((opened.back() as Array)[0])).is_equal("challenge_result")


func test_end_message_without_an_open_activity_opens_the_result_at_once() -> void:
	h.http.reply(F069Fixtures.ROUTE_ATTEMPT, 200, F069Fixtures.attempt("in_progress", 2, 2), {"Date": F069Fixtures.SERVER_DATE})
	await client.resume(F069Fixtures.ATTEMPT_ID)
	client.handle_message(ChallengeClient.MSG_UPDATED, {"challenge_attempt_id": F069Fixtures.ATTEMPT_ID, "status": "succeeded"})
	assert_str(String((opened.back() as Array)[0])).is_equal("challenge_result")


func test_resume_takes_back_a_run_and_invalid_state_reloads() -> void:
	h.http.reply(F069Fixtures.ROUTE_ATTEMPT, 200, F069Fixtures.attempt("in_progress", 2, 2), {"Date": F069Fixtures.SERVER_DATE})
	var result: ApiResult = await client.resume(F069Fixtures.ATTEMPT_ID)
	assert_bool(result.ok).is_true()
	assert_bool(client.is_running()).is_true()
	assert_int(client.attempt().activities_completed).is_equal(2)
	h.http.reply(F069Fixtures.ROUTE_NEXT, 422, F069Fixtures.problem("INVALID_STATE_TRANSITION", 422))
	h.http.clear(F069Fixtures.ROUTE_ATTEMPT)
	h.http.reply(F069Fixtures.ROUTE_ATTEMPT, 200, F069Fixtures.attempt("failed", 4, 0))
	await client.next()
	assert_str(client.attempt().status).is_equal("failed")
	assert_str(String((opened.back() as Array)[0])).is_equal("challenge_result")


func test_abandon_carries_a_key_and_ends_the_run() -> void:
	await _start()
	h.http.reply(F069Fixtures.ROUTE_ABANDON, 200, F069Fixtures.attempt("abandoned", 1))
	await client.abandon()
	assert_bool(Uuid.is_valid(String(h.http.last(F069Fixtures.ROUTE_ABANDON).headers.get("idempotency-key", "")))).is_true()
	assert_str(client.attempt().status).is_equal("abandoned")
	assert_str(String((opened.back() as Array)[0])).is_equal("challenge_result")


func test_not_found_detail_refreshes_the_list() -> void:
	await client.list()
	var before: int = h.http.count(F069Fixtures.ROUTE_CHALLENGES)
	h.http.clear(F069Fixtures.ROUTE_CHALLENGE)
	h.http.reply(F069Fixtures.ROUTE_CHALLENGE, 404, F069Fixtures.problem("NOT_FOUND", 404))
	var detail: ChallengeDetailModel = await client.get_challenge(F069Fixtures.CHALLENGE_TIME)
	assert_object(detail).is_null()
	assert_array(notices).contains(["ui.challenge.gone"])
	assert_int(h.http.count(F069Fixtures.ROUTE_CHALLENGES)).is_greater(before)


func test_no_request_carries_score_lives_or_result() -> void:
	await _start()
	h.http.reply(F069Fixtures.ROUTE_ABANDON, 200, F069Fixtures.attempt("abandoned", 1))
	await client.abandon()
	for rec: FakeHttp.Recorded in h.http.requests:
		for field: String in ["score", "lives_left", "activities_completed", "status", "xp", "amount", "result"]:
			assert_bool(rec.body.has(field)).override_failure_message("%s %s" % [rec.path, field]).is_false()
