extends GdUnitTestSuite
## F061-AC-13 (TC-TEN-014, ADR-0063): `403 TENANT_NOT_SELECTED` → `tenant_choice_required(choices)` and the
## TenantChoice screen; choosing sends `selectGameLoginTenant` with `login_ticket`, `tenant_id`, `device_id` only
## (no password, no token, no `X-Tenant-Id`); the ticket stays in RAM.

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()


func after_test() -> void:
	h.dispose()


func test_tenant_choice_then_select_enters_the_game() -> void:
	var offered: Array[int] = []
	h.auth.tenant_choice_required.connect(func(choices: Array) -> void: offered.append(choices.size()))
	h.http.reply("POST /auth/game/login", 403, AuthHarness.tenant_choices())
	await h.auth.login("hs.multi", AuthHarness.PASSWORD)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.TENANT_CHOICE)
	assert_array(offered).is_equal([2])
	assert_int(h.auth.choices.size()).is_equal(2)
	assert_str(h.auth.choices[1].tenant_id).is_equal(AuthHarness.OTHER_TENANT_ID)
	assert_bool(h.auth.holds_password()).is_false()
	assert_dict(h.stored_record()).is_empty()
	h.http.reply("POST /auth/game/tenant:select", 200, AuthHarness.login_ok(AuthHarness.ACCESS_1, AuthHarness.REFRESH_1,
			AuthHarness.OTHER_TENANT_ID))
	h.http.reply("POST /game/sessions", 201, AuthHarness.session_ok())
	await h.auth.select_tenant(AuthHarness.OTHER_TENANT_ID)
	var rec: FakeHttp.Recorded = h.http.last("POST /auth/game/tenant:select")
	assert_array(rec.body.keys()).contains_exactly_in_any_order(["login_ticket", "tenant_id", "device_id"])
	assert_str(String(rec.body["login_ticket"])).is_equal(AuthHarness.TICKET)
	assert_str(String(rec.body["tenant_id"])).is_equal(AuthHarness.OTHER_TENANT_ID)
	assert_str(String(rec.body["device_id"])).is_equal(AuthHarness.DEVICE_ID)
	assert_str(rec.raw_body).not_contains(AuthHarness.PASSWORD)
	assert_bool(rec.headers.has("authorization")).is_false()
	assert_bool(rec.headers.has("x-tenant-id")).is_false()
	assert_str(h.auth.tokens.tenant_id).is_equal(AuthHarness.OTHER_TENANT_ID)
	assert_str(String(h.http.last("POST /game/sessions").headers["x-tenant-id"])).is_equal(AuthHarness.OTHER_TENANT_ID)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.IN_WORLD)


func test_ticket_is_used_once() -> void:
	h.http.reply("POST /auth/game/login", 403, AuthHarness.tenant_choices())
	await h.auth.login("hs.multi", AuthHarness.PASSWORD)
	h.http.reply("POST /auth/game/tenant:select", 503, AuthHarness.problem(503, "UNAVAILABLE"))
	await h.auth.select_tenant(AuthHarness.TENANT_ID)
	await h.auth.select_tenant(AuthHarness.TENANT_ID)
	assert_int(h.http.count("POST /auth/game/tenant:select")).is_equal(1)
	assert_int(h.last_state()).is_equal(GameStateMachine.State.LOGIN)
