extends GdUnitTestSuite
## F061-AC-01 and F061-AC-27 (TC-AUTH-066, FR-GAME-008): `getClientConfig` (platform, version, no token, no
## `X-Tenant-Id`) runs before the form; `update_required` or a version below `min_client_version` → ForceUpdate
## without login; `maintenance.active` → Maintenance, checked again every 60 s; `426` at login → ForceUpdate with
## `min_client_version` and `store_url`, no retry.

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()


func after_test() -> void:
	h.dispose()


func _config(overrides: Dictionary) -> Dictionary:
	var c: Dictionary = AuthHarness.fixture("ClientConfig")
	c["min_client_version"] = "1.0.0"
	c["update_required"] = false
	c["maintenance"] = {"active": false}
	c.merge(overrides, true)
	return c


func test_semver_comparison() -> void:
	assert_int(VersionCheck.compare_versions("1.2.3", "1.2.3")).is_equal(0)
	assert_int(VersionCheck.compare_versions("1.2.3", "1.10.0")).is_equal(-1)
	assert_int(VersionCheck.compare_versions("2.0.0", "1.99.99")).is_equal(1)
	assert_int(VersionCheck.compare_versions("1.0.0-rc.1", "1.0.0")).is_equal(-1)
	assert_int(VersionCheck.compare_versions("1.0.0+build.7", "1.0.0")).is_equal(0)


func test_client_config_comes_first_and_is_public() -> void:
	h.http.reply("GET /game/client-config", 200, _config({}))
	await h.auth.start()
	assert_str(h.http.requests[0].path).is_equal("/game/client-config")
	var rec: FakeHttp.Recorded = h.http.requests[0]
	assert_str(rec.query).contains("platform=windows")
	assert_str(rec.query).contains("version=%s" % AuthHarness.VERSION)
	assert_bool(rec.headers.has("authorization")).is_false()
	assert_bool(rec.headers.has("x-tenant-id")).is_false()
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)


func test_old_client_is_forced_to_update() -> void:
	for overrides: Dictionary in [{"min_client_version": "1.3.0"}, {"update_required": true}]:
		h.dispose()
		h = AuthHarness.create()
		h.store_session()
		h.http.reply("GET /game/client-config", 200, _config(overrides))
		await h.auth.start()
		assert_array(h.states).is_equal([GameStateMachine.State.FORCE_UPDATE])
		assert_int(h.http.count("POST /auth/game/*")).is_equal(0)
		assert_str(String(h.stored_record()["refresh_token"])).is_equal(AuthHarness.REFRESH_1)


func test_maintenance_is_checked_again() -> void:
	h.http.reply("GET /game/client-config", 200, _config({"maintenance": {"active": true}}))
	h.http.reply("GET /game/client-config", 200, _config({}))
	await h.auth.start()
	assert_array(h.states).is_equal([GameStateMachine.State.MAINTENANCE, GameStateMachine.State.LOGIN])
	assert_int(h.http.count("GET /game/client-config")).is_equal(2)
	assert_array(h.clock.waits).contains([GameAuthService.MAINTENANCE_RECHECK_SEC])


func test_login_426_shows_force_update_once() -> void:
	h.http.reply("POST /auth/game/login", 426, AuthHarness.problem(426, "CLIENT_VERSION_UNSUPPORTED",
			{"min_client_version": "1.5.0", "store_url": "https://store.example.invalid/eduworld"}, "ClientVersionProblem"))
	await h.auth.login("hs.an.3a", AuthHarness.PASSWORD)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.FORCE_UPDATE)
	assert_str(h.auth.min_client_version).is_equal("1.5.0")
	assert_str(h.auth.store_url).is_equal("https://store.example.invalid/eduworld")
	assert_int(h.http.count("POST /auth/game/login")).is_equal(1)
	assert_bool(h.auth.holds_password()).is_false()


func test_unreachable_client_config_falls_back_to_the_server_check() -> void:
	h.http.fail("GET /game/client-config")
	await h.auth.start()
	assert_int(h.http.count("GET /game/client-config")).is_equal(4)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
