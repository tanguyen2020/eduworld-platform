extends GdUnitTestSuite
## F061-AC-17 (GAME_NETWORKING §2.3, CONFIG_DEFAULTS `client_reconnect_backoff`): automatic retries only for
## network/timeout/502/503/504 with GET or a keyed write, backoff 0.5–15 s ±20 % at most 3 times; `429` waits
## `Retry-After` at most 2 times; an unkeyed write is never retried; other errors are not retried.

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()


func after_test() -> void:
	h.dispose()


func _fetch(path: String) -> ApiResult:
	return await h.api.send(ApiRequest.make_get(path).as_public())


func test_get_retries_three_times_with_backoff() -> void:
	h.http.reply("GET /game/x", 503)
	var result: ApiResult = await _fetch("/game/x")
	assert_int(result.status).is_equal(503)
	assert_int(h.http.count("GET /game/x")).is_equal(4)
	assert_int(h.clock.waits.size()).is_equal(3)
	var bases: Array[float] = [0.5, 1.0, 2.0]
	for i: int in 3:
		assert_float(h.clock.waits[i]).is_between(bases[i] * 0.8, bases[i] * 1.2)


func test_network_error_then_success() -> void:
	h.http.fail("GET /game/x").reply("GET /game/x", 200, {"ok": true})
	var result: ApiResult = await _fetch("/game/x")
	assert_bool(result.ok).is_true()
	assert_int(h.http.count("GET /game/x")).is_equal(2)


func test_unkeyed_post_is_never_retried() -> void:
	for status: int in [502, 503, 504]:
		h.http.reply("POST /game/w%d" % status, status)
		var result: ApiResult = await h.api.send(ApiRequest.make_post("/game/w%d" % status, {}).as_public())
		assert_int(result.status).is_equal(status)
		assert_int(h.http.count("POST /game/w%d" % status)).is_equal(1)
	h.http.fail("POST /game/offline")
	await h.api.send(ApiRequest.make_post("/game/offline", {}).as_public())
	assert_int(h.http.count("POST /game/offline")).is_equal(1)
	assert_array(h.clock.waits).is_empty()


func test_keyed_post_is_retried() -> void:
	h.http.reply("POST /game/w", 502).reply("POST /game/w", 504).reply("POST /game/w", 200, {})
	var req: ApiRequest = ApiRequest.make_post("/game/w", {}).as_public()
	req.idempotency_key = Uuid.v7()
	var result: ApiResult = await h.api.send(req)
	assert_bool(result.ok).is_true()
	assert_int(h.http.count("POST /game/w")).is_equal(3)


func test_rate_limit_waits_retry_after_twice() -> void:
	h.http.reply("GET /game/x", 429, AuthHarness.problem(429, "RATE_LIMITED"), {"Retry-After": "7"})
	var result: ApiResult = await _fetch("/game/x")
	assert_str(result.code).is_equal("RATE_LIMITED")
	assert_int(h.http.count("GET /game/x")).is_equal(3)
	assert_array(h.clock.waits).is_equal([7.0, 7.0])


func test_other_errors_are_not_retried() -> void:
	for status: int in [400, 404, 409, 500]:
		h.http.reply("GET /game/e%d" % status, status, AuthHarness.problem(status, "BAD_REQUEST"))
		await _fetch("/game/e%d" % status)
		assert_int(h.http.count("GET /game/e%d" % status)).override_failure_message(str(status)).is_equal(1)
	assert_array(h.clock.waits).is_empty()


func test_backoff_formula() -> void:
	assert_float(Backoff.delay(1)).is_equal(0.5)
	assert_float(Backoff.delay(2)).is_equal(1.0)
	assert_float(Backoff.delay(10)).is_equal(15.0)
	assert_float(Backoff.queue_delay(4.0)).is_equal(4.0)
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	for i: int in 50:
		assert_float(Backoff.queue_delay(4.0, rng)).is_between(4.0, 6.0)
