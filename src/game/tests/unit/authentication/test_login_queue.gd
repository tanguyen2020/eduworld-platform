extends GdUnitTestSuite
## F061-AC-06 (TC-AUTH-044, TC-AUTH-063, ADR-0045, GAME_NETWORKING §3): `429 LOGIN_QUEUE_BUSY` → "Đang vào lớp…",
## automatic resend after `Retry-After` + jitter (0–50 %) until success or Hủy; `429 RATE_LIMITED` → countdown from
## `Retry-After`, no automatic resend.

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()


func after_test() -> void:
	h.dispose()


func _busy() -> Dictionary:
	return AuthHarness.problem(429, "LOGIN_QUEUE_BUSY")


func test_queue_retries_until_success() -> void:
	h.http.reply("POST /auth/game/login", 429, _busy(), {"Retry-After": "4"})
	h.http.reply("POST /auth/game/login", 429, _busy(), {"Retry-After": "4"})
	h.http.reply("POST /auth/game/login", 200, AuthHarness.login_ok())
	h.http.reply("POST /game/sessions", 201, AuthHarness.session_ok())
	var waited: Array[float] = []
	h.auth.queue_waited.connect(func(seconds: float) -> void: waited.append(seconds))
	await h.auth.login("hs.an.3a", AuthHarness.PASSWORD)
	assert_int(h.http.count("POST /auth/game/login")).is_equal(3)
	assert_array(h.states).contains([GameStateMachine.State.LOGIN_WAITING])
	assert_int(waited.size()).is_equal(2)
	for w: float in waited:
		assert_float(w).is_between(4.0, 6.0)
	for rec: FakeHttp.Recorded in h.http.find("POST /auth/game/login"):
		assert_str(String(rec.body["password"])).is_equal(AuthHarness.PASSWORD)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.IN_WORLD)
	assert_bool(h.auth.holds_password()).is_false()


func test_cancel_stops_the_queue_and_wipes_the_password() -> void:
	h.http.reply("POST /auth/game/login", 429, _busy(), {"Retry-After": "1500"})
	h.auth.login("hs.an.3a", AuthHarness.PASSWORD)
	for i: int in 30:
		if h.auth.state == GameStateMachine.State.LOGIN_WAITING:
			break
		await get_tree().process_frame
	assert_int(h.auth.state).is_equal(GameStateMachine.State.LOGIN_WAITING)
	assert_bool(h.auth.holds_password()).is_true()
	h.auth.cancel_login()
	assert_bool(h.auth.holds_password()).is_false()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
	h.clock.release()
	for i: int in 5:
		await get_tree().process_frame
	assert_int(h.http.count("POST /auth/game/login")).is_equal(1)


func test_rate_limited_counts_down_without_resending() -> void:
	h.http.reply("POST /auth/game/login", 429, AuthHarness.problem(429, "RATE_LIMITED"), {"Retry-After": "60"})
	await h.auth.login("hs.an.3a", AuthHarness.PASSWORD)
	for i: int in 5:
		await get_tree().process_frame
	assert_int(h.http.count("POST /auth/game/login")).is_equal(1)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
	assert_str(h.auth.form.error_code).is_equal("RATE_LIMITED")
	assert_int(h.auth.form.rate_limit_left_sec(h.clock.now_msec())).is_equal(60)
	h.clock.advance(45.0)
	assert_int(h.auth.form.rate_limit_left_sec(h.clock.now_msec())).is_equal(15)
	assert_bool(h.auth.form.tick_rate_limit(h.clock.now_msec())).is_true()
	h.clock.advance(15.0)
	assert_bool(h.auth.form.tick_rate_limit(h.clock.now_msec())).is_false()
	assert_int(h.auth.form.rate_limited_until_msec).is_equal(0)
	assert_bool(h.auth.holds_password()).is_false()
