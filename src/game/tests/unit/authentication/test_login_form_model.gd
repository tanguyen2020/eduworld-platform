extends GdUnitTestSuite
## F061-AC-05, F061-AC-07, F061-AC-08, F061-AC-09 (TC-AUTH-062, TC-AUTH-064, TC-AUTH-013, TC-AUTH-032,
## GODOT_ARCHITECTURE §9): each `gameLogin` refusal maps to its screen/message by `code` (never `detail`), keeps
## no token and is never retried automatically; the password field is emptied after every final response.

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()


func after_test() -> void:
	TranslationServer.set_locale("vi-VN")
	h.dispose()


func _login_with(status: int, code: String) -> void:
	h.http.reply("POST /auth/game/login", status, AuthHarness.problem(status, code, {"detail": "server detail text"}))
	await h.auth.login("hs.an.3a", AuthHarness.PASSWORD)


func test_invalid_credentials_one_message_and_wait_hint_from_third_try() -> void:
	var cleared_before: int = h.auth.form.password_clear_count
	await _login_with(401, "INVALID_CREDENTIALS")
	assert_int(h.http.count("POST /auth/game/login")).is_equal(1)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
	assert_str(h.auth.form.error_code).is_equal("INVALID_CREDENTIALS")
	assert_str(LoginFormModel.message_for(h.auth.form.error_code)).is_equal(tr("ui.login.invalid_credentials"))
	assert_int(h.auth.form.password_clear_count).is_greater(cleared_before)
	assert_bool(h.auth.form.show_wait_hint()).is_false()
	await h.auth.login("hs.an.3a", AuthHarness.PASSWORD)
	await h.auth.login("hs.an.3a", AuthHarness.PASSWORD)
	assert_int(h.http.count("POST /auth/game/login")).is_equal(3)
	assert_bool(h.auth.form.show_wait_hint()).is_true()
	assert_bool(h.auth.holds_password()).is_false()
	assert_bool(h.auth.tokens.has_access()).is_false()


func test_staff_account_shows_the_portal_notice() -> void:
	await _login_with(403, "GAME_ACCESS_STUDENT_ONLY")
	assert_int(h.last_state()).is_equal(GameStateMachine.State.NOT_STUDENT)
	assert_bool(h.auth.tokens.has_access()).is_false()
	assert_str(h.auth.tokens.refresh_token).is_empty()
	h.auth.back_to_login()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)


func test_not_provisioned_clears_everything() -> void:
	h.store_session()
	await _login_with(403, "ACCOUNT_NOT_PROVISIONED")
	assert_int(h.last_state()).is_equal(GameStateMachine.State.NOT_PROVISIONED)
	assert_bool(h.auth.tokens.has_access()).is_false()
	assert_dict(h.stored_record()).is_empty()


func test_blocked_codes_use_translated_messages_not_detail() -> void:
	for code: String in ["ACCOUNT_LOCKED", "ACCOUNT_DEACTIVATED", "NO_ACTIVE_ENROLLMENT", "TENANT_ACCESS_DISABLED",
			"ACCESS_OUTSIDE_ALLOWED_HOURS"]:
		await _login_with(403, code)
		assert_int(h.last_state()).override_failure_message(code).is_equal(GameStateMachine.State.ACCOUNT_BLOCKED)
		assert_str(h.auth.notice_code).is_equal(code)
		TranslationServer.set_locale("vi-VN")
		var vi: String = L10n.error_text(code)
		TranslationServer.set_locale("en-US")
		var en: String = L10n.error_text(code)
		TranslationServer.set_locale("vi-VN")
		assert_str(vi).is_equal(tr("ui.error.%s" % code))
		assert_str(vi).is_not_equal(en)
		assert_str(vi).not_contains("server detail text")
	assert_int(h.http.count("POST /auth/game/login")).is_equal(5)


func test_validation_and_network_errors_stay_on_the_form() -> void:
	await _login_with(422, "VALIDATION_FAILED")
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
	assert_str(LoginFormModel.message_for(h.auth.form.error_code)).is_equal(tr("ui.error.VALIDATION_FAILED"))
	h.http.fail("POST /auth/game/login")
	await h.auth.login("hs.an.3a", AuthHarness.PASSWORD)
	assert_str(h.auth.form.error_code).is_equal(ApiResult.CODE_NETWORK_ERROR)
	TranslationServer.set_locale("en-US")
	assert_str(LoginFormModel.message_for(h.auth.form.error_code)).is_equal("No Internet connection. Check the network and try again.")
	assert_int(h.http.count("POST /auth/game/login")).is_equal(2)
	assert_bool(h.auth.holds_password()).is_false()


func test_state_mapping_is_pure() -> void:
	assert_int(GameStateMachine.state_for_login_failure("TENANT_NOT_SELECTED")).is_equal(GameStateMachine.State.TENANT_CHOICE)
	assert_int(GameStateMachine.state_for_login_failure("CLIENT_VERSION_UNSUPPORTED")).is_equal(GameStateMachine.State.FORCE_UPDATE)
	assert_int(GameStateMachine.state_for_login_failure("CURFEW_ACTIVE")).is_equal(GameStateMachine.State.CURFEW)
	assert_int(GameStateMachine.state_for_login_failure("WHATEVER")).is_equal(GameStateMachine.State.LOGIN)
	assert_bool(GameStateMachine.is_refresh_fatal(401, "REFRESH_TOKEN_REUSED")).is_true()
	assert_bool(GameStateMachine.is_refresh_fatal(403, "TENANT_MISMATCH")).is_true()
	assert_bool(GameStateMachine.is_refresh_fatal(503, "UNAVAILABLE")).is_false()
	assert_bool(GameStateMachine.is_refresh_fatal(403, "CURFEW_ACTIVE")).is_false()
