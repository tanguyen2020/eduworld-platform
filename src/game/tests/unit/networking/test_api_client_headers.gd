extends GdUnitTestSuite
## F061-AC-16 (TC-TEN-011, ADR-0065, GAME_NETWORKING §2.1): standard headers on every request; no
## `Authorization` / `X-Tenant-Id` on `getClientConfig`, `gameLogin`, `selectGameLoginTenant`; a fresh
## `traceparent` per request; the caller's `Idempotency-Key` is kept on retry.

const TRACEPARENT: String = "^00-[0-9a-f]{32}-[0-9a-f]{16}-01$"

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()


func after_test() -> void:
	h.dispose()


func test_tenant_route_after_login_has_every_standard_header() -> void:
	await h.sign_in()
	h.http.reply("GET /game/content-manifest", 200, {"version": "1"})
	var result: ApiResult = await h.api.send(ApiRequest.make_get("/game/content-manifest"))
	assert_bool(result.ok).is_true()
	var rec: FakeHttp.Recorded = h.http.last("GET /game/content-manifest")
	assert_str(String(rec.headers["authorization"])).is_equal("Bearer %s" % AuthHarness.ACCESS_1)
	assert_str(String(rec.headers["x-tenant-id"])).is_equal(AuthHarness.TENANT_ID)
	assert_str(String(rec.headers["x-device-id"])).is_equal(AuthHarness.DEVICE_ID)
	assert_str(String(rec.headers["x-client-version"])).is_equal(AuthHarness.VERSION)
	assert_str(String(rec.headers["x-client-platform"])).is_equal(AuthHarness.PLATFORM)
	assert_str(String(rec.headers["accept-language"])).is_equal(TranslationServer.get_locale().replace("_", "-"))
	assert_object(RegEx.create_from_string(TRACEPARENT).search(String(rec.headers["traceparent"]))).is_not_null()
	assert_bool(rec.headers.has("idempotency-key")).is_false()


func test_public_routes_have_no_token_and_no_tenant() -> void:
	h.http.reply("GET /game/client-config", 200, AuthHarness.fixture("ClientConfig"))
	h.http.reply("POST /auth/game/login", 403, AuthHarness.tenant_choices())
	h.http.reply("POST /auth/game/tenant:select", 200, AuthHarness.login_ok())
	h.http.reply("POST /game/sessions", 201, AuthHarness.session_ok())
	h.auth.tokens.tenant_id = AuthHarness.TENANT_ID
	h.auth.tokens.access_token = "stale-access"
	await h.auth.version_check.run(h.api, AuthHarness.PLATFORM, AuthHarness.VERSION)
	h.auth.tokens.clear()
	await h.auth.login("hs.multi", AuthHarness.PASSWORD)
	await h.auth.select_tenant(AuthHarness.OTHER_TENANT_ID)
	for route: String in ["GET /game/client-config", "POST /auth/game/login", "POST /auth/game/tenant:select"]:
		var rec: FakeHttp.Recorded = h.http.last(route)
		assert_object(rec).override_failure_message(route).is_not_null()
		assert_bool(rec.headers.has("authorization")).override_failure_message(route).is_false()
		assert_bool(rec.headers.has("x-tenant-id")).override_failure_message(route).is_false()
		assert_bool(rec.headers.has("idempotency-key")).override_failure_message(route).is_false()
		assert_str(String(rec.headers["x-device-id"])).is_equal(AuthHarness.DEVICE_ID)
	assert_str(h.http.last("GET /game/client-config").query).contains("platform=windows")
	assert_str(h.http.last("GET /game/client-config").query).contains("version=1.2.3")
	var session: FakeHttp.Recorded = h.http.last("POST /game/sessions")
	assert_str(String(session.headers["x-tenant-id"])).is_equal(AuthHarness.TENANT_ID)


func test_traceparent_is_new_for_every_request() -> void:
	h.http.reply("GET /game/client-config", 200, AuthHarness.fixture("ClientConfig"))
	await h.api.send(ApiRequest.make_get("/game/client-config").as_public())
	await h.api.send(ApiRequest.make_get("/game/client-config").as_public())
	var all: Array[FakeHttp.Recorded] = h.http.find("GET /game/client-config")
	assert_int(all.size()).is_equal(2)
	assert_str(String(all[0].headers["traceparent"])).is_not_equal(String(all[1].headers["traceparent"]))


func test_idempotency_key_is_kept_on_retry() -> void:
	await h.sign_in()
	h.http.reply("POST /game/answers", 503).reply("POST /game/answers", 503).reply("POST /game/answers", 201, {})
	var req: ApiRequest = ApiRequest.make_post("/game/answers", {"answer_id": "a"})
	req.idempotency_key = Uuid.v7()
	var result: ApiResult = await h.api.send(req)
	assert_bool(result.ok).is_true()
	var sent: Array[FakeHttp.Recorded] = h.http.find("POST /game/answers")
	assert_int(sent.size()).is_equal(3)
	for rec: FakeHttp.Recorded in sent:
		assert_str(String(rec.headers["idempotency-key"])).is_equal(req.idempotency_key)
	assert_bool(Uuid.is_v7(req.idempotency_key)).is_true()


func test_release_build_refuses_plain_http() -> void:
	h.api.allow_insecure = false
	h.api.base_url = "http://api.test.invalid/api/v1"
	var result: ApiResult = await h.api.send(ApiRequest.make_get("/game/client-config").as_public())
	assert_str(result.code).is_equal(RestApiClient.CODE_INSECURE_TRANSPORT)
	assert_int(h.http.requests.size()).is_equal(0)
