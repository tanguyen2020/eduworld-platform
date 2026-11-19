extends GdUnitTestSuite
## F061-AC-35 (TC-AUTH-091, AC-AUTH-031, `resume_confirm_screen`): resume 200 → S01b "Tiếp tục là <nickname>?" BEFORE
## `createGameSession`; "Tiếp tục" → createGameSession; "Không phải tôi" → `gameLogout`, SecureStorage + RAM
## erased, form without a prefilled username. Only the game nickname is shown, never the real name.

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()
	h.settings.call("set_login_memory", true, "hs.an.3a")
	h.store_session()
	h.http.reply("GET /game/client-config", 200, AuthHarness.fixture("ClientConfig"))
	h.http.reply("POST /auth/game/token:refresh", 200, AuthHarness.refresh_ok(AuthHarness.ACCESS_2, AuthHarness.REFRESH_2, true))


func after_test() -> void:
	h.dispose()


func test_confirm_screen_comes_before_the_game_session() -> void:
	var resumed: Array[String] = []
	h.auth.session_resumed.connect(func(n: String) -> void: resumed.append(n))
	await h.auth.start()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.RESUME_CONFIRM)
	assert_array(resumed).is_equal(["MeoXanh42"])
	assert_int(h.http.count("POST /game/sessions")).is_equal(0)
	h.http.reply("POST /game/sessions", 201, AuthHarness.session_ok())
	await h.auth.continue_resume()
	assert_int(h.http.count("POST /game/sessions")).is_equal(1)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.IN_WORLD)


func test_not_me_logs_out_and_shows_an_empty_form() -> void:
	await h.auth.start()
	h.http.reply("POST /auth/game/logout", 204)
	await h.auth.switch_account()
	var logout: FakeHttp.Recorded = h.http.last("POST /auth/game/logout")
	assert_object(logout).is_not_null()
	assert_str(String(logout.body["refresh_token"])).is_equal(AuthHarness.REFRESH_2)
	assert_dict(h.stored_record()).is_empty()
	assert_bool(h.auth.tokens.has_access()).is_false()
	assert_str(h.auth.form.username).is_empty()
	assert_int(h.http.count("POST /game/sessions")).is_equal(0)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)


func test_only_the_nickname_is_used() -> void:
	var b: Dictionary = AuthHarness.bootstrap()
	(b["user"] as Dictionary)["display_name"] = "Nguyen Van An"
	b.erase("game_profile")
	assert_str(LoginFormModel.nickname_of(b)).is_empty()
	assert_str(LoginFormModel.nickname_of(AuthHarness.bootstrap(AuthHarness.TENANT_ID, "SaoBang7"))).is_equal("SaoBang7")
