extends GdUnitTestSuite
## F061-AC-31 (ADR-0086, AC-UX-0112): resume network error / 5xx / 429 → SessionStore kept, "Đang kết nối…", retry
## with `client_reconnect_backoff`, "Đăng nhập bằng tài khoản khác" available; `426` → ForceUpdate, record kept.

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()
	h.store_session()
	h.http.reply("GET /game/client-config", 200, AuthHarness.fixture("ClientConfig"))


func after_test() -> void:
	h.dispose()


func test_transient_errors_keep_the_record_and_retry() -> void:
	var attempts: Array[int] = []
	h.auth.connection_retried.connect(func(n: int) -> void:
		attempts.append(n)
		assert_str(String(h.stored_record().get("refresh_token", ""))).is_equal(AuthHarness.REFRESH_1)
		assert_bool(h.auth.connecting).is_true())
	h.http.fail("POST /auth/game/token:refresh")
	h.http.reply("POST /auth/game/token:refresh", 503, AuthHarness.problem(503, "UNAVAILABLE"))
	h.http.reply("POST /auth/game/token:refresh", 429, AuthHarness.problem(429, "RATE_LIMITED"), {"Retry-After": "3"})
	h.http.reply("POST /auth/game/token:refresh", 200, AuthHarness.refresh_ok(AuthHarness.ACCESS_2, AuthHarness.REFRESH_2, true))
	await h.auth.start()
	assert_array(attempts).is_equal([1, 2, 3])
	assert_int(h.http.count("POST /auth/game/token:refresh")).is_equal(4)
	assert_float(h.clock.waits[2]).is_equal(3.0)
	assert_float(h.clock.waits[0]).is_between(0.4, 0.6)
	assert_bool(h.auth.connecting).is_false()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.RESUME_CONFIRM)
	assert_array(h.states).not_contains([GameStateMachine.State.LOGIN])


func test_other_account_stops_retrying() -> void:
	h.http.fail("POST /auth/game/token:refresh")
	h.auth.start()
	for i: int in 30:
		if h.auth.connecting:
			break
		await get_tree().process_frame
	assert_bool(h.auth.connecting).is_true()
	await h.auth.switch_account()
	var count: int = h.http.count("POST /auth/game/token:refresh")
	for i: int in 10:
		await get_tree().process_frame
	assert_int(h.http.count("POST /auth/game/token:refresh")).is_less_equal(count + 1)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
	assert_dict(h.stored_record()).is_empty()
	assert_str(h.auth.form.username).is_empty()


func test_version_refused_shows_force_update_and_keeps_the_record() -> void:
	h.http.reply("POST /auth/game/token:refresh", 426, AuthHarness.problem(426, "CLIENT_VERSION_UNSUPPORTED",
			{"min_client_version": "3.0.0", "store_url": "https://store.example.invalid/eduworld"}, "ClientVersionProblem"))
	await h.auth.start()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.FORCE_UPDATE)
	assert_str(h.auth.min_client_version).is_equal("3.0.0")
	assert_str(h.auth.store_url).is_equal("https://store.example.invalid/eduworld")
	assert_str(String(h.stored_record()["refresh_token"])).is_equal(AuthHarness.REFRESH_1)
	assert_int(h.http.count("POST /auth/game/token:refresh")).is_equal(1)
