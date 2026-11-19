extends GdUnitTestSuite
## F061-AC-25 (TC-AUTH-047, TC-AUTH-045, `shared_device_idle_logout` 30 min): a shared device idle for 30 minutes
## logs out; a mobile app back from the background (or reopened) continues with the refresh token, without the
## password — a session older than `ws_timeout` is created again.

var h: AuthHarness = null


func after_test() -> void:
	if h != null:
		h.dispose()
		h = null


func test_idle_limit_only_when_enabled() -> void:
	var watcher: LifecycleWatcher = auto_free(LifecycleWatcher.new()) as LifecycleWatcher
	var clock: FakeClock = FakeClock.new()
	watcher.clock = clock
	watcher.touch()
	var reached: Array[int] = [0]
	watcher.idle_limit_reached.connect(func() -> void: reached[0] += 1)
	clock.advance(31 * 60)
	assert_bool(watcher.check_idle()).is_false()
	watcher.idle_enabled = true
	watcher.touch()
	clock.advance(29 * 60)
	assert_bool(watcher.check_idle()).is_false()
	clock.advance(60)
	assert_bool(watcher.check_idle()).is_true()
	assert_bool(watcher.check_idle()).is_false()
	assert_int(reached[0]).is_equal(1)
	assert_float(watcher.idle_limit_sec).is_equal(1800.0)


func test_pause_resume_reports_the_background_time() -> void:
	var watcher: LifecycleWatcher = auto_free(LifecycleWatcher.new()) as LifecycleWatcher
	var clock: FakeClock = FakeClock.new()
	watcher.clock = clock
	var resumed: Array[float] = []
	watcher.app_resumed.connect(func(sec: float) -> void: resumed.append(sec))
	var closes: Array[int] = [0]
	watcher.close_requested.connect(func() -> void: closes[0] += 1)
	watcher.notification(Node.NOTIFICATION_APPLICATION_PAUSED)
	clock.advance(600)
	watcher.notification(Node.NOTIFICATION_APPLICATION_RESUMED)
	watcher.notification(Node.NOTIFICATION_WM_CLOSE_REQUEST)
	assert_array(resumed).is_equal([600.0])
	assert_int(closes[0]).is_equal(1)


func test_shared_device_idle_logs_out() -> void:
	h = AuthHarness.create(true)
	await h.sign_in()
	await h.auth.on_idle_limit_reached()
	assert_int(h.http.count("POST /auth/game/logout")).is_equal(1)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)


func test_personal_device_idle_does_nothing() -> void:
	h = AuthHarness.create(false)
	await h.sign_in()
	await h.auth.on_idle_limit_reached()
	assert_int(h.http.count("POST /auth/game/logout")).is_equal(0)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.IN_WORLD)


func test_back_from_background_continues_without_password() -> void:
	h = AuthHarness.create(false)
	await h.sign_in()
	h.http.reply("POST /game/sessions/*:heartbeat", 204)
	await h.auth.on_app_resumed(10.0)
	assert_int(h.http.count("POST /game/sessions/*:heartbeat")).is_equal(1)
	h.clock.advance(600)
	h.http.reply("POST /auth/game/token:refresh", 200, AuthHarness.refresh_ok(AuthHarness.ACCESS_2, AuthHarness.REFRESH_2, false))
	h.http.reply("POST /game/sessions", 201, AuthHarness.session_ok())
	await h.auth.on_app_resumed(600.0)
	assert_int(h.http.count("POST /game/sessions")).is_equal(2)
	assert_int(h.http.count("POST /auth/game/login")).is_equal(1)
	assert_int(h.http.count("POST /auth/game/token:refresh")).is_equal(1)
	assert_str(String(h.http.last("POST /game/sessions").headers["authorization"])).is_equal("Bearer %s" % AuthHarness.ACCESS_2)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.IN_WORLD)
