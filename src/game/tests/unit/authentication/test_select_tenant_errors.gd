extends GdUnitTestSuite
## F061-AC-14 (TC-AUTH-065, `game_login_ticket_ttl`): `401 LOGIN_TICKET_INVALID` or `403 TENANT_MISMATCH` from
## `selectGameLoginTenant` → "Phiên chọn trường đã hết, em đăng nhập lại nhé", ticket erased, form with an empty
## password field.

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()


func after_test() -> void:
	h.dispose()


func test_expired_ticket_or_mismatch_returns_to_the_form() -> void:
	for pair: Array in [[401, "LOGIN_TICKET_INVALID"], [403, "TENANT_MISMATCH"]]:
		h.http.reply("POST /auth/game/login", 403, AuthHarness.tenant_choices())
		await h.auth.login("hs.multi", AuthHarness.PASSWORD)
		var cleared: int = h.auth.form.password_clear_count
		h.http.reply("POST /auth/game/tenant:select", int(pair[0]), AuthHarness.problem(int(pair[0]), String(pair[1])))
		await h.auth.select_tenant(AuthHarness.TENANT_ID)
		assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
		assert_str(h.auth.form.banner_code).is_equal("LOGIN_TICKET_INVALID")
		assert_str(LoginFormModel.message_for(h.auth.form.banner_code)).is_equal(tr("ui.login.ticket_expired"))
		assert_int(h.auth.form.password_clear_count).is_greater(cleared)
		assert_array(h.auth.choices).is_empty()
		assert_bool(h.auth.tokens.has_access()).is_false()
	var before: int = h.http.count("POST /auth/game/tenant:select")
	await h.auth.select_tenant(AuthHarness.TENANT_ID)
	assert_int(h.http.count("POST /auth/game/tenant:select")).is_equal(before)


func test_back_button_forgets_the_ticket() -> void:
	h.http.reply("POST /auth/game/login", 403, AuthHarness.tenant_choices())
	await h.auth.login("hs.multi", AuthHarness.PASSWORD)
	h.auth.back_to_login()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
	await h.auth.select_tenant(AuthHarness.TENANT_ID)
	assert_int(h.http.count("POST /auth/game/tenant:select")).is_equal(0)
