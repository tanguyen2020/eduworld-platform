extends GdUnitTestSuite
## F061-AC-30 (TC-AUTH-083, TC-AUTH-086) and F061-AC-34 (TC-AUTH-087): resume/renew answered `401
## REFRESH_TOKEN_INVALID` / `REFRESH_TOKEN_REUSED` / `REFRESH_TOKEN_DEVICE_MISMATCH` or a `403` account/tenant code
## → SessionStore + RAM erased, form with `ui.login.session_expired` (401) or `ui.error.<CODE>` (403), username
## prefilled when remembered — exactly one attempt, no retry loop.

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()


func after_test() -> void:
	h.dispose()


func _resume_with(status: int, code: String) -> void:
	h.dispose()
	h = AuthHarness.create()
	h.settings.call("set_login_memory", true, "hs.an.3a")
	h.auth.configure(h.api, h.clock, h.storage, h.settings, AuthHarness.DEVICE_ID, AuthHarness.VERSION, AuthHarness.PLATFORM, false)
	h.auth.navigator = func(s: int) -> void: h.states.append(s)
	h.store_session()
	h.http.reply("GET /game/client-config", 200, AuthHarness.fixture("ClientConfig"))
	h.http.reply("POST /auth/game/token:refresh", status, AuthHarness.problem(status, code))
	await h.auth.start()
	for i: int in 5:
		await get_tree().process_frame


func test_refresh_token_errors_show_the_form_with_session_expired() -> void:
	for code: String in ["REFRESH_TOKEN_INVALID", "REFRESH_TOKEN_REUSED", "REFRESH_TOKEN_DEVICE_MISMATCH"]:
		await _resume_with(401, code)
		assert_int(h.http.count("POST /auth/game/token:refresh")).override_failure_message(code).is_equal(1)
		assert_dict(h.stored_record()).override_failure_message(code).is_empty()
		assert_str(h.auth.tokens.refresh_token).is_empty()
		assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
		assert_str(LoginFormModel.message_for(h.auth.form.banner_code)).is_equal(tr("ui.login.session_expired"))
		assert_str(h.auth.form.username).is_equal("hs.an.3a")
		assert_int(h.http.count("POST /game/sessions")).is_equal(0)


func test_account_and_tenant_403_erase_the_session() -> void:
	await _resume_with(403, "ACCOUNT_LOCKED")
	assert_dict(h.stored_record()).is_empty()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.ACCOUNT_BLOCKED)
	assert_str(h.auth.notice_code).is_equal("ACCOUNT_LOCKED")
	await _resume_with(403, "TENANT_MISMATCH")
	assert_dict(h.stored_record()).is_empty()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
	assert_str(LoginFormModel.message_for(h.auth.form.banner_code)).is_equal(tr("ui.login.relogin"))
	await _resume_with(403, "GAME_ACCESS_STUDENT_ONLY")
	assert_dict(h.stored_record()).is_empty()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.NOT_STUDENT)
	assert_int(h.http.count("POST /auth/game/token:refresh")).is_equal(1)


func test_revoked_session_while_playing_ends_at_the_form_once() -> void:
	await h.sign_in()
	h.http.reply("GET /game/profile", 401, AuthHarness.problem(401, "UNAUTHENTICATED"))
	h.http.reply("POST /auth/game/token:refresh", 401, AuthHarness.problem(401, "REFRESH_TOKEN_INVALID"))
	await h.api.send(ApiRequest.make_get("/game/profile"))
	await h.api.send(ApiRequest.make_get("/game/profile"))
	assert_int(h.http.count("POST /auth/game/token:refresh")).is_equal(1)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
	assert_dict(h.stored_record()).is_empty()
