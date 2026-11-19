extends GdUnitTestSuite
## F061-AC-33 (TC-AUTH-085, ADR-0086): refresh is single-flight and a refresh token that was replaced is never sent
## again; `401 REFRESH_TOKEN_REUSED` → telemetry `auth_refresh_reused` (no token value) and the F061-AC-30 path.

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()


func after_test() -> void:
	h.dispose()


func test_each_refresh_sends_the_latest_token() -> void:
	await h.sign_in()
	h.http.reply("POST /auth/game/token:refresh", 200, AuthHarness.refresh_ok(AuthHarness.ACCESS_2, AuthHarness.REFRESH_2, false))
	assert_bool(await h.auth.refresh()).is_true()
	h.http.reply("POST /auth/game/token:refresh", 200, AuthHarness.refresh_ok(AuthHarness.ACCESS_3, AuthHarness.REFRESH_3, false))
	assert_bool(await h.auth.refresh()).is_true()
	var sent: Array[FakeHttp.Recorded] = h.http.find("POST /auth/game/token:refresh")
	assert_str(String(sent[0].body["refresh_token"])).is_equal(AuthHarness.REFRESH_1)
	assert_str(String(sent[1].body["refresh_token"])).is_equal(AuthHarness.REFRESH_2)
	assert_str(String(h.stored_record()["refresh_token"])).is_equal(AuthHarness.REFRESH_3)


func test_concurrent_refresh_calls_share_one_request() -> void:
	await h.sign_in()
	h.http.reply("POST /auth/game/token:refresh", 200, AuthHarness.refresh_ok(AuthHarness.ACCESS_2, AuthHarness.REFRESH_2, false))
	var results: Array[bool] = []
	var run: Callable = func() -> void: results.append(await h.auth.refresh(AuthHarness.ACCESS_1))
	for i: int in 4:
		run.call()
	for i: int in 30:
		if results.size() == 4:
			break
		await get_tree().process_frame
	assert_array(results).is_equal([true, true, true, true])
	assert_int(h.http.count("POST /auth/game/token:refresh")).is_equal(1)


func test_reuse_is_reported_without_the_token() -> void:
	await h.sign_in()
	h.http.reply("POST /auth/game/token:refresh", 401, AuthHarness.problem(401, "REFRESH_TOKEN_REUSED"))
	assert_bool(await h.auth.refresh()).is_false()
	var reused: Array[Dictionary] = []
	for e: Dictionary in AppState.pending_telemetry:
		if String(e["event"]) == "auth_refresh_reused":
			reused.append(e)
	assert_int(reused.size()).is_equal(1)
	assert_str(JSON.stringify(reused)).not_contains(AuthHarness.REFRESH_1)
	assert_dict(h.stored_record()).is_empty()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
