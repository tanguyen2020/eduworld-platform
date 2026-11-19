extends GdUnitTestSuite
## F061-AC-29 (TC-AUTH-082, ADR-0086, ADR-0083): with a stored session the start-up shows ResumeSplash (no form) and
## calls `refreshGameToken` with `refresh_token`, `device_id`, `client_version`, `mode: "resume"` and `X-Tenant-Id`
## = stored tenant; on 200 the new refresh token overwrites SessionStore before any other request, the bootstrap
## is applied, ResumeConfirm is shown, then `createGameSession` (409 → takeover).

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()
	h.http.reply("GET /game/client-config", 200, AuthHarness.fixture("ClientConfig"))


func after_test() -> void:
	h.dispose()


func test_resume_skips_the_form() -> void:
	h.store_session()
	h.http.reply("POST /auth/game/token:refresh", 200, AuthHarness.refresh_ok(AuthHarness.ACCESS_2, AuthHarness.REFRESH_2, true))
	await h.auth.start()
	assert_array(h.states).contains_exactly([GameStateMachine.State.RESUMING, GameStateMachine.State.RESUME_CONFIRM])
	assert_array(h.states).not_contains([GameStateMachine.State.LOGIN])
	var rec: FakeHttp.Recorded = h.http.last("POST /auth/game/token:refresh")
	assert_str(String(rec.body["refresh_token"])).is_equal(AuthHarness.REFRESH_1)
	assert_str(String(rec.body["device_id"])).is_equal(AuthHarness.DEVICE_ID)
	assert_str(String(rec.body["client_version"])).is_equal(AuthHarness.VERSION)
	assert_str(String(rec.body["mode"])).is_equal("resume")
	assert_str(String(rec.headers["x-tenant-id"])).is_equal(AuthHarness.TENANT_ID)
	assert_bool(rec.headers.has("authorization")).is_false()
	# Rotation saved before anything else: no createGameSession yet, record already holds the new token.
	assert_str(String(h.stored_record()["refresh_token"])).is_equal(AuthHarness.REFRESH_2)
	assert_int(h.http.count("POST /game/sessions")).is_equal(0)
	assert_str(h.auth.nickname).is_equal("MeoXanh42")
	assert_str(String(AppState.player.get("tenant_id", ""))).is_equal(AuthHarness.TENANT_ID)
	h.http.reply("POST /game/sessions", 201, AuthHarness.session_ok())
	await h.auth.continue_resume()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.IN_WORLD)
	assert_str(String(h.http.last("POST /game/sessions").headers["authorization"])).is_equal("Bearer %s" % AuthHarness.ACCESS_2)
	assert_int(h.http.count("POST /auth/game/login")).is_equal(0)


func test_resume_then_takeover() -> void:
	h.store_session()
	h.http.reply("POST /auth/game/token:refresh", 200, AuthHarness.refresh_ok(AuthHarness.ACCESS_2, AuthHarness.REFRESH_2, true))
	await h.auth.start()
	h.http.reply("POST /game/sessions", 409, AuthHarness.problem(409, "ACTIVE_SESSION_ON_OTHER_DEVICE", {}, "ActiveSessionConflict"))
	await h.auth.continue_resume()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.TAKEOVER_CONFIRM)
	h.http.reply("POST /game/sessions", 201, AuthHarness.session_ok())
	await h.auth.confirm_takeover()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.IN_WORLD)


func test_no_stored_session_shows_the_form() -> void:
	await h.auth.start()
	assert_int(h.http.count("POST /auth/game/token:refresh")).is_equal(0)
	assert_array(h.states).is_equal([GameStateMachine.State.LOGIN])


func test_corrupt_record_is_erased_and_the_form_is_shown() -> void:
	h.storage.save(SessionStore.KEY, "not json".to_utf8_buffer())
	await h.auth.start()
	assert_int(h.http.count("POST /auth/game/token:refresh")).is_equal(0)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
	assert_bool(h.storage.calls.has("erase:%s" % SessionStore.KEY)).is_true()
