extends GdUnitTestSuite
## F061-AC-21 (TC-AUTH-015, ADR-0031, ADR-0083, AC-UX-0103): `createGameSession` `409 ACTIVE_SESSION_ON_OTHER_DEVICE`
## → `takeover_required(other_device)` and the confirm dialog; "Tiếp tục ở đây" calls `createGameSession` again with
## `takeover: true`, a NEW `Idempotency-Key` and the current token — no password in RAM or in any request;
## "Ở lại máy kia" → `gameLogout`, RAM erased, form.

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()


func after_test() -> void:
	h.dispose()


func _login_into_conflict() -> void:
	h.http.reply("POST /auth/game/login", 200, AuthHarness.login_ok())
	h.http.reply("POST /game/sessions", 409, AuthHarness.problem(409, "ACTIVE_SESSION_ON_OTHER_DEVICE",
			{"other_device": {"platform": "android", "device_label": "Tablet Android", "last_seen_at": "2026-11-20T00:42:00Z"}},
			"ActiveSessionConflict"))
	await h.auth.login("hs.an.3a", AuthHarness.PASSWORD)


func test_conflict_shows_the_other_device() -> void:
	var devices: Array[String] = []
	h.auth.takeover_required.connect(func(d: ApiProblem.OtherDevice) -> void: devices.append(d.device_label))
	await _login_into_conflict()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.TAKEOVER_CONFIRM)
	assert_array(devices).is_equal(["Tablet Android"])
	assert_str(h.auth.other_device.platform).is_equal("android")
	assert_bool(h.auth.holds_password()).is_false()
	assert_str(AppState.game_session_id).is_empty()


func test_continue_here_retries_with_takeover_and_a_new_key() -> void:
	await _login_into_conflict()
	h.http.reply("POST /game/sessions", 201, AuthHarness.session_ok())
	await h.auth.confirm_takeover()
	var calls: Array[FakeHttp.Recorded] = h.http.find("POST /game/sessions")
	assert_int(calls.size()).is_equal(2)
	assert_bool(bool(calls[0].body["takeover"])).is_false()
	assert_bool(bool(calls[1].body["takeover"])).is_true()
	assert_str(String(calls[1].headers["idempotency-key"])).is_not_equal(String(calls[0].headers["idempotency-key"]))
	assert_str(String(calls[1].headers["authorization"])).is_equal("Bearer %s" % AuthHarness.ACCESS_1)
	for rec: FakeHttp.Recorded in h.http.requests.slice(1):
		assert_str(rec.raw_body).not_contains(AuthHarness.PASSWORD)
	assert_int(h.http.count("POST /auth/game/login")).is_equal(1)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.IN_WORLD)
	assert_str(AppState.game_session_id).is_equal(AuthHarness.SESSION_ID)


func test_stay_on_the_other_device_logs_out() -> void:
	await _login_into_conflict()
	h.http.reply("POST /auth/game/logout", 204)
	await h.auth.decline_takeover()
	assert_int(h.http.count("POST /auth/game/logout")).is_equal(1)
	assert_str(String(h.http.last("POST /auth/game/logout").body["refresh_token"])).is_equal(AuthHarness.REFRESH_1)
	assert_int(h.http.count("POST /game/sessions/*:end")).is_equal(0)
	assert_bool(h.auth.tokens.has_access()).is_false()
	assert_dict(h.stored_record()).is_empty()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
