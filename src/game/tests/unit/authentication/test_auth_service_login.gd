extends GdUnitTestSuite
## F061-AC-03 (TC-AUTH-001, ADR-0063, ADR-0041): `gameLogin` body is exactly `GameLoginRequest` (`username`
## trimmed, `password`, `device_id`, `client_version`, `platform`, `locale`) over HTTPS, without
## `Authorization`, `X-Tenant-Id`, `Idempotency-Key`; field lengths follow the schema `maxLength`.

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()


func after_test() -> void:
	h.dispose()


func test_login_request_body_and_headers() -> void:
	await h.sign_in()
	var rec: FakeHttp.Recorded = h.http.last("POST /auth/game/login")
	assert_str(rec.url).starts_with("https://")
	assert_array(rec.body.keys()).contains_exactly_in_any_order(
			["username", "password", "device_id", "client_version", "platform", "locale"])
	assert_str(String(rec.body["username"])).is_equal("hs.an.3a")
	assert_str(String(rec.body["password"])).is_equal(AuthHarness.PASSWORD)
	assert_str(String(rec.body["device_id"])).is_equal(AuthHarness.DEVICE_ID)
	assert_str(String(rec.body["client_version"])).is_equal(AuthHarness.VERSION)
	assert_str(String(rec.body["platform"])).is_equal("windows")
	assert_str(String(rec.body["locale"])).is_equal(TranslationServer.get_locale().replace("_", "-"))
	for header: String in ["authorization", "x-tenant-id", "idempotency-key"]:
		assert_bool(rec.headers.has(header)).override_failure_message(header).is_false()


func test_username_is_trimmed_and_limited() -> void:
	h.script_login_and_session()
	await h.auth.login("   hs.an.3a \t", AuthHarness.PASSWORD)
	assert_str(String(h.http.last("POST /auth/game/login").body["username"])).is_equal("hs.an.3a")
	assert_int(LoginFormModel.normalize_username("a".repeat(300)).length()).is_equal(LoginFormModel.USERNAME_MAX_LENGTH)
	assert_int(LoginFormModel.USERNAME_MAX_LENGTH).is_equal(128)
	assert_int(LoginFormModel.PASSWORD_MAX_LENGTH).is_equal(256)


func test_empty_or_too_long_fields_send_nothing() -> void:
	h.script_login_and_session()
	await h.auth.login("", AuthHarness.PASSWORD)
	await h.auth.login("   ", AuthHarness.PASSWORD)
	await h.auth.login("hs.an.3a", "")
	await h.auth.login("hs.an.3a", "x".repeat(257))
	assert_int(h.http.count("POST /auth/game/login")).is_equal(0)
	assert_bool(LoginFormModel.can_submit("hs.an.3a", "x".repeat(256))).is_true()
