extends GdUnitTestSuite
## F061-AC-24 (TC-AUTH-017, CONFIG_DEFAULTS `ws_heartbeat` 15 s / `ws_timeout` 45 s): while there is a game session
## and no realtime connection, `heartbeatGameSession` (with `X-Device-Id`) runs every `ws_heartbeat`; it stops
## at logout.

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()


func after_test() -> void:
	h.dispose()


func _tick() -> void:
	h.clock.release()
	for i: int in 4:
		await get_tree().process_frame


func test_heartbeat_every_interval_until_logout() -> void:
	assert_float(GameSessionService.WS_HEARTBEAT_SEC).is_equal(15.0)
	assert_float(GameSessionService.WS_TIMEOUT_SEC).is_equal(45.0)
	await h.sign_in()
	h.http.reply("POST /game/sessions/*:heartbeat", 204)
	assert_int(h.http.count("POST /game/sessions/*:heartbeat")).is_equal(0)
	await _tick()
	await _tick()
	assert_int(h.http.count("POST /game/sessions/*:heartbeat")).is_equal(2)
	var beat: FakeHttp.Recorded = h.http.last("POST /game/sessions/*:heartbeat")
	assert_str(beat.path).is_equal("/game/sessions/%s:heartbeat" % AuthHarness.SESSION_ID)
	assert_str(String(beat.headers["x-device-id"])).is_equal(AuthHarness.DEVICE_ID)
	assert_str(String(beat.headers["x-tenant-id"])).is_equal(AuthHarness.TENANT_ID)
	assert_float(h.clock.waits.back()).is_equal(h.auth.sessions.heartbeat_interval_sec)
	await h.auth.logout()
	await _tick()
	await _tick()
	assert_int(h.http.count("POST /game/sessions/*:heartbeat")).is_equal(2)


func test_default_interval_is_ws_heartbeat() -> void:
	var service: GameSessionService = GameSessionService.new(h.api, h.clock, AuthHarness.VERSION, AuthHarness.PLATFORM)
	assert_float(service.heartbeat_interval_sec).is_equal(GameSessionService.WS_HEARTBEAT_SEC)
