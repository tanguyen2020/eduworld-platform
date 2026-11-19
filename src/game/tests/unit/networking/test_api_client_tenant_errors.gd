extends GdUnitTestSuite
## F061-AC-19 (TC-TEN-011, ADR-0065): after login, `400 TENANT_HEADER_REQUIRED` → telemetry `client_bug`, tokens
## erased, form "Em hãy đăng nhập lại"; `403 TENANT_MISMATCH` → tokens erased, form.

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()


func after_test() -> void:
	h.dispose()


func _telemetry_events() -> Array[String]:
	var out: Array[String] = []
	for e: Dictionary in AppState.pending_telemetry:
		out.append(String(e["event"]))
	return out


func test_missing_tenant_header_is_a_client_bug() -> void:
	await h.sign_in()
	h.http.reply("GET /game/profile", 400, AuthHarness.problem(400, "TENANT_HEADER_REQUIRED"))
	var result: ApiResult = await h.api.send(ApiRequest.make_get("/game/profile"))
	assert_str(result.code).is_equal("TENANT_HEADER_REQUIRED")
	assert_array(_telemetry_events()).contains(["client_bug"])
	assert_bool(h.auth.tokens.has_access()).is_false()
	assert_str(h.auth.tokens.refresh_token).is_empty()
	assert_dict(h.stored_record()).is_empty()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
	assert_str(LoginFormModel.message_for(h.auth.form.banner_code)).is_equal(tr("ui.login.relogin"))


func test_tenant_mismatch_returns_to_the_form() -> void:
	await h.sign_in()
	h.http.reply("GET /game/profile", 403, AuthHarness.problem(403, "TENANT_MISMATCH"))
	await h.api.send(ApiRequest.make_get("/game/profile"))
	assert_array(_telemetry_events()).not_contains(["client_bug"])
	assert_bool(h.auth.tokens.has_access()).is_false()
	assert_dict(h.stored_record()).is_empty()
	assert_str(AppState.game_session_id).is_empty()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
	assert_str(h.auth.form.banner_code).is_equal("TENANT_MISMATCH")


func test_other_403_does_not_end_the_session() -> void:
	await h.sign_in()
	h.http.reply("GET /game/shop", 403, AuthHarness.problem(403, "FORBIDDEN"))
	await h.api.send(ApiRequest.make_get("/game/shop"))
	assert_bool(h.auth.tokens.has_access()).is_true()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.IN_WORLD)
