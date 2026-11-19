extends GdUnitTestSuite
## F061-AC-18 (TC-AUTH-002, TC-AUTH-003, TC-AUTH-068, ADR-0086): 5 requests hitting `401` share ONE
## `refreshGameToken` (`refresh_token`, `device_id`, `mode: "renew"`, `X-Tenant-Id`); the rotated refresh token
## overwrites SessionStore; each request is replayed once. Refresh `401 REFRESH_TOKEN_*` → RAM + SessionStore
## erased, `EventBus.session_expired`, form.

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()


func after_test() -> void:
	h.dispose()


func test_five_parallel_401_share_one_refresh() -> void:
	await h.sign_in()
	for i: int in 5:
		h.http.reply("GET /game/profile", 401, AuthHarness.problem(401, "UNAUTHENTICATED"))
	h.http.reply("GET /game/profile", 200, {"nickname": "x"})
	h.http.reply("POST /auth/game/token:refresh", 200, AuthHarness.refresh_ok(AuthHarness.ACCESS_2, AuthHarness.REFRESH_2, false))
	var results: Array[ApiResult] = []
	var run: Callable = func() -> void:
		results.append(await h.api.send(ApiRequest.make_get("/game/profile")))
	for i: int in 5:
		run.call()
	for i: int in 60:
		if results.size() == 5:
			break
		await get_tree().process_frame
	assert_int(results.size()).is_equal(5)
	for r: ApiResult in results:
		assert_bool(r.ok).is_true()
	assert_int(h.http.count("POST /auth/game/token:refresh")).is_equal(1)
	var refresh: FakeHttp.Recorded = h.http.last("POST /auth/game/token:refresh")
	assert_str(String(refresh.body["refresh_token"])).is_equal(AuthHarness.REFRESH_1)
	assert_str(String(refresh.body["device_id"])).is_equal(AuthHarness.DEVICE_ID)
	assert_str(String(refresh.body["mode"])).is_equal("renew")
	assert_str(String(refresh.headers["x-tenant-id"])).is_equal(AuthHarness.TENANT_ID)
	assert_bool(refresh.headers.has("authorization")).is_false()
	assert_int(h.http.count("GET /game/profile")).is_equal(10)
	for rec: FakeHttp.Recorded in h.http.find("GET /game/profile").slice(5):
		assert_str(String(rec.headers["authorization"])).is_equal("Bearer %s" % AuthHarness.ACCESS_2)
	assert_str(h.auth.tokens.refresh_token).is_equal(AuthHarness.REFRESH_2)
	assert_str(String(h.stored_record()["refresh_token"])).is_equal(AuthHarness.REFRESH_2)


func test_token_expiring_within_60_seconds_is_refreshed_first() -> void:
	await h.sign_in()
	h.clock.advance(250.0)
	h.http.reply("POST /auth/game/token:refresh", 200, AuthHarness.refresh_ok(AuthHarness.ACCESS_2, AuthHarness.REFRESH_2, false))
	h.http.reply("GET /game/quests", 200, {"items": []})
	var result: ApiResult = await h.api.send(ApiRequest.make_get("/game/quests"))
	assert_bool(result.ok).is_true()
	assert_int(h.http.count("POST /auth/game/token:refresh")).is_equal(1)
	assert_str(String(h.http.last("GET /game/quests").headers["authorization"])).is_equal("Bearer %s" % AuthHarness.ACCESS_2)


func test_refresh_failure_expires_the_session() -> void:
	for code: String in ["REFRESH_TOKEN_INVALID", "REFRESH_TOKEN_REUSED", "REFRESH_TOKEN_DEVICE_MISMATCH"]:
		h.dispose()
		h = AuthHarness.create()
		await h.sign_in()
		var expired: Array[int] = [0]
		var on_expired: Callable = func() -> void: expired[0] += 1
		EventBus.session_expired.connect(on_expired)
		h.http.reply("GET /game/profile", 401, AuthHarness.problem(401, "UNAUTHENTICATED"))
		h.http.reply("POST /auth/game/token:refresh", 401, AuthHarness.problem(401, code))
		var result: ApiResult = await h.api.send(ApiRequest.make_get("/game/profile"))
		EventBus.session_expired.disconnect(on_expired)
		assert_int(result.status).is_equal(401)
		assert_int(expired[0]).override_failure_message(code).is_equal(1)
		assert_bool(h.auth.tokens.has_access()).is_false()
		assert_str(h.auth.tokens.refresh_token).is_empty()
		assert_dict(h.stored_record()).override_failure_message(code).is_empty()
		assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
		assert_str(LoginFormModel.message_for(h.auth.form.banner_code)).is_equal(tr("ui.login.session_expired"))
		assert_int(h.http.count("GET /game/profile")).is_equal(1)


func test_transient_refresh_failure_keeps_the_session() -> void:
	await h.sign_in()
	h.http.reply("GET /game/profile", 401, AuthHarness.problem(401, "UNAUTHENTICATED"))
	h.http.reply("POST /auth/game/token:refresh", 503, AuthHarness.problem(503, "UNAVAILABLE"))
	await h.api.send(ApiRequest.make_get("/game/profile"))
	assert_str(h.auth.tokens.refresh_token).is_equal(AuthHarness.REFRESH_1)
	assert_str(String(h.stored_record()["refresh_token"])).is_equal(AuthHarness.REFRESH_1)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.IN_WORLD)
