extends GdUnitTestSuite
## F061-AC-26 (TC-AUTH-084, AC-UX-0101, ADR-0086): window close on a personal device only ends the game session
## (`endGameSession`, no `gameLogout`) and keeps the stored session; on Web / `shared_device` the next start always
## shows the form with an empty password field and never uses an old token.

var h: AuthHarness = null


func after_test() -> void:
	if h != null:
		h.dispose()
		h = null


func test_personal_device_close_keeps_the_stored_session() -> void:
	h = AuthHarness.create(false)
	await h.sign_in()
	await h.auth.close_game()
	assert_int(h.http.count("POST /game/sessions/%s:end" % AuthHarness.SESSION_ID)).is_equal(1)
	assert_int(h.http.count("POST /auth/game/logout")).is_equal(0)
	assert_str(String(h.stored_record()["refresh_token"])).is_equal(AuthHarness.REFRESH_1)
	assert_bool(h.auth.tokens.has_access()).is_false()
	assert_int(h.quits).is_greater_equal(1)
	# Next start on the same device resumes without the form.
	var next: AuthHarness = AuthHarness.create(false)
	next.storage = h.storage
	next.auth.configure(next.api, next.clock, h.storage, next.settings, AuthHarness.DEVICE_ID, AuthHarness.VERSION,
			AuthHarness.PLATFORM, false)
	next.http.reply("GET /game/client-config", 200, AuthHarness.fixture("ClientConfig"))
	next.http.reply("POST /auth/game/token:refresh", 200, AuthHarness.refresh_ok(AuthHarness.ACCESS_2, AuthHarness.REFRESH_2, true))
	await next.auth.start()
	assert_array(next.states).not_contains([GameStateMachine.State.LOGIN])
	assert_int(next.last_state()).is_equal(GameStateMachine.State.RESUME_CONFIRM)
	next.dispose()


func test_shared_device_start_always_shows_an_empty_form() -> void:
	h = AuthHarness.create(true, false)
	await h.sign_in()
	await h.auth.close_game()
	var next: AuthHarness = AuthHarness.create(true, false)
	next.http.reply("GET /game/client-config", 200, AuthHarness.fixture("ClientConfig"))
	await next.auth.start()
	assert_array(next.states).is_equal([GameStateMachine.State.LOGIN])
	assert_int(next.http.count("POST /auth/game/token:refresh")).is_equal(0)
	assert_str(next.auth.form.username).is_empty()
	assert_bool(next.auth.holds_password()).is_false()
	next.dispose()
